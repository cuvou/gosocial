package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/log"
	"gorm.io/gorm"
)

// PrivatePhoto table to track who you have unlocked your private photos for.
type PrivatePhoto struct {
	ID           uint64 `gorm:"primaryKey"`
	SourceUserID uint64 `gorm:"index"` // the owner of a photo
	TargetUserID uint64 `gorm:"index"` // the receiver
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// UnlockPrivatePhotos is sourceUserId allowing targetUserId to see their private photos.
func UnlockPrivatePhotos(sourceUserID, targetUserID uint64) error {
	// Did we already allow this user?
	var pb *PrivatePhoto
	exist := DB.Where(
		"source_user_id = ? AND target_user_id = ?",
		sourceUserID, targetUserID,
	).First(&pb).Error

	// Update existing.
	if exist == nil {
		return nil
	}

	// Create the PrivatePhoto.
	pb = &PrivatePhoto{
		SourceUserID: sourceUserID,
		TargetUserID: targetUserID,
	}
	return DB.Create(pb).Error
}

// RevokePrivatePhotos is sourceUserId revoking targetUserId to see their private photos.
func RevokePrivatePhotos(sourceUserID, targetUserID uint64) error {
	result := DB.Where(
		"source_user_id = ? AND target_user_id = ?",
		sourceUserID, targetUserID,
	).Delete(&PrivatePhoto{})
	return result.Error
}

// RevokePrivatePhotosAll is sourceUserId revoking ALL USERS from their private photos.
func RevokePrivatePhotosAll(sourceUserID uint64) error {
	result := DB.Where(
		"source_user_id = ?",
		sourceUserID,
	).Delete(&PrivatePhoto{})
	return result.Error
}

// RevokePrivatePhotoNotifications removes notifications about newly uploaded private photos
// that were sent to one (or multiple) members when the user revokes their access later. Pass
// a nil fromUserID to revoke the photo upload notifications from ALL users.
func RevokePrivatePhotoNotifications(currentUser, fromUser *User) error {
	// Gather the IDs of all our private photos to nuke notifications for.
	photoIDs, err := currentUser.AllPrivatePhotoIDs()
	if err != nil {
		return err
	} else if len(photoIDs) == 0 {
		// Nothing to do.
		return nil
	}

	// Who to clear the notifications for?
	if fromUser == nil {
		log.Info("RevokePrivatePhotoNotifications(%s): forget about private photo uploads for EVERYBODY on photo IDs: %v", currentUser.Username, photoIDs)
		return RemoveNotificationBulk("photos", photoIDs)
	} else {
		log.Info("RevokePrivatePhotoNotifications(%s): forget about private photo uploads for user %s on photo IDs: %v", currentUser.Username, fromUser.Username, photoIDs)
		return RemoveSpecificNotificationBulk([]*User{currentUser, fromUser}, NotificationNewPhoto, "photos", photoIDs)
	}
}

// AllPrivatePhotoIDs returns the listing of all IDs of the user's private photos.
func (u *User) AllPrivatePhotoIDs() ([]uint64, error) {
	var photoIDs = []uint64{}
	err := DB.Table(
		"photos",
	).Select(
		"photos.id AS id",
	).Where(
		"user_id = ? AND visibility = ?",
		u.ID, PhotoPrivate,
	).Scan(&photoIDs)

	if err.Error != nil {
		return photoIDs, fmt.Errorf("AllPrivatePhotoIDs(%s): %s", u.Username, err.Error)
	}

	return photoIDs, nil
}

// AllPhotoIDs returns the listing of all IDs of the user's photos.
func (u *User) AllPhotoIDs() ([]uint64, error) {
	if u.cachePhotoIDs != nil {
		return u.cachePhotoIDs, nil
	}

	var photoIDs = []uint64{}
	err := DB.Table(
		"photos",
	).Select(
		"photos.id AS id",
	).Where(
		"user_id = ?",
		u.ID,
	).Scan(&photoIDs)

	if err.Error != nil {
		return photoIDs, fmt.Errorf("AllPhotoIDs(%s): %s", u.Username, err.Error)
	}

	u.cachePhotoIDs = photoIDs

	return photoIDs, nil
}

/*
ShouldShowPrivateUnlockPrompt determines whether the current user should be shown a prompt, when viewing
the other user's gallery, to unlock their private photos for that user.

This function verifies that the source user actually has a private photo to share, and that the target
user doesn't have a privacy setting enabled that should block the private photo unlock request.
*/
func ShouldShowPrivateUnlockPrompt(sourceUser, targetUser *User) (bool, error) {

	// If the current user doesn't even have a private photo to share, no prompt.
	var (
		myPrivatePhotos = CountUserPhotosByVisibility(sourceUser.ID, PhotoPrivate)
	)
	if myPrivatePhotos == 0 {
		return false, errors.New("you do not currently have a private photo or blog to share")
	}

	// If no target user (generic check if we have privates to share) return now.
	if targetUser == nil {
		return true, nil
	}

	// Does the target user have a privacy setting enabled?
	if pp := GetPrivacySetting(targetUser.ID).PrivatePhotos; pp != "" {
		areFriends := AreFriends(sourceUser.ID, targetUser.ID)

		switch pp {
		case "nobody":
			return false, errors.New("they decline all private photo sharing")
		case "friends":
			if areFriends {
				return true, nil
			}
			return false, errors.New("they are only accepting private photos from their friends")
		case "messaged":
			if areFriends || HasSentAMessage(targetUser, sourceUser) {
				return true, nil
			}
			return false, errors.New("they are only accepting private photos from their friends or from people they have sent a DM to")
		}
	}

	return true, nil
}

// IsPrivateUnlocked quickly sees if sourceUserID has unlocked private photos for targetUserID to see.
func IsPrivateUnlocked(sourceUserID, targetUserID uint64) bool {
	pb := &PrivatePhoto{}
	result := DB.Where(
		"source_user_id = ? AND target_user_id = ?",
		sourceUserID, targetUserID,
	).First(&pb)
	return result.Error == nil
}

// CountPrivateGrantee returns how many users have granted you access to their private photos.
func CountPrivateGrantee(userID uint64) int64 {
	var count int64
	DB.Model(&PrivatePhoto{}).Where(
		"target_user_id = ?",
		userID,
	).Count(&count)
	return count
}

// PrivateGrantedUserIDs returns all user IDs who have granted access for userId to see their private photos.
func PrivateGrantedUserIDs(userId uint64) []uint64 {
	var (
		ps      = []*PrivatePhoto{}
		userIDs = []uint64{userId}
	)
	DB.Where("target_user_id = ?", userId).Find(&ps)
	for _, row := range ps {
		userIDs = append(userIDs, row.SourceUserID)
	}
	return userIDs
}

// PrivateGrantedUserIDsAreFriends returns user IDs who have granted us their private pictures, and are also our friends.
func PrivateGrantedUserIDsAreFriends(currentUser *User) []uint64 {
	var (
		ps      = []*PrivatePhoto{}
		userIDs = []uint64{}
	)
	DB.Model(&PrivatePhoto{}).Joins(
		"JOIN friends ON friends.source_user_id = private_photos.source_user_id AND friends.target_user_id = private_photos.target_user_id",
	).Where(
		"private_photos.target_user_id = ? AND friends.approved IS true", currentUser.ID,
	).Find(&ps)
	for _, row := range ps {
		userIDs = append(userIDs, row.SourceUserID)
	}
	return userIDs
}

// PrivateGranteeUserIDs are the users whom WE have granted access to our photos (userId is the photo owners).
func PrivateGranteeUserIDs(userId uint64) []uint64 {
	var (
		ps      = []*PrivatePhoto{}
		userIDs = []uint64{}
	)
	DB.Where("source_user_id = ?", userId).Find(&ps)
	for _, row := range ps {
		userIDs = append(userIDs, row.TargetUserID)
	}
	return userIDs
}

// PrivateGranteeAreExplicitUserIDs gets your private photo grantees who have opted-in to see explicit content.
func PrivateGranteeAreExplicitUserIDs(userId uint64) []uint64 {
	var (
		userIDs = []uint64{}
	)

	err := DB.Table(
		"private_photos",
	).Joins(
		"JOIN users ON (users.id = private_photos.target_user_id)",
	).Select(
		"private_photos.target_user_id AS user_id",
	).Where(
		"source_user_id = ? AND users.explicit IS TRUE",
		userId,
	).Scan(&userIDs)

	if err.Error != nil {
		log.Error("PrivateGranteeAreExplicitUserIDs: %s", err.Error)
	}

	return userIDs
}

/*
PaginatePrivatePhotoList views a user's list of private photo grants.

If grantee is true, it returns the list of users who have granted YOU access to see THEIR
private photos. If grantee is false, it returns the users that YOU have granted access to
see YOUR OWN private photos.
*/
func PaginatePrivatePhotoList(user *User, grantee bool, pager *Pagination) ([]*User, error) {
	var (
		pbs          = []*PrivatePhoto{}
		userIDs      = []uint64{}
		query        *gorm.DB
		wheres       = []string{}
		placeholders = []interface{}{}

		// Column name of "other user" depending on direction
		otherUserColumn string
	)

	// Which direction are we going?
	if grantee {
		// Return the private photo grants for whom YOU are the recipient.
		wheres = append(wheres, "target_user_id = ?")
		placeholders = append(placeholders, user.ID)
		otherUserColumn = "source_user_id"
	} else {
		// Return the users that YOU have granted access to YOUR private pictures.
		wheres = append(wheres, "source_user_id = ?")
		placeholders = append(placeholders, user.ID)
		otherUserColumn = "target_user_id"
	}

	// Filter out users who are banned/disabled.
	wheres = append(wheres,
		fmt.Sprintf(`
			EXISTS (
				SELECT 1
				FROM users
				WHERE private_photos.%s = users.id
				AND users.status = 'active'
			)`,
			otherUserColumn,
		),
	)

	// Filter blocked users.
	bw, bp := BlockedUserSubquery(otherUserColumn, user.ID)
	wheres = append(wheres, bw)
	placeholders = append(placeholders, bp...)

	query = DB.Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	)

	query = query.Order(pager.Sort)
	query.Model(&PrivatePhoto{}).Count(&pager.Total)
	result := query.Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&pbs)
	if result.Error != nil {
		return nil, result.Error
	}

	// Now of these user IDs get their User objects.
	for _, b := range pbs {
		if grantee {
			userIDs = append(userIDs, b.SourceUserID)
		} else {
			userIDs = append(userIDs, b.TargetUserID)
		}
	}

	return GetUsers(user, userIDs)
}

