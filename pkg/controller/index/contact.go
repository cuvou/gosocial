package index

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/markdown"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/ratelimit"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/spam"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/utility"
)

// Contact or report a problem.
func Contact() http.HandlerFunc {
	tmpl := templates.Must("contact.html")

	// Optional fields for advanced reports (e.g. specific questions asked when reporting Events).
	type OptionalField struct {
		Key  string
		Name string
	}
	var OptionalFields = []OptionalField{
		{"reason", "Reason for Report"},
		{"event_name", "Event Name"},
		{"event_organizer", "Event Organizer"},
		{"preferred_outcome", "Preferred Outcome"},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Query and form POST parameters.
		var (
			intent          = r.FormValue("intent")
			subject         = r.FormValue("subject")
			title           = "Contact Us"
			message         = r.FormValue("message")
			footer          string // appends to the message only when posting the feedback
			replyTo         = r.FormValue("email")
			username        = r.FormValue("username")
			trap1           = r.FormValue("url") != "https://"
			trap2           = r.FormValue("comment") != ""
			tableID         uint64
			tableName       string
			tableLabel      string       // front-end user feedback about selected report item
			aboutUser       *models.User // associated user (e.g. owner of reported photo)
			messageRequired = true       // unless we have a table ID to work with
			success         = "Thank you for your feedback! Your message has been delivered to the website administrators."

			// CAPTCHA response for logged-out users to combat spam.
			turnstileCAPTCHA = r.PostFormValue("cf-turnstile-response")
		)

		// For report intents: ID of the user, photo, message, etc.
		tableID, err := strconv.ParseUint(r.FormValue("id"), 10, 64)
		if err != nil {
			// The tableID is not an int - was it a username?
			if user, err := models.FindUsername(r.FormValue("id")); err == nil {
				tableID = user.ID
			}
		}
		if tableID > 0 {
			messageRequired = false
		}

		// In what context is the ID given?
		if subject != "" && tableID > 0 {
			switch subject {
			case "report.user":
				tableName = "users"
				if user, err := models.GetUser(tableID); err == nil {
					tableLabel = fmt.Sprintf(`User account "%s"`, user.Username)
					aboutUser = user
				} else {
					log.Error("/contact: couldn't produce table label for user %d: %s", tableID, err)
				}
			case "report.photo":
				tableName = "photos"

				// Find this photo and the user associated.
				if pic, err := models.GetPhoto(tableID); err == nil {
					if user, err := models.GetUser(pic.UserID); err == nil {
						tableLabel = fmt.Sprintf(`A profile photo of user account "%s"`, user.Username)
						aboutUser = user
					} else {
						log.Error("/contact: couldn't produce table label for user %d: %s", tableID, err)
					}
				} else {
					log.Error("/contact: couldn't produce table label for photo %d: %s", tableID, err)
				}
			case "report.message":
				tableName = "messages"
				tableLabel = "Direct Message conversation"

				// Find this message, and attach it to the report.
				if msg, err := models.GetMessage(tableID); err == nil {
					var username = "[unavailable]"
					if sender, err := models.GetUser(msg.SourceUserID); err == nil {
						username = sender.Username
						aboutUser = sender
					}

					footer = fmt.Sprintf(`

---

From: <a href="/u/%s">@%s</a>

%s`,
						username, username,
						markdown.Quotify(msg.Message),
					)
				}
			case "report.comment":
				tableName = "comments"

				// Find this comment.
				if comment, err := models.GetComment(tableID); err == nil {
					tableLabel = fmt.Sprintf(`A comment written by "%s"`, comment.User.Username)
					aboutUser = &comment.User
					footer = fmt.Sprintf(`

---

From: <a href="/u/%s">@%s</a>

%s`,
						comment.User.Username, comment.User.Username,
						markdown.Quotify(comment.Message),
					)
				} else {
					log.Error("/contact: couldn't produce table label for comment %d: %s", tableID, err)
				}
			case "report.forum", "forum.adopt":
				tableName = "forums"

				// Find this forum.
				if forum, err := models.GetForum(tableID); err == nil {
					tableLabel = fmt.Sprintf(`The forum "%s" (/f/%s)`, forum.Title, forum.Fragment)
				} else {
					log.Error("/contact: couldn't produce table label for comment %d: %s", tableID, err)
				}
			}
		}

		// On POST: take what we have now and email the admins.
		if r.Method == http.MethodPost {
			// Look up the current user, in case logged in.
			currentUser, err := session.CurrentUser(r)
			if err == nil {
				replyTo = currentUser.Email
			}

			// We were getting too much spam logged-out: prevent logged-out bots from still posting.
			if currentUser == nil {

				// For logged-out posts, validate the CAPTCHA token to reduce spam.
				if config.Current.Turnstile.Enabled {
					if err := spam.ValidateTurnstileCAPTCHA(turnstileCAPTCHA, "contact"); err != nil {
						session.FlashError(w, r, "There was an error validating your CAPTCHA response. Please check the box to prove you are a human when submitting this form.")
						templates.Redirect(w, r.URL.Path)
						return
					}
				}

			}

			// If the (logged out) user left their username, add it to the message.
			if username != "" {
				message = fmt.Sprintf(
					"**Given Username:** [%s](/u/%s)\n\n%s",
					username, username,
					message,
				)
			}

			// Are any optional fields given for Advanced Reports?
			var optionals = []string{}
			for _, field := range OptionalFields {
				var (
					values  = r.PostForm[field.Key]
					isEmpty = strings.Join(values, "") == ""
				)
				if !isEmpty {
					optionals = append(optionals, fmt.Sprintf(
						"%s:\n\n%s",
						field.Name,
						markdown.Quotify(strings.Join(values, "\n\n")),
					))
				}
			}
			if len(optionals) > 0 {
				message += fmt.Sprintf(
					"\n\n**Additional Information Provided:**\n\n%s",
					strings.Join(optionals, "\n\n"),
				)
			}

			// Rate limit submissions.
			var rateLimitKey any
			if currentUser != nil {
				rateLimitKey = currentUser.ID
			} else {
				rateLimitKey = utility.IPAddress(r)
			}
			limiter := &ratelimit.Limiter{
				Namespace:  "contact",
				ID:         rateLimitKey,
				Limit:      config.ContactRateLimit,
				Window:     config.ContactRateLimitWindow,
				CooldownAt: config.ContactRateLimitCooldownAt,
				Cooldown:   config.ContactRateLimitCooldown,
			}

			if err := limiter.Ping(); err != nil {
				session.FlashError(w, r, "%s", err.Error())
				templates.Redirect(w, r.URL.Path)
				return
			}

			// If they have tripped the spam bot trap fields, don't save their message.
			if trap1 || trap2 {
				log.Error("Contact form: bot has tripped the trap fields, do not save message")
				session.Flash(w, r, "%s", success)
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Store feedback in the database.
			fb := &models.Feedback{
				Intent:    intent,
				Subject:   subject,
				Message:   message + footer,
				TableName: tableName,
				TableID:   tableID,
			}

			if aboutUser != nil {
				fb.AboutUserID = aboutUser.ID
			}

			if currentUser != nil && currentUser.ID > 0 {
				fb.UserID = currentUser.ID
			} else if replyTo != "" {
				fb.ReplyTo = replyTo
			}

			if err := models.CreateFeedback(fb); err != nil {
				session.FlashError(w, r, "Couldn't save feedback: %s", err)
				templates.Redirect(w, r.URL.Path)
				return
			}

			session.Flash(w, r, "%s", success)
			templates.Redirect(w, r.URL.Path)
			return
		}

		// Default intent = contact
		if intent == "report" {
			title = "Report a Problem"
		} else {
			intent = "contact"
		}

		// Validate the subject.
		if subject != "" {
			var found bool
			for _, group := range config.ContactUsChoices {
				for _, opt := range group.Options {
					if opt.Value == subject {
						found = true
						break
					}
				}
			}

			if !found {
				subject = ""
			}
		}

		var vars = map[string]interface{}{
			"Intent":          intent,
			"TableID":         r.FormValue("id"),
			"TableLabel":      tableLabel,
			"Subject":         subject,
			"PageTitle":       title,
			"Subjects":        config.ContactUsChoices,
			"Message":         message,
			"MessageRequired": messageRequired,
			"Username":        username,
			"Email":           replyTo,

			// Specific models being reported on to aid the frontend for Advanced Contact forms.
			"AboutUser": aboutUser,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
