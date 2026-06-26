package models

import (
	"math"
	"net/http"
	"strconv"
)

// Pagination result object.
type Pagination struct {
	Page    int // provide <0 to mean "last page"
	PerPage int
	Total   int64
	Sort    string

	// privates
	lastPage bool
}

// Load the page from form or query parameters.
func (p *Pagination) ParsePage(r *http.Request) {
	raw := r.FormValue("page")
	a, err := strconv.Atoi(raw)
	if err == nil {
		if a <= 0 {
			p.lastPage = true
			a = 1
		}
		p.Page = a
	} else {
		p.Page = 1
	}
}

// Page for Iter.
type Page struct {
	Page      int  // label to show in the button
	IsCurrent bool // highlight the currently selected page button
}

// Iter the pages, for templates.
//
// This will return up to 5 page numbers which includes the current page. If there
// are enough pages in total, the current page is centered with 3 neighboring pages
// on either side.
func (p *Pagination) Iter() []Page {
	var (
		pages     = []Page{}
		total     = p.Pages()
		firstPage int
		lastPage  int
	)

	firstPage = p.Page - 2
	if firstPage < 1 {
		firstPage = 1
	}

	lastPage = firstPage + 4
	if lastPage > total {
		lastPage = total

		// If we are on the very last pages, see if we can count further backwards
		// to still show all 5 page buttons.
		firstPage = total - 4
		if firstPage < 1 {
			firstPage = 1
		}
	}

	for i := firstPage; i <= lastPage; i++ {
		pages = append(pages, Page{
			Page:      i,
			IsCurrent: i == p.Page,
		})
	}

	return pages
}

func (p Pagination) Pages() int {
	if p.PerPage == 0 {
		return 0
	}
	return int(math.Ceil(float64(p.Total) / float64(p.PerPage)))
}

func (p *Pagination) GetOffset() int {
	// Are we looking for the FINAL page?
	if p.lastPage && p.Pages() >= 1 {
		p.Page = p.Pages()
	}
	return (p.Page - 1) * p.PerPage
}

func (p *Pagination) HasNext() bool {
	return p.Page < p.Pages()
}

func (p *Pagination) HasPrevious() bool {
	return p.Page > 1
}

func (p *Pagination) Next() int {
	if p.Page >= p.Pages() {
		return p.Pages()
	}
	return p.Page + 1
}

func (p *Pagination) Previous() int {
	if p.Page > 1 {
		return p.Page - 1
	}
	return 1
}
