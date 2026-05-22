<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import { useRoute } from "vue-router";
import CardFrame from "../components/CardFrame.vue";
import ColorWheel from "../components/ColorWheel.vue";
import HarmonySelector from "../components/HarmonySelector.vue";
import PaletteStrip from "../components/PaletteStrip.vue";
import ColorSliders from "../components/ColorSliders.vue";
import ColorPicker from "../components/ColorPicker.vue";
import ColorOutput from "../components/ColorOutput.vue";
import PaletteExport from "../components/PaletteExport.vue";
import ImageExtractor from "../components/ImageExtractor.vue";
import ImagePalettePicker from "../components/ImagePalettePicker.vue";
import ExtractedPalette from "../components/ExtractedPalette.vue";
import GradientStudio from "../components/GradientStudio.vue";
import ColorBlindnessSim from "../components/ColorBlindnessSim.vue";
import ContrastChecker from "../components/ContrastChecker.vue";
import { useWorkspaceStore } from "../stores/workspace";
import { useHarmonyStore } from "../stores/harmony";
import { useExtractionStore } from "../stores/extraction";
import { fromHex, toHSL, toOkLCH } from "../composables/useColor";
import { stripToDataURL, type AnalyzeMetric } from "../api/analyze";

// Strip row labels — matches `internal/analyze.AllMetrics` order so the
// labels and the strip dataURLs stay in step.
const STRIP_LABELS: { metric: AnalyzeMetric; label: string }[] = [
    { metric: "hue", label: "Hue" },
    { metric: "luminance", label: "Luminance" },
    { metric: "saturation", label: "Saturation" },
    { metric: "distance", label: "Distance" },
];

// Every generator / tool / checker card lives on this one page. The
// sidebar's old per-route links are now in-page scroll anchors — each
// CardFrame carries an `id` that matches a `?section=` query value.
const workspace = useWorkspaceStore();
const harmony = useHarmonyStore();
const extraction = useExtractionStore();
const route = useRoute();

// Pin toggle for the Color wheel card — keeps the wheel in view as the
// user scrolls through the long tools page. Local UI state, never
// persisted: pinning is meant to be a transient assist while editing.
const wheelPinned = ref(false);

// Position of the pinned card in viewport coordinates. `null` until the
// user drags it for the first time — until then the default CSS anchor
// (top-right under the topbar) applies. Set once and then kept until
// the user reloads or unpins+repins.
const wheelPinPos = ref<{ x: number; y: number } | null>(null);

// Pointer-drag wiring for the pinned card. The handler is attached to
// CardFrame's root via fallthrough listeners and only acts when the
// pointerdown originated inside `.card-h` (the title strip) — clicks on
// the pin button or any other interactive control inside the header are
// ignored so they keep their normal behaviour.
let drag: {
    startX: number;
    startY: number;
    origX: number;
    origY: number;
    // Cached card box at drag-start. Re-reading offsetWidth/offsetHeight
    // every pointermove forces a synchronous layout flush, which scales
    // with total DOM cost — e.g. a level-8 LUT cube preview that parks
    // 64³ lines of <pre> text in the page turns the drag into a layout
    // thrash. The card can't resize mid-gesture, so the snapshot is safe.
    cardW: number;
    cardH: number;
} | null = null;

function onWheelCardPointerDown(e: PointerEvent) {
    if (!wheelPinned.value) return;
    if (e.button !== undefined && e.button !== 0) return;
    const target = e.target as HTMLElement | null;
    if (!target) return;
    if (target.closest("button, input, select, textarea, a")) return;
    const header = target.closest(".card-h") as HTMLElement | null;
    if (!header) return;
    const card = header.parentElement as HTMLElement | null;
    if (!card) return;

    // One layout read for the whole gesture: pos (if needed) + size, both
    // off the same rect.
    const r = card.getBoundingClientRect();
    if (!wheelPinPos.value) {
        wheelPinPos.value = { x: r.left, y: r.top };
    }

    drag = {
        startX: e.clientX,
        startY: e.clientY,
        origX: wheelPinPos.value.x,
        origY: wheelPinPos.value.y,
        cardW: r.width,
        cardH: r.height,
    };
    e.preventDefault();
    window.addEventListener("pointermove", onPointerMove);
    window.addEventListener("pointerup", onPointerUp, { once: true });
    window.addEventListener("pointercancel", onPointerUp, { once: true });
    document.body.style.userSelect = "none";
}

