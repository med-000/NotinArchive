package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type Client struct {
	apiKey      string
	httpClient  *http.Client
	minInterval time.Duration
	lastRequest time.Time
	mu          sync.Mutex
}

type APIError struct {
	Status     int
	Code       string
	Message    string
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s (status=%d code=%s)", e.Message, e.Status, e.Code)
}

func NewClient() *Client {
	return &Client{
		apiKey:      os.Getenv("NOTION_API_KEY"),
		httpClient:  http.DefaultClient,
		minInterval: 350 * time.Millisecond,
	}
}

func (c *Client) waitTurn() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.lastRequest.IsZero() {
		c.lastRequest = time.Now()
		return
	}

	next := c.lastRequest.Add(c.minInterval)
	if sleep := time.Until(next); sleep > 0 {
		time.Sleep(sleep)
	}

	c.lastRequest = time.Now()
}

func (c *Client) request(method, endpoint string, body []byte) ([]byte, error) {
	const maxAttempts = 5

	for attempt := 0; attempt < maxAttempts; attempt++ {
		c.waitTurn()

		var reader io.Reader
		if body != nil {
			reader = bytes.NewReader(body)
		}

		req, _ := http.NewRequest(method, endpoint, reader)
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Notion-Version", "2022-06-28")
		req.Header.Set("Content-Type", "application/json")

		res, err := c.httpClient.Do(req)
		if err != nil {
			if attempt == maxAttempts-1 {
				return nil, err
			}
			time.Sleep(backoff(attempt))
			continue
		}

		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			if attempt == maxAttempts-1 {
				return nil, readErr
			}
			time.Sleep(backoff(attempt))
			continue
		}

		if res.StatusCode >= 200 && res.StatusCode < 300 {
			return raw, nil
		}

		reqErr := parseAPIError(res, raw)
		if !shouldRetry(reqErr) || attempt == maxAttempts-1 {
			return nil, reqErr
		}

		wait := time.Duration(0)
		var apiErr *APIError
		if errors.As(reqErr, &apiErr) {
			wait = apiErr.RetryAfter
		}
		if wait <= 0 {
			wait = backoff(attempt)
		}
		time.Sleep(wait)
	}

	return nil, errors.New("request failed after retries")
}

// -------- search --------
func (c *Client) Search() ([]map[string]interface{}, error) {
	url := "https://api.notion.com/v1/search"

	raw, err := c.request("POST", url, []byte(`{}`))
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []map[string]interface{} `json:"results"`
	}

	json.Unmarshal(raw, &result)

	return result.Results, nil
}

// -------- blocks --------
func (c *Client) GetBlocks(pageID string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("https://api.notion.com/v1/blocks/%s/children", pageID)

	raw, err := c.request("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []map[string]interface{} `json:"results"`
	}

	json.Unmarshal(raw, &result)

	return result.Results, nil
}

// -------- database --------
func (c *Client) QueryDatabase(dbID string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("https://api.notion.com/v1/databases/%s/query", dbID)

	raw, err := c.request("POST", url, []byte(`{}`))
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []map[string]interface{} `json:"results"`
	}

	json.Unmarshal(raw, &result)

	return result.Results, nil
}

func (c *Client) GetAllBlocks(pageID string) ([]map[string]interface{}, error) {
	var all []map[string]interface{}
	cursor := ""

	for {
		endpoint := fmt.Sprintf("https://api.notion.com/v1/blocks/%s/children", pageID)
		if cursor != "" {
			params := url.Values{}
			params.Set("start_cursor", cursor)
			endpoint += "?" + params.Encode()
		}

		raw, err := c.request("GET", endpoint, nil)
		if err != nil {
			return nil, err
		}

		var result struct {
			Results    []map[string]interface{} `json:"results"`
			HasMore    bool                     `json:"has_more"`
			NextCursor string                   `json:"next_cursor"`
		}

		json.Unmarshal(raw, &result)

		all = append(all, result.Results...)

		if !result.HasMore {
			break
		}

		cursor = result.NextCursor
	}

	return all, nil
}

func (c *Client) QueryAllDatabase(dbID string) ([]map[string]interface{}, error) {
	var all []map[string]interface{}
	cursor := ""

	for {
		url := fmt.Sprintf("https://api.notion.com/v1/databases/%s/query", dbID)

		body := []byte(`{}`)
		if cursor != "" {
			body = []byte(fmt.Sprintf(`{"start_cursor":"%s"}`, cursor))
		}

		raw, err := c.request("POST", url, body)
		if err != nil {
			return nil, err
		}

		var result struct {
			Results    []map[string]interface{} `json:"results"`
			HasMore    bool                     `json:"has_more"`
			NextCursor string                   `json:"next_cursor"`
		}

		json.Unmarshal(raw, &result)

		all = append(all, result.Results...)

		if !result.HasMore {
			break
		}

		cursor = result.NextCursor
	}

	return all, nil
}

func parseAPIError(res *http.Response, raw []byte) error {
	apiErr := &APIError{
		Status:  res.StatusCode,
		Message: string(raw),
	}

	var payload struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &payload); err == nil {
		if payload.Code != "" {
			apiErr.Code = payload.Code
		}
		if payload.Message != "" {
			apiErr.Message = payload.Message
		}
	}

	if retryAfter := res.Header.Get("Retry-After"); retryAfter != "" {
		if seconds, err := time.ParseDuration(retryAfter + "s"); err == nil {
			apiErr.RetryAfter = seconds
		}
	}

	return apiErr
}

func shouldRetry(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return true
	}

	if apiErr.Status == http.StatusTooManyRequests {
		return true
	}

	if apiErr.Status >= 500 {
		return true
	}

	return false
}

func backoff(attempt int) time.Duration {
	base := 400 * time.Millisecond
	wait := base * time.Duration(1<<attempt)
	maxWait := 5 * time.Second
	if wait > maxWait {
		return maxWait
	}
	return wait
}

func IsInaccessibleDatabaseError(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}

	return apiErr.Status == http.StatusBadRequest &&
		apiErr.Code == "validation_error" &&
		strings.Contains(apiErr.Message, "does not contain any data sources accessible by this API bot")
}
