// Package geoip provides IP address geolocation features.
package geoip

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/utility"
	"github.com/oschwald/geoip2-golang"
)

// Insights returns structured GeoIP insights useful for the Location tab of the settings page.
type Insights struct {
	CountryCode  string
	CountryName  string
	Subdivisions []string
	City         string
	PostalCode   string
	Latitude     float64
	Longitude    float64
	FlagEmoji    string
}

// IsZero checks if the insights are unpopulated.
func (i Insights) IsZero() bool {
	return i.CountryCode == ""
}

// String pretty prints the insights for front-end display.
func (i Insights) String() string {
	var parts = []string{
		i.CountryName,
		strings.Join(i.Subdivisions, ", "),
		i.City,
	}
	if i.PostalCode != "" {
		parts = append(parts, "Postal Code "+i.PostalCode)
	}
	parts = append(parts, fmt.Sprintf("Lat: %f; Long: %f", i.Latitude, i.Longitude))
	return strings.Join(parts, "; ")
}

// Short prints a short summary string of the insights.
func (i Insights) Short() string {
	var parts = []string{
		i.CountryName,
		strings.Join(i.Subdivisions, ", "),
		i.City,
	}
	return strings.Join(parts, "; ")
}

// Medium prints a summary including country flag emoji with all fields except lat/long.
func (i Insights) Medium() string {
	var parts = []string{
		strings.Join([]string{i.FlagEmoji, i.CountryCode}, " "),
	}
	parts = append(parts, i.Short())
	return strings.Join(parts, "; ")
}

// GetRequestInsights returns structured insights based on the current HTTP request.
func GetRequestInsights(r *http.Request) (Insights, error) {
	var (
		addr = utility.IPAddress(r)
		ip   = net.ParseIP(addr)
	)
	return GetInsights(ip)
}

// GetInsights returns structured insights based on an IP address.
func GetInsights(ip net.IP) (Insights, error) {
	city, err := GetCity(ip)
	if err != nil {
		return Insights{}, err
	}

	// Country flag emoji.
	emoji, err := CountryFlagEmoji(city.Country.IsoCode)
	if err != nil {
		emoji = "🏴‍☠️"
	}

	var result = Insights{
		City:         city.City.Names["en"],
		CountryCode:  city.Country.IsoCode,
		CountryName:  city.Country.Names["en"],
		Subdivisions: []string{},
		PostalCode:   city.Postal.Code,
		Latitude:     city.Location.Latitude,
		Longitude:    city.Location.Longitude,
		FlagEmoji:    emoji,
	}
	for _, sub := range city.Subdivisions {
		if name, ok := sub.Names["en"]; ok {
			result.Subdivisions = append(result.Subdivisions, name)
		}
	}

	return result, nil
}

type InsightsMap map[string]Insights

func (i InsightsMap) Get(key string) Insights {
	return i[key]
}

// MapInsights returns a hash map of IP address (strings) to their Insights.
func MapInsights(addrs []string) InsightsMap {
	var result = map[string]Insights{}
	for _, addr := range addrs {
		if _, ok := result[addr]; ok {
			continue
		}

		ip := net.ParseIP(addr)
		insights, err := GetInsights(ip)
		if err != nil {
			log.Error("MapInsights(%s): %s", addr, err)
		}
		result[addr] = insights
	}
	return result
}

// GetRequestCity returns the GeoIP City result for the current HTTP request.
func GetRequestCity(r *http.Request) (*geoip2.City, error) {
	var (
		addr = utility.IPAddress(r)
		ip   = net.ParseIP(addr)
	)
	return GetCity(ip)
}

// GetRequestCountryFlag returns the country flag based on the current HTTP request IP address.
func GetRequestCountryFlag(r *http.Request) (string, error) {
	city, err := GetRequestCity(r)
	if err != nil {
		// If the remote addr is localhost (local dev testing), default to US flag.
		if addr := utility.IPAddress(r); addr == "127.0.0.1" || addr == "::1" {
			return CountryFlagEmoji("US")
		}

		return "", err
	}

	return CountryFlagEmoji(city.Country.IsoCode)
}

// GetRequestCountryFlagWithCode returns the flag joined with the country code by a space (like CountryFlagEmojiWithCode).
func GetRequestCountryFlagWithCode(r *http.Request) (string, error) {
	city, err := GetRequestCity(r)
	if err != nil {
		// If the remote addr is localhost (local dev testing), default to US flag.
		if addr := utility.IPAddress(r); addr == "127.0.0.1" || addr == "::1" {
			return CountryFlagEmojiWithCode("US")
		}

		return "", err
	}

	return CountryFlagEmojiWithCode(city.Country.IsoCode)
}

// GetChatFlagEmoji returns a specialized country flag emoji string for the BareRTC chat room.
//
// This will include the country flag emoji along with the country and territory/state name.
func GetChatFlagEmoji(r *http.Request) (string, error) {
	city, err := GetRequestCity(r)
	if err != nil {
		// If the remote addr is localhost (local dev testing), default to US flag and only "US" code.
		if addr := utility.IPAddress(r); addr == "127.0.0.1" || addr == "::1" {
			return CountryFlagEmojiWithCode("US")
		}

		return "", err
	}

	// Codes to attach (state, country, etc.)
	emoji, err := CountryFlagEmoji(city.Country.IsoCode)
	if err != nil {
		return emoji, err
	}

	// The components of text location part of the string.
	var flags = []string{}

	// The country. Name or ISO code?
	if name, ok := city.Country.Names["en"]; ok {
		flags = append(flags, name)
	} else {
		flags = append(flags, city.Country.IsoCode)
	}

	// Subdivisions (states)
	if len(city.Subdivisions) > 0 {
		// Stop at just one subdivision. This will be US states
		// and general regions, but without getting too specific
		// for UK users especially where the subdivisions can hone
		// in on their city of 1,000 population!
		sub := city.Subdivisions[0]

		// Can we get its name?
		if name, ok := sub.Names["en"]; ok {
			flags = append(flags, name)
		} else {
			flags = append(flags, sub.IsoCode)
		}
	}

	return emoji + " " + strings.Join(flags, ", "), nil
}

// GetCity queries the GeoIP database for city information for an IP address.
func GetCity(ip net.IP) (*geoip2.City, error) {
	db, err := geoip2.Open(config.GeoIPPath)
	if err != nil {
		return nil, err
	}

	return db.City(ip)
}

// CountryFlagEmoji returns the emoji sequence for a country flag based on
// the two-letter country code.
func CountryFlagEmoji(alpha2 string) (string, error) {
	if len(alpha2) != 2 {
		return "", errors.New("country code must be two letters long")
	}

	alpha2 = strings.ToLower(alpha2)

	var (
		flagBaseIndex = '\U0001F1E6' - 'a'
		box           = func(ch byte) string {
			return string(rune(ch) + flagBaseIndex)
		}
	)

	return string(box(alpha2[0]) + box(alpha2[1])), nil
}

// CountryFlagEmojiWithCode returns a string consisting of the country flag, a space, and the alpha2 code passed in.
func CountryFlagEmojiWithCode(alpha2 string) (string, error) {
	if emoji, err := CountryFlagEmoji(alpha2); err != nil {
		return emoji, err
	} else {
		return emoji + " " + alpha2, nil
	}
}
