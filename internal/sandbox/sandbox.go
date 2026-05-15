// Package sandbox holds the image-loading security policy shared by every
// huetension network transport (mcp, web). The split lets transports
// declare their security posture (--read-only, --root, --allow-host,
// --max-image-bytes, --block-private-networks) once and apply it via the
// same ImageSandbox value — instead of each transport carrying its own
// drift-prone copy of the validation rules.
package sandbox

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ImageSandbox enforces the security boundary around image-loading code
// paths. Zero value is the most permissive: filesystem access ENABLED at
// the caller's CWD, all hosts allowed, default size limit applied by
// imageio.
//
// Callers running over network transports should flip ReadOnly on and
// pin Root, AllowHosts, MaxImageBytes — preventing an LLM-driven or
// browser-facing server from being asked to read arbitrary files or
// SSRF internal targets.
type ImageSandbox struct {
	// ReadOnly, when true, rejects any input that requires filesystem
	// access. URL / data-URI / inline-bytes paths remain available.
	ReadOnly bool

	// Root is the directory below which file-path inputs must resolve.
	// Empty string disables filesystem access entirely (any path is
	// rejected, same as ReadOnly).
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
	// (169.254.0.0/16) and multicast addresses. Closes the SSRF gap that
	// would otherwise let a remote caller use the server as a probe of
	// the host's internal network.
	BlockPrivateNetworks bool
}

// CheckPath validates that path is allowed under this sandbox. Returns
// the validated, fully resolved (symlinks dereferenced) absolute path
// so callers can pass it to the loader without re-resolving.
//
// Rules:
//   - ReadOnly=true → reject.
//   - Root="" → reject (filesystem access disabled).
//   - Otherwise: resolve symlinks on both Root and path, then require
//     the resolved path to live inside the resolved Root (no `..`
//     escape).
//
// Resolving symlinks on the input is the load-bearing step — without
// it, a symlink inside Root pointing outside Root would slip past a
// purely lexical check. EvalSymlinks fails when the target does not
// exist; that's acceptable because we need to open the file anyway.
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
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes root %q", path, rootResolved)
	}
	return absResolved, nil
}
