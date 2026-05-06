## tests

To run tests use `go test ./... -short` to avoid the slow `TestExtractWritesPaletteSidecar` (which regenerates every preview JPG under `internal/extract/testdata/`). Test fixtures: `img1.png`, `img2.jpg`, `img3.jpg` — note `img1` is PNG, not JPG.


## architecture

- All features live in pure-Go internal packages (`internal/color`, `internal/palette`, `internal/extract`, `internal/harmony`, `internal/gradient`, `internal/contrast`, `internal/blindness`, `internal/exporter`, `internal/imageio`, `internal/cliutil`). The CLI is one of three planned thin frontends (CLI / MCP / Web).
- `./huetension.go` is the public library facade — re-exports the canonical types/functions. External Go programs use `import "github.com/leporel/huetension"`. Internal packages stay private.
- Cli must NOT import MCP or web UI types. `go list -deps ./cmd/huetension` should show neither.

## wire contract

JSON envelope shape is fixed at `huetension/v1`:

```jsonc
{
  "schema": "huetension/v1",
  "tool": "extract",                 // optional; CLI/MCP set, library callers omit
  "params": { ... },                 // optional; mirrors the tool args
  "result": {
    "palette": { "size": N, "name": "...", "colors": [...] },
    "metadata": { "source": "...", "method": "...", "params": {...} }
  }
}
```

Bumping the schema is a coordination event with MCP and web UI — every JSON consumer keys off this string. Don't invent per-command shapes; the library helpers (`exporter.Export(p, FormatJSON, opts)`) emit the canonical envelope.

## determinism

Extract methods MUST be reproducible across runs and processes. Two non-obvious sources of non-determinism worth guarding:

1. Go map iteration order — `internal/extract/binning.go` sorts bin output by RGB key. Any new binning code must do the same.
2. Global PRNG state — `internal/extract/lloyd.go` seeds k-means++ from input pixels (`seedRNGFromPixels`). Don't reach for `math/rand` directly in any extract path.

`TestExtractWritesPaletteSidecar` regeneration is one way to catch regressions: same image + same options → byte-identical sidecar JPG.

## CLI conventions

- Default `--format` for palette-producing commands is `text` (ANSI swatches via lipgloss). Machine-oriented flows pass `--format json`.
- `-o FILE` extension inference: if `--format` is unset, the extension on `-o` picks the format (`-o swatch.png` → PNG, `-o brand.css` → CSS).
- Stdin readers: `convert` / `sort` / `css` / `tailwind` / `blindness` read one color per line from stdin when no positional args are given.
- Exit codes follow Pylette: `0` ok, `1` total failure, `2` partial failure. Partial failures propagate via the `errPartialFailure` sentinel in `cmd/huetension/cli/stdin.go`.
- Globals: `--no-color` (also honours `NO_COLOR` env), `--quiet`/`-q` (suppresses text-mode header), `--config FILE` (viper config).

## dependencies

- Use `charm.land/lipgloss/v2` for terminal styling. 
- Image decoders: stdlib (jpeg/png/gif) + `golang.org/x/image` (bmp/tiff/webp) registered via blank imports in `internal/imageio/imageio.go`.
