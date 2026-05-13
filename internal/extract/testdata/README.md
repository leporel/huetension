# extract testdata

All previews use `PaletteSize: 6` and `SortBy: palette.SortByOkL` so the
swatch row reads dark → light like a gradient.

## Method × source matrix

| Method | img1.png | img2.jpg | img3.jpg |
|---|---|---|---|
| **source** | ![source](img1.png) | ![source](img2.jpg) | ![source](img3.jpg) |
| `kmeans` (CIE Lab) | ![kmeans](img1_palette_kmeans.jpg) | ![kmeans](img2_palette_kmeans.jpg) | ![kmeans](img3_palette_kmeans.jpg) |
| `okkmeans` (OkLab) | ![okkmeans](img1_palette_okkmeans.jpg) | ![okkmeans](img2_palette_okkmeans.jpg) | ![okkmeans](img3_palette_okkmeans.jpg) |
| `mediancut` | ![mediancut](img1_palette_mediancut.jpg) | ![mediancut](img2_palette_mediancut.jpg) | ![mediancut](img3_palette_mediancut.jpg) |
| `soft` (default) | ![soft](img1_palette_soft.jpg) | ![soft](img2_palette_soft.jpg) | ![soft](img3_palette_soft.jpg) |
| `softk` | ![softk](img1_palette_softk.jpg) | ![softk](img2_palette_softk.jpg) | ![softk](img3_palette_softk.jpg) |
| `octree` | ![octree](img1_palette_octree.jpg) | ![octree](img2_palette_octree.jpg) | ![octree](img3_palette_octree.jpg) |
| `popularity` | ![popularity](img1_palette_popularity.jpg) | ![popularity](img2_palette_popularity.jpg) | ![popularity](img3_palette_popularity.jpg) |
| `wu` | ![wu](img1_palette_wu.jpg) | ![wu](img2_palette_wu.jpg) | ![wu](img3_palette_wu.jpg) |
| `dbscan` | ![dbscan](img1_palette_dbscan.jpg) | ![dbscan](img2_palette_dbscan.jpg) | ![dbscan](img3_palette_dbscan.jpg) |
| `wkmeans` | ![wkmeans](img1_palette_wkmeans.jpg) | ![wkmeans](img2_palette_wkmeans.jpg) | ![wkmeans](img3_palette_wkmeans.jpg) |

## Two flavours, picked by intent

**Frequency-faithful** — palette mirrors how the image is *covered*:

- `kmeans` / `okkmeans` — Lloyd-style clustering in perceptual space.
- `mediancut` — recursive RGB-cube splits along the longest axis.
- `octree` — 8-ary RGB tree, depth 8. Phase 1 collapses lowest-population
  subtrees; Phase 2 finishes with greedy nearest-pair merging in OkLab.
- `popularity` — 5-bit-per-channel histogram, top-N bins.
- `wu` — Xiaolin Wu's variance-minimising quantiser.

**Saliency-aware** — palette surfaces what the eye *notices*, including
small-but-distinctive regions:

- `soft` (default) — Lab over-clustering, ΔE merge with fixed threshold,
  population × saturation ranking. Default Epsilon = 12.0.
- `softk` — same pipeline as `soft` but with dynamic merging: clusters are
  merged greedily until `k × 1.5` remain, guaranteeing that large areas
  are reassembled instead of being fragmented and outranked. Final ranking
  still uses population × saturation.
- `dbscan` — density-based clustering in OkLab. When natural-photo gradient
  chaining merges everything into one cluster (typical), pads with
  saliency-weighted farthest-point sampling so vibrant minor colors win
  against JPEG-noise extremes on the lightness axis.
- `wkmeans` — over-clustered weighted Lloyd in OkLab, 
  ΔE-merge of perceptually-close centroids, then top-K by saliency.
  Output `Freq` still reports honest pixel coverage; saliency only drives
  ranking.

`saliency` here is the proxy `log(count + 1) × (saturationFloor + saturation)^SaliencyPow` — see `internal/extract/saliency.go` for the rationale.
It's not a real visual saliency model; that would require the original 2D pixel grid, which the extract layer deliberately flattens.

## Soft preset × source matrix (mood comparison)

Adobe Kuler-style mood presets for the `soft` / `softk` methods. Each
preset combines a perceptual pre-filter (OkLCH chroma + OkL bounds) with
a ranking tweak (saturation bias / exponent and OkL preference). Numbers
live in `internal/extract/preset.go`; tuning intent is summarised below.

