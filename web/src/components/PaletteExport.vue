<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { watchDebounced } from "@vueuse/core";
import { exportPalette, lut } from "../api";
import type { ExportFormat } from "../api/exportPalette";
import type { LutFormat, LutMethod } from "../api/lut";
import { useWorkspaceStore } from "../stores/workspace";
import ImagePickerModal from "./ImagePickerModal.vue";

// Export the workspace palette in any exporter format via POST /export, OR
// a 3D colour-grading LUT via POST /lut. Text formats render into a <pre>;
// binary formats (PNG / JPEG / Adobe / HALD PNG) come back base64-encoded
// and are shown as a thumbnail / download-only note.

// Format tab shown in the top button row. The 'lut' entry pivots the
// panel into LUT mode where an inner format radio picks Cube vs HALD PNG.
type FormatTab = ExportFormat | "lut";

const FORMATS: { value: FormatTab; label: string }[] = [
    { value: "json", label: "JSON" },
    { value: "css", label: "CSS" },
    { value: "scss", label: "SCSS" },
    { value: "less", label: "Less" },
    { value: "tailwind", label: "Tailwind" },
    { value: "txt", label: "Plain" },
    { value: "gpl", label: "GIMP" },
    { value: "ggr", label: "GGR" },
    { value: "svg", label: "SVG" },
    { value: "png", label: "PNG" },
    { value: "jpeg", label: "JPEG" },
    { value: "ase", label: "ASE" },
    { value: "aco", label: "ACO" },
    { value: "lut", label: "LUT" },
];

// LUT cube levels — `level` is a shorthand for the cube edge per
// channel (= level²) and the LUT-texture image side (= level³). The
// dropdown is shared by both LUT sub-tabs (Cube text and LUT texture
// PNG) so the test grade and the export grade always use the same
// cube. Level 16 is omitted from the UI on purpose: its .cube text
// payload is ~480 MB and ffmpeg's lut3d spends seconds on lookups,
// so a "preview" at that level is impractical.
const LUT_LEVELS: { level: number; value: number; label: string }[] = [
    { level: 2, value: 4, label: "Level 2 · cube 4 (very low)" },
    { level: 4, value: 16, label: "Level 4 · cube 16 (low)" },
    { level: 6, value: 36, label: "Level 6 · cube 36 (medium)" },
    { level: 8, value: 64, label: "Level 8 · cube 64 (standard)" },
];

// Defaults for the LUT knobs — mirror the CLI's `lut` command so
// behaviour stays consistent between transports. The Reset button
// re-applies these. Both methods share the shared knobs (lutSize,
// saturation); each method has its own knob set so toggling the
// method doesn't fight with a previous method's slider drag.
const DEFAULTS = {
    method: "grade" as LutMethod,
    // Grade (default) — hue-wheel compression, lightness preserved.
    compression: 0.7,
    mute: 0.3,
    // K-NN (legacy "Layered").
    radius: 0.4,
    distribution: 0.15,
    intensity: 0.9,
    blend: 2,
    // RBF ("Smooth").
    reach: 0.2,
    sharpness: 2.0,
    strength: 0.9,
    // Shared.
    saturation: false,
    lutSize: 64,
} as const;

const workspace = useWorkspaceStore();

const format = ref<FormatTab>("css");
const name = ref("palette");
// Shade-scale count for Tailwind: 0 = flat shape; >0 expands each color.
const shades = ref(0);
// Color notation for text-based exports; empty = default hex.
const colorNotation = ref("");

const NOTATIONS: { value: string; label: string }[] = [
    { value: "", label: "hex" },
    { value: "hex-upper", label: "HEX" },
    { value: "rgb", label: "rgb()" },
    { value: "hsl", label: "hsl()" },
    { value: "hsv", label: "hsv()" },
    { value: "oklab", label: "oklab()" },
    { value: "oklch", label: "oklch()" },
];

// LUT-only state. `lutFormat` toggles the preview (Cube text vs LUT
// texture PNG); every other knob — sliders + cube size — is shared so
// the test grade is identical regardless of which sub-tab is open.
const lutFormat = ref<LutFormat>("cube");
const lutMethod = ref<LutMethod>(DEFAULTS.method);
// K-NN knobs.
const lutRadius = ref(DEFAULTS.radius);
const lutDistribution = ref(DEFAULTS.distribution);
const lutIntensity = ref(DEFAULTS.intensity);
const lutBlend = ref<number>(DEFAULTS.blend);
// RBF knobs.
const lutReach = ref(DEFAULTS.reach);
const lutSharpness = ref(DEFAULTS.sharpness);
const lutStrength = ref(DEFAULTS.strength);
// Grade knobs.
const lutCompression = ref(DEFAULTS.compression);
const lutMute = ref(DEFAULTS.mute);
// Shared.
const lutSaturation = ref(DEFAULTS.saturation);
const lutSize = ref<number>(DEFAULTS.lutSize);

const content = ref("");
const filename = ref("");
// Object URL of the decoded binary output; null for text formats.
const blobUrl = ref<string | null>(null);
const loading = ref(false);
const errorMsg = ref<string | null>(null);
const copied = ref(false);

