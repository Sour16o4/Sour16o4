package github

import "context"

// ContributionDay/ContributionWeek mirror GraphQL's
// contributionsCollection.contributionCalendar shape directly — this is
// intentionally identical to internal/activity's fixture structs, so the
// adapter between them is a type conversion, not a data transformation.
type ContributionDay struct {
	Date              string `json:"date"`
	ContributionCount int    `json:"contributionCount"`
}

type ContributionWeek struct {
	ContributionDays []ContributionDay `json:"contributionDays"`
}

// ContributionFetcher is the interface internal/activity's loader
// selection swaps on.
type ContributionFetcher interface {
	Contributions(ctx context.Context, login string) ([]ContributionWeek, error)
}

const contributionsQuery = `
query($login: String!) {
  user(login: $login) {
    contributionsCollection {
      contributionCalendar {
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
				Weeks []ContributionWeek `json:"weeks"`
			} `json:"contributionCalendar"`
		} `json:"contributionsCollection"`
	} `json:"user"`
}

// Contributions fetches the last year's contribution calendar via GraphQL —
// REST has no equivalent for this data (§ build spec is explicit on this).
//
// NOTE on pagination: contributionCalendar.weeks is NOT a paginated
// connection in GitHub's schema — one query returns the full ~52-53 weeks
// for the requested window, no cursor involved. There is nothing to page
// through here. If multi-year history is wanted later, that's achieved by
// issuing separate queries with different `from`/`to` variables (one call
// per ~year), not cursor pagination — worth flagging now rather than
// shipping fake pagination code against a field that doesn't have any.
func (c *Client) Contributions(ctx context.Context, login string) ([]ContributionWeek, error) {
	var resp contributionsResponse
	variables := map[string]any{"login": login}
	if err := c.doGraphQL(ctx, contributionsQuery, variables, &resp); err != nil {
		return nil, err
	}
	return resp.User.ContributionsCollection.ContributionCalendar.Weeks, nil
}
