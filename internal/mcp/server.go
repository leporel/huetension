package mcp

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/leporel/huetension/internal/mcp/tools"
)

// implementationName is the MCP `serverInfo.name` reported to clients. Kept
// in sync with the binary name so logs and tool listings line up.
const implementationName = "huetension"

// depsFromConfig converts the public Config into the tools.Deps each tool
// register sees. Centralised so the mapping is in one place.
func depsFromConfig(cfg Config) tools.Deps {
	return tools.Deps{
		ImageSandbox: tools.ImageSandbox{
			ReadOnly:             cfg.ReadOnly,
			Root:                 cfg.Root,
			AllowHosts:           cfg.AllowHosts,
			MaxImageBytes:        cfg.MaxImageBytes,
			BlockPrivateNetworks: cfg.BlockPrivateNetworks,
		},
	}
}

// Build constructs an *sdk.Server with every tool that survives the cfg
// enable/disable filter registered. It does not run the server.
//
// Exposed for tests that drive the server through an in-memory transport.
func Build(cfg Config) (*sdk.Server, []Descriptor, error) {
	enabled, err := ResolveEnabled(cfg)
	if err != nil {
		return nil, nil, err
	}
	srv := sdk.NewServer(&sdk.Implementation{
		Name:    implementationName,
		Version: cfg.Version,
	}, nil)
	deps := depsFromConfig(cfg)
	for _, d := range enabled {
		d.register(srv, deps)
	}
	return srv, enabled, nil
}

// RunStdio builds the server per cfg and serves it over stdin/stdout until
// the client disconnects or ctx is cancelled.
func RunStdio(ctx context.Context, cfg Config) error {
	srv, _, err := Build(cfg)
	if err != nil {
		return err
	}
	if err := srv.Run(ctx, &sdk.StdioTransport{}); err != nil {
		return fmt.Errorf("mcp: stdio transport: %w", err)
	}
	return nil
}

// Run dispatches per cfg.Transports, blocking until the first transport
// returns an error or ctx is cancelled. When multiple transports are
// listed they run concurrently; the first failure cancels the rest.
//
// Empty Transports defaults to ["stdio"] for backwards compatibility with
// slice A–D callers.
func Run(ctx context.Context, cfg Config) error {
	tlist := normaliseTransports(cfg.Transports)
	if len(tlist) == 0 {
		tlist = []string{"stdio"}
	}

	srv, _, err := Build(cfg)
	if err != nil {
		return err
	}

	// Single transport — call directly, no goroutines / channel plumbing.
	if len(tlist) == 1 {
		return runTransport(ctx, srv, cfg, tlist[0])
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, len(tlist))
	for _, t := range tlist {
		go func() {
			errCh <- runTransport(ctx, srv, cfg, t)
		}()
	}

	// Wait for first non-nil error. Cancel ctx so the others wind down,
	// then drain. Returns the first failure; subsequent wind-down errors
	// are dropped because they are usually "context cancelled".
	var firstErr error
	for range tlist {
		if e := <-errCh; e != nil && firstErr == nil {
			firstErr = e
			cancel()
		}
	}
	return firstErr
}

func runTransport(ctx context.Context, srv *sdk.Server, cfg Config, transport string) error {
	switch transport {
	case "stdio":
		if err := srv.Run(ctx, &sdk.StdioTransport{}); err != nil {
			return fmt.Errorf("mcp: stdio transport: %w", err)
		}
		return nil
	case "http":
		return runHTTP(ctx, srv, cfg, false)
	case "sse":
		return runHTTP(ctx, srv, cfg, true)
	}
	return fmt.Errorf("mcp: unknown transport %q", transport)
}

func normaliseTransports(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, t := range in {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if t == "all" {
			for _, n := range []string{"stdio", "http", "sse"} {
				if _, ok := seen[n]; !ok {
					seen[n] = struct{}{}
					out = append(out, n)
				}
			}
			continue
		}
		if _, ok := seen[t]; !ok {
			seen[t] = struct{}{}
			out = append(out, t)
		}
	}
	return out
}
