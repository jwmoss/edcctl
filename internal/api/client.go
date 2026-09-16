// Package api implements a client for the Dance Studio Pro parent
// portal that powers the Evolution Dance Complex (EDC) mobile app.
//
// The portal is a classic PHP web app. Requests carry a CSRF token and
// a PHP session cookie. Pages return HTML that the client parses, and
// a few AJAX endpoints return JSON.
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

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
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
	return func(c *Client) {
		c.trace = trace
	}
}

func WithDryRun(dryRun bool) Option {
	return func(c *Client) {
		c.dryRun = dryRun
	}
}

func New(baseURL, accountID string, opts ...Option) *Client {
	jar, _ := cookiejar.New(nil)
	c := &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		accountID: strings.TrimSpace(accountID),
		httpClient: &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
		},
		userAgent: DefaultUserAgent,
	}
	for _, opt := range opts {
		opt(c)
	}
	c.httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return fmt.Errorf("portal redirect refused; check credentials or portal URL")
	}
	return c
}

type APIError struct {
	Status  int
	Method  string
	Path    string
	Body    []byte
	Message string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s %s: HTTP %d: %s", e.Method, e.Path, e.Status, e.Message)
	}
	return fmt.Sprintf("%s %s: HTTP %d", e.Method, e.Path, e.Status)
}

// PortalQuery returns the query parameters that identify the portal
// account on every page request.
func (c *Client) PortalQuery() url.Values {
	return url.Values{
		"account_id": {c.accountID},
		"app":        {"1"},
		"app_mi":     {"1"},
	}
}

// Get fetches a portal page and returns the raw HTML.
func (c *Client) Get(ctx context.Context, requestPath string, query url.Values) ([]byte, error) {
	if len(query) == 0 {
		query = c.PortalQuery()
	}
	body, status, err := c.do(ctx, http.MethodGet, requestPath, query, nil)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(status, http.MethodGet, requestPath, body); err != nil {
		return nil, err
	}
	if requestPath != "/index.php" && (loginFormPresent(body) || strings.Contains(string(body), `url=/online/index.php`)) {
		return nil, fmt.Errorf("portal session expired; run the command again")
	}
	return body, nil
}

// Post submits a form-encoded POST request and returns the raw body.
func (c *Client) Post(ctx context.Context, requestPath string, form url.Values) ([]byte, error) {
	if c.dryRun {
		return nil, fmt.Errorf("dry-run: refusing POST %s", requestPath)
	}
	if len(form) == 0 {
		form = url.Values{}
	}
	body, status, err := c.do(ctx, http.MethodPost, requestPath, nil, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	if err := checkStatus(status, http.MethodPost, requestPath, body); err != nil {
		return nil, err
	}
	return body, nil
}

func (c *Client) do(
	ctx context.Context,
	method string,
	requestPath string,
	query url.Values,
	body io.Reader,
) ([]byte, int, error) {
	endpoint, err := c.url(requestPath, query)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if c.csrfToken != "" {
			req.Header.Set("X-CSRF-Token", c.csrfToken)
		}
	}
	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	if c.trace != nil {
		c.trace(method, req.URL.Path, resp.StatusCode, time.Since(start))
	}
	return data, resp.StatusCode, nil
}

func checkStatus(status int, method, requestPath string, body []byte) error {
	if status >= 200 && status < 300 {
		return nil
	}
	message := "portal request failed"
	return &APIError{
		Status:  status,
		Method:  method,
		Path:    requestPath,
		Body:    body,
		Message: message,
	}
}

func (c *Client) url(requestPath string, query url.Values) (string, error) {
	parsed, err := url.Parse(requestPath)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || strings.Contains(parsed.Path, "..") || strings.Contains(parsed.Path, `\`) {
		return "", fmt.Errorf("use a relative portal path without traversal")
	}
	if !strings.HasPrefix(requestPath, "/") &&
		!strings.HasPrefix(requestPath, "http://") &&
		!strings.HasPrefix(requestPath, "https://") {
		requestPath = "/" + requestPath
	}
	if strings.HasPrefix(requestPath, "http://") || strings.HasPrefix(requestPath, "https://") {
		u, err := url.Parse(requestPath)
		if err != nil {
			return "", fmt.Errorf("parse URL: %w", err)
		}
		if len(query) > 0 {
			u.RawQuery = mergeQuery(u.Query(), query).Encode()
		}
		return u.String(), nil
	}
	joined := c.baseURL + requestPath
	u, err := url.Parse(joined)
	if err != nil {
		return "", fmt.Errorf("parse URL: %w", err)
	}
	if len(query) > 0 {
		u.RawQuery = mergeQuery(u.Query(), query).Encode()
	}
	return u.String(), nil
}

func mergeQuery(dst, src url.Values) url.Values {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
	return dst
}