// Cap for the in-DOM cube preview. A level-8 LUT (cube 64) is 64³ ≈ 262k
// lines (~6 MB of text); parking that in a single <pre> makes every page
// layout pass expensive and lags unrelated interactions like dragging the
// pinned wheel card. Copy/download still emit the untouched full
// `content.value`.
const CUBE_PREVIEW_MAX_LINES = 1024;

const displayedCubeContent = computed<string>(() => {
    const full = content.value;
    if (!full) return "";
    let nl = 0;
    let cut = -1;
    for (let i = 0; i < full.length; i++) {
        if (full.charCodeAt(i) === 10) {
            nl++;
            if (nl === CUBE_PREVIEW_MAX_LINES) {
                cut = i;
                break;
            }
        }
    }
    if (cut === -1) return full;
    let remaining = 0;
    for (let i = cut + 1; i < full.length; i++) {
        if (full.charCodeAt(i) === 10) remaining++;
    }
    return `${full.slice(0, cut)}\n… (${remaining.toLocaleString()} more lines — use Copy or Download for the full cube)`;
});

const isLUT = computed(() => format.value === "lut");
const isLUTTexture = computed(() => isLUT.value && lutFormat.value === "png");
const isBinary = computed(() => blobUrl.value !== null);
// Notation selector visible only for text-based formats.
const showNotation = computed(() =>
    ["css", "scss", "less", "tailwind", "txt"].includes(format.value as string),
);
// Image binaries (png/jpeg, plus LUT texture in LUT mode) get a
// thumbnail preview; opaque binaries (Adobe ase/aco) only offer a download.
const isImage = computed(() => {
    if (!isBinary.value) return false;
    if (format.value === "png" || format.value === "jpeg") return true;
    if (isLUTTexture.value) return true;
    return false;
});
const hasOutput = computed(
    () => content.value !== "" || blobUrl.value !== null,
);

// "Test on your image" state — visible on BOTH Cube and LUT-texture
// sub-tabs (the grade is computed by ffmpeg's `lut3d` filter from a
// .cube text the backend regenerates from `params`, so the sub-tab
// only affects what the user SEES in the preview slot, not what is
// applied). 503 from /apply-lut means ffmpeg is missing — surfaced
// in `applyError`.
const testImage = ref<File | null>(null);
const testImageUrl = ref<string | null>(null);
const gradedUrl = ref<string | null>(null);
const gradedSpectrumUrl = ref<string | null>(null);
const applyLoading = ref(false);
const applyError = ref<string | null>(null);
const pickerOpen = ref(false);
// Reference hue-spectrum strip — generated once on mount via canvas,
// reused on every apply call so the user can see the grade's effect
// across the full hue range alongside their test image. The companion
// `originalSpectrumUrl` is the same blob's object URL, shown under the
// Original frame as a before/after reference for the graded strip.
const spectrumBlob = ref<Blob | null>(null);
const originalSpectrumUrl = ref<string | null>(null);

function setTestImageUrl(blob: Blob | null): void {
    if (testImageUrl.value) URL.revokeObjectURL(testImageUrl.value);
    testImageUrl.value = blob ? URL.createObjectURL(blob) : null;
}
function setGradedUrl(blob: Blob | null): void {
    if (gradedUrl.value) URL.revokeObjectURL(gradedUrl.value);
    gradedUrl.value = blob ? URL.createObjectURL(blob) : null;
}
function setGradedSpectrumUrl(blob: Blob | null): void {
    if (gradedSpectrumUrl.value) URL.revokeObjectURL(gradedSpectrumUrl.value);
    gradedSpectrumUrl.value = blob ? URL.createObjectURL(blob) : null;
}

function onTestImagePicked(file: File): void {
    testImage.value = file;
    setTestImageUrl(file);
    // Discard a stale graded preview when the source changes — the LUT
    // params are still the same but the input image is not.
    setGradedUrl(null);
    setGradedSpectrumUrl(null);
    applyError.value = null;
}

function clearTest(): void {
    testImage.value = null;
    setTestImageUrl(null);
    setGradedUrl(null);
    setGradedSpectrumUrl(null);
    applyError.value = null;
}

/** Build a 512×32 hue rainbow PNG via canvas — used once per session as
 *  the second image in every /apply-lut call so the user can see the
 *  grade's effect across the entire hue range below the test result. */
function makeSpectrumBlob(): Promise<Blob> {
    const W = 512;
    const H = 32;
    const canvas = document.createElement("canvas");
    canvas.width = W;
    canvas.height = H;
    const ctx = canvas.getContext("2d");
    if (!ctx) return Promise.reject(new Error("no canvas context"));
    for (let x = 0; x < W; x++) {
        const hue = (x / W) * 360;
        ctx.fillStyle = `hsl(${hue}, 100%, 50%)`;
        ctx.fillRect(x, 0, 1, H);
    }
    return new Promise<Blob>((resolve, reject) => {
        canvas.toBlob(
            (b) =>
                b
                    ? resolve(b)
                    : reject(new Error("canvas.toBlob returned null")),
            "image/png",
        );
    });
}

/** Current LUT params snapshot — used by both `generate()` and
 *  `applyTest()` so the export-side preview and the apply-side grade
 *  always share state. */
