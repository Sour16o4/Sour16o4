// Package github is the live data client: four surfaces (repos, commits,
// contributions, actions) behind interfaces, so internal/activity and
// internal/projects can swap between this and their fixture loaders without
// the renderers ever knowing which one is in play.
//
// Auth: GITHUB_TOKEN from the environment, never committed. Unauthenticated
// calls do work for public data on repos/commits/actions (at a much lower
// rate limit), but the GraphQL contributions query requires a token — GitHub
// does not serve GraphQL to anonymous callers at all.
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
	token         string
	fallbackToken string // PROFILE_TOKEN, optional — see Contributions
}

// NewClient reads GITHUB_TOKEN from the environment. Returns an error if
// it's unset — every surface here needs at least read auth to be worth
// calling (GraphQL requires it outright, and REST's rate limit without it
// is too thin for a nightly cron across several repos). PROFILE_TOKEN is
// optional: a fine-grained PAT with read:user, used only as a fallback if
// GITHUB_TOKEN is rejected for the contributions GraphQL query specifically
// (the default Actions token generally can't read another user's — or even
// its own account's — contribution calendar).
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

// doREST performs an authenticated REST GET and decodes the JSON body into out.
func (c *Client) doREST(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, restBase+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
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

// doGraphQL performs an authenticated GraphQL POST using the primary token.
func (c *Client) doGraphQL(ctx context.Context, query string, variables map[string]any, out any) error {
	return c.doGraphQLAs(ctx, c.token, query, variables, out)
}

// doGraphQLAs is doGraphQL with an explicit token — how Contributions falls
// back to PROFILE_TOKEN without a second code path.
func (c *Client) doGraphQLAs(ctx context.Context, token, query string, variables map[string]any, out any) error {
	payload, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphqlURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
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
