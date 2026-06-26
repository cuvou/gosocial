package models

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cuvou/gosocial/pkg/chat/flairs"
	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User account table.
type User struct {
	ID             uint64         `gorm:"primaryKey"`
	Username       string         `gorm:"uniqueIndex"`
	Email          string         `gorm:"uniqueIndex"`
	HashedPassword string         `json:"-"`
	IsAdmin        bool           `gorm:"index"`
	Status         UserStatus     `gorm:"index"` // active, disabled
	Visibility     UserVisibility `gorm:"index"` // public, private
	Name           *string
	Birthdate      time.Time
	Explicit       bool      `gorm:"index"` // user has opted-in to see explicit content
	CreatedAt      time.Time `gorm:"index"`
	UpdatedAt      time.Time `gorm:"index"`
	LastLoginAt    time.Time `gorm:"index"`

	// Relational tables.
	ProfilePhotoID *uint64
	ProfilePhoto   Photo         `gorm:"foreignKey:profile_photo_id"`
	AdminGroups    []*AdminGroup `gorm:"many2many:admin_group_users;" json:"-"`

	// Current user's relationship to this user -- not stored in DB.
	UserRelationship UserRelationship `gorm:"-"`

	// Caches
	cachePhotoTypes     map[PhotoVisibility]struct{}
	cacheBlockedUserIDs []uint64
	cachePhotoIDs       []uint64
	cacheProfileFields  map[string]*ProfileField

	// Feature mutexes.
	muStatistic sync.Mutex
}

type UserVisibility string

const (
	UserVisibilityPublic   UserVisibility = "public"
	UserVisibilityExternal UserVisibility = "external"
	UserVisibilityPrivate  UserVisibility = "private"
)

// All visibility options.
var UserVisibilityOptions = []UserVisibility{
	UserVisibilityPublic,
	UserVisibilityExternal,
	UserVisibilityPrivate,
}

// Preload related tables for the user (classmethod).
func (u *User) Preload() *gorm.DB {
	return DB.Preload("ProfilePhoto").Preload("AdminGroups.Scopes")
}

// UserStatus options.
type UserStatus string

const (
	UserStatusActive   = "active"
	UserStatusDisabled = "disabled"
	UserStatusBanned   = "banned"
)

// CreateUser. It is assumed username and email are correctly formatted.
func CreateUser(username, email, password string) (*User, error) {
	// Verify username and email are unique.
	if _, err := FindUsername(username); err == nil {
		return nil, errors.New("That username already exists. Please try a different username.")
	} else if _, err := FindUsernameOrEmail(email); err == nil {
		return nil, errors.New("That email address is already registered.")
	}

	u := &User{
		Username:   username,
		Email:      email,
		Status:     UserStatusActive,
		Visibility: UserVisibilityPublic,
	}

	if err := u.HashPassword(password); err != nil {
		return nil, err
	}

	result := DB.Create(u)
	return u, result.Error
}

// GetUser by ID.
func GetUser(userId uint64) (*User, error) {
	user := &User{}
	result := user.Preload().First(&user, userId)
	return user, result.Error
}

// GetUsers queries for multiple user IDs and returns users in the same order.
func GetUsers(currentUser *User, userIDs []uint64) ([]*User, error) {
	userMap, err := MapUsers(currentUser, userIDs)
	if err != nil {
		return nil, err
	}

	// Re-order them per the original sequence.
	var users = []*User{}
	for _, uid := range userIDs {
		if user, ok := userMap[uid]; ok {
			users = append(users, user)
		}
	}

	return users, nil
}

// GetUsersAlphabetically queries for multiple user IDs and returns them sorted by username.
//
// Note: it doesn't respect blocked lists or a viewer context. Used for things like the forum moderators lists.
func GetUsersAlphabetically(userIDs []uint64) ([]*User, error) {
	var (
		users  = []*User{}
		result = (&User{}).Preload().Where(
			"id IN ?", userIDs,
		).Order("username asc").Find(&users)
	)

	// Inject user relationships.
	for _, user := range users {
		SetUserRelationships(user, users)
	}

	return users, result.Error
}

