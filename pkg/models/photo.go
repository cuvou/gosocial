package models

import (
	"errors"
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/redis"
	"gorm.io/gorm"
)

// Photo table.
type Photo struct {
	ID              uint64 `gorm:"primaryKey"`
	UserID          uint64 `gorm:"index;index:idx_photos_user_visibility"`
	Filename        string
	CroppedFilename string // if cropped, e.g. for profile photo
	IsGif           bool   `gorm:"index"`
	Filesize        int64
	Caption         string
	AltText         string
	Visibility      PhotoVisibility `gorm:"index;index:idx_photos_user_visibility"`
	Gallery         bool            `gorm:"index"` // photo appears in the public gallery (if public)
	Explicit        bool            `gorm:"index"` // is an explicit photo
	Pinned          bool            `gorm:"index"` // user pins it to the front of their gallery
	LikeCount       int64           `gorm:"index"` // cache of 'likes' count
	CommentCount    int64           `gorm:"index"` // cache of comments count
	Views           uint64          `gorm:"index"` // view count
	CreatedAt       time.Time       `gorm:"index"`
	UpdatedAt       time.Time
}

// PhotoVisibility settings.
type PhotoVisibility string

const (
	PhotoPublic  PhotoVisibility = "public"  // on profile page and/or public gallery
	PhotoFriends PhotoVisibility = "friends" // only friends can see it
	PhotoPrivate PhotoVisibility = "private" // private

	// Special visibility in case, on User Gallery view, user applies a filter
	// for friends-only picture but they are not friends with the user.
	PhotoNotAvailable PhotoVisibility = "not_available"
)

// PhotoVisibility preset settings.
var (
	PhotoVisibilityAll = []PhotoVisibility{
		PhotoPublic,
		PhotoFriends,
		PhotoPrivate,
	}

	// Site Gallery visibility for when your friends show up in the gallery.
	// Or: "Friends + Gallery" photos can appear to your friends in the Site Gallery.
	PhotoVisibilityFriends = []string{
		string(PhotoPublic),
		string(PhotoFriends),
	}
)

// CreatePhoto with most of the settings you want (not ID or timestamps) in the database.
func CreatePhoto(tmpl Photo) (*Photo, error) {
	if tmpl.UserID == 0 {
		return nil, errors.New("UserID required")
	}

	p := &Photo{
		UserID:          tmpl.UserID,
		Filename:        tmpl.Filename,
		CroppedFilename: tmpl.CroppedFilename,
		IsGif:           filepath.Ext(tmpl.Filename) == ".mp4",
		Filesize:        tmpl.Filesize,
		Caption:         tmpl.Caption,
		AltText:         tmpl.AltText,
		Visibility:      tmpl.Visibility,
		Gallery:         tmpl.Gallery,
		Pinned:          tmpl.Pinned,
		Explicit:        tmpl.Explicit,
	}

	result := DB.Create(p)
	return p, result.Error
}

// GetPhoto by ID.
func GetPhoto(id uint64) (*Photo, error) {
	p := &Photo{}
	result := DB.First(&p, id)
	return p, result.Error
}

// GetPhotos by an array of IDs, mapped to their IDs.
func GetPhotos(IDs []uint64) (map[uint64]*Photo, error) {
	var (
		mp = map[uint64]*Photo{}
		ps = []*Photo{}
	)

	result := DB.Model(&Photo{}).Where("id IN ?", IDs).Find(&ps)
	for _, row := range ps {
		mp[row.ID] = row
	}

	return mp, result.Error
}

// CanBeEditedBy checks whether a photo can be edited by the current user.
//
// Admins with PhotoModerator scope can always edit.
func (p *Photo) CanBeEditedBy(currentUser *User) bool {
	if currentUser.HasAdminScope(config.ScopePhotoModerator) {
		return true
	}

	return p.UserID == currentUser.ID
}

// CanBeSeenBy checks whether a photo can be seen by the current user.
//
// An admin user with omni photo view permission can always see the photo.
//
// Note: this function incurs several DB queries to look up the photo's owner and block lists.
func (p *Photo) CanBeSeenBy(currentUser *User) (bool, error) {
	// Admins with photo moderator ability can always see.
	if currentUser.HasAdminScope(config.ScopePhotoModerator) {
		return true, nil
	}

	return p.ShouldBeSeenBy(currentUser)
}

