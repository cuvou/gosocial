package photo

import (
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
)

type lightboxPhoto struct {
	ID       uint64
	Username string
	Filename string
	AltText  string
}

// PhotosToJSONForLightbox parses a page of gallery photos and creates front-end json serializable data.
//
// The UserMap is optional, e.g. direct user galleries won't have one but the Site Gallery will.
//
// Only still images are included in the result, as the lightbox modal doesn't support videos yet.
func PhotosToJSONForLightbox(photos []*models.Photo, currentUser *models.User, um models.UserMap) []*lightboxPhoto {
	var result = []*lightboxPhoto{}

	// Generate a user map?
	if um == nil {
		var userIDs []uint64
		for _, p := range photos {
			userIDs = append(userIDs, p.UserID)
		}

		if m, err := models.MapUsers(currentUser, userIDs); err != nil {
			log.Error("PhotosToJSONForLightbox: couldn't MapUsers: %s", err)
			um = models.UserMap{}
		} else {
			um = m
		}
	}

	for _, p := range photos {
		var username string
		if user := um.Get(p.UserID); user != nil {
			username = user.Username
		}

		result = append(result, &lightboxPhoto{
			ID:       p.ID,
			Username: username,
			Filename: SignedPhotoURL(currentUser, p.Filename),
			AltText:  p.AltText,
		})
	}

	return result
}
