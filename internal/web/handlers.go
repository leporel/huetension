package web

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/leporel/huetension/internal/blindness"
	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/contrast"
	"github.com/leporel/huetension/internal/exporter"
	"github.com/leporel/huetension/internal/gradient"
	"github.com/leporel/huetension/internal/harmony"
	"github.com/leporel/huetension/internal/palette"
	"github.com/leporel/huetension/internal/sandbox"
)

// apiHandlers carries cross-cutting deps individual handlers need. Most
// endpoints in this package are stateless (color math + exporter calls);
// /extract consults the sandbox before loading a file/URL/data-uri, and
// /library* serves the curated catalogue from the configured index.
type apiHandlers struct {
	sandbox sandbox.ImageSandbox
	library *libraryState
}

// registerAPI mounts the REST endpoints on mux under base. Method patterns
// rely on Go 1.22+ ServeMux behaviour ("GET /path"); see go.mod for the
// version pin. Stateless handlers are referenced by name; deps.* methods
// close over the apiHandlers value so /extract sees the right sandbox.
func registerAPI(mux *http.ServeMux, base string, deps apiHandlers) {
	mux.HandleFunc("GET "+base+"/color/convert", handleColorConvert)
	mux.HandleFunc("POST "+base+"/color/sort", handleColorSort)
	mux.HandleFunc("GET "+base+"/harmony/{type}/{color}", handleHarmonyGenerate)
	mux.HandleFunc("GET "+base+"/gradient", handleGradient)
	mux.HandleFunc("GET "+base+"/contrast", handleContrastCheck)
	mux.HandleFunc("GET "+base+"/random", handlePaletteRandom)
	mux.HandleFunc("POST "+base+"/export", handleExport)
	mux.HandleFunc("POST "+base+"/lut", handleLUT)
	mux.HandleFunc("POST "+base+"/apply-lut", deps.handleApplyLUT)
	mux.HandleFunc("POST "+base+"/blindness/simulate", handleBlindnessSimulate)
	mux.HandleFunc("POST "+base+"/extract", deps.handleExtract)
	mux.HandleFunc("POST "+base+"/analyze", deps.handleAnalyze)
	registerLibrary(mux, base, deps.library)
}

// ---------- /color/convert ----------

type convertResult struct {
	Input   string            `json:"input"`
	Hex     string            `json:"hex"`
	Formats map[string]string `json:"formats,omitempty"`
	Value   string            `json:"value,omitempty"`
}

func handleColorConvert(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	colorStr := strings.TrimSpace(q.Get("color"))
	to := strings.ToLower(strings.TrimSpace(q.Get("to")))
	if to == "" {
		to = "all"
	}
	if colorStr == "" {
		writeError(w, http.StatusBadRequest, errors.New("color parameter is required"))
		return
	}
	c, err := color.Parse(colorStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("color: %w", err))
		return
	}
	res := convertResult{Input: colorStr, Hex: c.Hex()}
	if to == "all" {
		pairs := c.FormatAll()
		res.Formats = make(map[string]string, len(pairs))
		for _, p := range pairs {
			res.Formats[string(p.Format)] = p.Value
		}
	} else {
		v, ferr := c.Format(color.Format(to))
		if ferr != nil {
			writeError(w, http.StatusBadRequest, ferr)
			return
		}
		res.Value = v
	}
	writeEnvelope(w, "color.convert", map[string]any{"color": colorStr, "to": to}, res)
}

// ---------- /color/sort ----------

type sortRequest struct {
	Colors  []string `json:"colors"`
	By      string   `json:"by,omitempty"`
	Reverse bool     `json:"reverse,omitempty"`
}

func handleColorSort(w http.ResponseWriter, r *http.Request) {
	var req sortRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(req.Colors) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("no colors provided"))
		return
	}
	cs, err := parseColorList(req.Colors)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	by := palette.SortBy(req.By)
	if by == "" {
		by = palette.SortByLuminance
	}
	pal := palette.New(cs)
	if err := pal.Sort(by, req.Reverse); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	params := map[string]any{
		"colors":  req.Colors,
		"by":      string(by),
		"reverse": req.Reverse,
	}
	writePaletteJSON(w, "color.sort", params, pal)
}

// ---------- /harmony/{type}/{color} ----------

