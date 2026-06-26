package account

import (
	"net/http"
	"net/url"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/middleware"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/worker"
)

// User profile page (/u/username)
func Profile() http.HandlerFunc {
	tmpl := templates.Must("account/profile.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse the username out of the URL parameters.
		var (
			username       = r.PathValue("username")
			isExternalView = r.FormValue("view") == "external"
		)

		// Find this user.
		user, err := models.FindUsername(username)
		if err != nil {
			templates.NotFoundPage(w, r)
			return
		}

		// Get the current user (if logged in). If not, check for external view.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			// The viewer is not logged in, bail now with the basic profile page. If this
			// user doesn't allow external viewers, redirect to login page.
			if user.Visibility != models.UserVisibilityExternal {
				session.FlashError(w, r, "You must be signed in to view this page.")
				templates.Redirect(w, "/login?next="+url.QueryEscape(r.URL.String()))
				return
			}

			isExternalView = true
		}

		// Don't show a banned or deactivated profile, except for admin view.
		if user.Status != models.UserStatusActive && !(currentUser != nil && currentUser.IsAdmin) {
			templates.NotFoundPage(w, r)
			return
		}

		// Get block list status between the two users.
		var (
			blockForward, blockReverse bool
		)
		if currentUser != nil {
			blockForward, blockReverse = models.BlockDirections(currentUser.ID, user.ID)
		}

		// If we are logged in and we block the target user, also show the minimal external view.
		// Note: if the target blocks us back, it will be a Not Found error.
		if currentUser != nil && !currentUser.IsAdmin && blockForward && !blockReverse {
			isExternalView = true
		}

		// Showing the minimal 'external' view of their profile page? Used when:
		// - The viewer is logged out and the user enables their limited logged-out view.
		// - The current user is previewing their own profile in view=external mode.
		// - The current user blocks the target (they can unblock from the minimal view).
		if isExternalView {
			vars := map[string]any{
				"User":           user,
				"IsExternalView": true,
				"IsBlocked":      blockForward,
				"IsPrivate":      true,
			}
			if err := tmpl.Execute(w, r, vars); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			return
		}

		// Is the site under a Maintenance Mode restriction?
		if middleware.MaintenanceMode(currentUser, w, r) {
			return
		}

		// Inject relationship booleans for profile picture display.
		models.SetUserRelationships(currentUser, []*models.User{user})

		// Admin user (photo moderator) can always see the profile pic - but only on this page.
		// Other avatar displays will show the yellow or pink shy.png if the admin is not friends or not granted.
		if currentUser.HasAdminScope(config.ScopePhotoModerator) {
			user.UserRelationship.IsFriend = true
			user.UserRelationship.IsPrivateGranted = true
		}

		var isSelf = currentUser.ID == user.ID

		// Give a Not Found page if we can not see this user (banned, blocking).
		if err := user.CanBeSeenBy(currentUser); err != nil {
			log.Error("%s can not be seen by viewer %s: %s", user.Username, currentUser.Username, err)
			templates.NotFoundPage(w, r)
			return
		}

		// Are they friends? And/or is this user private?
		var (
			isFriend  = models.FriendStatus(currentUser.ID, user.ID)
			isPrivate = !currentUser.IsAdmin && !isSelf && user.Visibility == models.UserVisibilityPrivate && isFriend != "approved"
		)

		// Get Likes for this profile.
		likeMap := models.MapLikes(currentUser, "users", []uint64{user.ID})

		// Get the summary of WHO liked this picture.
		likeExample, likeRemainder, err := models.WhoLikes(currentUser, "users", user.ID)
		if err != nil {
			log.Error("WhoLikes(user %d): %s", user.ID, err)
		}

		// Get a pretty printed (emoji flag) location from the user's Location Settings.
		var prettyLocation = models.GetUserLocationPrettyEmojiString(user.ID)

		// Profile tab counts.
		tabCounts, err := user.GetProfileTabCounts(currentUser)
		if err != nil {
			session.FlashError(w, r, "Error getting profile tab counts: %s", err)
		}

		vars := map[string]any{
			"ActiveProfileTab":  "profile",
			"User":              user,
			"LikeMap":           likeMap,
			"IsFriend":          isFriend,
			"IsPrivate":         isPrivate,
			"IsBlocking":        blockReverse,
			"IsBlocked":         blockForward,
			"IsFollowing":       models.IsFollowing(currentUser.ID, user.ID),
			"FollowMap":         models.MapFollows(currentUser, []uint64{user.ID}),
			"PrivacySetting":    models.GetPrivacySetting(user.ID),
			"ProfileTheme":      models.GetProfileTheme(user.ID),
			"ProfileTabCount":   tabCounts,
			"MutualFriendCount": models.CountMutualFriends(currentUser, user),
			"OnChat":            worker.GetChatStatistics().IsOnline(user.Username),
			"UserLocation":      prettyLocation,

			// Details on who likes their profile page.
			"LikeExample":   likeExample,
			"LikeRemainder": likeRemainder,
			"LikeTableName": "users",
			"LikeTableID":   user.ID,
		}

		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
