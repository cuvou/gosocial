package models

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
)

// Feedback table for Contact Us & Reporting to admins.
type Feedback struct {
	ID           uint64 `gorm:"primaryKey"`
	UserID       uint64 `gorm:"index"` // if logged-in user posted this
	AboutUserID  uint64 // associated 'about' user (e.g., owner of a reported photo)
	Acknowledged bool   `gorm:"index"` // admin dashboard "read" status
	Intent       string `gorm:"index"`
	Subject      string `gorm:"index"`
	Message      string
	TableName    string
	TableID      uint64
	ReplyTo      string // logged-out user may leave their email for reply
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// GetFeedback by ID.
func GetFeedback(id uint64) (*Feedback, error) {
	m := &Feedback{}
	result := DB.First(&m, id)
	return m, result.Error
}

// CountUnreadFeedback gets the count of unacknowledged feedback for admins.
func CountUnreadFeedback() int64 {
	query := DB.Where(
		"acknowledged = ?",
		false,
	)

	var count int64
	result := query.Model(&Feedback{}).Count(&count)
	if result.Error != nil {
		log.Error("models.CountUnreadFeedback: %s", result.Error)
	}
	return count
}

// PaginateFeedback
func PaginateFeedback(acknowledged bool, intent, subject string, search *Search, pager *Pagination) ([]*Feedback, error) {
	var (
		fb           = []*Feedback{}
		wheres       = []string{}
		placeholders = []interface{}{}
	)

	wheres = append(wheres, "acknowledged = ?")
	placeholders = append(placeholders, acknowledged)

	if intent != "" {
		wheres = append(wheres, "intent = ?")
		placeholders = append(placeholders, intent)
	}

	if subject != "" {
		wheres = append(wheres, "subject = ?")
		placeholders = append(placeholders, subject)
	}

	// Search terms.
	for _, term := range search.Includes {
		var ilike = "%" + strings.ToLower(term) + "%"
		wheres = append(wheres, "message ILIKE ?")
		placeholders = append(placeholders, ilike)
	}
	for _, term := range search.Excludes {
		var ilike = "%" + strings.ToLower(term) + "%"
		wheres = append(wheres, "message NOT ILIKE ?")
		placeholders = append(placeholders, ilike)
	}

	query := DB.Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	).Order(
		pager.Sort,
	)

	query.Model(&Feedback{}).Count(&pager.Total)

	result := query.Offset(
		pager.GetOffset(),
	).Limit(pager.PerPage).Find(&fb)

	return fb, result.Error
}

// PaginateFeedbackAboutUser digs through feedback about a specific user ID or one of their Photos.
//
// It returns reports where table_name=users and their user ID, or where table_name=photos and about any
// of their current photo IDs. Additionally, it will look for chat room reports which were about their
// username.
//
// The 'show' parameter applies some basic filter choices:
//
//   - Blank string (default) = all reports From or About this user
//   - "about" = all reports About this user (by table_name=users table_id=userID, or table_name=photos
//     for any of their existing photo IDs)
//   - "from" = all reports From this user (where reporting user_id is the user's ID)
//   - "fuzzy" = fuzzy full text search on all reports that contain the user's username.
//
// The lowSensitivity flag will return admin reports that are suitable for ALL admins to view (low
// sensitivity reports), especially intended for chat moderators.
//
// Admin only: if the current user is not an admin, this returns empty.
func PaginateFeedbackAboutUser(currentUser, user *User, show string, lowSensitivity bool, pager *Pagination) ([]*Feedback, error) {
	if !currentUser.IsAdmin {
		return nil, errors.New("admin required")
	}

	var (
		fb           = []*Feedback{}
		photoIDs, _  = user.AllPhotoIDs()
		wheres       = []string{}
		placeholders = []interface{}{}
		like         = "%" + user.Username + "%"
		atLike       = "%@" + user.Username + "%"
	)

	switch show {
	case "about":
		wheres = append(wheres, `
				(
					about_user_id = ? OR
					(table_name = 'users' AND table_id = ?) OR
					(table_name = 'photos' AND table_id IN ?)
				)
			`)
		placeholders = append(placeholders, user.ID, user.ID, photoIDs)
	case "from":
		wheres = append(wheres, "user_id = ?")
		placeholders = append(placeholders, user.ID)
	case "fuzzy":
		wheres = append(wheres, "message LIKE ?")
		placeholders = append(placeholders, like)
	default:
		// Default=everything.
		wheres = append(wheres, `
				(
					user_id = ? OR
					about_user_id = ? OR
					(table_name = 'users' AND table_id = ?) OR
					(table_name = 'photos' AND table_id IN ?) OR
					message LIKE ?
				)
			`)
		placeholders = append(placeholders, user.ID, user.ID, user.ID, photoIDs, atLike)
	}

	// Limit the search to low sensitivity reports?
	if lowSensitivity {

		// Blacklist by subjects.
		wheres = append(wheres, `subject NOT IN ?`)
		placeholders = append(placeholders, config.HighSensitivityAdminFeedbackSubjects)

		// Blacklist by message substrings.
		var ors []string
		for _, substr := range config.HighSensitivityAdminFeedbackSubstrings {
			ors = append(ors, "message NOT LIKE ?")
			placeholders = append(placeholders, substr)
		}
		wheres = append(wheres, fmt.Sprintf("(%s)", strings.Join(ors, " OR ")))

	}

	query := DB.Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	).Order(
		pager.Sort,
	)

	query.Model(&Feedback{}).Count(&pager.Total)

	result := query.Offset(
		pager.GetOffset(),
	).Limit(pager.PerPage).Find(&fb)

	return fb, result.Error
}

// DistinctFeedbackSubjects returns the distinct subjects on feedback & reports.
func DistinctFeedbackSubjects() []string {
	var results = []string{}
	query := DB.Model(&Feedback{}).
		Select("DISTINCT feedbacks.subject").
		Group("feedbacks.subject").
		Find(&results)
	if query.Error != nil {
		log.Error("DistinctFeedbackSubjects: %s", query.Error)
		return nil
	}

	sort.Strings(results)
	return results
}

// CreateFeedback saves a new Feedback row to the DB.
func CreateFeedback(fb *Feedback) error {
	result := DB.Create(fb)
	return result.Error
}

// Save Feedback.
func (fb *Feedback) Save() error {
	result := DB.Save(fb)
	return result.Error
}
