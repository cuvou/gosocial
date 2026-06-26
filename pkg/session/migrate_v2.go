package session

import (
	"fmt"
	"net/http"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/redis"
	"github.com/oklog/ulid/v2"
)

// MigrateV2 migrates a legacy session from Redis to Postgres.
func MigrateV2(w http.ResponseWriter, r *http.Request, sess *Session) error {
	oldUUID := sess.UUID

	sess.ULID = ulid.Make()
	sess.UUID = ""
	sess.ClientSecret = ""

	// Delete the old Redis key.
	key := fmt.Sprintf(config.SessionRedisKeyFormat, oldUUID)
	if err := redis.Delete(key); err != nil {
		log.Error("Session.MigrateV2: error deleting old key %s: %s", key, err)
	}

	// Save the new ULID session cookies now.
	sess.Save(w, r)

	return nil
}