function onPointerMove(e: PointerEvent) {
    if (!drag || !wheelPinPos.value) return;
    const w = drag.cardW;
    const h = drag.cardH;
    const x = Math.max(
        0,
        Math.min(window.innerWidth - w, drag.origX + e.clientX - drag.startX),
    );
    const y = Math.max(
        0,
        Math.min(
            Math.max(0, window.innerHeight - h),
            drag.origY + e.clientY - drag.startY,
        ),
    );
    wheelPinPos.value = { x, y };
}

function onPointerUp() {
    drag = null;
    window.removeEventListener("pointermove", onPointerMove);
    document.body.style.userSelect = "";
}

// Compute the inline style for the pinned card. While dragging (or after
// the first drag) we anchor by left/top and release the default right
// anchor; until then we leave styling to the `.pinned-card` CSS rule.
const wheelCardStyle = computed(() => {
    if (!wheelPinned.value || !wheelPinPos.value) return undefined;
    return {
        left: `${wheelPinPos.value.x}px`,
        top: `${wheelPinPos.value.y}px`,
        right: "auto",
    };
});

const base = computed(() => {
    // The Harmony card's read-out always tracks the wheel-selected slot, so
    // it reflects whichever colour the user is currently editing.
    const c = workspace.colors[workspace.selectedSlot];
    if (!c) return null;
    const rgb = fromHex(c.hex);
    return {
        hex: c.hex.toUpperCase(),
        hsl: toHSL(rgb),
        oklch: toOkLCH(rgb),
    };
});

// Sidebar links set `?section=<card-id>`; scroll the matching card into
// view. The watcher only fires when the value actually changes, so it
// never fights the user's manual scrolling. `immediate` covers deep-links
// on first load (incl. the /tools and /contrast redirects).
watch(
    () => route.query.section,
    (section) => {
        if (typeof section !== "string" || section === "") return;
        nextTick(() => {
            document
                .getElementById(section)
                ?.scrollIntoView({ behavior: "smooth", block: "start" });
        });
    },
    { immediate: true },
);
</script>

