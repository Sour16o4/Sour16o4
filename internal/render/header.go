package render

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/Sour16o4/profilegen/internal/theme"
)

// namePathD is "SOURAV SALAMPURIA" set in Space Grotesk Bold at 38px with
// -0.04em letter-spacing, converted to a path at build time (see
// cmd/fontcompare — measure.js). This is the font-question decision: paths
// for the one large display string, generic stack for everything else.
// Regenerate by re-running the Node script if the name or size ever changes.
//
//go:embed assets/name-sourav-salampuria.path
var namePathD string

// nameWidthPx is the measured advance width of namePathD (opentype.js,
// unitsPerEm=1000, includes letter-spacing). Used to lay out what comes
// after the name.
const nameWidthPx = 363.13

const (
	avatarSize = 64.0
	headerPad  = 24.0
)

// TypingLine is one phrase in the header's cycling typing line.
type TypingLine struct {
	Text string
}

// DefaultTyping is the three lines the header cycles through (§5 item 1).
// The third is what the reduced-motion static frame must show complete.
var DefaultTyping = []TypingLine{
	{"building relay — provider failover"},
	{"shipping tenantguard v0.2"},
	{"backend engineer, gurgaon in"},
}

const tickerString = "GO — GRPC — KUBERNETES — POSTGRES — ARGOCD — GRAPHQL — DOCKER — REDIS — "

// monoCharPx is a rough JetBrains-Mono advance-width estimate (0.6em) used
// only to size CSS clip rects — approximate is fine, it never has to be
// pixel-exact, just wide enough to never clip a fully "typed" line.
func monoCharPx(size float64) float64 { return size * 0.6 }

// Header renders the header + full-bleed ticker as one section, per §5
// items 1-2 and §6.3. Motion is CSS keyframes only (no SMIL — the arch
// strip is why), gated by prefers-reduced-motion with a meaningful static
// frame: the third typing line shown complete, not a half-typed fragment.
func Header(t theme.Tokens, name string, subtitle string, lines []TypingLine, useNamePath bool) string {
	headerH := 150.0
	tickerH := 44.0
	height := headerH + tickerH

	var b strings.Builder
	fmt.Fprintf(&b, `<svg width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" xmlns="http://www.w3.org/2000/svg" role="img" aria-labelledby="hd-title hd-desc">`+"\n",
		contentW, height, contentW, height)
	fmt.Fprintf(&b, `<title id="hd-title">%s</title>`+"\n", name)
	fmt.Fprintf(&b, `<desc id="hd-desc">%s. %s.</desc>`+"\n", name, subtitle)

	writeHeaderStyle(&b, t, lines)

	fmt.Fprintf(&b, `<rect width="%.0f" height="%.0f" fill="%s"/>`+"\n", contentW, height, t.Ground)

	writeAvatar(&b, t)
	writeName(&b, t, name, useNamePath)
	writeSubtitle(&b, t, subtitle)
	writeTyping(&b, t, lines, headerH-34)

	writeTicker(&b, t, headerH, tickerH)

	b.WriteString("</svg>\n")
	return b.String()
}

func writeHeaderStyle(b *strings.Builder, t theme.Tokens, lines []TypingLine) {
	fmt.Fprintf(b, `<style>
text{font-family:%s}
.mono{font-family:%s}
.riseA,.riseB,.riseC{opacity:1;transform:translateY(0)}
.avatar-shadow{fill:%s;transform:translate(3px,3px)}
.cursor{fill:%s}

@media (prefers-reduced-motion: no-preference){
  .riseA,.riseB,.riseC{animation:riseFade .6s cubic-bezier(.32,.72,0,1) both}
  .riseA{animation-delay:0s}
  .riseB{animation-delay:.08s}
  .riseC{animation-delay:.14s}
  @keyframes riseFade{
    0%%{opacity:0;transform:translateY(16px)}
    100%%{opacity:1;transform:translateY(0)}
  }
  .avatar-shadow{animation:avatarBreath 5.5s ease-in-out infinite}
  @keyframes avatarBreath{
    0%%{transform:translate(3px,3px)}
    50%%{transform:translate(5px,5px)}
    100%%{transform:translate(3px,3px)}
  }
  .cursor{animation:cursorBlink 1.05s step-end infinite}
  @keyframes cursorBlink{ 0%%,50%%{opacity:1} 50.01%%,100%%{opacity:0} }
}
</style>
`,
		theme.Fonts.Sans, theme.Fonts.Mono, t.Accent, t.Accent2,
	)
	writeTypingKeyframes(b, lines)
}

