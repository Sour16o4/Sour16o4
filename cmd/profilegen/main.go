// Command profilegen generates the SVG sections for the Sour16o4 GitHub
// profile README.
//
// Data source: live only. GITHUB_TOKEN must be set — there is no fixture
// fallback (removed deliberately; see internal/activity's package comment
// for why). PROFILE_TOKEN is an optional fallback specifically for the
// contributions GraphQL query — see internal/github/contributions.go.
//
// Failure behaviour: every fetch happens BEFORE any file is written. If
// anything fails — no token, rate limit, network, an empty response where
// one isn't expected — the run exits non-zero and touches nothing on disk.
// The previous run's committed assets stay live and correct, just stale by
// one refresh cycle, rather than the page silently showing fabricated data
// or a partially-regenerated, inconsistent asset set.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Sour16o4/profilegen/internal/activity"
	ghclient "github.com/Sour16o4/profilegen/internal/github"
	"github.com/Sour16o4/profilegen/internal/projects"
	"github.com/Sour16o4/profilegen/internal/render"
	"github.com/Sour16o4/profilegen/internal/theme"
)

const (
	profileLogin = "Sour16o4"
	commitLimit  = 5
)

// output collects every generated file in memory. Nothing touches disk
// until every data source has succeeded — see the failure-behaviour note
// above.
type output struct {
	files map[string]string
}

func newOutput() *output { return &output{files: map[string]string{}} }

func (o *output) set(path, content string) { o.files[path] = content }

func (o *output) flush() {
	for path, content := range o.files {
		if dir := filepath.Dir(path); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				fatal(err)
			}
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			fatal(err)
		}
	}
	for _, path := range sortedKeys(o.files) {
		fmt.Printf("wrote %s (%d bytes)\n", path, len(o.files[path]))
	}
}

func main() {
	ctx := context.Background()
	out := newOutput()

	client, err := ghclient.NewClient()
	if err != nil {
		fatal(fmt.Errorf("no live data source available, refusing to run: %w", err))
	}
	source := activity.FromAPI{Client: client, Login: profileLogin, Limit: commitLimit}

	commits, err := source.Commits(ctx)
	if err != nil {
		fatalLive("commits", err)
	}
	cal, err := source.Contributions(ctx)
	if err != nil {
		fatalLive("contributions", err)
	}

	allProjects, links, err := projects.Load("projects.yaml")
	if err != nil {
		fatal(err)
	}
	active := projects.Active(allProjects)
	shipped := projects.Shipped(allProjects)

	const name = "SOURAV SALAMPURIA"
	const subtitle = "full stack developer · gurgaon, in · open to roles"
	out.set("assets/header-dark.svg", render.Header(theme.Dark, name, subtitle, render.DefaultTyping))
	out.set("assets/header-light.svg", render.Header(theme.Light, name, subtitle, render.DefaultTyping))

	for _, g := range []struct {
		items []projects.Project
		meta  bool
	}{{active, true}, {shipped, false}} {
		for _, th := range []struct {
			suffix string
			tokens theme.Tokens
		}{{"dark", theme.Dark}, {"light", theme.Light}} {
			for i, svg := range render.ProjectCards(th.tokens, g.items, g.meta) {
				out.set(fmt.Sprintf("assets/card-%s-%s.svg", slug(g.items[i]), th.suffix), svg)
			}
		}
	}

	now := time.Now()
	out.set("assets/commits-dark.svg", render.Commits(theme.Dark, commits, now))
	out.set("assets/commits-light.svg", render.Commits(theme.Light, commits, now))

	out.set("assets/contributions-dark.svg", render.Contributions(theme.Dark, cal, theme.HeatmapRampDark))
	out.set("assets/contributions-light.svg", render.Contributions(theme.Light, cal, theme.HeatmapRampLight))

	out.set("README.md", buildReadme(active, shipped, links))

	out.flush()
}

