// Package config holds some (mostly static) configuration for the app.
package config

import (
	"regexp"
	"time"
)

// Branding
const (
	Title      = "GoSocial"
	Subtitle   = "A fully featured social networking website."
	WebsiteURL = "https://gosocial.cuvou.com" // No trailing slash!

	// Pretty version of your title (e.g. with HTML tags for color/style).
	// Used with the {{PrettyTitle}} macro in frontend templates.
	PrettyTitle = `<strong style="color: #FF7700">Go</strong>` +
		`<strong style="color: #0077FF">Social</strong>`

	// BBCode special tag to insert the PrettyTitle.
	BBCodePrettyTitle = "[gosocial]"

	// Default website color scheme to use
	// (one of the values from WebsiteThemeHueChoices in config/enum.go)
	// A blank value "" will use the default Bulma CSS theme.
	DefaultWebsiteThemeHue = "blue-pink"
)

// Main branding PrettyTitle. This is a function so you can set custom
// seasonal stylings based on the date, etc. This is used only in the
// site's main top nav bar.
func PrettyTitleBranding() string {
	// The main title in the corner of the nav bar.
	if now := time.Now(); now.Month() == 4 && now.Day() == 1 {
		// April Fool's, flip the colors.
		return `<strong style="color: #0077FF">Go</strong>` +
			`<strong style="color: #FF7700">Social</strong>`
	}
	return PrettyTitle
}

// Paths and layouts
const (
	TemplatePath = "./web/templates"
	StaticPath   = "./web/static"
	SettingsPath = "./settings.json"

	// Web path where photos are kept. Photos in DB store only their filenames, this
	// is the base URL that goes in front. TODO: support setting a CDN URL prefix.
	PhotoWebPath  = "/static/photos"
	PhotoDiskPath = "./web/static/photos"
)

// Feature Flags
const (
	// 'Explicit' (NSFW) content supported.
	// Note: if you disable this after Explicit content was already uploaded, the content may still
	// be visible to users who've opted-in to see Explicit content on their settings while it was enabled.
	// It is best to decide up front if your website will support Explicit content; disabling this feature
	// will hide various UI elements making it difficult to toggle Explicit settings on content after.
	FeatureFlagExplicitEnabled = true
)

// Regular expressions to match URLs within your website.
var (
	// PhotoURLRegexp describes an image path under "/static/photos" that can be parsed from Markdown or HTML input.
	// It is used by e.g. the ReSignURLs function - if you move image URLs to a CDN this may need updating.
	PhotoURLRegexp = regexp.MustCompile(`(?:['"])(/static/photos/[^'"\s?]+(?:\?[^'"\s]*)?)(?:['"]|[^'"\s]*)`)

	// Photo permalink pages.
	PhotoPermalinkRegexp = regexp.MustCompile(`/photo/view\?[A-Za-z0-9=&#]+`)
)

const (
	// Security and password settings.
	BcryptCost              = 14
	SessionCookieName       = "session_id"
	SessionSecretCookieName = "client_secret"
	CSRFCookieName          = "xsrf_token"
	CSRFInputName           = "_csrf" // html input name
	SessionCookieMaxAge     = 60 * 60 * 24 * 30
	SessionRedisKeyFormat   = "session/%s"

	// Max upload size (e.g. 10 MB GIFs)
	MaxBodyMegaBytes   = 10
	MaxBodyBytes       = 1024 * 1024 * MaxBodyMegaBytes
	MultipartMaxMemory = 1024 * 1024 * 1024 * 20 // 20 MB

	// Max size for long indexed fields, such as blog post bodies.
	// Posts longer than this will be stored in non-indexed fields.
	MaxDatabaseStringIndexSize = 2048

	TwoFactorBackupCodeCount  = 12
	TwoFactorBackupCodeLength = 8 // characters a-z0-9

	// Signed URLs for static photo authentication.
	SignedPhotoJWTExpires        = 30 * time.Second   // Regular, per-user, short window
	SignedPublicAvatarJWTExpires = 7 * 24 * time.Hour // Widely public, e.g. chat room
	SignedPublicAvatarUsername   = "@"                // JWT 'username' for widely public JWT

	// Security Checkup cooldown period, to remind users to enable 2FA.
	SecurityCheckupCooldownDaysHard = 180                // cooldown for hard interstitial
	SecurityCheckupCooldownDaysSoft = 14                 // cooldown for soft (on login) interstitial
	SecurityCheckupMinAccountAge    = 7 * 24 * time.Hour // min. account age before first checkup

	// Cooldown for admin access to user messages.
	AdminReaderCooldown = 4 * time.Hour
)

