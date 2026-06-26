package settings

import (
	"net/http"

	"github.com/cuvou/gosocial/pkg/chat"
	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/utility"
)

// Privacy settings (/settings/privacy).
func Privacy() http.HandlerFunc {
	tmpl := templates.Must("settings/privacy.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Load the current user in case of updates.
		user, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get CurrentUser: %s", err)
			templates.Redirect(w, r.URL.Path)
			return
		}

		// Are we POSTing?
		if r.Method == http.MethodPost {

			var (
				visibility      = models.UserVisibility(r.PostFormValue("visibility"))
				dmPrivacy       = r.PostFormValue("dm_privacy")
				followMe        = r.PostFormValue("follow_me")
				commentPrivacy  = r.PostFormValue("photo_comment_permission")
				ppPrivacy       = r.PostFormValue("private_photo_gate")
				friendsPrivacy  = r.PostFormValue("friends_privacy")
				hideAge         = r.PostFormValue("hidden_age")
				hideGender      = r.PostFormValue("hidden_gender")
				hideOrientation = r.PostFormValue("hidden_orientation")
			)

			// Set account visibility overall.
			user.Visibility = models.UserVisibilityPublic
			for _, cmp := range models.UserVisibilityOptions {
				if visibility == cmp {
					user.Visibility = visibility
				}
			}

			if err := user.Save(); err != nil {
				session.FlashError(w, r, "Failed to save user to database: %s", err)
			}

			// Set other privacy settings.
			ps := models.GetPrivacySetting(user.ID)
			ps.FirstMessages = utility.StringIn(dmPrivacy, config.PrivacySettingFirstMessages, "")
			ps.FollowMe = utility.StringIn(followMe, config.PrivacySettingFollowMe, "")
			ps.PhotoComments = utility.StringIn(commentPrivacy, config.PrivacySettingPhotoComments, "")
			ps.PrivatePhotos = utility.StringIn(ppPrivacy, config.PrivacySettingPrivatePhotos, "")
			ps.FriendsList = utility.StringIn(friendsPrivacy, config.PrivacySettingFriendsList, "")
			ps.HiddenAge = hideAge == "true"
			ps.HiddenGender = hideGender == "true"
			ps.HiddenOrientation = hideOrientation == "true"
			if err := ps.Save(); err != nil {
				session.FlashError(w, r, "Failed to save privacy settings: %s", err)
			}

			session.Flash(w, r, "Privacy settings updated!")

			// If the user is currently on chat, push their updated JWT token
			// so we refresh their profile picture and/or Shy Account status.
			go func() {
				if err := chat.AmendJWTToken(r, user.ID); err != nil {
					log.Error("AmendJWTToken: Couldn't send amended JWT token for %s to chat room: %w", user.Username, err)
				}
			}()

			templates.Redirect(w, r.URL.Path)
			return
		}

		vars := map[string]any{
			"PrivacySetting": models.GetPrivacySetting(user.ID),
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
