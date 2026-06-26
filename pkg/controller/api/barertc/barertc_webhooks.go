package barertc

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/controller/api"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/utility"
)

// WebhookRequest is a JSON request wrapper around all webhook messages.
type WebhookRequest struct {
	Action string
	APIKey string

	// Relevant body per request.
	Report WebhookRequestReport `json:",omitempty"`

	// Optional params per request.
	Username  string   `json:",omitempty"` // e.g. for profile webhook
	Usernames []string `json:",omitempty"` // e.g. for friends webhook
}

// WebhookRequestReport is the body for 'report' webhook messages.
type WebhookRequestReport struct {
	FromUsername  string
	AboutUsername string
	Channel       string
	Timestamp     string
	Reason        string
	Message       string
	Comment       string
}

// Report webhook controller.
func Report() http.HandlerFunc {

	// Response JSON schema.
	type Response struct {
		OK    bool
		Error string `json:",omitempty"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			api.SendJSON(w, http.StatusNotAcceptable, Response{
				Error: "POST method only",
			})
			return
		}

		// Parse request payload.
		var req WebhookRequest
		if err := api.ParseJSON(r, &req); err != nil {
			api.SendJSON(w, http.StatusBadRequest, Response{
				Error: fmt.Sprintf("Error with request payload: %s", err),
			})
			return
		}

		// Validate the AdminAPIKey.
		if req.APIKey != config.Current.CronAPIKey {
			api.SendJSON(w, http.StatusForbidden, Response{
				Error: "Invalid API Key",
			})
			return
		}

		// Get the report out.
		report := req.Report
		if report.Comment == "" {
			report.Comment = "(no comment)"
		}

		log.Debug("Got chat report: %+v", report)

		// Make a clickable profile link for the channel ID (other user).
		otherUsername := strings.TrimPrefix(report.Channel, "@")

		// Create an admin Feedback model.
		fb := &models.Feedback{
			Intent:  "report",
			Subject: "report.chat",
			Message: fmt.Sprintf(
				"A message was reported on the chat room!\n\n"+
					"* From username: [%s](/u/%s)\n"+
					"* About username: [%s](/u/%s)\n"+
					"* Channel: [**%s**](/u/%s)\n"+
					"* Timestamp: %s\n"+
					"* Classification: %s\n"+
					"* User comment: %s\n\n"+
					"- - - - -\n\n"+
					"The reported message on chat was:\n\n%s",
				report.FromUsername, report.FromUsername,
				report.AboutUsername, report.AboutUsername,
				report.Channel, otherUsername,
				report.Timestamp,
				report.Reason,
				report.Comment,
				report.Message,
			),
		}

		// Get the Reply-To user if possible.
		currentUser, err := models.FindUsername(report.FromUsername)
		if err == nil {
			fb.UserID = currentUser.ID
		} else {
			currentUser = nil
		}

		// Look up the AboutUser ID if possible.
		targetUser, err := models.FindUsername(report.AboutUsername)
		if err == nil {
			fb.TableName = "users"
			fb.TableID = targetUser.ID
			fb.AboutUserID = targetUser.ID
		} else {
			log.Error("BareRTC Chat Feedback: couldn't find user ID for AboutUsername=%s: %s", report.AboutUsername, err)
		}

		// Save the feedback.
		if err := models.CreateFeedback(fb); err != nil {
			log.Error("Couldn't save feedback from BareRTC report endpoint: %s", err)
		}

		// Send success response.
		api.SendJSON(w, http.StatusOK, Response{
			OK: true,
		})
	})
}

// Profile webhook controller to fetch more profile details for a user.
func Profile() http.HandlerFunc {

	// Response JSON schema.
	type ProfileField struct {
		Name  string
		Value string
	}
	type Response struct {
		OK            bool
		Error         string `json:",omitempty"`
		Headline      string
		ProfileFields []ProfileField
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			api.SendJSON(w, http.StatusNotAcceptable, Response{
				Error: "POST method only",
			})
			return
		}

		// Parse request payload.
		var req WebhookRequest
		if err := api.ParseJSON(r, &req); err != nil {
			api.SendJSON(w, http.StatusBadRequest, Response{
				Error: fmt.Sprintf("Error with request payload: %s", err),
			})
			return
		}

		// Validate the AdminAPIKey.
		if req.APIKey != config.Current.CronAPIKey {
			api.SendJSON(w, http.StatusForbidden, Response{
				Error: "Invalid API Key",
			})
			return
		}

		// Get the Reply-To user if possible.
		currentUser, err := models.FindUsername(req.Username)
		if err != nil {
			api.SendJSON(w, http.StatusForbidden, Response{
				Error: "Username Not Found",
			})
			return
		}

		// Populate their profile fields.
		var maritalStatus = currentUser.GetProfileFieldOr("marital_status", "n/a")
		if relationshipType := currentUser.GetProfileField("relationship_type"); relationshipType != "" {
			maritalStatus += fmt.Sprintf(" (%s)", relationshipType)
		}

		var gender = currentUser.GetProfileFieldOr("gender", "n/a")
		if pronouns := currentUser.GetProfileField("pronouns"); pronouns != "" {
			gender += fmt.Sprintf(" (%s)", pronouns)
		}

		var photoCount = models.CountPublicPhotos(currentUser.ID)

		// Member Since date.
		var memberSinceDate = currentUser.CreatedAt

		var resp = Response{
			OK:       true,
			Headline: currentUser.GetProfileField("headline"),
			ProfileFields: []ProfileField{
				{
					Name:  "Certified since",
					Value: fmt.Sprintf("%s ago", utility.FormatDurationCoarse(time.Since(memberSinceDate))),
				},
				{
					Name:  "📸 Gallery",
					Value: fmt.Sprintf("At least %d photo%s", photoCount, templates.Pluralize(photoCount)),
				},
				{
					Name:  "Age",
					Value: currentUser.GetDisplayAge(),
				},
				{
					Name:  "Gender",
					Value: gender,
				},
				{
					Name:  "City",
					Value: currentUser.GetProfileFieldOr("city", "n/a"),
				},
				{
					Name:  "Job",
					Value: currentUser.GetProfileFieldOr("job", "n/a"),
				},
				{
					Name:  "Marital status",
					Value: maritalStatus,
				},
				{
					Name:  "Orientation",
					Value: currentUser.GetProfileFieldOr("orientation", "n/a"),
				},
				{
					Name:  "Here for",
					Value: strings.Join(strings.Split(currentUser.GetProfileFieldOr("here_for", "n/a"), ","), ", "),
				},
			},
		}

		// Send success response.
		api.SendJSON(w, http.StatusOK, resp)
	})
}

// Friends webhook controller to fetch friendship booleans between one user and others.
func Friends() http.HandlerFunc {

	// Response JSON schema.
	type Response struct {
		OK      bool
		Error   string `json:",omitempty"`
		Friends map[string]bool
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			api.SendJSON(w, http.StatusNotAcceptable, Response{
				Error: "POST method only",
			})
			return
		}

		// Parse request payload.
		var req WebhookRequest
		if err := api.ParseJSON(r, &req); err != nil {
			api.SendJSON(w, http.StatusBadRequest, Response{
				Error: fmt.Sprintf("Error with request payload: %s", err),
			})
			return
		}

		// Validate the AdminAPIKey.
		if req.APIKey != config.Current.CronAPIKey {
			api.SendJSON(w, http.StatusForbidden, Response{
				Error: "Invalid API Key",
			})
			return
		}

		// Get the current user asking.
		currentUser, err := models.FindUsername(req.Username)
		if err != nil {
			api.SendJSON(w, http.StatusForbidden, Response{
				Error: "Username Not Found",
			})
			return
		}

		// Get all the other users.
		users, err := models.GetUsersByUsernames(currentUser, req.Usernames)
		if err != nil {
			api.SendJSON(w, http.StatusInternalServerError, Response{
				Error: "Couldn't look up those usernames",
			})
			return
		}

		// Map the users by ID for easy lookup.
		var userMap = map[uint64]*models.User{}
		for _, user := range users {
			userMap[user.ID] = user
		}

		// Map friendships.
		var friendMap = map[string]bool{}
		if m := models.MapFriends(currentUser, users); len(m) > 0 {
			for userID, isFriends := range m {
				if !isFriends {
					continue
				}
				if user, ok := userMap[userID]; ok {
					friendMap[user.Username] = true
				}
			}
		}

		var resp = Response{
			OK:      true,
			Friends: friendMap,
		}

		// Send success response.
		api.SendJSON(w, http.StatusOK, resp)
	})
}
