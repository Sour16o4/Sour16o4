// Command profilegen generates the SVG sections for the Sour16o4 GitHub
// profile README (see build spec, §8).
//
// Data source: if GITHUB_TOKEN is set, this pulls live data (repos, commits,
// contributions) from the GitHub API; otherwise it falls back to the
// committed YAML fixtures, which is the local-dev default (no token
// required to iterate on a renderer).
//
// Failure behaviour (decided, not defaulted): in live mode, every fetch
// happens BEFORE any file is written. If anything fails — rate limit, auth,
// network, an empty response where one isn't expected — the run exits
// non-zero and touches nothing on disk. The previous run's committed assets
// stay live and correct, just one day stale, rather than the page silently
// showing fabricated fixture data or a partially-regenerated, inconsistent
// asset set. A failed nightly cron is visible in the Actions tab; a silent
// fallback to placeholder numbers on a public profile is not.
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
	// Deterministic order for the printed report, not the write itself.
	for _, path := range sortedKeys(o.files) {
		fmt.Printf("wrote %s (%d bytes)\n", path, len(o.files[path]))
	}
}

func main() {
	ctx := context.Background()
	out := newOutput()

	commits, weeks := loadActivity(ctx)
	allProjects, links, err := projects.Load("projects.yaml")
	if err != nil {
		fatal(err)
	}
	active := projects.Active(allProjects)
	shipped := projects.Shipped(allProjects)

	out.set("assets/pipeline-dark.svg", render.Pipeline(theme.Dark, render.DefaultStages))
	out.set("assets/pipeline-light.svg", render.Pipeline(theme.Light, render.DefaultStages))

	out.set("assets/arch-dark.svg", render.Architecture(theme.Dark, render.DefaultArch))
	out.set("assets/arch-light.svg", render.Architecture(theme.Light, render.DefaultArch))

	const name = "SOURAV SALAMPURIA"
	const subtitle = "backend engineer · gurgaon, in · open to roles"
	out.set("assets/header-dark.svg", render.Header(theme.Dark, name, subtitle, render.DefaultTyping))
	out.set("assets/header-light.svg", render.Header(theme.Light, name, subtitle, render.DefaultTyping))

	out.set("assets/now-dark.svg", render.Projects(theme.Dark, "Working on now", active, true))
	out.set("assets/now-light.svg", render.Projects(theme.Light, "Working on now", active, true))
	out.set("assets/shipped-dark.svg", render.Projects(theme.Dark, "Shipped", shipped, false))
	out.set("assets/shipped-light.svg", render.Projects(theme.Light, "Shipped", shipped, false))

	now := time.Now()
	out.set("assets/commits-dark.svg", render.Commits(theme.Dark, commits, now))
	out.set("assets/commits-light.svg", render.Commits(theme.Light, commits, now))

	out.set("assets/contributions-dark.svg", render.Contributions(theme.Dark, weeks, theme.HeatmapRampDark))
	out.set("assets/contributions-light.svg", render.Contributions(theme.Light, weeks, theme.HeatmapRampLight))

	out.set("assets/chips-dark.svg", render.Chips(theme.Dark, render.DefaultChips))
	out.set("assets/chips-light.svg", render.Chips(theme.Light, render.DefaultChips))

	out.set("README.md", buildReadme(links))

	out.flush()
}

// loadActivity picks live API data (GITHUB_TOKEN set) or the committed
// fixtures (local dev default), and enforces the failure behaviour: in live
// mode, any error here must stop the run before main() ever calls out.flush().
func loadActivity(ctx context.Context) ([]activity.Commit, []activity.ContributionWeek) {
	client, err := ghclient.NewClient()
	if err != nil {
		fmt.Println("GITHUB_TOKEN not set — using committed fixtures (local-dev mode)")
		return loadFixtures()
	}

	fmt.Println("GITHUB_TOKEN set — fetching live data")
	source := activity.FromAPI{Client: client, Login: profileLogin, Limit: commitLimit}

	commits, err := source.Commits(ctx)
	if err != nil {
		fatalLive("commits", err)
	}
	weeks, err := source.Contributions(ctx)
	if err != nil {
		fatalLive("contributions", err)
	}
	return commits, weeks
}

func loadFixtures() ([]activity.Commit, []activity.ContributionWeek) {
	commits, err := activity.LoadCommits("commits.yaml")
	if err != nil {
		fatal(err)
	}
	weeks, err := activity.LoadContributions("contributions.yaml")
	if err != nil {
		fatal(err)
	}
	return commits, weeks
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

func buildReadme(links projects.Links) string {
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
	return b.String()
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
