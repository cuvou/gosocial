package models

import (
	"time"

	"github.com/cuvou/gosocial/pkg/config"
)

// CommentPhoto table associates a photo attachment to a (forum) comment.
type CommentPhoto struct {
	ID        uint64 `gorm:"primaryKey"`
	UserID    uint64 `gorm:"index"`
	CommentID uint64 `gorm:"index"`
	Filename  string
	Filesize  int64
	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiredAt time.Time
}

// CreateCommentPhoto with most of the settings you want (not ID or timestamps) in the database.
func CreateCommentPhoto(tmpl CommentPhoto) (*CommentPhoto, error) {
	p := &CommentPhoto{
		CommentID: tmpl.CommentID,
		UserID:    tmpl.UserID,
		Filename:  tmpl.Filename,
		Filesize:  tmpl.Filesize,
	}

	result := DB.Create(p)
	return p, result.Error
}

// GetCommentPhoto by ID.
func GetCommentPhoto(id uint64) (*CommentPhoto, error) {
	p := &CommentPhoto{}
	result := DB.First(&p, id)
	return p, result.Error
}

// GetCommentPhotos by an array of IDs, mapped to their IDs.
func GetCommentPhotos(IDs []uint64) (map[uint64]*CommentPhoto, error) {
	var (
		mp = map[uint64]*CommentPhoto{}
		ps = []*CommentPhoto{}
	)

	result := DB.Model(&CommentPhoto{}).Where("id IN ?", IDs).Find(&ps)
	for _, row := range ps {
		mp[row.ID] = row
	}

	return mp, result.Error
}

// CanBeEditedBy checks whether a comment photo can be edited by the current user.
//
// Admins with PhotoModerator scope can always edit.
func (p *CommentPhoto) CanBeEditedBy(currentUser *User) bool {
	if currentUser.HasAdminScope(config.ScopePhotoModerator) {
		return true
	}

	return p.UserID == currentUser.ID
}

// GetPhotos returns the comment photos for a given comment.
func (c *Comment) GetPhotos() ([]*CommentPhoto, error) {
	mapping, err := MapCommentPhotos([]*Comment{c})
	if err != nil {
		return nil, err
	}

	return mapping.Get(c.ID), nil
}

/*
PaginateUserCommentPhotos gets a page of all CommentPhotos by the user.
*/
func PaginateUserCommentPhotos(userID uint64, pager *Pagination) ([]*CommentPhoto, error) {
	var p = []*CommentPhoto{}

	query := DB.Where(
		"user_id = ? AND filename <> ?",
		userID, "",
	).Order(
		pager.Sort,
	)

	// Get the total count.
	query.Model(&CommentPhoto{}).Count(&pager.Total)

	result := query.Offset(
		pager.GetOffset(),
	).Limit(pager.PerPage).Find(&p)

	return p, result.Error
}

// CommentPhotoMap maps comment IDs to CommentPhotos.
type CommentPhotoMap map[uint64][]*CommentPhoto

// Get like stats from the map.
func (lm CommentPhotoMap) Get(id uint64) []*CommentPhoto {
	if stats, ok := lm[id]; ok {
		return stats
	}
	return nil
}

// MapCommentPhotos returns a map of photo attachments to a series of comments.
func MapCommentPhotos(comments []*Comment) (CommentPhotoMap, error) {
	var (
		result = CommentPhotoMap{} // map[uint64][]*CommentPhoto{}
		ps     = []*CommentPhoto{}
		IDs    = []uint64{}
	)

	for _, c := range comments {
		if c == nil || c.ID == 0 {
			continue
		}
		IDs = append(IDs, c.ID)
	}

	if len(IDs) == 0 {
		return result, nil
	}

	res := DB.Model(&CommentPhoto{}).Where("comment_id IN ?", IDs).Find(&ps)
	if res.Error != nil {
		return nil, res.Error
	}

	for _, row := range ps {
		if _, ok := result[row.CommentID]; !ok {
			result[row.CommentID] = []*CommentPhoto{}
		}
		result[row.CommentID] = append(result[row.CommentID], row)
	}

	return result, nil
}

// Save CommentPhoto.
func (p *CommentPhoto) Save() error {
	result := DB.Save(p)
	return result.Error
}

// Delete CommentPhoto.
func (p *CommentPhoto) Delete() error {
	result := DB.Delete(p)
	return result.Error
}

// GetOrphanedCommentPhotos gets all (up to 500) photos having a blank CommentID older than 24 hours.
func GetOrphanedCommentPhotos() ([]*CommentPhoto, int64, error) {
	var (
		count  int64
		cutoff = time.Now().Add(-24 * time.Hour)
		ps     = []*CommentPhoto{}
	)

	query := DB.Model(&CommentPhoto{}).Where(`
		(comment_id <> 0 AND NOT EXISTS (
			SELECT 1 FROM comments
			WHERE comments.id = comment_photos.comment_id
		))
		OR
		(comment_id = 0 AND created_at < ?)`,
		cutoff,
	)
	query.Count(&count)
	res := query.Limit(500).Find(&ps)
	if res.Error != nil {
		return nil, 0, res.Error
	}

	return ps, count, res.Error
}