// GetUsersByUsernames queries for multiple usernames and returns users in the same order.
func GetUsersByUsernames(currentUser *User, usernames []string) ([]*User, error) {
	// Map the usernames.
	var (
		usermap  = map[string]*User{}
		set      = map[string]interface{}{}
		distinct = []string{}
	)

	// Uniqueify usernames.
	for _, name := range usernames {
		if _, ok := set[name]; ok {
			continue
		}
		set[name] = nil
		distinct = append(distinct, name)
	}

	var (
		users  = []*User{}
		result = (&User{}).Preload().Where("username IN ?", distinct).Find(&users)
	)
	if result.Error != nil {
		return nil, result.Error
	}

	// Map users.
	for _, user := range users {
		usermap[user.Username] = user
	}

	// Inject relationships.
	SetUserRelationships(currentUser, users)

	// Re-order them per the original sequence.
	var ordered = []*User{}
	for _, name := range usernames {
		if user, ok := usermap[name]; ok {
			ordered = append(ordered, user)
		}
	}

	return ordered, nil
}

// FindUsername finds a user by their username, for most public lookups by the site.
func FindUsername(username string) (*User, error) {
	if username == "" {
		return nil, errors.New("username is required")
	}

	u := &User{}
	result := u.Preload().Where("username = ?", username).Limit(1).First(u)
	return u, result.Error
}

// FindUsernameOrEmail searches for a user by username or e-mail, e.g. for login and admin search endpoints.
func FindUsernameOrEmail(username string) (*User, error) {
	if strings.ContainsRune(username, '@') {
		u := &User{}
		result := u.Preload().Where("email = ?", username).Limit(1).First(u)
		return u, result.Error
	}

	return FindUsername(username)
}

// IsValidUsername checks if a username is available and not reserved.
func IsValidUsername(username string) error {
	// Check the formatting of the name.
	if !config.UsernameRegexp.MatchString(username) {
		return errors.New("Your username must consist of only numbers, letters, - . and be 3-32 characters.")
	}

	// Reserved username check.
	for _, cmp := range config.ReservedUsernames {
		if username == cmp {
			return errors.New("That username is reserved, please choose a different username.")
		}
	}

	// No static file extension on usernames.
	if ext := filepath.Ext(username); ext != "" {
		ext = strings.ToLower(strings.TrimPrefix(ext, "."))
		for _, cmp := range config.StaticFileExtensions {
			if ext == cmp {
				return fmt.Errorf("Your username suffix looks too much like a file extension: .%s", ext)
			}
		}
	}

	// Does the username already exist?
	if _, err := FindUsername(username); err == nil {
		return errors.New("That username already exists. Please try a different username.")
	}

	return nil
}

// PingLastLoginAt refreshes the user's "last logged in" time.
func (u *User) PingLastLoginAt() error {
	// Also ping their daily active user statistic.
	if err := LogDailyActiveUser(u); err != nil {
		log.Error("PingLastLoginAt(%s): couldn't log daily active user statistic: %s", u.Username, err)
	}

	u.LastLoginAt = time.Now()
	return u.Save()
}

// IsBanned returns if the user account is banned.
func (u *User) IsBanned() bool {
	return u.Status == UserStatusBanned
}

// ChatFlair returns an appropriate custom flair for the chat room.
//
// - If the user is a Shy Account, gets the Shy flair.
// - On the user's birthday, gets the Birthday flair.
//
// If the user is a Supporter who opts to hide their Supporter flair, the NoVIP tag is added.
func (u *User) ChatFlair() flairs.Flair {
	var result = flairs.Flair{}

	if u.IsBirthday() {
		result = config.BirthdayFlair
	}

	return result
}

// CanBeSeenBy checks whether the user can be seen to exist by the viewer.
//
// An admin viewer can always see them, but a user may be hidden to others when they are
// blocking, disabled or banned.
//
// The user should always be given a Not Found page so they can't tell the user even
// exists. The returned error will include a specific reason, for debugging purposes.
//
// May return ErrUsersBlockingEachOther to check if specifically it is due to blocking.
func (u *User) CanBeSeenBy(viewer *User) error {
	if viewer.IsAdmin {
		return nil
	}
	return u.ShouldBeSeenBy(viewer)
}