// ShouldBeSeenBy checks whether a photo should be seen by the current user.
//
// Even if the current user is an admin with photo moderator ability, this function will return
// whether the admin 'should' be able to see if not for their admin status. Example: a private
// photo may be shown to the admin so they can moderate it, but they shouldn't be able to "like"
// it or mark it as "viewed."
//
// Note: this function incurs several DB queries to look up the photo's owner and block lists.
func (p *Photo) ShouldBeSeenBy(currentUser *User) (bool, error) {
	// Our own photo?
	var isOwnPhoto = currentUser.ID == p.UserID
	if isOwnPhoto {
		return true, nil
	}

	// Find the photo's owner.
	user, err := GetUser(p.UserID)
	if err != nil {
		return false, err
	}

	// Can the viewer see the photo's owner? (Blocking, banned?)
	if err := user.CanBeSeenBy(currentUser); err != nil {
		return false, err
	}

	// Is this user private and we're not friends?
	var (
		areFriends = AreFriends(user.ID, currentUser.ID)
		isPrivate  = user.Visibility == UserVisibilityPrivate && !areFriends
	)
	if isPrivate && !isOwnPhoto {
		return false, errors.New("user is private and we aren't friends")
	}

	// The photo is friends-only and we're not friends?
	if p.Visibility == PhotoFriends && !areFriends {
		return false, errors.New("photo is friends-only and we aren't friends")
	}

	// Is this a private photo and are we allowed to see?
	isGranted := IsPrivateUnlocked(user.ID, currentUser.ID)
	if p.Visibility == PhotoPrivate && !isGranted && !isOwnPhoto {
		return false, errors.New("photo is private")
	}

	return true, nil
}

// UserGallery configuration for filtering gallery pages.
type UserGallery struct {
	Explicit   string // "", "true", "false"
	GIF        string // "", "true", "false"
	Visibility []PhotoVisibility
}

/*
PaginateUserPhotos gets a page of photos belonging to a user ID.
*/
func PaginateUserPhotos(currentUser *User, userID uint64, conf UserGallery, pager *Pagination) ([]*Photo, error) {
	var (
		p            = []*Photo{}
		wheres       = []string{}
		placeholders = []interface{}{}
	)

	// If we will be ordering by 'random', get the ID ranges and pick a large
	// sample of random IDs. Not all of them will exist (e.g. deleted photos),
	// but we will select way more IDs than the page size to hopefully compensate.
	var isRandom bool
	if pager.Sort == "random" {
		isRandom = true
		filteredIDs, err := GetRandomPhotoIDs(&userID, pager.PerPage)
		if err != nil {
			return nil, err
		}

		wheres = append(wheres, "photos.id IN ?")
		placeholders = append(placeholders, filteredIDs)

		// Adjust the paging and sorting details.
		// "ORDER BY id=1 DESC, id=2 DESC, id=3 DESC, ..."
		var orderByIDs []string
		for _, id := range filteredIDs {
			orderByIDs = append(orderByIDs, fmt.Sprintf("id=%d DESC", id))
		}
		pager.Sort = strings.Join(orderByIDs, ", ")
		pager.Page = 1
	}

	var explicit = []bool{}
	switch conf.Explicit {
	case "true":
		explicit = []bool{true}
	case "false":
		explicit = []bool{false}
	}

	wheres = append(wheres, "user_id = ? AND visibility IN ?")
	placeholders = append(placeholders, userID, conf.Visibility)

	// Admin filter for only pictures currently marked as Flagged Explicit by the community.
	if conf.Explicit == "flagged" {
		wheres = append(wheres, "photos.flagged IS TRUE")
	}

	if len(explicit) > 0 {
		wheres = append(wheres, "explicit = ?")
		placeholders = append(placeholders, explicit[0])
	}

	// Filter by GIFs?
	switch conf.GIF {
	case "true":
		wheres = append(wheres, "is_gif IS TRUE")
	case "false":
		wheres = append(wheres, "is_gif IS NOT TRUE")
	}

	query := DB.Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	)

	// If we are sorting by "recently_commented"
	if pager.Sort == "recently_commented" {
		query = query.Joins(`
			LEFT JOIN (
				SELECT table_id, MAX(created_at) AS latest_comment_at
				FROM comments
				WHERE table_name='photos'
				GROUP BY table_id
			) lc ON lc.table_id = photos.id
		`)
		pager.Sort = "lc.latest_comment_at DESC NULLS LAST"
	}

	query = query.Order(
		pager.Sort,
	)

	// Get the total count.
	query.Model(&Photo{}).Count(&pager.Total)

	result := query.Offset(
		pager.GetOffset(),
	).Limit(pager.PerPage).Find(&p)

	// If we are sorting by random, edit the Total count to be how many on this page.
	// Otherwise the Total will appear to fluctuate each time and only be a small subset
	// of the total available, due to the ID filtering.
	if isRandom {
		pager.Total = int64(len(p))
	}

	return p, result.Error
}

