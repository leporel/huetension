# Configuration reference

huetension reads `config.yaml` from its data directory. All keys are optional — anything unset falls back to a built-in default.

## Data directory

Resolved in this order:

1. `--data-dir DIR` flag.
2. `HUETENSION_DATA_DIR` env var.
3. Default user config directory:
   - Linux/macOS: `~/.config/huetension/`
   - Windows: `%APPDATA%\huetension\`

The directory is created on first run if it doesn't exist. The same folder holds:

- `config.yaml` — the file documented below.
- `library.json` — your saved palettes (see [Library](#library)).

## Precedence

For any single setting:

```
explicit flag  >  HUETENSION_<KEY>  env  >  config.yaml  >  built-in default
```

Env-var names follow viper's convention: prefix `HUETENSION_`, dots and dashes replaced with underscores. `mcp.enable` is `HUETENSION_MCP_ENABLE`. List values are comma-separated.

## Keys

### `mcp:`

Tool selection for the MCP server. Shared between `huetension mcp` and `huetension serve` — both honour this section.

Semantics:

- `enable` — **whitelist**. When non-empty, **only** listed tools are exposed; every other tool is off (whether or not it's default-enabled). Empty / unset means "expose every default-enabled tool".
- `disable` — **blacklist** applied *after* `enable`. It removes listed tools from whatever survived the whitelist.
- Use one or the other. Combining them is redundant unless you whitelist `tools:*` and then subtract specific tools (see the third example below).
- Bare names target tools (`color.convert`). Use `resources:<uri>` / `prompts:<name>` to select those kinds; `kind:*` is a wildcard.

Examples — blacklist (most common: keep everything except a few):

```yaml
mcp:
  disable:
    - image.extract
    - image.extractBatch
    - library.save
```

Whitelist (curated subset):

```yaml
mcp:
  enable:
    - color.convert
    - color.sort
    - harmony.generate
    - contrast.check
    - library.list
    - library.get
```

Wildcard whitelist + blacklist (equivalent to plain blacklist today, since every shipped tool is default-enabled — kept available for future tools that ship default-off):

```yaml
mcp:
  enable:
    - tools:*
  disable:
    - image.extract
    - image.extractBatch
```

A flag on the command line overrides the file. `huetension mcp --enable harmony.generate --list-tools` will surface `harmony.generate` even when `config.yaml` whitelists something else.

## Library

`library.json` lives next to `config.yaml`. Schema (excerpt):

```json
{
  "version": 1,
  "palettes": [
    {
      "id": "brand-minimal",
      "name": "Brand minimal",
      "source": "user",
      "categories": ["Saved"],
      "tags": ["brand"],
      "colors": [
        { "hex": "#111827", "name": "Ink" },
        { "hex": "#F9FAFB", "name": "Paper" },
        { "hex": "#2563EB", "name": "Accent" }
      ]
    }
  ]
}
```

The file is appended to by:

- `huetension library add --name "..." <colors...>`
- POST `/api/v1/library` (web)
- `library.save` (MCP)

Saved palettes are always filed under the `Saved` category alongside the bundled defaults. The id is derived from `name` and collision-avoided automatically.

## Environment variables (cheatsheet)

| Var | Effect |
| --- | --- |
| `HUETENSION_DATA_DIR` | Override the data directory. |
| `HUETENSION_MCP_ENABLE` | Comma-separated tool whitelist. |
| `HUETENSION_MCP_DISABLE` | Comma-separated tool blacklist. |
| `NO_COLOR` | Disable ANSI color in text output (any non-empty value). |

## Example config

A minimal, copy-pasteable starting point lives in [`huetension.example.yaml`](../huetension.example.yaml) at the repo root.
