// Package demographic handles periodic report pulling for high level website statistics.
//
// It powers the home page and insights page, where a prospective new user can get a peek inside
// the website to see the split between regular vs. explicit content and membership statistics.
//
// These database queries could get slow so the demographics are pulled and cached in this package.
package demographic

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/utility"
)

// Demographic is the top level container struct with all the insights needed for front-end display.
type Demographic struct {
	Computed    bool
	LastUpdated time.Time
	Photo       Photo
	People      People
}

// Photo statistics show the split between explicit and non-explicit content.
type Photo struct {
	Total       int64
	NonExplicit int64
	Explicit    int64
}

// People statistics.
type People struct {
	Total         int64
	ExplicitOptIn int64
	ExplicitPhoto int64
	ByAgeRange    map[string]int64
	ByGender      map[string]int64
	ByOrientation map[string]int64
}

// MemberDemographic of members.
type MemberDemographic struct {
	Label   string // e.g. age range "18-25" or gender
	Count   int64
	Percent string
}

/**
 * Dynamic calculation methods on the above types (percentages, etc.)
 */

func (d Demographic) PrettyPrint() string {
	b, err := json.MarshalIndent(d, "", "\t")
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func (p Photo) PercentExplicit() string {
	if p.Total == 0 {
		return "0"
	}
	return utility.FormatFloatToPrecision((float64(p.Explicit)/float64(p.Total))*100, 1)
}

func (p Photo) PercentNonExplicit() string {
	if p.Total == 0 {
		return "0"
	}
	return utility.FormatFloatToPrecision((float64(p.NonExplicit)/float64(p.Total))*100, 1)
}

func (p People) PercentExplicit() string {
	if p.Total == 0 {
		return "0"
	}
	return utility.FormatFloatToPrecision((float64(p.ExplicitOptIn)/float64(p.Total))*100, 1)
}

func (p People) PercentExplicitPhoto() string {
	if p.Total == 0 {
		return "0"
	}
	return utility.FormatFloatToPrecision((float64(p.ExplicitPhoto)/float64(p.Total))*100, 1)
}

func (p People) IterAgeRanges() []MemberDemographic {
	var (
		result = []MemberDemographic{}
		values = []string{}
		unique = map[string]struct{}{}
	)

	for age := range p.ByAgeRange {
		if _, ok := unique[age]; !ok {
			values = append(values, age)
		}
		unique[age] = struct{}{}
	}

	sort.Strings(values)
	for _, age := range values {
		var (
			count = p.ByAgeRange[age]
			pct   float64
		)
		if p.Total > 0 {
			pct = ((float64(count) / float64(p.Total)) * 100)
		}

		result = append(result, MemberDemographic{
			Label:   age,
			Count:   p.ByAgeRange[age],
			Percent: utility.FormatFloatToPrecision(pct, 1),
		})
	}

	return result
}

func (p People) IterGenders() []MemberDemographic {
	var (
		result = []MemberDemographic{}
		values = append(config.Gender, "")
		unique = map[string]struct{}{}
	)

	for _, option := range values {
		unique[option] = struct{}{}
	}

	for gender := range p.ByGender {
		if _, ok := unique[gender]; !ok {
			values = append(values, gender)
			unique[gender] = struct{}{}
		}
	}

	for _, gender := range values {
		var (
			count = p.ByGender[gender]
			pct   float64
		)
		if p.Total > 0 {
			pct = ((float64(count) / float64(p.Total)) * 100)
		}

		result = append(result, MemberDemographic{
			Label:   gender,
			Count:   p.ByGender[gender],
			Percent: utility.FormatFloatToPrecision(pct, 1),
		})
	}

	return result
}

func (p People) IterOrientations() []MemberDemographic {
	var (
		result = []MemberDemographic{}
		values = append(config.Orientation, "")
		unique = map[string]struct{}{}
	)

	for _, option := range values {
		unique[option] = struct{}{}
	}

	for orientation := range p.ByOrientation {
		if _, ok := unique[orientation]; !ok {
			values = append(values, orientation)
			unique[orientation] = struct{}{}
		}
	}

	for _, gender := range values {
		var (
			count = p.ByOrientation[gender]
			pct   float64
		)
		if p.Total > 0 {
			pct = ((float64(count) / float64(p.Total)) * 100)
		}

		result = append(result, MemberDemographic{
			Label:   gender,
			Count:   p.ByOrientation[gender],
			Percent: utility.FormatFloatToPrecision(pct, 1),
		})
	}

	return result
}
