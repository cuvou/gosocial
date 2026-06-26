package photo

import (
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/encryption"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/utility"
	"github.com/golang-jwt/jwt/v4"
)

// VisibleAvatarURL returns the visible URL image to a user's square profile picture, from the point of view of the currentUser.
func VisibleAvatarURL(user, currentUser *models.User) string {
	canSee, visibility := user.CanSeeProfilePicture(currentUser)
	if canSee && user.ProfilePhoto.ID > 0 {
		return SignedPublicAvatarURL(user.ProfilePhoto.CroppedFilename)
	}

	switch visibility {
	case models.PhotoPrivate:
		return "/static/img/shy-private.png"
	case models.PhotoFriends:
		return "/static/img/shy-friends.png"
	}

	// Was it Explicit? If Explicit and Public but the current user doesn't opt-in, the code
	// path gets to this point.
	if user.ProfilePhoto.Explicit {
		return "/static/img/shy-explicit.png"
	}

	return "/static/img/shy.png"
}

// ReSignPhotoLinks will search a blob of text for photo gallery links ("/static/photos/*") and re-sign
// their JWT security tokens.
func ReSignPhotoLinks(currentUser *models.User, text string) string {
	var matches = config.PhotoURLRegexp.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		var (
			origString = m[0]
			url        = m[1]
			filename   string
		)
		log.Error("ReSignPhotoLinks: got [%s] url [%s]", origString, url)

		// Trim the /static/photos/ prefix off to get the URL down to its base filename.
		filename = strings.Split(url, "?")[0]
		filename = strings.TrimPrefix(filename, config.PhotoWebPath)
		filename = strings.TrimPrefix(filename, "/")

		// Sign the URL and replace the original.
		signed := SignedPhotoURL(currentUser, filename)
		text = strings.ReplaceAll(text, origString, signed)
	}
	return text
}

// SignedPhotoURL returns a URL path to a photo's filename, signed for the current user only.
func SignedPhotoURL(user *models.User, filename string) string {
	return createSignedPhotoURL(user.ID, user.Username, filename, false)
}

// SignedPublicAvatarURL returns a signed URL for a user's public square avatar image, which has
// a much more generous JWT expiration lifetime on it.
//
// The primary use case is for the chat room: users are sent into chat with their avatar URL,
// and it must be viewable to all users for a long time.
func SignedPublicAvatarURL(filename string) string {
	return createSignedPhotoURL(0, "@", filename, true)
}

// SignedPhotoClaims are a JWT claims object used to sign and authenticate image (direct .jpg) links.
type SignedPhotoClaims struct {
	FilenameHash string `json:"f"`           // Short hash of the Filename being signed.
	Anyone       bool   `json:"a,omitempty"` // Non-authenticated signature (e.g. public sq avatar URLs)

	// Standard claims. Notes:
	// .Subject = username
	jwt.RegisteredClaims
}

// FilenameHash returns a 'short' hash of the filename, for encoding in the SignedPhotoClaims.
//
// The hash is a truncated SHA256 hash as a basic validation measure against one JWT token being
// used to reveal an unrelated picture.
func FilenameHash(filename string) string {
	return encryption.Hash([]byte(filename))[:6]
}

// Common function to create a signed photo URL with an expiration.
func createSignedPhotoURL(userID uint64, username string, filename string, anyone bool) string {

	if !config.Current.SignedPhoto.Enabled {
		return URLPath(filename)
	}

	// Claims expire on the 10th of next month.
	var (
		expiresAt = utility.NextMonth(time.Now(), 10)
		claims    = SignedPhotoClaims{
			FilenameHash:     FilenameHash(filename),
			Anyone:           anyone,
			RegisteredClaims: encryption.StandardClaims(userID, username, expiresAt),
		}
	)

	// Lock the date stamps for a consistent JWT value for caching.
	claims.IssuedAt = nil
	claims.NotBefore = nil

	log.Debug("createSignedPhotoURL(%s): %+v", filename, claims)

	token, err := encryption.SignClaims(claims, []byte(config.Current.SignedPhoto.JWTSecret))
	if err != nil {
		log.Error("PhotoURL: SignClaims: %s", err)
	}

	// JWT query string to append?
	if token != "" {
		token = "?jwt=" + token
	}

	return URLPath(filename) + token
}
