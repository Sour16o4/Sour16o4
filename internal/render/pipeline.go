// Package render emits the profile's SVG sections, one file per section (§8).
package render

import (
	"fmt"
	"strings"

	"github.com/Sour16o4/profilegen/internal/theme"
)

// Stage is one node in the delivery pipeline (§5 item 4, §6.1).
type Stage struct {
	Index int    // 0-based, printed as "01".."05"
	Name  string // uppercase card title, e.g. "COMMIT"
}

// DefaultStages is the pipeline shown on the profile: commit → observe.
var DefaultStages = []Stage{
	{0, "COMMIT"},
	{1, "BUILD"},
	{2, "TEST"},
	{3, "DEPLOY"},
	{4, "OBSERVE"},
}

const (
	contentW = 890.0
	stageW   = 140.0
	stageH   = 88.0
	topPad   = 40.0
	botPad   = 42.0
)

// stageCenters are the packet dwell positions as fractions of row width (§6.1):
// 7.6% / 28.8% / 50% / 71.2% / 92.4%.
var stageCenterFrac = [5]float64{0.076, 0.288, 0.500, 0.712, 0.924}

func stageCenterX(i int) float64 { return stageCenterFrac[i] * contentW }
func stageLeftX(i int) float64   { return stageCenterX(i) - stageW/2 }

// Pipeline renders the delivery-pipeline section as a complete, self-contained
// SVG: static frame is meaningful on its own (stage 0 active, packet at rest),
// full animation layers on top and is disabled under prefers-reduced-motion (§9).
func Pipeline(t theme.Tokens, stages []Stage) string {
	if len(stages) != 5 {
		panic("Pipeline: spec fixes the row at 5 stages; update stageCenterFrac before changing this")
	}
	height := topPad + stageH + botPad
	rowY := topPad
	// Packet track sits low in the cards, clear of the index/name text at
	// the top — the earlier version ran it through box vertical-center and
	// it collided with the stage labels in the static (reduced-motion) frame.
	midY := rowY + stageH - 20

	var b strings.Builder

	fmt.Fprintf(&b, `<svg width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" xmlns="http://www.w3.org/2000/svg" role="img" aria-labelledby="pl-title pl-desc">`+"\n",
		contentW, height, contentW, height)
	fmt.Fprintf(&b, `<title id="pl-title">Delivery pipeline</title>`+"\n")
	fmt.Fprintf(&b, `<desc id="pl-desc">Five stage pipeline: commit, build, test, deploy, observe, with a packet travelling stage to stage.</desc>`+"\n")

	writePipelineStyle(&b, t)

	fmt.Fprintf(&b, `<rect width="%.0f" height="%.0f" fill="%s"/>`+"\n", contentW, height, t.Ground)

	// Connectors first (drawn under everything else).
	for i := 0; i < len(stages)-1; i++ {
		x1 := stageLeftX(i) + stageW
		x2 := stageLeftX(i + 1)
		writeConnector(&b, t, x1, x2, midY)
	}

	// Stage boxes: shadow, then card, then label text, then workbar.
	for i, s := range stages {
		x := stageLeftX(i)
		writeStage(&b, t, s, x, rowY)
	}

	// Packet layer, drawn last so it renders above the cards (§6.1).
	writePacket(&b, t, midY)

	b.WriteString("</svg>\n")
	return b.String()
}

