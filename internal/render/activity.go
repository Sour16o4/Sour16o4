package render

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Sour16o4/profilegen/internal/activity"
	"github.com/Sour16o4/profilegen/internal/theme"
)

// ---------------------------------------------------------------- commits --

const (
	logRowH  = 44.0
	logPad   = 16.0
	hashColW = 62.0
	statColW = 70.0
	timeColW = 92.0
	colGap   = 16.0
)

// shortSHA is the display form: 7 hex characters, matching what the hash
// column was actually sized for (hashColW=62 at 12px mono leaves room for
// 7 glyphs, not the 40-character full SHA the API returns — rendering the
// full SHA overlapped straight into the message column).
func shortSHA(sha string) string {
	if len(sha) <= 7 {
		return sha
	}
	return sha[:7]
}

// truncateToWidth is the single-line counterpart of wrapText: cuts s (with a
// trailing "…") if it would exceed maxWidth at fontSize, generic-stack
// estimate. The message is the only column that truncates this way — the
// hash column uses shortSHA instead (a fixed 7 characters, not a width fit).
func truncateToWidth(s string, maxWidth, fontSize float64) string {
	charW := fontSize * charFactor
	maxChars := int(maxWidth / charW)
	if len(s) <= maxChars {
		return s
	}
	if maxChars <= 1 {
		return "…"
	}
	return s[:maxChars-1] + "…"
}

// Commits renders the recent-commits log panel (§5 item 7).
func Commits(t theme.Tokens, commits []activity.Commit, now time.Time) string {
	n := len(commits)
	panelH := 2*logPad + float64(n)*logRowH
	height := topPad + panelH + botPad*0.4

	messageW := contentW - 2*contentPad - hashColW - statColW - timeColW - 3*colGap

	var b strings.Builder
	fmt.Fprintf(&b, `<svg width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" xmlns="http://www.w3.org/2000/svg" role="img" aria-labelledby="cm-title cm-desc">`+"\n",
		contentW, height, contentW, height)
	b.WriteString(`<title id="cm-title">Recent commits</title>` + "\n")
	fmt.Fprintf(&b, `<desc id="cm-desc">Latest %d commits across active repositories, with diffstat and relative time.</desc>`+"\n", n)

	writeCommitsStyle(&b, t, n)
	fmt.Fprintf(&b, `<rect width="%.0f" height="%.0f" fill="%s"/>`+"\n", contentW, height, t.Ground)

	panelY := topPad
	fmt.Fprintf(&b, `<rect x="0" y="%.1f" width="%.0f" height="%.1f" fill="%s" stroke="%s" stroke-width="2"/>`+"\n",
		panelY, contentW, panelH, t.Card, t.Line)

	for i, c := range commits {
		rowY := panelY + logPad + float64(i)*logRowH
		writeCommitRow(&b, t, c, rowY, i, n, messageW, now)
	}

	b.WriteString("</svg>\n")
	return b.String()
}

func writeCommitsStyle(b *strings.Builder, t theme.Tokens, n int) {
	b.WriteString(`<style>
text{font-family:` + theme.Fonts.Sans + `}
.mono{font-family:` + theme.Fonts.Mono + `}
.row-entrance{opacity:1;transform:translateY(0)}
@media (prefers-reduced-motion: no-preference){
`)
	for i := 0; i < n; i++ {
		fmt.Fprintf(b, "  .r%d{animation:rowIn .6s cubic-bezier(.32,.72,0,1) %.2fs forwards}\n", i, float64(i)*0.05)
	}
	b.WriteString(`  @keyframes rowIn{
    0%{opacity:0;transform:translateY(16px)}
    100%{opacity:1;transform:translateY(0)}
  }
}
</style>
`)
}