function currentLutParams() {
    const colors = workspace.colors.map((c) => c.hex);
    return {
        colors,
        method: lutMethod.value,
        // Both knob sets ship every request so the backend can pick the
        // active one without the SPA branching at send-time. Unused
        // knobs are ignored by the per-method validator.
        radius: lutRadius.value,
        distribution: lutDistribution.value,
        intensity: lutIntensity.value,
        blend_neighbors: Math.min(lutBlend.value, Math.max(colors.length, 1)),
        reach: lutReach.value,
        sharpness: lutSharpness.value,
        strength: lutStrength.value,
        compression: lutCompression.value,
        mute: lutMute.value,
        include_saturation: lutSaturation.value,
        size: lutSize.value,
    };
}

async function applyTest(): Promise<void> {
    if (!testImage.value) return;
    const params = currentLutParams();
    if (params.colors.length === 0) return;
    applyLoading.value = true;
    applyError.value = null;
    try {
        const images: Array<File | Blob> = [testImage.value];
        if (spectrumBlob.value) images.push(spectrumBlob.value);
        const { results } = await lut.applyLUT(images, params);
        if (results[0]) setGradedUrl(lut.decodeApplyResult(results[0]));
        if (results[1]) setGradedSpectrumUrl(lut.decodeApplyResult(results[1]));
    } catch (err) {
        applyError.value = err instanceof Error ? err.message : String(err);
        setGradedUrl(null);
        setGradedSpectrumUrl(null);
    } finally {
        applyLoading.value = false;
    }
}

const paletteSize = computed(() => workspace.colors.length);
const blendMax = computed(() => Math.max(1, paletteSize.value));

// Clamp blend to the live palette length so a wheel slot deletion can't
// leave the slider stranded above the cap.
watch(paletteSize, (n) => {
    if (lutBlend.value > n && n > 0) lutBlend.value = n;
});

/** Swap in a new binary preview URL, revoking the previous one. */
function setBlobUrl(blob: Blob | null): void {
    if (blobUrl.value) URL.revokeObjectURL(blobUrl.value);
    blobUrl.value = blob ? URL.createObjectURL(blob) : null;
}

/** Reset every LUT knob (sliders + checkbox + cube size) to its default.
 *  `lutFormat` (Cube/LUT texture) and `name` are intentionally NOT reset —
 *  the user typically wants those preserved across grading sessions. */
function resetLUTDefaults(): void {
    lutRadius.value = DEFAULTS.radius;
    lutDistribution.value = DEFAULTS.distribution;
    lutIntensity.value = DEFAULTS.intensity;
    lutBlend.value = Math.min(DEFAULTS.blend, blendMax.value);
    lutReach.value = DEFAULTS.reach;
    lutSharpness.value = DEFAULTS.sharpness;
    lutStrength.value = DEFAULTS.strength;
    lutCompression.value = DEFAULTS.compression;
    lutMute.value = DEFAULTS.mute;
    lutSaturation.value = DEFAULTS.saturation;
    lutSize.value = DEFAULTS.lutSize;
}

async function generate(): Promise<void> {
    const fmt = format.value;
    const colors = workspace.colors.map((c) => c.hex);
    if (colors.length === 0) return;
    const palName = name.value.trim() || "palette";
    errorMsg.value = null;
    loading.value = true;
    // Hide stale preview while regenerating — user shouldn't see the
    // previous LUT texture / .cube text after a slider change has scheduled
    // a fresh fetch.
    content.value = "";
    setBlobUrl(null);
    try {
        if (fmt === "lut") {
            const params = currentLutParams();
            const lf = lutFormat.value;
            const res = await lut.generate({
                ...params,
                format: lf,
            });
            filename.value = res.filename;
            if (res.encoding === "base64") {
                setBlobUrl(lut.decodePNG(res));
            } else {
                content.value = res.content;
            }
        } else {
            const res = await exportPalette.general({
                format: fmt,
                colors,
                name: palName,
                shades:
                    fmt === "tailwind" && shades.value > 0
                        ? shades.value
                        : undefined,
                notation:
                    showNotation.value && colorNotation.value
                        ? colorNotation.value
                        : undefined,
            });
            filename.value = res.filename;
            if (res.encoding === "base64") {
                setBlobUrl(exportPalette.decodeBinary(res));
            } else {
                content.value = res.content;
            }
        }
    } catch (err) {
        errorMsg.value = err instanceof Error ? err.message : String(err);
        filename.value = "";
    } finally {
        loading.value = false;
    }
}

// Auto-regenerate the export preview (cube text or LUT texture PNG)
// on every input that affects it. Debounced so slider drags don't
// spam the endpoint.
watchDebounced(
    [
        format,
        name,
        shades,
        colorNotation,
        lutFormat,
        lutMethod,
        lutRadius,
        lutDistribution,
        lutIntensity,
        lutBlend,
        lutReach,
        lutSharpness,
        lutStrength,
        lutCompression,
        lutMute,
        lutSaturation,
        lutSize,
        () => workspace.colors,
    ],
    generate,
    { debounce: 300, immediate: true, deep: true },
);

