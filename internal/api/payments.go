package api

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// No JSON endpoint serves balances or payment history; the EDC app shows
// these portal pages in a web view. These parsers accept only the verified
// page structure and fail closed on any other markup.

var (
	moneyPattern     = regexp.MustCompile(`^(?:(Owes?) )?(\()?\$?(\d{1,3}(?:,\d{3})*|\d+)\.(\d{2})(\))?$`)
	statementPattern = regexp.MustCompile(`^app_statements\.php\?sid=(\d+)$`)
)

// AccountBalance is the balance summary from my_payments.php. Positive
// cents are owed; negative cents are credit.
type AccountBalance struct {
	BalanceCents int64            `json:"balance_cents"`
	Students     []StudentBalance `json:"students"`
}

type StudentBalance struct {
	StudentID    string `json:"student_id"`
	Name         string `json:"name"`
	BalanceCents int64  `json:"balance_cents"`
}

// PaymentHistory is the account ledger from my_history.php. Entries are
// newest first, as the portal lists them.
type PaymentHistory struct {
	From                string        `json:"from"`
	BalanceCents        int64         `json:"balance_cents"`
	OpeningBalanceCents int64         `json:"opening_balance_cents"`
	Entries             []LedgerEntry `json:"entries"`
}

type LedgerEntry struct {
	Date         string `json:"date"`
	Student      string `json:"student"`
	Description  string `json:"description"`
	PaymentCents int64  `json:"payment_cents"`
	ChargeCents  int64  `json:"charge_cents"`
	BalanceCents int64  `json:"balance_cents"`
}

// Balance reads per-student balances with a GET request.
func (c *Client) Balance(ctx context.Context) (*AccountBalance, error) {
	doc, err := c.page(ctx, http.MethodGet, "/my_payments.php", nil)
	if err != nil {
		return nil, fmt.Errorf("load balance: %w", err)
	}
	balance, err := parseBalance(doc)
	if err != nil {
		return nil, fmt.Errorf("balance page has an unexpected structure: %w", err)
	}
	return balance, nil
}

// History reads the payment ledger. A zero from uses the portal's default
// start date with a GET request. Otherwise it submits the portal's
// read-only date search, which --dry-run refuses because it is a POST.
func (c *Client) History(ctx context.Context, from time.Time) (*PaymentHistory, error) {
	method, form := http.MethodGet, url.Values(nil)
	if !from.IsZero() {
		method = http.MethodPost
		form = url.Values{
			"csrf_token": {c.csrfToken},
			"show_month": {from.Format("01")},
			"show_day":   {from.Format("02")},
			"show_year":  {from.Format("2006")},
			"btn_search": {"Search"},
		}
	}
	doc, err := c.page(ctx, method, "/my_history.php", form)
	if err != nil {
		return nil, fmt.Errorf("load payment history: %w", err)
	}
	history, err := parseHistory(doc)
	if err != nil {
		return nil, fmt.Errorf("payment history page has an unexpected structure: %w", err)
	}
	if !from.IsZero() && history.From != from.Format("2006-01-02") {
		return nil, fmt.Errorf("payment history starts on %s, not the requested %s", history.From, from.Format("2006-01-02"))
	}
	return history, nil
}

func (c *Client) page(ctx context.Context, method, path string, form url.Values) (*html.Node, error) {
	data, err := c.do(ctx, method, path, form, "text/html")
	if err != nil {
		return nil, err
	}
	if loginFormPresent(data) {
		return nil, fmt.Errorf("portal returned the login form; the session may have expired")
	}
	return html.Parse(bytes.NewReader(data))
}

func parseBalance(doc *html.Node) (*AccountBalance, error) {
	table, err := findTable(doc, "Name", "Balance", "Statement")
	if err != nil {
		return nil, err
	}
	result := &AccountBalance{Students: []StudentBalance{}}
	for _, row := range dataRows(table) {
		cells := children(row, atom.Td)
		if len(cells) != 3 {
			return nil, fmt.Errorf("balance row has %d cells, not 3", len(cells))
		}
		links := findAll(cells[2], func(n *html.Node) bool { return n.DataAtom == atom.A })
		if len(links) != 1 {
			return nil, fmt.Errorf("balance row has no single statement link")
		}
		match := statementPattern.FindStringSubmatch(attr(links[0], "href"))
		if match == nil {
			return nil, fmt.Errorf("statement link has no student ID")
		}
		name := text(cells[0])
		amount, err := parseMoney(text(cells[1]))
		if err != nil || name == "" {
			return nil, fmt.Errorf("balance row has an invalid name or amount")
		}
		result.BalanceCents += amount
		result.Students = append(result.Students, StudentBalance{StudentID: match[1], Name: name, BalanceCents: amount})
	}
	// The heading repeats the total only when money is owed.
	for _, heading := range findAll(doc, func(n *html.Node) bool { return n.DataAtom == atom.H3 }) {
		if owed, ok := strings.CutPrefix(text(heading), "You owe "); ok {
			amount, err := parseMoney(owed)
			if err != nil || amount != result.BalanceCents {
				return nil, fmt.Errorf("balance heading does not match the student balances")
			}
		}
	}
	return result, nil
}

