package markdown_test

import (
	"testing"

	"github.com/cuvou/gosocial/pkg/markdown"
)

func TestDeMarkify(t *testing.T) {
	var cases = []struct {
		Input  string
		Expect string
	}{
		{
			Input:  "Hello world!",
			Expect: "Hello world!",
		},
		{
			Input:  "[@username](/go/comment?id=1234) Very well said!",
			Expect: "@username Very well said!",
		},
		{
			Input:  `<a href="https://wikipedia.org">Wikipedia</a> said **otherwise.**`,
			Expect: "Wikipedia said **otherwise.**",
		},
		{
			Input:  "[Here](/here) is one [link](https://example.com), while [Here](/here) is [another](/another).",
			Expect: "Here is one link, while Here is another.",
		},
	}

	for i, tc := range cases {
		actual := markdown.DeMarkify(tc.Input)
		if actual != tc.Expect {
			t.Errorf("Test #%d: expected '%s' but got '%s'", i, tc.Expect, actual)
		}
	}
}
