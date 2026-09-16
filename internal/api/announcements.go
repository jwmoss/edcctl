package api

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var messageIDPattern = regexp.MustCompile(`show_com_log\('(\d+)'\)`)

// Announcement is one studio communication on the bulletin board.
type Announcement struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Subject  string `json:"subject"`
	Sent     string `json:"sent,omitempty"`
	Body     string `json:"body,omitempty"`
}

// Announcements parses the News & Notes page (bulletin_board.php).
// The portal groups items into a Messages tab and an Email History
// tab; both are returned in one list.
func (c *Client) Announcements(ctx context.Context) ([]Announcement, error) {
	page, err := c.Get(ctx, "/bulletin_board.php", nil)
	if err != nil {
		return nil, fmt.Errorf("load bulletin board: %w", err)
	}
	doc, err := parseHTML(page)
	if err != nil {
		return nil, err
	}

	announcements := []Announcement{}
	doc.Find("div.tab-pane").Each(func(_ int, pane *goquery.Selection) {
		category := "messages"
		if id, ok := pane.Attr("id"); ok && id == "tab2" {
			category = "email-history"
		}
		pane.Find("a[onclick*=\"show_com_log(\"]").Each(func(_ int, link *goquery.Selection) {
			onClick, _ := link.Attr("onclick")
			match := messageIDPattern.FindStringSubmatch(onClick)
			if match == nil {
				return
			}
			item := Announcement{
				ID:       match[1],
				Category: category,
				Subject:  strings.TrimSpace(link.Text()),
			}
			if sent := link.ParentsFiltered("td").First().Find("span.muted").First(); sent.Length() > 0 {
				item.Sent = strings.TrimSpace(strings.TrimPrefix(sent.Text(), "Sent on "))
			}
			announcements = append(announcements, item)
		})
	})
	return announcements, nil
}

// Announcement fetches the full body of one message by ID.
func (c *Client) Announcement(ctx context.Context, id string) (*Announcement, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("message ID is required")
	}
	form := url.Values{
		"action": {"show_com_log"},
		"mid":    {id},
	}
	body, err := c.Post(ctx, "/bulletin_board-ajax.php?"+c.PortalQuery().Encode(), form)
	if err != nil {
		return nil, fmt.Errorf("load message %s: %w", id, err)
	}
	doc, err := parseHTML(body)
	if err != nil {
		return nil, err
	}
	return &Announcement{
		ID:   id,
		Body: strings.TrimSpace(htmlToText(doc)),
	}, nil
}

// htmlToText renders a document's body as plain text, preserving
// paragraph breaks.
func htmlToText(doc *goquery.Document) string {
	doc.Find("script, style, head").Remove()
	selection := doc.Find("body")
	if selection.Length() == 0 {
		selection = doc.Selection
	}
	var lines []string
	selection.Find("p").Each(func(_ int, block *goquery.Selection) {
		text := strings.Join(strings.Fields(block.Text()), " ")
		if text != "" {
			lines = append(lines, text)
		}
	})
	if len(lines) == 0 {
		return strings.Join(strings.Fields(selection.Text()), " ")
	}
	return strings.Join(lines, "\n\n")
}
