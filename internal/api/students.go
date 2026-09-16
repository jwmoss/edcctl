package api

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var (
	studentIDPattern = regexp.MustCompile(`myModalEdit_(\d+)`)
	classIDPattern   = regexp.MustCompile(`#collapse_\d+_(\d+)`)
	agePattern       = regexp.MustCompile(`(\d+)\s+years? old`)
)

// Student is one dancer on the account.
type Student struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Age     string        `json:"age,omitempty"`
	Classes []ClassDetail `json:"classes"`
}

// ClassDetail is one class the student is enrolled in.
type ClassDetail struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name"`
	Day        string `json:"day,omitempty"`
	Time       string `json:"time,omitempty"`
	Dates      string `json:"dates,omitempty"`
	Instructor string `json:"instructor,omitempty"`
	Room       string `json:"room,omitempty"`
}

// Students parses the Mobile Student Manager page (my_students.php).
func (c *Client) Students(ctx context.Context) ([]Student, error) {
	page, err := c.Get(ctx, "/my_students.php", nil)
	if err != nil {
		return nil, fmt.Errorf("load students page: %w", err)
	}
	doc, err := parseHTML(page)
	if err != nil {
		return nil, err
	}

	students := []Student{}
	links := doc.Find(`#div_student_table a[onclick*="myModalEdit_"]`)
	links.Each(func(_ int, link *goquery.Selection) {
		onClick, _ := link.Attr("onclick")
		match := studentIDPattern.FindStringSubmatch(onClick)
		if match == nil {
			return
		}
		student := Student{ID: match[1], Name: strings.TrimSpace(link.Text())}
		if holder := link.Parent(); holder.Length() > 0 {
			if age := agePattern.FindStringSubmatch(holder.Text()); age != nil {
				student.Age = age[1]
			}
		}
		cell := link.ParentsFiltered("td").First()
		cell.Find(".accordion").First().Find(".accordion-item").Each(func(_ int, item *goquery.Selection) {
			student.Classes = append(student.Classes, parseClassItem(item))
		})
		students = append(students, student)
	})
	return students, nil
}

func parseClassItem(item *goquery.Selection) ClassDetail {
	class := ClassDetail{}
	button := item.Find(".accordion-button").First()
	class.Name = strings.TrimSpace(button.Text())
	if target, ok := button.Attr("data-bs-target"); ok {
		if match := classIDPattern.FindStringSubmatch(target); match != nil {
			class.ID = match[1]
		}
	}
	lines := splitLines(blockText(item.Find(".accordion-body")))
	if len(lines) > 0 {
		class.Day = lines[0]
	}
	if len(lines) > 1 {
		class.Time = lines[1]
	}
	if len(lines) > 2 {
		class.Dates = lines[2]
	}
	if len(lines) < 3 {
		return class
	}
	if remaining := lines[3:]; len(remaining) > 1 {
		class.Instructor = strings.TrimSpace(remaining[len(remaining)-2])
		class.Room = strings.TrimSpace(remaining[len(remaining)-1])
	} else if len(remaining) == 1 {
		class.Room = strings.TrimSpace(remaining[0])
	}
	return class
}

// blockText renders a selection as text with <br> tags converted to
// line breaks. goquery's Text method drops <br> tags entirely, which
// would merge the instructor and room lines.
func blockText(selection *goquery.Selection) string {
	markup, err := goquery.OuterHtml(selection)
	if err != nil {
		return selection.Text()
	}
	text := brPattern.ReplaceAllString(markup, "\n")
	text = tagPattern.ReplaceAllString(text, "")
	return html.UnescapeString(text)
}

var (
	brPattern  = regexp.MustCompile(`(?i)<br\s*/?>`)
	tagPattern = regexp.MustCompile(`(?s)<[^>]*>`)
)

func splitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := []string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func parseHTML(data []byte) (*goquery.Document, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}
	return doc, nil
}
