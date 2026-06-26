package worker

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
)

// ChatStatistics is the json result of the BareRTC /api/statistics endpoint.
type ChatStatistics struct {
	UserCount int
	Usernames []string
	Cameras   struct {
		Blue int
		Red  int
	}
}

// GetChatStatistics returns the latest (cached) chat statistics.
func GetChatStatistics() *ChatStatistics {
	chatStatisticsMu.RLock()
	defer chatStatisticsMu.RUnlock()
	return cachedChatStatistics
}

// SetChatStatistics updates the cached chat statistics, holding a write lock briefly.
func SetChatStatistics(stats *ChatStatistics) {
	chatStatisticsMu.Lock()
	defer chatStatisticsMu.Unlock()

	if stats == nil {
		cachedChatStatistics = &ChatStatistics{}
		return

	}
	cachedChatStatistics = stats
}

// IsOnline returns whether the username is currently logged-in to chat.
func (cs *ChatStatistics) IsOnline(username string) bool {
	for _, user := range cs.Usernames {
		if user == username {
			return true
		}
	}
	return false
}

// SetOnlineNow patches the current ChatStatistics to mark a user as online immediately, e.g.
// because the main site has just sent them to the chat with a JWT token.
func (cs *ChatStatistics) SetOnlineNow(username string) {
	if !cs.IsOnline(username) {
		chatStatisticsMu.Lock()
		defer chatStatisticsMu.Unlock()
		cs.Usernames = append(cs.Usernames, username)
	}
}

type UserOnChatMap map[string]bool

// MapUsersOnline returns a hashmap of usernames to online status.
func (cs *ChatStatistics) MapUsersOnline(usernames []string) UserOnChatMap {
	var result = UserOnChatMap{}
	for _, user := range cs.Usernames {
		result[user] = true
	}
	return result
}

// Get a result from the UserOnChatMap.
func (m UserOnChatMap) Get(username string) bool {
	return m[username]
}

var (
	cachedChatStatistics = &ChatStatistics{}
	chatStatisticsMu     sync.RWMutex
)

// WatchBareRTC is a worker goroutine that caches the current online chatters in the chat room.
func WatchBareRTC() {
	if config.Current.BareRTC.JWTSecret == "" || config.Current.BareRTC.URL == "" {
		log.Error("Worker (WatchBareRTC): chat room is not configured, will not watch chat room status")
		return
	}

	// Check it immediately.
	DoCheckBareRTC()

	// And on an interval forever.
	for {
		time.Sleep(config.ChatStatusRefreshInterval)
		DoCheckBareRTC()
	}
}

// DoCheckBareRTC invokes the attempt to refresh data from the chat server about who's online.
func DoCheckBareRTC() {
	log.Info("Refresh BareRTC")
	req, err := http.NewRequest(http.MethodGet, config.Current.BareRTC.URL+"/api/statistics", nil)

	if err != nil {
		log.Error("WatchBareRTC: couldn't make request: %s", err)
		SetChatStatistics(nil)
		return
	}

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		log.Error("WatchBareRTC: request error: %s", err)
		SetChatStatistics(nil)
		return
	} else if res.StatusCode != http.StatusOK {
		log.Error("WatchBareRTC: didn't get expected 200 OK from statistics endpoint, instead got: %s", res.Status)
		SetChatStatistics(nil)
		return
	}

	if res.StatusCode == http.StatusOK {
		var cs ChatStatistics
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if err = json.Unmarshal(body, &cs); err != nil {
			log.Error("WatchBareRTC: json decode error: %s", err)
			return
		}

		SetChatStatistics(&cs)
	}
}
