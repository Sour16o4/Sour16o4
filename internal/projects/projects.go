// Package projects loads the hand-maintained projects.yaml — real,
// checkable repo data only. No project appears here unless it corresponds
// to a public repo on github.com/Sour16o4 right now; no card carries a
// number that doesn't trace back to something in this file (which in turn
// should trace back to the repo itself: language, last push date, status).
// It also carries the footer's link URLs, kept in the same file per §5
// item 10.
package projects

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Project is one entry under projects.yaml's `projects:` list. Every field
// is a fact you can check against the repo, not a judgment call — there is
// deliberately no "progress percentage" here; a percentage with nothing
// behind it is exactly what got cut.
type Project struct {
	Name     string `yaml:"name"`
	Blurb    string `yaml:"blurb"` // verbatim GitHub description, or an honest note if there isn't one
	Language string `yaml:"language"`
	LastPush string `yaml:"last_push"` // YYYY-MM-DD
	Status   string `yaml:"status"`    // "active" -> working on now, "shipped" -> shipped
	RepoURL  string `yaml:"repo_url"`
}

// Links holds the footer's four URLs (§5 item 10). The footer ships as
// markdown, not SVG — links inside an SVG don't work once GitHub serves it
// as a plain image (§1) — so these feed README generation directly, not a
// renderer.
type Links struct {
	Portfolio string `yaml:"portfolio"`
	LinkedIn  string `yaml:"linkedin"`
	Email     string `yaml:"email"`
	Resume    string `yaml:"resume"`
}

type file struct {
	Projects []Project `yaml:"projects"`
	Links    Links     `yaml:"links"`
}

// Load reads and parses projects.yaml from path.
func Load(path string) ([]Project, Links, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, Links{}, err
	}
	var f file
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, Links{}, err
	}
	return f.Projects, f.Links, nil
}

// Active returns status: active entries — rendered in "working on now".
func Active(all []Project) []Project {
	var out []Project
	for _, p := range all {
		if p.Status == "active" {
			out = append(out, p)
		}
	}
	return out
}

// Shipped returns status: shipped entries — rendered in "shipped".
func Shipped(all []Project) []Project {
	var out []Project
	for _, p := range all {
		if p.Status == "shipped" {
			out = append(out, p)
		}
	}
	return out
}
