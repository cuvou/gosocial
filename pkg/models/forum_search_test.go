package models_test

import (
	"testing"

	"github.com/cuvou/gosocial/pkg/models"
)

func TestSearchPars(t *testing.T) {
	var table = []struct {
		Query  string
		Expect models.Search
	}{
		{
			Query: "hello world",
			Expect: models.Search{
				Includes: []string{"hello", "world"},
				Excludes: []string{},
			},
		},
		{
			Query: "hello world -mars",
			Expect: models.Search{
				Includes: []string{"hello", "world"},
				Excludes: []string{"mars"},
			},
		},
		{
			Query: `"hello world" -mars`,
			Expect: models.Search{
				Includes: []string{"hello world"},
				Excludes: []string{"mars"},
			},
		},
		{
			Query: `the "quick brown" fox -jumps -"over the" lazy -dog`,
			Expect: models.Search{
				Includes: []string{"the", "quick brown", "fox", "lazy"},
				Excludes: []string{"jumps", "over the", "dog"},
			},
		},
		{
			Query: `how now brown cow`,
			Expect: models.Search{
				Includes: []string{"how", "now", "brown", "cow"},
				Excludes: []string{},
			},
		},
		{
			Query: `"this exact phrase"`,
			Expect: models.Search{
				Includes: []string{"this exact phrase"},
				Excludes: []string{},
			},
		},
		{
			Query: `-"not this exact phrase"`,
			Expect: models.Search{
				Includes: []string{},
				Excludes: []string{"not this exact phrase"},
			},
		},
		{
			Query: `something "bust"ed" -this "-way" comes  `,
			Expect: models.Search{
				Includes: []string{"something", "bust", "ed -this"},
				Excludes: []string{"way comes"},
			},
		},
		{
			Query:  "",
			Expect: models.Search{},
		},
		{
			Query:  `"`,
			Expect: models.Search{},
		},
		{
			Query:  "-",
			Expect: models.Search{},
		},
		{
			Query: "-1",
			Expect: models.Search{
				Excludes: []string{"1"},
			},
		},
		{
			Query:  `""`,
			Expect: models.Search{},
		},
		{
			Query:  `"""`,
			Expect: models.Search{},
		},
		{
			Query:  `""""`,
			Expect: models.Search{},
		},
		{
			Query:  `--`,
			Expect: models.Search{},
		},
		{
			Query:  `---`,
			Expect: models.Search{},
		},
		{
			Query: `"chat room" -spam -naked`,
			Expect: models.Search{
				Includes: []string{"chat room"},
				Excludes: []string{"spam", "naked"},
			},
		},
		{
			Query: `yes1 yes2 -no1 -no2 -no3 yes3 -no4 -no5 yes4 -no6`,
			Expect: models.Search{
				Includes: []string{"yes1", "yes2", "yes3", "yes4"},
				Excludes: []string{"no1", "no2", "no3", "no4", "no5", "no6"},
			},
		},
	}

	for i, test := range table {
		actual := models.ParseSearchString(test.Query)
		if !actual.Equals(test.Expect) {
			t.Errorf("Test #%d failed: search string `%s` expected %s but got %s", i, test.Query, test.Expect, actual)
		}
	}
}
