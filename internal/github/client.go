// Package github is the live data client: four surfaces (repos, commits,
// contributions, actions) behind interfaces, so internal/activity and
// internal/projects can swap between this and their fixture loaders without
// the renderers ever knowing which one is in play.
//
// Auth: GITHUB_TOKEN and/or PROFILE_TOKEN from the environment, never
// committed. When run from GitHub Actions, GITHUB_TOKEN is a
// repository-scoped installation token with no authenticated user behind
// it — the entire /user/* REST family and the GraphQL contributionsCollection
// field are unavailable to it structurally, not as a permissions setting
// that could be granted. PROFILE_TOKEN, when set, is a real user token and
// works everywhere GITHUB_TOKEN does plus everywhere it doesn't, so it is
// preferred for every call this client makes, not only as a
// contributions-specific fallback. There is no unauthenticated fallback:
// every surface here needs at least one of the two tokens to be worth
// calling.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	restBase            = "https://api.github.com"
	graphqlURL          = "https://api.github.com/graphql"
	tokenEnvVar         = "GITHUB_TOKEN"
	fallbackTokenEnvVar = "PROFILE_TOKEN"
)

// RateLimitError means the call was rejected for rate limiting, not a real
// failure. Reset is when the limit clears. Callers must not hot-loop on
// this — surface it and stop; a nightly cron gets another chance tomorrow.
type RateLimitError struct {
	Reset time.Time
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited until %s", e.Reset.Format(time.RFC3339))
}

// HTTPError carries the real status code back to the caller — callers that
// need to tell "this one repo is empty (409) or gone (404), skip it" apart
// from "the token is bad (401/403), stop everything" can't do that from a
// pre-formatted error string. Every non-200, non-rate-limited doREST
// response returns one of these.
type HTTPError struct {
	StatusCode int
	Status     string
	Path       string
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("GET %s: %s: %s", e.Path, e.Status, e.Body)
}

// Client wraps http.Client with GitHub auth and rate-limit handling.
type Client struct {
	http          *http.Client
	token         string // GITHUB_TOKEN
	fallbackToken string // PROFILE_TOKEN, optional — preferred over token when set; see effectiveToken
}

// NewClient reads GITHUB_TOKEN from the environment. Returns an error if
// it's unset — every surface here needs at least read auth to be worth
// calling (GraphQL requires it outright, and REST's rate limit without it
// is too thin for a nightly cron across several repos). PROFILE_TOKEN is
// optional: a fine-grained PAT with read:user. When set, it is used for
// every call this client makes (see effectiveToken), not only as a
// contributions-specific fallback — it is a real user token and works
// against endpoints the default Actions token structurally cannot reach.
func NewClient() (*Client, error) {
	token := os.Getenv(tokenEnvVar)
	if token == "" {
		return nil, fmt.Errorf("%s is not set", tokenEnvVar)
	}
	return &Client{
		http:          &http.Client{Timeout: 30 * time.Second},
		token:         token,
		fallbackToken: os.Getenv(fallbackTokenEnvVar),
	}, nil
}

// effectiveToken is the token used for every API call. PROFILE_TOKEN, when
// set, is preferred over GITHUB_TOKEN unconditionally: it's a real
// user-authenticated token and works against every endpoint this client
// calls, whereas GITHUB_TOKEN (a repository-scoped Actions installation
// token when run in CI) does not — the /user/* REST family and the GraphQL
// contributionsCollection field are unavailable to it structurally.
func (c *Client) effectiveToken() string {
	if c.fallbackToken != "" {
		return c.fallbackToken
	}
	return c.token
}

// doREST performs an authenticated REST GET and decodes the JSON body into out.
func (c *Client) doREST(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, restBase+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.effectiveToken())
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if rlErr := checkRateLimit(resp); rlErr != nil {
		return rlErr
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &HTTPError{StatusCode: resp.StatusCode, Status: resp.Status, Path: path, Body: string(body)}
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// doGraphQL performs an authenticated GraphQL POST using effectiveToken.
func (c *Client) doGraphQL(ctx context.Context, query string, variables map[string]any, out any) error {
	payload, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphqlURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.effectiveToken())
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if rlErr := checkRateLimit(resp); rlErr != nil {
		return rlErr
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST graphql: %s: %s", resp.Status, string(body))
	}

	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("graphql error: %s", envelope.Errors[0].Message)
	}
	return json.Unmarshal(envelope.Data, out)
}

// checkRateLimit reads x-ratelimit-remaining/x-ratelimit-reset. Per the
// brief: respect the reset header, never hot-loop — this returns a typed
// error immediately instead of sleeping or retrying, so the caller (the
// failure-behavior decision in cmd/profilegen) controls what happens next.
func checkRateLimit(resp *http.Response) error {
	remaining := resp.Header.Get("x-ratelimit-remaining")
	if remaining != "0" {
		return nil
	}
	resetHeader := resp.Header.Get("x-ratelimit-reset")
	resetUnix, err := strconv.ParseInt(resetHeader, 10, 64)
	if err != nil {
		return &RateLimitError{Reset: time.Now().Add(time.Hour)} // unknown reset, assume the standard window
	}
	return &RateLimitError{Reset: time.Unix(resetUnix, 0)}
}