// View a photo, incrementing its Views count but not its UpdatedAt.
// Debounced with a Redis key.
func (p *Photo) View(user *User) error {
	// The owner of the photo does not count views.
	if p.UserID == user.ID {
		return nil
	}

	// Should the viewer be able to see this, regardless of their admin ability?
	if ok, err := p.ShouldBeSeenBy(user); !ok {
		return err
	}

	// Debounce this.
	var redisKey = fmt.Sprintf(config.PhotoViewDebounceRedisKey, user.ID, p.ID)
	if redis.Exists(redisKey) {
		return nil
	}
	redis.Set(redisKey, nil, config.PhotoViewDebounceCooldown)

	return DB.Model(&Photo{}).Where(
		"id = ?",
		p.ID,
	).Updates(map[string]interface{}{
		"views":      p.Views + 1,
		"updated_at": p.UpdatedAt,
	}).Error
}

// CountPhotos returns the total number of photos on a user's account.
func CountPhotos(userID uint64) int64 {
	var count int64
	result := DB.Where(
		"user_id = ?",
		userID,
	).Model(&Photo{}).Count(&count)
	if result.Error != nil {
		log.Error("CountPhotos(%d): %s", userID, result.Error)
	}
	return count
}

// GetPhotoIDRange returns the min and max Photo ID columns, optionally belonging to a specific user.
func GetPhotoIDRange(userID *uint64) (min, max int64, err error) {
	type record struct {
		MinID int64
		MaxID int64
	}
	var result record

	if userID != nil {
		res := DB.Raw(`
			SELECT
				MIN(id) AS min_id,
				MAX(id) AS max_id
			FROM photos
			WHERE user_id = ?
		`, userID).Scan(&result)
		if res.Error != nil {
			return 0, 0, res.Error
		}
	} else {
		res := DB.Raw(`
			SELECT
				MIN(id) AS min_id,
				MAX(id) AS max_id
			FROM photos
		`).Scan(&result)
		if res.Error != nil {
			return 0, 0, res.Error
		}
	}

	return result.MinID, result.MaxID, nil
}

// GetRandomPhotoIDs returns a slice of randomize photo IDs several times larger than your requested page size.
func GetRandomPhotoIDs(userID *uint64, perPage int) ([]uint64, error) {

	// If a specific user ID, shuffle their narrow set of IDs.
	if userID != nil {
		var photoIDs []uint64
		res := DB.Raw(
			"SELECT id FROM photos WHERE user_id = ?",
			userID,
		).Scan(&photoIDs)
		if res.Error != nil {
			return nil, res.Error
		}

		rand.Shuffle(len(photoIDs), func(i, j int) {
			photoIDs[i], photoIDs[j] = photoIDs[j], photoIDs[i]
		})
		return photoIDs, nil
	}

	// Public gallery view: get the min/max ID range.
	min, max, err := GetPhotoIDRange(userID)
	if err != nil {
		return nil, err
	}

	var (
		filteredIDs = []uint64{}
		chosenIDs   = map[int64]interface{}{}
	)
	for i := 0; i < perPage*10; i++ {
		id := rand.Int63n(max) + min - 1
		if _, ok := chosenIDs[id]; ok {
			continue
		}
		filteredIDs = append(filteredIDs, uint64(id))
	}

	return filteredIDs, nil
}

// GetOrphanedPhotos gets all photos having no user ID associated.
func GetOrphanedPhotos() ([]*Photo, int64, error) {
	var (
		count int64
		ps    = []*Photo{}
	)

	query := DB.Model(&Photo{}).Where(`
		NOT EXISTS (
			SELECT 1 FROM users WHERE users.id = photos.user_id
		)
		OR photos.user_id = 0
	`)
	query.Count(&count)
	res := query.Find(&ps)
	if res.Error != nil {
		return nil, 0, res.Error
	}

	return ps, count, res.Error
}

// PhotoMap helps map a set of users to look up by ID.
type PhotoMap map[uint64]*Photo

// MapPhotos looks up a set of photos IDs in bulk and returns a PhotoMap suitable for templates.
func MapPhotos(photoIDs []uint64) (PhotoMap, error) {
	var (
		photoMap = PhotoMap{}
		set      = map[uint64]interface{}{}
		distinct = []uint64{}
	)

	if len(photoIDs) == 0 {
		return photoMap, nil
	}

	// Uniqueify the IDs.
	for _, uid := range photoIDs {
		if _, ok := set[uid]; ok {
			continue
		}
		set[uid] = nil
		distinct = append(distinct, uid)
	}

	var (
		photos = []*Photo{}
		result = DB.Model(&Photo{}).Where("id IN ?", distinct).Find(&photos)
	)

	if result.Error == nil {
		for _, row := range photos {
			photoMap[row.ID] = row
		}
	}

	return photoMap, result.Error
}