// Auto-apply the current LUT to the test image + reference spectrum.
// Triggers on any LUT-param change OR on test-image swap. The backend
// regenerates the cube from `params` each call (deterministic, cheap),
// so we never upload the cube text.
watchDebounced(
    [
        testImage,
        lutMethod,
        lutRadius,
        lutDistribution,
        lutIntensity,
        lutBlend,
        lutReach,
        lutSharpness,
        lutStrength,
        lutCompression,
        lutMute,
        lutSaturation,
        lutSize,
        () => workspace.colors,
    ],
    () => {
        if (isLUT.value && testImage.value) {
            void applyTest();
        } else if (!testImage.value) {
            setGradedUrl(null);
            setGradedSpectrumUrl(null);
        }
    },
    { debounce: 300, deep: true },
);

onMounted(async () => {
    try {
        const blob = await makeSpectrumBlob();
        spectrumBlob.value = blob;
        originalSpectrumUrl.value = URL.createObjectURL(blob);
    } catch {
        // Spectrum generation failure is non-fatal — the test-image grade
        // still works, the user just doesn't get the rainbow strips.
    }
});

onBeforeUnmount(() => {
    setBlobUrl(null);
    setTestImageUrl(null);
    setGradedUrl(null);
    setGradedSpectrumUrl(null);
    if (originalSpectrumUrl.value)
        URL.revokeObjectURL(originalSpectrumUrl.value);
});

// Drop test-apply state when the user navigates away from LUT mode so a
// stale test image / graded preview can't leak into a re-entry. (Stays
// alive when switching between Cube and LUT texture sub-tabs.)
watch(isLUT, (active) => {
    if (!active) clearTest();
});

// Reset notation when format changes to a non-text format.
watch(format, (newFormat) => {
    if (
        !["css", "scss", "less", "tailwind", "txt"].includes(
            newFormat as string,
        )
    ) {
        colorNotation.value = "";
    }
});

async function copy(): Promise<void> {
    if (!content.value) return;
    try {
        await navigator.clipboard.writeText(content.value);
        copied.value = true;
        window.setTimeout(() => {
            copied.value = false;
        }, 1200);
    } catch {
        errorMsg.value = "clipboard unavailable";
    }
}

function download(): void {
    if (!hasOutput.value) return;
    // Binary output already has an object URL; text output gets a throwaway.
    const href =
        blobUrl.value ??
        URL.createObjectURL(
            new Blob([content.value], { type: "text/plain;charset=utf-8" }),
        );
    const a = document.createElement("a");
    a.href = href;
    a.download = filename.value || "palette.txt";
    a.click();
    if (!blobUrl.value) URL.revokeObjectURL(href);
}
</script>

