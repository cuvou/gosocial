package claims

import (
	"net/http"
	"time"

	"github.com/cuvou/gosocial/pkg/chat/flairs"
	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/encryption"
	"github.com/cuvou/gosocial/pkg/geoip"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/photo"
	"github.com/golang-jwt/jwt/v4"
)

// Claims are the JWT claims for the BareRTC chat room.
type Claims struct {
	// Custom claims.
	IsAdmin    bool         `json:"op,omitempty"`
	VIP        bool         `json:"vip,omitempty"`
	Avatar     string       `json:"img,omitempty"`
	ProfileURL string       `json:"url,omitempty"`
	Nickname   string       `json:"nick,omitempty"`
	Emoji      string       `json:"emoji,omitempty"`
	Gender     string       `json:"gender,omitempty"`
	Flair      flairs.Flair `json:"flair,omitempty"`
	Rules      []string     `json:"rules,omitempty"`

	// Standard claims. Notes:
	// subject = username
	jwt.RegisteredClaims
}

// Gender returns the BareRTC gender string for the user's gender selection.
func Gender(u *models.User) string {
	switch u.GetProfileField("gender") {
	case "Man", "Trans (FTM)":
		return "m"
	case "Woman", "Trans (MTF)":
		return "f"
	case "Non-binary", "Trans", "Other":
		return "o"
	default:
		return ""
	}
}

// GetChatFlagEmoji tries to fetch a chat flag emoji string for the current user.
//
// If http.Request is nil, it will only rely on the user's Location Settings or return a fallback flag.
func GetChatFlagEmoji(r *http.Request, user *models.User) string {
	var (
		fallback = "🏳️ Not Available"
		emoji    = models.GetUserLocationPrettyEmojiString(user.ID)
	)

	if emoji != "" {
		return emoji
	}

	// Do we have an HTTP request to fall back on?
	if r == nil {
		return fallback
	}

	if emoji, err := geoip.GetChatFlagEmoji(r); err == nil {
		return emoji
	}

	if emoji, err := geoip.CountryFlagEmojiWithCode("US"); err == nil {
		return emoji
	}

	return fallback
}

// SignClaims signs the JWT claims to log a user into the chat room.
func SignClaims(currentUser *models.User, emoji string, flair flairs.Flair) (Claims, string, error) {

	// Avatar URL - masked if non-public.
	avatar := photo.SignedPublicAvatarURL(currentUser.ProfilePhoto.CroppedFilename)
	switch currentUser.ProfilePhoto.Visibility {
	case models.PhotoPrivate:
		avatar = "/static/img/shy-private.png"
	case models.PhotoFriends:
		avatar = "/static/img/shy-friends.png"
	}

	// Explicit pictures show the placeholder graphic.
	if currentUser.ProfilePhoto.Explicit {
		avatar = "/static/img/shy-explicit.png"
	}

	// Apply chat moderation rules.
	var rules = []string{}

	// VIP user? (Paid supporter tier)
	var isVIP bool

	// Create the JWT claims.
	claims := Claims{
		IsAdmin:          currentUser.HasAdminScope(config.ScopeChatModerator),
		Avatar:           avatar,
		ProfileURL:       "/u/" + currentUser.Username,
		Nickname:         currentUser.NameOrUsername(),
		Emoji:            emoji,
		Gender:           Gender(currentUser),
		VIP:              isVIP,
		Rules:            rules,
		Flair:            flair,
		RegisteredClaims: encryption.StandardClaims(currentUser.ID, currentUser.Username, time.Now().Add(5*time.Minute)),
	}

	token, err := encryption.SignClaims(claims, []byte(config.Current.BareRTC.JWTSecret))
	return claims, token, err
}