func parseHistory(doc *html.Node) (*PaymentHistory, error) {
	from, err := historyStart(doc)
	if err != nil {
		return nil, err
	}
	balance, err := historyBalance(doc)
	if err != nil {
		return nil, err
	}
	table, err := findTable(doc, "--", "Pmts", "Chrgs", "Bal")
	if err != nil {
		return nil, err
	}
	rows := dataRows(table)
	if len(rows) == 0 {
		return nil, fmt.Errorf("ledger has no opening balance row")
	}
	result := &PaymentHistory{From: from, BalanceCents: balance, Entries: []LedgerEntry{}}
	for i, row := range rows {
		cells := children(row, atom.Td)
		if len(cells) != 4 {
			return nil, fmt.Errorf("ledger row has %d cells, not 4", len(cells))
		}
		running, err := parseMoney(text(cells[3]))
		if err != nil {
			return nil, fmt.Errorf("ledger row has an invalid balance")
		}
		if i == len(rows)-1 {
			if text(cells[0]) != "" || text(cells[1]) != "" || text(cells[2]) != "" {
				return nil, fmt.Errorf("ledger does not end with an opening balance row")
			}
			result.OpeningBalanceCents = running
			break
		}
		entry, err := ledgerEntry(cells)
		if err != nil {
			return nil, err
		}
		entry.BalanceCents = running
		result.Entries = append(result.Entries, entry)
	}

	// Rows are newest first: each running balance is the next older balance
	// plus this row's charge minus its payment.
	previous := result.OpeningBalanceCents
	for i := len(result.Entries) - 1; i >= 0; i-- {
		entry := result.Entries[i]
		if entry.BalanceCents != previous+entry.ChargeCents-entry.PaymentCents {
			return nil, fmt.Errorf("ledger running balances do not reconcile")
		}
		previous = entry.BalanceCents
	}
	if previous != result.BalanceCents {
		return nil, fmt.Errorf("ledger balance does not match the account balance")
	}
	return result, nil
}

func ledgerEntry(cells []*html.Node) (LedgerEntry, error) {
	var entry LedgerEntry
	var err error
	if cell := text(cells[1]); cell != "" {
		if entry.PaymentCents, err = parseMoney(cell); err != nil {
			return entry, fmt.Errorf("ledger row has an invalid payment")
		}
	}
	if cell := text(cells[2]); cell != "" {
		if entry.ChargeCents, err = parseMoney(cell); err != nil {
			return entry, fmt.Errorf("ledger row has an invalid charge")
		}
	}

	detail := cells[0]
	dates := findAll(detail, func(n *html.Node) bool { return n.DataAtom == atom.Small })
	students := findAll(detail, func(n *html.Node) bool { return n.DataAtom == atom.P && hasClass(n, "text-info") })
	if len(dates) != 1 || len(students) != 1 {
		return entry, fmt.Errorf("ledger row has no single date and student")
	}
	date, err := time.Parse("Jan 02,2006", text(dates[0]))
	if err != nil {
		return entry, fmt.Errorf("ledger row has an invalid date")
	}
	entry.Date = date.Format("2006-01-02")
	entry.Student = text(students[0])

	// Long memos have a truncated span and a hidden full span.
	full := findAll(detail, func(n *html.Node) bool {
		return n.DataAtom == atom.Span && strings.HasPrefix(attr(n, "id"), "sp_full_")
	})
	switch len(full) {
	case 0:
		var parts []string
		for n := detail.FirstChild; n != nil && n.DataAtom != atom.Br; n = n.NextSibling {
			parts = append(parts, text(n))
		}
		entry.Description = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.Join(parts, " ")), "--"))
	case 1:
		entry.Description = text(full[0])
	default:
		return entry, fmt.Errorf("ledger row has several descriptions")
	}
	if entry.Description == "" || entry.Student == "" {
		return entry, fmt.Errorf("ledger row has no description or student")
	}
	return entry, nil
}

