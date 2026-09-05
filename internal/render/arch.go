package render

import (
	"fmt"
	"strings"

	"github.com/Sour16o4/profilegen/internal/theme"
)

// ArchNode is one box in the architecture strip (§5 item 5, §6.2).
type ArchNode struct {
	Name  string // uppercase node name, e.g. "GATEWAY"
	Sub   string // mono sub-label, e.g. "go · routing · auth"
	Owned bool   // true for the two nodes the profile owner actually works on
}

// DefaultArch is the four-node request path shown on the profile.
// Ownership is encoded structurally (§6.2): gateway and services get the
// accent border/shadow treatment, client and postgres stay neutral.
var DefaultArch = []ArchNode{
	{"CLIENT", "http · grpc", false},
	{"GATEWAY", "go · routing · auth", true},
	{"SERVICES", "go · queues", true},
	{"POSTGRES", "tenant-scoped", false},
}

const (
	archNodeW = 180.0
	archNodeH = 88.0
)

func archGap() float64 {
	return (contentW - float64(len(DefaultArch))*archNodeW) / float64(len(DefaultArch)-1)
}

func archCenterX(i int) float64 {
	step := archNodeW + archGap()
	return float64(i)*step + archNodeW/2
}

func archLeftX(i int) float64 { return archCenterX(i) - archNodeW/2 }

// Architecture renders the "system I work on" strip: continuous traffic
// across three links (unlike the pipeline's discrete 12s run — §6.2 is
// explicit that this distinction must stay visible), with ownership of the
// middle two nodes shown in the border/shadow treatment, not just stated.
func Architecture(t theme.Tokens, nodes []ArchNode) string {
	if len(nodes) != 4 {
		panic("Architecture: layout is fixed at 4 nodes; update archCenterX before changing this")
	}
	height := topPad + archNodeH + botPad + 26 // +26 for the metadata line below
	rowY := topPad
	trackY := rowY + archNodeH - 20 // lower third, clear of label text — same fix as the pipeline

	var b strings.Builder

	fmt.Fprintf(&b, `<svg width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" xmlns="http://www.w3.org/2000/svg" role="img" aria-labelledby="arch-title arch-desc">`+"\n",
		contentW, height, contentW, height)
	b.WriteString(`<title id="arch-title">The system I work on</title>` + "\n")
	b.WriteString(`<desc id="arch-desc">Request path: client to gateway to services to postgres, with continuous traffic on every link; gateway and services are the two components I own.</desc>` + "\n")

	writeArchStyle(&b, t, nodes)

	fmt.Fprintf(&b, `<rect width="%.0f" height="%.0f" fill="%s"/>`+"\n", contentW, height, t.Ground)

	for i := 0; i < len(nodes)-1; i++ {
		x1 := archLeftX(i) + archNodeW
		x2 := archLeftX(i + 1)
		writeArchConnector(&b, t, x1, x2, trackY)
	}

	for i, n := range nodes {
		writeArchNode(&b, t, n, i, archLeftX(i), rowY)
	}

	for i := 0; i < len(nodes)-1; i++ {
		writeArchTraffic(&b, t, i, archLeftX(i)+archNodeW, archLeftX(i+1), trackY)
	}

	writeArchMeta(&b, t, rowY+archNodeH+30)

	b.WriteString("</svg>\n")
	return b.String()
}

func writeArchStyle(b *strings.Builder, t theme.Tokens, nodes []ArchNode) {
	gap := archGap()
	// Rest-frame offsets match the CSS negative animation-delay below
	// (-0s/-0.5s/-1s into a 4.2s loop), so the static frame is a plausible
	// snapshot of the same continuous stream, not an arbitrary pose.
	off1 := gap * (0.5 / 4.2)
	off2 := gap * (1.0 / 4.2)
	fmt.Fprintf(b, `<style>
text{font-family:%s}
.mono{font-family:%s}
.node-name{font-weight:700;letter-spacing:.02em;font-size:12.5px}
.node-sub{font-size:10.5px;fill:%s}
.node-owned{fill:%s;stroke:%s;stroke-width:2px}
.node-neutral{fill:%s;stroke:%s;stroke-width:2px}
.shadow-owned{fill:%s;transform:translate(4px,4px)}
.shadow-neutral{fill:%s;transform:translate(3px,3px)}
.p0{transform:translateX(0px)}
.p1{transform:translateX(%.2fpx)}
.p2{transform:translateX(%.2fpx)}

@media (prefers-reduced-motion: no-preference){
  .p0,.p1,.p2{animation:archFlow 4.2s linear infinite}
  .p0{animation-delay:0s}
  .p1{animation-delay:-0.5s}
  .p2{animation-delay:-1s}
}
@keyframes archFlow{
  0%%{transform:translateX(0px)}
  100%%{transform:translateX(%.2fpx)}
}
</style>
`,
		theme.Fonts.Sans, theme.Fonts.Mono,
		t.Mute,
		t.CardHi, t.Bone,
		t.Card, t.Line,
		t.Accent,
		t.Line,
		off1, off2,
		gap,
	)
}

