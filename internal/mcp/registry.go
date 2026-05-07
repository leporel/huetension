// Package mcp wires the huetension internal libraries (color, palette,
// harmony, …) up as an MCP server.
//
// The package is namespaced as huemcp when imported by the CLI so it does
// not collide with the SDK's own "mcp" package — internally we alias the
// SDK as `sdk` so call sites read like `sdk.NewServer(...)`.
//
// Slices A–C wired the read-only color/palette/export tools. Slice D adds
// image.extract / image.extractBatch behind a sandbox (ReadOnly, Root,
// AllowHosts, MaxImageBytes) configured via Config.
package mcp

import (
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/leporel/huetension/internal/mcp/tools"
)

// Descriptor describes a single MCP tool we expose, plus the function that
// registers it on a *sdk.Server. The registration is split from metadata so
// `--list-tools` can list the catalogue without spinning up a server.
//
// register receives a tools.Deps struct so tools that need cross-cutting
// configuration (image.* tools want a sandbox) can read it. Tools that
// don't are wrapped via simple() to ignore the parameter.
type Descriptor struct {
	Name           string
	Description    string
	DefaultEnabled bool
	register       func(*sdk.Server, tools.Deps)
}

// simple lifts a no-deps register function (most tools) into the Deps-aware
// signature the catalogue uses. Keeps simple tools unchanged.
func simple(f func(*sdk.Server)) func(*sdk.Server, tools.Deps) {
	return func(s *sdk.Server, _ tools.Deps) { f(s) }
}

// allDescriptors is the canonical tool catalogue. New tools are added here
// as later slices implement them.
var allDescriptors = []Descriptor{
	{
		Name:           "color.convert",
		Description:    "Convert a color between hex, rgb, hsl, hsv, hls, lab, lch, oklab, oklch, CSS named, and integer formats.",
		DefaultEnabled: true,
		register:       simple(tools.RegisterColorConvert),
	},
	{
		Name:           "color.sort",
		Description:    "Sort a list of colors by luminance, lightness, OkLab L, hue, saturation, or frequency.",
		DefaultEnabled: true,
		register:       simple(tools.RegisterColorSort),
	},
	{
		Name:           "harmony.generate",
		Description:    "Generate a color harmony (complementary, analogous, triadic, split, tetradic/square, double-complementary, monochromatic, shades) around a base color.",
		DefaultEnabled: true,
		register:       simple(tools.RegisterHarmonyGenerate),
	},
	{
		Name:           "gradient.generate",
		Description:    "Build a color gradient between two endpoints (or through ≥2 stops) in OkLab/OkLCH/Lab/RGB/HSL space with optional easing.",
		DefaultEnabled: true,
		register:       simple(tools.RegisterGradientGenerate),
	},
	{
		Name:           "contrast.check",
		Description:    "Compute foreground/background contrast using WCAG 2.1, APCA, or both.",
		DefaultEnabled: true,
		register:       simple(tools.RegisterContrastCheck),
	},
	{
		Name:           "blindness.simulate",
		Description:    "Simulate how colors are perceived under protanopia/deuteranopia/tritanopia/achromatopsia.",
		DefaultEnabled: true,
		register:       simple(tools.RegisterBlindnessSimulate),
	},
	{
		Name:           "palette.random",
		Description:    "Generate a random palette (optionally driven by a harmony rule), deterministic when seeded.",
		DefaultEnabled: true,
		register:       simple(tools.RegisterPaletteRandom),
	},
	{
		Name:           "export.css",
		Description:    "Render a list of colors as CSS custom properties, SCSS variables, or LESS variables.",
		DefaultEnabled: true,
		register:       simple(tools.RegisterExportCSS),
	},
	{
		Name:           "export.tailwind",
		Description:    "Render a list of colors as a Tailwind theme.extend.colors snippet (flat or shade-scale).",
		DefaultEnabled: true,
		register:       simple(tools.RegisterExportTailwind),
	},
	{
		Name:           "image.extract",
		Description:    "Extract a color palette from an image (local path, http(s):// URL, or base64 bytes). Behaviour gated by the server's image sandbox (read-only, root, allow-hosts, max bytes).",
		DefaultEnabled: true,
		register:       tools.RegisterImageExtract,
	},
	{
		Name:           "image.extractBatch",
		Description:    "Extract palettes from multiple images in parallel. Per-source failures show as inline error fields.",
		DefaultEnabled: true,
		register:       tools.RegisterImageExtractBatch,
	},
}

