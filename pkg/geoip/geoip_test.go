package geoip_test

import (
	"testing"

	"github.com/cuvou/gosocial/pkg/geoip"
)

func TestCountryFlags(t *testing.T) {
	table := []struct {
		input  string
		expect string
		err    bool
	}{
		{"US", "🇺🇸", false},
		{"CA", "🇨🇦", false},
		{"AU", "🇦🇺", false},
		{"NZ", "🇳🇿", false},
		{"CN", "🇨🇳", false},
		{"invalid", "", true},
	}

	for _, test := range table {
		emoji, err := geoip.CountryFlagEmoji(test.input)
		if err != nil && !test.err {
			t.Errorf("Country %s: got an error but did not expect to: %s", test.input, err)
			continue
		}

		if emoji != test.expect {
			t.Errorf("Country %s: did not get expected emoji %s, got %+v", test.input, test.expect, emoji)
		}
	}
}
