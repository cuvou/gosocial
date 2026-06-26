package utility_test

import (
	"testing"
	"time"

	"github.com/cuvou/gosocial/pkg/utility"
)

func TestNextMonth(t *testing.T) {
	var tests = []struct {
		Now    string
		Day    int
		Expect string
	}{
		{
			Now:    "1995-08-01",
			Day:    15,
			Expect: "1995-09-15",
		},
		{
			Now:    "2006-12-15",
			Day:    11,
			Expect: "2007-01-11",
		},
		{
			Now:    "2006-12-01",
			Day:    15,
			Expect: "2007-01-15",
		},
		{
			Now:    "2007-01-15",
			Day:    29, // no leap day
			Expect: "2007-03-01",
		},
		{
			Now:    "2004-01-08",
			Day:    29, // leap day
			Expect: "2004-02-29",
		},
	}

	for i, test := range tests {
		now, err := time.Parse(time.DateOnly, test.Now)
		if err != nil {
			t.Errorf("Test #%d: parse error: %s", i, err)
			continue
		}

		actual := utility.NextMonth(now, test.Day).Format("2006-01-02")
		if actual != test.Expect {
			t.Errorf("Test #%d: expected %s but got %s", i, test.Expect, actual)
		}
	}
}

func TestCalendarDaysRounded(t *testing.T) {
	now, _ := time.Parse(time.DateTime, "2001-06-01 12:00:00")

	var tests = []struct {
		T2     time.Time
		T1     time.Time
		Expect int
	}{
		// Base case.
		{
			T2:     now,
			T1:     now,
			Expect: 0,
		},

		// Days rounded back down.
		{
			T2:     now.Add(6 * time.Hour),
			T1:     now,
			Expect: 0,
		},
		{
			T2:     now.Add(11 * time.Hour),
			T1:     now,
			Expect: 0,
		},

		// Days rounded up (1 day) at hour 11->12
		{
			T2:     now.Add(12 * time.Hour),
			T1:     now,
			Expect: 1,
		},
		{
			T2:     now.Add(13 * time.Hour),
			T1:     now,
			Expect: 1,
		},
		{
			T2:     now.Add(20 * time.Hour),
			T1:     now,
			Expect: 1,
		},
		{
			T2:     now.Add(21 * time.Hour),
			T1:     now,
			Expect: 1,
		},
		{
			T2:     now.Add(22 * time.Hour),
			T1:     now,
			Expect: 1,
		},
		{
			T2:     now.Add(23 * time.Hour),
			T1:     now,
			Expect: 1,
		},
		{
			T2:     now.Add(24 * time.Hour),
			T1:     now,
			Expect: 1,
		},
		{
			T2:     now.Add(25 * time.Hour),
			T1:     now,
			Expect: 1,
		},
		{
			T2:     now.Add(26 * time.Hour),
			T1:     now,
			Expect: 1,
		},
		{
			T2:     now.Add(34 * time.Hour),
			T1:     now,
			Expect: 1,
		},
		{
			T2:     now.Add(35 * time.Hour),
			T1:     now,
			Expect: 1,
		},

		// Days rounded up (2 days) at hour 35->36
		{
			T2:     now.Add(36 * time.Hour),
			T1:     now,
			Expect: 2,
		},
		{
			T2:     now.Add(37 * time.Hour),
			T1:     now,
			Expect: 2,
		},
		{
			T2:     now.Add(40 * time.Hour),
			T1:     now,
			Expect: 2,
		},

		{
			T2:     now.Add(46 * time.Hour),
			T1:     now,
			Expect: 2,
		},
		{
			T2:     now.Add(48 * time.Hour),
			T1:     now,
			Expect: 2,
		},
		{
			T2:     now.Add(59 * time.Hour),
			T1:     now,
			Expect: 2,
		},

		// Days rounded up (3 days) at hour 59->60.
		{
			T2:     now.Add(60 * time.Hour),
			T1:     now,
			Expect: 3,
		},
	}

	for i, test := range tests {
		actual := utility.CalendarDaysRounded(test.T2, test.T1)
		if actual != test.Expect {
			t.Errorf("Test #%d: expected %d but got %d", i, test.Expect, actual)
		}
	}
}

func TestFormatDurationCoarse(t *testing.T) {
	var tests = []struct {
		In     time.Duration
		Expect string
	}{
		{
			In:     0,
			Expect: "0 seconds",
		},
		{
			In:     1 * time.Second,
			Expect: "1 second",
		},
		{
			In:     2 * time.Second,
			Expect: "2 seconds",
		},
		{
			In:     25 * time.Second,
			Expect: "25 seconds",
		},
		{
			In:     59 * time.Second,
			Expect: "59 seconds",
		},
		{
			In:     60 * time.Second,
			Expect: "1 minute",
		},
		{
			In:     90 * time.Second,
			Expect: "1 minute",
		},
		{
			In:     119 * time.Second,
			Expect: "1 minute",
		},
		{
			In:     120 * time.Second,
			Expect: "2 minutes",
		},
		{
			In:     15 * time.Minute,
			Expect: "15 minutes",
		},
		{
			In:     59 * time.Minute,
			Expect: "59 minutes",
		},
		{
			In:     60 * time.Minute,
			Expect: "1 hour",
		},
		{
			In:     75 * time.Minute,
			Expect: "1 hour",
		},
		{
			In:     119 * time.Minute,
			Expect: "1 hour",
		},
		{
			In:     120 * time.Minute,
			Expect: "2 hours",
		},
		{
			In:     14 * time.Hour,
			Expect: "14 hours",
		},
		{
			In:     23 * time.Hour,
			Expect: "23 hours",
		},
		{
			In:     24 * time.Hour,
			Expect: "1 day",
		},
		{
			In:     36 * time.Hour,
			Expect: "1 day",
		},
		{
			In:     48 * time.Hour,
			Expect: "2 days",
		},
		{
			In:     time.Hour * 24 * 60,
			Expect: "2 months",
		},
		{
			In:     time.Hour * 24 * 365,
			Expect: "1 year",
		},
		{
			In:     time.Hour * 24 * 30 * 12,
			Expect: "1 year",
		},
		{
			In:     time.Hour*24*30*12 + time.Hour*12,
			Expect: "1 year",
		},
		{
			In:     time.Hour * 24 * 30 * 13,
			Expect: "1.1 years",
		},
		{
			In:     time.Hour * 24 * 30 * 18,
			Expect: "1.5 years",
		},
		{
			In:     time.Hour * 24 * 30 * 22,
			Expect: "1.8 years",
		},
		{
			In:     time.Hour * 24 * 30 * 24,
			Expect: "2 years",
		},
		{
			In:     time.Hour * 24 * 30 * 36,
			Expect: "3 years",
		},
		{
			In:     time.Hour * 24 * 30 * 49,
			Expect: "4 years",
		},
	}

	for _, test := range tests {
		actual := utility.FormatDurationCoarse(test.In)
		if actual != test.Expect {
			t.Errorf("Expected %d to be '%s' but got '%s'", test.In, test.Expect, actual)
		}
	}
}
