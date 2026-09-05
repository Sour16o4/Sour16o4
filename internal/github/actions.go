package github

import (
	"context"
	"fmt"
)

// WorkflowRun is the subset of REST /actions/runs this project needs.
type WorkflowRun struct {
	Status     string `json:"status"`     // "completed", "in_progress", ...
	Conclusion string `json:"conclusion"` // "success", "failure", "cancelled", ... (only when completed)
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// ActionsFetcher is the interface internal/activity's loader selection
// swaps on.
type ActionsFetcher interface {
	WorkflowRuns(ctx context.Context, repoFullName string, limit int) ([]WorkflowRun, error)
}

type runsResponse struct {
	TotalCount int           `json:"total_count"`
	Runs       []WorkflowRun `json:"workflow_runs"`
}

// WorkflowRuns fetches the most recent Actions runs for one repo, newest
// first (the API's default order).
func (c *Client) WorkflowRuns(ctx context.Context, repoFullName string, limit int) ([]WorkflowRun, error) {
	var resp runsResponse
	path := fmt.Sprintf("/repos/%s/actions/runs?per_page=%d", repoFullName, limit)
	if err := c.doREST(ctx, path, &resp); err != nil {
		return nil, err
	}
	return resp.Runs, nil
}

// ActionsStats is the derived summary the pipeline metadata strip would
// show, if there's enough history to make it honest.
type ActionsStats struct {
	TotalRuns   int
	SuccessRuns int
	SuccessRate float64 // 0-100
}

// Summarize computes stats from completed runs only (in-progress/queued
// runs have no conclusion yet).
func Summarize(runs []WorkflowRun) ActionsStats {
	var s ActionsStats
	for _, r := range runs {
		if r.Status != "completed" {
			continue
		}
		s.TotalRuns++
		if r.Conclusion == "success" {
			s.SuccessRuns++
		}
	}
	if s.TotalRuns > 0 {
		s.SuccessRate = float64(s.SuccessRuns) / float64(s.TotalRuns) * 100
	}
	return s
}
