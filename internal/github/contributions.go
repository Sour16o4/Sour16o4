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
// REST has no equivalent for this data. Tries the primary token
// (GITHUB_TOKEN) first; if that comes back either rejected OR with an
// empty/null calendar, and PROFILE_TOKEN is set, retries the identical
// query with it.
//
// Both failure shapes are handled deliberately, not just an outright
// rejection: a repository-scoped Actions token querying a user-scoped field
// like contributionsCollection is at least as likely to come back a
// *successful* response with a null/empty field as an auth error — GraphQL
// doesn't have to reject the request to simply not have the data for that
// token. Keying the retry on rejection alone would leave that case falling
// straight through to "zero contributions," which is exactly the kind of
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

	cal, err := c.contributionCalendar(ctx, c.token, login, variables)
	primaryEmpty := err == nil && len(cal.Weeks) == 0

	if (err != nil || primaryEmpty) && c.fallbackToken != "" {
		fbCal, fbErr := c.contributionCalendar(ctx, c.fallbackToken, login, variables)
		switch {
		case fbErr != nil:
			return ContributionCalendar{}, fmt.Errorf("GITHUB_TOKEN %s, PROFILE_TOKEN also failed: %w", primaryFailureDesc(err, primaryEmpty), fbErr)
		case len(fbCal.Weeks) == 0:
			return ContributionCalendar{}, fmt.Errorf("GITHUB_TOKEN %s, PROFILE_TOKEN returned an empty calendar too", primaryFailureDesc(err, primaryEmpty))
		default:
			return fbCal, nil
		}
	}

	if err != nil {
		return ContributionCalendar{}, err
	}
	if primaryEmpty {
		return ContributionCalendar{}, fmt.Errorf("contribution calendar came back empty for %s and no PROFILE_TOKEN is set to retry with", login)
	}
	return cal, nil
}

func primaryFailureDesc(err error, empty bool) string {
	if empty {
		return "returned an empty calendar"
	}
	return fmt.Sprintf("was rejected (%v)", err)
}

// contributionCalendar runs the query with a specific token — the one place
// both Contributions' primary and fallback attempts share, so "does the
// response actually have data" is checked identically either way.
func (c *Client) contributionCalendar(ctx context.Context, token, login string, variables map[string]any) (ContributionCalendar, error) {
	var resp contributionsResponse
	if err := c.doGraphQLAs(ctx, token, contributionsQuery, variables, &resp); err != nil {
		return ContributionCalendar{}, err
	}
	cal := resp.User.ContributionsCollection.ContributionCalendar
	return ContributionCalendar{TotalContributions: cal.TotalContributions, Weeks: cal.Weeks}, nil
}
