package activity

import (
	"context"
	"fmt"

	"github.com/Sour16o4/profilegen/internal/github"
)

// FromAPI adapts internal/github's live client into this package's shapes.
// This is the only data source now — see activity.go's package comment.
type FromAPI struct {
	Client *github.Client
	Login  string // profile owner's username
	Limit  int    // how many commits to keep
}

// ContributionCalendar mirrors github.ContributionCalendar — kept as this
// package's own type so the renderer depends on internal/activity, not
// internal/github, directly.
type ContributionCalendar struct {
	TotalContributions int
	Weeks              []ContributionWeek
}

// Commits fetches the newest real commits (events feed, falling back to
// per-repo lists — see github.Client.RecentCommits) with real diffstat.
// An empty result is an error, not zero commits shown: GitHub accounts with
// any real activity always have *something* in the events feed or repo
// commit lists, so empty means the fetch went wrong, not that nothing
// happened.
func (a FromAPI) Commits(ctx context.Context) ([]Commit, error) {
	raw, err := a.Client.RecentCommits(ctx, a.Login, a.Limit)
	if err != nil {
		return nil, fmt.Errorf("fetching commits: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("no commits found for %s (events feed and per-repo fallback both empty) — treating as a failure, not zero activity", a.Login)
	}

	out := make([]Commit, len(raw))
	for i, c := range raw {
		out[i] = Commit{
			SHA:       c.SHA,
			Repo:      c.Repo,
			Message:   c.Message,
			Additions: c.Additions,
			Deletions: c.Deletions,
			Date:      c.Date,
		}
	}
	return out, nil
}

// Contributions fetches the live contribution calendar via GraphQL. An
// empty result is an error, not a zero-contribution grid.
func (a FromAPI) Contributions(ctx context.Context) (ContributionCalendar, error) {
	raw, err := a.Client.Contributions(ctx, a.Login)
	if err != nil {
		return ContributionCalendar{}, fmt.Errorf("fetching contribution calendar: %w", err)
	}
	if len(raw.Weeks) == 0 {
		return ContributionCalendar{}, fmt.Errorf("contribution calendar came back empty for %s — treating as a failure, not zero contributions", a.Login)
	}

	weeks := make([]ContributionWeek, len(raw.Weeks))
	for i, w := range raw.Weeks {
		days := make([]ContributionDay, len(w.ContributionDays))
		for j, d := range w.ContributionDays {
			days[j] = ContributionDay{Date: d.Date, ContributionCount: d.ContributionCount}
		}
		weeks[i] = ContributionWeek{ContributionDays: days}
	}
	return ContributionCalendar{TotalContributions: raw.TotalContributions, Weeks: weeks}, nil
}