// Has a photo ID in the map?
func (pm PhotoMap) Has(id uint64) bool {
	_, ok := pm[id]
	return ok
}

// Get a photo from the PhotoMap.
func (pm PhotoMap) Get(id uint64) *Photo {
	if photo, ok := pm[id]; ok {
		return photo
	}
	return nil
}

/*
IsSiteGalleryThrottled returns whether the user is throttled from marking additional pictures for the Site Gallery.

The thresholds are in pkg/config but the idea is a user can only upload (say) 5 Site Gallery photos within a
24 hour time span, so that new users who sign up and immediately max out their full gallery don't end up
spamming the Site Gallery for pages and pages.

If the user has too many recent Site Gallery pictures:

  - Newly uploaded photos can NOT check the Gallery box.
  - Editing any existing photo which is NOT in the Gallery: you can not mark the box either.
  - Existing Gallery photos CAN be un-marked for the gallery, which (if it is one of the 5 recent
    photos) may put the user below the threshold again.

If the user is on the Edit page for an existing photo, provide the Photo; otherwise leave it nil
if the user is uploading a new photo for the first time.
*/
func IsSiteGalleryThrottled(user *User, editPhoto *Photo) bool {
	// If the editing photo is already in the gallery, allow the user to keep or remove it.
	if editPhoto != nil && editPhoto.Gallery {
		return false
	}

	var count = CountRecentGalleryPhotos(user, config.SiteGalleryRateLimitInterval)
	log.Debug("IsSiteGalleryThrottled(%s): they have %d recent Gallery photos", user.Username, count)
	return count >= config.SiteGalleryRateLimitMax
}

// CountRecentGalleryPhotos returns the count of recently uploaded Site Gallery photos for a user,
// within the past 24 hours, to rate limit spammy bulk uploads that will flood the gallery.
func CountRecentGalleryPhotos(user *User, duration time.Duration) (count int64) {
	result := DB.Where(
		"user_id = ? AND created_at >= ? AND gallery IS TRUE",
		user.ID,
		time.Now().Add(-duration),
	).Model(&Photo{}).Count(&count)
	if result.Error != nil {
		log.Error("CountRecentGalleryPhotos(%d): %s", user.ID, result.Error)
	}
	return
}

// AllFriendsOnlyPhotoIDs returns the listing of all friends-only photo IDs belonging to the user(s) given.
func AllFriendsOnlyPhotoIDs(users ...*User) ([]uint64, error) {
	var userIDs = []uint64{}
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}

	if len(userIDs) == 0 {
		return nil, errors.New("no user IDs given")
	}

	var photoIDs = []uint64{}
	err := DB.Table(
		"photos",
	).Select(
		"photos.id AS id",
	).Where(
		"user_id IN ? AND visibility = ?",
		userIDs, PhotoFriends,
	).Scan(&photoIDs)

	if err.Error != nil {
		return photoIDs, fmt.Errorf("AllFriendsOnlyPhotoIDs(%+v): %s", userIDs, err.Error)
	}

	return photoIDs, nil
}

// CountPhotosICanSee returns the number of photos on an account which can be seen by the given viewer.
func CountPhotosICanSee(user *User, viewer *User) int64 {
	// Visibility filters to query by.
	var visibilities = []PhotoVisibility{
		PhotoPublic,
	}

	// Is the viewer friends with the target?
	if AreFriends(user.ID, viewer.ID) {
		visibilities = append(visibilities, PhotoFriends)
	}

	// Is the viewer granted private access?
	if IsPrivateUnlocked(user.ID, viewer.ID) {
		visibilities = append(visibilities, PhotoPrivate)
	}

	// Get the photo count now.
	var count int64
	result := DB.Where(
		"user_id = ? AND visibility IN ?",
		user.ID, visibilities,
	).Model(&Photo{}).Count(&count)
	if result.Error != nil {
		log.Error("CountPhotosICanSee(%d, %d): %s", user.ID, viewer.ID, result.Error)
	}
	return count
}

// MapPhotoCounts returns a mapping of user ID to the CountPhotos()-equivalent result for each.
// It's used on the member directory to show photo counts on each user card.
func MapPhotoCounts(users []*User) PhotoCountMap {
	return MapPhotoCountsByVisibility(users, PhotoPublic)
}

