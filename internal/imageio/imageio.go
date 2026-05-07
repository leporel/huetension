// Package imageio loads images from a variety of sources (file paths, http
// URLs, stdin, raw bytes, data URIs) and exposes the helpers extract needs:
// pixel iteration with optional alpha-mask filtering, and aspect-preserving
// resize through bild.
//
// Image format support comes from the standard library's image/jpeg,
// image/png, image/gif decoders plus the bmp, tiff, and webp decoders in
// golang.org/x/image. Decoders register themselves via blank imports.
package imageio

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/gif"  // register decoder
	_ "image/jpeg" // register decoder
	_ "image/png"  // register decoder
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/anthonynsimon/bild/transform"
	_ "golang.org/x/image/bmp"  // register decoder
	_ "golang.org/x/image/tiff" // register decoder
	_ "golang.org/x/image/webp" // register decoder

	"github.com/leporel/huetension/internal/color"
)

// SourceType labels how an image was loaded.
type SourceType string

const (
	SourceFile    SourceType = "file"
	SourceURL     SourceType = "url"
	SourceStdin   SourceType = "stdin"
	SourceData    SourceType = "data_uri"
	SourceBytes   SourceType = "bytes"
	SourceUnknown SourceType = "unknown"
)

// Loaded carries the decoded image plus the bookkeeping metadata that
// extract needs to populate palette.ImageInfo.
type Loaded struct {
	Image      image.Image
	Format     string
	Source     string
	SourceType SourceType
	HasAlpha   bool
}

// LoadOptions tunes Load. Zero values default to 64 MiB / 30 s.
type LoadOptions struct {
	MaxBytes   int64
	Timeout    time.Duration
	HTTPClient *http.Client
	// AllowedHosts, when non-nil and non-empty, restricts URL fetching to
	// hosts that match one of the given suffixes (e.g. "*.unsplash.com" or
	// the bare host "raw.githubusercontent.com"). Use the MCP layer to wire
	// this from config; the CLI leaves it unset.
	AllowedHosts []string
}

// Defaults applied when LoadOptions has zero values for the corresponding fields.
const (
	DefaultMaxBytes = 64 * 1024 * 1024
	DefaultTimeout  = 30 * time.Second
)

// Load decodes an image from a string source descriptor. Recognised forms:
//
//	"path/to/file.png"      — local file
//	"http://..."            — HTTP fetch (uses opts.HTTPClient or http.DefaultClient)
//	"https://..."           — HTTPS fetch
//	"-"                     — read from stdin
//	"data:image/png;base64,…" — RFC 2397 data URI (supports base64; raw URL-encoded payloads not supported)
//
// Returned errors wrap the underlying cause and include the source string for
// diagnostics.
func Load(source string, opts LoadOptions) (*Loaded, error) {
	switch {
	case strings.HasPrefix(source, "data:"):
		img, format, err := loadDataURI(source, maxBytes(opts))
		if err != nil {
			return nil, fmt.Errorf("imageio: data uri: %w", err)
		}
		return finalise(img, format, source, SourceData), nil

	case strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://"):
		img, format, err := loadURL(source, opts)
		if err != nil {
			return nil, fmt.Errorf("imageio: %s: %w", source, err)
		}
		return finalise(img, format, source, SourceURL), nil

	case source == "-":
		img, format, err := loadReader(os.Stdin, maxBytes(opts))
		if err != nil {
			return nil, fmt.Errorf("imageio: stdin: %w", err)
		}
		return finalise(img, format, "stdin", SourceStdin), nil
	}

	img, format, err := loadFile(source, maxBytes(opts))
	if err != nil {
		return nil, fmt.Errorf("imageio: %s: %w", source, err)
	}
	return finalise(img, format, source, SourceFile), nil
}

// LoadBytes decodes an image from raw bytes. Used by MCP's `image.extract`
// when the caller passes the image inline as base64.
func LoadBytes(data []byte) (*Loaded, error) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("imageio: decode bytes: %w", err)
	}
	return finalise(img, format, "<bytes>", SourceBytes), nil
}

func finalise(img image.Image, format, source string, st SourceType) *Loaded {
	return &Loaded{
		Image:      img,
		Format:     format,
		Source:     source,
		SourceType: st,
		HasAlpha:   detectAlpha(img),
	}
}

// detectAlpha returns true if the image's color model carries alpha
// information (RGBA, NRGBA, NYCbCrA, …).
func detectAlpha(img image.Image) bool {
	switch img.(type) {
	case *image.RGBA, *image.RGBA64, *image.NRGBA, *image.NRGBA64, *image.NYCbCrA, *image.Alpha, *image.Alpha16:
		return true
	}
	return false
}

