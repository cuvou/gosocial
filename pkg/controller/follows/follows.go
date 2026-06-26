package follows

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/utility"
)

// Follow and unfollow endpoint (POST /following/add).
func Follow() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			username = r.PostFormValue("username")
			unfollow = r.PostFormValue("unfollow") == "true"
			nextURL  = fmt.Sprintf("/u/%s", username)
		)

		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get current user: %s", err)
			templates.Redirect(w, nextURL)
			return
		}

		user, err := models.FindUsername(username)
		if err != nil {
			session.FlashError(w, r, "Couldn't find username %s: %s", username, err)
			templates.Redirect(w, "/")
			return
		}

		if currentUser.ID == user.ID {
			session.FlashError(w, r, "You can't follow yourself.")
			templates.Redirect(w, nextURL)
			return
		}

		// Does the target user currently follow us too?
		isFollowingUs := models.IsFollowing(user.ID, currentUser.ID)

		// Do the needful.
		if unfollow {
			if err := models.Unfollow(currentUser.ID, user.ID); err != nil {
				session.FlashError(w, r, "Error unfollowing %s: %s", username, err)
			} else {
				session.Flash(w, r, "You have unfollowed %s and will no longer receive notifications when they post new photos, videos or blogs.", username)

				// Revoke follow notifications.
				models.RemoveSpecificNotification(user.ID, models.NotificationFollow, "users", currentUser.ID)
				models.RemoveSpecificNotification(user.ID, models.NotificationFollowBack, "users", currentUser.ID)
			}
		} else {

			// Does the target allow the follow?
			ps := models.GetPrivacySetting(user.ID)
			if ps.FollowMe == "friends" && !models.AreFriends(currentUser.ID, user.ID) && !isFollowingUs {
				session.FlashError(w, r, "Could not follow %s: they only allow their friends to follow them.", user.Username)
				templates.Redirect(w, nextURL)
				return
			}

			if _, err := models.AddFollow(currentUser.ID, user.ID); err != nil {
				session.FlashError(w, r, "Error following %s: %s", username, err)
			} else {
				session.Flash(w, r, "You are now following %s and will receive notifications when they post new photos, videos or blogs.", username)

				// Notify the target user.
				notif := &models.Notification{
					UserID:      user.ID,
					AboutUserID: &currentUser.ID,
					Type:        models.NotificationFollow,
					TableName:   "users",
					TableID:     currentUser.ID,
					Link:        "/followers",
				}
				if isFollowingUs {
					notif.Type = models.NotificationFollowBack
				}
				if err := models.CreateNotification(notif); err != nil {
					log.Error("Couldn't notify user %s about being followed: %s", user.ID, err)
				}
			}
		}

		templates.Redirect(w, nextURL)
	})
}

// ConfirmUnfollow shows a UI to confirm (deep linked from Notifications page).
func ConfirmUnfollow() http.HandlerFunc {
	tmpl := templates.Must("follows/confirm_unfollow.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			username = r.FormValue("username")
			nextURL  = r.FormValue("next")
		)

		if !strings.HasPrefix(nextURL, "/") {
			nextURL = "/me"
		}

		user, err := models.FindUsername(username)
		if err != nil {
			templates.NotFoundPage(w, r)
			return
		}

		var vars = map[string]any{
			"User":    user,
			"NextURL": nextURL,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

// Edit a follower, to remove them or to follow them back.
func EditFollower() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			username = r.PostFormValue("username")
			verdict  = r.PostFormValue("verdict")
			nextURL  = "/followers"
		)

		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get current user: %s", err)
			templates.Redirect(w, nextURL)
			return
		}

		user, err := models.FindUsername(username)
		if err != nil {
			session.FlashError(w, r, "Couldn't find username %s: %s", username, err)
			templates.Redirect(w, "/")
			return
		}

		// Do the needful.
		switch verdict {
		case "follow-back":

			// Sanity check that they follow us first.
			if !models.IsFollowing(user.ID, currentUser.ID) {
				session.FlashError(w, r, "Couldn't follow %s back: they are not following you right now.", user.Username)
				templates.Redirect(w, nextURL)
				return
			}

			if _, err := models.AddFollow(currentUser.ID, user.ID); err != nil {
				session.FlashError(w, r, "Error following %s: %s", username, err)
			} else {
				session.Flash(w, r, "You are now following %s and will receive notifications when they post new photos, videos or blogs.", username)

				// Notify the target user.
				notif := &models.Notification{
					UserID:      user.ID,
					AboutUserID: &currentUser.ID,
					Type:        models.NotificationFollowBack,
					TableName:   "users",
					TableID:     currentUser.ID,
					Link:        "/followers",
				}
				if err := models.CreateNotification(notif); err != nil {
					log.Error("Couldn't notify user %s about being followed: %s", user.ID, err)
				}
			}
		case "remove":
			if err := models.Unfollow(user.ID, currentUser.ID); err != nil {
				session.FlashError(w, r, "Error removing follower %s: %s", username, err)
			} else {
				session.Flash(w, r, "%s is no longer following you.", username)

				// Revoke follow notifications.
				models.RemoveSpecificNotification(user.ID, models.NotificationFollow, "users", currentUser.ID)
				models.RemoveSpecificNotification(user.ID, models.NotificationFollowBack, "users", currentUser.ID)
			}
		default:
			session.FlashError(w, r, "Unexpected action: %s", verdict)
		}

		templates.Redirect(w, nextURL)
	})
}

