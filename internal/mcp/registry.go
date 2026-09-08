// Package mcp wires the huetension internal libraries (color, palette,
// harmony, …) up as an MCP server.
//
// The package is namespaced as huemcp when imported by the CLI so it does
// not collide with the SDK's own "mcp" package — internally we alias the
// SDK as `sdk` so call sites read like `sdk.NewServer(...)`.
package mcp

import (
	"fmt"
	"log/slog"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/leporel/huetension/internal/mcp/tools"
	"github.com/leporel/huetension/internal/palette/library"
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
		Description:    "Generate a color harmony (complementary, analogous, triadic, split, tetradic/square, double-complementary, compound, monochromatic, shades) around a base color.",
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
	{
		Name:           "library.categories",
		Description:    "List the categories in the curated palette catalogue (with palette counts). Slugs are stable URL-safe identifiers usable as the 'category' filter on library.list.",
		DefaultEnabled: true,
		register:       tools.RegisterLibraryCategories,
	},
	{
		Name:           "library.list",
		Description:    "List palettes in the curated catalogue, optionally filtered by category (slug or display name) and/or tag.",
		DefaultEnabled: true,
		register:       tools.RegisterLibraryList,
	},
	{
		Name:           "library.get",
		Description:    "Fetch a single palette from the curated catalogue by id.",
		DefaultEnabled: true,
		register:       tools.RegisterLibraryGet,
	},
	{
		Name:           "library.save",
		Description:    "Save a palette to the on-disk library so it joins the catalogue (filed under the \"Saved\" category). The server generates the id. Disabled on a read-only server or one with no data directory.",
		DefaultEnabled: true,
		register:       tools.RegisterLibrarySave,
	},
}

// All returns a copy of the catalogue. Useful for `--list-tools` and tests.
func All() []Descriptor {
	out := make([]Descriptor, len(allDescriptors))
	copy(out, allDescriptors)
	return out
}

// enabledToolNames returns the registered tool names in catalogue order —
// used by the HTTP/SSE startup log so operators can see, at a glance,
// which tools the server is exposing.
func enabledToolNames(enabled []Descriptor) []string {
	out := make([]string, len(enabled))
	for i, d := range enabled {
		out[i] = d.Name
	}
	return out
}

// Config controls which tools the server registers, the server's reported
// version, and the image sandbox.
type Config struct {
	// Version is reported in the MCP `serverInfo.version` field. The CLI
	// passes the binary's --version-stamped value.
	Version string

	// Enable, when non-empty for a given kind, restricts registration of
	// that kind to the listed names. Entries are namespaced — bare names
	// ("color.convert") select tools for back-compat; "resources:..." and
	// "prompts:..." select those kinds. "kind:*" matches everything in
	// the kind.
	Enable []string

	// Disable is applied after Enable: any name listed here is dropped.
	// Same namespaced syntax as Enable.
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

	// LogLevel selects verbosity for the default logger built when Logger
	// is nil. Accepted values: "debug", "info", "warn", "error". Empty
	// defaults to "info". Ignored when Logger is set explicitly.
	LogLevel string

	// LogFormat selects the default logger's handler: "text" (default,
	// human-readable) or "json". Ignored when Logger is set explicitly.
	LogFormat string

	// Logger overrides the default logger. When nil, Build constructs a
	// logger writing to stderr in LogFormat at LogLevel. Tests and
	// embedding hosts can pass their own logger to capture or re-route
	// output. Stdio servers must avoid stdout — JSON-RPC owns it.
	Logger *slog.Logger

	// Library is the curated palette catalogue served by library.* tools.
	// nil disables those tools' actual work (they will return an error if
	// invoked); the tools themselves stay registered so the catalogue
	// listing reflects the canonical surface. The CLI loads this via
	// library.Load(externalPath) and threads it in.
	Library *library.Index

	// LibraryPath is the on-disk library.json the library.save tool
	// persists to. Empty disables saving (library.save returns an error);
	// the other library.* tools are read-only and ignore it. A read-only
	// server also refuses saves, regardless of this path.
	LibraryPath string

	// LibraryStore, when set, is used instead of building a store from
	// Library + LibraryPath. The combined `serve` command shares one store
	// between the REST and MCP surfaces so saves never clobber each other.
	LibraryStore *library.Store
}

// libraryStore returns the shared store when one was injected, otherwise
// a fresh store over the configured index and path.
func (c Config) libraryStore() *library.Store {
	if c.LibraryStore != nil {
		return c.LibraryStore
	}
	return library.NewStore(c.Library, c.LibraryPath)
}

// kind labels a selector entry. Bare names (no prefix) are kindTool for
// back-compat with --enable color.convert; everything else uses kind:name.
type kind string

const (
	kindTool     kind = "tools"
	kindResource kind = "resources"
	kindPrompt   kind = "prompts"
)