// All returns a copy of the catalogue. Useful for `--list-tools` and tests.
func All() []Descriptor {
	out := make([]Descriptor, len(allDescriptors))
	copy(out, allDescriptors)
	return out
}

// Config controls which tools the server registers, the server's reported
// version, and the image sandbox.
type Config struct {
	// Version is reported in the MCP `serverInfo.version` field. The CLI
	// passes the binary's --version-stamped value.
	Version string

	// Enable, when non-empty, restricts registration to the listed tool
	// names. When empty, every tool with DefaultEnabled=true is included.
	Enable []string

	// Disable is applied after Enable: any name listed here is dropped.
	Disable []string

	// ReadOnly forbids tools that touch the local filesystem with write or
	// arbitrary-read intent. Currently this only affects image.extract's
	// `path` input.
	ReadOnly bool

	// Root constrains image.extract `path` inputs to a specific directory.
	// Empty disables filesystem access.
	Root string

	// AllowHosts gates image.extract URL fetches. Empty allows all hosts.
	AllowHosts []string

	// MaxImageBytes caps URL bodies and decoded base64 payloads for the
	// image.extract* tools. 0 falls back to imageio's 64 MiB default.
	MaxImageBytes int64

	// BlockPrivateNetworks, when true, refuses outbound TCP connections
	// (in image.extract URL fetches) to loopback / private / link-local
	// IPs. Closes a documented SSRF gap; the CLI auto-enables this for
	// HTTP/SSE transports.
	BlockPrivateNetworks bool

	// Transports lists the wire transports to start: any subset of
	// "stdio", "http", "sse". Empty defaults to ["stdio"]. The CLI's
	// --transport flag maps "all" → ["stdio","http","sse"].
	Transports []string

	// Address is the listen address for HTTP / SSE (e.g. ":7337" or
	// "127.0.0.1:7337"). Required when Transports includes "http" or
	// "sse"; ignored otherwise.
	Address string

	// BasePath is the URL prefix under which the streamable HTTP handler
	// is mounted. SSE is mounted at BasePath + "/sse". Default "/mcp".
	BasePath string

	// AuthToken, when non-empty, is required as a Bearer token on every
	// HTTP/SSE request. When the bind address is non-loopback, an empty
	// AuthToken is treated as a configuration error and the server
	// refuses to start (loopback binds are permissive).
	AuthToken string

	// CORSOrigins, when non-empty, sets Access-Control-Allow-Origin for
	// HTTP/SSE responses (one per origin; "*" is supported). Empty list
	// disables CORS handling entirely.
	CORSOrigins []string
}

// ResolveEnabled returns the descriptors that pass the enable/disable
// filters in cfg. Order matches the catalogue order in All(). Returns an
// error if cfg references an unknown tool — typos in --enable / --disable
// should fail loudly rather than silently disable everything.
func ResolveEnabled(cfg Config) ([]Descriptor, error) {
	enable := normaliseSet(cfg.Enable)
	disable := normaliseSet(cfg.Disable)

	known := make(map[string]struct{}, len(allDescriptors))
	for _, d := range allDescriptors {
		known[d.Name] = struct{}{}
	}
	for name := range enable {
		if _, ok := known[name]; !ok {
			return nil, fmt.Errorf("mcp: unknown tool in --enable: %q", name)
		}
	}
	for name := range disable {
		if _, ok := known[name]; !ok {
			return nil, fmt.Errorf("mcp: unknown tool in --disable: %q", name)
		}
	}

	out := make([]Descriptor, 0, len(allDescriptors))
	for _, d := range allDescriptors {
		if len(enable) > 0 {
			if _, ok := enable[d.Name]; !ok {
				continue
			}
		} else if !d.DefaultEnabled {
			continue
		}
		if _, blocked := disable[d.Name]; blocked {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

// normaliseSet trims and lower-cases each entry. Empty strings (e.g. from a
// stray comma) are dropped silently.
func normaliseSet(names []string) map[string]struct{} {
	if len(names) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		out[n] = struct{}{}
	}
	return out
}
