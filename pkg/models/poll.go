package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
)

// Poll table for user surveys posted in the forums.
type Poll struct {
	ID uint64 `gorm:"primaryKey"`

	// Poll options
	Choices        string // line-separated choices
	MultipleChoice bool   // User can vote multiple choices
	CustomAnswers  bool   // Users can contribute a custom response

	Expires   bool      // if it stops accepting new votes
	ExpiresAt time.Time // when it stops accepting new votes

	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreatePoll initializes a poll.
//
// expires is in days (0 = doesn't expire)
func CreatePoll(choices []string, expires int) *Poll {
	return &Poll{
		Choices:   strings.Join(choices, "\n"),
		Expires:   expires > 0,
		ExpiresAt: time.Now().Add(time.Duration(expires) * 24 * time.Hour),
	}
}

// GetPoll by ID.
func GetPoll(id uint64) (*Poll, error) {
	m := &Poll{}
	result := DB.First(&m, id)
	return m, result.Error
}

// Options returns a conveniently formatted listing of the options.
func (p *Poll) Options() []string {
	return strings.Split(p.Choices, "\n")
}

// IsExpired returns if the poll has ended.
func (p *Poll) IsExpired() bool {
	return p.Expires && time.Now().After(p.ExpiresAt)
}

// InputType returns "radio" or "checkbox" for multiple choice polls.
func (p *Poll) InputType() string {
	if p.MultipleChoice {
		return "checkbox"
	}
	return "radio"
}

// Result returns metadata about a poll's status and results, for frontend assist.
func (p *Poll) Result(currentUser *User) PollResult {
	var (
		result = PollResult{
			AcceptingVotes:  true,
			CurrentUserVote: []string{},
			Results:         map[string]int{},
			ResultsPercent:  map[string]float64{},
			ResultsClass:    map[string]string{},
		}
		votes           = p.GetAllVotes()
		distinctAnswers int
	)

	// Populate the CSS classes.
	for i, answer := range p.Options() {
		result.ResultsClass[answer] = config.PollProgressBarClasses[i%len(config.PollProgressBarClasses)]
	}

	result.TotalVotes = len(votes)
	for _, res := range votes {
		if res.UserID == currentUser.ID {
			result.CurrentUserVote = append(result.CurrentUserVote, res.Answer)
			result.AcceptingVotes = false
		}

		if _, ok := result.Results[res.Answer]; !ok {
			distinctAnswers++
			result.Results[res.Answer] = 0
			result.ResultsPercent[res.Answer] = 0
		}
		result.Results[res.Answer]++
	}

	// Compute the percent splits.
	if result.TotalVotes > 0 {
		for answer, count := range result.Results {
			result.ResultsPercent[answer] = float64(count) / float64(result.TotalVotes)
		}
	}

	// Expired polls don't accept answers.
	if p.IsExpired() {
		result.AcceptingVotes = false
	}

	return result
}

// Save Poll.
func (p *Poll) Save() error {
	result := DB.Save(p)
	return result.Error
}

// Delete Poll, which also deletes its PollVotes.
func (p *Poll) Delete() error {

	// Delete votes first.
	if result := DB.Exec(
		"DELETE FROM poll_votes WHERE poll_id = ?",
		p.ID,
	); result.Error != nil {
		return fmt.Errorf("deleting votes: %s", result.Error)
	}

	result := DB.Delete(p)
	return result.Error
}

// PollResult holds metadata about the poll result for frontend display.
type PollResult struct {
	AcceptingVotes  bool           // user voted or it expired
	CurrentUserVote []string       // current user's selection, if any
	Results         map[string]int // answers and their %
	ResultsPercent  map[string]float64
	ResultsClass    map[string]string // progress bar classes
	TotalVotes      int
}

func (pr PollResult) GetPercent(answer string) string {
	value := pr.ResultsPercent[answer]
	return fmt.Sprintf("%.1f", value*100)
}

func (pr PollResult) GetClass(answer string) string {
	return pr.ResultsClass[answer]
}

func (pr PollResult) GetCount(answer string) int {
	return pr.Results[answer]
}

// GetOrphanedPolls gets all (up to 500) polls that don't have Threads pointing to them.
func GetOrphanedPolls() ([]*Poll, int64, error) {
	var (
		count int64
		ps    = []*Poll{}
	)

	query := DB.Model(&Poll{}).Where(`
		NOT EXISTS (
			SELECT 1 FROM threads
			WHERE threads.poll_id = polls.id
		)
		AND NOT EXISTS (
			SELECT 1 FROM blogs
			WHERE blogs.poll_id = polls.id
		)
	`)
	query.Count(&count)
	res := query.Limit(500).Find(&ps)
	if res.Error != nil {
		return nil, 0, res.Error
	}

	return ps, count, res.Error
}
