package models

import (
	"net/http"
)

// NotificationFilter handles users filtering their notification list by category. It is populated
// from front-end checkboxes and translates to SQL filters for PaginateNotifications.
type NotificationFilter struct {
	Likes         bool `json:"likes"` // form field name
	Comments      bool `json:"comments"`
	Friends       bool `json:"friends"`
	NewPhotos     bool `json:"photos"`
	AlsoCommented bool `json:"replies"` // also_comment and also_posted
	PrivatePhoto  bool `json:"private"`
	Events        bool `json:"events"`
	Misc          bool `json:"misc"` // friendship_approved, cert_approved, cert_rejected, custom
}

var defaultNotificationFilter = NotificationFilter{
	Likes:         true,
	Comments:      true,
	Friends:       true,
	NewPhotos:     true,
	AlsoCommented: true,
	PrivatePhoto:  true,
	Events:        true,
	Misc:          true,
}

// NotificationFilterMap associates notification types with their relevant NotificationFilter setting.
//
// Keep the names here in the same order as the const in notification.go.
var NotificationFilterMap = map[NotificationType]string{
	NotificationLike:           "likes",
	NotificationFriendApproved: "friends",
	NotificationComment:        "comments",
	NotificationAlsoCommented:  "replies",
	NotificationAlsoPosted:     "replies",
	NotificationPrivatePhoto:   "private",
	NotificationNewPhoto:       "photos",
	NotificationForumModerator: "misc",
	NotificationFollow:         "friends",
	NotificationFollowBack:     "friends",
	NotificationCustom:         "misc",
}

// NewNotificationFilterFromForm creates a NotificationFilter struct parsed from an HTTP form.
func NewNotificationFilterFromForm(r *http.Request) NotificationFilter {
	// Are these boxes checked in a frontend post?
	var (
		nf = NotificationFilter{
			Likes:         r.FormValue("likes") == "true",
			Comments:      r.FormValue("comments") == "true",
			Friends:       r.FormValue("friends") == "true",
			NewPhotos:     r.FormValue("photos") == "true",
			AlsoCommented: r.FormValue("replies") == "true",
			PrivatePhoto:  r.FormValue("private") == "true",
			Events:        r.FormValue("events") == "true",
			Misc:          r.FormValue("misc") == "true",
		}
	)

	// Default view or when no checkboxes were sent, all are true.
	if nf.IsZero() {
		return defaultNotificationFilter
	}
	return nf
}

// IsZero checks for an empty filter.
func (nf NotificationFilter) IsZero() bool {
	return nf == NotificationFilter{}
}

// IsAll checks if all filters are checked.
func (nf NotificationFilter) IsAll() bool {
	return nf == defaultNotificationFilter
}

// Query returns the SQL "WHERE" clause that applies the filters to the Notifications query.
//
// If no filters should be added, ok returns false.
func (nf NotificationFilter) Query() (where string, placeholders []interface{}, ok bool) {
	if nf.IsAll() || nf.IsZero() {
		return "", nil, false
	}

	var (
		// Notification types to include.
		types  = []any{}
		reduce = func(category string) {
			for k, v := range NotificationFilterMap {
				if v == category {
					types = append(types, k)
				}
			}
		}
	)

	// Translate
	if nf.Likes {
		reduce("likes")
	}
	if nf.Comments {
		reduce("comments")
	}
	if nf.Friends {
		reduce("friends")
	}
	if nf.NewPhotos {
		reduce("photos")
	}
	if nf.AlsoCommented {
		reduce("replies")
	}
	if nf.PrivatePhoto {
		reduce("private")
	}
	if nf.Events {
		reduce("events")
	}
	if nf.Misc {
		reduce("misc")
	}

	return "type IN ?", types, true
}
