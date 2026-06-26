package deletion

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
)

// DeleteManyUsers can delete one or more users.
//
// It is driven by the `./gosocial user delete` subcommand.
func DeleteManyUsers(c DeleteManyUsersConfig) error {

	log.Warn("BEGIN DeleteManyUsers with config: %+v", c)

	// A single user?
	if c.Username != "" {
		if user, err := models.FindUsernameOrEmail(c.Username); err != nil {
			return fmt.Errorf("username '%s': %s", c.Username, err)
		} else {
			if c.DryRun {
				log.Error("Dry Run: would have deleted user %s", c.Username)
				return nil
			}
			return DeleteUser(user)
		}
	}

	// Many users? Do some validation.
	if !c.Many {
		return errors.New("no username provided and neither was the Many flag opted in")
	}

	// Ensure a filter is set.
	if c.CreatedAfter == nil && c.LastLoginBefore == nil && c.Status == "" {
		return errors.New("filters are required when providing --many users to delete")
	}

	// Iterate over the users.
	var (
		wheres       = []string{}
		placeholders = []any{}
		page         = 1
	)
	if c.Status != "" {
		wheres = append(wheres, "status = ?")
		placeholders = append(placeholders, c.Status)
	}
	if c.CreatedAfter != nil {
		wheres = append(wheres, "created_at >= ?")
		placeholders = append(placeholders, *c.CreatedAfter)
	}
	if c.LastLoginBefore != nil {
		wheres = append(wheres, "last_login_at <= ?")
		placeholders = append(placeholders, *c.LastLoginBefore)
	}

	// Safety check: prompt the user before we begin, unless --force-dangerously.
	if !c.Force {
		var b, _ = json.Marshal(placeholders)
		log.Warn("WARNING: We are about to delete potentially MANY user accounts!")
		log.Info("The user filters are: SELECT * FROM users WHERE %s", strings.Join(wheres, " AND "))
		log.Info("With parameters: %s", b)
		log.Warn("ARE YOU SURE? Type 'yes' to continue.")

		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		if strings.TrimSpace(text) != "yes" {
			log.Error("You did not answer 'yes' so we are quitting here.")
			return nil
		}
	}

	// Paginate over them.
	for {
		log.Warn("DeleteManyUsers: processing page %d", page)
		var (
			users []*models.User
			res   = models.DB.Model(&models.User{}).Where(
				strings.Join(wheres, " AND "),
				placeholders...,
			).Order("id asc").Limit(100).Find(&users)
		)
		if res.Error != nil {
			log.Error("DeleteManyUsers(page %d): %s", page, res.Error)
			return res.Error
		}

		if len(users) == 0 {
			break
		}

		for _, user := range users {
			if c.DryRun {
				log.Error("Dry Run: would have deleted username %s", user.Username)
				continue
			}
			if err := DeleteUser(user); err != nil {
				return err
			}
		}

		page++

		if c.DryRun {
			log.Error("Dry Run: breaking after page 1 since no users were deleted")
			break
		}
	}

	return nil
}

type DeleteManyUsersConfig struct {
	DryRun          bool
	Force           bool
	Username        string
	Many            bool
	Status          string
	CreatedAfter    *time.Time
	LastLoginBefore *time.Time
}
