package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/log"
)

// Friend table.
type Friend struct {
	ID           uint64 `gorm:"primaryKey"`
	SourceUserID uint64 `gorm:"index"`
	TargetUserID uint64 `gorm:"index"`
	Approved     bool   `gorm:"index"`
	Ignored      bool
	Message      *string `gorm:"index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time `gorm:"index"`
}

// AddFriend sends a friend request or accepts one if there was already a pending one.
//
// A message can be included in the friend request. If the message is blank, a NULL value is stored in the
// database. Friend requests with messages are prioritized for the user's display (ones with messages appear
// on top over the ones without).
//
// If a friend request with a message is accepted, the message is copied into a DM to the target user
// so that they may refer back to it after accepting.
func AddFriend(sourceUserID, targetUserID uint64, message string) error {
	// Did we already send a friend request?
	f := &Friend{}
	forward := DB.Where(
		"source_user_id = ? AND target_user_id = ?",
		sourceUserID, targetUserID,
	).First(&f).Error

	// Is there a reverse friend request pending?
	rev := &Friend{}
	reverse := DB.Where(
		"source_user_id = ? AND target_user_id = ?",
		targetUserID, sourceUserID,
	).First(&rev).Error

	// If we have previously Ignored the friend request, we can not Accept it but only Reject it.
	if reverse == nil && rev.Ignored {
		return errors.New(
			"you have previously ignored a friend request from this person and can not accept it now - " +
				"please go to your Friends page, Ignored tab, and remove the ignored friend request there and try again",
		)
	}

	// If the reverse exists (requested us) but not the forward, this completes the friendship.
	if reverse == nil && forward != nil {
		// Approve the reverse.
		rev.Approved = true
		rev.Ignored = false
		rev.Save()

		// Add the matching forward.
		f = &Friend{
			SourceUserID: sourceUserID,
			TargetUserID: targetUserID,
			Approved:     true,
			Ignored:      false,
		}

		// Attach a message if one was sent, otherwise leave the message as NULL.
		if len(message) > 0 {
			f.Message = &message
		}
		if err := DB.Create(f).Error; err != nil {
			return err
		}

		// If either friend request had messages attached, move them into DMs.
		go func() {
			FriendRequestMessageToDM(rev)
			FriendRequestMessageToDM(f)
		}()
		return nil
	}

	// If the forward already existed, error.
	if forward == nil {
		if f.Approved {
			return errors.New("you are already friends")
		}
		return errors.New("a friend request had already been sent")
	}

	// Create the pending forward request.
	f = &Friend{
		SourceUserID: sourceUserID,
		TargetUserID: targetUserID,
		Approved:     false,
	}

	// Attach a message if one was sent, otherwise leave the message as NULL.
	if len(message) > 0 {
		f.Message = &message
	}

	return DB.Create(f).Error
}

// AreFriends quickly checks if two user IDs are friends.
func AreFriends(sourceUserID, targetUserID uint64) bool {
	f := &Friend{}
	DB.Where(
		"source_user_id = ? AND target_user_id = ?",
		sourceUserID, targetUserID,
	).First(&f)
	return f.Approved
}

// HasIgnoredFriendRequest checks whether sourceUserID had sent a friend request to targetUserID and it was ignored.
func HasIgnoredFriendRequest(sourceUserID, targetUserID uint64) bool {
	f := &Friend{}
	DB.Where(
		"source_user_id = ? AND target_user_id = ? AND ignored IS true",
		sourceUserID, targetUserID,
	).First(&f)
	return f.Ignored
}

// GetBothFriends will look up the Friend row for both the source and target user, if they exist.
//
// The forward friend request is from the source user to the target, and the reverse request is
// from the target user back to the source. If there is no friendship or friend request between
// these user IDs, both forward and reverse will be nil.
//
// The error should only return on a database error, but will usually be nil.
func GetBothFriends(sourceUserID, targetUserID uint64) (forward, reverse *Friend, err error) {
	var fs []*Friend
	err = DB.Where(
		"(source_user_id = ? AND target_user_id = ?) OR (source_user_id = ? AND target_user_id = ?)",
		sourceUserID, targetUserID, targetUserID, sourceUserID,
	).Find(&fs).Error

	// Map the forward and reverse requests.
	for _, row := range fs {
		switch row.SourceUserID {
		case sourceUserID:
			forward = row
		case targetUserID:
			reverse = row
		}
	}

	return
}

/*
FriendStatus returns an indicator of friendship status.

Possible returned values are:

* approved: the two users are already friends.
* pending: the source user has a sent friend request to the target.
* requested: the target user has a friend request to the source.
* none: there is no friendship or request between the two users.
*/
func FriendStatus(sourceUserID, targetUserID uint64) string {
	// Look for friendships (either direction).
	forward, reverse, err := GetBothFriends(sourceUserID, targetUserID)

	// No friendships or requests, either direction?
	if err != nil || (forward == nil && reverse == nil) {
		return "none"
	}

	// Our friend status to the target.
	if forward != nil {
		if forward.Approved {
			return "approved"
		}
		return "pending"
	}

	// Was there a pending request from the reverse?
	if reverse != nil {
		return "requested"
	}

	return "none"
}

// FriendRequestMessageToDM will copy an attached friend request message into a DM conversation when
// the friend request is approved.
func FriendRequestMessageToDM(f *Friend) (*Message, error) {
	if f.Message != nil {
		message := *f.Message

		// First attempt to create the DM.
		msg, err := SendMessage(
			f.SourceUserID,
			f.TargetUserID,
			"_This is an automated message. My friendship request to you included this introduction "+
				"message, which has been copied below for your reference:_\n\n"+
				message,
		)
		if err != nil {
			return nil, err
		}

		// Then clear the message off the original friend request.
		f.Message = nil
		f.Save()
		return msg, err
	}
	return nil, nil
}

// FriendIDs returns all user IDs with approved friendship to the user.
func FriendIDs(userId uint64) []uint64 {
	var (
		fs      = []*Friend{}
		userIDs = []uint64{}
	)
	DB.Where("source_user_id = ? AND approved = ?", userId, true).Find(&fs)
	for _, row := range fs {
		userIDs = append(userIDs, row.TargetUserID)
	}
	return userIDs
}

// FilterFriendIDs can filter down a listing of user IDs and return only the ones who are your friends.
func FilterFriendIDs(userIDs []uint64, friendIDs []uint64) []uint64 {
	var (
		seen     = map[uint64]any{}
		filtered = []uint64{}
	)

	// Map the friend IDs out.
	for _, friendID := range friendIDs {
		seen[friendID] = nil
	}

	// Filter the userIDs.
	for _, userID := range userIDs {
		if _, ok := seen[userID]; ok {
			filtered = append(filtered, userID)
		}
	}

	return filtered
}

// FilterFriendUsernames takes a list of usernames and returns only the ones who are your friends.
func FilterFriendUsernames(currentUser *User, usernames []string) []string {
	var (
		fs      = []*Friend{}
		userIDs = []uint64{}
		userMap = map[uint64]string{}
		result  = []string{}
	)

	// Map usernames to user IDs.
	users, err := GetUsersByUsernames(currentUser, usernames)
	if err != nil {
		log.Error("FilterFriendUsernames: GetUsersByUsernames: %s", err)
		return result
	}

	for _, user := range users {
		userIDs = append(userIDs, user.ID)
		userMap[user.ID] = user.Username
	}

	if len(userIDs) == 0 {
		return result
	}

	DB.Where("source_user_id = ? AND approved = ? AND target_user_id IN ?", currentUser.ID, true, userIDs).Find(&fs)
	for _, row := range fs {
		result = append(result, userMap[row.TargetUserID])
	}

	return result
}

// FriendIDsAreExplicit returns friend IDs who have opted-in for Explicit content,
// e.g. to notify only them when you uploaded a new Explicit photo so that non-explicit
// users don't need to see that notification.
func FriendIDsAreExplicit(userId uint64) []uint64 {
	var (
		userIDs = []uint64{}
	)

	err := DB.Table(
		"friends",
	).Joins(
		"JOIN users ON (users.id = friends.target_user_id)",
	).Select(
		"friends.target_user_id AS friend_id",
	).Where(
		"friends.source_user_id = ? AND friends.approved = ? AND users.explicit = ?",
		userId, true, true,
	).Scan(&userIDs)

	if err.Error != nil {
		log.Error("SQL error collecting explicit FriendIDs for %d: %s", userId, err.Error)
	}

	return userIDs
}

// FriendIDsInCircle returns friend IDs who are part of the inner circle.
func FriendIDsInCircle(userId uint64) []uint64 {
	var (
		userIDs = []uint64{}
	)

	err := DB.Table(
		"friends",
	).Joins(
		"JOIN users ON (users.id = friends.target_user_id)",
	).Select(
		"friends.target_user_id AS friend_id",
	).Where(
		"friends.source_user_id = ? AND friends.approved = ? AND (users.inner_circle = ? OR users.is_admin = ?)",
		userId, true, true, true,
	).Scan(&userIDs)

	if err.Error != nil {
		log.Error("SQL error collecting circle FriendIDs for %d: %s", userId, err.Error)
	}

	return userIDs
}

// FriendIDsInCircleAreExplicit returns the combined friend IDs who are in the inner circle + have opted in to explicit content.
// It is the combination of FriendIDsAreExplicit and FriendIDsInCircle.
func FriendIDsInCircleAreExplicit(userId uint64) []uint64 {
	var (
		userIDs = []uint64{}
	)

	err := DB.Table(
		"friends",
	).Joins(
		"JOIN users ON (users.id = friends.target_user_id)",
	).Select(
		"friends.target_user_id AS friend_id",
	).Where(
		"friends.source_user_id = ? AND friends.approved = ? AND users.explicit = ? AND (users.inner_circle = ? OR users.is_admin = ?)",
		userId, true, true, true, true,
	).Scan(&userIDs)

	if err.Error != nil {
		log.Error("SQL error collecting explicit FriendIDs for %d: %s", userId, err.Error)
	}

	return userIDs
}

// CountFriendRequests gets a count of pending requests for the user.
func CountFriendRequests(userID uint64) (int64, error) {
	var (
		count  int64
		wheres = []string{
			"target_user_id = ? AND approved = ? AND ignored IS NOT true",
			"EXISTS (SELECT 1 FROM users WHERE users.id = source_user_id AND users.status = 'active')",
		}
		placeholders = []interface{}{
			userID, false,
		}
	)
	result := DB.Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	).Model(&Friend{}).Count(&count)
	return count, result.Error
}

// CountIgnoredFriendRequests gets a count of ignored pending friend requests for the user.
func CountIgnoredFriendRequests(userID uint64) (int64, error) {
	var count int64
	result := DB.Where(
		"target_user_id = ? AND approved = ? AND ignored = ? AND EXISTS (SELECT 1 FROM users WHERE users.id = friends.source_user_id AND users.status = 'active')",
		userID,
		false,
		true,
	).Model(&Friend{}).Count(&count)
	return count, result.Error
}

// CountFriends gets a count of friends for the user.
func CountFriends(userID uint64) int64 {
	var count int64
	result := DB.Where(
		"target_user_id = ? AND approved = ? AND EXISTS (SELECT 1 FROM users WHERE users.id = friends.source_user_id AND users.status = 'active')",
		userID,
		true,
	).Model(&Friend{}).Count(&count)
	if result.Error != nil {
		log.Error("CountFriends(%d): %s", userID, result.Error)
	}
	return count
}

// CountMutualFriends gets a count of friends in common between the two users.
//
// Always returns zero when the two users are the same.
func CountMutualFriends(currentUser, user *User) int64 {
	var count int64

	if currentUser.ID == user.ID {
		return 0
	}

	result := DB.Raw(`
			SELECT
				COUNT(ours.target_user_id)
			FROM friends AS theirs
			JOIN friends AS ours ON (
				ours.source_user_id = ?
				AND ours.target_user_id = theirs.target_user_id
				AND ours.approved iS TRUE
			)
			WHERE theirs.source_user_id = ?
			AND theirs.approved IS TRUE

			-- active users only
			AND EXISTS (
				SELECT 1 FROM users WHERE users.id = theirs.source_user_id AND users.status = 'active'
			)

			-- TODO: not blocked mutuals
		`,
		currentUser.ID, // 'ours'
		user.ID,        // 'theirs'
	).Count(&count)
	if result.Error != nil {
		log.Error("CountFriends(%d): %s", user.ID, result.Error)
	}
	return count
}

/*
PaginateFriends gets a page of friends (or pending friend requests) as User objects ordered
by friendship date.

The `requests` and `sent` bools are mutually exclusive (use only one, or neither). `requests`
asks for unanswered friend requests to you, and `sent` returns the friend requests that you
have sent and have not been answered.
*/
func PaginateFriends(user *User, requests bool, sent bool, ignored bool, pager *Pagination) ([]*User, error) {
	// We paginate over the Friend table.
	var (
		fs           = []*Friend{}
		userIDs      = []uint64{}
		wheres       = []string{}
		placeholders = []interface{}{}
		query        = DB.Model(&Friend{})
	)

	if requests && sent && ignored {
		return nil, errors.New("requests and sent are mutually exclusive options, use one or neither")
	}

	// Don't show our blocked users in the result.
	bw, bp := BlockedUserSubquery("target_user_id", user.ID)
	wheres = append(wheres, bw)
	placeholders = append(placeholders, bp...)

	// Don't show disabled or banned users.
	var (
		// Source user is banned (Requests, Ignored tabs)
		bannedWhereRequest = `
			EXISTS (
				SELECT 1
				FROM users
				WHERE users.id = friends.source_user_id
				AND users.status = 'active'
			)
		`

		// Target user is banned (Friends, Sent tabs)
		bannedWhereFriend = `
			EXISTS (
				SELECT 1
				FROM users
				WHERE users.id = friends.target_user_id
				AND users.status = 'active'
			)
		`
	)

	if requests {
		wheres = append(wheres, "target_user_id = ? AND approved = ? AND ignored IS NOT true")
		placeholders = append(placeholders, user.ID, false)

		// Don't show friend requests from currently banned/disabled users.
		wheres = append(wheres, bannedWhereRequest)
	} else if sent {
		wheres = append(wheres, "source_user_id = ? AND approved = ? AND ignored IS NOT true")
		placeholders = append(placeholders, user.ID, false)

		// Don't show friends who are currently banned/disabled.
		wheres = append(wheres, bannedWhereFriend)
	} else if ignored {
		wheres = append(wheres, "target_user_id = ? AND approved = ? AND ignored = ?")
		placeholders = append(placeholders, user.ID, false, true)

		// Don't show friend requests from currently banned/disabled users.
		wheres = append(wheres, bannedWhereRequest)
	} else {
		wheres = append(wheres, "source_user_id = ? AND approved = ?")
		placeholders = append(placeholders, user.ID, true)

		// Don't show friends who are currently banned/disabled.
		wheres = append(wheres, bannedWhereFriend)
	}

	// Sorting by username? Join the users table.
	if strings.HasPrefix(pager.Sort, "users.") {
		query = query.Joins(
			"JOIN users ON friends.target_user_id = users.id",
		)
	}

	query = query.Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	).Order(pager.Sort)
	query.Model(&Friend{}).Count(&pager.Total)
	result := query.Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&fs)
	if result.Error != nil {
		return nil, result.Error
	}

	// Now of these friends get their User objects.
	for _, friend := range fs {
		if requests || ignored {
			userIDs = append(userIDs, friend.SourceUserID)
		} else {
			userIDs = append(userIDs, friend.TargetUserID)
		}
	}

	return GetUsers(user, userIDs)
}

/*
PaginateOtherUserFriends gets a page of friends from another user, for their profile page.
*/
func PaginateOtherUserFriends(currentUser *User, user *User, mutuals bool, pager *Pagination) ([]*User, error) {
	// We paginate over the Friend table.
	var (
		fs           = []*Friend{}
		userIDs      = []uint64{}
		wheres       = []string{}
		placeholders = []interface{}{}
		query        = DB.Model(&Friend{}).Joins(
			"JOIN users ON (friends.target_user_id = users.id)",
		)
	)

	// Get friends of the target user.
	wheres = append(wheres, "friends.source_user_id = ? AND friends.approved = ?")
	placeholders = append(placeholders, user.ID, true)

	// Don't show our blocked users in the result.
	bw, bp := BlockedUserSubquery("friends.target_user_id", currentUser.ID)
	wheres = append(wheres, bw)
	placeholders = append(placeholders, bp...)

	// Don't show disabled or banned users.
	wheres = append(wheres, "users.status = ?")
	placeholders = append(placeholders, UserStatusActive)

	// Are we narrowing to our mutual friends?
	if mutuals {
		wheres = append(wheres, `
			EXISTS (
				SELECT 1
				FROM friends AS my_friends
				WHERE my_friends.source_user_id = ?
				AND my_friends.target_user_id = friends.target_user_id
				AND my_friends.approved IS TRUE
			)
		`)
		placeholders = append(placeholders, currentUser.ID)
	}

	query = query.Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	).Order(pager.Sort)

	// Get the total count.
	query.Count(&pager.Total)

	// Get the page.
	result := query.Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&fs)
	if result.Error != nil {
		return nil, result.Error
	}

	// Now of these friends get their User objects.
	for _, friend := range fs {
		userIDs = append(userIDs, friend.TargetUserID)
	}

	return GetUsers(currentUser, userIDs)
}

// GetFriendRequests returns all pending friend requests for a user.
func GetFriendRequests(userID uint64) ([]*Friend, error) {
	var fs = []*Friend{}
	result := DB.Where(
		"target_user_id = ? AND approved = ?",
		userID,
		false,
	).Find(&fs)
	return fs, result.Error
}

// IgnoreFriendRequest ignores a pending friend request that was sent to targetUserID.
func IgnoreFriendRequest(currentUser *User, fromUser *User) error {
	// Is there a reverse friend request pending? (The one we ideally hope to mark Ignored)
	rev := &Friend{}
	reverse := DB.Where(
		"source_user_id = ? AND target_user_id = ?",
		fromUser.ID, currentUser.ID,
	).First(&rev).Error

	// If the reverse exists (requested us) mark it as Ignored.
	if reverse == nil {
		// Ignore the reverse friend request (happy path).
		log.Error("%s ignoring friend request from %s", currentUser.Username, fromUser.Username)
		rev.Approved = false
		rev.Ignored = true
		return rev.Save()
	}

	log.Error("rev: %+v", rev)
	return errors.New("unexpected error while ignoring friend request")
}

// RemoveFriend severs a friend connection both directions, used when
// rejecting a request or removing a friend.
func RemoveFriend(sourceUserID, targetUserID uint64) error {
	result := DB.Where(
		"(source_user_id = ? AND target_user_id = ?) OR "+
			"(target_user_id = ? AND source_user_id = ?)",
		sourceUserID, targetUserID,
		sourceUserID, targetUserID,
	).Delete(&Friend{})
	return result.Error
}

// RevokeFriendPhotoNotifications removes notifications about newly uploaded friends photos
// that were sent to your former friends, when you remove their friendship.
//
// For example: if I unfriend you, all your past notifications that showed my friends-only photos should
// be revoked so that you can't see them anymore.
//
// Notifications about friend photos are revoked going in both directions.
func RevokeFriendPhotoNotifications(currentUser, other *User) error {
	// Gather the IDs of all their friends-only photos to nuke notifications for.
	allPhotoIDs, err := AllFriendsOnlyPhotoIDs(currentUser, other)
	if err != nil {
		return err
	} else if len(allPhotoIDs) == 0 {
		// Nothing to do.
		return nil
	}

	log.Info("RevokeFriendPhotoNotifications(%s): forget about friend photo uploads for user %s on photo IDs: %v", currentUser.Username, other.Username, allPhotoIDs)
	return RemoveSpecificNotificationBulk([]*User{currentUser, other}, NotificationNewPhoto, "photos", allPhotoIDs)
}

// Save photo.
func (f *Friend) Save() error {
	result := DB.Save(f)
	return result.Error
}

// FriendMap maps user IDs to friendship status (booleans) for the current user.
type FriendMap map[uint64]bool

// MapFriends looks up a set of user IDs in bulk and returns a FriendMap suitable for templates.
func MapFriends(currentUser *User, users []*User) FriendMap {
	var (
		usermap  = FriendMap{}
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
		matched = []*Friend{}
		result  = DB.Model(&Friend{}).Where(
			"source_user_id = ? AND target_user_id IN ? AND approved = ?",
			currentUser.ID, distinct, true,
		).Find(&matched)
	)

	if result.Error == nil {
		for _, row := range matched {
			usermap[row.TargetUserID] = true
		}
	}

	return usermap
}

// Get a user from the FriendMap.
func (um FriendMap) Get(id uint64) bool {
	return um[id]
}

// FriendRequestMap maps user IDs to their Friend Request objects, e.g. for the Friend Requests page
// so we can show attached messages on those requests.
type FriendRequestMap map[uint64]*Friend

/*
MapFriendRequests looks up a set of user IDs in bulk and returns a FriendRequestMap suitable for templates.

The view parameter controls which direction and status of friend requests to return, and corresponds
to the various tabs of the Friends page of the website:

- friends: get the currentUser's approved friendship list.
- requests: get current un-approved friend requests targeted to the currentUser.
- pending: get un-approved friend requests sent by currentUser to others.
- ignored: get friend requests targeted to the currentUser which have been ignored.
*/
func MapFriendRequests(currentUser *User, users []*User, view string) (FriendRequestMap, error) {
	var (
		usermap  = FriendRequestMap{}
		set      = map[uint64]interface{}{}
		distinct = []uint64{}

		wheres       = []string{}
		placeholders = []interface{}{}

		// Depending on the direction, is the SourceUserID the field to pull from the result set?
		takeSourceUserID bool
	)

	// Uniqueify users.
	for _, user := range users {
		if _, ok := set[user.ID]; ok {
			continue
		}
		set[user.ID] = nil
		distinct = append(distinct, user.ID)
	}

	// Direction of the friend requests.
	switch view {
	case "friends":
		wheres = append(wheres, "source_user_id = ? AND target_user_id IN ? AND approved IS TRUE")
		placeholders = append(placeholders, currentUser.ID, distinct)
	case "requests":
		wheres = append(wheres, "target_user_id = ? AND source_user_id IN ? AND approved IS FALSE")
		placeholders = append(placeholders, currentUser.ID, distinct)
		takeSourceUserID = true
	case "pending":
		wheres = append(wheres, "source_user_id = ? AND target_user_id IN ? AND approved IS FALSE")
		placeholders = append(placeholders, currentUser.ID, distinct)
	case "ignored":
		wheres = append(wheres, "target_user_id = ? AND source_user_id IN ? AND ignored IS TRUE")
		placeholders = append(placeholders, currentUser.ID, distinct)
		takeSourceUserID = true
	default:
		return usermap, fmt.Errorf("unsupported view for FriendRequestMap: %s", view)
	}

	var (
		matched = []*Friend{}
		result  = DB.Model(&Friend{}).Where(
			strings.Join(wheres, " AND "),
			placeholders...,
		).Find(&matched)
	)

	if result.Error == nil {
		for _, row := range matched {
			if takeSourceUserID {
				usermap[row.SourceUserID] = row
			} else {
				usermap[row.TargetUserID] = row
			}
		}
	}

	return usermap, nil
}

// Get a user from the FriendMap.
func (um FriendRequestMap) Get(id uint64) *Friend {
	return um[id]
}
