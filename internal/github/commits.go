package github

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Commit is the subset of commit data this project needs. Additions and
// deletions aren't in either source below — they require a second,
// per-commit fetch (see attachStats).
type Commit struct {
	SHA       string
	Repo      string // short name (e.g. "tenantguard"), not full_name
	Message   string
	Additions int
	Deletions int
	Date      string // RFC3339 — from the push event's created_at (primary source) or the commit's own author date (fallback source)
}

// CommitFetcher is the interface internal/activity's loader selection
// swaps on.
type CommitFetcher interface {
	RecentCommits(ctx context.Context, login string, limit int) ([]Commit, error)
}

// pushEvent is the subset of GET /users/{login}/events/public this project
// needs — specifically PushEvent entries.
type pushEvent struct {
	Type      string `json:"type"`
	CreatedAt string `json:"created_at"`
	Repo      struct {
		Name string `json:"name"` // "owner/repo"
	} `json:"repo"`
	Payload struct {
		Commits []struct {
			SHA     string `json:"sha"`
			Message string `json:"message"`
		} `json:"commits"`
	} `json:"payload"`
}

type listCommitEntry struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Date string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
	Parents []struct {
		SHA string `json:"sha"`
	} `json:"parents"`
}

// mergeMessagePattern is the fallback merge detector for sources that don't
// carry parent data — the events feed's payload.commits has no `parents`
// field at all, so a merge commit arriving that way can only be recognized
// by its GitHub-generated message shape.
var mergeMessagePattern = regexp.MustCompile(`(?i)^Merge (pull request|branch) `)

// isMergeCommit detects GitHub-generated merge commits: more than one
// parent is the definitive signal (a merge commit always has 2+ parents,
// a regular commit always has exactly 1); the message pattern is only a
// fallback for when parent data isn't available at all. Deliberately narrow
// — this project doesn't filter by commit type or message content beyond
// merges specifically.
func isMergeCommit(parentCount int, message string) bool {
	if parentCount > 1 {
		return true
	}
	if parentCount == 0 { // parent data unavailable (events source) — fall back to message shape
		return mergeMessagePattern.MatchString(message)
	}
	return false
}

type commitDetail struct {
	Stats struct {
		Additions int `json:"additions"`
		Deletions int `json:"deletions"`
	} `json:"stats"`
}

// perRepoCommitCap bounds how many of one repo's commits can enter the
// merged candidate pool before the final sort+truncate to `limit`. Without
// this, a repo with a burst of very recent activity — the profile repo
// itself while this generator was actively being built, in practice — can
// win every slot in the final merge, and "recent commits" stops meaning
// "across your projects" and starts meaning "the one repo you touched
// today." A small cap trades a little recency for guaranteed diversity.
const perRepoCommitCap = 2

// isProfileRepo reports whether repoName is the profile repo itself
// (case-insensitive: GitHub repo names are case-insensitive for routing
// purposes, so "sour16o4" and "Sour16o4" are the same repo). Excluded from
// both commit sources — a profile README reporting commits made to build
// the profile README is noise, not evidence of work on real projects.
func isProfileRepo(repoName, login string) bool {
	return strings.EqualFold(repoName, login)
}

// RecentCommits tries the public events feed first (it carries real commit
// SHAs and messages from actual pushes, in one call); if that yields
// nothing — a quiet week, or events aged out of GitHub's ~90-day window —
// it falls back to per-repo commit lists filtered to this author, merged
// and sorted. Either way, the final `limit` commits get a follow-up fetch
// for real additions/deletions.
func (c *Client) RecentCommits(ctx context.Context, login string, limit int) ([]Commit, error) {
	commits, err := c.commitsFromEvents(ctx, login, limit)
	if err != nil {
		return nil, fmt.Errorf("fetching public events: %w", err)
	}

	if len(commits) == 0 {
		commits, err = c.commitsFromRepos(ctx, login, limit)
		if err != nil {
			return nil, fmt.Errorf("falling back to per-repo commits: %w", err)
		}
	}

	if len(commits) > limit {
		commits = commits[:limit]
	}
	if err := c.attachStats(ctx, commits, login); err != nil {
		return nil, fmt.Errorf("fetching commit stats: %w", err)
	}
	return commits, nil
}

// commitsFromEvents is the primary source: GET /users/{login}/events/public,
// filtered to PushEvent, flattened in the order GitHub returns events
// (newest first) so the result is already roughly date-sorted without
// needing per-commit timestamps the events payload doesn't provide.
func (c *Client) commitsFromEvents(ctx context.Context, login string, limit int) ([]Commit, error) {
	var events []pushEvent
	path := fmt.Sprintf("/users/%s/events/public?per_page=100", login)
	if err := c.doREST(ctx, path, &events); err != nil {
		return nil, err
	}

	var commits []Commit
	for _, e := range events {
		if e.Type != "PushEvent" || len(e.Payload.Commits) == 0 {
			continue
		}
		repoShort := e.Repo.Name
		if idx := strings.LastIndex(repoShort, "/"); idx >= 0 {
			repoShort = repoShort[idx+1:]
		}
		if isProfileRepo(repoShort, login) {
			continue
		}
		// A single push can carry several commits; within one push, list
		// them newest-last-in-payload-first since GitHub orders payload
		// commits oldest-to-newest within a push.
		for i := len(e.Payload.Commits) - 1; i >= 0; i-- {
			pc := e.Payload.Commits[i]
			if isMergeCommit(0, pc.Message) { // no parent data in the events payload — message is the only signal here
				continue
			}
			commits = append(commits, Commit{
				SHA:     pc.SHA,
				Repo:    repoShort,
				Message: pc.Message,
				Date:    e.CreatedAt,
			})
			if len(commits) >= limit {
				return commits, nil
			}
		}
	}
	return commits, nil
}

