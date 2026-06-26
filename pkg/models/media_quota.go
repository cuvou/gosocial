package models

import (
	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/utility"
)

// Functions related to the user's media quota for photos
// and comment photos.

// MediaQuota carries details about the current user's usage.
type MediaQuota struct {
	User *User

	// Count and file size of Gallery photos.
	GalleryPhotoCount int64
	GalleryPhotoSize  int64

	// Count and file size of Comment photos.
	CommentPhotoCount int64
	CommentPhotoSize  int64
}

// IsOverQuota checks if the user is currently exceeding their storage quota.
//
// Returns a boolean (true if over quota), the MediaQuota object, and an error.
// In case of error, the boolean will be true.
func IsOverQuota(user *User) (bool, *MediaQuota, error) {
	quota, err := GetUserMediaQuota(user)
	if err != nil {
		return true, quota, err
	}

	return quota.IsOver(), quota, nil
}

// GetUserMediaQuota returns details about the user's media usage.
func GetUserMediaQuota(user *User) (*MediaQuota, error) {

	type record struct {
		MetricType  string // gallery vs. forum photos
		MetricSize  int64  // total filesize
		MetricCount int64  // total count
	}
	var records []record
	res := DB.Raw(`
		-- Gallery photos
		WITH subquery_gallery AS (
			SELECT
				SUM(filesize) AS total_filesize,
				COUNT(*) AS total_count
			FROM photos
			WHERE user_id = ?
		),

		-- Forum photos
		subquery_comment_photos AS (
			SELECT
				SUM(filesize) AS total_filesize,
				COUNT(*) AS total_count
			FROM comment_photos
			WHERE user_id = ?
			AND filename <> ''
		)

		-- Combine the data.
		SELECT
			'photos' AS metric_type,
			total_filesize AS metric_size,
			total_count AS metric_count
		FROM subquery_gallery

		UNION ALL

		SELECT
			'comment_photos' AS metric_type,
			total_filesize AS metric_size,
			total_count AS metric_count
		FROM subquery_comment_photos
	`, user.ID, user.ID, user.ID).Scan(&records)
	if res.Error != nil {
		return nil, res.Error
	}

	// Load the records in.
	result := &MediaQuota{
		User: user,
	}
	for _, row := range records {
		switch row.MetricType {
		case "photos":
			result.GalleryPhotoCount = row.MetricCount
			result.GalleryPhotoSize = row.MetricSize
		case "comment_photos":
			result.CommentPhotoCount = row.MetricCount
			result.CommentPhotoSize = row.MetricSize
		}
	}

	return result, nil
}

// TotalSize returns the total filesize usage of the quota.
func (q *MediaQuota) TotalSize() int64 {
	return q.GalleryPhotoSize + q.CommentPhotoSize
}

// MaxSize returns the user's maximum filesize quota.
func (q *MediaQuota) MaxSize() int64 {
	return config.MediaQuotaMaxLimit
}

// TotalCount returns the total counts of the quota.
func (q *MediaQuota) TotalCount() int64 {
	return q.GalleryPhotoCount + q.CommentPhotoCount
}

// IsOver returns if the quota is currently exceeded.
func (q *MediaQuota) IsOver() bool {
	return q.TotalSize() >= q.MaxSize()
}

// PercentUsage returns the percent of file storage against the user's quota.
func (q *MediaQuota) PercentUsage() string {
	return utility.FormatFloatToPrecision(q.PercentUsageFloat(), 1)
}

// PercentUsageFloat returns the percent usage as a float64.
func (q *MediaQuota) PercentUsageFloat() float64 {
	var (
		used  = q.TotalSize()
		limit = q.MaxSize()
		pct   = ((float64(used) / float64(limit)) * 100)
	)
	return pct
}

// ProgressBarClass returns the CSS class to color the progress bar.
func (q *MediaQuota) ProgressBarClass() string {
	var pct = q.PercentUsageFloat()
	if pct < 25 {
		return "is-success"
	}
	if pct < 70 {
		return "is-link"
	}
	if pct < 85 {
		return "is-warning"
	}
	return "is-danger"
}