// MapPhotoCountsByVisibility returns a mapping of user ID to the CountPhotos()-equivalent result for each.
func MapPhotoCountsByVisibility(users []*User, visibility PhotoVisibility) PhotoCountMap {
	var (
		userIDs = []uint64{}
		result  = PhotoCountMap{}
	)

	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}

	type group struct {
		UserID     uint64
		PhotoCount int64
	}
	var groups = []group{}

	if res := DB.Table(
		"photos",
	).Select(
		"user_id, count(id) AS photo_count",
	).Where(
		"user_id IN ? AND visibility = ?", userIDs, visibility,
	).Group("user_id").Scan(&groups); res.Error != nil {
		log.Error("CountPhotosForUsers: %s", res.Error)
	}

	// Map the results in.
	for _, row := range groups {
		result[row.UserID] = row.PhotoCount
	}

	return result
}

// MapPhotoCounts returns a mapping of user ID to the CountPhotosICanSee()-equivalent result for each.
// It's used on the member directory to show photo counts on each user card.
/* TODO: under construction..
func MapPhotoCounts(users []*User, viewer *User) PhotoCountMap {
	var (
		userIDs = []uint64{}
		result  = PhotoCountMap{}

		wheres       = []string{}
		placeholders = []interface{}{}

		// User ID filters for the viewer's context.
		myFriendIDs         = FriendIDs(viewer.ID)
		myPrivateGrantedIDs = PrivateGrantedUserIDs(viewer.ID)
	)

	// Define "all photos visibilities"
	var (
		photosPublic = []PhotoVisibility{
			PhotoPublic,
		}
		photosFriends = []PhotoVisibility{
			PhotoPublic,
			PhotoFriends,
		}
		photosPrivate = []PhotoVisibility{
			PhotoPublic,
			PhotoPrivate,
		}
	)

	// Flatten the userIDs of all passed in users.
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}

	// Build the where clause.
	wheres = append(wheres, "user_id IN ?")
	placeholders = append(placeholders, userIDs)

	log.Error("FRIEND IDS: %+v", myFriendIDs)

	// Filter by which photos are visible to us.
	wheres = append(wheres,
		"((user_id IN ? AND visibility IN ?) OR "+
			"(user_id IN ? AND visibility IN ?) OR "+
			"(user_id NOT IN ? AND visibility IN ?))",
	)
	placeholders = append(placeholders,
		myFriendIDs, photosFriends,
		myPrivateGrantedIDs, photosPrivate,
		myFriendIDs, photosPublic,
	)

	type group struct {
		UserID     uint64
		PhotoCount int64
	}
	var groups = []group{}

	if res := DB.Table(
		"photos",
	).Select(
		"user_id, count(id) AS photo_count",
	).Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	).Group("user_id").Scan(&groups); res.Error != nil {
		log.Error("CountPhotosForUsers: %s", res.Error)
	}

	// Map the results in.
	for _, row := range groups {
		result[row.UserID] = row.PhotoCount
	}

	return result
}
*/

type PhotoCountMap map[uint64]int64

// Get a photo count for the given user ID from the map.
func (pc PhotoCountMap) Get(id uint64) int64 {
	if value, ok := pc[id]; ok {
		return value
	}
	return 0
}

// CountExplicitPhotos returns the number of explicit photos a user has (so non-explicit viewers can see some do exist)
func CountExplicitPhotos(userID uint64, visibility []PhotoVisibility) (int64, error) {
	query := DB.Where(
		"user_id = ? AND visibility IN ? AND explicit = ?",
		userID,
		visibility,
		true,
	)

	var count int64
	result := query.Model(&Photo{}).Count(&count)
	return count, result.Error
}

// CountPublicPhotos returns the number of public photos on a user's page.
func CountPublicPhotos(userID uint64) int64 {
	return CountUserPhotosByVisibility(userID, PhotoPublic)
}

// CountUserPhotosByVisibility returns the number of a user's photos by visibility.
func CountUserPhotosByVisibility(userID uint64, visibility PhotoVisibility) int64 {
	query := DB.Where(
		"user_id = ? AND visibility = ?",
		userID,
		visibility,
	)

	var count int64
	result := query.Model(&Photo{}).Count(&count)
	if result.Error != nil {
		log.Error("CountUserPhotosByVisibility(%d, %s): %s", userID, visibility, result.Error)
	}
	return count
}

// DistinctPhotoTypes returns types of photos the user has: a set of public, friends, or private.
//
// The result is cached on the User the first time it's queried.
//
// Only counts Approved photos.
func (u *User) DistinctPhotoTypes() (result map[PhotoVisibility]struct{}) {
	if u.cachePhotoTypes != nil {
		return u.cachePhotoTypes
	}

	result = map[PhotoVisibility]struct{}{}

	var results = []*Photo{}
	query := DB.Model(&Photo{}).
		Select("DISTINCT photos.visibility").
		Where("user_id = ? AND status = 'approved'", u.ID).
		Group("photos.visibility").
		Find(&results)
	if query.Error != nil {
		log.Error("User.DistinctPhotoTypes(%s): %s", u.Username, query.Error)
		return
	}

	for _, row := range results {
		result[row.Visibility] = struct{}{}
	}

	u.cachePhotoTypes = result
	return
}

