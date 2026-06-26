package worker

import (
	"fmt"
	"time"

	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/photo"
)

// Vacuum runs database cleanup tasks for data consistency. Run it like `gosocial vacuum` from the CLI.
func Vacuum(dryrun bool) error {

	var steps = []struct {
		Label string
		Fn    func(bool) (int64, error)
	}{
		{"Old Notifications", VacuumOldNotifications},
		{"Old Chat Room DMs", VacuumOldDirectMessages},
		{"Comment Photos", VacuumOrphanedCommentPhotos},
		{"Photos", VacuumOrphanedPhotos},
		{"Polls", VacuumOrphanedPolls},
		{"Likes", VacuumOrphanedLikes},
		{"Expired Login Sessions", VacuumExpiredLoginSessions},
	}

	for _, step := range steps {
		log.Warn("Vacuum: %s", step.Label)
		if total, err := step.Fn(dryrun); err != nil {
			log.Error("%s: %s", step.Label, err)
		} else {
			log.Info("Removed %d rows", total)
		}
	}

	return nil
}

// VacuumOldNotifications removes Notifications older than 90 days.
func VacuumOldNotifications(dryrun bool) (int64, error) {
	var (
		cutoff = time.Now().Add(-24 * 90 * time.Hour)
		count  int64
	)
	res := models.DB.Model(&models.Notification{}).Where(
		"created_at < ?",
		cutoff,
	).Count(&count)
	if res.Error != nil {
		return count, res.Error
	}

	if !dryrun {
		res = models.DB.Exec(
			`DELETE FROM notifications WHERE created_at < ?`,
			cutoff,
		)
	}

	return count, res.Error
}

// VacuumExpiredLoginSessions removes old sessions.
func VacuumExpiredLoginSessions(dryrun bool) (int64, error) {
	var (
		now   = time.Now()
		count int64
	)
	res := models.DB.Model(&models.LoginSession{}).Where(
		"expires_at < ?",
		now,
	).Count(&count)
	if res.Error != nil {
		return count, res.Error
	}

	if !dryrun {
		res = models.DB.Exec(
			`DELETE FROM login_sessions WHERE expires_at < ?`,
			now,
		)
	}

	return count, res.Error
}

// VacuumOldDirectMessages removes chat room DMs older than 90 days.
func VacuumOldDirectMessages(dryrun bool) (int64, error) {
	var (
		cutoff = time.Now().Add(-24 * 90 * time.Hour)
		count  int64
	)
	res := models.DB.Model(&models.DirectMessage{}).Where(
		"created_at < ?",
		cutoff,
	).Count(&count)
	if res.Error != nil {
		return count, res.Error
	}

	if !dryrun {
		res = models.DB.Exec(
			`DELETE FROM direct_messages WHERE created_at < ?`,
			cutoff,
		)
	}

	return count, res.Error
}

// VacuumOrphanedPolls removes any polls with forum threads no longer pointing to them.
func VacuumOrphanedPolls(dryrun bool) (int64, error) {
	polls, count, err := models.GetOrphanedPolls()
	if err != nil {
		return count, err
	}

	if dryrun {
		return count, nil
	}

	for _, row := range polls {
		log.Info("    #%d: %s", row.ID, row.Choices)
		if err := row.Delete(); err != nil {
			return count, fmt.Errorf("deleting orphaned poll (%d): %s", row.ID, err)
		}
	}

	return count, nil
}

// VacuumOrphanedPhotos removes any lingering photo from failed account deletion.
func VacuumOrphanedPhotos(dryrun bool) (int64, error) {
	photos, count, err := models.GetOrphanedPhotos()
	if err != nil {
		return count, err
	}

	if dryrun {
		return count, nil
	}

	for _, row := range photos {
		log.Info("    #%d: %s", row.ID, row.Filename)
		if err := photo.Delete(row.Filename); err != nil {
			return count, fmt.Errorf("photo ID %d: removing file %s: %s", row.ID, row.Filename, err)
		}

		if row.CroppedFilename != "" {
			if err := photo.Delete(row.CroppedFilename); err != nil {
				return count, fmt.Errorf("photo ID %d: removing file %s: %s", row.ID, row.Filename, err)
			}
		}

		if err := row.Delete(); err != nil {
			return count, fmt.Errorf("deleting orphaned photo (%d): %s", row.ID, err)
		}
	}

	return count, nil
}

// VacuumOrphanedCommentPhotos cleans up comment photos that weren't associated to a post, returning the count removed.
func VacuumOrphanedCommentPhotos(dryrun bool) (int64, error) {
	// Do the needful.
	photos, total, err := models.GetOrphanedCommentPhotos()
	if err != nil {
		return total, err
	}

	if dryrun {
		return total, nil
	}

	for _, row := range photos {

		// CommentPhotos may have been expired/deleted by its owner and the filename is blank.
		if row.Filename != "" {
			if err := photo.Delete(row.Filename); err != nil {
				return total, fmt.Errorf("photo ID %d: removing file %s: %s", row.ID, row.Filename, err)
			}
		}

		if err := row.Delete(); err != nil {
			return total, fmt.Errorf("deleting orphaned comment photo (%d): %s", row.ID, err)
		}
	}

	return total, nil
}

// VacuumOrphanedLikes removes any likes about table objects which no longer exist.
func VacuumOrphanedLikes(dryrun bool) (int64, error) {
	var count int64
	query := models.DB.Raw(`
		SELECT count(*)
		FROM likes
		WHERE (
			table_name = 'comments'
			AND NOT EXISTS (
				SELECT 1
				FROM comments
				WHERE comments.id = likes.table_id
			)
		) OR (
		 	table_name = 'photos'
			AND NOT EXISTS (
				SELECT 1
				FROM photos
				WHERE photos.id = likes.table_id
			)
		)
	`).Scan(&count)
	if query.Error != nil {
		return 0, query.Error
	}

	if dryrun {
		return count, nil
	}

	// Delete them.
	return count, models.DB.Exec(`
		DELETE FROM likes
		WHERE (
			table_name = 'comments'
			AND NOT EXISTS (
				SELECT 1
				FROM comments
				WHERE comments.id = likes.table_id
			)
		) OR (
		 	table_name = 'photos'
			AND NOT EXISTS (
				SELECT 1
				FROM photos
				WHERE photos.id = likes.table_id
			)
		)
	`).Error
}