func writeCommitRow(b *strings.Builder, t theme.Tokens, c activity.Commit, rowY float64, idx, total int, messageW float64, now time.Time) {
	fmt.Fprintf(b, `<g class="row-entrance r%d">`+"\n", idx)

	if idx < total-1 {
		fmt.Fprintf(b, `<line x1="0" y1="%.1f" x2="%.0f" y2="%.1f" stroke="%s" stroke-width="1"/>`+"\n",
			rowY+logRowH, contentW, rowY+logRowH, t.Line)
	}

	textY := rowY + logRowH/2 + 4.5
	x := contentPad

	fmt.Fprintf(b, `<text class="mono" x="%.1f" y="%.1f" font-size="12" fill="%s">%s</text>`+"\n",
		x, textY, t.Accent2, shortSHA(c.SHA))
	x += hashColW + colGap

	msg := truncateToWidth(c.Message, messageW, 13)
	fmt.Fprintf(b, `<text x="%.1f" y="%.1f" font-size="13" fill="%s">%s</text>`+"\n",
		x, textY, t.Body, msg)
	x += messageW + colGap

	diffstat := fmt.Sprintf("+%d -%d", c.Additions, c.Deletions)
	fmt.Fprintf(b, `<text class="mono" x="%.1f" y="%.1f" font-size="11.5" fill="%s" text-anchor="end">%s</text>`+"\n",
		x+statColW, textY, t.Dim, diffstat)
	x += statColW + colGap

	rel := activity.RelativeTime(c.Date, now)
	fmt.Fprintf(b, `<text class="mono" x="%.1f" y="%.1f" font-size="11" fill="%s" text-anchor="end">%s</text>`+"\n",
		x+timeColW, textY, t.Dim, rel)

	b.WriteString(`</g>` + "\n")
}

// ------------------------------------------------------------ contributions --

const (
	heatCell = 10.0
	heatGap  = 3.0
	heatCols = 52
	heatRows = 7
)

func heatLevel(count int) int {
	switch {
	case count == 0:
		return 0
	case count <= 2:
		return 1
	case count <= 5:
		return 2
	case count <= 9:
		return 3
	default:
		return 4
	}
}

// Contributions renders the 52x7 heatmap plus the footer stats/legend line
// (§5 item 8). File size is the real constraint here (§ instructions): cells
// are <use> references against one shared <rect id="cell"> in <defs> rather
// than repeating width/height/rx on all 364, and the entrance "wave" delay is
// bucketed per week-column (52 CSS rules) instead of per cell (364) — visually
// indistinguishable at 3.4ms/cell resolution, and it reproduces the same
// total ~1.2s sweep (52 * 7 * 3.4ms ≈ 1.24s) for a fraction of the CSS.
func Contributions(t theme.Tokens, cal activity.ContributionCalendar, ramp [5]string) string {
	weeks := cal.Weeks
	gridH := heatRows*(heatCell+heatGap) - heatGap
	footerH := 30.0
	height := topPad + gridH + footerH + botPad*0.4

	streaks := activity.ComputeStreaks(weeks)

	var b strings.Builder
	fmt.Fprintf(&b, `<svg width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" xmlns="http://www.w3.org/2000/svg" role="img" aria-labelledby="ct-title ct-desc">`+"\n",
		contentW, height, contentW, height)
	b.WriteString(`<title id="ct-title">Contributions</title>` + "\n")
	fmt.Fprintf(&b, `<desc id="ct-desc">%d contributions in the last year, longest streak %d days, current streak %d days.</desc>`+"\n",
		cal.TotalContributions, streaks.Longest, streaks.Current)

	writeContributionsStyle(&b, t, ramp)
	fmt.Fprintf(&b, `<rect width="%.0f" height="%.0f" fill="%s"/>`+"\n", contentW, height, t.Ground)
	b.WriteString(`<defs><rect id="cell" width="10" height="10"/></defs>` + "\n")

	gridX := contentPad
	gridY := topPad
	for week := 0; week < len(weeks) && week < heatCols; week++ {
		wx := gridX + float64(week)*(heatCell+heatGap)
		for day, d := range weeks[week].ContributionDays {
			if day >= heatRows {
				break
			}
			wy := gridY + float64(day)*(heatCell+heatGap)
			level := heatLevel(d.ContributionCount)
			fmt.Fprintf(&b, `<use href="#cell" x="%.1f" y="%.1f" class="w%d lvl%d"/>`+"\n",
				wx, wy, week, level)
		}
	}

	writeContributionsFooter(&b, t, gridY+gridH+22, cal.TotalContributions, streaks, ramp)

	b.WriteString("</svg>\n")
	return b.String()
}

