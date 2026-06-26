package backfill

import (
	"fmt"

	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
)

// BackfillPrivacySettings migrates old ProfileField-based privacy settings into the new table.
//
// This moves the old settings for:
//
// - Who can slide into your DMs
// - Who can comment on your photos
// - Who can unlock their private photos for you
func BackfillPrivacySettings() error {
	var (
		profileFieldNames = []string{
			"dm_privacy",
			"photo_comment_permission",
			"private_photo_gate",
		}
		privacySettingColumns = map[string]string{
			// Legacy PF name: privacy_settings column
			"dm_privacy":               "first_messages",
			"photo_comment_permission": "photo_comments",
			"private_photo_gate":       "private_photos",
		}
		perPage = 100
		offset  = 0
	)

	log.Warn("BEGIN Backfill Privacy Settings")
	for {
		var pf = []*models.ProfileField{}

		// Get the next page of pending profile fields to migrate over.
		res := models.DB.Model(&models.ProfileField{}).Where(
			"name IN ? AND value <> ?",
			profileFieldNames, "",
		).Order("user_id ASC, name ASC").Offset(offset).Limit(perPage).Scan(&pf)
		if res.Error != nil {
			return fmt.Errorf("paginating profile fields: %w", res.Error)
		}

		if len(pf) == 0 {
			break
		}

		for _, row := range pf {
			log.Info("Migrate uid=%d %s=%s", row.UserID, row.Name, row.Value)

			column := privacySettingColumns[row.Name]
			if column == "" {
				return fmt.Errorf("couldn't map privacy_settings column: %s", row.Name)
			}

			// Upsert the privacy setting.
			res := models.DB.Exec(
				fmt.Sprintf(`
					INSERT INTO privacy_settings (user_id, %s)
					VALUES (?, ?)
					ON CONFLICT (user_id) DO UPDATE
					SET %s=?
				`, column, column),
				row.UserID, row.Value,
				row.Value,
			)
			if res.Error != nil {
				return fmt.Errorf("upserting privacy_settings: %s", res.Error)
			}
		}

		offset += perPage
	}

	return nil
}
