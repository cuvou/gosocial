package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/log"
)

// ChangeLog table to track updates to the database.
type ChangeLog struct {
	ID          uint64 `gorm:"primaryKey"`
	AboutUserID uint64 `gorm:"index"`
	AdminUserID uint64 `gorm:"index"` // if an admin edits a user's item
	TableName   string `gorm:"index"`
	TableID     uint64 `gorm:"index"`
	Event       string `gorm:"index"`
	Message     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Types of ChangeLog events.
const (
	ChangeLogCreated = "created"
	ChangeLogUpdated = "updated"
	ChangeLogDeleted = "deleted"

	// Certification photos.
	ChangeLogApproved = "approved"
	ChangeLogRejected = "rejected"

	// Account status updates for easier filtering.
	ChangeLogBanned    = "banned"
	ChangeLogAdmin     = "admin"     // admin status toggle
	ChangeLogLifecycle = "lifecycle" // de/reactivated accounts
	ChangeLogAnalytics = "analytics" // misc analytics
)

var ChangeLogEventTypes = []string{
	ChangeLogCreated,
	ChangeLogUpdated,
	ChangeLogDeleted,
	ChangeLogApproved,
	ChangeLogRejected,
	ChangeLogBanned,
	ChangeLogAdmin,
	ChangeLogLifecycle,
	ChangeLogAnalytics,
}

// PaginateChangeLog lists the change logs.
func PaginateChangeLog(tableName string, tableID, aboutUserID, adminUserID uint64, event string, search *Search, pager *Pagination) ([]*ChangeLog, error) {
	var (
		cl           = []*ChangeLog{}
		where        = []string{}
		placeholders = []interface{}{}
	)

	if tableName != "" {
		where = append(where, "table_name = ?")
		placeholders = append(placeholders, tableName)
	}

	if tableID != 0 {
		where = append(where, "table_id = ?")
		placeholders = append(placeholders, tableID)
	}

	if aboutUserID != 0 {
		where = append(where, "about_user_id = ?")
		placeholders = append(placeholders, aboutUserID)
	}

	if adminUserID != 0 {
		where = append(where, "admin_user_id = ?")
		placeholders = append(placeholders, adminUserID)
	}

	if event != "" {
		where = append(where, "event = ?")
		placeholders = append(placeholders, event)
	}

	// Text search terms
	for _, term := range search.Includes {
		var ilike = "%" + strings.ToLower(term) + "%"
		where = append(where, "change_logs.message ILIKE ?")
		placeholders = append(placeholders, ilike)
	}
	for _, term := range search.Excludes {
		var ilike = "%" + strings.ToLower(term) + "%"
		where = append(where, "change_logs.message NOT ILIKE ?")
		placeholders = append(placeholders, ilike)
	}

	query := DB.Model(&ChangeLog{}).Where(
		strings.Join(where, " AND "),
		placeholders...,
	).Order(
		pager.Sort,
	)

	query.Count(&pager.Total)
	result := query.Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&cl)
	return cl, result.Error
}

// ChangeLogTables returns all the distinct table_names appearing in the change log.
func ChangeLogTables() []string {
	var result = []string{}

	query := DB.Model(&ChangeLog{}).
		Select("DISTINCT change_logs.table_name").
		Group("change_logs.table_name").
		Find(&result)
	if query.Error != nil {
		log.Error("ChangeLogTables: %s", query.Error)
	}

	sort.Strings(result)

	return result
}

// LogEvent puts in a generic/miscellaneous change log event (e.g. certification photo updates).
func LogEvent(aboutUser, adminUser *User, event, tableName string, tableID uint64, message string) (*ChangeLog, error) {
	cl := &ChangeLog{
		TableName: tableName,
		TableID:   tableID,
		Event:     event,
		Message:   message,
	}

	if aboutUser != nil {
		cl.AboutUserID = aboutUser.ID
	}

	if adminUser != nil && adminUser != aboutUser {
		cl.AdminUserID = adminUser.ID
	}

	result := DB.Create(cl)
	return cl, result.Error
}

// LogCreated puts in a ChangeLog "created" event.
func LogCreated(aboutUser *User, tableName string, tableID uint64, message string) (*ChangeLog, error) {
	cl := &ChangeLog{
		TableName: tableName,
		TableID:   tableID,
		Event:     ChangeLogCreated,
		Message:   message,
	}

	if aboutUser != nil {
		cl.AboutUserID = aboutUser.ID
	}

	result := DB.Create(cl)
	return cl, result.Error
}

// LogDeleted puts in a ChangeLog "deleted" event.
func LogDeleted(aboutUser, adminUser *User, tableName string, tableID uint64, message string, original interface{}) (*ChangeLog, error) {
	// If the original model is given, JSON serialize it nicely.
	if original != nil {
		w := bytes.NewBuffer([]byte{})
		enc := json.NewEncoder(w)
		enc.SetIndent("\n", "* ")
		if err := enc.Encode(original); err != nil {
			log.Error("LogDeleted(%s %d): couldn't encode original model to JSON: %s", tableName, tableID, err)
		} else {
			message += "\n\n" + w.String()
		}
	}

	cl := &ChangeLog{
		TableName: tableName,
		TableID:   tableID,
		Event:     ChangeLogDeleted,
		Message:   message,
	}

	if aboutUser != nil {
		cl.AboutUserID = aboutUser.ID
	}

	if adminUser != nil && adminUser != aboutUser {
		cl.AdminUserID = adminUser.ID
	}

	result := DB.Create(cl)
	return cl, result.Error
}

type FieldDiff struct {
	Key    string
	Before interface{}
	After  interface{}
}

func NewFieldDiff(key string, before, after interface{}) FieldDiff {
	return FieldDiff{
		Key:    key,
		Before: before,
		After:  after,
	}
}

// LogUpdated puts in a ChangeLog "updated" event.
func LogUpdated(aboutUser, adminUser *User, tableName string, tableID uint64, message string, diffs []FieldDiff) (*ChangeLog, error) {
	// Append field diffs to the message?
	lines := []string{message}
	if len(diffs) > 0 {
		lines = append(lines, "")
		for _, row := range diffs {
			var (
				before = fmt.Sprintf("%v", row.Before)
				after  = fmt.Sprintf("%v", row.After)
			)

			if before != after {
				lines = append(lines,
					fmt.Sprintf("* **%s** changed to <code>%s</code> from <code>%s</code>",
						row.Key,
						strings.ReplaceAll(after, "`", "'"),
						strings.ReplaceAll(before, "`", "'"),
					),
				)
			}
		}
	}

	cl := &ChangeLog{
		TableName: tableName,
		TableID:   tableID,
		Event:     ChangeLogUpdated,
		Message:   strings.Join(lines, "\n"),
	}

	if aboutUser != nil {
		cl.AboutUserID = aboutUser.ID
	}

	if adminUser != nil && adminUser != aboutUser {
		cl.AdminUserID = adminUser.ID
	}

	result := DB.Create(cl)
	return cl, result.Error
}
