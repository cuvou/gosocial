package settings

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/chat"
	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/spam"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/utility"
)

// Profile settings page (/settings/profile).
func Profile() http.HandlerFunc {
	tmpl := templates.Must("settings/profile.html")
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

			// Setting profile values.
			var (
				displayName = r.PostFormValue("display_name")
				dob         = r.PostFormValue("dob")
			)

			// Set user attributes.
			user.Name = &displayName

			// Birthdate, now required.
			if birthdate, err := time.Parse("2006-01-02", dob); err != nil {
				session.FlashError(w, r, "Incorrect format for birthdate; should be in yyyy-mm-dd format but got: %s", dob)
			} else {
				// Validate birthdate is at least age 18.
				if utility.Age(birthdate) < 18 {
					session.FlashError(w, r, "Invalid birthdate: you must be at least 18 years old to use this site.")
					templates.Redirect(w, r.URL.Path)
					return
				}

				// If the user changes their birthdate, notify the admin.
				if !user.Birthdate.IsZero() && user.Birthdate.Format("2006-01-02") != dob {
					// Create an admin Feedback model.
					fb := &models.Feedback{
						Intent:    "report",
						Subject:   "report.dob",
						UserID:    user.ID,
						TableName: "users",
						TableID:   user.ID,
						Message: fmt.Sprintf(
							"A user has modified their birthdate on their profile page!\n\n"+
								"* Original: %s (age %d)\n* Updated: %s (age %d)",
							user.Birthdate, utility.Age(user.Birthdate),
							birthdate, utility.Age(birthdate),
						),
					}

					// Save the feedback.
					if err := models.CreateFeedback(fb); err != nil {
						log.Error("Couldn't save feedback from user updating their DOB: %s", err)
					}
				}

				// Work around DST issues: set the hour to noon.
				user.Birthdate = birthdate.Add(12 * time.Hour)
			}

			// Set profile attributes (free text fields).
			for _, attr := range config.ProfileFields {
				var value = strings.TrimSpace(r.PostFormValue(attr))

				// Look for spammy links to restricted video sites or things.
				if err := spam.DetectSpamLinks(value); err != nil {
					session.FlashError(w, r, "On field '%s': %s", attr, err.Error())
					continue
				}

				user.SetProfileField(attr, value)
			}

			// Set profile attributes (constrained enum fields).
			for attr, allowed := range config.EnumProfileFields {
				var value = strings.TrimSpace(r.PostFormValue(attr))
				value = utility.StringIn(value, allowed, "")
				user.SetProfileField(attr, value)
			}

			// Set long (essay) profile fields.
			for _, attr := range config.EssayProfileFields {
				var value = strings.TrimSpace(r.PostFormValue(attr))

				// Look for spammy links to restricted video sites or things.
				if err := spam.DetectSpamLinks(value); err != nil {
					session.FlashError(w, r, "On field '%s': %s", attr, err.Error())
					continue
				}

				user.SetLongProfileField(attr, value)
			}

			// "Looking For" checkbox list.
			hereFor := r.PostForm["here_for"]
			user.SetProfileField("here_for", strings.Join(hereFor, ","))

			// "Spoken Languages" checkbox list.
			languages := r.PostForm["spoken_languages"]
			user.SetProfileField("spoken_languages", strings.Join(languages, ","))

			if err := user.Save(); err != nil {
				session.FlashError(w, r, "Failed to save user to database: %s", err)
			}

			session.Flash(w, r, "Profile settings updated!")
			templates.Redirect(w, r.URL.Path)

			// If the user is currently on chat, push their updated Display Name.
			go func() {
				if err := chat.AmendJWTToken(r, user.ID); err != nil {
					log.Error("AmendJWTToken: Couldn't send amended JWT token for %s to chat room: %w", user.Username, err)
				}
			}()
			return
		}

		vars := map[string]interface{}{
			"Enum":        config.ProfileEnums,
			"HereForEnum": config.HereFor,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
