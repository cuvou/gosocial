package backfill

import (
	"fmt"

	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
)

// BackfillProfileThemes migrates old ProfileField-based theme settings into the new table.
func BackfillProfileThemes() error {
	var (
		profileFieldNames = []string{
			"hero-color-start",
			"hero-color-end",
			"hero-text-dark",
			"card-title-bg",
			"card-title-fg",
			"card-link-color",
			"card-lightness",
		}
		profileThemeColumns = map[string]string{
			// Legacy PF name: profile_themes column
			"hero-color-start": "hero_color_start",
			"hero-color-end":   "hero_color_end",
			"hero-text-dark":   "hero_text_dark",
			"card-title-bg":    "card_title_bg",
			"card-title-fg":    "card_title_fg",
			"card-link-color":  "card_link_color",
			"card-lightness":   "card_lightness",
		}
		perPage = 100
		offset  = 0
	)

	log.Warn("BEGIN Backfill Profile Themes")
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

			column := profileThemeColumns[row.Name]
			if column == "" {
				return fmt.Errorf("couldn't map profile_themes column: %s", row.Name)
			}

			// Upsert the privacy setting.
			res := models.DB.Exec(
				fmt.Sprintf(`
					INSERT INTO profile_themes (user_id, %s)
					VALUES (?, ?)
					ON CONFLICT (user_id) DO UPDATE
					SET %s=?
				`, column, column),
				row.UserID, row.Value,
				row.Value,
			)
			if res.Error != nil {
				return fmt.Errorf("upserting profile_themes: %s", res.Error)
			}
		}

		offset += perPage
	}

	return nil
}
