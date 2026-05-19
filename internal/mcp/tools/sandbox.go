package tools

import (
	"log/slog"

	"github.com/leporel/huetension/internal/palette/library"
	"github.com/leporel/huetension/internal/sandbox"
)

// Deps carries cross-cutting configuration the parent mcp package threads
// into tool registrations. Most tools ignore it; image.* tools consume
// ImageSandbox to gate filesystem and network access; library.* tools
// consume Library to read the catalogue and (library.save) grow it.
// Logger, when non-nil, can be used by tools to emit handler-internal
// events; nil is a valid value (tools should fall back to a no-op rather
// than panicking).
type Deps struct {
	ImageSandbox ImageSandbox
	Library      *library.Store
	Logger       *slog.Logger
}

// ImageSandbox is the shared security policy. Aliased here so existing
// callers (and `tools.ImageSandbox{...}` literals in tests) keep working
// after the type moved to internal/sandbox to be shared between MCP and
// the Web REST API.
type ImageSandbox = sandbox.ImageSandbox
