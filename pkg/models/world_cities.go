package models

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/geoip"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/utility"
)

// WorldCities is a lookup database of cities/countries to their geo coordinates.
//
// It aids the search by location feature of the member directory. Data obtained from simplemaps.com
type WorldCities struct {
	ID             uint64 `gorm:"primary_key"`
	City           string
	State          string
	Country        string
	ISO            string
	Canonical      string  `gorm:"index"` // the full City, State, ISO string
	CanonicalAscii string  `gorm:"index"` // the Canonical string with only ASCII characters, for search.
	Latitude       float64 `gorm:"index"`
	Longitude      float64 `gorm:"index"`
}

// SearchWorldCities handles a type-ahead search query.
func SearchWorldCities(query string) ([]*WorldCities, error) {

	// Tokenize the user's search query (split it apart at spaces and commas).
	// For example: searching for "Rome, IT" is hard because it's spelled in the
	// DB as "Rome, Lazio, IT" and too many other Romes come up for searches.
	var (
		words        = strings.Fields(strings.ToLower(query))
		wheres       = []string{}
		placeholders = []any{}
	)
	for _, w := range words {
		like := fmt.Sprintf("%%%s%%", strings.Trim(w, ","))
		wheres = append(wheres, "(canonical ILIKE ? OR canonical_ascii ILIKE ?)")
		placeholders = append(placeholders, like, like)
	}

	if len(placeholders) == 0 {
		return nil, errors.New("no search terms")
	}

	var (
		result = []*WorldCities{}
		tx     = DB.Model(&WorldCities{}).Where(
			strings.Join(wheres, " AND "),
			placeholders...,
		).Order("canonical asc").Limit(50).Scan(&result)
	)
	return result, tx.Error
}

// NearestWorldCity returns the nearest matching WorldCity to a given geo coordinate.
func NearestWorldCity(lat, long float64) (*WorldCities, error) {
	if !config.Current.Database.IsPostgres {
		return nil, errors.New("NearestWorldCity requires a PostgreSQL database with the PostGIS extension")
	}

	var (
		result *WorldCities
		res    = DB.Model(&WorldCities{}).Order(
			fmt.Sprintf(`ST_Distance(
				ST_MakePoint(world_cities.longitude, world_cities.latitude)::geography,
				ST_MakePoint(%f, %f)::geography)`,
				long, lat,
			),
		).First(&result)
	)

	if res.Error != nil {
		return nil, res.Error
	}

	return result, nil
}

// FindWorldCity looks up a world city by its Canonical name from typeahead search.
func FindWorldCity(canonical string) (*WorldCities, error) {
	var (
		city   *WorldCities
		result = DB.Model(&WorldCities{}).Where("canonical = ?", canonical).First(&city)
	)
	if result.Error != nil {
		return nil, result.Error
	}
	return city, nil
}

// PrettyEmojiString returns a pretty printed string with a country flag emoji for this city.
//
// It is suitable for use in the chat room and showing on profile pages.
//
// Returns a result like "🇺🇸 United States, Oregon"
func (c *WorldCities) PrettyEmojiString() (string, error) {
	emoji, err := geoip.CountryFlagEmoji(c.ISO)
	if err != nil {
		return "", err
	}

	parts := []string{
		c.Country,
		c.State,
	}
	return emoji + " " + strings.Join(parts, ", "), nil
}

// InitializeWorldCities from an input CSV spreadsheet from simplemaps.com.
//
// This will wipe and reset the WorldCities table from the imported spreadsheet.
//
// The CSV file needs at least the columns: id, city, admin_name, country, iso2, lat, lng.
//
// The CSV can be downloaded from: https://simplemaps.com/data/world-cities
func InitializeWorldCities(csvFilename string, fh io.Reader) error {
	if fh == nil {
		file, err := os.Open(csvFilename)
		if err != nil {
			return err
		}
		fh = file
	}

	reader := csv.NewReader(fh)

	// Read the header row and find the required fields.
	fields := map[string]*int{
		"id":         nil,
		"city":       nil,
		"admin_name": nil,
		"country":    nil,
		"iso2":       nil,
		"lat":        nil,
		"lng":        nil,
	}
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("CSV header row: %s", err)
	}

	// Map the header fields to their index.
	for i, field := range header {
		if _, ok := fields[field]; ok {
			fields[field] = &i
		}
	}

	// Sanity check that all header fields are found.
	var errHeader error
	for k, v := range fields {
		if v == nil {
			log.Error("WorldCities CSV: required header %s not found", k)
			errHeader = errors.New("missing one or more required csv headers")
		}
	}
	if errHeader != nil {
		return errHeader
	}

	// Action!

	// Flush the existing WorldCities table.
	if tx := DB.Exec("DELETE FROM world_cities"); tx.Error != nil {
		return fmt.Errorf("deleting world_cities: %s", tx.Error)
	}

	// Populate the database.
	for {
		row, err := reader.Read()
		if err != nil {
			break
		}

		// Cast data types.
		id, err1 := strconv.ParseUint(row[*fields["id"]], 10, 64)
		lat, err2 := strconv.ParseFloat(row[*fields["lat"]], 64)
		lon, err3 := strconv.ParseFloat(row[*fields["lng"]], 64)
		if err1 != nil || err2 != nil || err3 != nil {
			return fmt.Errorf("row %+v failed to cast one or more data types: id (%s), lat (%s), lng (%s)", row, err1, err2, err3)
		}

		record := &WorldCities{
			ID:        id,
			City:      row[*fields["city"]],
			State:     row[*fields["admin_name"]],
			Country:   row[*fields["country"]],
			ISO:       row[*fields["iso2"]],
			Latitude:  lat,
			Longitude: lon,
		}

		// Canonical string for search.
		var canonical string
		if record.State != "" {
			canonical = fmt.Sprintf("%s, %s, %s", record.City, record.State, record.ISO)
		} else {
			canonical = fmt.Sprintf("%s, %s", record.City, record.ISO)
		}
		record.Canonical = canonical
		record.CanonicalAscii = utility.NormalizeUnicode(canonical)

		result := DB.Create(&record)
		if result.Error != nil {
			return fmt.Errorf("create row %+v: %s", record, result.Error)
		}

		log.Info("WorldCities: loaded %s, %s", record.City, record.Country)
	}

	return nil
}