// selectorSet groups parsed selector entries by kind. wildcards[k] is true
// when the user wrote "k:*"; names[k] holds explicit names for that kind.
// names is keyed by kind so a single helper can ResolveEnabled* without
// caring which kinds exist.
type selectorSet struct {
	names     map[kind]map[string]struct{}
	wildcards map[kind]bool
}

func (s selectorSet) hasAny(k kind) bool {
	if s.wildcards[k] {
		return true
	}
	return len(s.names[k]) > 0
}

func (s selectorSet) matches(k kind, name string) bool {
	if s.wildcards[k] {
		return true
	}
	_, ok := s.names[k][name]
	return ok
}

// parseSelectors splits the namespaced selector list into per-kind sets.
// Recognised prefixes: "tools:", "resources:", "prompts:". A bare entry
// (no prefix) is treated as a tool name for back-compat with the older
// --enable/--disable syntax that only knew about tools. Entries with an
// unknown prefix produce an error so typos in --enable foo:bar fail loudly.
//
// Resource URIs (e.g. "huetension://schemas/v1") happen to contain ":";
// the parser only strips a prefix when it matches one of the three known
// kinds, so "resources:huetension://schemas/v1" parses to kind=resources,
// name="huetension://schemas/v1".
func parseSelectors(items []string) (selectorSet, error) {
	out := selectorSet{
		names:     map[kind]map[string]struct{}{},
		wildcards: map[kind]bool{},
	}
	for _, raw := range items {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		k, name := splitSelector(raw)
		if k == "" {
			return selectorSet{}, fmt.Errorf("mcp: unknown selector kind in %q (want tools:, resources:, prompts:, or a bare tool name)", raw)
		}
		if name == "*" {
			out.wildcards[k] = true
			continue
		}
		if name == "" {
			return selectorSet{}, fmt.Errorf("mcp: empty selector name in %q", raw)
		}
		if out.names[k] == nil {
			out.names[k] = map[string]struct{}{}
		}
		out.names[k][name] = struct{}{}
	}
	return out, nil
}

func splitSelector(s string) (kind, string) {
	for _, prefix := range []kind{kindTool, kindResource, kindPrompt} {
		if rest, ok := strings.CutPrefix(s, string(prefix)+":"); ok {
			return prefix, rest
		}
	}
	if strings.Contains(s, ":") {
		// Has a colon but no recognised kind prefix — only legitimate when
		// it would have been a kind we know. Bare-name back-compat does not
		// allow colons (tool names use "."), so reject loudly.
		return "", ""
	}
	return kindTool, s
}

// validateNames asserts every explicit name in s for kind k exists in
// known. Wildcards bypass validation. Used by ResolveEnabled* to catch
// typos like --enable resources:no-such-uri.
func validateNames(s selectorSet, k kind, known map[string]struct{}, label string) error {
	for name := range s.names[k] {
		if _, ok := known[name]; !ok {
			return fmt.Errorf("mcp: unknown %s in --%s: %q", k, label, name)
		}
	}
	return nil
}

// ResolveEnabled returns the tool descriptors that pass the enable/disable
// filters in cfg. Order matches the catalogue order in All(). Returns an
// error if cfg references an unknown tool — typos in --enable / --disable
// should fail loudly rather than silently disable everything.
func ResolveEnabled(cfg Config) ([]Descriptor, error) {
	enable, disable, err := parseSelectorsBoth(cfg)
	if err != nil {
		return nil, err
	}
	known := descriptorNames(allDescriptors)
	if err := validateNames(enable, kindTool, known, "enable"); err != nil {
		return nil, err
	}
	if err := validateNames(disable, kindTool, known, "disable"); err != nil {
		return nil, err
	}

	out := make([]Descriptor, 0, len(allDescriptors))
	for _, d := range allDescriptors {
		if enable.hasAny(kindTool) {
			if !enable.matches(kindTool, d.Name) {
				continue
			}
		} else if !d.DefaultEnabled {
			continue
		}
		if disable.matches(kindTool, d.Name) {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

// parseSelectorsBoth parses Enable and Disable in one go and returns both
// sets. Centralised so ResolveEnabled / ResolveEnabledResources /
// ResolveEnabledPrompts share the same validation pipeline.
func parseSelectorsBoth(cfg Config) (selectorSet, selectorSet, error) {
	enable, err := parseSelectors(cfg.Enable)
	if err != nil {
		return selectorSet{}, selectorSet{}, err
	}
	disable, err := parseSelectors(cfg.Disable)
	if err != nil {
		return selectorSet{}, selectorSet{}, err
	}
	return enable, disable, nil
}

func descriptorNames(in []Descriptor) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for _, d := range in {
		out[d.Name] = struct{}{}
	}
	return out
}
