package models

import (
	"errors"
	"time"

	"github.com/cuvou/gosocial/pkg/log"
	"gorm.io/gorm"
)

// PollVote table records answers to polls.
type PollVote struct {
	ID        uint64 `gorm:"primaryKey"`
	PollID    uint64 `gorm:"index"`
	Poll      Poll
	UserID    uint64 `gorm:"index"`
	Answer    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Preload related tables for the poll (classmethod).
func (u *PollVote) Preload() *gorm.DB {
	return DB.Preload("Poll")
}

// CastVote on a poll. Multiple answers OK for multiple choice polls.
func (p *Poll) CastVote(user *User, answers []string) error {
	if len(answers) > 1 && !p.MultipleChoice {
		return errors.New("multiple answers not accepted for this poll")
	}

	// If this user has already voted, remove their vote.
	result := DB.Where(
		"poll_id = ? AND user_id = ?",
		p.ID, user.ID,
	).Delete(&PollVote{})
	if result.Error != nil {
		return result.Error
	}

	// Insert their votes.
	var err error
	for _, answer := range answers {
		vote := &PollVote{
			PollID: p.ID,
			UserID: user.ID,
			Answer: answer,
		}
		err = vote.Save()
	}

	return err
}

// GetAllVotes for a poll.
func (p *Poll) GetAllVotes() []*PollVote {
	var pv = []*PollVote{}

	result := DB.Where(
		"poll_id = ?", p.ID,
	).Find(&pv)
	if result.Error != nil {
		log.Error("Poll(%d).GetAllVotes(): %s", p.ID, result.Error)
	}

	return pv
}

// Save Poll.
func (v *PollVote) Save() error {
	result := DB.Save(v)
	return result.Error
}