// writeAvatar wraps its content in an unpositioned "riseA" <g> for the
// entrance animation, and keeps the shadow's own persistent offset on a
// separate inner element — CSS `transform` fully replaces (does not compose
// with) an element's own `transform`/positioning attribute, so an
// animation class and a positioning transform can never share one element.
func writeAvatar(b *strings.Builder, t theme.Tokens) {
	x, y := headerPad, headerPad
	b.WriteString(`<g class="riseA">` + "\n")
	fmt.Fprintf(b, `<rect class="avatar-shadow" x="%.1f" y="%.1f" width="%.0f" height="%.0f"/>`+"\n",
		x, y, avatarSize, avatarSize)
	fmt.Fprintf(b, `<rect x="%.1f" y="%.1f" width="%.0f" height="%.0f" fill="%s"/>`+"\n",
		x, y, avatarSize, avatarSize, t.Accent)
	fmt.Fprintf(b, `<text x="%.1f" y="%.1f" font-size="24" font-weight="700" fill="%s" text-anchor="middle">SS</text>`+"\n",
		x+avatarSize/2, y+avatarSize/2+9, t.Ground)
	b.WriteString(`</g>` + "\n")
}

func writeName(b *strings.Builder, t theme.Tokens, name string, useNamePath bool) {
	x := headerPad + avatarSize + 24
	y := headerPad + 32
	if useNamePath {
		// riseB (animation transform) on the outer g; positioning transform
		// on an inner g — same collision as the avatar shadow, fixed the
		// same way.
		fmt.Fprintf(b, `<g class="riseB"><g transform="translate(%.1f,%.1f)"><path d="%s" fill="%s"/></g></g>`+"\n",
			x, y, namePathD, t.Text)
		return
	}
	fmt.Fprintf(b, `<text class="riseB" x="%.1f" y="%.1f" font-size="38" font-weight="700" letter-spacing="-0.04em" fill="%s">%s</text>`+"\n",
		x, y, t.Text, name)
}

func writeSubtitle(b *strings.Builder, t theme.Tokens, subtitle string) {
	x := headerPad + avatarSize + 24
	y := headerPad + 58
	fmt.Fprintf(b, `<text class="riseB" x="%.1f" y="%.1f" font-size="14" fill="%s">%s</text>`+"\n",
		x, y, t.Mute, subtitle)
}

// writeTyping lays out three overlapping clip-revealed text elements that
// take turns on one shared master timeline, so only one is ever visible.
// The base (non-media) state shows line index 2 fully revealed — the
// meaningful static / reduced-motion frame the spec requires.
func writeTyping(b *strings.Builder, t theme.Tokens, lines []TypingLine, y float64) {
	x := headerPad + avatarSize + 24
	fontSize := 13.0
	charW := monoCharPx(fontSize)

	b.WriteString(`<g class="riseC">` + "\n")
	for i, l := range lines {
		w := charW * float64(len(l.Text))
		clipID := fmt.Sprintf("typeclip%d", i)
		// y=-16 h=22: text sits on the baseline (y=0) and its ascenders
		// extend upward into negative y — a clip rect starting at y=0
		// was cutting off nearly the entire glyph, leaving only descender
		// fragments visible. This was the actual bug behind what looked
		// like a width/animation problem.
		fmt.Fprintf(b, `<clipPath id="%s"><rect class="tclip%d" x="0" y="-16" width="%.1f" height="22"/></clipPath>`+"\n",
			clipID, i, w)
		visibleWidth := "0px"
		if i == len(lines)-1 {
			// static/no-animation default: last line shown complete
			visibleWidth = fmt.Sprintf("%.1fpx", w)
		}
		fmt.Fprintf(b, `<style>.tclip%d{width:%s}</style>`+"\n", i, visibleWidth)
		fmt.Fprintf(b, `<g transform="translate(%.1f,%.1f)" clip-path="url(#%s)"><text class="mono" x="0" y="0" font-size="%.0f" fill="%s">%s</text></g>`+"\n",
			x, y, clipID, fontSize, t.Accent2, l.Text)
	}
	maxW := 0.0
	for _, l := range lines {
		w := charW * float64(len(l.Text))
		if w > maxW {
			maxW = w
		}
	}
	fmt.Fprintf(b, `<rect class="cursor" x="%.1f" y="%.1f" width="2" height="15"/>`+"\n",
		x+maxW+4, y-13)
	b.WriteString(`</g>` + "\n")
}