func writePipelineStyle(b *strings.Builder, t theme.Tokens) {
	fmt.Fprintf(b, `<style>
text{font-family:%s}
.mono{font-family:%s}
.stage-name{font-weight:700;letter-spacing:.02em}
.stage-idx{fill:%s}
.workbar{opacity:0}

/* base (reduced-motion / no-JS-equivalent) state: stage 1 reads as active,
   this is the meaningful static frame, not a blank one (§9) */
.stage-box{fill:%s;stroke:%s;stroke-width:2px}
.stage-box.s0{fill:%s;stroke:%s}
.stage-shadow{fill:%s;transform:translate(4px,4px)}
.stage-shadow.s0{transform:translate(9px,9px)}
.packet-group{transform:translateX(%.2fpx)}

@media (prefers-reduced-motion: no-preference){
  .stage-box{animation:stagePulse 12s infinite}
  .stage-box.s0{animation-delay:0s}
  .stage-box.s1{animation-delay:2.4s}
  .stage-box.s2{animation-delay:4.8s}
  .stage-box.s3{animation-delay:7.2s}
  .stage-box.s4{animation-delay:9.6s}
  .stage-shadow{animation:shadowPulse 12s infinite}
  .stage-shadow.s0{animation-delay:0s}
  .stage-shadow.s1{animation-delay:2.4s}
  .stage-shadow.s2{animation-delay:4.8s}
  .stage-shadow.s3{animation-delay:7.2s}
  .stage-shadow.s4{animation-delay:9.6s}
  .workbar{animation:workbarSweep 12s infinite}
  .workbar.s0{animation-delay:0s}
  .workbar.s1{animation-delay:2.4s}
  .workbar.s2{animation-delay:4.8s}
  .workbar.s3{animation-delay:7.2s}
  .workbar.s4{animation-delay:9.6s}
  .packet-group{animation:packetTravel 12s linear infinite}
  .trail-1{animation-delay:.13s}
  .trail-2{animation-delay:.26s}

  @keyframes stagePulse{
    0%%{fill:%s;stroke:%s}
    2%%{fill:%s;stroke:%s}
    10%%{fill:%s;stroke:%s}
    18%%{fill:%s;stroke:%s}
    100%%{fill:%s;stroke:%s}
  }
  @keyframes shadowPulse{
    0%%{transform:translate(4px,4px)}
    2%%{transform:translate(9px,9px)}
    10%%{transform:translate(9px,9px)}
    18%%{transform:translate(4px,4px)}
    100%%{transform:translate(4px,4px)}
  }
  @keyframes workbarSweep{
    0%%{width:0px;opacity:1}
    10%%{width:%.0fpx;opacity:1}
    14%%{opacity:0}
    100%%{opacity:0}
  }
  @keyframes packetTravel{
    0%%{transform:translateX(%.2fpx);animation-timing-function:cubic-bezier(0,0,1,1)}
    10%%{transform:translateX(%.2fpx);animation-timing-function:cubic-bezier(.5,0,.5,1)}
    20%%{transform:translateX(%.2fpx);animation-timing-function:cubic-bezier(0,0,1,1)}
    30%%{transform:translateX(%.2fpx);animation-timing-function:cubic-bezier(.5,0,.5,1)}
    40%%{transform:translateX(%.2fpx);animation-timing-function:cubic-bezier(0,0,1,1)}
    50%%{transform:translateX(%.2fpx);animation-timing-function:cubic-bezier(.5,0,.5,1)}
    60%%{transform:translateX(%.2fpx);animation-timing-function:cubic-bezier(0,0,1,1)}
    70%%{transform:translateX(%.2fpx);animation-timing-function:cubic-bezier(.5,0,.5,1)}
    80%%{transform:translateX(%.2fpx);animation-timing-function:cubic-bezier(0,0,1,1)}
    100%%{transform:translateX(%.2fpx)}
  }
}
</style>
`,
		theme.Fonts.Sans, theme.Fonts.Mono, t.Dim,
		t.Card, t.Bone,
		t.CardHi, t.Accent,
		t.Accent,
		stageCenterX(0),
		t.Card, t.Bone, t.CardHi, t.Accent, t.CardHi, t.Accent, t.Card, t.Bone, t.Card, t.Bone,
		stageW,
		stageCenterX(0), stageCenterX(0), stageCenterX(1), stageCenterX(1), stageCenterX(2), stageCenterX(2), stageCenterX(3), stageCenterX(3), stageCenterX(4), stageCenterX(4),
	)
}

func writeConnector(b *strings.Builder, t theme.Tokens, x1, x2, y float64) {
	arrowLen := 8.0
	fmt.Fprintf(b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="2" stroke-dasharray="5,5"/>`+"\n",
		x1, y, x2-arrowLen, y, t.Rail)
	fmt.Fprintf(b, `<polygon points="%.1f,%.1f %.1f,%.1f %.1f,%.1f" fill="%s"/>`+"\n",
		x2-arrowLen, y-5, x2, y, x2-arrowLen, y+5, t.Rail)
}

func writeStage(b *strings.Builder, t theme.Tokens, s Stage, x, y float64) {
	cls := fmt.Sprintf("s%d", s.Index)
	// hard offset shadow — flat fill, never blurred, never rgba (§4)
	fmt.Fprintf(b, `<rect class="stage-shadow %s" x="%.1f" y="%.1f" width="%.0f" height="%.0f" fill="%s"/>`+"\n",
		cls, x, y, stageW, stageH, t.Accent)
	fmt.Fprintf(b, `<rect class="stage-box %s" x="%.1f" y="%.1f" width="%.0f" height="%.0f"/>`+"\n",
		cls, x, y, stageW, stageH)
	fmt.Fprintf(b, `<text class="mono stage-idx" x="%.1f" y="%.1f" font-size="10.5">%02d</text>`+"\n",
		x+14, y+24, s.Index+1)
	fmt.Fprintf(b, `<text class="stage-name" x="%.1f" y="%.1f" font-size="14" fill="%s">%s</text>`+"\n",
		x+14, y+52, t.Text, s.Name)
	fmt.Fprintf(b, `<rect class="workbar %s" x="%.1f" y="%.1f" width="0" height="3" fill="%s"/>`+"\n",
		cls, x, y+stageH-3, t.Accent)
}

func writePacket(b *strings.Builder, t theme.Tokens, y float64) {
	b.WriteString(`<g class="packet-group">` + "\n")
	// trail: 7px / 5px squares, fading, lagging behind the lead packet
	fmt.Fprintf(b, `<rect class="trail-2" x="-2.5" y="%.1f" width="5" height="5" fill="%s" fill-opacity="0.28" stroke="%s" stroke-width="2"/>`+"\n",
		y-2.5, t.Accent, t.Ground)
	fmt.Fprintf(b, `<rect class="trail-1" x="-3.5" y="%.1f" width="7" height="7" fill="%s" fill-opacity="0.5" stroke="%s" stroke-width="2"/>`+"\n",
		y-3.5, t.Accent, t.Ground)
	// lead packet: 10px, fully opaque
	fmt.Fprintf(b, `<rect x="-5" y="%.1f" width="10" height="10" fill="%s" stroke="%s" stroke-width="2"/>`+"\n",
		y-5, t.Accent, t.Ground)
	b.WriteString(`</g>` + "\n")
}