<template>
    <div class="grid">
        <CardFrame id="harmony" title="Harmony" sub="tonal rules" class="col-1">
            <HarmonySelector />
            <div v-if="base" class="base-readout">
                <div class="kv">
                    <span class="k">Hex</span>
                    <span class="v mono">{{ base.hex }}</span>
                </div>
                <div class="kv">
                    <span class="k">HSL</span>
                    <span class="v mono">
                        {{ Math.round(base.hsl.h) }}°
                        {{ Math.round(base.hsl.s * 100) }}%
                        {{ Math.round(base.hsl.l * 100) }}%
                    </span>
                </div>
                <div class="kv">
                    <span class="k">OkLCH</span>
                    <span class="v mono">
                        {{ base.oklch.L.toFixed(3) }}
                        {{ base.oklch.C.toFixed(3) }}
                        {{ Math.round(base.oklch.H) }}°
                    </span>
                </div>
            </div>
            <div class="controls">
                <button
                    class="ctrl-btn"
                    type="button"
                    :disabled="!workspace.canUndo"
                    @click="workspace.undo"
                >
                    Undo
                </button>
                <button
                    class="ctrl-btn"
                    type="button"
                    :disabled="!workspace.canRedo"
                    @click="workspace.redo"
                >
                    Redo
                </button>
                <button class="ctrl-btn" type="button" @click="workspace.reset">
                    Reset
                </button>
            </div>
        </CardFrame>

        <CardFrame
            id="wheel"
            title="Color wheel"
            :sub="`${harmony.type} · ${workspace.size} slots`"
            class="col-2"
            :class="{ 'pinned-card': wheelPinned }"
            :style="wheelCardStyle"
            @pointerdown="onWheelCardPointerDown"
        >
            <template #header-right>
                <button
                    type="button"
                    class="pin-btn"
                    :class="{ active: wheelPinned }"
                    :aria-pressed="wheelPinned"
                    :title="wheelPinned ? 'Unpin Color wheel' : 'Pin Color wheel to stay visible while scrolling'"
                    @click="wheelPinned = !wheelPinned"
                >
                    <svg
                        width="14"
                        height="14"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        aria-hidden="true"
                    >
                        <path d="M12 17v5" />
                        <path
                            d="M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.89A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.89A2 2 0 0 1 15 10.76V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H8a2 2 0 0 0 0 4 1 1 0 0 1 1 1z"
                        />
                    </svg>
                </button>
            </template>
            <div class="wheel-host">
                <ColorWheel />
            </div>
            <ul class="wheel-hints" aria-hidden="true">
                <li><kbd>drag</kbd><span>hue + sat</span></li>
                <li><kbd>scroll</kbd><span>value</span></li>
                <li><kbd>shift</kbd><span>lock hue</span></li>
                <li><kbd>alt</kbd><span>lock sat</span></li>
            </ul>
            <label
                v-if="harmony.type !== 'custom'"
                class="indep-toggle"
                title="Drag a non-base handle to tweak only its saturation / value — the harmony keeps its hues"
            >
                <input v-model="harmony.independentSV" type="checkbox" />
                <span
                    >Independent S/V — non-base drags tweak saturation / value
                    only</span
                >
            </label>
            <PaletteStrip />
            <ColorSliders />
        </CardFrame>

        <CardFrame
            id="color-picker"
            title="Color picker"
            sub="HSV"
            class="col-3"
        >
            <ColorPicker />
        </CardFrame>

        <CardFrame
            id="from-image"
            title="From image"
            :sub="
                extraction.image
                    ? 'drag pins to resample'
                    : 'drop · URL · pin overlay'
            "
            class="col-1-2 image-card"
        >
            <div class="image-card-body">
                <ImageExtractor class="image-controls" />
                <div class="image-viewport">
                    <ImagePalettePicker />
                </div>
            </div>
            <div v-if="extraction.image" class="analysis-block">
                <div class="analysis-controls">
                    <label class="ana-fld">
                        <span>Space</span>
                        <select v-model="extraction.analyzeSpace">
                            <option value="oklch">OkLCH</option>
                            <option value="hsl">HSL</option>
                        </select>
                    </label>
                    <label class="ana-fld">
                        <span>Distance target</span>
                        <select v-model="extraction.analyzeDistanceTarget">
                            <option value="red">Red</option>
                            <option value="green">Green</option>
                            <option value="blue">Blue</option>
                        </select>
                    </label>
                </div>
                <div class="metric-strip-row">
                    <div
                        v-for="entry in STRIP_LABELS"
                        :key="entry.metric"
                        class="metric-strip"
                    >
                        <div class="metric-strip-label mono">{{ entry.label }}</div>
                        <img
                            v-if="extraction.strips[entry.metric]"
                            class="metric-strip-img"
                            :alt="`${entry.label} distribution`"
                            :src="stripToDataURL(extraction.strips[entry.metric]!)"
                        />
                        <div
                            v-else
                            class="metric-strip-placeholder"
                            aria-hidden="true"
                        />
                    </div>
                </div>
            </div>
        </CardFrame>

        <CardFrame
            id="extracted"
            title="Extracted palette"
            sub="frequency · saliency"
            class="col-3"
        >
            <ExtractedPalette />
        </CardFrame>

        <CardFrame
            id="values"
            title="Values"
            sub="copy any format"
            class="col-1"
        >
            <ColorOutput />
        </CardFrame>

        <CardFrame
            id="export"
            title="Export"
            sub="CSS · SCSS · Less · Tailwind"
            class="col-2-3"
        >
            <PaletteExport />
        </CardFrame>

        <CardFrame
            id="gradient"
            title="Gradient"
            sub="multi-stop · interpolate"
            class="full"
        >
            <GradientStudio />
        </CardFrame>

        <CardFrame
            id="blindness"
            title="Color blindness"
            sub="protan · deutan · tritan · achroma"
            class="full"
        >
            <ColorBlindnessSim />
        </CardFrame>

        <CardFrame
            id="contrast"
            title="Contrast checker"
            sub="WCAG 2.1 · APCA"
            class="full"
        >
            <ContrastChecker />
        </CardFrame>
    </div>