// commitsFromRepos is the fallback: per-repo commit lists filtered to this
// author, capped per repo, merged and sorted by real commit date.
//
// Repos() -> Active() already excludes forks and archived repos before this
// loop ever runs (see repos.go) — that filtering doesn't belong here too.
// The profile repo itself is excluded here (see isProfileRepo).
//
// A single repo failing must not kill the whole run: an empty repo (409
// Conflict — no commits, no default branch) or one that's gone/inaccessible
// (404) is a fact about that repo, not a reason to abandon every other one.
// Both are skipped, logged, and the loop continues. A 401/403 means the
// token itself is bad — that applies to every remaining call too, so it
// aborts immediately rather than repeating the same failure per repo. Any
// other status is treated the same as 401/403: unexpected enough that
// continuing past it silently is riskier than stopping.
func (c *Client) commitsFromRepos(ctx context.Context, login string, limit int) ([]Commit, error) {
	repos, err := c.Repos(ctx, login)
	if err != nil {
		return nil, fmt.Errorf("listing repos: %w", err)
	}
	active := Active(repos)

	var candidates []Commit
	skipped := 0
	excluded := 0
	for _, r := range active {
		if isProfileRepo(r.Name, login) {
			excluded++
			continue
		}
		var entries []listCommitEntry
		path := fmt.Sprintf("/repos/%s/commits?author=%s&per_page=10", r.FullName, login)
		err := c.doREST(ctx, path, &entries)
		if err != nil {
			var httpErr *HTTPError
			if errors.As(err, &httpErr) && (httpErr.StatusCode == 409 || httpErr.StatusCode == 404) {
				fmt.Fprintf(os.Stderr, "profilegen: skipping %s: HTTP %d (%s)\n", r.FullName, httpErr.StatusCode, httpErr.Status)
				skipped++
				continue
			}
			return nil, fmt.Errorf("listing commits for %s: %w", r.FullName, err)
		}
		// Merge commits are filtered out before the cap, not after — the cap
		// is meant to bound how much of a repo's real work fills the pool;
		// letting GitHub-generated merge commits occupy cap slots that then
		// get discarded would silently shrink that repo's actual
		// contribution below what the cap intends.
		var nonMerge []listCommitEntry
		for _, e := range entries {
			if !isMergeCommit(len(e.Parents), e.Commit.Message) {
				nonMerge = append(nonMerge, e)
			}
		}

		// nonMerge is still newest-first (GitHub's default commit-list
		// order, unaffected by filtering), so capping to the first
		// perRepoCommitCap keeps this repo's most recent real commits
		// without a second sort.
		n := len(nonMerge)
		if n > perRepoCommitCap {
			n = perRepoCommitCap
		}
		for _, e := range nonMerge[:n] {
			candidates = append(candidates, Commit{
				SHA:     e.SHA,
				Repo:    r.Name,
				Message: e.Commit.Message,
				Date:    e.Commit.Author.Date,
			})
		}
	}

	if len(candidates) == 0 {
		queried := len(active) - excluded
		return nil, fmt.Errorf("no commits found across %d active repos (%d excluded as the profile repo, %d skipped on error, %d queried with no commits by %s)",
			len(active), excluded, skipped, queried-skipped, login)
	}

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Date > candidates[j].Date })
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates, nil
}

// attachStats fetches real additions/deletions per commit. Needs each
// commit's full "owner/repo" — reconstructed via a fresh repo list rather
// than threading full names through both call sites above, since this is
// only ever called once per run on at most `limit` commits.
func (c *Client) attachStats(ctx context.Context, commits []Commit, login string) error {
	if len(commits) == 0 {
		return nil
	}
	repos, err := c.Repos(ctx, login)
	if err != nil {
		return fmt.Errorf("listing repos: %w", err)
	}
	for i, cm := range commits {
		full := fullNameFor(repos, cm.Repo)
		var detail commitDetail
		path := fmt.Sprintf("/repos/%s/commits/%s", full, cm.SHA)
		if err := c.doREST(ctx, path, &detail); err != nil {
			return fmt.Errorf("fetching stats for %s@%s: %w", cm.Repo, cm.SHA, err)
		}
		commits[i].Additions = detail.Stats.Additions
		commits[i].Deletions = detail.Stats.Deletions
	}
	return nil
}

func fullNameFor(repos []Repo, shortName string) string {
	for _, r := range repos {
		if r.Name == shortName {
			return r.FullName
		}
	}
	return shortName
}
