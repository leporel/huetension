# huetension

Color palette toolkit — a pure-Go core with three thin frontends: CLI, MCP, Web.

## Layout

- All logic lives in `internal/*` packages (`color`, `palette`, `extract`,
  `harmony`, `gradient`, `contrast`, `blindness`, `exporter`, `imageio`,
  `cliutil`); the CLI / MCP / web frontends stay thin.
- `./huetension.go` is the public facade — re-exports the canonical types
  (`import "github.com/leporel/huetension"`); internal packages stay private.
- The CLI touches MCP / web / serve only through their `Config` + `Run` /
  `Handler` seams in `cmd/huetension/cli/{mcp,web,serve}.go`; it never
  builds on their internal types.

## Wire contract

All JSON output uses the fixed `huetension/v1` envelope:
`{schema, tool?, params?, result: {palette, metadata}}`. Emit it through the
library helpers (`exporter.Export(p, FormatJSON, opts)`) — never invent
per-command shapes. Bumping `schema` is a coordination event across all three
frontends.

## Determinism

Extract methods must be reproducible across runs and processes:

- Sort any map-derived output by a stable key — Go map order is random.
- Seed the PRNG from the input pixels; never use `math/rand` directly in an
  extract path.

## CLI conventions

- Default `--format`: `text` (ANSI swatches) for palette commands, `json` for
  machine-oriented ones.
- `-o FILE` — when `--format` is unset, the file extension picks the format.
- `convert` / `sort` / `css` / `tailwind` / `blindness` read colors from stdin
  (one per line) when given no positional args.
- Exit codes: `0` ok, `1` total failure, `2` partial failure.
- Globals: `--no-color` (honours `NO_COLOR`), `--quiet`/`-q`, `--data-dir DIR`
  (config + palette library; honours `HUETENSION_DATA_DIR`).

## Tests & dependencies

- `go test ./... -short` — skips the slow `TestExtractWritesPaletteSidecar`
  (regenerates extract testdata previews). Fixtures: `img1.png` (PNG, not JPG),
  `img2.jpg`, `img3.jpg`.
- Terminal styling: `charm.land/lipgloss/v2`. Image decoders: stdlib +
  `golang.org/x/image`, blank-imported in `internal/imageio/imageio.go`.