// ShouldBeSeenBy checks whether the user should be visible to the viewer.
//
// This will e.g. still allow admin users to be blocked by regular users, unless the admin
// has the Unblockable scope.
//
// May return ErrUsersBlockingEachOther to check if specifically it is due to blocking.
func (u *User) ShouldBeSeenBy(viewer *User) error {
	// Banned or disabled? Only admin can view then.
	if u.Status != UserStatusActive {
		return fmt.Errorf("user status is %s", u.Status)
	}

	// Is either one blocking?
	if IsBlocking(viewer.ID, u.ID) {
		return ErrUsersBlockingEachOther
	}

	return nil
}

// ErrUsersBlockingEachOther may be returned by User.ShouldBeSeenBy (and CanBeSeenBy) to
// signal the specific case that it's due to blocklists and not profile status.
var ErrUsersBlockingEachOther = errors.New("users block each other")

// SecurityCheckupEligible checks whether the user should be shown a Security Checkup page
// on their next login. Currently this will check if they have enabled Two Factor Auth, and
// prompt them to do so on a recurring basis.
//
// isLoginEvent is true on the login page, so the user will see the checkpoint once on every
// login even if they have snoozed it. (Only if they have an actionable item such as 2FA setup).
func (u *User) SecurityCheckupEligible(isLoginEvent bool) bool {

	// If they have snoozed their security checkup, skip.
	if !isLoginEvent {
		if ttl := u.GetProfileField("security_checkup_not_before"); ttl != "" {
			if notBefore, err := time.Parse(time.RFC3339Nano, ttl); err == nil {
				if time.Now().Before(notBefore) {
					return false
				}
			}
		}
	} else {
		// On login with password, only remind them once in a while (shorter TTL than the hard interstitial).
		if ttl := u.GetProfileField("security_checkup_not_before_soft"); ttl != "" {
			if notBefore, err := time.Parse(time.RFC3339Nano, ttl); err == nil {
				if time.Now().Before(notBefore) {
					return false
				}
			}
		}
	}

	// Is their 2FA enabled?
	tf := Get2FA(u.ID)
	return !tf.Enabled
}

// UserSearch config.
type UserSearch struct {
	Username       string   // fuzzy search by name or username
	InUsername     []string // exact set of usernames (e.g. On Chat)
	Gender         string
	Orientation    string
	MaritalStatus  string
	HereFor        string
	SpokenLanguage string
	ProfileText    *Search
	NearCity       *WorldCities
	MaxDistance    float64
	LastOnline     int // hours since last login
	IsBanned       bool
	IsDisabled     bool
	IsAdmin        bool // search for admin users
	IsAllUsers     bool // admin 'All users' search (ignores status)
	Friends        bool
	Followers      bool
	Following      bool
	Liked          bool
	AgeMin         int
	AgeMax         int
}

