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

// Active filters out forks and archived repos — what "active repos" means
// for the commits surface.
func Active(repos []Repo) []Repo {
	var out []Repo
	for _, r := range repos {
		if !r.Fork && !r.Archived {
			out = append(out, r)
		}
	}
	return out
}