</template>

<style scoped>
.grid {
    display: grid;
    grid-template-columns: 280px 1fr 320px;
    grid-auto-rows: min-content;
    gap: 14px;
}

/* Cards are declared row-major; auto-flow fills the rows. `.full` cards
   span every column, which also forces the next card onto a fresh row. */
.col-1 {
    grid-column: 1;
}
.col-2 {
    grid-column: 2;
}
.col-3 {
    grid-column: 3;
}
.col-1-2 {
    grid-column: 1 / span 2;
}
.col-2-3 {
    grid-column: 2 / span 2;
}
.full {
    grid-column: 1 / -1;
}

@media (max-width: 1100px) {
    .grid {
        grid-template-columns: 1fr;
    }

    /* Single column — explicit placement no longer applies. */
    .grid > * {
        grid-column: 1 !important;
    }
}

.wheel-host {
    display: flex;
    justify-content: center;
    margin-bottom: 8px;
}

/* Pin toggle in the wheel card header — small icon button that flips
   the card into a viewport-fixed overlay so it remains reachable while
   scrolling through the long tools page. */
.pin-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    padding: 0;
    background: transparent;
    border: 1px solid transparent;
    border-radius: 6px;
    color: var(--fg-3);
    cursor: pointer;
    transition:
        color 0.12s ease,
        background 0.12s ease,
        border-color 0.12s ease,
        transform 0.12s ease;
}

.pin-btn:hover {
    color: var(--fg-0);
    background: var(--bg-2);
    border-color: var(--line-soft);
}

.pin-btn.active {
    color: var(--accent);
    background: var(--bg-2);
    border-color: var(--line);
    transform: rotate(-30deg);
}

/* When pinned the wheel card floats over the page, anchored to the
   top-right under the 52px sticky topbar. It vacates its grid track —
   neighbouring cards reflow up. Scrollable internally in case the
   viewport is short. */
.pinned-card {
    position: fixed;
    top: 64px;
    right: 14px;
    width: 360px;
    max-height: calc(100vh - 78px);
    overflow: auto;
    z-index: 40;
    box-shadow:
        0 12px 32px rgba(0, 0, 0, 0.35),
        0 4px 10px rgba(0, 0, 0, 0.25);
}

/* Drag affordance on the title strip while pinned. `:deep` pierces
   CardFrame's scoped styles so the cursor change reaches `.card-h`,
   which is owned by that child component. */
.pinned-card :deep(.card-h) {
    cursor: grab;
}

.pinned-card :deep(.card-h):active {
    cursor: grabbing;
}

/* The pin button inside the header should keep a pointer cursor — it's
   a click target, not a drag handle. */
.pinned-card :deep(.card-h) .pin-btn,
.pinned-card :deep(.card-h) .pin-btn:active {
    cursor: pointer;
}

@media (max-width: 1100px) {
    /* On the single-column layout pinning would cover most of the
       screen — disable so the card behaves like every other one. */
    .pinned-card {
        position: static;
        width: auto;
        max-height: none;
        box-shadow: var(--shadow-card);
    }
}

