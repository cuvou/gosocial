package models

// PrivacySetting table stores a user's privacy settings in a convenient joinable location.
type PrivacySetting struct {
	UserID uint64 `gorm:"primaryKey"`

	// Interaction Preferences.
	FirstMessages string // Who can slide into my DMs?
	PhotoComments string // Who can comment on my photos?
	PrivatePhotos string // Who can share their private pics with me?
	FriendsList   string // Who can see my Friends list?
	FollowMe      string // Who can follow me?

	// Member Directory Search Opt-outs.
	HiddenAge         bool `gorm:"index"`
	HiddenGender      bool `gorm:"index"`
	HiddenOrientation bool `gorm:"index"`
}

// IsZero returns if the privacy settings are all blank/default.
func (ps *PrivacySetting) IsZero() bool {
	var (
		enums = ps.FirstMessages + ps.FollowMe + ps.PhotoComments + ps.PrivatePhotos + ps.FriendsList
		bools = ps.HiddenAge || ps.HiddenGender || ps.HiddenOrientation
	)
	return enums == "" && !bools
}

// GetPrivacySetting loads a user's privacy settings or returns the default
// struct if they have none stored in the database.
func GetPrivacySetting(userID uint64) *PrivacySetting {
	var (
		ps     = &PrivacySetting{}
		result = DB.First(&ps, userID)
	)
	if result.Error != nil {
		return &PrivacySetting{
			UserID: userID,
		}
	}
	return ps
}

// Save privacy settings.
//
// If all settings are blank/empty, the row will be deleted instead.
func (ps *PrivacySetting) Save() error {
	if ps.IsZero() {
		return ps.Delete()
	}
	return DB.Save(ps).Error
}

// Delete privacy settings.
func (ps *PrivacySetting) Delete() error {
	return DB.Delete(ps).Error
}
