# CLI reference

The CLI is the most direct way to talk to huetension. Run `huetension <cmd> --help` for the canonical, always-up-to-date help — this page is a guided tour.

## Global flags

These work on every subcommand:

| Flag | Default | Notes |
| --- | --- | --- |
| `--data-dir DIR` | `~/.config/huetension` | Holds `config.yaml` and `library.json`. Honours `HUETENSION_DATA_DIR`. Created on first run. |
| `--no-color` | off | Disable ANSI color in text output. Honours `NO_COLOR`. |
| `--quiet`, `-q` | off | Suppress the header line in text mode. JSON output unaffected. |

Configuration precedence: explicit flag > `HUETENSION_*` env > `config.yaml` > built-in default.

## Output flags (palette-producing commands)

| Flag | Default | Notes |
| --- | --- | --- |
| `--format`, `-f` | `text` (palette cmds), `json` (others) | `text\|json\|css\|scss\|less\|tailwind\|txt\|gpl\|ggr\|svg\|png\|jpeg\|ase\|aco` |
| `--output`, `-o FILE` | `-` (stdout) | When `--format` is unset, the file extension picks the format (e.g. `-o swatch.png`). |
| `--pretty` | off | Pretty-print JSON. |
| `--prefix` | `color` | CSS/SCSS/Tailwind variable prefix. |
| `--name` | `huetension` | Palette name in JSON / GPL exports. |
| `--swatch-size WxH` | `96x96` | PNG/JPEG only. |

## Exit codes

- `0` — success.
- `1` — total failure (could not run, fatal error, single-input failure).
- `2` — partial failure (some inputs processed, others failed).

## Commands

### `extract <source>` — pull a palette from an image

```sh
huetension extract photo.jpg -k 6
huetension extract https://example.com/photo.jpg --method softk
huetension extract photo.jpg -o palette.png --swatch-size 120x80
```

`<source>` is a file path, an `http`/`https` URL, a `data:` URI, or `-` for stdin.

Key flags: `--method`, `--size`/`-k`, `--resize`/`-r`, `--soft-preset`, `--merge-epsilon`, `--sort`, `--reverse`, `--timeout`, `--allow-host`, `--max-bytes`.

### `analyze <source>` — image color distribution strips

Renders distribution strips (hue / luminance / saturation / distance-to-primary) as a PNG by default. Use `--strip` to pick a single metric (default `all`) and `--space oklch|hsl` to switch the sort space.

### `harmony <type> <color>` — generate a color harmony

```sh
huetension harmony complementary "#3498db"
huetension harmony triadic "hsl(220, 70%, 50%)" --count 6
```

Types: `complementary`, `analogous`, `triadic`, `split`, `tetradic`, `square`, `double` (alias of `double-complementary`), `compound`, `monochromatic`, `shades`.

### `gradient <color1> <color2> [color3 ...]` — build a gradient

```sh
huetension gradient "#ff0066" "#00bcd4" --steps 9
huetension gradient "#ff0066" "#fffadc" "#00bcd4" --steps 12 --space oklab --easing ease-in-out
```

Stops are positional. With 3+ stops they are spaced evenly across the gradient. PNG/JPEG outputs render a smooth raster strip instead of the discrete `--steps` palette.

### `contrast <foreground> <background>` — WCAG / APCA contrast check

```sh
huetension contrast "#222" "#fff" --algo wcag21
huetension contrast "#222" "#fff" --algo apca
```

`--algo` is `wcag21` (default) or `apca`. The MCP `contrast.check` tool additionally accepts `both`; the CLI is one-algo-per-call.

### `blindness` — simulate color-vision deficiency

```sh
echo "#3498db" | huetension blindness --kind deutan
huetension blindness --kind all "#3498db" "#e74c3c"
```

Kinds: `protan` (protanopia), `deutan` (deuteranopia), `tritan` (tritanopia), `achroma` (achromatopsia), `all`.

### `convert` / `sort` — color list utilities

Read colors from stdin (one per line) when no positional args are given.

```sh
echo -e "#ff0066\n#00bcd4" | huetension convert --to hsl
echo -e "#222\n#fff\n#0af" | huetension sort --by luminance
```

### `random` — generate a random palette

```sh
huetension random --count 5
huetension random --count 6 --harmony triadic --seed 42
```

### `css` / `tailwind` — export-only convenience commands

```sh
echo -e "#222\n#fff" | huetension css --kind scss --name brand
echo -e "#222\n#fff" | huetension tailwind --shades 11
```

### `lut` — palette-driven 3D LUT

Generates a `.cube` LUT from a palette (or applies one to an image). Two grading methods: `rbf` (smooth, default) and `knn` (legacy layered).

### `library` / `library add` — manage the palette catalogue

Stored in `library.json` inside the data directory.

```sh
huetension library add --name "Brand minimal" "#111827" "#F9FAFB" "#2563EB"
```

### `mcp` / `web` / `serve` — long-running servers

See [`mcp.md`](mcp.md) and [`config.md`](config.md). Quick examples:

```sh
huetension mcp --transport stdio                           # Claude Desktop, editors
huetension mcp --transport http --address 0.0.0.0:7337 --auth-token "$TOKEN"
huetension web                                             # SPA at http://127.0.0.1:8080
huetension serve --address 0.0.0.0:8080 --auth-token "$TOKEN"  # SPA + REST + MCP on one port
```

### `version`

Prints the binary's version (the value injected via `-X main.version=…` at release build time; `dev` on a from-source `go install`).

### `completion`

Generates shell completion scripts for bash, zsh, fish, and PowerShell. Pipe to your shell's completion directory.