/* Gesture legend for the wheel — mirrors the modifiers
   useHandleGesture reads. Decorative, hence aria-hidden. */
.wheel-hints {
    list-style: none;
    margin: 0 0 12px;
    padding: 0;
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 5px 10px;
    font-size: 10px;
    color: var(--fg-3);
}

.wheel-hints li {
    display: inline-flex;
    align-items: center;
    gap: 5px;
}

.wheel-hints kbd {
    font-size: 9px;
    padding: 1px 5px;
    border: 1px solid var(--line-soft);
    border-radius: 4px;
    background: var(--bg-2);
    color: var(--fg-2);
}

/* Per-slot S/V tweak toggle. Sits with the wheel legend; centred
   to stay visually tied to the wheel above it. */
.indep-toggle {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 7px;
    margin: 0 0 12px;
    font-size: 11px;
    color: var(--fg-2);
    cursor: pointer;
    user-select: none;
}

.indep-toggle input {
    accent-color: var(--accent);
    cursor: pointer;
}

.base-readout {
    margin-top: 12px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 10px 12px;
    background: var(--bg-2);
    border: 1px solid var(--line-soft);
    border-radius: var(--r-md);
}

.kv {
    display: grid;
    grid-template-columns: 56px 1fr;
    gap: 10px;
    font-size: 11px;
}

.kv .k {
    color: var(--fg-3);
}

.kv .v {
    color: var(--fg-0);
}

.controls {
    display: flex;
    gap: 6px;
    margin-top: 10px;
}

.ctrl-btn {
    flex: 1 1 auto;
    padding: 6px 8px;
    background: var(--bg-2);
    border: 1px solid var(--line-soft);
    border-radius: 7px;
    color: var(--fg-1);
    font-size: 11px;
    font-family: inherit;
    cursor: pointer;
}

.ctrl-btn:hover:not(:disabled) {
    color: var(--fg-0);
    border-color: var(--line);
}

.ctrl-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
}

.image-card .image-card-body {
    display: grid;
    grid-template-columns: 220px 1fr;
    gap: 14px;
    align-items: stretch;
    min-height: 280px;
}

.image-viewport {
    position: relative;
    border-radius: var(--r-md);
    overflow: hidden;
    background: var(--bg-2);
}

/* Colour-distribution strips for the active image. Stack vertically so
   each metric reads as a full-width band — labels sit above the band,
   making the row easy to scan at a glance. Prefixed `metric-` to avoid
   colliding with PaletteStrip.vue's root `.strip`, which inherits this
   scope marker as a child-component root. */
.analysis-block {
    margin-top: 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.analysis-controls {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
    align-items: flex-end;
}

.ana-fld {
    display: flex;
    flex-direction: column;
    gap: 3px;
    font-size: 10.5px;
    color: var(--fg-2);
    font-weight: 500;
}

.ana-fld select {
    background: var(--bg-2);
    border: 1px solid var(--line-soft);
    border-radius: 7px;
    padding: 5px 8px;
    color: var(--fg-0);
    font-family: inherit;
    font-size: 12px;
}

.metric-strip-row {
    display: flex;
    flex-direction: column;
    gap: 6px;
}

.metric-strip {
    display: flex;
    flex-direction: column;
    gap: 3px;
}

.metric-strip-label {
    font-size: 10px;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    color: var(--fg-3);
}

.metric-strip-img,
.metric-strip-placeholder {
    display: block;
    width: 100%;
    height: 40px;
    border-radius: 4px;
    /* The strips are 800 px wide upstream; this avoids the browser
     smoothing the bucket boundaries into a blurry haze when the card
     is wider than 800 px. */
    image-rendering: pixelated;
    background: var(--bg-2);
}

.metric-strip-placeholder {
    background: linear-gradient(
        90deg,
        var(--bg-2) 0%,
        var(--line-soft) 50%,
        var(--bg-2) 100%
    );
    opacity: 0.5;
}
</style>
