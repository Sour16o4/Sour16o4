// Package projects loads the hand-maintained projects.yaml (build spec §7) —
// the one place progress percentages are a judgement call, not derived data.
// It also carries the footer's link URLs, kept in the same file per §5 item 10.
package projects

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Project is one entry under projects.yaml's `projects:` list.
type Project struct {
	Name     string `yaml:"name"`
	Blurb    string `yaml:"blurb"`
	Progress int    `yaml:"progress"` // only meaningful when Active
	Active   bool   `yaml:"active"`
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

// Active returns entries with active: true — rendered in "working on now".
func Active(all []Project) []Project {
	var out []Project
	for _, p := range all {
		if p.Active {
			out = append(out, p)
		}
	}
	return out
}

// Shipped returns entries with active: false — rendered in "shipped".
func Shipped(all []Project) []Project {
	var out []Project
	for _, p := range all {
		if !p.Active {
			out = append(out, p)
		}
	}
	return out
}
