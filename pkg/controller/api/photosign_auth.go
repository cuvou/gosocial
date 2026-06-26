package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/encryption"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/photo"
	"github.com/cuvou/gosocial/pkg/session"
)

// PhotoSignAuth API protects paths like /static/photos/ to authenticated user requests only.
func PhotoSignAuth() http.HandlerFunc {
	type Response struct {
		Success  bool   `json:"success"`
		Error    string `json:",omitempty"`
		Username string `json:"username"`
	}

	logAndError := func(w http.ResponseWriter, m string, v ...interface{}) {
		log.Debug("ERROR PhotoSignAuth: "+m, v...)
		SendJSON(w, http.StatusForbidden, Response{
			Error: fmt.Sprintf(m, v...),
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// We only protect the /static/photos subpath.
		// And check if the SignedPhoto feature is enabled and enforcing.
		var originalURI = r.Header.Get("X-Original-URI")
		if !config.Current.SignedPhoto.Enabled || !strings.HasPrefix(originalURI, config.PhotoWebPath) {
			SendJSON(w, http.StatusOK, Response{
				Success: true,
			})
			return
		}

		// Get the base filename.
		var filename = strings.TrimPrefix(
			strings.SplitN(originalURI, config.PhotoWebPath, 2)[1],
			"/",
		)
		filename = strings.SplitN(filename, "?", 2)[0] // inner query string too

		// Parse the JWT token parameter from the original URL.
		var token string
		if path, err := url.Parse(originalURI); err == nil {
			query := path.Query()
			token = query.Get("jwt")
		}

		// The JWT token is required from here on out.
		if token == "" {
			logAndError(w, "JWT token is required")
			return
		}

		// Check if we're logged in and who the current username is.
		var username string
		if currentUser, err := session.CurrentUser(r); err == nil {
			username = currentUser.Username
		}

		// Validate the JWT token is correctly signed and not expired.
		claims, ok, err := encryption.ValidateClaims(
			token,
			[]byte(config.Current.SignedPhoto.JWTSecret),
			&photo.SignedPhotoClaims{},
		)
		if !ok || err != nil {
			logAndError(w, "When validating JWT claims: %v", err)
			return
		}

		// Parse the claims to get data to validate this request.
		c, ok := claims.(*photo.SignedPhotoClaims)
		if !ok {
			logAndError(w, "JWT claims were not the correct shape: %+v", claims)
			return
		}

		// Was the signature for our username? (Skip if for Anyone)
		if !c.Anyone && c.Subject != username {
			logAndError(w, "That token did not belong to you")
			return
		}

		// Is the file name correct?
		hash := photo.FilenameHash(filename)
		if hash != c.FilenameHash {
			logAndError(w, "Filename hash mismatch: fn=%s  hash=%s  jwt=%s", filename, hash, c.FilenameHash)
			return
		}

		log.Debug("PhotoSignAuth: JWT Signature OK! fn=%s  u=%s  anyone=%v  expires=%+v", filename, c.Subject, c.Anyone, c.ExpiresAt)

		SendJSON(w, http.StatusOK, Response{
			Success:  true,
			Username: username,
		})
	})
}