// SearchUsers from the perspective of a given user.
func SearchUsers(user *User, search *UserSearch, pager *Pagination) ([]*User, error) {
	if search == nil {
		search = &UserSearch{}
	}

	var (
		users        = []*User{}
		query        *gorm.DB
		joins        string // GPS location join.
		wheres       = []string{}
		placeholders = []any{}
		myLocation   = GetUserLocation(user.ID)
	)

	// Sort by distance? Requires PostgreSQL.
	if pager.Sort == "distance" || search.NearCity != nil {
		if !config.Current.Database.IsPostgres {
			return users, errors.New("ordering by distance requires PostgreSQL with the PostGIS extension")
		}

		// If the current user doesn't have their location on file, they can't do this.
		if myLocation.IsEmpty() {
			return users, errors.New("can not sort members by distance because your location is not known")
		}

		// Which location to search from?
		var (
			latitude, longitude = myLocation.Latitude, myLocation.Longitude
		)
		if search.NearCity != nil {
			latitude = search.NearCity.Latitude
			longitude = search.NearCity.Longitude
		}

		// Only query for users who have locations.
		joins = "JOIN user_locations ON (user_locations.user_id = users.id)"
		wheres = append(wheres,
			"user_locations.latitude IS NOT NULL",
			"user_locations.longitude IS NOT NULL",
			"user_locations.latitude <> 0",
			"user_locations.longitude <> 0",
		)

		pager.Sort = fmt.Sprintf(`ST_Distance(
				ST_MakePoint(user_locations.longitude, user_locations.latitude)::geography,
				ST_MakePoint(%f, %f)::geography)`,
			longitude, latitude,
		)

		// Apply a distance limiter too?
		if search.MaxDistance > 0 {
			wheres = append(wheres, fmt.Sprintf(`ST_Distance(
				ST_MakePoint(user_locations.longitude, user_locations.latitude)::geography,
				ST_MakePoint(%f, %f)::geography
			) <= ?`, longitude, latitude))
			placeholders = append(placeholders, search.MaxDistance*1000)
		}
	}

	// Blocking user IDs?
	bw, bp := BlockedUserSubquery("users.id", user.ID)
	wheres = append(wheres, bw)
	placeholders = append(placeholders, bp...)

	if search.Username != "" {
		ilike := "%" + strings.TrimSpace(strings.ToLower(search.Username)) + "%"

		if user.IsAdmin {
			// Admins can use the Username field to do an exact-match on email address.
			wheres = append(wheres, "(users.username LIKE ? OR users.name ILIKE ? OR users.email = ?)")
			placeholders = append(placeholders, ilike, ilike, search.Username)
		} else {
			// Others can only search by name or username.
			wheres = append(wheres, "(users.username LIKE ? OR users.name ILIKE ?)")
			placeholders = append(placeholders, ilike, ilike)
		}
	}

	if len(search.InUsername) > 0 {
		wheres = append(wheres, "users.username IN ?")
		placeholders = append(placeholders, search.InUsername)
	}

	if search.Gender != "" {
		switch search.Gender {
		case "Trans":
			// Include subcategories.
			wheres = append(wheres, `
				EXISTS (
					SELECT 1 FROM profile_fields
					WHERE user_id = users.id AND name = ? AND value IN ?
				)

				-- Privacy opt-outs.
				AND privacy_settings.hidden_gender IS NOT TRUE
			`)
			placeholders = append(placeholders, "gender", config.TransGender)
		default:
			wheres = append(wheres, `
				EXISTS (
					SELECT 1 FROM profile_fields
					WHERE user_id = users.id AND name = ? AND value = ?
				)

				-- Privacy opt-outs.
				AND privacy_settings.hidden_gender IS NOT TRUE
			`)
			placeholders = append(placeholders, "gender", search.Gender)
		}
	}

	if search.Orientation != "" {
		wheres = append(wheres, `
			EXISTS (
				SELECT 1 FROM profile_fields
				WHERE user_id = users.id AND name = ? AND value = ?
			)

			-- Privacy opt-outs.
			AND privacy_settings.hidden_orientation IS NOT TRUE
		`)
		placeholders = append(placeholders, "orientation", search.Orientation)
	}

	if search.MaritalStatus != "" {
		wheres = append(wheres, `
			EXISTS (
				SELECT 1 FROM profile_fields
				WHERE user_id = users.id AND name = ? AND value = ?
			)
		`)
		placeholders = append(placeholders, "marital_status", search.MaritalStatus)
	}

	if search.HereFor != "" {
		wheres = append(wheres, `
			EXISTS (
				SELECT 1 FROM profile_fields
				WHERE user_id = users.id AND name = ? AND value LIKE ?
			)
		`)
		placeholders = append(placeholders, "here_for", "%"+search.HereFor+"%")
	}

	if search.SpokenLanguage != "" {
		wheres = append(wheres, `
			EXISTS (
				SELECT 1 FROM profile_fields
				WHERE user_id = users.id AND name = ? AND value LIKE ?
			)
		`)
		placeholders = append(placeholders, "spoken_languages", "%"+search.SpokenLanguage+"%")
	}

	if search.LastOnline > 0 {
		wheres = append(wheres, `
			users.last_login_at > ?
		`)
		placeholders = append(placeholders, time.Now().Add(time.Duration(-search.LastOnline)*time.Hour))
	}

	// Profile text search.
	if terms := search.ProfileText; terms != nil {
		for _, term := range terms.Includes {
			var ilike = "%" + strings.ToLower(term) + "%"
			wheres = append(wheres, `
				EXISTS (
					SELECT 1 FROM profile_fields
					WHERE user_id = users.id AND name IN ? AND value ILIKE ?
				)
			`)
			placeholders = append(placeholders, config.EssayProfileFields, ilike)
		}
		for _, term := range terms.Excludes {
			var ilike = "%" + strings.ToLower(term) + "%"
			wheres = append(wheres, `
				NOT EXISTS (
					SELECT 1 FROM profile_fields
					WHERE user_id = users.id AND name IN ? AND value ILIKE ?
				)
			`)
			placeholders = append(placeholders, config.EssayProfileFields, ilike)
		}
	}

	// Only admin user can show disabled/banned users.
	var statuses = []string{}
	if user.HasAdminScope(config.ScopeUserBan) {
		if search.IsAllUsers {
			// No status filter for this search.
		} else if search.IsBanned {
			statuses = append(statuses, UserStatusBanned)
		} else if search.IsDisabled {
			statuses = append(statuses, UserStatusDisabled)
		} else {
			statuses = append(statuses, UserStatusActive)
		}
	}

	// Non-admin user only ever sees active accounts.
	if user.HasAdminScope(config.ScopeUserBan) {
		if len(statuses) > 0 && !search.IsAllUsers {
			wheres = append(wheres, "users.status IN ?")
			placeholders = append(placeholders, statuses)
		}
	} else {
		wheres = append(wheres, "users.status = ?")
		placeholders = append(placeholders, UserStatusActive)
	}

	if search.IsAdmin {
		wheres = append(wheres, "users.is_admin = true")
	}

	if search.Friends {
		wheres = append(wheres, `
			EXISTS (
				SELECT 1 FROM friends
				WHERE source_user_id = ?
				AND target_user_id = users.id
				AND approved = ?
			)
		`)
		placeholders = append(placeholders,
			user.ID,
			true,
		)
	}

	if search.Followers {
		wheres = append(wheres, `
			EXISTS (
				SELECT 1
				FROM follows
				WHERE follows.source_user_id = users.id
				AND follows.target_user_id = ?
			)
		`)
		placeholders = append(placeholders, user.ID)
	} else if search.Following {
		wheres = append(wheres, `
			EXISTS (
				SELECT 1
				FROM follows
				WHERE follows.target_user_id = users.id
				AND follows.source_user_id = ?
			)
		`)
		placeholders = append(placeholders, user.ID)
	}

	if search.Liked {
		wheres = append(wheres, `
			EXISTS (
				SELECT 1 FROM likes
				WHERE user_id = ?
				AND table_name = 'users'
				AND table_id = users.id
			)
		`)
		placeholders = append(placeholders,
			user.ID,
		)
	}

	if search.AgeMin > 0 {
		date := time.Now().AddDate(-search.AgeMin, 0, 0)
		wheres = append(wheres, `
			birthdate <= ? AND NOT EXISTS (
				SELECT 1
				FROM profile_fields
				WHERE user_id = users.id
				AND name = 'hide_age'
				AND value = 'true'
			)

			-- Privacy opt-outs.
			AND privacy_settings.hidden_age IS NOT TRUE
		`)
		placeholders = append(placeholders, date)
	}

	if search.AgeMax > 0 {
		date := time.Now().AddDate(-search.AgeMax-1, 0, 0)
		wheres = append(wheres, `
			birthdate >= ? AND NOT EXISTS (
				SELECT 1
				FROM profile_fields
				WHERE user_id = users.id
				AND name = 'hide_age'
				AND value = 'true'
			)

			-- Privacy opt-outs.
			AND privacy_settings.hidden_age IS NOT TRUE
		`)
		placeholders = append(placeholders, date)
	}

	query = (&User{}).Preload().Joins(
		"LEFT JOIN privacy_settings ON (privacy_settings.user_id = users.id)",
	)
	if joins != "" {
		query = query.Joins(joins)
	}
	query = query.Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	).Order(pager.Sort)
	query.Model(&User{}).Count(&pager.Total)
	result := query.Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&users)

	// Inject relationship booleans.
	SetUserRelationships(user, users)

	return users, result.Error
}

