package account

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// TagUser controller to tag somebody in your photo or video.
func TagUser() http.HandlerFunc {
	tmpl := templates.Must("account/tag_user.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			tableName  = r.FormValue("table")
			tableID, _ = strconv.ParseUint(r.FormValue("id"), 10, 64)
			thisURL    = fmt.Sprintf("%s?table=%s&id=%d", r.URL.Path, tableName, tableID)
			nextURL    = fmt.Sprintf("/go/tagged?table=%s&id=%d", tableName, tableID)

			// The item being tagged.
			photo *models.Photo
		)

		// Verify it's a valid table name.
		if _, ok := models.TaggableUserTables[tableName]; !ok {
			session.FlashError(w, r, "That table doesn't accept tagged users.")
			templates.Redirect(w, "/")
			return
		}

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Error getting the current user: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Are they simply asking to be untagged?
		if r.Method == http.MethodPost && r.PostFormValue("intent") == "untag-me" {
			if err := models.UntagUser(currentUser.ID, tableName, tableID); err != nil {
				session.FlashError(w, r, "Error removing the tag: %s", err)
			} else {
				session.Flash(w, r, "You have been untagged from this content.")
			}
			templates.Redirect(w, nextURL)
			return
		}

		// Look up the item being tagged and verify permission.
		switch tableName {
		case "photos":
			if p, err := models.GetPhoto(tableID); err != nil {
				templates.NotFoundPage(w, r)
				return
			} else {
				if !p.CanBeEditedBy(currentUser) {
					templates.ForbiddenPage(w, r)
					return
				}
				photo = p
			}
		default:
			session.FlashError(w, r, "Unexpected table name: %s", tableName)
			templates.Redirect(w, "/")
			return
		}

		// Submitting the form?
		if r.Method == http.MethodPost {
			var intent = r.PostFormValue("intent")

			switch intent {
			case "remove":
				var username = r.PostFormValue("username")
				if user, err := models.FindUsername(username); err != nil {
					session.FlashError(w, r, "Couldn't find that username: %s", err)
				} else {
					if err := models.UntagUser(user.ID, tableName, tableID); err != nil {
						session.FlashError(w, r, "Error untagging that user: %s", err)
					} else {
						session.Flash(w, r, "Untagged @%s from your content.", user.Username)
					}
				}

				templates.Redirect(w, thisURL)
				return
			case "add":
				// Gather the usernames.
				var (
					postedUsernames = strings.Split(r.PostFormValue("users"), ",")
					usernames       = []string{}
					taggedCount     = models.CountTaggedUsers(tableName, tableID)
				)
				for _, name := range postedUsernames {
					name = strings.ToLower(strings.TrimSpace(name))
					if name == "" {
						continue
					}

					// Too many?
					if len(usernames) >= 10 {
						session.FlashError(w, r, "Please only tag up to 10 people at a time. Only the first 10 you entered will be tagged.")
						break
					}

					usernames = append(usernames, name)
				}

				// Find these users.
				users, err := models.GetUsersByUsernames(currentUser, usernames)
				if err != nil {
					session.FlashError(w, r, "Couldn't find usernames: %s", err)
					templates.Redirect(w, thisURL)
					return
				}

				// Do the needful.
				var added int
				for i, user := range users {

					// Will this exceed the max limit?
					if taggedCount+i+1 > config.MaxTaggedUsers {
						session.FlashError(w, r, "Couldn't tag @%s: this item has the maximum limit (%d) of tagged people already.", user.Username, config.MaxTaggedUsers)
						break
					}

					if err := models.TagUser(currentUser.ID, user.ID, tableName, tableID); err != nil {
						session.FlashError(w, r, "Error tagging users: %s", err)
						templates.Redirect(w, thisURL)
						return
					}

					added++
				}

				var noun = "person"
				if added != 1 {
					noun = "people"
				}
				session.Flash(w, r, "Tagged %d %s in your content.", added, noun)
				templates.Redirect(w, nextURL)
				return
			default:
				session.FlashError(w, r, "Unknown intent: %s", intent)
				templates.Redirect(w, thisURL)
				return
			}
		}

		// Get the current tagged user list.
		users, err := models.GetTaggedUsers(currentUser, tableName, tableID)
		if err != nil {
			session.FlashError(w, r, "Error getting the tagged users: %s", err)
		}

		var vars = map[string]any{
			"TableName": tableName,
			"TableID":   tableID,
			"Photo":     photo,
			"Users":     users,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
