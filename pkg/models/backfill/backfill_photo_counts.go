package backfill

import (
	"github.com/cuvou/gosocial/pkg/models"
)

// BackfillPhotoCounts recomputes the cached Likes and Comment counts on photos.
func BackfillPhotoCounts() error {
	res := models.DB.Exec(`
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
		);
	`)
	return res.Error
}
