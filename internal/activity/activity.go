// Package activity loads commit and contribution-calendar fixtures shaped
// exactly like the real GitHub API responses (build spec §7), so wiring in
// the real client later is a data-source swap, not a renderer change.
package activity

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Commit mirrors the fields used from GitHub REST's
// /repos/{owner}/{repo}/commits response.
type Commit struct {
	SHA       string `yaml:"sha"`
	Repo      string `yaml:"repo"`
	Message   string `yaml:"message"`
	Additions int    `yaml:"additions"`
	Deletions int    `yaml:"deletions"`
	Date      string `yaml:"date"` // RFC3339
}

type commitsFile struct {
	Commits []Commit `yaml:"commits"`
}

// LoadCommits reads commits.yaml.
func LoadCommits(path string) ([]Commit, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f commitsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return f.Commits, nil
}

// RelativeTime formats an RFC3339 timestamp relative to now, GitHub-style.
func RelativeTime(rfc3339 string, now time.Time) string {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return rfc3339
	}
	d := now.Sub(t)
	switch {
	case d < time.Hour:
		m := int(d.Minutes())
		if m < 1 {
			m = 1
		}
		return plural(m, "minute")
	case d < 24*time.Hour:
		return plural(int(d.Hours()), "hour")
	case d < 30*24*time.Hour:
		return plural(int(d.Hours()/24), "day")
	default:
		return plural(int(d.Hours()/24/30), "month")
	}
}

func plural(n int, unit string) string {
	s := ""
	if n != 1 {
		s = "s"
	}
	return itoa(n) + " " + unit + s + " ago"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ContributionDay mirrors one entry in GitHub GraphQL's
// contributionsCollection.contributionCalendar.weeks[].contributionDays.
type ContributionDay struct {
	Date              string `yaml:"date"`
	ContributionCount int    `yaml:"contributionCount"`
}

// ContributionWeek mirrors one weeks[] entry.
type ContributionWeek struct {
	ContributionDays []ContributionDay `yaml:"contributionDays"`
}

type contributionsFile struct {
	Weeks []ContributionWeek `yaml:"weeks"`
}

// LoadContributions reads contributions.yaml.
func LoadContributions(path string) ([]ContributionWeek, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f contributionsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return f.Weeks, nil
}

// Streaks holds the three numbers the footer line reports.
type Streaks struct {
	Total   int
	Longest int
	Current int
}

// ComputeStreaks walks the calendar in date order (weeks are already
// chronological, Sunday-first columns) to total contributions and find the
// longest streak and the streak still running at the most recent day.
func ComputeStreaks(weeks []ContributionWeek) Streaks {
	var s Streaks
	running := 0
	for _, w := range weeks {
		for _, d := range w.ContributionDays {
			s.Total += d.ContributionCount
			if d.ContributionCount > 0 {
				running++
				if running > s.Longest {
					s.Longest = running
				}
			} else {
				running = 0
			}
		}
	}
	s.Current = running
	return s
}
