# MCP server

huetension speaks the [Model Context Protocol](https://modelcontextprotocol.io) via three transports: **stdio** (child-process; for Claude Desktop, editors), **http** (modern streamable HTTP, recommended for remote use), and **sse** (legacy server-sent events).

Same JSON wire contract as the CLI's `--format json` mode and the REST API, so any consumer can read all three.

## Tool catalogue

15 tools, all default-enabled:

| Tool | Description |
| --- | --- |
| `color.convert` | Convert between hex / rgb / hsl / hsv / lab / lch / oklab / oklch / CSS named / integer. |
| `color.sort` | Sort by luminance, lightness, OkLab L, hue, saturation, or frequency. |
| `harmony.generate` | Complementary / analogous / triadic / split / tetradic / square / double-complementary / compound / monochromatic / shades. |
| `gradient.generate` | Two-endpoint or N-stop gradients in OkLab/OkLCH/Lab/RGB/HSL with optional easing. |
| `contrast.check` | WCAG 2.1, APCA, or both. |
| `blindness.simulate` | Protanopia / deuteranopia / tritanopia / achromatopsia. |
| `palette.random` | Random palette, optionally driven by a harmony rule; deterministic when seeded. |
| `export.css` | CSS custom properties / SCSS / LESS variables. |
| `export.tailwind` | Tailwind `theme.extend.colors` snippet (flat or shade scale). |
| `image.extract` | Extract a palette from a file path, http(s) URL, or base64. Gated by the sandbox. |
| `image.extractBatch` | Parallel batch over multiple sources. |
| `library.categories` | List catalogue categories with palette counts. |
| `library.list` | List palettes filtered by category / tag. |
| `library.get` | Fetch a palette by id. |
| `library.save` | Save a palette to `library.json`. Disabled on read-only servers or when no data dir is resolved. |

List the running server's actual catalogue at any time:

```sh
huetension mcp --list-tools                 # text
huetension mcp --list-tools --list-format json
```

## Selecting a subset of tools

Two equivalent ways: command-line flags or the `mcp:` section in `config.yaml`. Flags beat the file.

```sh
huetension mcp --enable color.convert,harmony.generate
huetension mcp --disable image.extract,image.extractBatch,library.save
```

Semantics:

- `enable` is a **whitelist**. When non-empty, *only* listed tools are exposed; everything else is off (whether or not it's default-enabled).
- `disable` is a **blacklist** applied *after* `enable`. It removes listed tools from whatever survived the whitelist.
- Use one or the other; combining them is redundant unless you whitelist `tools:*` and then subtract specific tools.

`config.yaml`:

```yaml
# Blacklist — start from the default-enabled set, remove specific tools.
mcp:
  disable:
    - image.extract
    - image.extractBatch
    - library.save
```

```yaml
# Whitelist — expose ONLY these tools; everything else is off.
mcp:
  enable:
    - color.convert
    - harmony.generate
    - contrast.check
    - library.list
    - library.get
```

```yaml
# Wildcard whitelist + blacklist — expose every tool except image.*.
mcp:
  enable:
    - tools:*
  disable:
    - image.extract
    - image.extractBatch
```

The same `mcp:` section feeds `huetension serve` (which has no `--enable`/`--disable` flag of its own).

## Transports

### stdio — Claude Desktop / editors

```json
{
  "mcpServers": {
    "huetension": {
      "command": "huetension",
      "args": ["mcp", "--transport", "stdio"]
    }
  }
}
```

stdio servers must keep stdout for JSON-RPC — huetension logs always go to stderr.

### HTTP / SSE — remote use

```sh
huetension mcp --transport http --address 127.0.0.1:7337
huetension mcp --transport http,sse --address 0.0.0.0:7337 --auth-token "$TOKEN"
```

Routes (under `--base-path`, default `/mcp`):

- `POST /mcp` — streamable HTTP transport.
- `GET  /mcp/sse` — legacy SSE transport.

Non-loopback binds **require** `--auth-token`; the server refuses to start without one. They also auto-enable `--read-only` and `--block-private-networks` (override with `--read-only=false` + an explicit `--root`).

Pass `--transport all` to expose stdio, http, and sse from one process.

## Image sandbox

`image.extract` / `image.extractBatch` are gated by these flags (shared with `huetension web` and `huetension serve`):

| Flag | Default | Notes |
| --- | --- | --- |
| `--read-only` | off (loopback), **on** (non-loopback) | Reject filesystem path inputs; URLs and data URIs still work. |
| `--root DIR` | `.` | Filesystem root. Empty disables path inputs. |
| `--allow-host HOST` | empty (any) | Host allowlist for URL fetches; `*.example.com` allowed. Repeatable. |
| `--max-image-bytes N` | 64 MiB | Cap on URL body / decoded base64 / upload. |
| `--block-private-networks` | off (loopback), **on** (non-loopback) | Refuse outbound TCP to loopback / private / link-local. |

Refusing `--read-only=false` on a network transport without an explicit `--root` is intentional: the default `.` is too broad to be a sandbox.

## Running with Docker

The release image is `ghcr.io/leporel/huetension`. Three common shapes:

### Web UI + REST API

```sh
docker run --rm -p 8080:8080 \
  ghcr.io/leporel/huetension:latest \
  web --address 0.0.0.0:8080
```

Open <http://127.0.0.1:8080>. For non-loopback binds add `--auth-token "$TOKEN"`.

### MCP (HTTP)

```sh
docker run --rm -p 7337:7337 \
  -v "$HOME/.config/huetension:/data:ro" \
  ghcr.io/leporel/huetension:latest \
  mcp --transport http --address 0.0.0.0:7337 \
      --auth-token "$TOKEN" \
      --data-dir /data
```

For stdio MCP from a client config (Claude Desktop and similar) the container itself becomes the command:

```json
{
  "mcpServers": {
    "huetension": {
      "command": "docker",
      "args": [
        "run", "--rm", "-i",
        "-v", "${HOME}/.config/huetension:/data:ro",
        "ghcr.io/leporel/huetension:latest",
        "mcp", "--transport", "stdio", "--data-dir", "/data"
      ]
    }
  }
}
```

`-i` is required (stdin needs to stay attached for JSON-RPC).

### Web + MCP on the same port (`serve`)

```sh
docker run --rm -p 8080:8080 \
  -v "$HOME/.config/huetension:/data:ro" \
  ghcr.io/leporel/huetension:latest \
  serve --address 0.0.0.0:8080 \
        --auth-token "$TOKEN" \
        --data-dir /data
```

Routes on the single listener:

- `GET  /` — SPA.
- `POST /api/v1/…` — REST.
- `POST /mcp` — MCP HTTP (add `--mcp-transport http,sse` to expose SSE at `/mcp/sse`).

The MCP and API path prefixes must not overlap. To expose only a subset of MCP tools, set `mcp.enable` / `mcp.disable` in `config.yaml` (see [`config.md`](config.md)) — `serve` has no `--enable`/`--disable` flag.

## Security defaults summary

| Bind | Auth | `--read-only` | `--block-private-networks` |
| --- | --- | --- | --- |
| Loopback (`127.0.0.1`, `::1`, `localhost`) | optional | default off | default off |
| Any other address | **required** | default **on** | default **on** |

The CLI fails fast when these constraints are violated rather than starting an insecure server.