<template>
    <div class="export">
        <div class="fmt-row" role="group" aria-label="Export format">
            <button
                v-for="f in FORMATS"
                :key="f.value"
                type="button"
                class="fmt-btn"
                :class="{ active: format === f.value }"
                :aria-pressed="format === f.value"
                @click="format = f.value"
            >
                {{ f.label }}
            </button>
        </div>

        <div v-if="!isLUT" class="fields">
            <label class="name-fld">
                <span>Name</span>
                <input
                    v-model="name"
                    type="text"
                    spellcheck="false"
                    autocomplete="off"
                    placeholder="palette"
                />
            </label>

            <!-- Shade-scale count is meaningful for Tailwind only. -->
            <label v-if="format === 'tailwind'" class="name-fld shades-fld">
                <span
                    >Shades <em>{{ shades === 0 ? "flat" : shades }}</em></span
                >
                <input
                    v-model.number="shades"
                    type="range"
                    min="0"
                    max="10"
                    aria-label="tailwind shade-scale count"
                />
            </label>

            <!-- Color notation for text-based formats. -->
            <label v-if="showNotation" class="name-fld">
                <span>Notation</span>
                <select
                    v-model="colorNotation"
                    aria-label="color value notation"
                >
                    <option
                        v-for="n in NOTATIONS"
                        :key="n.value"
                        :value="n.value"
                    >
                        {{ n.label }}
                    </option>
                </select>
            </label>
        </div>

        <!-- LUT panel — 2-column grid on both sub-tabs (Cube and LUT texture).
         Left column: sliders box on top + preview slot beneath. Right
         column: "Test on your image" block spanning both rows. The
         preview slot shows either the .cube text or the LUT texture PNG
         depending on the active sub-tab; everything below it (sliders
         state, test image, graded result) is shared across sub-tabs. -->
        <div v-if="isLUT" class="lut-panel lut-panel-split">
            <div class="lut-fields">
                <div class="lut-sub-row" role="group" aria-label="LUT format">
                    <button
                        type="button"
                        class="lut-sub-btn"
                        :class="{ active: lutFormat === 'cube' }"
                        :aria-pressed="lutFormat === 'cube'"
                        @click="lutFormat = 'cube'"
                    >
                        Cube
                    </button>
                    <button
                        type="button"
                        class="lut-sub-btn"
                        :class="{ active: lutFormat === 'png' }"
                        :aria-pressed="lutFormat === 'png'"
                        @click="lutFormat = 'png'"
                    >
                        LUT texture
                    </button>
                    <button
                        type="button"
                        class="lut-reset"
                        aria-label="reset LUT controls to defaults"
                        @click="resetLUTDefaults"
                    >
                        Reset
                    </button>
                </div>

                <div
                    class="lut-method-row"
                    role="radiogroup"
                    aria-label="LUT algorithm"
                >
                    <button
                        type="button"
                        class="lut-method-btn"
                        :class="{ active: lutMethod === 'grade' }"
                        role="radio"
                        :aria-checked="lutMethod === 'grade'"
                        @click="lutMethod = 'grade'"
                    >
                        Grade
                    </button>
                    <button
                        type="button"
                        class="lut-method-btn"
                        :class="{ active: lutMethod === 'rbf' }"
                        role="radio"
                        :aria-checked="lutMethod === 'rbf'"
                        @click="lutMethod = 'rbf'"
                    >
                        Smooth
                    </button>
                    <button
                        type="button"
                        class="lut-method-btn"
                        :class="{ active: lutMethod === 'knn' }"
                        role="radio"
                        :aria-checked="lutMethod === 'knn'"
                        @click="lutMethod = 'knn'"
                    >
                        Layered
                    </button>
                </div>

                <div class="lut-grid">
                    <!-- Grade knobs — the hue wheel is squeezed onto the
                         palette's hues (vectorscope compression); lightness
                         is never touched, so the image keeps its tonality. -->
                    <template v-if="lutMethod === 'grade'">
                        <label class="lut-fld">
                            <span
                                >Compression
                                <em>{{ lutCompression.toFixed(2) }}</em></span
                            >
                            <input
                                v-model.number="lutCompression"
                                type="range"
                                min="0"
                                max="1"
                                step="0.01"
                                aria-label="how hard hues are squeezed onto the palette"
                            />
                        </label>
                        <label class="lut-fld">
                            <span>Mute <em>{{ lutMute.toFixed(2) }}</em></span>
                            <input
                                v-model.number="lutMute"
                                type="range"
                                min="0"
                                max="1"
                                step="0.01"
                                aria-label="desaturate hues between palette colours"
                            />
                        </label>
                    </template>

                    <!-- RBF (Smooth) knobs — Gaussian-like kernel over every
                         palette colour, blended in OkLab a/b. No Voronoi edges,
                         no antipodal hue collapse. -->
                    <template v-else-if="lutMethod === 'rbf'">
                        <label class="lut-fld">
                            <span
                                >Reach <em>{{ lutReach.toFixed(2) }}</em></span
                            >
                            <input
                                v-model.number="lutReach"
                                type="range"
                                min="0.05"
                                max="0.6"
                                step="0.01"
                                aria-label="RBF kernel reach (OkLab σ)"
                            />
                        </label>
                        <label class="lut-fld">
                            <span
                                >Sharpness
                                <em>{{ lutSharpness.toFixed(1) }}</em></span
                            >
                            <input
                                v-model.number="lutSharpness"
                                type="range"
                                min="0.5"
                                max="6"
                                step="0.1"
                                aria-label="RBF kernel exponent"
                            />
                        </label>
                        <label class="lut-fld">
                            <span
                                >Strength
                                <em>{{ lutStrength.toFixed(2) }}</em></span
                            >
                            <input
                                v-model.number="lutStrength"
                                type="range"
                                min="0"
                                max="1"
                                step="0.01"
                                aria-label="RBF pull strength"
                            />
                        </label>
                    </template>

                    <!-- K-NN (Layered) knobs — legacy K-nearest with falloff +
                         OkLCH circular mean. Sharper, can seam across antipodal
                         palettes. -->
                    <template v-else>
                        <label class="lut-fld">
                            <span
                                >Radius
                                <em>{{ lutRadius.toFixed(2) }}</em></span
                            >
                            <input
                                v-model.number="lutRadius"
                                type="range"
                                min="0"
                                max="1.5"
                                step="0.01"
                                aria-label="OkLab pull radius"
                            />
                        </label>
                        <label class="lut-fld">
                            <span
                                >Distribution
                                <em>{{ lutDistribution.toFixed(2) }}</em></span
                            >
                            <input
                                v-model.number="lutDistribution"
                                type="range"
                                min="0"
                                max="1"
                                step="0.01"
                                aria-label="falloff distribution"
                            />
                        </label>
                        <label class="lut-fld">
                            <span
                                >Intensity
                                <em>{{ lutIntensity.toFixed(2) }}</em></span
                            >
                            <input
                                v-model.number="lutIntensity"
                                type="range"
                                min="0"
                                max="1"
                                step="0.01"
                                aria-label="pull intensity"
                            />
                        </label>
                        <label class="lut-fld">
                            <span
                                >Blend
                                <em>{{ lutBlend }} / {{ blendMax }}</em></span
                            >
                            <input
                                v-model.number="lutBlend"
                                type="range"
                                min="1"
                                :max="blendMax"
                                step="1"
                                aria-label="K-NN neighbours count"
                            />
                        </label>
                    </template>

                    <label class="lut-fld lut-toggle">
                        <input
                            v-model="lutSaturation"
                            type="checkbox"
                            aria-label="shift chroma along with hue"
                        />
                        <span>Saturation</span>
                    </label>
                    <label class="lut-fld lut-fld-wide">
                        <span>LUT level</span>
                        <select v-model.number="lutSize" aria-label="LUT level">
                            <option
                                v-for="opt in LUT_LEVELS"
                                :key="opt.level"
                                :value="opt.value"
                            >
                                {{ opt.label }}
                            </option>
                        </select>
                    </label>
                </div>
            </div>

            <!-- Preview slot — Cube text (mono pre) or LUT texture (image),
           same outer dimensions on both sub-tabs. While `loading` we
           clear `content` and `blobUrl` so the placeholder shows. -->
            <div class="lut-preview">
                <pre
                    v-if="lutFormat === 'cube'"
                    class="cube-text mono"
                    :class="{ placeholder: !content }"
                    >{{
                        displayedCubeContent ||
                        (loading ? "Generating…" : "No output")
                    }}</pre
                >
                <img
                    v-else-if="blobUrl"
                    class="thumb thumb-hald"
                    :src="blobUrl"
                    alt="LUT texture preview"
                />
                <div v-else class="thumb thumb-hald thumb-placeholder">
                    {{ loading ? "Generating…" : "No output" }}
                </div>
            </div>

            <!-- Test block — visible on both sub-tabs. Backend re-generates
           the cube from `params` on every call, so the grade uses the
           same LUT regardless of which sub-tab is open for preview. -->
            <div class="hald-test">
                <div class="hald-test-header">Test on your image</div>

                <button
                    type="button"
                    class="file-picker"
                    @click="pickerOpen = true"
                >
                    {{ testImage ? "Change image" : "Choose image…" }}
                </button>

                <div v-if="testImageUrl" class="hald-test-previews">
                    <div class="hald-test-cell">
                        <span class="hald-test-label">Original</span>
                        <div class="hald-test-frame">
                            <img :src="testImageUrl" alt="original" />
                        </div>
                        <!-- Ungraded reference hue spectrum — sits below the test
                 image as a "before" so the graded strip below the
                 Graded image can be compared at a glance. -->
                        <div
                            v-if="originalSpectrumUrl"
                            class="hald-test-spectrum"
                        >
                            <img
                                :src="originalSpectrumUrl"
                                alt="reference spectrum"
                            />
                        </div>
                    </div>
                    <div class="hald-test-cell">
                        <span class="hald-test-label">Graded</span>
                        <!-- Reference hue spectrum graded through the same LUT —
                 thin strip under the test result so the user can see
                 the grade's effect across the full hue range. -->
                        <div
                            v-if="gradedSpectrumUrl"
                            class="hald-test-spectrum"
                        >
                            <img
                                :src="gradedSpectrumUrl"
                                alt="graded spectrum"
                            />
                        </div>
                        <div class="hald-test-frame">
                            <img
                                v-if="gradedUrl"
                                :src="gradedUrl"
                                alt="graded"
                            />
                            <span v-else class="hald-test-empty-text">
                                {{ applyLoading ? "Applying…" : "Waiting…" }}
                            </span>
                        </div>
                    </div>
                </div>

                <div v-if="testImage" class="hald-test-actions">
                    <button
                        type="button"
                        class="btn btn-ghost"
                        :disabled="applyLoading"
                        @click="clearTest"
                    >
                        Clear
                    </button>
                </div>

                <div v-if="applyError" class="hald-test-error mono">
                    {{ applyError }}
                </div>
            </div>
        </div>

        <!-- Non-LUT output (regular palette export: thumb / binary note / text). -->
        <img
            v-if="!isLUT && isImage"
            class="thumb"
            :src="blobUrl ?? ''"
            alt="exported palette preview"
        />
        <div v-else-if="!isLUT && isBinary" class="output binary-note">
            Binary swatch file ready — {{ filename }}
        </div>
        <pre
            v-else-if="!isLUT"
            class="output mono"
            :class="{ placeholder: !content }"
            >{{ content || (loading ? "Generating…" : "No output") }}</pre
        >

        <div v-if="errorMsg" class="error mono" role="alert">
            {{ errorMsg }}
        </div>

        <div class="actions">
            <button
                v-if="!isBinary"
                type="button"
                class="btn"
                :disabled="!content"
                @click="copy"
            >
                {{ copied ? "Copied!" : "Copy" }}
            </button>
            <button
                type="button"
                class="btn"
                :disabled="!hasOutput"
                @click="download"
            >
                Download
            </button>
        </div>

        <ImagePickerModal
            v-model:open="pickerOpen"
            title="Test on your image"
            @pick="onTestImagePicked"
        />
    </div>
