package models

import (
	"encoding/json"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
)

// Search represents a parsed search query with inclusions and exclusions.
type Search struct {
	Includes []string
	Excludes []string
}

// ParseSearchString parses a user search query and supports "quoted phrases" and -negations.
func ParseSearchString(input string) *Search {
	var result = new(Search)

	var (
		negate bool
		phrase bool
		buf    = []rune{}
		commit = func() {
			var text = strings.TrimSpace(string(buf))
			if len(text) == 0 {
				return
			}
			if negate {
				result.Excludes = append(result.Excludes, text)
				negate = false
			} else {
				result.Includes = append(result.Includes, text)
			}
			buf = []rune{}
		}
	)

	for _, char := range input {
		// Inside a quoted phrase?
		if phrase {
			if char == '"' {
				// End of quoted phrase.
				commit()
				phrase = false
				continue
			}
			buf = append(buf, char)
			continue
		}

		// Start a quoted phrase?
		if char == '"' {
			phrase = true
			continue
		}

		// Negation indicator?
		if len(buf) == 0 && char == '-' {
			negate = true
			continue
		}

		// End of a word?
		if char == ' ' {
			commit()
			continue
		}

		buf = append(buf, char)
	}

	// Last word?
	commit()

	return result
}

// ForumSearchFilters apply additional filters specific to searching the forum.
type ForumSearchFilters struct {
	UserID      uint64
	Fragment    string
	ThreadsOnly bool
	WithPhotos  bool
}

// SearchForum searches the forum.
func SearchForum(user *User, categories []string, search *Search, filters ForumSearchFilters, pager *Pagination) ([]*Comment, error) {
	var (
		coms         = []*Comment{}
		query        = (&Comment{}).Preload()
		wheres       = []string{"table_name = 'threads'"}
		placeholders = []any{}
	)

	// Specific forum fragment? If provided, it overrides the categories filter.
	if filters.Fragment != "" {
		wheres = append(wheres, "forums.fragment = ?")
		placeholders = append(placeholders, filters.Fragment)
	} else if len(categories) > 0 {
		wheres = append(wheres, "category IN ?")
		placeholders = append(placeholders, categories)
	}

	// Hide explicit forum if user hasn't opted into it.
	if !user.Explicit && !user.IsAdmin {
		wheres = append(wheres, "forums.explicit = false")
	}

	// Private forums.
	if !user.HasAdminScope(config.ScopeAdminBase) {
		wheres = append(wheres, "forums.private is not true")
	}

	// Blocking user IDs?
	bw, bp := BlockedUserSubquery("comments.user_id", user.ID)
	wheres = append(wheres, bw)
	placeholders = append(placeholders, bp...)

	// Don't show comments from banned or disabled accounts.
	wheres = append(wheres, `
		EXISTS (
			SELECT 1
			FROM users
			WHERE users.id = comments.user_id
			AND users.status = 'active'
		)
	`)

	// Apply their search terms.
	if filters.UserID > 0 {
		wheres = append(wheres, "comments.user_id = ?")
		placeholders = append(placeholders, filters.UserID)
	}
	if filters.ThreadsOnly {
		wheres = append(wheres, "comments.id = threads.comment_id")
	}
	for _, term := range search.Includes {
		var ilike = "%" + strings.ToLower(term) + "%"
		wheres = append(wheres, "(comments.message ILIKE ? OR threads.title ILIKE ?)")
		placeholders = append(placeholders, ilike, ilike)
	}
	for _, term := range search.Excludes {
		var ilike = "%" + strings.ToLower(term) + "%"
		wheres = append(wheres, "(comments.message NOT ILIKE ? AND threads.title NOT ILIKE ?)")
		placeholders = append(placeholders, ilike, ilike)
	}
	if filters.WithPhotos {
		wheres = append(wheres, `
			EXISTS (
				SELECT 1
				FROM comment_photos
				WHERE comment_photos.comment_id = comments.id
			)
		`)
	}

	query = query.Joins(
		"JOIN threads ON (comments.table_id = threads.id)",
	).Joins(
		"JOIN forums ON (threads.forum_id = forums.id)",
	).Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	).Order(pager.Sort)

	query.Model(&Comment{}).Count(&pager.Total)
	res := query.Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&coms)
	if res.Error != nil {
		return nil, res.Error
	}

	// Inject user relationships into all comments now.
	SetUserRelationshipsInComments(user, coms)

	return coms, nil
}

// Equals inspects if the search result matches, to help with unit tests.
func (s Search) Equals(other Search) bool {
	if len(s.Includes) != len(other.Includes) || len(s.Excludes) != len(other.Excludes) {
		return false
	}

	for i, v := range s.Includes {
		if other.Includes[i] != v {
			return false
		}
	}

	for i, v := range s.Excludes {
		if other.Excludes[i] != v {
			return false
		}
	}

	return true
}

func (s Search) String() string {
	b, _ := json.Marshal(s)
	return string(b)
}