func handleHarmonyGenerate(w http.ResponseWriter, r *http.Request) {
	typeStr := r.PathValue("type")
	baseStr := r.PathValue("color")
	q := r.URL.Query()

	if strings.TrimSpace(typeStr) == "" {
		writeError(w, http.StatusBadRequest, errors.New("type is required"))
		return
	}
	if strings.TrimSpace(baseStr) == "" {
		writeError(w, http.StatusBadRequest, errors.New("color is required"))
		return
	}
	t, err := resolveHarmonyType(typeStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	base, err := color.Parse(baseStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("base color: %w", err))
		return
	}
	count, err := parseIntParam(q, "count")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	step, err := parseFloatParam(q, "step")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	colors, err := harmony.Generate(t, base, harmony.Options{Count: count, Step: step})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	pal := palette.New(colors)
	pal.Name = string(t)
	pal.Metadata.Method = "harmony"
	pal.Metadata.Source = "harmony:" + string(t)
	pal.Metadata.Params = map[string]any{
		"type":  string(t),
		"base":  base.Hex(),
		"count": count,
		"step":  step,
	}
	params := map[string]any{
		"type":  string(t),
		"base":  baseStr,
		"count": count,
		"step":  step,
	}
	writePaletteJSON(w, "harmony.generate", params, pal)
}

// ---------- /gradient ----------

func handleGradient(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from := strings.TrimSpace(q.Get("from"))
	to := strings.TrimSpace(q.Get("to"))
	stopsRaw := strings.TrimSpace(q.Get("stops"))
	positionsRaw := strings.TrimSpace(q.Get("positions"))
	stepsStr := strings.TrimSpace(q.Get("steps"))
	space := strings.ToLower(strings.TrimSpace(q.Get("space")))
	easing := strings.ToLower(strings.TrimSpace(q.Get("easing")))

	if stepsStr == "" {
		writeError(w, http.StatusBadRequest, errors.New("steps parameter is required"))
		return
	}
	steps, err := strconv.Atoi(stepsStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("steps: %w", err))
		return
	}
	opts := gradient.Options{
		Steps:  steps,
		Space:  gradient.Space(space),
		Easing: gradient.Easing(easing),
	}

	var colors []color.Color
	switch {
	case stopsRaw != "":
		stopStrs := strings.Split(stopsRaw, ",")
		if len(stopStrs) < 2 {
			writeError(w, http.StatusBadRequest, errors.New("stops requires ≥ 2 comma-separated colors"))
			return
		}
		stops, perr := parseColorList(stopStrs)
		if perr != nil {
			writeError(w, http.StatusBadRequest, perr)
			return
		}
		// Positioned stops: an optional comma-separated 0..1 list parallel
		// to stops=. Omitting it keeps the even-spacing default.
		if positionsRaw != "" {
			positions, pperr := parsePositionList(positionsRaw)
			if pperr != nil {
				writeError(w, http.StatusBadRequest, pperr)
				return
			}
			colors, err = gradient.MultiStopAt(stops, positions, opts)
		} else {
			colors, err = gradient.MultiStop(stops, opts)
		}
	case from != "" && to != "":
		fromC, ferr := color.Parse(from)
		if ferr != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("from: %w", ferr))
			return
		}
		toC, terr := color.Parse(to)
		if terr != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("to: %w", terr))
			return
		}
		colors, err = gradient.Build(fromC, toC, opts)
	default:
		writeError(w, http.StatusBadRequest, errors.New("provide either {from, to} or stops (≥ 2 comma-separated colors)"))
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	pal := palette.New(colors)
	pal.Name = "gradient"
	pal.Metadata.Method = "gradient"
	pal.Metadata.Params = map[string]any{
		"steps":  steps,
		"space":  string(opts.Space),
		"easing": string(opts.Easing),
	}
	params := map[string]any{
		"from":      from,
		"to":        to,
		"stops":     stopsRaw,
		"positions": positionsRaw,
		"steps":     steps,
		"space":     space,
		"easing":    easing,
	}
	writePaletteJSON(w, "gradient.generate", params, pal)
}

// ---------- /contrast ----------

type contrastResult struct {
	WCAG21 *contrast.WCAG21Result `json:"wcag21,omitempty"`
	APCA   *contrast.APCAResult   `json:"apca,omitempty"`
}

func handleContrastCheck(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	fgStr := strings.TrimSpace(q.Get("fg"))
	bgStr := strings.TrimSpace(q.Get("bg"))
	algo := strings.ToLower(strings.TrimSpace(q.Get("algo")))
	if fgStr == "" || bgStr == "" {
		writeError(w, http.StatusBadRequest, errors.New("fg and bg are required"))
		return
	}
	fg, err := color.Parse(fgStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("fg: %w", err))
		return
	}
	bg, err := color.Parse(bgStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("bg: %w", err))
		return
	}
	if algo == "" {
		algo = string(contrast.AlgoWCAG21)
	}
	res := contrastResult{}
	switch algo {
	case string(contrast.AlgoWCAG21):
		v := contrast.WCAG21(fg, bg)
		res.WCAG21 = &v
	case string(contrast.AlgoAPCA):
		v := contrast.APCA(fg, bg)
		res.APCA = &v
	case "both":
		wv := contrast.WCAG21(fg, bg)
		av := contrast.APCA(fg, bg)
		res.WCAG21 = &wv
		res.APCA = &av
	default:
		writeError(w, http.StatusBadRequest, fmt.Errorf("unknown algo %q (want wcag21|apca|both)", algo))
		return
	}
	writeEnvelope(w, "contrast.check", map[string]any{"fg": fgStr, "bg": bgStr, "algo": algo}, res)
}