</template>

<style scoped>
.export {
    display: flex;
    flex-direction: column;
    gap: 10px;
}

.fmt-row {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
}

.fmt-btn {
    padding: 5px 9px;
    font-size: 11px;
    font-weight: 500;
    font-family: inherit;
    color: var(--fg-2);
    background: var(--bg-2);
    border: 1px solid var(--line-soft);
    border-radius: 7px;
    cursor: pointer;
}

.fmt-btn:hover {
    color: var(--fg-0);
    border-color: var(--accent-line);
}

.fmt-btn.active {
    background: var(--accent-soft);
    color: var(--fg-0);
    border-color: var(--accent-line);
}

.fields {
    display: flex;
    gap: 10px;
}

.name-fld {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 11px;
    color: var(--fg-2);
    font-weight: 500;
    flex: 1 1 auto;
}

.name-fld em {
    color: var(--fg-3);
    font-weight: 400;
    font-style: normal;
}

.name-fld input[type="text"] {
    background: var(--bg-2);
    border: 1px solid var(--line-soft);
    border-radius: 7px;
    padding: 6px 8px;
    color: var(--fg-0);
    font-family: inherit;
    font-size: 12px;
}

.name-fld input[type="text"]:focus {
    outline: none;
    border-color: var(--accent);
}

