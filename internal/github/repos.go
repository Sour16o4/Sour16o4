package github

import (
	"context"
	"fmt"
	"sort"
)

// Repo is the subset of REST /user/repos this project needs.
type Repo struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Fork     bool   `json:"fork"`
	Archived bool   `json:"archived"`
	Private  bool   `json:"private"`
	PushedAt string `json:"pushed_at"` // RFC3339
}

// RepoLister is the interface internal/activity's loader selection swaps
// on — a fixture-backed implementation satisfies the same shape.
type RepoLister interface {
	Repos(ctx context.Context) ([]Repo, error)
}

// Repos fetches the authenticated user's repos (REST /user/repos), newest
// pushed first. This feeds the commits surface — it needs to know which
// repos are "active" before it can pull commits from them.
func (c *Client) Repos(ctx context.Context) ([]Repo, error) {
	var all []Repo
	for page := 1; ; page++ {
		var batch []Repo
		path := fmt.Sprintf("/user/repos?per_page=100&page=%d&sort=pushed", page)
		if err := c.doREST(ctx, path, &batch); err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) < 100 {
			break
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].PushedAt > all[j].PushedAt })
	return all, nil
}

// Active filters out forks, archived repos, and private repos — what
// "active repos" means for the commits surface.
//
// Private matters for a reason beyond "don't show what isn't public":
// Repos() enumerates via /user/repos with an authenticated token, which
// lists every repo the token's owner can see, private ones included. Without
// this filter, a private repo's commits could reach the public profile page
// — a real disclosure risk, not just noise. It also removes fish's 409 at
// the source: fish is private, invisible to unauthenticated requests
// (hence 404 from outside, per the earlier investigation), and now never
// enters the commits pool to fail against in the first place.
func Active(repos []Repo) []Repo {
	var out []Repo
	for _, r := range repos {
		if !r.Fork && !r.Archived && !r.Private {
			out = append(out, r)
		}
	}
	return out
}