// writeArchConnector uses a solid, unbroken rail — deliberately different
// from the pipeline's dashed connector. Differentiation rule from the
// duplicate-graphic check: rail treatment and particle shape carry the
// distinction, not colour (single hue holds) and not size. A solid line
// also fits the meaning — continuous traffic, not a discrete route.
func writeArchConnector(b *strings.Builder, t theme.Tokens, x1, x2, y float64) {
	arrowLen := 7.0
	fmt.Fprintf(b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="2"/>`+"\n",
		x1, y, x2-arrowLen, y, t.Rail)
	fmt.Fprintf(b, `<polygon points="%.1f,%.1f %.1f,%.1f %.1f,%.1f" fill="%s"/>`+"\n",
		x2-arrowLen, y-5, x2, y, x2-arrowLen, y+5, t.Rail)
}

func writeArchNode(b *strings.Builder, t theme.Tokens, n ArchNode, i int, x, y float64) {
	nodeClass := "node-neutral"
	shadowClass := "shadow-neutral"
	nameColor := t.Mute
	if n.Owned {
		nodeClass = "node-owned"
		shadowClass = "shadow-owned"
		nameColor = t.Text
	}
	fmt.Fprintf(b, `<rect class="%s" x="%.1f" y="%.1f" width="%.0f" height="%.0f"/>`+"\n",
		shadowClass, x, y, archNodeW, archNodeH)
	fmt.Fprintf(b, `<rect class="%s" x="%.1f" y="%.1f" width="%.0f" height="%.0f"/>`+"\n",
		nodeClass, x, y, archNodeW, archNodeH)
	fmt.Fprintf(b, `<text class="node-name" x="%.1f" y="%.1f" fill="%s">%s</text>`+"\n",
		x+14, y+30, nameColor, n.Name)
	fmt.Fprintf(b, `<text class="mono node-sub" x="%.1f" y="%.1f">%s</text>`+"\n",
		x+14, y+48, n.Sub)
}

func writeArchTraffic(b *strings.Builder, t theme.Tokens, linkIdx int, x1, x2, y float64) {
	// Three squares per link, staggered 0/.5/1s over a continuous 4.2s loop —
	// a request path is always under load, unlike the pipeline's discrete
	// per-commit run (§6.2). Position driven by CSS transform (not SMIL) so
	// it can be gated by prefers-reduced-motion the same way as the pipeline.
	//
	// Hollow (stroke-only, no fill) — the pipeline's packet stays a solid
	// filled square. Same shape, same 8px size, same single hue; only the
	// fill treatment differs, which is what makes the two strips read as
	// distinct graphics instead of one repeated.
	fmt.Fprintf(b, `<g transform="translate(%.1f,0)">
<rect class="p0" x="0" y="%.1f" width="8" height="8" fill="none" stroke="%s" stroke-width="2"/>
<rect class="p1" x="0" y="%.1f" width="8" height="8" fill="none" stroke="%s" stroke-width="2"/>
<rect class="p2" x="0" y="%.1f" width="8" height="8" fill="none" stroke="%s" stroke-width="2"/>
</g>
`,
		x1,
		y-4, t.Accent,
		y-4, t.Accent,
		y-4, t.Accent,
	)
}

func writeArchMeta(b *strings.Builder, t theme.Tokens, y float64) {
	// x=0 (flush against the viewBox edge, no breathing room) was reported
	// as clipped on the live page — the node boxes can sit flush by design
	// since a rect's edge is exact, but a glyph's outline can extend
	// slightly past its nominal x position, and 0 leaves no margin to
	// absorb that.
	const metaX = 8.0
	fmt.Fprintf(b, `<text class="mono" x="%.1f" y="%.1f" font-size="11" fill="%s">`+"\n",
		metaX, y, t.Mute)
	fmt.Fprintf(b, `i own the <tspan fill="%s">middle two</tspan> &#183; request path <tspan fill="%s">left to right</tspan> &#183; every row scoped by <tspan fill="%s">tenant_id</tspan>`+"\n",
		t.Accent2, t.Accent2, t.Accent2)
	b.WriteString(`</text>` + "\n")
}
