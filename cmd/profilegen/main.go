// Command profilegen generates the SVG sections for the Sour16o4 GitHub
// profile README (see build spec, §8). Build order: tokens → static SVG
// rendering → animation → data wiring → workflow — this first pass covers
// the pipeline section end-to-end before the rest.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sour16o4/profilegen/internal/activity"
	"github.com/Sour16o4/profilegen/internal/projects"
	"github.com/Sour16o4/profilegen/internal/render"
	"github.com/Sour16o4/profilegen/internal/theme"
)

func main() {
	outDir := "assets"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatal(err)
	}

	write(filepath.Join(outDir, "pipeline-dark.svg"), render.Pipeline(theme.Dark, render.DefaultStages))
	write(filepath.Join(outDir, "pipeline-light.svg"), render.Pipeline(theme.Light, render.DefaultStages))

	write(filepath.Join(outDir, "arch-dark.svg"), render.Architecture(theme.Dark, render.DefaultArch))
	write(filepath.Join(outDir, "arch-light.svg"), render.Architecture(theme.Light, render.DefaultArch))

	const name = "SOURAV SALAMPURIA"
	const subtitle = "backend engineer · gurgaon, in · open to roles"

	// Shipped: name converted to paths (the font-question recommendation).
	write(filepath.Join(outDir, "header-dark.svg"), render.Header(theme.Dark, name, subtitle, render.DefaultTyping, true))
	write(filepath.Join(outDir, "header-light.svg"), render.Header(theme.Light, name, subtitle, render.DefaultTyping, true))

	// Comparison only, for the font-question review — not referenced by README.
	write(filepath.Join(outDir, "_compare-header-textname.svg"), render.Header(theme.Dark, name, subtitle, render.DefaultTyping, false))

	allProjects, links, err := projects.Load("projects.yaml")
	if err != nil {
		fatal(err)
	}
	active := projects.Active(allProjects)
	shipped := projects.Shipped(allProjects)

	write(filepath.Join(outDir, "now-dark.svg"), render.Projects(theme.Dark, "Working on now", active, true))
	write(filepath.Join(outDir, "now-light.svg"), render.Projects(theme.Light, "Working on now", active, true))
	write(filepath.Join(outDir, "shipped-dark.svg"), render.Projects(theme.Dark, "Shipped", shipped, false))
	write(filepath.Join(outDir, "shipped-light.svg"), render.Projects(theme.Light, "Shipped", shipped, false))

	commits, err := activity.LoadCommits("commits.yaml")
	if err != nil {
		fatal(err)
	}
	now := time.Now()
	write(filepath.Join(outDir, "commits-dark.svg"), render.Commits(theme.Dark, commits, now))
	write(filepath.Join(outDir, "commits-light.svg"), render.Commits(theme.Light, commits, now))

	weeks, err := activity.LoadContributions("contributions.yaml")
	if err != nil {
		fatal(err)
	}
	write(filepath.Join(outDir, "contributions-dark.svg"), render.Contributions(theme.Dark, weeks, theme.HeatmapRampDark))
	write(filepath.Join(outDir, "contributions-light.svg"), render.Contributions(theme.Light, weeks, theme.HeatmapRampLight))

	write(filepath.Join(outDir, "chips-dark.svg"), render.Chips(theme.Dark, render.DefaultChips))
	write(filepath.Join(outDir, "chips-light.svg"), render.Chips(theme.Light, render.DefaultChips))

	writeReadme(links)
}

// section is one <picture> block in the README: a dark source, a light img
// fallback, and real alt text describing the content (not the filename) —
// §9's accessibility requirement.
type section struct {
	base string
	alt  string
}

// Order: header -> pipeline -> arch -> now -> shipped -> commits ->
// contributions -> chips -> footer. Puts header (194px) + pipeline (170px) +
// arch (196px) = 560px above the fold, so the two sections nobody else has
// land first, and land whole rather than getting cut mid-graphic.
var sections = []section{
	{"header", "Sourav Salampuria, backend engineer, Gurgaon, India — open to roles"},
	{"pipeline", "Delivery pipeline: commit, build, test, deploy, observe"},
	{"arch", "The system I work on: client, gateway, services, and postgres, with gateway and services owned"},
	{"now", "Working on now: relay and tenantguard, with progress bars"},
	{"shipped", "Shipped: paykit, gitops observability platform, and book inventory api"},
	{"commits", "Recent commits across active repositories, with diffstat and relative time"},
	{"contributions", "Contribution calendar for the last year, with total, longest streak, and current streak"},
	{"chips", "Stack: Go, gRPC, GraphQL, REST, Docker, Kubernetes, PostgreSQL, MySQL, Redis, ArgoCD"},
}

func writeReadme(links projects.Links) {
	var b strings.Builder
	for _, s := range sections {
		fmt.Fprintf(&b, `<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/%s-dark.svg">
  <img alt="%s" src="assets/%s-light.svg">
</picture>

`, s.base, s.alt, s.base)
	}
	fmt.Fprintf(&b, "[portfolio](%s) · [linkedin](%s) · [email](%s) · [resume](%s)\n",
		links.Portfolio, links.LinkedIn, links.Email, links.Resume)

	write("README.md", b.String())
}

func write(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fatal(err)
	}
	info, _ := os.Stat(path)
	fmt.Printf("wrote %s (%d bytes)\n", path, info.Size())
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "profilegen:", err)
	os.Exit(1)
}
