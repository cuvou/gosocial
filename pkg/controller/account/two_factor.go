package account

import (
	"net/http"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// 2FA Setup page (/account/two-factor/setup)
func Setup2FA() http.HandlerFunc {
	tmpl := templates.Must("account/two_factor_setup.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Load the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get CurrentUser: %s", err)
			templates.Redirect(w, r.URL.Path)
			return
		}

		// Get their current 2FA settings.
		tf := models.Get2FA(currentUser.ID)

		// If they aren't already set up, prepare a new TOTP secret for first-time setup.
		var key *otp.Key
		if tf.IsNew() {
			// Generate new TOTP parameters.
			if newKey, err := totp.Generate(totp.GenerateOpts{
				Issuer:      config.Title,
				AccountName: currentUser.Username,
			}); err != nil {
				session.FlashError(w, r, "Error generating TOTP: %s", err)
				templates.Redirect(w, "/me")
				return
			} else {
				key = newKey
			}

			// Set the secret.
			tf.SetSecret(key.URL())

			// Save it.
			if err := tf.Save(); err != nil {
				session.FlashError(w, r, "Error saving TOTP settings to the database: %s", err)
				templates.Redirect(w, "/me")
				return
			}
		} else {
			// Reconstruct the stored TOTP key.
			secret, err := tf.GetSecret()
			if err != nil {
				session.FlashError(w, r, "Error retrieving 2FA secret: %s", err)
				templates.Redirect(w, "/me")
				return
			}

			// Reconstruct the OTP key object.
			if k, err := otp.NewKeyFromURL(secret); err != nil {
				session.FlashError(w, r, "Error retrieving TOTP key: %s", err)
				templates.Redirect(w, "/me")
				return
			} else {
				key = k
			}
		}

		// Are they (re)viewing their original QR code?
		var isPairingSecondDevice bool

		// POST form actions.
		if r.Method == http.MethodPost {
			var intent = r.PostFormValue("intent")
			switch intent {
			case "setup-verify":
				// Setup: verify correct enrollment.
				var (
					code  = r.PostFormValue("code")
					valid = totp.Validate(code, key.Secret())
				)

				// Valid?
				if !valid {
					session.FlashError(w, r, "The passcode you submitted didn't seem correct. Try a new six-digit code.")

					// If they were reconfiguring a second device, go back to the re-setup screen.
					if tf.Enabled {
						isPairingSecondDevice = true
						break
					} else {
						templates.Redirect(w, r.URL.Path)
						return
					}
				}

				// OK!
				tf.Enabled = true
				if err := tf.Save(); err != nil {
					session.FlashError(w, r, "Error saving your TOTP settings to the database: %s", err)
				} else {
					session.Flash(w, r, "The authentication code was validated successfully! Two-Factor Authentication is now active for your account.")
				}
			case "regenerate-backup-codes":
				// Re-generate backup codes.
				if err := tf.GenerateBackupCodes(); err != nil {
					session.FlashError(w, r, "Error generating backup codes: %s", err)
				} else {
					// Save the changes.
					if err := tf.Save(); err != nil {
						session.FlashError(w, r, "Error saving your TOTP settings to the database: %s", err)
					} else {
						session.Flash(w, r, "Your backup codes have been regenerated!")
					}
				}
			case "disable":
				// Disable 2FA. User password is required.
				var password = r.PostFormValue("password")
				if err := currentUser.CheckPassword(password); err != nil {
					session.FlashError(w, r, "Couldn't disable 2FA: the password you entered is incorrect.")
				} else {
					// Delete the 2FA configuration.
					if err := tf.Delete(); err != nil {
						session.FlashError(w, r, "Couldn't delete 2FA setting from the database: %s", err)
					} else {
						session.Flash(w, r, "Your 2FA settings have been cleared and disabled.")
					}
				}
			case "resetup":
				// View the original QR code to set up a new device.
				var password = r.PostFormValue("password")
				if err := currentUser.CheckPassword(password); err != nil {
					session.FlashError(w, r, "Couldn't access your 2FA QR code: the password you entered is incorrect.")
				} else {
					session.Flash(w, r, "Password accepted. Your 2FA QR code and setup steps will be displayed below.")
					isPairingSecondDevice = true
				}
			default:
				session.FlashError(w, r, "Unknown intent: %s", intent)
			}

			// All POST requests redirect away except resetup.
			if !isPairingSecondDevice {
				templates.Redirect(w, r.URL.Path)
				return
			}
		}

		// Generate the QR code.
		qrCode, err := tf.QRCodeAsDataURL(key)
		if err != nil {
			log.Error("TwoFactor: Couldn't create QR code: %s", err)
		}

		var vars = map[string]interface{}{
			"TwoFactor":             tf,
			"Key":                   key,
			"QRCode":                qrCode,
			"IsPairingSecondDevice": isPairingSecondDevice,
		}

		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
