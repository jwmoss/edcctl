package api

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
)

var (
	inputPattern     = regexp.MustCompile(`(?i)<input\b[^>]*>`)
	attributePattern = regexp.MustCompile(`(?i)([a-z][a-z0-9_-]*)\s*=\s*["']([^"']*)["']`)
	csrfMetaPattern  = regexp.MustCompile(`(?i)<meta\s+name="csrf-token"\s+content="([^"]+)"`)
)

// Login authenticates against the portal login form. The flow mirrors
// the EDC mobile app: load index.php for a CSRF token and session
// cookie, then POST the credentials.
func (c *Client) Login(ctx context.Context, email, password string) error {
	if strings.TrimSpace(email) == "" {
		return fmt.Errorf("email is required")
	}
	if password == "" {
		return fmt.Errorf("password is required")
	}

	page, err := c.Get(ctx, "/index.php", c.PortalQuery())
	if err != nil {
		return fmt.Errorf("load login page: %w", err)
	}
	if !loginFormPresent(page) {
		return fmt.Errorf("portal did not return the expected login form")
	}
	token, err := csrfToken(page)
	if err != nil {
		return err
	}

	c.csrfToken = token
	form := url.Values{
		"csrf_token": {token},
		"email":      {email},
		"password":   {password},
		"btn_login":  {"1"},
	}
	response, err := c.Post(ctx, "/index.php?"+c.PortalQuery().Encode(), form)
	if err != nil {
		return fmt.Errorf("submit login: %w", err)
	}
	if loginFormPresent(response) || !strings.Contains(string(response), `messageType:"logininfo"`) || !strings.Contains(string(response), "app_my_class_schedule.php") {
		return fmt.Errorf("login failed: check the configured email and password")
	}
	return nil
}

// csrfToken extracts the CSRF token from the login page. The token is
// present both in a meta tag and a hidden form input.
func csrfToken(data []byte) (string, error) {
	if match := csrfMetaPattern.FindSubmatch(data); match != nil {
		return string(match[1]), nil
	}
	for _, input := range inputPattern.FindAll(data, -1) {
		attributes := map[string]string{}
		for _, match := range attributePattern.FindAllSubmatch(input, -1) {
			attributes[strings.ToLower(string(match[1]))] = html.UnescapeString(string(match[2]))
		}
		if attributes["name"] == "csrf_token" && attributes["value"] != "" {
			return attributes["value"], nil
		}
	}
	return "", fmt.Errorf("login page did not contain a CSRF token")
}

func loginFormPresent(data []byte) bool {
	inputNames := map[string]bool{}
	for _, input := range inputPattern.FindAll(data, -1) {
		for _, match := range attributePattern.FindAllSubmatch(input, -1) {
			if strings.EqualFold(string(match[1]), "name") {
				inputNames[strings.ToLower(string(match[2]))] = true
			}
		}
	}
	return inputNames["email"] && inputNames["password"]
}
