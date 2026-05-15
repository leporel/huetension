package library

import (
	"strings"
	"unicode"
)

// SlugifyCategory derives the URL-safe slug for a category display
// name. Lowercase, alphanumerics + dash; runs of separators collapsed;
// edges trimmed. Stable: the same display name always produces the
// same slug, across runs and across processes — REST routing and MCP
// filter inputs both key off this. Unicode letters/digits survive
// (so a future "Café" category would slug as "café" rather than ""),
// but nothing else does.
//
// "Brand & Tech"            -> "brand-tech"
// "Editorial / Publishing"  -> "editorial-publishing"
// "Theme-Factory"           -> "theme-factory"
// "Vintage / Retro"         -> "vintage-retro"
func SlugifyCategory(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	prevSep := true // start of string acts as a separator → no leading dash
	for _, r := range name {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
			prevSep = false
		default:
			if !prevSep {
				b.WriteByte('-')
				prevSep = true
			}
		}
	}
	out := b.String()
	return strings.TrimRight(out, "-")
}