func historyStart(doc *html.Node) (string, error) {
	var parts []string
	for _, name := range []string{"show_year", "show_month", "show_day"} {
		selects := findAll(doc, func(n *html.Node) bool { return n.DataAtom == atom.Select && attr(n, "name") == name })
		if len(selects) != 1 {
			return "", fmt.Errorf("history form has no single %s field", name)
		}
		selected := findAll(selects[0], func(n *html.Node) bool { return n.DataAtom == atom.Option && hasAttr(n, "selected") })
		if len(selected) != 1 {
			return "", fmt.Errorf("history form has no single selected %s", name)
		}
		parts = append(parts, attr(selected[0], "value"))
	}
	start, err := time.Parse("2006-01-02", strings.Join(parts, "-"))
	if err != nil {
		return "", fmt.Errorf("history form has an invalid start date")
	}
	return start.Format("2006-01-02"), nil
}

func historyBalance(doc *html.Node) (int64, error) {
	headings := findAll(doc, func(n *html.Node) bool { return n.DataAtom == atom.H3 && text(n) == "Payment History" })
	if len(headings) != 1 {
		return 0, fmt.Errorf("history page has no single Payment History heading")
	}
	for n := headings[0].NextSibling; n != nil && n.Type == html.TextNode; n = n.NextSibling {
		if value, ok := strings.CutPrefix(text(n), "Balance: "); ok {
			amount, err := parseMoney(value)
			if err != nil {
				return 0, fmt.Errorf("history page has an invalid account balance")
			}
			return amount, nil
		}
	}
	return 0, fmt.Errorf("history page has no account balance")
}

// parseMoney accepts the verified portal formats: "12.34", "$1,234.56",
// "Owe $1.00", "Owes $1.00", and "($1.00)" for credit.
func parseMoney(value string) (int64, error) {
	match := moneyPattern.FindStringSubmatch(value)
	if match == nil || (match[2] == "") != (match[5] == "") || (match[1] != "" && match[2] != "") {
		return 0, fmt.Errorf("invalid amount %q", value)
	}
	dollars, err := strconv.ParseInt(strings.ReplaceAll(match[3], ",", ""), 10, 64)
	if err != nil || dollars > 1_000_000_000 {
		return 0, fmt.Errorf("invalid amount %q", value)
	}
	cents, _ := strconv.ParseInt(match[4], 10, 64)
	amount := dollars*100 + cents
	if match[2] != "" {
		amount = -amount
	}
	return amount, nil
}

// findTable returns the only table whose first row has exactly these headers.
func findTable(doc *html.Node, headers ...string) (*html.Node, error) {
	tables := findAll(doc, func(n *html.Node) bool {
		if n.DataAtom != atom.Table {
			return false
		}
		rows := tableRows(n)
		if len(rows) == 0 {
			return false
		}
		cells := children(rows[0], atom.Th)
		if len(cells) != len(headers) || len(children(rows[0], atom.Td)) != 0 {
			return false
		}
		for i, cell := range cells {
			if text(cell) != headers[i] {
				return false
			}
		}
		return true
	})
	if len(tables) != 1 {
		return nil, fmt.Errorf("found %d tables with headers %s, not 1", len(tables), strings.Join(headers, ", "))
	}
	return tables[0], nil
}

func dataRows(table *html.Node) []*html.Node {
	return tableRows(table)[1:]
}

// tableRows returns rows of this table, including the implicit tbody, but
// not rows of nested tables.
func tableRows(table *html.Node) []*html.Node {
	var rows []*html.Node
	for n := table.FirstChild; n != nil; n = n.NextSibling {
		switch n.DataAtom {
		case atom.Tr:
			rows = append(rows, n)
		case atom.Thead, atom.Tbody, atom.Tfoot:
			rows = append(rows, children(n, atom.Tr)...)
		}
	}
	return rows
}

func children(n *html.Node, a atom.Atom) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.DataAtom == a {
			out = append(out, c)
		}
	}
	return out
}

func findAll(n *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && match(c) {
			out = append(out, c)
		}
		out = append(out, findAll(c, match)...)
	}
	return out
}

// text returns the node's visible text with whitespace collapsed. Comments
// are separate nodes, so commented-out markup never contributes.
func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}

func hasClass(n *html.Node, class string) bool {
	return strings.Contains(" "+attr(n, "class")+" ", " "+class+" ")
}
