// Package session handles user login and other cookies.
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/encryption"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/mail"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/redis"
	"github.com/cuvou/gosocial/pkg/utility"
	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

// Session cookie object that is kept server side in Redis.
type Session struct {

	// v2 sessions with ULIDs and ClientSecret checksums.
	ULID         ulid.ULID            `json:"-"` // not stored
	ClientSecret string               `json:"-"` // string read from client cookie
	LoginSession *models.LoginSession `json:"-"`

	// v1 sessions (legacy UUIDs, stored in Redis) - DEPRECATED.
	UUID string `json:"-"` // not stored

	// Session values.
	LoggedIn     bool      `json:"loggedIn"`
	UserID       uint64    `json:"userId,omitempty"`
	Flashes      []string  `json:"flashes,omitempty"`
	Errors       []string  `json:"errors,omitempty"`
	Impersonator uint64    `json:"impersonator,omitempty"`
	SafeMode     bool      `json:"safeMode,omitempty"`
	LastSeen     time.Time `json:"lastSeen"`
}

type RequestContextKey string

const (
	ContextKey     RequestContextKey = "session"
	CurrentUserKey RequestContextKey = "current_user"
	CSRFKey        RequestContextKey = "csrf"
	RequestTimeKey RequestContextKey = "req_time"
)

// New creates a blank session object.
func New() *Session {
	return &Session{
		ULID:    ulid.Make(),
		Flashes: []string{},
		Errors:  []string{},
	}
}

// LogoutOtherSessions logs other sessions out for the current user.
func LogoutOtherSessions(r *http.Request) error {
	if currentUser, err := CurrentUser(r); err == nil {
		sess := Get(r)
		return models.RevokeAllLoginSessions(currentUser, sess.ULID)
	}
	return errors.New("not logged in")
}

// Load the session from the browser session_id token and Redis or creates a new session.
func LoadOrNew(w http.ResponseWriter, r *http.Request) *Session {
	var sess = New()

	// Read the session cookie value.
	cookie, err := r.Cookie(config.SessionCookieName)
	if err != nil {
		log.Debug("session.LoadOrNew: cookie error, new sess: %s", err)
		return sess
	}

	// v2 sessions: read the client_secret cookie, for later validation.
	if csCookie, err := r.Cookie(config.SessionSecretCookieName); err == nil {
		sess.ClientSecret = csCookie.Value
	}

	// If the cookie is a legacy v1 UUID, load it from Redis.
	// TODO: this code can be gutted once Redis sessions are all expired (30 days out).
	if _, err := uuid.Parse(cookie.Value); err == nil {
		// Look up this UUID in Redis.
		sess.UUID = cookie.Value
		key := fmt.Sprintf(config.SessionRedisKeyFormat, sess.UUID)

		err = redis.Get(key, sess)
		if err != nil {
			log.Error("session.LoadOrNew: didn't find %s in Redis: %s", key, err)
		}

		// Migrate the session to v2.
		if err := MigrateV2(w, r, sess); err != nil {
			log.Error("session.LoadOrNew: error migrating to v2 session: %s", err)
		}

		return sess
	}

	// It should be a v2 session ULID.
	if id, err := ulid.Parse(cookie.Value); err == nil {
		// Look it up from SQL.
		ls, err := models.GetLoginSession(id)
		if err != nil {
			log.Error("session.LoadOrNew: didn't find ULID %s in SQL: %s", id, err)
		} else {
			// Load the session data.
			sess.ULID = id
			sess.LoginSession = ls
			json.Unmarshal([]byte(ls.Data), &sess)

			// Validate the ClientSecretHash.
			if sess.LoginSession.ClientSecretHash != "" {
				if !encryption.VerifyHash([]byte(sess.ClientSecret), sess.LoginSession.ClientSecretHash) {
					log.Error("session.LoadOrNew: ClientSecretHash mismatch!")
					sess.Errors = append(sess.Errors, "An authentication error has occurred, please log in again.")
					sess.ClientSecret = ""
					sess.LoggedIn = false
					sess.UserID = 0
				}
			}
		}
	}

	return sess
}

