package backfill

import (
	"fmt"

	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
)

// BackfillFollowers creates Follow relationships for existing friendships.
func BackfillFollowers(offset int) error {
	var limit = 500

	for {
		// Get the next page of friends.
		var (
			friends []*models.Friend
			res     = models.DB.Model(&models.Friend{}).Where(
				`
					friends.approved IS TRUE
					AND friends.ignored IS NOT TRUE
				`,
			).Order(
				"created_at asc",
			).Offset(offset).Limit(limit).Find(&friends)
		)
		if res.Error != nil {
			return fmt.Errorf("Friends query offset %d: %s", offset, res.Error)
		}

		// Ran out of friends?
		if len(friends) == 0 {
			break
		}

		log.Info("Offset %d: adding follows for %d friendships", offset, len(friends))

		// Upsert follows.
		for _, row := range friends {
			if _, err := models.AddFollow(row.SourceUserID, row.TargetUserID); err != nil {
				return fmt.Errorf("following %d and %d: %s", row.SourceUserID, row.TargetUserID, err)
			}

			if _, err := models.AddFollow(row.TargetUserID, row.SourceUserID); err != nil {
				return fmt.Errorf("following %d and %d: %s", row.TargetUserID, row.SourceUserID, err)
			}
		}

		// Get the next page.
		offset += limit
	}

	return nil
}

// BackfillUnfollows migrates the legacy "mute specific friends' new photo notifications" to Unfollow them instead.
func BackfillUnfollows(offset int) error {
	var limit = 500

	for {

		// Paginate thru the "friend.photos" (un)subscriptions and have the owners unfollow
		// those people instead.
		var (
			subs []*models.Subscription
			res  = models.DB.Model(&models.Subscription{}).Where(
				`
					table_name = 'friend.photos'
					AND subscribed IS NOT TRUE
				`,
			).Order(
				"created_at asc",
			).Offset(offset).Limit(limit).Find(&subs)
		)
		if res.Error != nil {
			return fmt.Errorf("Subscriptions query offset %d: %s", offset, res.Error)
		}

		// Ran out of subs?
		if len(subs) == 0 {
			break
		}

		log.Info("Offset %d: unfollowing friends for %d muted friend.photos subscriptions", offset, len(subs))

		// Upsert unfollows.
		for _, row := range subs {
			if err := models.Unfollow(row.UserID, row.TableID); err != nil {
				log.Error("Error unfollowing %d->%d: %s", row.UserID, row.TableID, err)
			}
		}

		// Get the next page.
		offset += limit
	}

	return nil
}