// GetPhotoInsights maps out statistics about a user's photo gallery.
func GetPhotoInsights(currentUser, user *User, visibility []PhotoVisibility) PhotoInsights {
	var result = PhotoInsights{}

	type record struct {
		MetricType  string
		MetricValue string
		MetricCount int64
	}
	var records []record
	res := DB.Raw(`
		-- Photo counts by visibility.
		WITH subquery_visibility AS (
			SELECT
				COUNT(*) AS photo_count,
				visibility
			FROM photos
			WHERE user_id = ?
			AND visibility IN ?
			GROUP BY visibility
		),

		-- Photo counts by explicit.
		subquery_explicit AS (
			SELECT
				COUNT(*) AS photo_count
			FROM photos
			WHERE user_id = ?
			AND visibility IN ?
			AND explicit IS TRUE
		),

		-- Photo counts of GIFs.
		subquery_gifs AS (
			SELECT
				COUNT(*) AS photo_count
			FROM photos
			WHERE user_id = ?
			AND visibility IN ?
			AND is_gif IS TRUE
		)

		SELECT
			'Visibility' AS metric_type,
			visibility AS metric_value,
			photo_count AS metric_count
		FROM subquery_visibility

		UNION ALL

		SELECT
			'Explicit' AS metric_type,
			'explicit' AS metric_value,
			photo_count AS metric_count
		FROM subquery_explicit

		UNION ALL

		SELECT
			'GIF' AS metric_type,
			'gif' AS metric_value,
			photo_count AS metric_count
		FROM subquery_gifs
	`, user.ID, visibility, user.ID, visibility, user.ID, visibility).Scan(&records)
	if res.Error != nil {
		log.Error("GetPhotoInsights: %s", res.Error)
		return result
	}

	for _, row := range records {
		switch row.MetricType {
		case "Visibility":
			switch row.MetricValue {
			case "public":
				result.PublicCount = row.MetricCount
			case "friends":
				result.FriendsOnlyCount = row.MetricCount
			case "private":
				result.PrivateCount = row.MetricCount
			}
		case "Explicit":
			result.ExplicitCount = row.MetricCount
		case "GIF":
			result.GifCount = row.MetricCount
		default:
			log.Error("GetPhotoInsights: Unknown MetricType returned from DB: %s", row.MetricType)
		}
	}

	return result
}

type PhotoInsights struct {
	// Photo counts by visibility (that the current viewer can see).
	PublicCount      int64
	FriendsOnlyCount int64
	PrivateCount     int64

	// Counts of Explicit photos and GIF animations.
	ExplicitCount int64
	GifCount      int64
}

// FlushCaches clears any cached attributes (such as distinct photo types) for the user.
func (u *User) FlushCaches() {
	u.cachePhotoTypes = nil
	u.cacheBlockedUserIDs = nil
	u.cachePhotoIDs = nil
}

// Gallery config for the main Gallery paginator.
type Gallery struct {
	Explicit    string // Explicit filter
	Visibility  string // Visibility filter
	GIF         string // GIFs filter
	Tagged      bool   // I'm Tagged
	AdminView   bool   // Show all images
	FriendsOnly bool   // Only show self/friends instead of everybody's pics
	MyLikes     bool   // Filter to photos I have liked
}

