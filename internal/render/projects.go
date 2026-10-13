package render

import (
	"fmt"
	"strings"

	"github.com/Sour16o4/profilegen/internal/projects"
	"github.com/Sour16o4/profilegen/internal/theme"
)

// charFactor is the same rough generic-sans advance-width estimate used
// throughout (0.5em/char is a bit tighter than the mono 0.6em estimate,
// appropriate for a proportional face).
const charFactor = 0.5

// wrapText greedily wraps s into lines that fit maxWidth at fontSize, using
// the generic-stack width estimate — approximate by nature, generous enough
// that real glyphs never overflow it.
func wrapText(s string, maxWidth, fontSize float64) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}
	charW := fontSize * charFactor
	var lines []string
	cur := words[0]
	curW := float64(len(cur)) * charW
	for _, w := range words[1:] {
		wordW := float64(len(w)) * charW
		spaceW := charW
		if curW+spaceW+wordW <= maxWidth {
			cur += " " + w
			curW += spaceW + wordW
		} else {
			lines = append(lines, cur)
			cur = w
			curW = wordW
		}
	}
	lines = append(lines, cur)
	return lines
}

const (
	cardPad    = 18.0
	statusSize = 10.0
	descLineH  = 18.0
	metaH      = 20.0 // height reserved for the language/last-push metadata line
)

func entranceClass(i int) string {
	delays := []string{"e0", "e1", "e2"}
	return delays[i%len(delays)]
}

// ProjectCards renders one standalone SVG per project, all the same height,
// so the README can wrap each in its own link — links inside an SVG don't
// work once GitHub serves it as an image, but a link around the image does.
// Cards are sized to sit n-across (contentW/n each). Only active cards get
// the status square and the language/last-push line.
func ProjectCards(t theme.Tokens, items []projects.Project, showMeta bool) []string {
	n := len(items)
	svgW := contentW / float64(n)
	const padL, padR, padT, padB = 10.0, 14.0, 8.0, 14.0
	cardW := svgW - padL - padR

	lines := make([][]string, n)
	maxLines := 0
	for i, p := range items {
		lines[i] = wrapText(p.Blurb, cardW-2*cardPad, 13)
		if len(lines[i]) > maxLines {
			maxLines = len(lines[i])
		}
	}
	metaBlockH := 0.0
	if showMeta {
		metaBlockH = 14 + metaH
	}
	cardH := cardPad + 26 + float64(maxLines)*descLineH + metaBlockH + cardPad
	height := padT + cardH + padB

	out := make([]string, n)
	for i, p := range items {
		var b strings.Builder
		fmt.Fprintf(&b, `<svg width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" xmlns="http://www.w3.org/2000/svg" role="img" aria-labelledby="pr-title">`+"\n",
			svgW, height, svgW, height)
		fmt.Fprintf(&b, `<title id="pr-title">%s</title>`+"\n", p.Name)
		writeProjectsStyle(&b, t)
		fmt.Fprintf(&b, `<rect width="%.0f" height="%.0f" fill="%s"/>`+"\n", svgW, height, t.Ground)
		writeCard(&b, t, padL, padT, cardW, cardH, p, lines[i], showMeta, i)
		b.WriteString("</svg>\n")
		out[i] = b.String()
	}
	return out
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func writeProjectsStyle(b *strings.Builder, t theme.Tokens) {
	fmt.Fprintf(b, `<style>
text{font-family:%s}
.mono{font-family:%s}
.card-title{font-weight:700;letter-spacing:.02em}
.card-desc{fill:%s}
.card-shadow{fill:%s;transform:translate(4px,4px)}
.card-meta{fill:%s}
.entrance{opacity:1;transform:translate(0,0) scale(1)}

@media (prefers-reduced-motion: no-preference){
  .entrance{animation:cardIn .72s cubic-bezier(.32,.72,0,1) forwards}
  .e0{animation-delay:.16s}
  .e1{animation-delay:.24s}
  .e2{animation-delay:.32s}
  @keyframes cardIn{
    0%%{opacity:0;transform:translate(0,22px) scale(.97)}
    100%%{opacity:1;transform:translate(0,0) scale(1)}
  }
}
</style>
`,
		theme.Fonts.Sans, theme.Fonts.Mono, t.Body, t.Accent, t.Mute,
	)
}

func writeCard(b *strings.Builder, t theme.Tokens, x, y, w, h float64, p projects.Project, descLines []string, showMeta bool, idx int) {
	cls := entranceClass(idx)
	fmt.Fprintf(b, `<g class="entrance %s">`+"\n", cls)
	fmt.Fprintf(b, `<rect class="card-shadow" x="%.1f" y="%.1f" width="%.1f" height="%.1f"/>`+"\n", x, y, w, h)
	fmt.Fprintf(b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s" stroke="%s" stroke-width="2"/>`+"\n",
		x, y, w, h, t.Card, t.Bone)

	titleX := x + cardPad
	if showMeta {
		fmt.Fprintf(b, `<rect x="%.1f" y="%.1f" width="%.0f" height="%.0f" fill="%s"/>`+"\n",
			titleX, y+cardPad+2, statusSize, statusSize, t.Accent)
		titleX += statusSize + 10
	}
	fmt.Fprintf(b, `<text class="card-title" x="%.1f" y="%.1f" font-size="14" fill="%s">%s</text>`+"\n",
		titleX, y+cardPad+11, t.Text, strings.ToUpper(p.Name))

	descY := y + cardPad + 26 + 14
	for i, line := range descLines {
		fmt.Fprintf(b, `<text class="card-desc" x="%.1f" y="%.1f" font-size="13">%s</text>`+"\n",
			x+cardPad, descY+float64(i)*descLineH, line)
	}

	if showMeta {
		metaY := descY + float64(len(descLines)-1)*descLineH + 24
		// Real, checkable facts — language and last-push date come straight
		// from the repo, same fields shown in the verified-repo table this
		// was built from. No percentage: there was nothing behind the old
		// one.
		status := strings.ToUpper(p.Status)
		meta := fmt.Sprintf("%s · %s · pushed %s", status, p.Language, p.LastPush)
		fmt.Fprintf(b, `<text class="mono card-meta" x="%.1f" y="%.1f" font-size="11">%s</text>`+"\n",
			x+cardPad, metaY, meta)
	}

	b.WriteString(`</g>` + "\n")
}