// UserMap helps map a set of users to look up by ID.
type UserMap map[uint64]*User

// MapUsers looks up a set of user IDs in bulk and returns a UserMap suitable for templates.
// Useful to avoid circular reference issues with Photos especially; the Site Gallery queries
// photos of ALL users and MapUsers helps stitch them together for the frontend.
func MapUsers(user *User, userIDs []uint64) (UserMap, error) {
	var (
		usermap      = UserMap{}
		set          = map[uint64]any{}
		distinct     = []uint64{}
		wheres       = []string{}
		placeholders = []any{}
	)

	if len(userIDs) == 0 {
		return usermap, nil
	}

	// Uniqueify users.
	for _, uid := range userIDs {
		if _, ok := set[uid]; ok {
			continue
		}
		set[uid] = nil
		distinct = append(distinct, uid)
	}

	wheres = append(wheres, "users.id IN ?")
	placeholders = append(placeholders, distinct)

	// Exclude banned or disabled users (except when admin view).
	if user != nil && !user.IsAdmin {
		wheres = append(wheres, "users.status = ?")
		placeholders = append(placeholders, UserStatusActive)
	}

	var (
		users  = []*User{}
		result = (&User{}).Preload().Where(
			strings.Join(wheres, " AND "),
			placeholders...,
		).Find(&users)
	)

	// Inject user relationships.
	if user != nil {
		SetUserRelationships(user, users)
	}

	if result.Error == nil {
		for _, row := range users {
			usermap[row.ID] = row
		}
	}

	return usermap, result.Error
}