func writeContributionsStyle(b *strings.Builder, t theme.Tokens, ramp [5]string) {
	b.WriteString(`<style>
text{font-family:` + theme.Fonts.Sans + `}
.mono{font-family:` + theme.Fonts.Mono + `}
`)
	for lvl, color := range ramp {
		fmt.Fprintf(b, ".lvl%d{fill:%s}\n", lvl, color)
	}
	b.WriteString(`use{opacity:1;transform-box:fill-box;transform-origin:center;transform:scale(1)}
@media (prefers-reduced-motion: no-preference){
`)
	for week := 0; week < heatCols; week++ {
		delayMs := float64(week) * heatRows * 3.4
		fmt.Fprintf(b, "  .w%d{animation:cellIn .5s ease-out %.1fms forwards}\n", week, delayMs)
	}
	b.WriteString(`  @keyframes cellIn{
    0%{opacity:0;transform:scale(.3)}
    100%{opacity:1;transform:scale(1)}
  }
}
</style>
`)
}

func writeContributionsFooter(b *strings.Builder, t theme.Tokens, y float64, total int, s activity.Streaks, ramp [5]string) {
	fontSize := 11.0
	charW := fontSize * charFactor * 1.2 // mono runs a touch wider than the generic-sans estimate
	x := contentPad

	line1 := fmt.Sprintf("%d contributions", total)
	fmt.Fprintf(b, `<text class="mono" x="%.1f" y="%.1f" font-size="%.0f" fill="%s">%s</text>`+"\n",
		x, y, fontSize, t.Text, line1)
	x += float64(len(line1)) * charW

	// U+00A0 (nbsp), not plain spaces: SVG collapses ordinary leading/
	// trailing whitespace inside <text> content, which was eating the
	// padding around the separator dot.
	sep := "  ·  "
	sepRunes := float64(utf8.RuneCountInString(sep))
	writeSep := func(label string, muted bool) {
		fmt.Fprintf(b, `<text class="mono" x="%.1f" y="%.1f" font-size="%.0f" fill="%s">%s</text>`+"\n",
			x, y, fontSize, t.Dim, sep)
		x += sepRunes * charW
		color := t.Text
		if muted {
			color = t.Dim
		}
		fmt.Fprintf(b, `<text class="mono" x="%.1f" y="%.1f" font-size="%.0f" fill="%s">%s</text>`+"\n",
			x, y, fontSize, color, label)
		x += float64(len(label)) * charW
	}
	writeSep(fmt.Sprintf("longest streak %d", s.Longest), false)
	writeSep(fmt.Sprintf("current streak %d", s.Current), false)

	fmt.Fprintf(b, `<text class="mono" x="%.1f" y="%.1f" font-size="%.0f" fill="%s">%s</text>`+"\n",
		x, y, fontSize, t.Dim, sep)
	x += float64(len(sep)) * charW
	fmt.Fprintf(b, `<text class="mono" x="%.1f" y="%.1f" font-size="%.0f" fill="%s">less</text>`+"\n",
		x, y, fontSize, t.Dim)
	x += float64(len("less"))*charW + 8

	for lvl, color := range ramp {
		fmt.Fprintf(b, `<rect x="%.1f" y="%.1f" width="10" height="10" fill="%s"/>`+"\n",
			x, y-9, color)
		x += 10 + 4
		_ = lvl
	}
	x += 4
	fmt.Fprintf(b, `<text class="mono" x="%.1f" y="%.1f" font-size="%.0f" fill="%s">more</text>`+"\n",
		x, y, fontSize, t.Dim)
}
