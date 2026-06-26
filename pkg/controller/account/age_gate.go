package account

import (
	"fmt"
	"net/http"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/utility"
)

// User age gate page to collect birthdates retroactively (/settings/age-gate)
func AgeGate() http.HandlerFunc {
	tmpl := templates.Must("account/age_gate.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := map[string]interface{}{
			"Enum": config.ProfileEnums,
		}

		// Load the current user in case of updates.
		user, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get CurrentUser: %s", err)
			templates.Redirect(w, r.URL.Path)
			return
		}

		// If we have already set our age, don't allow changing it.
		if !user.Birthdate.IsZero() {
			templates.NotFoundPage(w, r)
			return
		}

		// Are we POSTing?
		if r.Method == http.MethodPost {
			var (
				dob     = r.PostFormValue("dob")
				hideAge = r.PostFormValue("hide_age")
			)

			birthdate, err := time.Parse("2006-01-02", dob)
			if err != nil {
				session.FlashError(w, r, "Incorrect format for birthdate; should be in yyyy-mm-dd format but got: %s", dob)
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Validate birthdate is at least age 18.
			if utility.Age(birthdate) <= 5 {
				// Probably an error: seen some users enter current year by mistake, don't instantly ban them.
				session.FlashError(w, r, "Please enter a valid birthdate. The year you entered (%d) was probably incorrect.", birthdate.Year())
				templates.Redirect(w, r.URL.Path)
				return
			} else if utility.Age(birthdate) < 18 {
				// Lock their account and notify the admins.
				fb := &models.Feedback{
					Intent:    "report",
					Subject:   "Age Gate has auto-banned a user account",
					TableName: "users",
					TableID:   user.ID,
					Message: fmt.Sprintf(
						"The user **%s** (id:%d) has seen the Age Gate page and entered their birthdate which was under 18 years old (their entry: %s, %d years old), and their account has been banned automatically.",
						user.Username, user.ID,
						birthdate.Format("2006-01-02"), utility.Age(birthdate),
					),
				}

				if err := models.CreateFeedback(fb); err != nil {
					session.FlashError(w, r, "Couldn't create admin notification: %s", err)
				}

				session.FlashError(w, r,
					"You must be 18 years old to use this site and you have entered a birthdate that looks to be %d. "+
						"If this was done by mistake, please contact support to resolve this issue. In the meantime, your "+
						"account will be locked and you have been logged out.",
					utility.Age(birthdate),
				)

				// Ban the account now.
				user.Status = models.UserStatusBanned
				if err := user.Save(); err != nil {
					session.FlashError(w, r, "Couldn't save update to your user account!")
				}

				session.LogoutUser(w, r)
				templates.Redirect(w, "/")
				return
			}

			user.Birthdate = birthdate

			if err := user.Save(); err != nil {
				session.FlashError(w, r, "Failed to save user to database: %s", err)
			}

			user.SetProfileField("hide_age", hideAge)

			session.Flash(w, r, "Thank you for entering your birthdate!")

			templates.Redirect(w, "/me")
			return
		}

		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