// Batch Edit (remove) followers.
func BatchRemove() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			isFollowing = r.PostFormValue("is_following") == "true"
			usernames   = strings.Split(r.PostFormValue("usernames"), ",")
			nextURL     = "/followers"
		)

		if isFollowing {
			nextURL = "/following"
		}

		if len(usernames) == 0 || (len(usernames) == 1 && usernames[0] == "") {
			session.FlashError(w, r, "No usernames were selected for bulk follow removal.")
			templates.Redirect(w, nextURL)
			return
		}

		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get current user: %s", err)
			templates.Redirect(w, nextURL)
			return
		}

		userMap, err := models.MapUsersByUsername(usernames)
		if err != nil {
			session.FlashError(w, r, "Error finding these users: %s", err)
			templates.Redirect(w, nextURL)
			return
		}

		// Do the needful.
		for username, user := range userMap {
			if isFollowing {
				if err := models.Unfollow(currentUser.ID, user.ID); err != nil {
					session.FlashError(w, r, "Error unfollowing user %s: %s", username, err)
				} else {
					// Revoke notifications.
					models.RemoveSpecificNotification(user.ID, models.NotificationFollow, "users", currentUser.ID)
					models.RemoveSpecificNotification(user.ID, models.NotificationFollowBack, "users", currentUser.ID)
				}
			} else {
				if err := models.Unfollow(user.ID, currentUser.ID); err != nil {
					session.FlashError(w, r, "Error removing follower %s: %s", username, err)
				} else {
					// Revoke notifications.
					models.RemoveSpecificNotification(currentUser.ID, models.NotificationFollow, "users", user.ID)
					models.RemoveSpecificNotification(currentUser.ID, models.NotificationFollowBack, "users", user.ID)
				}
			}
		}

		if isFollowing {
			session.Flash(w, r, "Unfollowed %d profile(s).", len(userMap))
		} else {
			session.Flash(w, r, "Removed %d follower(s).", len(userMap))
		}
		templates.Redirect(w, nextURL)
	})
}

// Follower list.
func Followers() http.HandlerFunc {
	return FollowList(false)
}

// Following list.
func Following() http.HandlerFunc {
	return FollowList(true)
}

// FollowList is the common UI handler for both Followers and Following.
func FollowList(isFollowing bool) http.HandlerFunc {
	tmpl := templates.Must("follows/followers.html")

	// Whitelist for ordering your friend list here.
	var sortWhitelist = []string{
		"follows.updated_at desc",
		"follows.updated_at asc",
		"users.username asc",
		"users.username desc",
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			sort           = utility.StringIn(r.FormValue("sort"), sortWhitelist, sortWhitelist[0])
			excludeFriends = r.FormValue("friends") == "false"
		)

		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Unexpected error: could not get currentUser.")
			templates.Redirect(w, "/")
			return
		}

		// Get our friends.
		pager := &models.Pagination{
			PerPage: config.PageSizeFollowers,
			Sort:    sort,
		}
		pager.ParsePage(r)

		followers, err := models.PaginateFollowers(currentUser, isFollowing, excludeFriends, pager)
		if err != nil {
			session.FlashError(w, r, "Couldn't paginate friends: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Inject relationship booleans.
		models.SetUserRelationships(currentUser, followers)

		// Counts for the tabs.
		countFollowers, countFollowing, err := models.CountFollows(currentUser)
		if err != nil {
			session.FlashError(w, r, "Couldn't count your followers: %s", err)
		}

		// Map our follower status with these users.
		var (
			userIDs []uint64
		)
		for _, user := range followers {
			userIDs = append(userIDs, user.ID)
		}

		var vars = map[string]any{
			"IsFollowing":    isFollowing,
			"ExcludeFriends": excludeFriends,
			"FollowerCount":  countFollowers,
			"FollowingCount": countFollowing,
			"FriendMap":      models.MapFriends(currentUser, followers),
			"FollowMap":      models.MapFollows(currentUser, userIDs),
			"Users":          followers,
			"Pager":          pager,
			"Sort":           sort,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
