package render

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Sour16o4/profilegen/internal/theme"
)

// DefaultChips is the stack row (§5 item 9).
var DefaultChips = []string{
	"go", "grpc", "graphql", "rest", "docker", "kubernetes",
	"postgresql", "mysql", "redis", "argocd",
}

// chipCharPx is a measured value, not an estimate: getComputedTextLength()
// against the real generic mono stack (ui-monospace, SF Mono, Menlo,
// monospace) in Chromium/Linux gave a flat 6.90px/char at 11.5px for every
// word in DefaultChips (a plain monospace font, so per-char width is
// uniform by construction). Rounded up to 7.2px/char (~4% headroom) since
// GitHub viewers land on different platform fallback fonts (Segoe UI Mono,
// Courier New, etc.) than this build machine, and "postgresql" — the
// longest chip — must never overflow its border on any of them.
const chipCharPx = 7.2

const (
	chipPadX     = 12.0
	chipPadY     = 5.0
	chipGap      = 10.0
	chipRowGap   = 10.0
	chipFontSize = 11.5
)

// chipWidth uses rune count, not byte length — this project's chip words are
// all ASCII today, but every other character-count layout estimate in this
// codebase (footer separators, message truncation) has to use
// utf8.RuneCountInString on principle, since len() silently miscounts as
// soon as a multi-byte character shows up. Consistency here means the next
// person who adds a non-ASCII chip doesn't reintroduce that bug.
func chipWidth(word string) float64 {
	return float64(utf8.RuneCountInString(word))*chipCharPx + 2*chipPadX
}

// Chips renders the wrapping stack-chip row (§5 item 9). No hover state, no
// motion beyond the shared entrance the rest of the page uses.
func Chips(t theme.Tokens, words []string) string {
	chipH := chipFontSize + 2*chipPadY

	type placed struct {
		word string
		w    float64
		x, y float64
	}
	var rows [][]placed
	var row []placed
	x := 0.0
	for _, word := range words {
		w := chipWidth(word)
		if x+w > contentW && len(row) > 0 {
			rows = append(rows, row)
			row = nil
			x = 0
		}
		row = append(row, placed{word, w, x, 0})
		x += w + chipGap
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	height := topPad + float64(len(rows))*(chipH+chipRowGap) - chipRowGap + botPad*0.4

	var b strings.Builder
	fmt.Fprintf(&b, `<svg width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" xmlns="http://www.w3.org/2000/svg" role="img" aria-labelledby="ch-title ch-desc">`+"\n",
		contentW, height, contentW, height)
	b.WriteString(`<title id="ch-title">Stack</title>` + "\n")
	fmt.Fprintf(&b, `<desc id="ch-desc">%s</desc>`+"\n", strings.Join(words, ", "))

	writeChipsStyle(&b, t, len(words))
	fmt.Fprintf(&b, `<rect width="%.0f" height="%.0f" fill="%s"/>`+"\n", contentW, height, t.Ground)

	idx := 0
	for ri, r := range rows {
		y := topPad + float64(ri)*(chipH+chipRowGap)
		for _, c := range r {
			writeChip(&b, t, c.word, c.x, y, c.w, chipH, idx)
			idx++
		}
	}

	b.WriteString("</svg>\n")
	return b.String()
}

func writeChipsStyle(b *strings.Builder, t theme.Tokens, n int) {
	b.WriteString(`<style>
.mono{font-family:` + theme.Fonts.Mono + `}
.chip-entrance{opacity:1;transform:translateY(0)}
@media (prefers-reduced-motion: no-preference){
  .chip-entrance{animation:chipIn .6s cubic-bezier(.32,.72,0,1) both}
}
@keyframes chipIn{
  0%{opacity:0;transform:translateY(16px)}
  100%{opacity:1;transform:translateY(0)}
}
</style>
`)
	// index-suffixed delays: one rule per chip rather than baking distinct
	// keyframes per chip — same shape, only the delay differs, so no
	// per-instance @keyframes needed (unlike the projects.go progress bars,
	// where the animated *value* itself differed per instance).
	fmt.Fprintf(b, `<style>@media (prefers-reduced-motion: no-preference){`)
	for i := 0; i < n; i++ {
		fmt.Fprintf(b, `.c%d{animation-delay:%.2fs}`, i, float64(i)*0.03)
	}
	b.WriteString(`}</style>` + "\n")
}

func writeChip(b *strings.Builder, t theme.Tokens, word string, x, y, w, h float64, idx int) {
	fmt.Fprintf(b, `<g class="chip-entrance c%d">`+"\n", idx)
	fmt.Fprintf(b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="none" stroke="%s" stroke-width="1.5"/>`+"\n",
		x, y, w, h, t.Line)
	fmt.Fprintf(b, `<text class="mono" x="%.1f" y="%.1f" font-size="%.1f" fill="%s">%s</text>`+"\n",
		x+chipPadX, y+h/2+chipFontSize*0.35, chipFontSize, t.Mute, word)
	b.WriteString(`</g>` + "\n")
}
