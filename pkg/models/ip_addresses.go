package models

import (
	"fmt"
	"net/http"
	"time"

	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/utility"
)

// IPAddress table to log which networks users have logged in from.
type IPAddress struct {
	ID           uint64    `gorm:"primaryKey"`
	UserID       uint64    `gorm:"index"`
	IPAddress    string    `gorm:"index"`
	NumberVisits uint64    // count of times their LastLoginAt pinged from this address
	CreatedAt    time.Time // first time seen
	UpdatedAt    time.Time // last time seen
}

// PingIPAddress logs or upserts the user's current IP address into the IPAddress table.
func PingIPAddress(r *http.Request, user *User, incrementVisit bool) error {
	var (
		addr = utility.IPAddress(r)
		ip   *IPAddress
	)

	// Have we seen it before?
	ip, err := LoadUserIPAddress(user, addr)
	if err != nil {
		// Insert it.
		log.Debug("User %s IP %s seen for the first time", user.Username, addr)
		ip = &IPAddress{
			UserID:    user.ID,
			IPAddress: addr,
			CreatedAt: time.Now(),
		}

		result := DB.Create(ip)
		if result.Error != nil {
			return result.Error
		}
	}

	// Are we refreshing the NumberVisits count? Note: this happens each
	// time the main website will refresh the user LastLoginAt.
	if incrementVisit || ip.NumberVisits == 0 {
		ip.NumberVisits++
	}

	// Ping the update.
	ip.UpdatedAt = time.Now()
	return ip.Save()
}

func LoadUserIPAddress(user *User, ipAddr string) (*IPAddress, error) {
	var ip = &IPAddress{}
	var result = DB.Model(&IPAddress{}).Where(
		"user_id = ? AND ip_address = ?",
		user.ID, ipAddr,
	).First(&ip)
	return ip, result.Error
}

// PaginateIPsByUser gets IPs from the user.
func PaginateIPsByUser(user *User, pager *Pagination) ([]*IPAddress, error) {
	var (
		ip    = []*IPAddress{}
		query = DB.Model(&IPAddress{}).Where(
			"user_id = ?",
			user.ID,
		)
	)

	query.Count(&pager.Total)
	res := query.Order(pager.Sort).Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&ip)

	return ip, res.Error
}

// PaginateUsersByIP searches by IP address and returns the matching users.
func PaginateUsersByIP(ip string, pager *Pagination) ([]*User, error) {
	var (
		users = []*User{}
		query = DB.Model(&User{}).Preload("ProfilePhoto").Joins(
			"JOIN ip_addresses ON (ip_addresses.user_id = users.id)",
		).Where(
			"ip_addresses.ip_address = ?",
			ip,
		)
	)

	query.Count(&pager.Total)
	res := query.Order(pager.Sort).Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&users)
	return users, res.Error
}

// PaginateBannedIPs searches IP addresses for links to banned user accounts.
func PaginateBannedIPs(pager *Pagination) ([]*IPAddress, error) {
	var (
		ip    = []*IPAddress{}
		query = DB.Model(&IPAddress{}).Joins(
			"JOIN users ON (users.id = ip_addresses.user_id)",
		).Where(
			"users.status = ?",
			UserStatusBanned,
		)
	)

	query.Count(&pager.Total)
	res := query.Order(pager.Sort).Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&ip)

	return ip, res.Error
}

// SharedIPInsights maps an IP address to the count of users who share it.
type SharedIPInsights struct {
	IPAddress string
	UserCount int
}

// PaginateSharedIPs searches IP addresses shared addresses linked to multiple users.
func PaginateSharedIPs(pager *Pagination) ([]*SharedIPInsights, error) {
	var (
		ip  = []*SharedIPInsights{}
		res = DB.Raw(fmt.Sprintf(`
			WITH subquery AS (
				SELECT
					ip_address,
					count(*) AS user_count
				FROM ip_addresses
				GROUP BY ip_address
			)

			SELECT ip_address, user_count
			FROM subquery
			WHERE user_count > 1
			ORDER BY user_count DESC
			OFFSET %d LIMIT %d
		`, pager.GetOffset(), pager.PerPage)).Scan(&ip)
	)

	// Count query for the pager.
	DB.Raw(`
		WITH subquery AS (
			SELECT
				ip_address,
				count(*) AS user_count
			FROM ip_addresses
			GROUP BY ip_address
		)

		SELECT count(*)
		FROM subquery
		WHERE user_count > 1
	`).Scan(&pager.Total)

	return ip, res.Error
}

// BannedIPMap maps IP addresses to their status of being linked with banned accounts.
type BannedIPMap map[string]bool

// Get the value of a banned IP.
func (bm BannedIPMap) Get(ip string) bool {
	return bm[ip]
}

// MapBannedIPs maps a set of IP addresses to their status with banned accounts.
func MapBannedIPs(ips []string) BannedIPMap {
	if len(ips) == 0 {
		return BannedIPMap{}
	}

	var (
		result = BannedIPMap{}
		ip     = []*IPAddress{}
		res    = DB.Model(&IPAddress{}).Joins(
			"JOIN users ON (users.id = ip_addresses.user_id)",
		).Where(
			"users.status = ? AND ip_addresses.ip_address IN ?",
			UserStatusBanned,
			ips,
		).Find(&ip)
	)

	if res.Error != nil {
		log.Error("MapBannedIPs: %s", res.Error)
		return result
	}

	for _, addr := range ip {
		result[addr.IPAddress] = true
	}

	return result
}

// Save photo.
func (ip *IPAddress) Save() error {
	result := DB.Save(ip)
	return result.Error
}

// Delete the DB entry.
func (ip *IPAddress) Delete() error {
	return DB.Delete(ip).Error
}