// ---------- /random ----------

func handlePaletteRandom(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	count, err := parseIntParam(q, "count")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	seed, err := parseUint64Param(q, "seed")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	htype := strings.ToLower(strings.TrimSpace(q.Get("harmony")))

	var pal *palette.Palette
	if htype == "" || htype == "none" {
		pal = palette.Random(palette.RandomOptions{Count: count, Seed: seed})
		pal.Name = "random"
	} else {
		seedPal := palette.Random(palette.RandomOptions{Count: 1, Seed: seed})
		base := seedPal.Colors[0]
		t, terr := resolveHarmonyType(htype)
		if terr != nil {
			writeError(w, http.StatusBadRequest, terr)
			return
		}
		hcolors, herr := harmony.Generate(t, base, harmony.Options{Count: count})
		if herr != nil {
			writeError(w, http.StatusBadRequest, herr)
			return
		}
		pal = palette.New(hcolors)
		pal.Name = "random-" + string(t)
		pal.Metadata.Method = "random+harmony"
		pal.Metadata.Source = "random:" + string(t)
		pal.Metadata.Params = map[string]any{
			"count":   count,
			"seed":    seed,
			"harmony": string(t),
			"base":    base.Hex(),
		}
	}
	if pal.Len() == 0 {
		writeError(w, http.StatusInternalServerError, errors.New("produced empty palette"))
		return
	}
	params := map[string]any{"count": count, "seed": seed, "harmony": htype}
	writePaletteJSON(w, "palette.random", params, pal)
}

// ---------- /export ----------

// exportFormats is the set of formats POST /export accepts — every
// exporter.AllFormats entry. Built once from the exporter's canonical
// list so a newly added format needs no change here.
var exportFormats = func() map[exporter.Format]bool {
	m := make(map[exporter.Format]bool, len(exporter.AllFormats))
	for _, f := range exporter.AllFormats {
		m[f] = true
	}
	return m
}()

type exportRequest struct {
	Format string   `json:"format"`
	Colors []string `json:"colors"`
	Name   string   `json:"name,omitempty"`
	Kind   string   `json:"kind,omitempty"`
	Shades int      `json:"shades,omitempty"`
}

type exportResult struct {
	Format string `json:"format"`
	Kind   string `json:"kind,omitempty"`
	// Content is the rendered output. For binary formats (png/jpeg) it is
	// base64-encoded and Encoding is set to "base64"; text formats carry
	// the raw rendering and omit Encoding.
	Content  string `json:"content"`
	Encoding string `json:"encoding,omitempty"`
	Filename string `json:"filename"`
}

// handleExport renders a color list into any exporter format. Binary
// formats are base64-encoded so the JSON envelope stays valid text.
func handleExport(w http.ResponseWriter, r *http.Request) {
	var req exportRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(req.Colors) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("no colors provided"))
		return
	}
	format := exporter.Format(strings.ToLower(strings.TrimSpace(req.Format)))
	if format == "" {
		writeError(w, http.StatusBadRequest, errors.New("format is required"))
		return
	}
	if !exportFormats[format] {
		writeError(w, http.StatusBadRequest, fmt.Errorf("unknown format %q", format))
		return
	}
	// `kind` is the CSS-family dialect alias kept for parity with
	// /export/css; it only refines format=css and is rejected elsewhere.
	kind := strings.TrimSpace(req.Kind)
	if kind != "" {
		if format != exporter.FormatCSS {
			writeError(w, http.StatusBadRequest,
				fmt.Errorf("kind is only valid with format=css (got format=%q)", format))
			return
		}
		var err error
		if format, err = cssKindToFormat(kind); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	if req.Shades < 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("shades must be ≥ 0 (got %d)", req.Shades))
		return
	}
	cs, err := parseColorList(req.Colors)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "color"
	}
	pal := palette.New(cs)
	pal.Name = name
	data, err := exporter.Export(pal, format, exporter.Options{
		Prefix:         name,
		Name:           name,
		TailwindShades: req.Shades,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	res := exportResult{
		Format:   string(format),
		Content:  string(data),
		Filename: name + "." + exporter.FileExtension(format),
	}
	if exporter.IsBinary(format) {
		res.Content = base64.StdEncoding.EncodeToString(data)
		res.Encoding = "base64"
	}
	params := map[string]any{"format": req.Format, "colors": req.Colors, "name": name}
	if kind != "" {
		res.Kind = strings.ToLower(kind)
		params["kind"] = res.Kind
	}
	if req.Shades > 0 {
		params["shades"] = req.Shades
	}
	writeEnvelope(w, "export", params, res)
}

// ---------- /blindness/simulate ----------

type blindnessRequest struct {
	Colors []string `json:"colors"`
	Kind   string   `json:"kind,omitempty"`
}

type blindnessVariant struct {
	Kind   string               `json:"kind"`
	Colors []exporter.ColorJSON `json:"colors"`
}

type blindnessResult struct {
	Variants []blindnessVariant `json:"variants"`
}

func handleBlindnessSimulate(w http.ResponseWriter, r *http.Request) {
	var req blindnessRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(req.Colors) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("no colors provided"))
		return
	}
	cs, err := parseColorList(req.Colors)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	if kind == "" {
		kind = "all"
	}

	var variants []blindnessVariant
	if kind == "all" {
		all, serr := blindness.SimulateAll(cs)
		if serr != nil {
			writeError(w, http.StatusBadRequest, serr)
			return
		}
		variants = make([]blindnessVariant, 0, len(blindness.AllKinds))
		for _, k := range blindness.AllKinds {
			variants = append(variants, blindnessVariant{
				Kind:   string(k),
				Colors: exporter.EncodeColors(all[k]),
			})
		}
	} else {
		out, serr := blindness.SimulatePalette(cs, blindness.Kind(kind))
		if serr != nil {
			writeError(w, http.StatusBadRequest, serr)
			return
		}
		variants = []blindnessVariant{{
			Kind:   kind,
			Colors: exporter.EncodeColors(out),
		}}
	}

	res := blindnessResult{Variants: variants}
	writeEnvelope(w, "blindness.simulate", map[string]any{"colors": req.Colors, "kind": kind}, res)
}

