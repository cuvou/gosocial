package chat

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/chat/claims"
	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
)

// MaybeDisconnectUser may send a DisconnectUserNow to BareRTC if the user should not be allowed in the chat room.
//
// For example, they have set their profile to private and become a shy account, or they deactivated or got banned.
//
// If the user is presently in the chat room, they will be removed and given an appropriate ChatServer message.
//
// Returns a boolean OK (they were online in chat, and were removed) with the error only returning in case of a
// communication or JSON encode error with BareRTC. If they were online and removed, an admin feedback notice is
// also generated for visibility and confirmation of success.
func MaybeDisconnectUser(user *models.User) (bool, error) {
	// What reason to remove them? If a message is provided, the DisconnectUserNow API will be called.
	var because = "You have been signed out of chat because "
	var reasons = []struct {
		If      bool
		Message string
	}{
		{
			If:      user.Status == models.UserStatusDisabled,
			Message: because + "you have deactivated your gosocial account.",
		},
		{
			If:      user.Status == models.UserStatusBanned,
			Message: because + "your gosocial account has been banned.",
		},
		{
			// Catch-all for any non-active user status.
			If:      user.Status != models.UserStatusActive,
			Message: because + "your gosocial account is no longer eligible to remain in the chat room.",
		},
	}

	for _, reason := range reasons {
		if reason.If {
			i, err := DisconnectUserNow(user, reason.Message)
			if err != nil {
				return false, err
			}

			// Were they online and were removed? Notify the admin for visibility.
			if i > 0 {
				fb := &models.Feedback{
					Intent:    "report",
					Subject:   "Auto-Disconnect from Chat",
					UserID:    user.ID,
					TableName: "users",
					TableID:   user.ID,
					Message: fmt.Sprintf(
						"A user was automatically disconnected from the chat room!\n\n"+
							"* Username: %s\n"+
							"* Number of users removed: %d\n"+
							"* Message sent to them: %s\n\n"+
							"Note: this is an informative message only. Users are expected to be removed from "+
							"chat when they do things such as deactivate their account, or private their profile "+
							"or pictures, and thus become ineligible to remain in the chat room.",
						user.Username,
						i,
						reason.Message,
					),
				}

				// Save the feedback.
				if err := models.CreateFeedback(fb); err != nil {
					log.Error("Couldn't save feedback from user updating their DOB: %s", err)
				}
			}

			// First removal reason wins.
			break
		}
	}

	return false, nil
}

// DisconnectUserNow tells the chat room to remove the user now if they are presently online.
func DisconnectUserNow(user *models.User, message string) (int, error) {
	// API request struct for BareRTC /api/block/now endpoint.
	var request = struct {
		APIKey    string
		Usernames []string
		Message   string
		Kick      bool
	}{
		APIKey: config.Current.CronAPIKey,
		Usernames: []string{
			user.Username,
		},
		Message: message,
		Kick:    false,
	}

	type response struct {
		OK      bool
		Removed int
		Error   string `json:",omitempty"`
	}

	// JSON request body.
	jsonStr, err := json.Marshal(request)
	if err != nil {
		return 0, err
	}

	// Make the API request to BareRTC.
	var url = strings.TrimSuffix(config.Current.BareRTC.URL, "/") + "/api/disconnect/now"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Ingest the JSON response to see the count and error.
	var (
		result  response
		body, _ = io.ReadAll(resp.Body)
	)
	err = json.Unmarshal(body, &result)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusOK || !result.OK {
		log.Error("DisconnectUserNow: error from BareRTC: status %d body %s", resp.StatusCode, body)
		return result.Removed, errors.New(result.Error)
	}

	return result.Removed, nil
}

// AmendJWTToken sends the chat server an updated JWT token for the logged-in user.
//
// If this is on behalf of the current user (HTTP request user), provide the HTTP request.
// This will lead to a more consistent country code emoji string.
//
// If this is an admin action (e.g., changing their chat rules), their emoji data may
// be lost if they don't have their Location Settings configured (as the user's real
// IP address is not available here for GeoIP).
func AmendJWTToken(r *http.Request, userID uint64) error {

	user, err := models.GetUser(userID)
	if err != nil {
		return fmt.Errorf("AmendJWTToken: didn't get user ID %d: %w", userID, err)
	}

	// Try and reproduce the user's country code emoji string.
	emoji := claims.GetChatFlagEmoji(r, user)

	// Get the user's new JWT token.
	_, token, err := claims.SignClaims(user, emoji, user.ChatFlair())
	if err != nil {
		return fmt.Errorf("AmendJWTToken; signing claims: %w", err)
	}

	// API request struct for BareRTC /api/amend-jwt endpoint.
	var request = struct {
		APIKey   string
		Username string
		JWTToken string
	}{
		APIKey:   config.Current.CronAPIKey,
		Username: user.Username,
		JWTToken: token,
	}

	type response struct {
		OK    bool
		Error string `json:",omitempty"`
	}

	// JSON request body.
	jsonStr, err := json.Marshal(request)
	if err != nil {
		return err
	}

	// Make the API request to BareRTC.
	var url = strings.TrimSuffix(config.Current.BareRTC.URL, "/") + "/api/amend-jwt"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Ingest the JSON response to see the count and error.
	var (
		result  response
		body, _ = io.ReadAll(resp.Body)
	)
	err = json.Unmarshal(body, &result)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK || !result.OK {
		log.Error("AmendJWTToken: error from BareRTC: status %d body %s", resp.StatusCode, body)
		return errors.New(result.Error)
	}

	return nil
}

// EraseChatHistory tells the chat room to clear DMs history for this user.
func EraseChatHistory(username string) (int, error) {
	// API request struct for BareRTC /api/message/clear endpoint.
	var request = struct {
		APIKey   string
		Username string
	}{
		APIKey:   config.Current.CronAPIKey,
		Username: username,
	}

	type response struct {
		OK             bool
		MessagesErased int
		Error          string `json:",omitempty"`
	}

	// JSON request body.
	jsonStr, err := json.Marshal(request)
	if err != nil {
		return 0, err
	}

	// Make the API request to BareRTC.
	var url = strings.TrimSuffix(config.Current.BareRTC.URL, "/") + "/api/message/clear"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Ingest the JSON response to see the count and error.
	var (
		result  response
		body, _ = io.ReadAll(resp.Body)
	)
	err = json.Unmarshal(body, &result)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusOK || !result.OK {
		log.Error("EraseChatHistory: error from BareRTC: status %d body %s", resp.StatusCode, body)
		return result.MessagesErased, errors.New(result.Error)
	}

	return result.MessagesErased, nil
}
