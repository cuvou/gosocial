package templates_test

import (
	"html/template"
	"testing"

	"github.com/cuvou/gosocial/pkg/templates"
)

func TestNumberF(t *testing.T) {
	// The underlying utility.FormatNumberShort already has thorough tests,
	// this one will test the interface{} type conversions.
	var tests = []struct {
		In     interface{}
		Expect template.HTML
	}{
		{int(0), "0"},
		{int64(0), "0"},
		{uint64(0), "0"},
		{uint(0), "0"},
		{int(123), "123"},
		{int64(123), "123"},
		{uint64(123), "123"},
		{uint(123), "123"},
	}

	for _, test := range tests {
		actual := templates.FormatNumberShort()(test.In)
		if actual != test.Expect {
			t.Errorf("Expected %d to be '%s' but got '%s'", test.In, test.Expect, actual)
		}
	}
}