// fatalLive is the failure-behaviour decision in code: exit before any
// output file is written or touched. A *github.RateLimitError's Error()
// text names its own reset time, so CI logs are actionable without any
// special-casing here.
func fatalLive(surface string, err error) {
	fmt.Fprintf(os.Stderr, "profilegen: live %s fetch failed, aborting without writing anything: %v\n", surface, err)
	os.Exit(1)
}

// section is one <picture> block in the README: a dark source, a light img
// fallback, real alt text (screen readers), and a title (the only hover
// affordance GitHub permits on an <img> — SVGs served this way get no
// pointer events, so :hover rules are dead code, but the browser's native
// title tooltip still works). Dropped once already in a regeneration;
// asserted here explicitly so it can't silently disappear again.
type section struct {
	base  string
	alt   string
	title string
}

var topSections = []section{
	{"header", "Sourav Salampuria, full stack developer, Gurgaon, India — open to roles", "Full stack developer — Go, Next.js, PostgreSQL, Kubernetes. Gurgaon, India. Open to roles."},
}

var activitySections = []section{
	{"commits", "Recent commits across active repositories, with diffstat and relative time", "Recent commits across active repositories, refreshed automatically."},
	{"contributions", "Contribution calendar for the last year, with total, longest streak, and current streak", "Contribution calendar for the last twelve months."},
}

// buildReadme wires a real anchor beneath every project-bearing section —
// there is no other way to click through from an <img>-rendered SVG (links
// inside an SVG don't work once GitHub serves it as a plain image). Activity
// (commits + contributions) is collapsed behind <details> — it's real,
// auto-refreshed data now, but still secondary to the header/now/shipped
// sections a first-time visitor should see without an extra click.
func buildReadme(active, shipped []projects.Project, links projects.Links) string {
	var b strings.Builder
	for _, s := range topSections {
		writeSection(&b, s)
	}
	writeCards(&b, active)
	writeCards(&b, shipped)

	b.WriteString("<details>\n<summary><b>Activity</b></summary>\n\n")
	for _, s := range activitySections {
		writeSection(&b, s)
	}
	b.WriteString("</details>\n\n")

	writeFooterLinks(&b, links)
	return b.String()
}

func writeSection(b *strings.Builder, s section) {
	fmt.Fprintf(b, `<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/%s-dark.svg">
  <img alt="%s" title="%s" src="assets/%s-light.svg">
</picture>

`, s.base, s.alt, s.title, s.base)
}

func slug(p projects.Project) string {
	return strings.ToLower(p.RepoURL[strings.LastIndex(p.RepoURL, "/")+1:])
}

// writeCards emits one <a><picture> per project on a single line. Each card
// is its own image, so each is its own link to the repo; width in percent
// lets the cards sit side by side and scale with the README column.
func writeCards(b *strings.Builder, items []projects.Project) {
	n := len(items)
	if n == 0 {
		return
	}
	w := 100/n - 1
	for _, p := range items {
		fmt.Fprintf(b, `<a href="%s"><picture><source media="(prefers-color-scheme: dark)" srcset="assets/card-%s-dark.svg"><img width="%d%%" alt="%s — open the repo" src="assets/card-%s-light.svg"></picture></a>`,
			p.RepoURL, slug(p), w, p.Name, slug(p))
	}
	b.WriteString("\n\n")
}

// writeFooterLinks only emits a link if it has a real value — an empty
// field produces no anchor at all, never a broken `[label]()`.
func writeFooterLinks(b *strings.Builder, links projects.Links) {
	type link struct{ label, url string }
	candidates := []link{
		{"portfolio", links.Portfolio},
		{"linkedin", links.LinkedIn},
		{"email", links.Email},
		{"resume", links.Resume},
	}
	var parts []string
	for _, c := range candidates {
		if c.url == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("[%s](%s)", c.label, c.url))
	}
	if len(parts) == 0 {
		return
	}
	fmt.Fprintln(b, strings.Join(parts, " · "))
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "profilegen:", err)
	os.Exit(1)
}