/*
PaginateGalleryPhotos gets a page of all public user photos for the site gallery.

Admin view returns ALL photos regardless of Gallery status.
*/
func PaginateGalleryPhotos(user *User, conf Gallery, pager *Pagination) ([]*Photo, error) {
	var (
		filterExplicit   = conf.Explicit
		filterVisibility = conf.Visibility
		friendsOnly      = conf.FriendsOnly // Show only self and friends pictures
		p                = []*Photo{}
		query            *gorm.DB

		// Admin settings: is the current user a Photo Moderator?
		// The gallery "Admin View" is only available to photo moderators.
		isModerator = user.HasAdminScope(config.ScopePhotoModerator)
		adminView   = isModerator && conf.AdminView

		// Get the user ID and their preferences.
		userID     = user.ID
		explicitOK = user.Explicit // User opted-in for explicit content

		privateUserIDs           = PrivateGrantedUserIDs(userID)
		privateUserIDsAreFriends = PrivateGrantedUserIDsAreFriends(user)
		wheres                   = []string{}
		placeholders             = []interface{}{}
	)

	// If we will be ordering by 'random', get the ID ranges and pick a large
	// sample of random IDs. Not all of them will exist (e.g. deleted photos),
	// but we will select way more IDs than the page size to hopefully compensate.
	var (
		isRandom  bool
		randomIDs []uint64
	)
	if pager.Sort == "random" {
		isRandom = true
		filteredIDs, err := GetRandomPhotoIDs(nil, pager.PerPage)
		if err != nil {
			return nil, err
		}

		// Keep the randomized IDs e.g. for admin view later.
		randomIDs = filteredIDs

		wheres = append(wheres, "photos.id IN ?")
		placeholders = append(placeholders, filteredIDs)

		// Adjust the paging and sorting details.
		// "ORDER BY id=1 DESC, id=2 DESC, id=3 DESC, ..."
		var orderByIDs []string
		for _, id := range filteredIDs {
			orderByIDs = append(orderByIDs, fmt.Sprintf("id=%d DESC", id))
		}
		pager.Sort = strings.Join(orderByIDs, ", ")
	}

	// Define "all photos visibilities"
	var (
		photosPublic = []PhotoVisibility{
			PhotoPublic,
		}
		photosFriends = []PhotoVisibility{
			PhotoPublic,
			PhotoFriends,
		}
		photosPrivate = []PhotoVisibility{
			PhotoPrivate,
		}
	)

	// Friend IDs subquery, used in a "WHERE user_id IN ?" clause.
	friendsQuery := fmt.Sprintf(`(
		SELECT target_user_id
		FROM friends
		WHERE source_user_id = %d
		AND approved IS TRUE
	)`, userID)

	// What sets of User ID * Visibility filters to query under?
	var (
		visOrs          = []string{}
		visPlaceholders = []interface{}{}
	)

	// Whose photos can you see on the Site Gallery?
	if friendsOnly {
		// User wants to see only self and friends photos.
		visOrs = append(visOrs,
			fmt.Sprintf("(photos.user_id IN %s AND photos.visibility IN ?)", friendsQuery),
			"photos.user_id = ?",
		)
		visPlaceholders = append(visPlaceholders, photosFriends, userID)

		// If their friends granted private photos, include those too.
		if len(privateUserIDsAreFriends) > 0 {
			visOrs = append(visOrs, "(photos.user_id IN ? AND photos.visibility IN ?)")
			visPlaceholders = append(visPlaceholders, privateUserIDsAreFriends, photosPrivate)
		}
	} else {
		// You can see friends' Friend photos but only public for non-friends.
		visOrs = append(visOrs,
			fmt.Sprintf("(photos.user_id IN %s AND photos.visibility IN ?)", friendsQuery),
			"(photos.user_id IN ? AND photos.visibility IN ?)",
			fmt.Sprintf("(photos.user_id NOT IN %s AND photos.visibility IN ?)", friendsQuery),
			"photos.user_id = ?",
		)
		visPlaceholders = append(visPlaceholders,
			photosFriends,
			privateUserIDs, photosPrivate,
			photosPublic,
			userID,
		)
	}

	// Join the User ID * Visibility filters into a nested "OR"
	wheres = append(wheres, fmt.Sprintf("(%s)", strings.Join(visOrs, " OR ")))
	placeholders = append(placeholders, visPlaceholders...)

	// Gallery photos only.
	wheres = append(wheres, "photos.gallery = ?")
	placeholders = append(placeholders, true)

	// Filter by photos the user has liked.
	if conf.MyLikes {
		wheres = append(wheres, `
			EXISTS (
				SELECT 1
				FROM likes
				WHERE likes.user_id = ?
				AND likes.table_name = 'photos'
				AND likes.table_id = photos.id
			)
		`)
		placeholders = append(placeholders, user.ID)
	}

	// Filter blocked users.
	bw, bp := BlockedUserSubquery("photos.user_id", user.ID)
	wheres = append(wheres, bw)
	placeholders = append(placeholders, bp...)

	// Non-explicit pics unless the user opted in. Allow explicit filter setting to override.
	if filterExplicit != "" {
		// The usual value is a true/false, but admin users can search for "flagged" or labeled explicit pictures.
		if filterExplicit == "flagged" && isModerator {
			wheres = append(wheres, "photos.flagged IS TRUE")
		} else if (filterExplicit == "non-explicit" || filterExplicit == "force-explicit") && isModerator {
			wheres = append(wheres, "photos.admin_label = ?")
			placeholders = append(placeholders, filterExplicit)
		} else {
			wheres = append(wheres, "photos.explicit = ?")
			placeholders = append(placeholders, filterExplicit == "true")
		}
	} else if !explicitOK {
		wheres = append(wheres, "photos.explicit = ?")
		placeholders = append(placeholders, false)
	}

	// Is the user furthermore clamping the visibility filter?
	if filterVisibility != "" {
		wheres = append(wheres, "photos.visibility = ?")
		placeholders = append(placeholders, filterVisibility)
	}

	// Filter by GIFs?
	switch conf.GIF {
	case "true":
		wheres = append(wheres, "is_gif IS TRUE")
	case "false":
		wheres = append(wheres, "is_gif IS NOT TRUE")
	}

	// I'm Tagged?
	if conf.Tagged {
		wheres = append(wheres, `
			EXISTS (
				SELECT 1
				FROM tagged_users
				WHERE user_id = ?
				AND table_name = 'photos'
				AND table_id = photos.id
			)
		`)
		placeholders = append(placeholders, user.ID)
	}

	// Only active accounts.
	wheres = append(wheres,
		"EXISTS (SELECT 1 FROM users WHERE id = photos.user_id AND users.status='active')",
	)

	// Exclude private users' photos.
	wheres = append(wheres,
		"NOT EXISTS (SELECT 1 FROM users WHERE id = photos.user_id AND users.visibility = 'private')",
	)

	// Admin view: get ALL PHOTOS on the site, period.
	if adminView {
		query = DB

		// If the viewing admin is not Unblockable, still withhold pictures from blocking users.
		if !user.HasAdminScope(config.ScopeUnblockable) {
			query = query.Where(bw, bp...)
		}

		// Admin may filter too.
		if filterVisibility != "" {
			query = query.Where("photos.visibility = ?", filterVisibility)
		}
		if filterExplicit != "" {
			if filterExplicit == "flagged" {
				query = query.Where("photos.explicit IS TRUE AND photos.flagged IS TRUE")
			} else {
				query = query.Where("photos.explicit = ?", filterExplicit == "true")
			}
		}
		switch conf.GIF {
		case "true":
			query = query.Where("is_gif IS TRUE")
		case "false":
			query = query.Where("is_gif IS NOT TRUE")
		}

		// Randomized sort for admin view?
		if len(randomIDs) > 0 {
			query = query.Where("photos.id IN ?", randomIDs)
		}
	} else {
		query = DB.Where(
			strings.Join(wheres, " AND "),
			placeholders...,
		)
	}

	// If we are sorting by "recently_commented"
	if pager.Sort == "recently_commented" {
		query = query.Joins(`
			LEFT JOIN (
				SELECT table_id, MAX(created_at) AS latest_comment_at
				FROM comments
				WHERE table_name='photos'
				GROUP BY table_id
			) lc ON lc.table_id = photos.id
		`)
		pager.Sort = "lc.latest_comment_at DESC NULLS LAST"
	}

	query = query.Order(pager.Sort)
	query.Model(&Photo{}).Count(&pager.Total)
	result := query.Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&p)

	// If we are sorting by random, edit the Total count to be how many on this page.
	// Otherwise the Total will appear to fluctuate each time and only be a small subset
	// of the total available, due to the ID filtering.
	if isRandom {
		pager.Total = int64(len(p))
	}

	return p, result.Error
}