// ---------- shared helpers ----------

// parseColorList parses a slice of color strings; returns the first parse
// failure annotated with its index for client-side debugging. Mirrors
// internal/mcp/tools.parseColors (kept independent so web and mcp do not
// import each other).
func parseColorList(in []string) ([]color.Color, error) {
	out := make([]color.Color, len(in))
	for i, raw := range in {
		c, err := color.Parse(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("colors[%d] %q: %w", i, raw, err)
		}
		out[i] = c
	}
	return out, nil
}

// parsePositionList parses a comma-separated list of gradient stop
// positions (each a 0..1 float). Ordering / endpoint validation is left
// to gradient.MultiStopAt so the rules live in one place.
func parsePositionList(raw string) ([]float64, error) {
	parts := strings.Split(raw, ",")
	out := make([]float64, len(parts))
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil, fmt.Errorf("positions[%d] %q: %w", i, p, err)
		}
		out[i] = v
	}
	return out, nil
}

// resolveHarmonyType matches user input against the canonical harmony
// names with the same aliases the CLI/MCP accept ("split", "double").
func resolveHarmonyType(raw string) (harmony.Type, error) {
	low := strings.ToLower(strings.TrimSpace(raw))
	switch low {
	case "split":
		return harmony.Split, nil
	case "double":
		return harmony.DoubleComplementary, nil
	}
	known := []harmony.Type{
		harmony.Complementary,
		harmony.Analogous,
		harmony.Triadic,
		harmony.Split,
		harmony.Tetradic,
		harmony.Square,
		harmony.DoubleComplementary,
		harmony.Compound,
		harmony.Monochromatic,
		harmony.Shades,
	}
	for _, t := range known {
		if string(t) == low {
			return t, nil
		}
	}
	names := make([]string, len(known))
	for i, t := range known {
		names[i] = string(t)
	}
	return "", fmt.Errorf("unknown harmony type %q (known: %s)", raw, strings.Join(names, ", "))
}

// cssKindToFormat resolves the CSS dialect alias used by both the CLI's
// `--kind` flag and the MCP `export.css` tool input.
func cssKindToFormat(kind string) (exporter.Format, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "vars", "var", "css":
		return exporter.FormatCSS, nil
	case "scss", "sass":
		return exporter.FormatSCSS, nil
	case "less":
		return exporter.FormatLESS, nil
	}
	return "", fmt.Errorf("unknown kind %q (want vars|scss|less)", kind)
}

// parseIntParam reads a non-negative int from query params. Empty value
// returns 0 with no error, matching the "optional, default zero" idiom
// used across handlers (count, shades, ...).
func parseIntParam(q map[string][]string, key string) (int, error) {
	raw := strings.TrimSpace(firstQuery(q, key))
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return v, nil
}

func parseUint64Param(q map[string][]string, key string) (uint64, error) {
	raw := strings.TrimSpace(firstQuery(q, key))
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return v, nil
}

func parseFloatParam(q map[string][]string, key string) (float64, error) {
	raw := strings.TrimSpace(firstQuery(q, key))
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return v, nil
}

func firstQuery(q map[string][]string, key string) string {
	if v := q[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}
