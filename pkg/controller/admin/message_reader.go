package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/redis"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// MessageReader controller (/admin/message-reader)
func MessageReader() http.HandlerFunc {
	tmpl := templates.Must("admin/message_reader.html")

	// Whitelist for ordering options.
	var sortWhitelist = []string{
		"created_at desc",
		"created_at asc",
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			intUserID, _ = strconv.Atoi(r.FormValue("user_id"))
			userID       = uint64(intUserID)
		)

		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Error getting the current user: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Look up the target user.
		user, err := models.GetUser(userID)
		if err != nil {
			session.FlashError(w, r, "User not found.")
			templates.Redirect(w, "/")
			return
		}

		// Redis cache key for the Gatekeeper.
		redisKey := fmt.Sprintf("admin/message-reader/%d/%d", currentUser.ID, user.ID)

		// Gatekeeper: Admins can look at their own insights.
		// For other users, gate the permission request.
		if user.ID != currentUser.ID && !redis.Exists(redisKey) {
			var (
				reason = r.PostFormValue("reason")
				vars   = map[string]any{
					"Gatekeeper": true,
					"User":       user,
				}
			)

			// POSTing a reason for the gatekeeper?
			if r.Method == http.MethodPost {
				if reason == "" {
					session.FlashError(w, r, "A reason is required.")
					templates.Redirect(w, "/")
					return
				}

				// Log it as an admin report.
				fb := &models.Feedback{
					Intent:    "report",
					Subject:   "User Messages have been accessed",
					TableName: "users",
					TableID:   user.ID,
					Message: fmt.Sprintf(
						"The admin user **%s** (id:%d) has accessed user messages for **%s** (id:%d)\n\n"+
							"The reason they have given:\n\n%s",
						currentUser.Username, currentUser.ID,
						user.Username, user.ID, reason,
					),
				}
				if err := models.CreateFeedback(fb); err != nil {
					session.FlashError(w, r, "Couldn't create admin notification: %s", err)
				}

				// Put in a temporary access grant.
				redis.Set(redisKey, true, config.AdminReaderCooldown)
				session.Flash(w, r, "Your reason has been logged and you can now access this user's messages.")

				// Redirect the user to their destination, with deeplink support.
				var queryParams = []string{
					fmt.Sprintf("user_id=%d", user.ID),
				}
				for _, field := range []string{
					"view", "sort", "page", "partner_id", "channel_id",
				} {
					value := r.FormValue(field)
					if value != "" {
						queryParams = append(queryParams, fmt.Sprintf("%s=%s", field, value))
					}
				}
				templates.Redirect(w, fmt.Sprintf("%s?%s", r.URL.Path, strings.Join(queryParams, "&")))
				return
			}

			if err := tmpl.Execute(w, r, vars); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}

			return
		}

		/*****************************
		 * Past the Gatekeeper here. *
		 *****************************/

		// Query parameters for main view.
		var (
			// View tab: website vs. chat
			view   = r.FormValue("view")
			sort   = r.FormValue("sort")
			sortOK bool

			// Viewing a main website thread by partner user ID.
			partnerID, _ = strconv.Atoi(r.FormValue("partner_id"))
			partnerUser  *models.User
			messages     []*models.Message

			// Viewing a chat room thread by channel ID.
			channelID = r.FormValue("channel_id")
			dms       []*models.DirectMessage

			// Pager where appropriate.
			pager = &models.Pagination{
				PerPage: config.PageSizeAdminInboxThread,
				Sort:    "created_at desc",
			}
		)
		pager.ParsePage(r)
		if view == "" {
			view = "website"
		}

		// Default sort value varies between main website vs. chat room DMs.
		if sort == "" {
			if view == "website" {
				sort = "created_at desc"
			} else {
				sort = "created_at asc"
			}
		}
		for _, wl := range sortWhitelist {
			if sort == wl {
				sortOK = true
			}
		}
		if !sortOK {
			sort = sortWhitelist[0]
		}
		pager.Sort = sort

		// Get their main website message insights.
		messageInsights, err := models.GetMessageInsights(user)
		if err != nil {
			session.FlashError(w, r, "Error getting main website message insights: %s", err)
		}

		// Count their chat room threads.
		dmInsights, err := models.GetBareRTCMessageInsights(user)
		if err != nil {
			session.FlashError(w, r, "Error getting chat room message insights: %s", err)
		}

		// Are we looking at a specific thread?
		if view == "website" && partnerID > 0 {
			// Look up the partner user.
			if u, err := models.GetUser(uint64(partnerID)); err != nil {
				session.FlashError(w, r, "Couldn't look up their partner user: %s", err)
			} else {
				partnerUser = u

				// Get the paginated thread.
				if m, err := models.GetMessageThread(user.ID, partnerUser.ID, pager); err != nil {
					session.FlashError(w, r, "Error getting message thread: %s", err)
				} else {
					messages = m
				}
			}
		} else if view == "chat" && channelID != "" {

			// The channelID param is the other party's username, look them up.
			if u, err := models.FindUsername(channelID); err != nil {
				session.FlashError(w, r, "Couldn't look up the chat partner by username (%s): %s", channelID, err)
			} else {
				partnerUser = u
			}

			// Generate the BareRTC channel ID.
			cid := models.BareRTCDirectMessageChannelName([]string{
				user.Username,
				partnerUser.Username,
			})

			if m, err := models.PaginateBareRTCDirectMessages(cid, pager); err != nil {
				session.FlashError(w, r, "Error getting message thread: %s", err)
			} else {
				dms = m
			}
		}

		// Create a UserMap for template convenience.
		var userMap = models.UserMap{
			user.ID: user,
		}
		if partnerUser != nil {
			userMap[partnerUser.ID] = partnerUser
		}

		var vars = map[string]interface{}{
			"User":    user,
			"TabView": view,
			"UserMap": userMap,

			// Provides the counts on the tabs and main website DMs list.
			"MessageInsights": messageInsights,
			"ChatInsights":    dmInsights,

			// When inspecting a conversation thread.
			"PartnerUser":    partnerUser,
			"Messages":       messages,
			"DirectMessages": dms,
			"Pager":          pager,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
