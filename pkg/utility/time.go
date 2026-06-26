package utility

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// NextMonth takes an input time (usually time.Now) and will return the next month on the given day.
//
// Example, NextMonth from any time in April should return e.g. May 10th.
func NextMonth(now time.Time, day int) time.Time {
	var (
		year, month, _ = now.Date()
		nextMonth      = month + 1
	)
	if nextMonth > 12 {
		nextMonth = 1
		year++
	}

	return time.Date(year, nextMonth, day, 0, 0, 0, 0, now.Location())
}

// CalendarDaysRounded returns the calendar day difference between times (t2 - t1).
func CalendarDaysRounded(t2, t1 time.Time) int {
	y, m, d := t2.Date()
	u2 := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	y, m, d = t1.In(t2.Location()).Date()
	u1 := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	days := u2.Sub(u1) / (24 * time.Hour)
	return int(math.Round(float64(days)))
}

// FormatDurationCoarse returns a pretty printed duration with coarse granularity.
func FormatDurationCoarse(duration time.Duration) string {
	// Negative durations (e.g. future dates) should work too.
	if duration < 0 {
		duration *= -1
	}

	var result = func(text string, v int64) string {
		if v == 1 {
			text = strings.TrimSuffix(text, "s")
		}
		return fmt.Sprintf(text, v)
	}

	if duration.Seconds() < 60.0 {
		return result("%d seconds", int64(duration.Seconds()))
	}

	if duration.Minutes() < 60.0 {
		return result("%d minutes", int64(duration.Minutes()))
	}

	if duration.Hours() < 24.0 {
		return result("%d hours", int64(duration.Hours()))
	}

	days := int64(duration.Hours() / 24)
	if days < 30 {
		return result("%d days", days)
	}

	months := int64(days / 30)
	if months < 12 {
		return result("%d months", months)
	}

	// Over one year: start to show it as a floating point number of years (e.g. "1.2 years")
	years := float64(days) / 365
	s := strconv.FormatFloat(years, 'f', 1, 64)
	if strings.HasSuffix(s, ".0") {
		y, _ := strconv.Atoi(strings.Split(s, ".")[0])
		return result("%d years", int64(y))
	}
	return fmt.Sprintf("%s years", s)
}

// FormatDurationFloatingCoarse returns a pretty printed duration with coarse granularity,
// with an optimization to display in terms of hours.
//
// The hours will show as a floating point number like "0.9 hours" instead of rounding down.
func FormatDurationFloatingCoarse(duration time.Duration) string {
	// Negative durations (e.g. future dates) should work too.
	if duration < 0 {
		duration *= -1
	}

	var result = func(text string, v string) string {
		if v == "1" || v == "1.0" {
			text = strings.TrimSuffix(text, "s")
		}
		return fmt.Sprintf(text, v)
	}

	if duration.Seconds() < 60.0 {
		return result("%s seconds", FormatFloatToPrecision(duration.Seconds(), 1))
	}

	if duration.Minutes() < 60.0 {
		return result("%s minutes", FormatFloatToPrecision(duration.Minutes(), 1))
	}

	if duration.Hours() < 24.0 {
		return result("%s hours", FormatFloatToPrecision(duration.Hours(), 1))
	}

	days := duration.Hours() / 24
	if days < 30 {
		return result("%s days", FormatFloatToPrecision(days, 1))
	}

	months := days / 30
	if months < 12 {
		return result("%s months", FormatFloatToPrecision(months, 1))
	}

	// Over one year: start to show it as a floating point number of years (e.g. "1.2 years")
	years := days / 365
	s := strconv.FormatFloat(years, 'f', 1, 64)
	return result("%s years", s)
}
