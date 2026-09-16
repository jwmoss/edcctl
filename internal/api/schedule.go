package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// CalendarEvent is one entry from the full calendar feed. The portal
// returns these for both recurring classes and studio events.
type CalendarEvent struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	StudentID string `json:"sid"`
	Title     string `json:"title"`
	Start     string `json:"start"`
	End       string `json:"end"`
}

// Schedule fetches class and event entries for the range [from, to).
// Both bounds are dates; to is exclusive.
func (c *Client) Schedule(ctx context.Context, from, to time.Time) ([]CalendarEvent, error) {
	if from.IsZero() || to.IsZero() || to.Format("2006-01-02") <= from.Format("2006-01-02") {
		return nil, fmt.Errorf("schedule end date must be after its start date")
	}
	form := url.Values{
		"action":        {"getclasses"},
		"season_id":     {""},
		"teacher_id":    {""},
		"location_id":   {""},
		"room_id":       {""},
		"start":         {from.Format("2006-01-02")},
		"end":           {to.Format("2006-01-02")},
		"selected_view": {"agendaWeek"},
	}
	body, err := c.do(ctx, http.MethodPost, "/class_calendar-ajax.php", form, "application/json")
	if err != nil {
		return nil, fmt.Errorf("load schedule: %w", err)
	}
	var events []CalendarEvent
	if err := json.Unmarshal(body, &events); err != nil || events == nil {
		return nil, fmt.Errorf("schedule endpoint did not return a JSON event array; the session may have expired")
	}

	// The feed can return recurrences beyond the requested range, so
	// filter and sort client-side.
	filtered := make([]CalendarEvent, 0, len(events))
	for _, event := range events {
		start, err := time.ParseInLocation("2006-01-02T15:04:05", event.Start, from.Location())
		if err != nil {
			// FullCalendar also supports all-day events with a date only.
			start, err = time.ParseInLocation("2006-01-02", event.Start, from.Location())
			if err != nil {
				return nil, fmt.Errorf("schedule JSON contains an invalid event start; refusing an incomplete schedule")
			}
		}
		if start.Before(from) || !start.Before(to) {
			continue
		}
		filtered = append(filtered, event)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Start < filtered[j].Start
	})
	return filtered, nil
}

// ScheduleWeek is a convenience wrapper that fetches the calendar week
// containing t. Weeks run Monday through Sunday.
func (c *Client) ScheduleWeek(ctx context.Context, t time.Time) ([]CalendarEvent, error) {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	start := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).AddDate(0, 0, -(weekday - 1))
	return c.Schedule(ctx, start, start.AddDate(0, 0, 7))
}

func (e CalendarEvent) Plain() string {
	return strings.Join([]string{e.ID, e.Type, e.StudentID, e.Title, e.Start, e.End}, "\t")
}