// MapAdminUsers returns a MapUsers result for all admin user accounts on the site.
func MapAdminUsers(user *User) (UserMap, error) {
	adminUsers, err := ListAdminUsers()
	if err != nil {
		return nil, err
	}

	var userIDs = []uint64{}
	for _, user := range adminUsers {
		userIDs = append(userIDs, user.ID)
	}
	return MapUsers(user, userIDs)
}

// CountBlockedAdminUsers returns a count of how many admin users the current user has blocked, out of how many total.
func CountBlockedAdminUsers(user *User) (count, total int64) {
	// Count the blocked admins.
	DB.Model(&User{}).Select(
		"count(users.id) AS cnt",
	).Joins(
		"JOIN blocks ON (blocks.target_user_id = users.id)",
	).Where(
		"blocks.source_user_id = ? AND users.is_admin IS TRUE",
		user.ID,
	).Count(&count)

	// And the total number of available admins.
	total = CountAdminUsers()
	return
}

// CountAdminUsers returns a count of how many admin users exist on the site.
func CountAdminUsers() (count int64) {
	DB.Model(&User{}).Select(
		"count(id) AS cnt",
	).Where(
		"users.is_admin IS TRUE",
	).Count(&count)
	return
}

// Has a user ID in the map?
func (um UserMap) Has(id uint64) bool {
	_, ok := um[id]
	return ok
}

// Get a user from the UserMap.
func (um UserMap) Get(id uint64) *User {
	if user, ok := um[id]; ok {
		return user
	}
	return nil
}

// MapUsersByUsername looks up a set of users in bulk and returns a UsernameMap suitable for templates.
//
// It is like MapUsers but by username instead of ID.
func MapUsersByUsername(usernames []string) (UsernameMap, error) {
	var (
		usermap  = UsernameMap{}
		set      = map[string]interface{}{}
		distinct = []string{}
	)

	// Uniqueify users.
	for _, uid := range usernames {
		if _, ok := set[uid]; ok {
			continue
		}
		set[uid] = nil
		distinct = append(distinct, uid)
	}

	var (
		users  = []*User{}
		result = (&User{}).Preload().Where("username IN ?", distinct).Find(&users)
	)

	if result.Error == nil {
		for _, row := range users {
			usermap[row.Username] = row
		}
	}

	// Assert we got the expected count.
	if len(usermap) != len(distinct) {
		return usermap, fmt.Errorf("didn't get all expected users (expected %d, got %d)", len(distinct), len(usermap))
	}

	return usermap, result.Error
}

// UsernameMap helps map a set of users to look up by ID.
type UsernameMap map[string]*User

// NameOrUsername returns the name (if not null or empty) or the username.
func (u *User) NameOrUsername() string {
	if u.Name != nil && len(*u.Name) > 0 {
		return *u.Name
	} else {
		return u.Username
	}
}

