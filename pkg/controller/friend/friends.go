package friend

import (
	"net/http"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Friends list and pending friend request endpoint.
func Friends() http.HandlerFunc {
	tmpl := templates.Must("friend/friends.html")

	// Whitelist for ordering your friend list here.
	var sortWhitelist = []string{
		"updated_at desc",
		"updated_at asc",
		"users.username asc",
		"users.username desc",
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			view       = r.FormValue("view")
			isRequests = view == "requests"
			isPending  = view == "pending"
			isIgnored  = view == "ignored"

			// Sorting the friend list.
			sort   = r.FormValue("sort")
			sortOK bool
		)

		// Sort options.
		for _, v := range sortWhitelist {
			if sort == v {
				sortOK = true
				break
			}
		}
		if !sortOK {
			sort = sortWhitelist[0]
		}

		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Unexpected error: could not get currentUser.")
			templates.Redirect(w, "/")
			return
		}

		// Get our friends.
		pager := &models.Pagination{
			PerPage: config.PageSizeFriends,
			Sort:    sort,
		}
		pager.ParsePage(r)

		// If we are doing Friend Requests, sort them so ones with messages are prioritized first.
		// ...when the user hasn't manually overridden the sort.
		if isRequests && !sortOK {
			pager.Sort = "CASE WHEN message IS NOT NULL THEN 0 ELSE 1 END, updated_at DESC"
		}

		friends, err := models.PaginateFriends(currentUser, isRequests, isPending, isIgnored, pager)
		if err != nil {
			session.FlashError(w, r, "Couldn't paginate friends: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Inject relationship booleans.
		models.SetUserRelationships(currentUser, friends)

		// Ignored friend request count.
		ignoredFriendCount, err := models.CountIgnoredFriendRequests(currentUser.ID)
		if err != nil {
			log.Error("Ignored Friend Request Count (%s): %s", currentUser.Username, err)
		}

		// If we are looking at the Requests page: map the friend requests so the template can easily get
		// to the attached message(s) on them.
		var (
			requestMap     models.FriendRequestMap
			requestMapView = "friends"
		)
		if isRequests {
			requestMapView = "requests"
		} else if isPending {
			requestMapView = "pending"
		} else if isIgnored {
			requestMapView = "ignored"
		}
		if m, err := models.MapFriendRequests(currentUser, friends, requestMapView); err != nil {
			session.FlashError(w, r, "Error mapping friend requests: %s", err)
		} else {
			requestMap = m
		}

		var vars = map[string]interface{}{
			"IsRequests":         isRequests,
			"IsPending":          isPending,
			"IsIgnored":          isIgnored,
			"Friends":            friends,
			"IgnoredFriendCount": ignoredFriendCount,
			"FriendRequestMap":   requestMap,
			"Pager":              pager,
			"Sort":               sort,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
