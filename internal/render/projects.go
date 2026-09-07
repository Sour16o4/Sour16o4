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

// Projects renders either the "working on now" (active) or "shipped"
// section, sharing one card component. Only active cards get the status
// square and the language/last-push metadata line (§5 items 3 & 6) — real,
// checkable facts, not a progress percentage with nothing behind it.
// n=1 (e.g. "now" with a single active project) naturally spans the full
// width: cardW = usableW when there's only one item, no special case needed.
func Projects(t theme.Tokens, title string, items []projects.Project, showMeta bool) string {
	n := len(items)
	gap := 24.0
	cardW := (usableW() - float64(n-1)*gap) / float64(n)

	descMaxW := cardW - 2*cardPad
	type laidOut struct {
		p     projects.Project
		lines []string
	}
	laid := make([]laidOut, n)
	maxLines := 0
	for i, p := range items {
		lines := wrapText(p.Blurb, descMaxW, 13)
		laid[i] = laidOut{p, lines}
		if len(lines) > maxLines {
			maxLines = len(lines)
		}
	}

	titleH := 26.0
	descH := float64(maxLines) * descLineH
	metaBlockH := 0.0
	if showMeta {
		metaBlockH = 14 + metaH
	}
	cardH := cardPad + titleH + descH + metaBlockH + cardPad
	height := topPad + cardH + botPad*0.4

	var b strings.Builder
	fmt.Fprintf(&b, `<svg width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" xmlns="http://www.w3.org/2000/svg" role="img" aria-labelledby="pr-title pr-desc">`+"\n",
		contentW, height, contentW, height)
	fmt.Fprintf(&b, `<title id="pr-title">%s</title>`+"\n", title)
	fmt.Fprintf(&b, `<desc id="pr-desc">%s: %d project card%s.</desc>`+"\n", title, n, plural(n))

	writeProjectsStyle(&b, t)
	fmt.Fprintf(&b, `<rect width="%.0f" height="%.0f" fill="%s"/>`+"\n", contentW, height, t.Ground)

	for i, lo := range laid {
		x := contentPad + float64(i)*(cardW+gap)
		writeCard(&b, t, x, topPad, cardW, cardH, lo.p, lo.lines, showMeta, i)
	}

	b.WriteString("</svg>\n")
	return b.String()
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