// writeTypingKeyframes computes the shared master cycle (type → hold →
// delete, once per line, back to back) and emits one @keyframes block per
// line's clip-rect width, scoped to prefers-reduced-motion like everything
// else in this file.
func writeTypingKeyframes(b *strings.Builder, lines []TypingLine) {
	const (
		msPerCharType   = 56.0
		msPerCharDelete = 24.0
		holdMs          = 1900.0
	)
	fontSize := 13.0
	charW := monoCharPx(fontSize)

	type window struct {
		startPct, typedPct, holdEndPct, deleteEndPct float64
		widthPx                                      float64
		charCount                                    int
	}
	total := 0.0
	spans := make([]float64, len(lines))
	for i, l := range lines {
		n := float64(len(l.Text))
		d := n*msPerCharType + holdMs + n*msPerCharDelete
		spans[i] = d
		total += d
	}
	windows := make([]window, len(lines))
	t0 := 0.0
	for i, l := range lines {
		n := float64(len(l.Text))
		typeMs := n * msPerCharType
		deleteMs := n * msPerCharDelete
		w := window{
			startPct:     t0 / total * 100,
			typedPct:     (t0 + typeMs) / total * 100,
			holdEndPct:   (t0 + typeMs + holdMs) / total * 100,
			deleteEndPct: (t0 + typeMs + holdMs + deleteMs) / total * 100,
			widthPx:      charW * n,
			charCount:    len(l.Text),
		}
		windows[i] = w
		t0 += spans[i]
	}

	b.WriteString(`<style>` + "\n@media (prefers-reduced-motion: no-preference){\n")
	for i, w := range windows {
		fmt.Fprintf(b, ".tclip%d{animation:typeLine%d %.0fms linear infinite}\n", i, i, total)
		fmt.Fprintf(b, "@keyframes typeLine%d{\n", i)
		if w.startPct > 0 {
			fmt.Fprintf(b, "0%%{width:0px}\n%.2f%%{width:0px;animation-timing-function:steps(%d,end)}\n", w.startPct, w.charCount)
		} else {
			fmt.Fprintf(b, "0%%{width:0px;animation-timing-function:steps(%d,end)}\n", w.charCount)
		}
		fmt.Fprintf(b, "%.2f%%{width:%.1fpx}\n", w.typedPct, w.widthPx)
		fmt.Fprintf(b, "%.2f%%{width:%.1fpx;animation-timing-function:steps(%d,end)}\n", w.holdEndPct, w.widthPx, w.charCount)
		fmt.Fprintf(b, "%.2f%%{width:0px}\n", w.deleteEndPct)
		if w.deleteEndPct < 100 {
			fmt.Fprintf(b, "100%%{width:0px}\n")
		}
		b.WriteString("}\n")
	}
	b.WriteString("}\n</style>\n")
}

func writeTicker(b *strings.Builder, t theme.Tokens, y, h float64) {
	fontSize := 12.0
	charW := monoCharPx(fontSize)
	baseWidth := charW * float64(len(tickerString))
	copies := 1
	for float64(copies)*baseWidth < contentW {
		copies++
	}
	single := strings.Repeat(tickerString, copies)
	singleWidth := charW * float64(len(single))
	doubled := single + single

	fmt.Fprintf(b, `<rect x="0" y="%.1f" width="%.0f" height="%.0f" fill="%s"/>`+"\n",
		y, contentW, h, t.Card)
	fmt.Fprintf(b, `<rect x="0" y="%.1f" width="%.0f" height="2" fill="%s"/>`+"\n", y, contentW, t.Accent)
	fmt.Fprintf(b, `<rect x="0" y="%.1f" width="%.0f" height="2" fill="%s"/>`+"\n", y+h-2, contentW, t.Accent)

	fmt.Fprintf(b, `<clipPath id="tickerclip"><rect x="0" y="%.1f" width="%.0f" height="%.0f"/></clipPath>`+"\n",
		y, contentW, h)
	fmt.Fprintf(b, `<g clip-path="url(#tickerclip)"><g class="ticker-track"><text class="mono" x="0" y="%.1f" font-size="%.0f" font-weight="700" fill="%s">%s</text></g></g>`+"\n",
		y+h/2+4, fontSize, t.Accent2, doubled)

	fmt.Fprintf(b, `<style>
.ticker-track{transform:translateX(0)}
@media (prefers-reduced-motion: no-preference){
  .ticker-track{animation:tickerScroll 22s linear infinite}
}
@keyframes tickerScroll{ 0%%{transform:translateX(0)} 100%%{transform:translateX(-%.1fpx)} }
</style>
`, singleWidth)
}
