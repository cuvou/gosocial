package account

import (
	"net/http"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/utility"
)

// User friends page (/friends/u/username)
func UserFriends() http.HandlerFunc {
	tmpl := templates.Must("account/friends.html")

	var sortWhitelist = []string{
		"updated_at desc",
		"updated_at asc",
		"users.username asc",
		"users.username desc",
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Filters
		var (
			showMutuals = r.FormValue("mutual") == "true"
			sort        = utility.StringIn(r.FormValue("sort"), sortWhitelist, sortWhitelist[0])
		)

		// Parse the username out of the URL parameters.
		var username = r.PathValue("username")

		// Find this user.
		user, err := models.FindUsername(username)
		if err != nil {
			templates.NotFoundPage(w, r)
			return
		}

		// Get the viewer.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Unexpected error: could not get currentUser.")
			templates.Redirect(w, "/")
			return
		}

		var isSelf = currentUser.ID == user.ID

		// Give a Not Found page if we can not see this user (banned, blocking).
		if err := user.CanBeSeenBy(currentUser); err != nil {
			log.Error("%s can not be seen by viewer %s: %s", user.Username, currentUser.Username, err)
			templates.NotFoundPage(w, r)
			return
		}

		// Get the user's privacy setting in case we will withhold the friend list.
		var (
			privacySetting = models.GetPrivacySetting(user.ID)
			friends        = []*models.User{}
			showFriends    = true
		)
		if !currentUser.IsAdmin {
			switch privacySetting.FriendsList {
			case "me":
				showFriends = isSelf
			case "friends":
				showFriends = models.AreFriends(currentUser.ID, user.ID)
			case "":
				showFriends = true // all certified members can see
			default:
				showFriends = false // safety fallback handler
			}
		}

		// Get their friends.
		pager := &models.Pagination{
			PerPage: config.PageSizeFriends,
			Sort:    sort,
		}
		pager.ParsePage(r)
		if showFriends {
			if f, err := models.PaginateOtherUserFriends(currentUser, user, showMutuals, pager); err != nil {
				session.FlashError(w, r, "Couldn't paginate friends: %s", err)
				templates.Redirect(w, "/")
				return
			} else {
				friends = f
			}
		}

		// Preload all of their profile fields, so their hometown/pronouns/etc. are available.
		if err := models.PreloadUserProfileFields(friends); err != nil {
			log.Error("Preloading %d users' profile fields: %s", len(friends), err)
		}

		// Profile tab counts.
		tabCounts, err := user.GetProfileTabCounts(currentUser)
		if err != nil {
			session.FlashError(w, r, "Error getting profile tab counts: %s", err)
		}

		var vars = map[string]interface{}{
			"ActiveProfileTab":  "friends",
			"User":              user,
			"ProfileTheme":      models.GetProfileTheme(user.ID),
			"IsSelf":            isSelf,
			"PrivacySetting":    models.GetPrivacySetting(user.ID),
			"FriendListVisible": showFriends,
			"ProfileTabCount":   tabCounts,
			"MutualFriendCount": models.CountMutualFriends(currentUser, user),
			"Friends":           friends,
			"Pager":             pager,
			"Sort":              sort,

			// Map our friendships to these users.
			"FriendMap": models.MapFriends(currentUser, friends),

			// Filters
			"FilterMutual": showMutuals,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
