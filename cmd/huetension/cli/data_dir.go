package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/leporel/huetension/internal/palette/library"
)

// dataDirEnvVar is the env-var override for the data directory.
// Same precedence model as flags: explicit setting beats auto-discovery.
const dataDirEnvVar = "HUETENSION_DATA_DIR"

// configFilename and libraryFilename are the names looked up inside the
// resolved data directory. The pair is fixed — operators carry one
// folder, the binary knows what to find inside it.
const (
	configFilename  = "config.yaml"
	libraryFilename = "library.json"
)

var (
	defaultUserDataDirFunc = defaultUserDataDir
	mkdirAllFunc           = os.MkdirAll
)

// resolveDataDir picks the data directory holding huetension's config
// and library JSON. Precedence:
//
//  1. flag (--data-dir)
//  2. HUETENSION_DATA_DIR env
//  3. user data dir if that directory exists
//  4. user data dir, created on demand
//
// Auto-discovery prefers the user config directory once the user has
// created it, even when it is still empty. If that directory is absent,
// the user data directory is created and used. Explicit settings (1, 2)
// require the directory to exist — a typoed --data-dir is reported as an
// error rather than silently falling back to the cascade.
func resolveDataDir(flag string) (string, error) {
	if v := strings.TrimSpace(flag); v != "" {
		if err := mustBeDir(v, "--data-dir"); err != nil {
			return "", err
		}
		return v, nil
	}
	if v := strings.TrimSpace(os.Getenv(dataDirEnvVar)); v != "" {
		if err := mustBeDir(v, dataDirEnvVar); err != nil {
			return "", err
		}
		return v, nil
	}

	userDir := defaultUserDataDirFunc()
	if userDir != "" {
		if info, err := os.Stat(userDir); err == nil && info.IsDir() {
			return userDir, nil
		}
	}
	if userDir != "" {
		if err := mkdirAllFunc(userDir, 0o700); err != nil {
			return "", fmt.Errorf("create user data dir %q: %w", userDir, err)
		}
		return userDir, nil
	}
	return "", nil
}

func defaultUserDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "huetension")
}

// mustBeDir validates an explicit path. Returns a labelled error so
// operators know which knob they typoed (--data-dir vs the env var).
func mustBeDir(p, label string) error {
	info, err := os.Stat(p)
	if err != nil {
		return fmt.Errorf("%s %q: %w", label, p, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s %q: not a directory", label, p)
	}
	return nil
}

// loadLibrary reads the library JSON from the resolved data dir.
// Missing file → embedded defaults (the common case before any user
// customisation). Present file → must parse and validate, hard-fail
// otherwise — silent fallback would mask user corruption.
//
// The second return is the on-disk library.json path a "save palette"
// action should write to. It is the would-be location even when the
// file does not exist yet (the first save bootstraps it); empty only
// when no data directory could be resolved, which disables saving.
func loadLibrary(dataDirFlag string) (*library.Index, string, error) {
	dir, err := resolveDataDir(dataDirFlag)
	if err != nil {
		return nil, "", err
	}
	var savePath, loadPath string
	if dir != "" {
		savePath = filepath.Join(dir, libraryFilename)
		if info, statErr := os.Stat(savePath); statErr == nil && !info.IsDir() {
			loadPath = savePath
		}
	}
	idx, err := library.Load(loadPath)
	if err != nil {
		if loadPath == "" {
			return nil, "", fmt.Errorf("library: failed to load embedded defaults: %w", err)
		}
		return nil, "", err
	}
	return idx, savePath, nil
}
