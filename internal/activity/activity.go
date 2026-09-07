// Package activity holds the commit and contribution-calendar data shapes
// and derived stats (relative time, streaks). Data only ever comes from
// the live GitHub client now (see live.go) — there is deliberately no
// fixture loader here anymore. The previous fixture path produced fake
// commit messages and a fake contribution grid that shipped to the live
// profile; removing it entirely, rather than just not calling it, means
// there's no code left that could ever silently reintroduce that.
package activity

import "time"

// Commit is one entry in the recent-commits log.
type Commit struct {
	SHA       string
	Repo      string
	Message   string
	Additions int
	Deletions int
	Date      string // RFC3339
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
	Date              string
	ContributionCount int
}

// ContributionWeek mirrors one weeks[] entry.
type ContributionWeek struct {
	ContributionDays []ContributionDay
}

// Streaks holds the two numbers that have to be walked from the raw day
// array — total contributions comes straight from the API's own
// totalContributions field instead (see github.ContributionCalendar),
// not recomputed here, so there's exactly one source of truth for it.
type Streaks struct {
	Longest int
	Current int
}

// ComputeStreaks walks the calendar in date order (weeks are already
// chronological, Sunday-first columns) to find the longest streak and the
// streak still running at the most recent day. Never hardcoded — always
// derived from whatever day array is passed in.
func ComputeStreaks(weeks []ContributionWeek) Streaks {
	var s Streaks
	running := 0
	for _, w := range weeks {
		for _, d := range w.ContributionDays {
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