.name-fld select {
    background: var(--bg-2);
    border: 1px solid var(--line-soft);
    border-radius: 7px;
    padding: 6px 8px;
    color: var(--fg-0);
    font-family: inherit;
    font-size: 12px;
}

.name-fld select:focus {
    outline: none;
    border-color: var(--accent);
}

.shades-fld {
    flex: 0 0 130px;
}

.shades-fld input[type="range"] {
    accent-color: var(--accent);
    margin-top: 5px;
}

/* LUT panel — single column in Cube mode; switches to a two-column grid
 * in HALD mode where the right column (`.hald-test`) spans both rows of
 * the left column (sliders box + thumb). Falls back to a single column
 * via the named-area override when the card is narrower than ~540px. */
.lut-panel {
    display: flex;
    flex-direction: column;
    gap: 10px;
}

.lut-panel-split {
    display: grid;
    grid-template-columns: minmax(240px, 1fr) minmax(240px, 1fr);
    grid-template-areas:
        "sliders test"
        "preview test";
    gap: 10px;
}

.lut-panel-split .lut-fields {
    grid-area: sliders;
}
.lut-panel-split .lut-preview {
    grid-area: preview;
}
.lut-panel-split .hald-test {
    grid-area: test;
}

@media (max-width: 540px) {
    .lut-panel-split {
        grid-template-columns: 1fr;
        grid-template-areas:
            "sliders"
            "preview"
            "test";
    }
}

/* Preview slot — same square outline for both .cube-text (mono pre,
 * scrollable) and .thumb-hald (LUT texture image). Lines up visually
 * with the sliders box above it. */
.lut-preview {
    display: flex;
    min-height: 0;
}

.cube-text {
    flex: 1 1 auto;
    margin: 0;
    width: 100%;
    height: auto;
    aspect-ratio: 1 / 1;
    overflow: auto;
    padding: 10px;
    background: var(--bg-2);
    border: 1px solid var(--line-soft);
    border-radius: var(--r-md);
    font-size: 10.5px;
    line-height: 1.45;
    color: var(--fg-1);
    white-space: pre;
    tab-size: 2;
    box-sizing: border-box;
}

.cube-text.placeholder {
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--fg-3);
}

.lut-fields {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 10px;
    border: 1px solid var(--line-soft);
    border-radius: var(--r-md);
    background: var(--bg-2);
}

.lut-sub-row {
    display: flex;
    gap: 4px;
    align-items: center;
}

.lut-sub-btn {
    padding: 4px 10px;
    font-size: 11px;
    font-weight: 500;
    font-family: inherit;
    color: var(--fg-2);
    background: var(--bg-1);
    border: 1px solid var(--line-soft);
    border-radius: 7px;
    cursor: pointer;
}

.lut-sub-btn:hover {
    color: var(--fg-0);
    border-color: var(--accent-line);
}

.lut-sub-btn.active {
    background: var(--accent-soft);
    color: var(--fg-0);
    border-color: var(--accent-line);
}

.lut-reset {
    margin-left: auto;
    padding: 4px 10px;
    font-size: 11px;
    font-weight: 500;
    font-family: inherit;
    color: var(--fg-3);
    background: transparent;
    border: 1px solid var(--line-soft);
    border-radius: 7px;
    cursor: pointer;
}

.lut-reset:hover {
    color: var(--fg-0);
    border-color: var(--accent-line);
}

.lut-method-row {
    display: flex;
    gap: 4px;
}

.lut-method-btn {
    flex: 1 1 0;
    padding: 5px 10px;
    font-size: 11px;
    font-weight: 500;
    font-family: inherit;
    color: var(--fg-2);
    background: var(--bg-1);
    border: 1px solid var(--line-soft);
    border-radius: 7px;
    cursor: pointer;
}

.lut-method-btn:hover {
    color: var(--fg-0);
    border-color: var(--accent-line);
}

.lut-method-btn.active {
    background: var(--accent-soft);
    color: var(--fg-0);
    border-color: var(--accent-line);
}

.lut-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px 12px;
}

.lut-fld {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 11px;
    color: var(--fg-2);
    font-weight: 500;
}

.lut-fld-wide {
    grid-column: 1 / -1;
}

.lut-fld em {
    color: var(--fg-3);
    font-weight: 400;
    font-style: normal;
}

.lut-fld input[type="range"] {
    accent-color: var(--accent);
}

