/**
 * Wire-contract types for the huetension/v1 JSON envelope.
 *
 * Mirror of `internal/exporter/json.go` (Go is the canonical shape).
 * Update both files together — the cross-transport parity test only
 * checks `colors[]` byte-equality, so a divergence here surfaces as
 * undefined fields in feature code, not in tests.
 */

export const SCHEMA_VERSION = 'huetension/v1' as const;

/** Per-color source coordinate from an extracted palette (S2). */
export interface Source {
  x: number;
  y: number;
}

/**
 * Single palette entry on the wire. Mirror of `exporter.ColorJSON`.
 * `omitempty` Go fields are optional here.
 *
 * - `rgb`: 0..255 channels (no alpha — opaque colors only)
 * - `rgba`: present only when alpha != 255
 * - `hsl`: [h°, s%, l%] (0..360, 0..100, 0..100)
 * - `oklch`: [L%, C%, h°] (0..100, 0..100, 0..360)
 * - `freq`: 0..1 fraction, present only for extracted palettes
 * - `source`: pixel-space origin (0..1, normalised), present only for extracted palettes
 */
export interface ColorJSON {
  hex: string;
  rgb: [number, number, number];
  rgba?: [number, number, number, number];
  hsl: [number, number, number];
  oklch: [number, number, number];
  freq?: number;
  source?: Source;
}

export interface ImageInfo {
  original_size: [number, number];
  processed_size: [number, number];
  format?: string;
  has_alpha: boolean;
}

export interface PaletteStats {
  total_pixels: number;
  valid_pixels: number;
  duration_ms: number;
}

export interface PaletteMetadata {
  source?: string;
  method?: string;
  params?: Record<string, unknown>;
  image_info?: ImageInfo;
  stats?: PaletteStats;
  generated_at?: string;
}

export interface PaletteJSON {
  size: number;
  name?: string;
  colors: ColorJSON[];
}

/** Generic envelope for huetension/v1 responses. */
export interface Envelope<R> {
  schema: typeof SCHEMA_VERSION;
  tool?: string;
  params?: Record<string, unknown>;
  result: R;
}

/** Palette-shaped envelope (extract, harmony, sort, random, gradient). */
export type PaletteEnvelope = Envelope<{
  palette: PaletteJSON;
  metadata?: PaletteMetadata;
}>;

/** `color.convert` result shape — see internal/web/handlers.go::convertResult. */
export interface ConvertResult {
  input: string;
  hex: string;
  formats?: Record<string, string>;
  value?: string;
}

/** `contrast.check` result — mirror of internal/contrast/contrast.go::WCAG21Result. */
export interface WCAG21Result {
  algo: string;
  ratio: number;
  aa: boolean;
  aa_large: boolean;
  aaa: boolean;
  aaa_large: boolean;
}

/** Mirror of internal/contrast/contrast.go::APCAResult. */
export interface APCAResult {
  algo: string;
  lc: number;
  abs_lc: number;
  body_text: boolean;
  content: boolean;
  large_heading: boolean;
  icon: boolean;
}

export interface ContrastResult {
  wcag21?: WCAG21Result;
  apca?: APCAResult;
}

/** `blindness.simulate` result — internal/web/handlers.go::blindnessResult. */
export interface BlindnessVariant {
  kind: string;
  colors: ColorJSON[];
}

export interface BlindnessResult {
  variants: BlindnessVariant[];
}

/** `export.css` / `export.tailwind` result — handlers.go::exportResult. */
export interface ExportResult {
  format: string;
  kind?: string;
  content: string;
  filename: string;
}

/** Library palette — mirror of internal/mcp/tools/library.go::LibraryPalette. */
export interface LibraryPalette {
  id: string;
  name: string;
  description?: string;
  categories: string[];
  tags?: string[];
  colors: ColorJSON[];
  source?: string;
  license?: string;
}

export interface LibraryCategory {
  slug: string;
  name: string;
  count: number;
}

/** GET /api/v1/library — full catalogue. */
export type LibraryIndexEnvelope = Envelope<{
  categories: LibraryCategory[];
  palettes: LibraryPalette[];
}>;

/** GET /api/v1/library/{category}. */
export type LibraryByCategoryEnvelope = Envelope<{
  category: LibraryCategory;
  palettes: LibraryPalette[];
}>;

/** GET /api/v1/library/palette/{id}. */
export type LibraryPaletteEnvelope = Envelope<{ palette: LibraryPalette }>;
