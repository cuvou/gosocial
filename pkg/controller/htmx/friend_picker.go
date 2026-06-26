package htmx

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/utility"
	"github.com/google/uuid"
)

// FriendPicker for searching/browsing for friends to share with.
func FriendPicker() http.HandlerFunc {
	tmpl := templates.MustLoadCustom("partials/htmx/friend_picker.html")

	var sortWhitelist = []string{
		"updated_at desc",
		"updated_at asc",
		"users.username asc",
		"users.username desc",
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// HTMX widget parameters.
		var (
			context = r.FormValue("context") // private_photos, event_invite, etc.
			sort    = utility.StringIn(r.FormValue("sort"), sortWhitelist, sortWhitelist[0])

			// For simple use cases where the usernames link to a GET endpoint,
			// e.g. `/photo/private/share?to=%s` for private photos.
			linkTo string

			// For use in POST-able forms.
			inputID    string // <input> element to fill on click
			formID     string // <form> ID
			submitForm bool   // fill and submit on click
			confirm    string // Confirmation message on click
			multiple   bool   // Multiple selection OK.
		)

		// Validation and sanity checks.
		switch context {
		case "private_photos":
			linkTo = "/photo/private/share?to=%s"
		case "event_invite":
			inputID = "invite_username"
			formID = "invite_form"
			submitForm = true
			confirm = "Invite ${username} to your event?"
		case "tagged_user":
			inputID = "users"
			multiple = true
		default:
			w.Write([]byte("Unexpected context for friend picker."))
			return
		}

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			w.Write([]byte("You must be logged in to view this page."))
			return
		}

		// Paginate the user's friends list.
		var pager = &models.Pagination{
			PerPage: config.PageSizePrivateShareFriends,
			Sort:    sort,
		}
		pager.ParsePage(r)

		friends, err := models.PaginateFriends(currentUser, false, false, false, pager)
		if err != nil {
			fmt.Fprintf(w, "Couldn't get your friends list: %s", err)
			return
		}

		// Map private photo grants.
		var granteeMap models.PrivateGrantedMap
		if context == "private_photos" {
			granteeMap = models.MapPrivatePhotoGranted(currentUser, friends)
		}

		// Are we submitting a form on click of a username?
		var onclick string
		if inputID != "" {
			// Preamble: put their username in a variable, the front-end resolves the %s placeholder once.
			preamble := fmt.Sprintf(`const username = '%%s';`)

			// Set the selected name into your text box.
			if multiple {
				onclick = fmt.Sprintf(`const el = document.getElementById('%s'); el.value = el.value ? el.value+', '+username : username;`, inputID)
			} else {
				onclick = fmt.Sprintf(`document.getElementById('%s').value = username;`, inputID)
			}

			// Submit the form on click?
			if formID != "" && submitForm {
				onclick += fmt.Sprintf(`document.getElementById('%s').submit();`, formID)
			}

			// Wrapped in a confirmation box?
			if confirm != "" {
				onclick = fmt.Sprintf(
					"modalConfirm({message: `%s`}).then(() => {%s})",
					confirm,
					onclick,
				)
			}

			onclick = fmt.Sprintf(
				"%s%s; return false",
				preamble, onclick,
			)
		}

		var (
			buf  = bytes.NewBuffer([]byte{})
			vars = map[string]any{
				"Friends":         friends,
				"GranteeMap":      granteeMap,
				"Sort":            sort,
				"Pager":           pager,
				"UsernameLinkTo":  linkTo,
				"UsernameOnClick": onclick,

				// A unique ID for the HTML component, to allow self-reloads with the Sort field.
				"UniqueID": uuid.New().String(),
			}
		)
		if err := tmpl.Execute(buf, r, vars); err != nil {
			fmt.Fprintf(w, "[template error: %s]", err)
			return
		}

		w.Write(buf.Bytes())
	})
}
