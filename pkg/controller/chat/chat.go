package chat

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/chat/claims"
	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/middleware"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/worker"
)

// Landing page for chat rooms.
func Landing() http.HandlerFunc {
	tmpl := templates.Must("chat.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Is the BareRTC chat room not configured?
		if config.Current.BareRTC.URL == "" || config.Current.BareRTC.JWTSecret == "" {
			templates.NotFoundPage(w, r)
			return
		}

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get current user: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Are they logging into the chat room?
		var (
			intent = r.FormValue("intent")
		)
		if intent == "join" {
			// Maintenance mode?
			if middleware.ChatMaintenance(currentUser, w, r) {
				return
			}

			// Get our Chat JWT secret.
			var (
				secret  = []byte(config.Current.BareRTC.JWTSecret)
				chatURL = config.Current.BareRTC.URL
			)
			if len(secret) == 0 || chatURL == "" {
				session.FlashError(w, r, "Couldn't sign you into the chat: JWT secret key or chat URL not configured!")
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Country flag emoji.
			emoji := claims.GetChatFlagEmoji(r, currentUser)

			// Create the JWT claims.
			_, token, err := claims.SignClaims(currentUser, emoji, currentUser.ChatFlair())
			if err != nil {
				session.FlashError(w, r, "Couldn't sign you into the chat: %s", err)
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Send over their blocklist to the chat server.
			if err := SendBlocklist(currentUser); err != nil {
				log.Error("SendBlocklist: %s", err)
			}

			// Mark them as online immediately: so e.g. on the Change Username screen we leave no window
			// of time where they can exist in chat but change their name on the site.
			worker.GetChatStatistics().SetOnlineNow(currentUser.Username)

			// Ping their chat login usage statistic.
			go func() {
				if err := models.LogDailyChatUser(currentUser); err != nil {
					log.Error("LogDailyChatUser(%s): error logging this user's chat statistic: %s", currentUser.Username, err)
				}
			}()

			// Redirect them to the chat room.
			templates.Redirect(w, strings.TrimSuffix(chatURL, "/")+"/?jwt="+token)
			return
		}

		// Get the ChatStatistics and select our friend names from it.
		var (
			stats         = FilteredChatStatistics(currentUser)
			friendsOnline = models.FilterFriendUsernames(currentUser, stats.Usernames)
		)

		sort.Strings(friendsOnline)

		var vars = map[string]interface{}{
			"ChatAPI": strings.TrimSuffix(config.Current.BareRTC.URL, "/") + "/api/statistics",

			// Pre-populate the "who's online" widget from backend cache data
			"ChatStatistics": stats,
			"FriendsOnline":  friendsOnline,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

// FilteredChatStatistics will return a copy of the cached ChatStatistics but where the Usernames list is
// filtered down (and the online user counts, accordingly) by blocklists.
func FilteredChatStatistics(currentUser *models.User) worker.ChatStatistics {
	var stats = worker.GetChatStatistics()
	var result = worker.ChatStatistics{
		UserCount: stats.UserCount,
		Usernames: []string{},
		Cameras:   stats.Cameras,
	}

	// Who are we blocking?
	var blockedUsernames = map[string]interface{}{}
	for _, username := range models.BlockedUsernames(currentUser) {
		blockedUsernames[username] = nil
	}

	// Filter the online users listing.
	for _, username := range stats.Usernames {
		if _, ok := blockedUsernames[username]; !ok {
			result.Usernames = append(result.Usernames, username)
		}
	}

	// Sort the names.
	sort.Strings(result.Usernames)

	return result
}

// SendBlocklist syncs the user blocklist to the chat server prior to sending them over.
func SendBlocklist(user *models.User) error {
	// Get the user's blocklist.
	blockedUsernames := models.BlockedUsernames(user)
	log.Info("SendBlocklist(%s) to BareRTC: %d blocked usernames", user.Username, len(blockedUsernames))

	// API request struct for BareRTC /api/blocklist endpoint.
	var request = struct {
		APIKey    string
		Username  string
		Blocklist []string
	}{
		config.Current.CronAPIKey,
		user.Username,
		blockedUsernames,
	}

	// JSON request body.
	jsonStr, err := json.Marshal(request)
	if err != nil {
		return err
	}

	// Make the API request to BareRTC.
	var url = strings.TrimSuffix(config.Current.BareRTC.URL, "/") + "/api/blocklist"
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

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Error("SendBlocklist: error syncing blocklist to BareRTC: status %d body %s", resp.StatusCode, body)
	}

	return nil
}

// BlockUserNow syncs the new block action to the chat server now, in case the user is already online.
func BlockUserNow(currentUser, user *models.User) error {
	// API request struct for BareRTC /api/block/now endpoint.
	var request = struct {
		APIKey    string
		Usernames []string
	}{
		config.Current.CronAPIKey,
		[]string{
			currentUser.Username,
			user.Username,
		},
	}

	// JSON request body.
	jsonStr, err := json.Marshal(request)
	if err != nil {
		return err
	}

	// Make the API request to BareRTC.
	var url = strings.TrimSuffix(config.Current.BareRTC.URL, "/") + "/api/block/now"
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

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Error("BlockUserNow: error syncing block to BareRTC: status %d body %s", resp.StatusCode, body)
	}

	return nil
}
