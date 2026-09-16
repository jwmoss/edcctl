package api

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var owedPattern = regexp.MustCompile(`(?i)owe[sd]?\s*\$?([\d,]+\.\d{2})`)

// Balance is the account balance summary from my_payments.php.
type Balance struct {
	Owed     string           `json:"owed"`
	Students []StudentBalance `json:"students"`
}

// StudentBalance is the per-student line on the payments page.
type StudentBalance struct {
	Name         string `json:"name"`
	Balance      string `json:"balance"`
	StudentID    string `json:"student_id,omitempty"`
	StatementURL string `json:"statement_url,omitempty"`
}

// Balance parses the online payment page (my_payments.php).
func (c *Client) Balance(ctx context.Context) (*Balance, error) {
	page, err := c.Get(ctx, "/my_payments.php", nil)
	if err != nil {
		return nil, fmt.Errorf("load payments page: %w", err)
	}
	doc, err := parseHTML(page)
	if err != nil {
		return nil, err
	}

	balance := &Balance{Students: []StudentBalance{}}
	if heading := doc.Find("h3").First(); heading.Length() > 0 {
		if match := owedPattern.FindStringSubmatch(heading.Text()); match != nil {
			balance.Owed = "$" + match[1]
		}
	}
	doc.Find(`table a[href^="app_statements.php"]`).Each(func(_ int, link *goquery.Selection) {
		href, _ := link.Attr("href")
		row := link.ParentsFiltered("tr").First()
		cells := row.Find("td")
		entry := StudentBalance{}
		if cells.Length() > 0 {
			entry.Name = strings.TrimSpace(cells.Eq(0).Text())
		}
		if cells.Length() > 1 {
			entry.Balance = strings.TrimSpace(cells.Eq(1).Text())
		}
		if match := regexp.MustCompile(`sid=(\d+)`).FindStringSubmatch(href); match != nil {
			entry.StudentID = match[1]
		}
		entry.StatementURL = href
		balance.Students = append(balance.Students, entry)
	})
	return balance, nil
}

// LedgerEntry is one payment or charge row in the account history.
type LedgerEntry struct {
	Description string `json:"description"`
	Student     string `json:"student,omitempty"`
	Date        string `json:"date,omitempty"`
	Payments    string `json:"payments,omitempty"`
	Charges     string `json:"charges,omitempty"`
	Balance     string `json:"balance,omitempty"`
}

// Ledger is the account history table from my_history.php.
type Ledger struct {
	Balance string        `json:"balance"`
	Entries []LedgerEntry `json:"entries"`
}

// History parses the payment history page (my_history.php). When from
// is non-zero the server form is submitted to start the listing at
// that date, matching the portal's search form.
func (c *Client) History(ctx context.Context, from time.Time) (*Ledger, error) {
	var page []byte
	var err error
	if !from.IsZero() {
		form := url.Values{
			"show_month": {fmt.Sprintf("%d", int(from.Month()))},
			"show_day":   {fmt.Sprintf("%02d", from.Day())},
			"show_year":  {fmt.Sprintf("%d", from.Year())},
			"btn_search": {"1"},
		}
		page, err = c.Post(ctx, "/my_history.php?"+c.PortalQuery().Encode(), form)
	} else {
		page, err = c.Get(ctx, "/my_history.php", nil)
	}
	if err != nil {
		return nil, fmt.Errorf("load history page: %w", err)
	}
	doc, err := parseHTML(page)
	if err != nil {
		return nil, err
	}

	ledger := &Ledger{Entries: []LedgerEntry{}}
	if text := doc.Find("h3").First().Parent(); text.Length() > 0 {
		if match := owedPattern.FindStringSubmatch(text.Text()); match != nil {
			ledger.Balance = "Owe $" + match[1]
		}
	}
	doc.Find("table.table-bordered tr").Each(func(_ int, row *goquery.Selection) {
		cells := row.Find("td")
		if cells.Length() != 4 {
			return
		}
		first := cells.Eq(0)
		if first.Find("th").Length() > 0 {
			return
		}
		entry := LedgerEntry{}
		if full := first.Find(`span[id^="sp_full_"]`).First(); full.Length() > 0 {
			entry.Description = strings.TrimSpace(full.Text())
		} else {
			entry.Description = strings.TrimSpace(first.Text())
		}
		if student := first.Find("p.text-info").First(); student.Length() > 0 {
			entry.Student = strings.TrimSpace(student.Text())
		}
		if date := first.Find("small").First(); date.Length() > 0 {
			entry.Date = strings.TrimSpace(date.Text())
		}
		entry.Payments = strings.TrimSpace(cells.Eq(1).Text())
		entry.Charges = strings.TrimSpace(cells.Eq(2).Text())
		entry.Balance = strings.TrimSpace(cells.Eq(3).Text())
		ledger.Entries = append(ledger.Entries, entry)
	})
	return ledger, nil
}
