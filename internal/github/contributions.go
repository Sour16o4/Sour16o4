package github

import (
	"context"
	"fmt"
)

// ContributionDay/ContributionWeek mirror GraphQL's
// contributionsCollection.contributionCalendar shape directly — this is
// intentionally identical to internal/activity's structs, so the adapter
// between them is a type conversion, not a data transformation.
type ContributionDay struct {
	Date              string `json:"date"`
	ContributionCount int    `json:"contributionCount"`
}

type ContributionWeek struct {
	ContributionDays []ContributionDay `json:"contributionDays"`
}

// ContributionCalendar is the full query result: the day-by-day breakdown
// plus GitHub's own total, so the renderer never has to trust a re-sum of
// the days matches what github.com itself shows for "contributions in the
// last year" — it's the same number, straight from the source.
type ContributionCalendar struct {
	TotalContributions int
	Weeks              []ContributionWeek
}

// ContributionFetcher is the interface internal/activity's loader
// selection swaps on.
type ContributionFetcher interface {
	Contributions(ctx context.Context, login string) (ContributionCalendar, error)
}

const contributionsQuery = `
query($login: String!) {
  user(login: $login) {
    contributionsCollection {
      contributionCalendar {
        totalContributions
        weeks {
          contributionDays {
            date
            contributionCount
          }
        }
      }
    }
  }
}`

type contributionsResponse struct {
	User struct {
		ContributionsCollection struct {
			ContributionCalendar struct {
				TotalContributions int                `json:"totalContributions"`
				Weeks              []ContributionWeek `json:"weeks"`
			} `json:"contributionCalendar"`
		} `json:"contributionsCollection"`
	} `json:"user"`
}

// Contributions fetches the last year's contribution calendar via GraphQL —
// REST has no equivalent for this data. Runs the query with effectiveToken
// (PROFILE_TOKEN when set, otherwise GITHUB_TOKEN) — see client.go. There is
// no retry with a second token here: PROFILE_TOKEN is already used first
// whenever it's set, so a GITHUB_TOKEN-only environment retrying against
// itself would just repeat the identical failure.
//
// An empty calendar is treated as a failure, not a legitimate zero: a
// repository-scoped Actions token querying a user-scoped field like
// contributionsCollection is at least as likely to come back a *successful*
// response with a null/empty field as an auth error — GraphQL doesn't have
// to reject the request to simply not have the data for that token. Letting
// that fall through as "zero contributions" would be exactly the kind of
// fake-looking result this project isn't supposed to produce silently.
//
// NOTE on pagination: contributionCalendar.weeks is NOT a paginated
// connection in GitHub's schema — one query returns the full ~52-53 weeks
// for the requested window, no cursor involved. There is nothing to page
// through here. Multi-year history would mean separate queries with
// different `from`/`to` variables (one call per ~year), not cursor
// pagination.
func (c *Client) Contributions(ctx context.Context, login string) (ContributionCalendar, error) {
	variables := map[string]any{"login": login}

	var resp contributionsResponse
	if err := c.doGraphQL(ctx, contributionsQuery, variables, &resp); err != nil {
		return ContributionCalendar{}, err
	}
	cal := resp.User.ContributionsCollection.ContributionCalendar
	if len(cal.Weeks) == 0 {
		return ContributionCalendar{}, fmt.Errorf("contribution calendar came back empty for %s", login)
	}
	return ContributionCalendar{TotalContributions: cal.TotalContributions, Weeks: cal.Weeks}, nil
}
