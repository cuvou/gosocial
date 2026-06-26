package worker

import (
	"fmt"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/utility"
)

// LaunchGoroutines kicks off background goroutines for worker tasks:
//
// - Watching BareRTC to refresh chat room statistics
// - Evaluating the Emergency Kill Switch
func LaunchGoroutines() {
	go WatchBareRTC()
	go EvaluateKillSwitch()
	go ExpireStatusMessages()
}

// EvaluateKillSwitch checks whether the emergency kill switch will activate.
func EvaluateKillSwitch() {
	log.Error("EvaluateKillSwitch goroutine is engaged")

	var doActivate = func() {
		config.Current.EmergencyKillSwitch.Activated = true
		if err := config.WriteSettings(); err != nil {
			log.Error("Error activating the Emergency Kill Switch: %s", err)
		}
	}

	var doEvaluate = func() {
		// Already activated?
		if config.Current.EmergencyKillSwitch.Activated {
			log.Error("EvaluateKillSwitch: it is already activated")
			return
		}

		// Armed and ready?
		if config.Current.EmergencyKillSwitch.Enabled && config.Current.EmergencyKillSwitch.OwnerUserID > 0 && config.Current.EmergencyKillSwitch.DaysMissingTTL > 0 {
			// Check if the site owner has gone missing.
			if owner, err := models.GetUser(config.Current.EmergencyKillSwitch.OwnerUserID); err == nil {
				if time.Since(owner.LastLoginAt) > time.Duration(config.Current.EmergencyKillSwitch.DaysMissingTTL)*24*time.Hour {
					log.Error("EvaluateKillSwitch: owner has gone missing since %s! Activating the kill switch!", utility.FormatDurationCoarse(time.Since(owner.LastLoginAt)))
					doActivate()
				}
			}
		}
	}

	// Check immediately on server start.
	doEvaluate()

	// And on an interval.
	for {
		time.Sleep(config.KillSwitchCheckInterval)
		doEvaluate()
	}
}

// ExpireStatusMessages will check for status messages that need expiring.
func ExpireStatusMessages() {
	log.Error("ExpireStatusMessages worker engaged!")

	var doEvaluate = func() {
		// Get the current unix time, any status expiration less than this will expire.
		now := fmt.Sprintf("%d", time.Now().Unix())

		// Query for status message expirations.
		var userIDs []uint64
		err := models.DB.Raw(
			`
				SELECT
					user_id
				FROM profile_fields
				WHERE name='headline_expires'
				AND value > '0'
				AND value < ?
				ORDER BY value DESC
			`,
			now,
		).Scan(&userIDs)
		if err.Error != nil {
			log.Error("ExpireStatusMessages: querying for expired statuses: %s", err.Error)
			return
		}

		// Clear out their headlines.
		if len(userIDs) > 0 {
			err := models.DB.Exec(
				`
					DELETE FROM profile_fields
					WHERE user_id IN ?
					AND name IN ?
				`,
				userIDs,
				[]string{"headline", "headline_expires"},
			)
			if err.Error != nil {
				log.Error("ExpireStatusMessages: flushing expired statuses: %s", err.Error)
			}
		}
	}

	// Run it immediately on startup.
	doEvaluate()

	// And on an interval.
	for {
		time.Sleep(config.ExpiredStatusMessageCheckInterval)
		doEvaluate()
	}
}