| Soft preset | Intent | Filter (OkL × chroma) | Ranking tweak |
|---|---|---|---|
| `default` | Balanced everyday mood — used implicitly when no preset is set | OkL 0.10–0.92, chroma ≥ 0.02 | exp 1.2, OkL pref 0 |
| `colorful` | Surface diverse, high-chroma hues | OkL 0.15–0.90, chroma ≥ 0.08 | exp 2.0, OkL pref 0 |
| `bright` | Light + vivid; avoid pastels-from-the-shadows | OkL 0.55–0.95, chroma ≥ 0.06 | exp 1.5, OkL pref **+0.5** |
| `muted` | Faded / desaturated; cap chroma low | OkL 0.20–0.85, chroma 0.01–0.10 | bias 1.5, exp 0.4 |
| `deep` | Rich and saturated, not pastel and not pure-dark | OkL 0.20–0.60, chroma ≥ 0.10 | exp 1.8, OkL pref **−0.3** |
| `dark` | Constrained low-OkL palette | OkL 0.05–0.45, chroma ≥ 0.04 | OkL pref **−0.6** |

Omitting `soft_preset` (or passing `""`) is equivalent to passing
`default` — the soft pipeline always runs the perceptual filter and
ranking. The MCP tool surface exposes only `soft_preset`.

If the pre-filter knocks out too many pixels (e.g. `deep` on a flat-grey
image), the pipeline falls back to the unfiltered set and metadata gains
`"preset_effective": false` and `"preset_fallback": "insufficient_pixels"`.

### Method × preset × source

The base `soft` / `softk` rows in the matrix above already render the
`default` preset (it's the implicit baseline); this table covers the
remaining five moods.

| Method × preset | img1.png | img2.jpg | img3.jpg |
|---|---|---|---|
| **source** | ![source](img1.png) | ![source](img2.jpg) | ![source](img3.jpg) |
| `soft - default` | ![soft](img1_palette_soft.jpg) | ![soft](img2_palette_soft.jpg) | ![soft](img3_palette_soft.jpg) |
| `soft - colorful` | ![soft colorful](img1_palette_soft_colorful.jpg) | ![soft colorful](img2_palette_soft_colorful.jpg) | ![soft colorful](img3_palette_soft_colorful.jpg) |
| `soft - bright` | ![soft bright](img1_palette_soft_bright.jpg) | ![soft bright](img2_palette_soft_bright.jpg) | ![soft bright](img3_palette_soft_bright.jpg) |
| `soft - muted` | ![soft muted](img1_palette_soft_muted.jpg) | ![soft muted](img2_palette_soft_muted.jpg) | ![soft muted](img3_palette_soft_muted.jpg) |
| `soft - deep` | ![soft deep](img1_palette_soft_deep.jpg) | ![soft deep](img2_palette_soft_deep.jpg) | ![soft deep](img3_palette_soft_deep.jpg) |
| `soft - dark` | ![soft dark](img1_palette_soft_dark.jpg) | ![soft dark](img2_palette_soft_dark.jpg) | ![soft dark](img3_palette_soft_dark.jpg) |

| Method × preset | img1.png | img2.jpg | img3.jpg |
|---|---|---|---|
| `softk - default` | ![softk](img1_palette_softk.jpg) | ![softk](img2_palette_softk.jpg) | ![softk](img3_palette_softk.jpg) |
| `softk - colorful` | ![softk colorful](img1_palette_softk_colorful.jpg) | ![softk colorful](img2_palette_softk_colorful.jpg) | ![softk colorful](img3_palette_softk_colorful.jpg) |
| `softk - bright` | ![softk bright](img1_palette_softk_bright.jpg) | ![softk bright](img2_palette_softk_bright.jpg) | ![softk bright](img3_palette_softk_bright.jpg) |
| `softk - muted` | ![softk muted](img1_palette_softk_muted.jpg) | ![softk muted](img2_palette_softk_muted.jpg) | ![softk muted](img3_palette_softk_muted.jpg) |
| `softk - deep` | ![softk deep](img1_palette_softk_deep.jpg) | ![softk deep](img2_palette_softk_deep.jpg) | ![softk deep](img3_palette_softk_deep.jpg) |
| `softk - dark` | ![softk dark](img1_palette_softk_dark.jpg) | ![softk dark](img2_palette_softk_dark.jpg) | ![softk dark](img3_palette_softk_dark.jpg) |

## Regenerating

```sh
go test ./internal/extract/ -run TestExtractWritesPaletteSidecar -v
```
