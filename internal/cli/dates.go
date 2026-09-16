package cli

import (
	"fmt"
	"time"
)

// parseDate parses a YYYY-MM-DD flag value. An empty value returns the
// zero time.
func parseDate(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: date must be YYYY-MM-DD", errUsage)
	}
	return parsed, nil
}

// scheduleRange resolves the --from/--to/--week flags into a date
// range. The default is the current calendar week, Monday through
// Sunday.
func scheduleRange(from, to string, week int) (time.Time, time.Time, error) {
	if from != "" || to != "" {
		if from == "" || to == "" {
			return time.Time{}, time.Time{}, fmt.Errorf("%w: --from and --to must be used together", errUsage)
		}
		start, err := parseDate(from)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		end, err := parseDate(to)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		if !end.After(start) {
			return time.Time{}, time.Time{}, fmt.Errorf("%w: --to must be after --from", errUsage)
		}
		return start, end, nil
	}
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).
		AddDate(0, 0, -(weekday-1)+week*7)
	return start, start.AddDate(0, 0, 7), nil
}
