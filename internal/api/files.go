package api

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// PortalFile is one downloadable or linked item on the shared files
// page (files.php).
type PortalFile struct {
	Category string `json:"category"` // shared, class-files, or class-music
	Owner    string `json:"owner,omitempty"`
	Folder   string `json:"folder,omitempty"`
	Name     string `json:"name"`
	URL      string `json:"url"` // direct target of the portal display link
}

// Files parses the Shared Files page (files.php).
func (c *Client) Files(ctx context.Context) ([]PortalFile, error) {
	page, err := c.Get(ctx, "/files.php", nil)
	if err != nil {
		return nil, fmt.Errorf("load files page: %w", err)
	}
	doc, err := parseHTML(page)
	if err != nil {
		return nil, err
	}

	categories := map[string]string{"tab1": "shared", "tab2": "class-files", "tab3": "class-music"}
	files := []PortalFile{}
	doc.Find("div.tab-pane").Each(func(_ int, pane *goquery.Selection) {
		id, _ := pane.Attr("id")
		category := categories[id]
		if category == "" {
			return
		}
		pane.Find("a[href^=\"app_display_file.php\"]").Each(func(_ int, link *goquery.Selection) {
			href, _ := link.Attr("href")
			file := PortalFile{
				Category: category,
				Name:     strings.TrimSpace(link.Text()),
				URL:      displayTarget(href),
			}
			if owner := link.ParentsFiltered("li").First().Find("em").First(); owner.Length() > 0 {
				file.Owner = strings.TrimSpace(owner.Text())
			}
			if folder := link.ParentsFiltered("li").First().Parent().ParentsFiltered("li").First().Find("a[onclick]").First(); folder.Length() > 0 {
				file.Folder = strings.TrimSpace(folder.Text())
			}
			files = append(files, file)
		})
	})
	return files, nil
}

// displayTarget extracts and decodes the real URL from an
// app_display_file.php?url=<encoded> link.
func displayTarget(href string) string {
	parsed, err := url.Parse(href)
	if err != nil {
		return href
	}
	if target := parsed.Query().Get("url"); target != "" {
		return target
	}
	return href
}