// Authentication
const (
	// Skip the email verification step. The signup page will directly ask for
	// email+username+password rather than only email and needing verification.
	SkipEmailVerification = false

	SignupTokenRedisKey   = "signup-token/%s"
	ResetPasswordRedisKey = "reset-password/%s"
	ChangeEmailRedisKey   = "change-email/%s"
	SignupTokenExpires    = 24 * time.Hour // used for all tokens so far

	// How to rate limit same types of emails being delivered, e.g.
	// signups, cert approvals (double post), etc.
	EmailDebounceDefault       = 24 * time.Hour // default debounce per type of email
	EmailDebounceResetPassword = 4 * time.Hour  // "forgot password" emails debounce

	// Rate limits
	RateLimitRedisKey        = "rate-limit/%s/%s" // namespace, id
	LoginRateLimitWindow     = 1 * time.Hour
	LoginRateLimit           = 10 // 10 failed login attempts = locked for full hour
	LoginRateLimitCooldownAt = 3  // 3 failed attempts = start throttling
	LoginRateLimitCooldown   = 30 * time.Second

	// 2FA rate limit. If the user gets the correct password, the regular login rate limit
	// is reset, but if they are met with a 2FA prompt, don't allow them to spam guesses.
	TwoFactorRateLimitWindow     = 1 * time.Hour
	TwoFactorRateLimit           = 10
	TwoFactorRateLimitCooldownAt = 3
	TwoFactorRateLimitCooldown   = 30 * time.Second

	// Contact form rate limits for logged-out users to curb spam robots:
	// - One message can be submitted every 2 minutes
	// - If they post 10 minutes in an hour they are paused for one hour.
	ContactRateLimitWindow     = 15 * time.Minute
	ContactRateLimit           = 10
	ContactRateLimitCooldownAt = 5
	ContactRateLimitCooldown   = time.Minute

	// Rate limit Blocklist additions, to discourage those who 'block the whole entire website' in
	// order to whitelist the very few they approve of.
	AddBlocklistRateLimitWindow     = 6 * time.Hour
	AddBlocklistRateLimit           = 20 // 20 total blocks can be added per 6 hours
	AddBlocklistRateLimitCooldownAt = 10 // 10 free blocks until rate limiting for 10 mins per block
	AddBlocklistRateLimitCooldown   = 10 * time.Minute

	// How frequently to refresh LastLoginAt since sessions are long-lived.
	LastLoginAtCooldown = time.Hour

	// Chat room status refresh interval.
	ChatStatusRefreshInterval = 30 * time.Second

	// Emergency Kill Switch check interval
	KillSwitchCheckInterval = 12 * time.Hour

	// Expired status message check interval.
	ExpiredStatusMessageCheckInterval = 15 * time.Minute

	// Cache TTL for the demographics page.
	DemographicsCacheTTL = time.Hour
)

var (
	UsernameRegexp    = regexp.MustCompile(`^[a-z0-9_.-]{3,32}$`)
	ReservedUsernames = []string{
		"admin",
		"admins",
		"administrator",
		"moderator",
		"support",
		"staff",
		"here",
		"all",
		"everyone",
		"everybody",
	}
)

// Photo Galleries
const (
	MaxPhotoWidth     = 1280
	ProfilePhotoWidth = 512
	AltTextMaxLength  = 5000

	// Quotas for uploaded photos (file size on disk).
	MediaQuotaMaxLimit = 1024 * 1024 * 25 // 25 MB

	// Rate limit for too many Site Gallery pictures.
	// Some users sign up and immediately max out their gallery and spam
	// the Site Gallery page. These limits can ensure only a few Site Gallery
	// pictures can be posted per day.
	SiteGalleryRateLimitMax      = 5
	SiteGalleryRateLimitInterval = 24 * time.Hour

	// Only ++ the Views count per user per photo within a small
	// window of time - if a user keeps reloading the same photo
	// rapidly it does not increment the view counter more.
	PhotoViewDebounceRedisKey = "debounce-view/user=%d/photoid=%d"
	PhotoViewDebounceCooldown = 1 * time.Hour

	// Save Gallery Photos in WebP format instead of jpeg?
	// WebP files can be on average 50% smaller for equivalent visual quality
	// as jpeg, saving on your storage and bandwidth costs.
	// Set to false to save in jpeg.
	PhotoGallerySaveAsWebP = true

	// JPEG and WebP compression settings.
	JpegQuality                   = 90
	WebPCompressionFactor float32 = 90

	// Max amount of users that can be tagged in a photo.
	MaxTaggedUsers = 20
)

// Forum settings
const (
	// Only ++ the Views count per user per thread within a small
	// window of time - if a user keeps reloading the same thread
	// rapidly it does not increment the view counter more.
	ThreadViewDebounceRedisKey = "debounce-view/user=%d/thr=%d"
	ThreadViewDebounceCooldown = 1 * time.Hour

	// Enable user-owned forums (feature flag)
	UserForumsEnabled = true
)

// Poll settings
var (
	// Max number of responses to accept for a poll (how many form
	// values the app will read in). NOTE: also enforced in frontend
	// UX in new_post.html, update there if you change this.
	PollMaxAnswers = 100

	// Poll color CSS classes (Bulma). Plugged in to templates like:
	// <progress class="$CLASS">
	// Values will wrap around for long polls.
	PollProgressBarClasses = []string{
		"progress is-success",
		"progress is-link",
		"progress is-warning",
		"progress is-danger",
		"progress is-primary",
		"progress is-info",
	}
)

// Variables set by main.go to make them readily available.
var (
	RuntimeVersion   string
	RuntimeBuild     string
	RuntimeBuildDate string
	Debug            bool // app is in debug mode
)