// UpdatePhotoCachedCounts will refresh the cached like/comment count on the photos table.
func UpdatePhotoCachedCounts(photoID uint64) error {
	res := DB.Exec(`
		UPDATE photos
		SET like_count = (
			SELECT count(id)
			FROM likes
			WHERE table_name='photos'
			AND table_id=photos.id
		),
		comment_count = (
			SELECT count(id)
			FROM comments
			WHERE table_name='photos'
			AND table_id=photos.id
		)
		WHERE photos.id = ?;
	`, photoID)
	return res.Error
}

// Save photo.
func (p *Photo) Save() error {
	result := DB.Save(p)
	return result.Error
}

// BreakRelationships nulls out foreign keys referencing a photo
// so that it can be cleanly deleted.
func (p *Photo) BreakRelationships() error {

	// Null out user profile pictures referencing this ID.
	if res := DB.Exec(
		`
			UPDATE users
			SET profile_photo_id = NULL
			WHERE profile_photo_id = ?
		`,
		p.ID,
	); res.Error != nil {
		return fmt.Errorf("user profile photos: %s", res.Error)
	}

	return nil
}

// Delete photo.
func (p *Photo) Delete() error {

	// Break relationships safely.
	if err := p.BreakRelationships(); err != nil {
		return fmt.Errorf("breaking foreign key relationships: %s", err)
	}

	result := DB.Delete(p)
	return result.Error
}