func loadFile(path string, max int64) (image.Image, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	info, err := f.Stat()
	if err == nil && max > 0 && info.Size() > max {
		return nil, "", fmt.Errorf("file size %d exceeds limit %d", info.Size(), max)
	}

	return loadReader(f, max)
}

func loadReader(r io.Reader, max int64) (image.Image, string, error) {
	if max > 0 {
		r = io.LimitReader(r, max+1)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, "", err
	}
	if max > 0 && int64(len(data)) > max {
		return nil, "", fmt.Errorf("payload exceeds %d bytes", max)
	}
	return image.Decode(bytes.NewReader(data))
}

func loadURL(rawURL string, opts LoadOptions) (image.Image, string, error) {
	if len(opts.AllowedHosts) > 0 {
		if err := checkHost(rawURL, opts.AllowedHosts); err != nil {
			return nil, "", err
		}
	}

	client := opts.HTTPClient
	if client == nil {
		timeout := opts.Timeout
		if timeout == 0 {
			timeout = DefaultTimeout
		}
		client = &http.Client{Timeout: timeout}
	}

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "huetension/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("http %d", resp.StatusCode)
	}
	return loadReader(resp.Body, maxBytes(opts))
}

func checkHost(rawURL string, allowed []string) error {
	return CheckHost(rawURL, allowed)
}

// CheckHost validates rawURL's host against the allowed patterns. Patterns
// may be a literal hostname or a "*.suffix" form. Returns nil on a match,
// or an error naming the offending host. Exposed so the MCP layer can
// re-apply this check on HTTP redirects (the SDK redirect handler runs
// after loadURL's initial gate).
func CheckHost(rawURL string, allowed []string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	host := strings.ToLower(u.Hostname())
	for _, pat := range allowed {
		pat = strings.ToLower(strings.TrimSpace(pat))
		switch {
		case pat == "":
			continue
		case strings.HasPrefix(pat, "*."):
			suffix := pat[1:] // ".example.com"
			if strings.HasSuffix(host, suffix) {
				return nil
			}
		case host == pat:
			return nil
		}
	}
	return fmt.Errorf("host %q not in allowed list", host)
}

func loadDataURI(s string, max int64) (image.Image, string, error) {
	// data:[<mediatype>][;base64],<data>
	rest := strings.TrimPrefix(s, "data:")
	comma := strings.IndexByte(rest, ',')
	if comma < 0 {
		return nil, "", errors.New("missing comma in data URI")
	}
	meta := rest[:comma]
	payload := rest[comma+1:]

	if !strings.Contains(meta, "base64") {
		return nil, "", errors.New("non-base64 data URIs are not supported")
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, "", fmt.Errorf("base64 decode: %w", err)
	}
	if max > 0 && int64(len(data)) > max {
		return nil, "", fmt.Errorf("payload exceeds %d bytes", max)
	}
	return image.Decode(bytes.NewReader(data))
}

func maxBytes(opts LoadOptions) int64 {
	if opts.MaxBytes > 0 {
		return opts.MaxBytes
	}
	return DefaultMaxBytes
}

// Resize rescales img so its longest side is at most `side` pixels, preserving
// aspect ratio. side <= 0 returns img unchanged. Smaller images are NOT
// up-scaled.
func Resize(img image.Image, side int) image.Image {
	if side <= 0 {
		return img
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= side && h <= side {
		return img
	}
	var nw, nh int
	if w >= h {
		nw = side
		nh = h * side / w
	} else {
		nh = side
		nw = w * side / h
	}
	if nh < 1 {
		nh = 1
	}
	if nw < 1 {
		nw = 1
	}
	return transform.Resize(img, nw, nh, transform.Linear)
}

// Pixels collects every pixel of img into a flat slice of huetension Colors.
// Pixels with alpha strictly below `alphaThreshold` are skipped — pass 0 to
// keep everything (the alpha channel itself is still preserved on each
// returned Color).
func Pixels(img image.Image, alphaThreshold uint8) []color.Color {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	out := make([]color.Color, 0, w*h)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.FromImageColor(img.At(x, y))
			if c.A < alphaThreshold {
				continue
			}
			out = append(out, c)
		}
	}
	return out
}

// Stats summarises a Pixels run for metadata reporting.
type Stats struct {
	TotalPixels int
	ValidPixels int
}

// PixelsWithStats is Pixels + a count of total pixels (for the alpha-mask
// rejection ratio reported in metadata).
func PixelsWithStats(img image.Image, alphaThreshold uint8) ([]color.Color, Stats) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	total := w * h
	pixels := Pixels(img, alphaThreshold)
	return pixels, Stats{TotalPixels: total, ValidPixels: len(pixels)}
}