// Save photo.
func (pb *PrivatePhoto) Save() error {
	result := DB.Save(pb)
	return result.Error
}

// PrivateGranteeMap maps user IDs to whether they have granted you their private photos.
type PrivateGranteeMap map[uint64]bool

// MapPrivatePhotoGrantee looks up a set of user IDs in bulk and returns a PrivateGranteeMap suitable for templates.
func MapPrivatePhotoGrantee(currentUser *User, users []*User) PrivateGranteeMap {
	var (
		usermap  = PrivateGranteeMap{}
		set      = map[uint64]interface{}{}
		distinct = []uint64{}
	)

	// Uniqueify users.
	for _, user := range users {
		if _, ok := set[user.ID]; ok {
			continue
		}
		set[user.ID] = nil
		distinct = append(distinct, user.ID)
	}

	var (
		matched = []*PrivatePhoto{}
		result  = DB.Model(&PrivatePhoto{}).
			Where("target_user_id = ? AND source_user_id IN ?", currentUser.ID, distinct).
			Find(&matched)
	)

	if result.Error == nil {
		for _, row := range matched {
			usermap[row.SourceUserID] = true
		}
	}

	return usermap
}

// Get a user from the PrivateGranteeMap.
func (um PrivateGranteeMap) Get(id uint64) bool {
	return um[id]
}

// PrivateGrantedMap maps user IDs to whether we have granted our private pictures to them.
type PrivateGrantedMap map[uint64]bool

// MapPrivatePhotoGranted looks up a set of user IDs in bulk and returns a PrivateGrantedMap suitable for templates.
func MapPrivatePhotoGranted(currentUser *User, users []*User) PrivateGrantedMap {
	var (
		usermap  = PrivateGrantedMap{}
		set      = map[uint64]interface{}{}
		distinct = []uint64{}
	)

	// Uniqueify users.
	for _, user := range users {
		if _, ok := set[user.ID]; ok {
			continue
		}
		set[user.ID] = nil
		distinct = append(distinct, user.ID)
	}

	var (
		matched = []*PrivatePhoto{}
		result  = DB.Model(&PrivatePhoto{}).
			Where("source_user_id = ? AND target_user_id IN ?", currentUser.ID, distinct).
			Find(&matched)
	)

	if result.Error == nil {
		for _, row := range matched {
			usermap[row.TargetUserID] = true
		}
	}

	return usermap
}

// Get a user from the PrivateGrantedMap.
func (um PrivateGrantedMap) Get(id uint64) bool {
	return um[id]
}
