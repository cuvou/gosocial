package account

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/middleware"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/ratelimit"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"golang.org/x/crypto/bcrypt"
)

// Login controller.
func Login() http.HandlerFunc {
	tmpl := templates.Must("account/login.html")
	tmpl2fa := templates.Must("account/two_factor_login.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var next = r.FormValue("next")

		// Posting?
		if r.Method == http.MethodPost {
			var (
				// Collect form fields.
				username = strings.TrimSpace(strings.ToLower(r.PostFormValue("username")))
				password = strings.TrimSpace(r.PostFormValue("password"))
			)

			// Rate limit login attempts by email or username they are trying (whether it exists or not).
			limiter := &ratelimit.Limiter{
				Namespace:  "login",
				ID:         username,
				Limit:      config.LoginRateLimit,
				Window:     config.LoginRateLimitWindow,
				CooldownAt: config.LoginRateLimitCooldownAt,
				Cooldown:   config.LoginRateLimitCooldown,
			}
			var takebackDeferredError bool
			if err := limiter.Ping(); err != nil {
				// Is it a deferred error? Flash it at the end of the request but continue
				// to process this login attempt as normal.
				if ratelimit.IsDeferredError(err) {
					defer func() {
						if takebackDeferredError {
							return
						}
						session.FlashError(w, r, err.Error())
					}()
				} else {
					// Lock-out error, show it now and quit.
					session.FlashError(w, r, err.Error())
					templates.Redirect(w, r.URL.Path)
					return
				}
			}

			// Look up their account.
			user, err := models.FindUsernameOrEmail(username)
			if err != nil {
				// The user wasn't found, but still hash the incoming password to take time:
				// so a mischievous user can't infer whether the username was valid based
				// on the server response time.
				bcrypt.GenerateFromPassword([]byte(password), config.BcryptCost)

				session.FlashError(w, r, "Incorrect username or password.")
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Verify password.
			if err := user.CheckPassword(password); err != nil {
				session.FlashError(w, r, "Incorrect username or password.")
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Is their account banned?
			if user.Status == models.UserStatusBanned {
				session.FlashError(w, r, "Your account has been %s. If you believe this was done in error, please contact support.", user.Status)
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Maintenance mode check.
			if middleware.LoginMaintenance(user, w, r) {
				return
			}

			// Clear their login rate limiter.
			limiter.Clear()

			// Does the user have Two-Factor Auth enabled?
			var (
				tf          = models.Get2FA(user.ID)
				twoFactorOK bool // has successfully entered the code
			)
			if tf.Enabled {
				// Are they submitting the 2FA code?
				var (
					intent = r.PostFormValue("intent")
					code   = strings.ReplaceAll(r.PostFormValue("code"), " ", "")
				)

				// Validate the submitted code.
				if intent == "two-factor" {

					// Rate limit 2FA specific attempts.
					limiter = &ratelimit.Limiter{
						Namespace:  "2FA",
						ID:         username,
						Limit:      config.TwoFactorRateLimit,
						Window:     config.TwoFactorRateLimitWindow,
						CooldownAt: config.TwoFactorRateLimitCooldownAt,
						Cooldown:   config.TwoFactorRateLimitCooldown,
					}
					if err := limiter.Ping(); err != nil {
						// Deferred error at end of request?
						if ratelimit.IsDeferredError(err) {
							defer func() {
								if takebackDeferredError {
									return
								}
								session.FlashError(w, r, err.Error())
							}()
						} else {
							// Lock-out error, show it now and quit.
							session.FlashError(w, r, err.Error())
							templates.Redirect(w, r.URL.Path)
							return
						}
					}

					// Verify the TOTP code.
					if err := tf.Validate(code); err != nil {
						session.FlashError(w, r, "Invalid authentication code; please try again.")
					} else {
						// We're in!
						twoFactorOK = true
						limiter.Clear()
					}
				}

				// Show the 2FA login form.
				if !twoFactorOK {
					var vars = map[string]interface{}{
						"Next":     next,
						"Username": username,
						"Email":    user.Email,
						"Password": password,
					}
					if err := tmpl2fa.Execute(w, r, vars); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
					}
					return
				}
			}

			// OK. Log in the user's session.
			session.LoginUser(w, r, user)
			session.Flash(w, r, "Login successful.")

			// If there was going to be a deferred ratelimit error, take it back.
			takebackDeferredError = true

			// Do they have a Security Checkup item?
			// Redirect them to the 'soft' interstitial page: they can freely navigate away and
			// clicking on the 'Continue to my account' button does NOT put in a 30-day cooldown
			// timer which the 'hard' interstitial would use.
			if user.SecurityCheckupEligible(true) {
				if next == "" {
					next = "/me"
				}
				next = "/settings/security-checkup?next=" + url.QueryEscape(next) + "&login=true"

				// Clear their eligible flag: so if they had it set and click to the Chat/Forum pages,
				// we don't immediately follow the 'soft' interstitial with the 'hard' interstitial.
				user.DeleteProfileField("security_checkup_eligible")
			}

			// Redirect to their dashboard.
			if strings.HasPrefix(next, "/") {
				templates.Redirect(w, next)
			} else {
				templates.Redirect(w, "/me")
			}
			return
		}

		var vars = map[string]interface{}{
			"Next": next,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

// Logout controller.
func Logout() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session.Flash(w, r, "You have been successfully logged out.")
		session.LogoutUser(w, r)
		templates.Redirect(w, "/")
	})
}
