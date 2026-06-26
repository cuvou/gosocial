package account

import (
	"fmt"
	"net/http"
	nm "net/mail"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/mail"
	"github.com/cuvou/gosocial/pkg/middleware"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/models/demographic"
	"github.com/cuvou/gosocial/pkg/redis"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/spam"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/utility"
	"github.com/google/uuid"
)

// SignupToken goes in Redis when the user first gives us their email address. They
// verify their email before signing up, so cache only in Redis until verified.
type SignupToken struct {
	Email string
	Token string
}

// Delete a SignupToken when it's been used up.
func (st SignupToken) Delete() error {
	return redis.Delete(fmt.Sprintf(config.SignupTokenRedisKey, st.Token))
}

// Initial signup controller.
func Signup() http.HandlerFunc {
	tmpl := templates.Must("account/signup.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Maintenance mode?
		if middleware.SignupMaintenance(w, r) {
			return
		}

		// Template vars.
		var vars = map[string]any{
			"SignupToken":           "",    // non-empty if user has clicked verification link
			"SkipEmailVerification": false, // true if email verification is disabled
			"Email":                 "",    // pre-filled user email
		}

		// Is email verification disabled?
		// If email is not configured either, then skip verification.
		var skipEmailVerification bool
		if config.SkipEmailVerification || !config.Current.Mail.Enabled {
			skipEmailVerification = true
			vars["SkipEmailVerification"] = true
		}

		// Are we called with an email verification token?
		var tokenStr = r.URL.Query().Get("token")
		if r.Method == http.MethodPost {
			tokenStr = r.PostFormValue("token")
		}

		var token SignupToken
		if tokenStr != "" {
			// Validate it.
			if err := redis.Get(fmt.Sprintf(config.SignupTokenRedisKey, tokenStr), &token); err != nil || token.Token != tokenStr {
				session.FlashError(w, r, "Invalid email verification token. Please try signing up again.")
				templates.Redirect(w, r.URL.Path)
				return
			}

			vars["SignupToken"] = tokenStr
			vars["Email"] = token.Email
		}

		// Posting?
		if r.Method == http.MethodPost {
			var (
				// Collect form fields.
				email   = strings.TrimSpace(strings.ToLower(r.PostFormValue("email")))
				confirm = r.PostFormValue("confirm") == "true"

				// Only on full signup form
				username  = strings.TrimSpace(strings.ToLower(r.PostFormValue("username")))
				password  = strings.TrimSpace(r.PostFormValue("password"))
				password2 = strings.TrimSpace(r.PostFormValue("password2"))
				dob       = r.PostFormValue("dob")

				// CAPTCHA response.
				turnstileCAPTCHA = r.PostFormValue("cf-turnstile-response")

				// Honeytrap fields for lazy spam bots.
				honeytrap1 = r.PostFormValue("phone") == ""
				honeytrap2 = r.PostFormValue("referral") == "Word of mouth"

				// Validation errors but still show the form again.
				hasError bool
			)

			// Honeytrap fields check.
			if !honeytrap1 || !honeytrap2 {
				session.Flash(w, r, "We have sent an e-mail to %s with a link to continue signing up your account. Please go and check your e-mail.", email)
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Validate the CAPTCHA token.
			// Note: only needed on initial signup (to validate email address), when the user returns with a tokenized
			// signup link from their email we don't need to check the CAPTCHA a second time.
			if config.Current.Turnstile.Enabled && tokenStr == "" {
				if err := spam.ValidateTurnstileCAPTCHA(turnstileCAPTCHA, "signup"); err != nil {
					session.FlashError(w, r, "There was an error validating your CAPTCHA response.")
					templates.Redirect(w, r.URL.Path)
					return
				}
			}

			// Don't let them sneakily change their verified email address on us.
			if vars["SignupToken"] != "" && email != vars["Email"] {
				session.FlashError(w, r, "This email address is not verified. Please start over from the beginning.")
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Cache username in case of passwd validation errors.
			vars["Email"] = email
			vars["Username"] = username

			// Validate the email.
			if _, err := nm.ParseAddress(email); err != nil {
				session.FlashError(w, r, "The email address you entered is not valid: %s", err)
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Didn't confirm?
			if !confirm {
				session.FlashError(w, r, "Confirm that you have read the rules.")
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Already an account?
			if user, err := models.FindUsernameOrEmail(email); err == nil {
				// We don't want to admit that the email already is registered, so send an email to the
				// address in case the user legitimately forgot, but flash the regular success message.
				if user.IsBanned() {
					log.Error("Do not send signup e-mail to %s: user is banned", email)
				} else {
					if err := mail.LockSending("signup", email, config.EmailDebounceDefault); err == nil {
						err := mail.Send(mail.Message{
							To:       email,
							Subject:  "You already have a gosocial account",
							Template: "email/already_signed_up.html",
							Data: map[string]interface{}{
								"Title": config.Title,
								"URL":   config.Current.BaseURL + "/forgot-password",
							},
						})
						if err != nil {
							session.FlashError(w, r, "Error sending an email: %s", err)
						}

					} else {
						log.Error("LockSending: signup e-mail is not sent to %s: one was sent recently", email)
					}
				}

				session.Flash(w, r, "We have sent an e-mail to %s with a link to continue signing up your account. Please go and check your e-mail.", email)
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Email verification step!
			if !skipEmailVerification && vars["SignupToken"] == "" {
				// Create a SignupToken verification link to send to their inbox.
				token = SignupToken{
					Email: email,
					Token: uuid.New().String(),
				}
				if err := redis.Set(fmt.Sprintf(config.SignupTokenRedisKey, token.Token), token, config.SignupTokenExpires); err != nil {
					session.FlashError(w, r, "Error creating a link to send you: %s", err)
				}

				// Is the app not configured to send email?
				if !config.Current.Mail.Enabled && !config.SkipEmailVerification {
					// Log the signup token for local dev.
					log.Error("Signup: the app is not configured to send email. To continue, visit the URL: /signup?token=%s", token.Token)
					session.FlashError(w, r, "This app is not configured to send email so you can not sign up at this time. "+
						"Please contact the website administrator about this issue!")
					templates.Redirect(w, r.URL.Path)
					return
				}

				if err := mail.LockSending("signup", email, config.SignupTokenExpires); err == nil {
					err := mail.Send(mail.Message{
						To:       email,
						Subject:  "Verify your e-mail address",
						Template: "email/verify_email.html",
						Data: map[string]interface{}{
							"Title": config.Title,
							"URL":   config.Current.BaseURL + "/signup?token=" + token.Token,
						},
					})
					if err != nil {
						session.FlashError(w, r, "Error sending an email: %s", err)
					}
				} else {
					log.Error("LockSending: signup e-mail is not sent to %s: one was sent recently", email)
				}

				session.Flash(w, r, "We have sent an e-mail to %s with a link to continue signing up your account. Please go and check your e-mail.", email)

				// Reminder to check their spam folder too (Gmail users)
				session.Flash(w, r, "If you don't see the confirmation e-mail, check in case it went to your spam folder.")

				templates.Redirect(w, r.URL.Path)
				return
			}

			// DOB check.
			birthdate, err := time.Parse("2006-01-02", dob)
			if err != nil {
				session.FlashError(w, r, "Incorrect format for birthdate; should be in yyyy-mm-dd format but got: %s", dob)
				templates.Redirect(w, r.URL.Path)
				return
			} else {
				// Validate birthdate is at least age 18.
				if utility.Age(birthdate) < 18 {
					session.FlashError(w, r, "You must be at least 18 years old to use this site.")
					templates.Redirect(w, "/")

					// Burn the signup token.
					if token.Token != "" {
						if err := token.Delete(); err != nil {
							log.Error("SignupToken.Delete(%s): %s", token.Token, err)
						}
					}

					return
				}
			}

			// Full sign-up step (w/ email verification token), validate more things.
			if len(password) < 3 {
				session.FlashError(w, r, "Please enter a password longer than 3 characters.")
				hasError = true
			} else if password != password2 {
				session.FlashError(w, r, "Your passwords do not match.")
				hasError = true
			}

			// Validate the username is OK: well formatted, not reserved, not existing.
			if err := models.IsValidUsername(username); err != nil {
				session.FlashError(w, r, err.Error())
				hasError = true
			}

			// Looking good?
			if !hasError {
				user, err := models.CreateUser(username, email, password)
				if err != nil {
					session.FlashError(w, r, err.Error())
				} else {
					session.Flash(w, r, "User account created. Now logged in as %s.", user.Username)

					// Burn the signup token.
					if token.Token != "" {
						if err := token.Delete(); err != nil {
							log.Error("SignupToken.Delete(%s): %s", token.Token, err)
						}
					}

					// Put their birthdate in.
					user.Birthdate = birthdate
					user.Save()

					// Log in the user and send them to their dashboard.
					session.LoginUser(w, r, user)
					templates.Redirect(w, "/me")
				}
			}
		}

		// Especially for the main signup page: get the member demographics so we can warn people
		// that the memberbase is largely made up of men.
		if demo, err := demographic.Get(); err == nil {
			var (
				men   demographic.MemberDemographic
				women demographic.MemberDemographic
			)
			for _, row := range demo.People.IterGenders() {
				switch row.Label {
				case "Man":
					men = row
				case "Woman":
					women = row
				}
			}
			vars["DemographicMenCount"] = men
			vars["DemographicWomenCount"] = women
		}

		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
