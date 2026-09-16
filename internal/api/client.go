// Package api reads EDC data from Studio Pro and MobileInventor JSON endpoints.
// Only authentication consumes HTML, to establish the session and CSRF token.
package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

const DefaultUserAgent = "edcctl/dev"

type Client struct {
	baseURL    string
	accountID  string
	csrfToken  string
	httpClient *http.Client
	userAgent  string
	trace      func(method, path string, status int, duration time.Duration)
	dryRun     bool
}

type Option func(*Client)

func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		if client != nil {
			c.httpClient = client
		}
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		if timeout > 0 {
			c.httpClient.Timeout = timeout
		}
	}
}

func WithUserAgent(userAgent string) Option {
	return func(c *Client) {
		if strings.TrimSpace(userAgent) != "" {
			c.userAgent = userAgent
		}
	}
}

func WithTrace(trace func(method, path string, status int, duration time.Duration)) Option {
	return func(c *Client) { c.trace = trace }
}

func WithDryRun(dryRun bool) Option {
	return func(c *Client) { c.dryRun = dryRun }
}

func New(baseURL, accountID string, opts ...Option) *Client {
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		accountID:  strings.TrimSpace(accountID),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		userAgent:  DefaultUserAgent,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.httpClient.Jar == nil {
		c.httpClient.Jar, _ = cookiejar.New(nil)
	}
	c.httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return fmt.Errorf("portal redirect refused; check credentials or portal URL")
	}
	return c
}

type APIError struct {
	Status int
	Method string
	Path   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s %s: HTTP %d: request failed", e.Method, e.Path, e.Status)
}

// do is shared transport for login and the verified JSON data endpoints.
// It never follows redirects or logs response bodies.
func (c *Client) do(ctx context.Context, method, requestPath string, form url.Values, accept string) ([]byte, error) {
	if c.dryRun && method != http.MethodGet {
		return nil, fmt.Errorf("dry-run: refusing %s %s", method, requestPath)
	}
	var query url.Values
	if c.accountID != "" {
		query = url.Values{"account_id": {c.accountID}, "app": {"1"}, "app_mi": {"1"}}
	}
	endpoint, err := c.url(requestPath, query)
	if err != nil {
		return nil, err
	}
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", c.userAgent)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if c.csrfToken != "" {
			req.Header.Set("X-CSRF-Token", c.csrfToken)
		}
	}
	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if c.trace != nil {
		c.trace(method, req.URL.Path, resp.StatusCode, time.Since(start))
	}
	if err := checkStatus(resp.StatusCode, method, req.URL.Path); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return data, nil
}

func checkStatus(status int, method, path string) error {
	if status >= 200 && status < 300 {
		return nil
	}
	return &APIError{Status: status, Method: method, Path: path}
}

func (c *Client) url(requestPath string, query url.Values) (string, error) {
	parsed, err := url.Parse(requestPath)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || strings.Contains(parsed.Path, "..") || strings.Contains(parsed.Path, `\`) {
		return "", fmt.Errorf("use a relative portal path without traversal")
	}
	u, err := url.Parse(c.baseURL + "/" + strings.TrimLeft(requestPath, "/"))
	if err != nil {
		return "", fmt.Errorf("parse URL: %w", err)
	}
	values := u.Query()
	for key, items := range query {
		values[key] = items
	}
	u.RawQuery = values.Encode()
	return u.String(), nil
}
