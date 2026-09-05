package github

import (
	"context"
	"fmt"
	"sort"
)

// Commit is the subset of REST commit data this project needs. Additions
// and deletions aren't in the list endpoint — they require a second,
// per-commit fetch (see RecentCommits).
type Commit struct {
	SHA        string
	Repo       string // short name, not full_name
	Message    string
	Additions  int
	Deletions  int
	AuthorDate string // RFC3339
}

// CommitFetcher is the interface internal/activity's loader selection
// swaps on.
type CommitFetcher interface {
	RecentCommits(ctx context.Context, repos []Repo, limit int) ([]Commit, error)
}

type listCommitEntry struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Date string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
}

type commitDetail struct {
	Stats struct {
		Additions int `json:"additions"`
		Deletions int `json:"deletions"`
	} `json:"stats"`
}

// RecentCommits gathers the newest commit from each active repo's default
// branch, merges across repos, and returns the newest `limit` overall —
// each with real additions/deletions from a follow-up per-commit fetch
// (the list endpoint doesn't carry stats).
//
// This is at most len(repos) + limit API calls, not one per commit in
// history — bounded and predictable for a nightly cron.
func (c *Client) RecentCommits(ctx context.Context, repos []Repo, limit int) ([]Commit, error) {
	var candidates []Commit
	for _, r := range repos {
		var entries []listCommitEntry
		path := fmt.Sprintf("/repos/%s/commits?per_page=5", r.FullName)
		if err := c.doREST(ctx, path, &entries); err != nil {
			return nil, fmt.Errorf("listing commits for %s: %w", r.FullName, err)
		}
		for _, e := range entries {
			candidates = append(candidates, Commit{
				SHA:        e.SHA,
				Repo:       r.Name,
				Message:    e.Commit.Message,
				AuthorDate: e.Commit.Author.Date,
			})
		}
	}

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].AuthorDate > candidates[j].AuthorDate })
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	for i, cm := range candidates {
		var detail commitDetail
		// full_name isn't on Commit — reconstruct via the repo list would
		// need a lookup; simplest correct path is repo owner/name from the
		// candidate's own Repo field is not sufficient alone, so callers
		// must pass repos with FullName and we match by short name here.
		full := fullNameFor(repos, cm.Repo)
		path := fmt.Sprintf("/repos/%s/commits/%s", full, cm.SHA)
		if err := c.doREST(ctx, path, &detail); err != nil {
			return nil, fmt.Errorf("fetching stats for %s@%s: %w", cm.Repo, cm.SHA, err)
		}
		candidates[i].Additions = detail.Stats.Additions
		candidates[i].Deletions = detail.Stats.Deletions
	}

	return candidates, nil
}

func fullNameFor(repos []Repo, shortName string) string {
	for _, r := range repos {
		if r.Name == shortName {
			return r.FullName
		}
	}
	return shortName
}
