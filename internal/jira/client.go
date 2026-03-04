package jira

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client is a Jira REST API client using HTTP Basic auth
type Client struct {
	baseURL  string
	email    string
	apiToken string
	http     *http.Client
}

// NewClient creates a new Jira API client
func NewClient(baseURL, email, apiToken string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		email:    email,
		apiToken: apiToken,
		http:     &http.Client{},
	}
}

func (c *Client) authHeader() string {
	creds := c.email + ":" + c.apiToken
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
}

func (c *Client) get(path string) ([]byte, error) {
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jira API error %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// GetBoards returns all boards visible to the authenticated user
func (c *Client) GetBoards() ([]Board, error) {
	data, err := c.get("/rest/agile/1.0/board?maxResults=50")
	if err != nil {
		return nil, err
	}

	var result struct {
		Values []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"values"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse boards: %w", err)
	}

	boards := make([]Board, len(result.Values))
	for i, v := range result.Values {
		boards[i] = Board{ID: v.ID, Name: v.Name}
	}
	return boards, nil
}

// GetBoardIssues returns issues for the given board (key, summary, status)
func (c *Client) GetBoardIssues(boardID int) ([]Issue, error) {
	path := fmt.Sprintf("/rest/agile/1.0/board/%d/issue?fields=summary,status&maxResults=100", boardID)
	data, err := c.get(path)
	if err != nil {
		return nil, err
	}

	var result struct {
		Issues []struct {
			Key    string `json:"key"`
			Fields struct {
				Summary string `json:"summary"`
				Status  struct {
					Name string `json:"name"`
				} `json:"status"`
			} `json:"fields"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse issues: %w", err)
	}

	issues := make([]Issue, len(result.Issues))
	for i, v := range result.Issues {
		issues[i] = Issue{
			Key:     v.Key,
			Summary: v.Fields.Summary,
			Status:  v.Fields.Status.Name,
		}
	}
	return issues, nil
}

// GetIssue fetches a single issue by key, returning its summary and status
func (c *Client) GetIssue(key string) (*Issue, error) {
	path := fmt.Sprintf("/rest/api/3/issue/%s?fields=summary,status", key)
	data, err := c.get(path)
	if err != nil {
		return nil, err
	}

	var result struct {
		Key    string `json:"key"`
		Fields struct {
			Summary string `json:"summary"`
			Status  struct {
				Name string `json:"name"`
			} `json:"status"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse issue: %w", err)
	}

	return &Issue{
		Key:     key,
		Summary: result.Fields.Summary,
		Status:  result.Fields.Status.Name,
	}, nil
}

// GetIssueStatuses returns a map of issue key -> status for the given keys
func (c *Client) GetIssueStatuses(keys []string) (map[string]string, error) {
	statuses := make(map[string]string, len(keys))
	for _, key := range keys {
		path := fmt.Sprintf("/rest/api/3/issue/%s?fields=status", key)
		data, err := c.get(path)
		if err != nil {
			// Skip issues that fail (e.g. deleted/inaccessible)
			continue
		}

		var result struct {
			Fields struct {
				Status struct {
					Name string `json:"name"`
				} `json:"status"`
			} `json:"fields"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			continue
		}
		statuses[key] = result.Fields.Status.Name
	}
	return statuses, nil
}
