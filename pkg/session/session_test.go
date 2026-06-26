package session_test

import (
	"net/http"
	"testing"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/utility"
)

func TestRemoteAddr(t *testing.T) {
	var tests = []struct {
		RemoteAddr    string
		XForwardedFor string
		Expect        string
	}{
		{
			"127.0.0.1:12345",
			"",
			"127.0.0.1",
		},
		{
			"127.0.0.1:22022",
			"8.8.4.4",
			"8.8.4.4",
		},
		{
			"127.0.0.1:11223",
			"8.8.4.4, 8.8.1.1, 1.1.1.1",
			"8.8.4.4",
		},
		{
			"127.0.0.1",
			"8.8.8.8, 8.8.4.4, 1.1.1.1",
			"8.8.8.8",
		},
		{
			"127.0.0.1",
			"2001:db8:85a3:8d3:1319:8a2e:370:7348",
			"2001:db8:85a3:8d3:1319:8a2e:370:7348",
		},
		{
			"127.0.0.1",
			"2001:db8:85a3:8d3:1319:8a2e:370:7bee",
			"2001:db8:85a3:8d3:1319:8a2e:370:7bee",
		},
		{
			"127.0.0.1",
			"2001:db8:85a3:8d3:1319:8a2e:370:7bee, 127.0.0.7",
			"2001:db8:85a3:8d3:1319:8a2e:370:7bee",
		},
	}

	// Test all cases with X-Forwarded-For enabled.
	config.Current.UseXForwardedFor = true
	for i, test := range tests {
		r, _ := http.NewRequest("GET", "/", nil)
		r.RemoteAddr = test.RemoteAddr
		if test.XForwardedFor != "" {
			r.Header.Set("X-Forwarded-For", test.XForwardedFor)
		}

		actual := utility.IPAddress(r)
		if actual != test.Expect {
			t.Errorf("#%d RemoteAddr expected %s but got %s for (RemoteAddr=%s  XForwardedFor=%s)",
				i, test.Expect, actual, test.RemoteAddr, test.XForwardedFor,
			)
		}
	}

	// Test them without X-Forwarded-For -- the expect should always be the RemoteAddr.
	config.Current.UseXForwardedFor = false
	for _, test := range tests {
		r, _ := http.NewRequest("GET", "/", nil)
		r.RemoteAddr = test.RemoteAddr
		if test.XForwardedFor != "" {
			r.Header.Set("X-Forwarded-For", test.XForwardedFor)
		}

		actual := utility.IPAddress(r)
		if actual != "127.0.0.1" {
			t.Errorf("Without X-Forwarded-For %+v did not return 127.0.0.1", test)
		}
	}
}
