package models

// ProfileTheme table stores a user's custom profile theme settings.
type ProfileTheme struct {
	UserID uint64 `gorm:"primaryKey"`

	// Hero banner settings.
	HeroColorStart   string
	HeroColorEnd     string
	HeroTextDark     bool
	HeroFilename     string
	HeroFilesize     int64
	HeroTransparency float64

	// Profile card settings.
	CardTitleBG   string
	CardTitleFG   string
	CardLinkColor string
	CardLightness string // ""/auto, light, dark, custom
	CardCustomBG  string
	CardCustomFG  string

	// Wallpaper image settings.
	WallpaperFilename string
	WallpaperFilesize int64
}

// GetProfileTheme loads a user's profile theme or returns the default
// struct if they have none stored in the database.
func GetProfileTheme(userID uint64) *ProfileTheme {
	var (
		pt     = &ProfileTheme{}
		result = DB.First(&pt, userID)
	)
	if result.Error != nil {
		return &ProfileTheme{
			UserID: userID,
		}
	}
	return pt
}

// GetCardTextColor returns the text color for profile cards, based on
// the lightness/darkness (or custom) color setting.
//
// If auto/undefined, returns an empty string.
func (pt *ProfileTheme) GetCardTextColor() string {
	switch pt.CardLightness {
	case "light":
		return "#4a4a4a"
	case "dark":
		return "#f5f5f5"
	case "custom":
		return pt.CardCustomFG
	default:
		return ""
	}
}

// GetCardBackgroundColor returns the BG color for profile cards, based on
// the lightness/darkness (or custom) color setting.
//
// If auto/undefined, returns an empty string.
func (pt *ProfileTheme) GetCardBackgroundColor() string {
	switch pt.CardLightness {
	case "light":
		return "#fff"
	case "dark":
		return "#3a3a3a"
	case "custom":
		return pt.CardCustomBG
	default:
		return ""
	}
}

// Save profile theme.
func (pt *ProfileTheme) Save() error {
	return DB.Save(pt).Error
}

// Delete profile theme.
func (pt *ProfileTheme) Delete() error {
	return DB.Delete(pt).Error
}
