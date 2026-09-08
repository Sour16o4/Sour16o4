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
	Repos(ctx context.Context, login string) ([]Repo, error)
}

// Repos fetches login's repos via REST /users/{login}/repos, newest pushed
// first. This feeds the commits surface — it needs to know which repos are
// "active" before it can pull commits from them.
//
// This is the public listing endpoint, not /user/repos: /user/repos only
// works for a user-authenticated token and 403s outright for a
// repository-scoped Actions installation token (GITHUB_TOKEN in CI has no
// authenticated user behind it at all). /users/{login}/repos works with any
// token — or none — but only ever returns login's *public* repos, which is
// what this page wants to show anyway. The consequence is that every Repo
// returned here has Private == false unconditionally; see Active for why
// the filter stays regardless.
func (c *Client) Repos(ctx context.Context, login string) ([]Repo, error) {
	var all []Repo
	for page := 1; ; page++ {
		var batch []Repo
		path := fmt.Sprintf("/users/%s/repos?per_page=100&page=%d&sort=pushed", login, page)
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
// The !Private check is kept even though Repos() now always returns
// Private == false (see Repos' comment): the real leak guard today is
// GET /users/{login}/repos itself, which only ever lists public repos in
// the first place — a private repo's commits can't reach this filter to
// begin with. The field check stays as a second, cheap guarantee in case
// that ever changes (e.g. a future authenticated call path being added
// back), rather than something this filter still needs to do the work
// itself. It also removes fish's 409 at the source: fish is private,
// invisible to this endpoint (hence 404 from outside, per the earlier
// investigation), and now never enters the commits pool to fail against in
// the first place.
func Active(repos []Repo) []Repo {
	var out []Repo
	for _, r := range repos {
		if !r.Fork && !r.Archived && !r.Private {
			out = append(out, r)
		}
	}
	return out
}
