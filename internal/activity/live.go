package activity

import (
	"context"
	"fmt"

	"github.com/Sour16o4/profilegen/internal/github"
)

// FromAPI adapts internal/github's live client into this package's Commit
// and ContributionWeek shapes — the same types the fixture loaders return,
// so cmd/profilegen picks one or the other without the renderers ever
// knowing which is in play.
type FromAPI struct {
	Client *github.Client
	Login  string // profile owner's username, for the contributions query
	Limit  int    // how many commits to keep
}

// Commits fetches active repos, then the newest commits across them.
func (a FromAPI) Commits(ctx context.Context) ([]Commit, error) {
	repos, err := a.Client.Repos(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing repos: %w", err)
	}
	active := github.Active(repos)
	if len(active) == 0 {
		return nil, fmt.Errorf("no active (non-fork, non-archived) repos found for %s", a.Login)
	}

	raw, err := a.Client.RecentCommits(ctx, active, a.Limit)
	if err != nil {
		return nil, fmt.Errorf("fetching commits: %w", err)
	}

	out := make([]Commit, len(raw))
	for i, c := range raw {
		out[i] = Commit{
			SHA:       c.SHA,
			Repo:      c.Repo,
			Message:   c.Message,
			Additions: c.Additions,
			Deletions: c.Deletions,
			Date:      c.AuthorDate,
		}
	}
	return out, nil
}

// Contributions fetches the live contribution calendar via GraphQL.
func (a FromAPI) Contributions(ctx context.Context) ([]ContributionWeek, error) {
	raw, err := a.Client.Contributions(ctx, a.Login)
	if err != nil {
		return nil, fmt.Errorf("fetching contribution calendar: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("contribution calendar came back empty for %s — treating as a failure, not zero contributions", a.Login)
	}

	out := make([]ContributionWeek, len(raw))
	for i, w := range raw {
		days := make([]ContributionDay, len(w.ContributionDays))
		for j, d := range w.ContributionDays {
			days[j] = ContributionDay{Date: d.Date, ContributionCount: d.ContributionCount}
		}
		out[i] = ContributionWeek{ContributionDays: days}
	}
	return out, nil
}
