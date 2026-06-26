package utility

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// FormatFloatToPrecision will trim a floating point number to at most a number of decimals of precision.
//
// If the precision is ".0" the decimal place will be stripped entirely.
func FormatFloatToPrecision(v float64, prec int) string {
	s := strconv.FormatFloat(v, 'f', prec, 64)
	if strings.HasSuffix(s, ".0") {
		return strings.Split(s, ".")[0]
	}
	return s
}

// FormatNumberShort compresses a number into as short a string as possible (e.g. "1.2K" when it gets into the thousands).
func FormatNumberShort(value int64) string {
	// Under 1,000?
	if value < 1000 {
		return fmt.Sprintf("%d", value)
	}

	// Start to bucket it.
	var (
		thousands = float64(value) / 1000
		millions  = float64(thousands) / 1000
		billions  = float64(millions) / 1000
	)

	if thousands < 1000 {
		return fmt.Sprintf("%sK", FormatFloatToPrecision(thousands, 1))
	}

	if millions < 1000 {
		return fmt.Sprintf("%sM", FormatFloatToPrecision(millions, 1))
	}

	return fmt.Sprintf("%sB", FormatFloatToPrecision(billions, 1))
}

// FormatNumberCommas formats a number with commas, e.g. "1,023,456"
//
// Works with *int, *int64 and floats.
func FormatNumberCommas(v any) string {
	var number int64
	switch t := v.(type) {
	case int:
		number = int64(t)
	case int64:
		number = int64(t)
	case uint:
		number = int64(t)
	case uint64:
		number = int64(t)
	case float32:
		return FormatFloatCommas(float64(t), 1)
	case float64:
		return FormatFloatCommas(t, 1)
	default:
		return "#INVALID#"
	}
	p := message.NewPrinter(language.English)
	return p.Sprintf("%d", number)
}

// FormatFloatCommas formats a float64 into commas with a level of precision.
func FormatFloatCommas(v float64, precision int) string {
	p := message.NewPrinter(language.English)
	return p.Sprintf("%.1f", v)
}

// FormatFilesize formats a file size in bytes to human readable format.
func FormatFilesize(value int64) string {
	// Under 1KB?
	if value < 1024 {
		return fmt.Sprintf("%d B", value)
	}

	// Start to bucket it.
	var (
		kb = float64(value) / 1024
		mb = kb / 1024
		gb = mb / 1024
		tb = gb / 1024
	)

	if kb < 1024 {
		return fmt.Sprintf("%s KiB", FormatFloatToPrecision(kb, 1))
	}

	if mb < 1024 {
		return fmt.Sprintf("%s MiB", FormatFloatToPrecision(mb, 1))
	}

	if gb < 1024 {
		return fmt.Sprintf("%s GiB", FormatFloatToPrecision(gb, 1))
	}

	return fmt.Sprintf("%s TiB", FormatFloatToPrecision(tb, 1))
}
