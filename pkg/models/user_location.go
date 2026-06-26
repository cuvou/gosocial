package models

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/geoip"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/utility"
)

// UserLocation table holds a user's location preference and coordinates.
type UserLocation struct {
	UserID      uint64 `gorm:"primaryKey"`
	Source      string
	Latitude    float64 `gorm:"index"`
	Longitude   float64 `gorm:"index"`
	EmojiString string  // cached PrettyEmojiString when their lat/lon changes.
}

// Source options for UserLocation.
const (
	LocationSourceNone  = ""
	LocationSourceGeoIP = "geoip"
	LocationSourceGPS   = "gps"
	LocationSourcePin   = "pin"
)

// GetUserLocation gets the UserLocation object for a user ID, or a new object.
func GetUserLocation(userId uint64) *UserLocation {
	var ul = &UserLocation{}
	result := DB.First(&ul, userId)
	if result.Error != nil {
		return &UserLocation{
			UserID: userId,
		}
	}
	return ul
}

// IsEmpty checks if the location data is missing.
func (ul *UserLocation) IsEmpty() bool {
	return ul.Source == LocationSourceNone || (ul.Latitude == 0 && ul.Longitude == 0)
}

// GetUserLocationPrettyEmojiString will return a PrettyEmojiString for the user's manually set location.
//
// If the user does not have a UserLocation configured or anything else goes wrong,
// this will return an empty string and eat any error messages.
//
// This is a convenience function to just quickly get their pretty location, and is a
// shortcut for doing the following steps:
//
//	location := GetUserLocation(userId)
//	pretty, _ := location.PrettyEmojiString()
func GetUserLocationPrettyEmojiString(userId uint64) string {
	var (
		location    = GetUserLocation(userId)
		pretty, err = location.PrettyEmojiString()
	)

	// Avoid the "Null Island" bug: if the user location is 0,0 consider it to be invalid.
	if err != nil || (location.Latitude == 0 && location.Longitude == 0) {
		return ""
	}
	return pretty
}

// PrettyEmojiString will match the user's location back to the WorldCities database
// and return its PrettyEmojiString.
//
// If the user's location setting is disabled, returns an empty string and error.
func (ul *UserLocation) PrettyEmojiString() (string, error) {
	if ul.Source == LocationSourceNone {
		return "", errors.New("location is disabled for this user")
	}

	// Do we have a cached string on their DB model?
	if ul.EmojiString != "" {
		return ul.EmojiString, nil
	}

	// Compute and then cache their emoji string.
	if city, err := NearestWorldCity(ul.Latitude, ul.Longitude); err == nil {
		result, err := city.PrettyEmojiString()
		if err != nil {
			return result, err
		}

		// Cache the result on their DB model.
		// Note: the cache is updated on Save() in case they change their lat/lon later.
		ul.EmojiString = result
		if err := ul.Save(); err != nil {
			log.Error("UserLocation.PrettyEmojiString: failed to save cached string to DB: %s", err)
		}

		return ul.EmojiString, nil
	} else {
		return "", err
	}
}

// Save the UserLocation.
func (ul *UserLocation) Save() error {
	if ul.Source == LocationSourceNone {
		ul.Latitude = 0
		ul.Longitude = 0
		ul.EmojiString = ""
	} else {
		// Pre-compute their emoji string and cache it.
		if city, err := NearestWorldCity(ul.Latitude, ul.Longitude); err == nil {
			if emoji, err := city.PrettyEmojiString(); err == nil {
				ul.EmojiString = emoji
			}
		}
	}
	return DB.Save(ul).Error
}

// RefreshGeoIP will auto-update a user's location by GeoIP if that's their setting.
func RefreshGeoIP(userID uint64, r *http.Request) (*UserLocation, error) {
	loc := GetUserLocation(userID)
	if loc.Source == LocationSourceGeoIP {
		if insights, err := geoip.GetRequestInsights(r); err == nil {
			loc.Latitude = truncate(insights.Latitude)
			loc.Longitude = truncate(insights.Longitude)
			return loc, loc.Save()
		} else {
			return loc, fmt.Errorf("didn't get insights: %s", err)
		}
	}
	return loc, nil
}

func truncate(f float64) float64 {
	s := strconv.FormatFloat(f, 'f', 2, 64)
	f, _ = strconv.ParseFloat(s, 64)
	return f
}

// MapDistances computes human readable distances between you and the set of users.
//
// The fromCity attribute is optional. If non-nil, the distance will be computed in
// relation to that city's location instead of the current user.
func MapDistances(currentUser *User, fromCity *WorldCities, others []*User) DistanceMap {
	// Get all the distances we can.
	var (
		result              = DistanceMap{}
		myDist              = GetUserLocation(currentUser.ID)
		latitude, longitude = myDist.Latitude, myDist.Longitude
		// dist    = map[uint64]*UserLocation{}
		userIDs = []uint64{}
	)
	for _, user := range others {
		userIDs = append(userIDs, user.ID)
	}

	// Are we ordering from a different city?
	if fromCity != nil {
		latitude = fromCity.Latitude
		longitude = fromCity.Longitude
	}

	// Query for their UserLocation objects, if exists.
	var ul = []*UserLocation{}
	res := DB.Where("user_id IN ?", userIDs).Find(&ul)
	if res.Error != nil {
		log.Error("MapDistances: %s", res.Error)
		return result
	}

	// Map them out.
	for _, row := range ul {
		result[row.UserID] = utility.HaversineDistanceString(
			latitude, longitude,
			row.Latitude, row.Longitude,
		)
	}

	return result
}

// DistanceMap maps user IDs to distance strings.
type DistanceMap map[uint64]string

// Get a value from the DistanceMap for easy front-end access.
func (dm DistanceMap) Get(key uint64) string {
	if value, ok := dm[key]; ok {
		return value
	}
	return "unknown distance"
}