// Save the session and send a cookie header.
func (s *Session) Save(w http.ResponseWriter, r *http.Request) {
	// Ensure we have a ULID.
	if s.ULID.IsZero() {
		s.ULID = ulid.Make()
	}

	// Ping last seen.
	s.LastSeen = time.Now()

	// Create a ClientSecret string as a checksum for their session.
	if s.ClientSecret == "" {
		s.ClientSecret = uuid.New().String()
	}

	// Serialize the session data payload.
	serialized, err := s.Serialize()
	if err != nil {
		log.Error("Session.Save: couldn't serialize to JSON: %s", err)
		return
	}

	// Keep the original CreatedAt if their session was already in SQL.
	var createdAt time.Time
	if s.LoginSession != nil {
		createdAt = s.LoginSession.CreatedAt
	}

	// Upsert their SQL LoginSession.
	s.LoginSession = &models.LoginSession{
		ULID:             s.ULID.String(),
		Data:             serialized,
		ClientSecretHash: encryption.Hash([]byte(s.ClientSecret)),
		IPAddress:        utility.IPAddress(r),
		UserAgent:        r.UserAgent(),
		CreatedAt:        createdAt,
		ExpiresAt:        time.Now().Add(time.Duration(config.SessionCookieMaxAge) * time.Second),
	}
	if s.LoggedIn {
		s.LoginSession.UserID = &s.UserID
	}
	if err := s.LoginSession.Save(); err != nil {
		log.Error("Session.Save: couldn't save session to Postgres: %s", err)
	}

	cookie := &http.Cookie{
		Name:     config.SessionCookieName,
		Value:    s.ULID.String(),
		MaxAge:   config.SessionCookieMaxAge,
		Path:     "/",
		HttpOnly: true,
	}
	secretCookie := &http.Cookie{
		Name:     config.SessionSecretCookieName,
		Value:    s.ClientSecret,
		MaxAge:   config.SessionCookieMaxAge,
		Path:     "/",
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
	http.SetCookie(w, secretCookie)
}

// Get the session from the current HTTP request context.
func Get(r *http.Request) *Session {
	if r == nil {
		panic("session.Get: http.Request is required")
	}

	ctx := r.Context()
	if sess, ok := ctx.Value(ContextKey).(*Session); ok {
		return sess
	}

	// If the session isn't on the request, it means I broke something.
	log.Error("session.Get(): didn't find session in request context!")
	return nil
}

// ReadFlashes returns and clears the Flashes and Errors for this session.
func (s *Session) ReadFlashes(w http.ResponseWriter, r *http.Request) (flashes, errors []string) {
	flashes = s.Flashes
	errors = s.Errors
	s.Flashes = []string{}
	s.Errors = []string{}
	if len(flashes)+len(errors) > 0 {
		s.Save(w, r)
	}
	return flashes, errors
}

// Flash adds a transient message to the user's session to show on next page load.
func Flash(w http.ResponseWriter, r *http.Request, msg string, args ...interface{}) {
	sess := Get(r)
	if sess.Flashes == nil {
		sess.Flashes = []string{}
	}
	sess.Flashes = append(sess.Flashes, fmt.Sprintf(msg, args...))
	sess.Save(w, r)
}

// FlashError adds a transient error message to the session.
func FlashError(w http.ResponseWriter, r *http.Request, msg string, args ...interface{}) {
	sess := Get(r)
	if sess.Errors == nil {
		sess.Errors = []string{}
	}
	sess.Errors = append(sess.Errors, fmt.Sprintf(msg, args...))
	sess.Save(w, r)
}

// LoginUser marks a session as logged in to an account.
func LoginUser(w http.ResponseWriter, r *http.Request, u *models.User) error {
	if u == nil || u.ID == 0 {
		return errors.New("not a valid user account")
	}

	sess := Get(r)
	sess.LoggedIn = true
	sess.UserID = u.ID
	sess.Impersonator = 0
	sess.Save(w, r)

	// Ping the user's last login time.
	return u.PingLastLoginAt()
}

// ImpersonateUser assumes the role of the user impersonated by an admin uid.
func ImpersonateUser(w http.ResponseWriter, r *http.Request, u *models.User, impersonator *models.User, reason string) error {
	if u == nil || u.ID == 0 {
		return errors.New("not a valid user account")
	}
	if impersonator == nil || impersonator.ID == 0 || !impersonator.IsAdmin {
		return errors.New("impersonator not a valid admin account")
	}

	sess := Get(r)
	sess.LoggedIn = true
	sess.UserID = u.ID
	sess.Impersonator = impersonator.ID
	sess.Save(w, r)

	// Issue an admin notification that this has happened.
	// NOTE: not DRY compared to contact.go
	fb := &models.Feedback{
		Intent:    "report",
		Subject:   "'Impersonate user' has been used",
		TableName: "users",
		TableID:   impersonator.ID,
		Message: fmt.Sprintf(
			"The admin user **%s** (id:%d) has impersonated user **%s** (id:%d)\n\n"+
				"The reason they have given:\n\n%s",
			impersonator.Username, impersonator.ID,
			u.Username, u.ID, reason,
		),
	}

	if err := models.CreateFeedback(fb); err != nil {
		FlashError(w, r, "Couldn't create admin notification: %s", err)
	}

	// Email the admins.
	if err := mail.Send(mail.Message{
		To:       config.Current.AdminEmail,
		Subject:  "Admin 'user impersonate' has been used",
		Template: "email/admin_impersonate.html",
		Data: map[string]interface{}{
			"Impersonator": impersonator,
			"User":         u,
			"Reason":       reason,
			"AdminURL":     config.Current.BaseURL + "/admin/feedback",
		},
	}); err != nil {
		log.Error("/contact page: couldn't send email: %s", err)
	}

	return u.Save()
}

// Impersonated returns if the current session has an impersonator.
func Impersonated(r *http.Request) bool {
	sess := Get(r)
	if sess == nil {
		return false
	}

	return sess.Impersonator > 0
}

// LogoutUser signs a user out.
func LogoutUser(w http.ResponseWriter, r *http.Request) {
	sess := Get(r)
	sess.LoggedIn = false
	sess.UserID = 0
	sess.Save(w, r)
}

// Serialize the session data to string.
func (s *Session) Serialize() (string, error) {
	b, err := json.Marshal(s)
	return string(b), err
}
