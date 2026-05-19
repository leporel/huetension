package library

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

//go:embed data/defaults.json
var defaultsFS embed.FS

const defaultsPath = "data/defaults.json"

// Load returns the curated palette catalogue. externalPath, when set
// and pointing at a readable file, replaces the embedded defaults
// entirely — that file is the source of truth (the future "add
// palette" UI writes defaults+user adds together into one file). When
// externalPath is empty, or names a file that does not exist, the
// embedded defaults are used.
//
// Failure modes:
//   - externalPath set, file present, parse/validate fails -> hard error
//     (silent fallback would mask user corruption).
//   - externalPath set, file exists but unreadable (perms, IO) -> error.
//   - externalPath set, file does not exist -> embedded defaults (the
//     common case before any user customisation).
//   - embedded defaults fail to parse -> error (compile-time bug).
func Load(externalPath string) (*Index, error) {
	if externalPath != "" {
		info, err := os.Stat(externalPath)
		switch {
		case err == nil:
			if info.IsDir() {
				return nil, fmt.Errorf("library: external path %q is a directory", externalPath)
			}
			data, rerr := os.ReadFile(externalPath)
			if rerr != nil {
				return nil, fmt.Errorf("library: read %s: %w", externalPath, rerr)
			}
			idx, perr := parseAndValidate(data)
			if perr != nil {
				return nil, fmt.Errorf("library: %s: %w", externalPath, perr)
			}
			return idx, nil
		case errors.Is(err, fs.ErrNotExist):
			// Fall through to embedded defaults.
		default:
			return nil, fmt.Errorf("library: stat %s: %w", externalPath, err)
		}
	}

	data, err := defaultsFS.ReadFile(defaultsPath)
	if err != nil {
		return nil, fmt.Errorf("library: read embedded %s: %w", defaultsPath, err)
	}
	idx, err := parseAndValidate(data)
	if err != nil {
		return nil, fmt.Errorf("library: embedded defaults: %w", err)
	}
	return idx, nil
}

// MustLoadDefaults loads the embedded defaults and panics on error.
// Useful for tests and as a sanity check at process start. Production
// code should call Load and surface the error to the operator.
func MustLoadDefaults() *Index {
	idx, err := Load("")
	if err != nil {
		panic(err)
	}
	return idx
}

// parseAndValidate decodes the JSON file shape and runs validateAndIndex.
// Rejects unknown top-level fields so a typo in a hand-edited library
// JSON ("paletes" instead of "palettes") surfaces as a parse error.
func parseAndValidate(data []byte) (*Index, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var f File
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	// Reject trailing content past the first JSON value — silent-accept
	// would let "{...} not json" pass with a truncated payload.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("decode: unexpected trailing content")
		}
		return nil, fmt.Errorf("decode: trailing content: %w", err)
	}
	// Forward-compat lever: if the file declares a schema string, it
	// must match the version this binary speaks. Empty / missing is
	// fine (older files predate the field). A future v2 binary will
	// know to negotiate; today's binary refuses rather than misparse.
	if f.Schema != "" && f.Schema != schemaVersion {
		return nil, fmt.Errorf("decode: unsupported schema %q (this binary speaks %q)", f.Schema, schemaVersion)
	}
	return validateAndIndex(f)
}
