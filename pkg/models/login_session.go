package models

import (
	"net"
	"time"

	"github.com/cuvou/gosocial/pkg/geoip"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/utility"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm/clause"
)

// LoginSession table stores browser sessions in the database.
type LoginSession struct {
	ULID             string  `gorm:"column:ulid;primaryKey"`
	UserID           *uint64 `gorm:"index"`
	Data             string
	ClientSecretHash string
	IPAddress        string    // last IP addr (for geolocation)
	UserAgent        string    // browser user-agent (for display)
	CreatedAt        time.Time // TODO: indexes
	UpdatedAt        time.Time
	ExpiresAt        time.Time
}

// GetLoginSession loads a LoginSession by its ULID.
func GetLoginSession(id ulid.ULID) (*LoginSession, error) {
	ls := &LoginSession{}
	res := DB.Model(&LoginSession{}).Where(
		"ulid = ? AND expires_at > ?",
		id.String(),
		time.Now(),
	).First(&ls)
	return ls, res.Error
}

// RevokeLoginSession deletes a session from the database.
func RevokeLoginSession(id ulid.ULID) error {
	return DB.Exec(
		"DELETE FROM login_sessions WHERE ulid = ?",
		id.String(),
	).Error
}

// RevokeAllLoginSessions deletes all of a user's sessions except their current one.
func RevokeAllLoginSessions(currentUser *User, except ulid.ULID) error {
	return DB.Exec(
		`
			DELETE FROM login_sessions
			WHERE user_id = ?
			AND ulid <> ?
		`,
		currentUser.ID,
		except.String(),
	).Error
}

// RevokeAllUserLogins removes all login sessions for a user ID.
//
// Used by the Admin reset password page: log out all of their sessions.
func RevokeAllUserLogins(userID uint64) error {
	return DB.Exec(
		"DELETE FROM login_sessions WHERE user_id = ?",
		userID,
	).Error
}

// CountLoginSessions returns the count of logins for the user.
func CountLoginSessions(currentUser *User) int64 {
	var count int64
	DB.Model(&LoginSession{}).Where(
		"user_id = ?",
		currentUser.ID,
	).Count(&count)
	return count
}

// PaginateLoginSessions lists the login sessions for the user.
func PaginateLoginSessions(currentUser *User, pager *Pagination) ([]*LoginSession, error) {
	var ls = []*LoginSession{}

	query := DB.Model(&LoginSession{}).Where(
		"user_id = ?",
		currentUser.ID,
	).Order(pager.Sort)

	query.Count(&pager.Total)

	result := query.Offset(
		pager.GetOffset(),
	).Limit(pager.PerPage).Find(&ls)
	return ls, result.Error
}

// GetParsedUserAgent gets structured insights from the User-Agent string.
func (ls *LoginSession) GetParsedUserAgent() utility.UserAgent {
	return utility.ParseUserAgent(ls.UserAgent)
}

// GetGeoIP gets GeoIP insights for the login session.
func (ls *LoginSession) GetGeoIP() geoip.Insights {
	i, err := geoip.GetInsights(net.ParseIP(ls.IPAddress))
	if err != nil {
		log.Error("LoginSession.GetGeoIP: %s", err)
	}
	return i
}

// SaveLoginSession writes a login session to SQL.
func (ls *LoginSession) Save() error {
	res := DB.Model(&LoginSession{}).Clauses(
		clause.OnConflict{
			Columns: []clause.Column{
				{Name: "ulid"},
			},
			UpdateAll: true,
		},
	).Create(ls)
	return res.Error
}