.lut-fld select {
    background: var(--bg-1);
    border: 1px solid var(--line-soft);
    border-radius: 6px;
    padding: 4px 6px;
    color: var(--fg-0);
    font-family: inherit;
    font-size: 12px;
}

.lut-fld select:focus {
    outline: none;
    border-color: var(--accent);
}

.lut-toggle {
    flex-direction: row;
    align-items: center;
    gap: 6px;
}

.lut-toggle input[type="checkbox"] {
    accent-color: var(--accent);
}

.output {
    margin: 0;
    height: 180px;
    overflow: auto;
    padding: 10px;
    background: var(--bg-2);
    border: 1px solid var(--line-soft);
    border-radius: var(--r-md);
    font-size: 11px;
    line-height: 1.5;
    color: var(--fg-1);
    white-space: pre;
    tab-size: 2;
}

.output.placeholder {
    color: var(--fg-3);
}

.thumb {
    object-fit: contain;
    padding: 10px;
    background: var(--bg-2);
    border: 1px solid var(--line-soft);
    border-radius: var(--r-md);
    image-rendering: pixelated;
    box-sizing: border-box;
}

/* HALD-mode thumb fills the grid column and stays square so it always
 * lines up with the LUT-fields box above it regardless of card width. */
.thumb-hald {
    width: 100%;
    height: auto;
    aspect-ratio: 1 / 1;
}

.thumb-placeholder {
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 11px;
    color: var(--fg-3);
    padding: 0;
}

.thumb.placeholder {
    opacity: 0;
}

.hald-test {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 10px;
    background: var(--bg-2);
    border: 1px solid var(--line-soft);
    border-radius: var(--r-md);
    min-height: 240px;
}

.hald-test-header {
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.03em;
    text-transform: uppercase;
    color: var(--fg-3);
}

.file-picker {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 6px 10px;
    font-size: 11px;
    font-weight: 500;
    font-family: inherit;
    color: var(--fg-2);
    background: var(--bg-1);
    border: 1px dashed var(--line);
    border-radius: 7px;
    cursor: pointer;
}

.file-picker:hover {
    color: var(--fg-0);
    border-color: var(--accent-line);
}

/* Single-column preview stack: Original on top, Graded below. Each cell
 * flexes to fill the remaining height of the `.hald-test` column so the
 * images get as much room as the surrounding sliders box gives them. */
.hald-test-previews {
    display: flex;
    flex-direction: column;
    gap: 6px;
    flex: 1 1 auto;
    min-height: 0;
}

.hald-test-cell {
    display: flex;
    flex-direction: column;
    gap: 3px;
    flex: 1 1 0;
    min-height: 0;
}

.hald-test-label {
    font-size: 10px;
    color: var(--fg-3);
    text-transform: uppercase;
    letter-spacing: 0.05em;
}

/* Frame is the flex-growing element; the <img> inside sizes naturally
 * up to the frame caps. Small images stay small (no upscaling), large
 * images shrink to fit while preserving aspect — and everything sits
 * centred on the neutral bg. */
.hald-test-frame {
    flex: 1 1 0;
    min-height: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-1);
    border: 1px solid var(--line-soft);
    border-radius: 6px;
    overflow: hidden;
}

.hald-test-frame img {
    max-width: 100%;
    max-height: 100%;
    display: block;
}

.hald-test-empty-text {
    font-size: 10.5px;
    color: var(--fg-3);
}

/* Reference spectrum strip — a 512×32 rainbow PNG that the backend
 * grades through the same LUT as the test image. Slim and full-width
 * to read as a "sample" rather than a full preview. */
.hald-test-spectrum {
    margin-top: 4px;
    margin-bottom: 4px;
    width: 100%;
    height: 24px;
    border: 1px solid var(--line-soft);
    border-radius: 4px;
    overflow: hidden;
    flex: 0 0 auto;
}

.hald-test-spectrum img {
    width: 100%;
    height: 100%;
    object-fit: fill;
    display: block;
}

.hald-test-actions {
    display: flex;
    gap: 6px;
    margin-top: auto;
}

.btn-ghost {
    flex: 0 0 auto;
    background: transparent;
    color: var(--fg-3);
}

.hald-test-error {
    padding: 6px 8px;
    background: oklch(0.7 0.18 25 / 0.16);
    color: var(--bad);
    border-radius: 6px;
    font-size: 10.5px;
    line-height: 1.4;
}

.binary-note {
    display: flex;
    align-items: center;
    justify-content: center;
    text-align: center;
    color: var(--fg-2);
    font-size: 11.5px;
}

.actions {
    display: flex;
    gap: 6px;
}

.btn {
    flex: 1 1 auto;
    padding: 7px 10px;
    border-radius: 8px;
    font-size: 11.5px;
    font-weight: 600;
    font-family: inherit;
    cursor: pointer;
    background: var(--bg-2);
    color: var(--fg-0);
    border: 1px solid var(--line);
}

.btn:hover:not(:disabled) {
    border-color: var(--accent);
}

.btn:disabled {
    opacity: 0.45;
    cursor: not-allowed;
}

.error {
    padding: 6px 10px;
    background: oklch(0.7 0.18 25 / 0.16);
    color: var(--bad);
    border-radius: 6px;
    font-size: 10.5px;
}
</style>
