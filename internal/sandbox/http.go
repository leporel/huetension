package sandbox

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/leporel/huetension/internal/imageio"
)

// HTTP-hardening defaults applied to every outbound image fetch. Kept
// here so MCP and Web can share one policy without re-tuning per
// transport — the timeouts match what an LLM-driven or browser-driven
// flow is expected to tolerate.
const (
	HTTPConnectTimeout = 5 * time.Second
	HTTPTotalTimeout   = 30 * time.Second
	HTTPMaxRedirects   = 5
)

// HardenedHTTPClient builds an http.Client with the constraints
// documented above: tight timeouts, redirect cap, host re-validation
// on every hop (defeats allowed-host → arbitrary-host redirect
// bypasses), and (when sandbox.BlockPrivateNetworks is true) refusal
// to dial loopback/private/link-local IPs even after DNS resolution.
//
// file:// is rejected by the standard library when DialContext refuses
// to service the scheme; URLs without http/https are screened in
// imageio.Load's prefix dispatch as well.
func HardenedHTTPClient(s ImageSandbox) *http.Client {
	dialer := &net.Dialer{
		Timeout:   HTTPConnectTimeout,
		KeepAlive: 30 * time.Second,
	}
	dialContext := dialer.DialContext
	if s.BlockPrivateNetworks {
		dialContext = blockPrivateDial(dialer)
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialContext,
		TLSHandshakeTimeout:   HTTPConnectTimeout,
		ResponseHeaderTimeout: HTTPTotalTimeout,
		ExpectContinueTimeout: HTTPConnectTimeout,
	}
	hosts := append([]string(nil), s.AllowHosts...)
	return &http.Client{
		Transport: transport,
		Timeout:   HTTPTotalTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= HTTPMaxRedirects {
				return fmt.Errorf("stopped after %d redirects", HTTPMaxRedirects)
			}
			if len(hosts) > 0 {
				if err := imageio.CheckHost(req.URL.String(), hosts); err != nil {
					return fmt.Errorf("redirect rejected: %w", err)
				}
			}
			return nil
		},
	}
}

// LoadOptionsFor returns the imageio.LoadOptions that match this
// sandbox: max bytes, allow-host list, total timeout, hardened client.
// Centralised so MCP and Web can share one wiring call.
func LoadOptionsFor(s ImageSandbox) imageio.LoadOptions {
	return imageio.LoadOptions{
		MaxBytes:     s.MaxImageBytes,
		Timeout:      HTTPTotalTimeout,
		AllowedHosts: s.AllowHosts,
		HTTPClient:   HardenedHTTPClient(s),
	}
}

// blockPrivateDial wraps a net.Dialer so connections to loopback,
// RFC1918 private, link-local, and multicast IPs are refused. The dial
// step receives the resolved IP, so DNS rebinding ("good" hostname →
// 127.0.0.1) is also caught here.
func blockPrivateDial(dialer *net.Dialer) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("ssrf check: %w", err)
		}
		// Resolve names to IPs so DNS-based rebinding can't slip past.
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("ssrf resolve %q: %w", host, err)
		}
		for _, ip := range ips {
			if ipIsBlocked(ip) {
				return nil, fmt.Errorf("ssrf: refused connection to %s (private/loopback/link-local IP)", ip)
			}
		}
		var lastErr error
		for _, ip := range ips {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("no addresses available for %s", host)
	}
}

func ipIsBlocked(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsUnspecified()
}
