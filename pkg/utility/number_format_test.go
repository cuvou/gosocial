package utility_test

import (
	"testing"

	"github.com/cuvou/gosocial/pkg/utility"
)

func TestNumiberFormat(t *testing.T) {
	var tests = []struct {
		In     int64
		Expect string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{15, "15"},
		{92, "92"},
		{100, "100"},
		{101, "101"},
		{150, "150"},
		{200, "200"},
		{404, "404"},
		{867, "867"},
		{990, "990"},
		{999, "999"},
		{1000, "1K"},
		{1001, "1K"},
		{1010, "1K"},
		{1100, "1.1K"},
		{1111, "1.1K"},
		{1200, "1.2K"},
		{1500, "1.5K"},
		{1700, "1.7K"},
		{1849, "1.8K"},
		{1850, "1.9K"},
		{1899, "1.9K"},
		{1900, "1.9K"},
		{12000, "12K"},
		{12300, "12.3K"},
		{900000, "900K"},
		{900100, "900.1K"},
		{999100, "999.1K"},
		{999500, "999.5K"},
		{999999, "1000K"}, // TODO: not ideal
		{1000000, "1M"},
		{1001000, "1M"},
		{1005000, "1M"},
		{1010000, "1M"},
		{1100000, "1.1M"},
		{1200000, "1.2M"},
		{1305000, "1.3M"},
		{1350000, "1.4M"},
		{1400000, "1.4M"},
		{100000000, "100M"},
		{990509000, "990.5M"},
		{1000000000, "1B"},
	}

	for _, test := range tests {
		actual := utility.FormatNumberShort(test.In)
		if actual != test.Expect {
			t.Errorf("Expected %d to be '%s' but got '%s'", test.In, test.Expect, actual)
		}
	}
}

func TestFilesizeFormat(t *testing.T) {
	var tests = []struct {
		In     int64
		Expect string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{10, "10 B"},
		{15, "15 B"},
		{92, "92 B"},
		{100, "100 B"},
		{101, "101 B"},
		{150, "150 B"},
		{200, "200 B"},
		{404, "404 B"},
		{867, "867 B"},
		{990, "990 B"},
		{999, "999 B"},
		{1000, "1000 B"},
		{1001, "1001 B"},
		{1010, "1010 B"},
		{1100, "1.1 KiB"},
		{1111, "1.1 KiB"},
		{1200, "1.2 KiB"},
		{1500, "1.5 KiB"},
		{1700, "1.7 KiB"},
		{1849, "1.8 KiB"},
		{1850, "1.8 KiB"},
		{1899, "1.9 KiB"},
		{1900, "1.9 KiB"},
		{12000, "11.7 KiB"},
		{12300, "12 KiB"},
		{900000, "878.9 KiB"},
		{900100, "879 KiB"},
		{999100, "975.7 KiB"},
		{999500, "976.1 KiB"},
		{999999, "976.6 KiB"}, // TODO: not ideal
		{1000000, "976.6 KiB"},
		{1001000, "977.5 KiB"},
		{1005000, "981.4 KiB"},
		{1010000, "986.3 KiB"},
		{1100000, "1 MiB"},
		{1200000, "1.1 MiB"},
		{1305000, "1.2 MiB"},
		{1350000, "1.3 MiB"},
		{1400000, "1.3 MiB"},
		{100000000, "95.4 MiB"},
		{990509000, "944.6 MiB"},
		{1510110000, "1.4 GiB"},
	}

	for _, test := range tests {
		actual := utility.FormatFilesize(test.In)
		if actual != test.Expect {
			t.Errorf("Expected %d to be '%s' but got '%s'", test.In, test.Expect, actual)
		}
	}
}