// CanSeeProfilePicture returns whether the current user can see the user's profile picture.
//
// Returns a boolean (false if currentUser can't see) and the Visibility setting of the profile photo.
//
// If the user has no profile photo, returns (false, PhotoPublic) which should manifest as the blue shy.png placeholder image.
func (u *User) CanSeeProfilePicture(currentUser *User) (bool, PhotoVisibility) {
	// Photo owner can always see.
	if currentUser != nil && currentUser.ID == u.ID {
		return true, PhotoPublic
	}

	// No profile picture set at all?
	if u.ProfilePhoto.ID == 0 {
		return false, PhotoPublic
	}

	if !u.UserRelationship.Computed && currentUser != nil {
		SetUserRelationships(currentUser, []*User{u})
	}

	// Blocking?
	if u.UserRelationship.IsBlocked {
		return false, PhotoPublic
	}

	visibility := u.ProfilePhoto.Visibility
	if visibility == PhotoPrivate && !u.UserRelationship.IsPrivateGranted {
		// Private photo
		return false, visibility
	} else if visibility == PhotoFriends && !u.UserRelationship.IsFriend {
		// Friends only
		return false, visibility
	} else if u.ProfilePhoto.CroppedFilename != "" {
		// Happy path. Is the photo Explicit?
		if u.ProfilePhoto.Explicit && !currentUser.Explicit {
			return false, PhotoPublic
		}
		return true, visibility
	}
	return false, PhotoPublic
}

// IsProfilePictureVisibleTo returns a simple boolean if the current user can see it.
//
// This boolean is easier to test in template code with the single return value.
func (u *User) IsProfilePictureVisibleTo(currentUser *User) bool {
	ok, _ := u.CanSeeProfilePicture(currentUser)
	return ok
}

// HashPassword sets the user's hashed (bcrypt) password.
func (u *User) HashPassword(password string) error {
	if trimmed := strings.TrimSpace(password); trimmed != password {
		return errors.New("password should not have space padding")
	}

	passwd, err := bcrypt.GenerateFromPassword([]byte(password), config.BcryptCost)
	if err != nil {
		return err
	}
	u.HashedPassword = string(passwd)
	return nil
}

// SaveNewPassword updates a user's password and saves their record to the database.
func (u *User) SaveNewPassword(password string) error {
	if err := u.HashPassword(password); err != nil {
		return err
	}
	return u.Save()
}

// CheckPassword verifies the password is correct. Returns nil on success.
func (u *User) CheckPassword(password string) error {
	return bcrypt.CompareHashAndPassword([]byte(u.HashedPassword), []byte(password))
}

// IsBirthday returns whether it is currently the user's birthday (+- a day for time zones).
func (u *User) IsBirthday() bool {
	if u.Birthdate.IsZero() {
		return false
	}

	// Window of time to be valid.
	var (
		now, _    = time.Parse(time.DateOnly, time.Now().Format(time.DateOnly))
		bday, _   = time.Parse(time.DateOnly, fmt.Sprintf("%d-%02d-%02d", now.Year(), u.Birthdate.Month(), u.Birthdate.Day()))
		startDate = now.Add(-6 * time.Hour)
		endDate   = now.Add(60 * time.Hour)
	)

	return bday.Before(endDate) && bday.After(startDate)
}

// RemoveProfilePhoto sets profile_photo_id=null to unset the foreign key.
func (u *User) RemoveProfilePhoto() error {
	result := DB.Model(&User{}).Where("id = ?", u.ID).Update("profile_photo_id", nil)
	return result.Error
}

// Save user.
func (u *User) Save() error {
	result := DB.Save(u)
	return result.Error
}

// Delete a user. NOTE: use the models/deletion/DeleteUser() function
// instead of this to do a deep scrub of all related data!
func (u *User) Delete() error {
	return DB.Delete(u).Error
}

// Print user object as pretty JSON.
func (u *User) Print() string {
	var (
		buf bytes.Buffer
		enc = json.NewEncoder(&buf)
	)
	enc.SetIndent("", "    ")
	enc.Encode(u)
	return buf.String()
}
