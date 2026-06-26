package backfill

import (
	"os"

	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/photo"
)

// BackfillFilesizes finds Photos and CommentPhotos which have a zero Filesize and recomputes them.
func BackfillFilesizes() error {
	// Find Photos with a filesize of zero.
	var (
		photos        []*models.Photo
		commentPhotos []*models.CommentPhoto
	)

	// Query for photos w/ zero filesize.
	result := models.DB.Model(&models.Photo{}).Where("filesize = 0").Find(&photos)
	if result.Error != nil {
		return result.Error
	}

	// And CommentPhotos too.
	result = models.DB.Model(&models.CommentPhoto{}).Where("filesize = 0").Find(&commentPhotos)
	if result.Error != nil {
		return result.Error
	}

	// Do the Photos.
	for i, row := range photos {
		// Set the filesize from disk.
		if stat, err := os.Stat(photo.DiskPath(row.Filename)); err == nil {
			row.Filesize = stat.Size()
		}

		log.Info("[%d of %d] Update photo %d: %s filesize=%d",
			i+1, len(photos), row.ID, row.Filename, row.Filesize,
		)

		if err := row.Save(); err != nil {
			return err
		}
	}

	// And same for the CommentPhotos.
	for i, row := range commentPhotos {
		// Set the filesize from disk.
		if stat, err := os.Stat(photo.DiskPath(row.Filename)); err == nil {
			row.Filesize = stat.Size()
		}

		log.Info("[%d of %d] Update comment_photo %d: %s filesize=%d",
			i+1, len(photos), row.ID, row.Filename, row.Filesize,
		)

		if err := row.Save(); err != nil {
			return err
		}
	}

	return nil
}
