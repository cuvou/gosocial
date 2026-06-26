package block

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/controller/chat"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/ratelimit"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Blocked list.
func Blocked() http.HandlerFunc {
	tmpl := templates.Must("account/block_list.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var search = r.FormValue("search")

		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Unexpected error: could not get currentUser.")
			templates.Redirect(w, "/")
			return
		}

		// Get our blocklist.
		pager := &models.Pagination{
			PerPage: config.PageSizeBlockList,
			Sort:    "updated_at desc",
		}
		pager.ParsePage(r)
		blocked, err := models.PaginateBlockList(currentUser, search, pager)
		if err != nil {
			session.FlashError(w, r, "Couldn't paginate block list: %s", err)
			templates.Redirect(w, "/")
			return
		}

		var vars = map[string]interface{}{
			"BlockedUsers": blocked,
			"SearchString": search,
			"Pager":        pager,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

// AddUser to manually add someone to your block list.
func AddUser() http.HandlerFunc {
	tmpl := templates.Must("account/block_list_add.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := tmpl.Execute(w, r, nil); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

// BlockUser controller.
func BlockUser() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// POST only.
		if r.Method != http.MethodPost {
			session.FlashError(w, r, "Unacceptable Request Method")
			templates.Redirect(w, "/")
			return
		}

		// Form fields
		var (
			username    = strings.ToLower(r.PostFormValue("username"))
			unblock     = r.PostFormValue("unblock") == "true"
			gotoProfile = r.PostFormValue("goto-profile") == "true"
			nextURL     = "/users/blocked"
		)
		if gotoProfile {
			nextURL = "/u/" + username
		}

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get CurrentUser: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Get the target user.
		user, err := models.FindUsername(username)
		if err != nil {
			session.FlashError(w, r, "User Not Found")
			templates.Redirect(w, "/users/blocklist/add")
			return
		}

		// Unblocking?
		if unblock {
			if err := models.UnblockUser(currentUser.ID, user.ID); err != nil {
				session.FlashError(w, r, "Couldn't unblock this user: %s.", err)
			} else {
				session.Flash(w, r, "You have removed %s from your block list.", user.Username)

				// Log the change.
				models.LogDeleted(currentUser, nil, "blocks", user.ID, "Unblocked user "+user.Username+".", nil)
			}
			templates.Redirect(w, nextURL)
			return
		}

		// Can't block yourself.
		if currentUser.ID == user.ID {
			session.FlashError(w, r, "You can't block yourself!")
			templates.Redirect(w, "/u/"+username)
			return
		}

		// If the target user is an admin, log this to the admin reports page.
		if user.IsAdmin {
			// Is the target admin user unblockable?
			var (
				unblockable = user.HasAdminScope(config.ScopeUnblockable)
				footer      string // qualifier for the admin report body
			)

			// Add a footer to the report to indicate whether the block goes through.
			if unblockable {
				footer = "**Unblockable:** this admin can not be blocked, so the block was not added and the user was shown an error message."
			} else {
				footer = "**Notice:** This admin is not unblockable, so the block has been added successfully."
			}

			// Also, include this user's current count of blocked admin users.
			count, total := models.CountBlockedAdminUsers(currentUser)
			footer += fmt.Sprintf("\n\nThis user now blocks %d of %d admin user(s) on this site.", count+1, total)

			// For curiosity's sake, log a report.
			fb := &models.Feedback{
				Intent:  "report",
				Subject: "A user tried to block an admin",
				Message: fmt.Sprintf(
					"A user has tried to block an admin user account!\n\n"+
						"* Username: %s\n* Tried to block: %s\n\n%s",
					currentUser.Username,
					user.Username,
					footer,
				),
				UserID:    currentUser.ID,
				TableName: "users",
				TableID:   currentUser.ID,
			}
			if err := models.CreateFeedback(fb); err != nil {
				log.Error("Could not log feedback for user %s trying to block admin %s: %s", currentUser.Username, user.Username, err)
			}

			// If the admin is unblockable, give the user an error message and return.
			if unblockable {
				session.FlashError(w, r, "You can not block site administrators.")
				templates.Redirect(w, nextURL)
				return
			}
		}

		// Apply a rate limit throttle to discourage "block the whole website!" people.
		// If the target user has a non-public picture, waive the rate limit.
		limiter := &ratelimit.Limiter{
			Namespace:  "add-blocklist",
			ID:         currentUser.ID,
			Limit:      config.AddBlocklistRateLimit,
			Window:     config.AddBlocklistRateLimitWindow,
			CooldownAt: config.AddBlocklistRateLimitCooldownAt,
			Cooldown:   config.AddBlocklistRateLimitCooldown,
		}
		if err := limiter.Ping(); err != nil {

			// Block the user with the error if they are currently locked out.
			if !ratelimit.IsDeferredError(err) {
				// Lock-out error for the full window.
				session.FlashError(w, r, "You have been blocking people at an unusual pace. Had you even interacted with @%s before?", user.Username)
				session.FlashError(w, r, err.Error())
				templates.Redirect(w, nextURL)
				return
			}
		}

		// Block the target user.
		if err := models.AddBlock(currentUser.ID, user.ID); err != nil {
			session.FlashError(w, r, "Couldn't block this user: %s.", err)
		} else {
			session.Flash(w, r, "You have added %s to your block list.", user.Username)

			// Log the change.
			models.LogCreated(currentUser, "blocks", user.ID, "Blocks user "+user.Username+".")
		}

		// Sync the block to the BareRTC chat server now, in case either user is currently online.
		go chat.BlockUserNow(currentUser, user)

		templates.Redirect(w, nextURL)
	})
}
