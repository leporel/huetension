package tools

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// Deps carries cross-cutting configuration the parent mcp package threads
// into tool registrations. Most tools ignore it; image.* tools consume
// ImageSandbox to gate filesystem and network access.
type Deps struct {
	ImageSandbox ImageSandbox
}

// ImageSandbox enforces the security boundary around image.extract* tools.
// Zero value is the most permissive: filesystem access ENABLED at the
// caller's CWD, all hosts allowed, default size limit (64 MiB).
//
// Callers running over network transports (HTTP / SSE in slice E) should
// flip ReadOnly on and pin Root, AllowHosts, MaxImageBytes — preventing an
// LLM-driven server from being asked to read arbitrary files or SSRF
// internal targets.
type ImageSandbox struct {
	// ReadOnly, when true, rejects any input that requires filesystem
	// access (`path` and `data` is allowed; URL is gated by AllowHosts).
	ReadOnly bool

	// Root is the directory below which `path` inputs must resolve. Empty
	// string disables filesystem access entirely (any `path` is rejected).
	Root string

	// AllowHosts, when non-empty, restricts URL fetches to hosts matching
	// one of the listed patterns. "*.example.com" suffix-matches; bare
	// hostnames must match exactly. Empty list = allow all hosts.
	AllowHosts []string

	// MaxImageBytes caps the size of a downloaded URL body or decoded
	// data payload. 0 falls back to imageio's DefaultMaxBytes (64 MiB).
	MaxImageBytes int64

	// BlockPrivateNetworks, when true, refuses outbound TCP connections
	// to loopback (127.0.0.0/8, ::1), private RFC1918, link-local
	// (169.254.0.0/16) and multicast addresses. Closes the SSRF gap
	// flagged in slice D. Stdio servers default to false (local user is
	// trusted); HTTP/SSE servers should set true.
	BlockPrivateNetworks bool
}

// CheckPath validates that path is allowed under this sandbox. Returns an
// error describing the violation; nil on success. The validated, fully
// resolved (symlinks dereferenced) absolute path is returned so callers can
// pass it to the loader without re-resolving.
//
// Rules:
//   - ReadOnly=true → reject.
//   - Root="" → reject (filesystem access disabled).
//   - Otherwise: resolve symlinks on both Root and path, then require the
//     resolved path to live inside the resolved Root (no `..` escape).
//
// Resolving symlinks on the *input* is the load-bearing step — without it,
// a symlink inside Root pointing outside Root would slip past a purely
// lexical check. EvalSymlinks fails when the target file does not exist;
// for image.extract that's fine because we need to open the file anyway.
func (s ImageSandbox) CheckPath(path string) (string, error) {
	if s.ReadOnly {
		return "", errors.New("server is read-only; path inputs are not allowed")
	}
	if strings.TrimSpace(s.Root) == "" {
		return "", errors.New("filesystem access disabled (root not configured)")
	}
	rootAbs, err := filepath.Abs(s.Root)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}
	rootResolved, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", fmt.Errorf("resolve root symlinks: %w", err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	absResolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve path symlinks: %w", err)
	}
	rel, err := filepath.Rel(rootResolved, absResolved)
	if err != nil {
		return "", fmt.Errorf("relativise path: %w", err)
	}
	// `..` at the start (or `..` as the whole rel) means abs is outside root.
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes root %q", path, rootResolved)
	}
	return absResolved, nil
}
