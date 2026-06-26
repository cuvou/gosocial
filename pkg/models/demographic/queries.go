package demographic

import (
	"errors"
	"sync"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"golang.org/x/sync/singleflight"
)

// Cached statistics (in case the queries are heavy to hit too often).
var (
	g                 = new(singleflight.Group)
	cachedDemographic Demographic
	cacheMu           sync.Mutex
)

// Get the current (cached) demographics result.
//
// This uses singleflight to ensure only the first goroutine (of multiple simultaneous requests) will run
// the (potentially heavy) query. Overlapping goroutines will wait and get the shared result of the first one.
// From there, the result is cached in RAM for a while.
func Get() (Demographic, error) {
	v, err, shared := g.Do("demographic-queries", func() (interface{}, error) {
		return GetAndCache()
	})

	if err != nil {
		return cachedDemographic, err
	}

	log.Error("demographic.Get: result was shared? %+v", shared)

	return v.(Demographic), err
}

// GetAndCache will return the cached demographic if existing/valid, or fetch a fresh result.
//
// Prefer to use the Get() method which will singleflight multiple requests, for better performance
// in case of a cache miss and multiple simultaneous requests.
func GetAndCache() (Demographic, error) {
	// Do we have the results cached?
	var result = cachedDemographic
	if !result.Computed || time.Since(result.LastUpdated) > config.DemographicsCacheTTL {
		cacheMu.Lock()
		defer cacheMu.Unlock()

		// If we have a race of threads: e.g. one request is pulling the stats and the second is locked.
		// Check if we have an updated result from the first thread.
		if time.Since(cachedDemographic.LastUpdated) < config.DemographicsCacheTTL {
			return cachedDemographic, nil
		}

		// Get the latest.
		res, err := Generate()
		if err != nil {
			return result, err
		}

		cachedDemographic = res
	}

	return cachedDemographic, nil
}

// Refresh the demographics cache, pulling fresh results from the database every time.
func Refresh() (Demographic, error) {
	cacheMu.Lock()
	cachedDemographic = Demographic{}
	cacheMu.Unlock()
	return Get()
}

// Generate the demographics result.
func Generate() (Demographic, error) {
	if !config.Current.Database.IsPostgres {
		return cachedDemographic, errors.New("this feature requires a PostgreSQL database")
	}

	result := Demographic{
		Computed:    true,
		LastUpdated: time.Now(),
		Photo:       PhotoStatistics(),
		People:      PeopleStatistics(),
	}

	return result, nil
}

// PeopleStatistics pulls various metrics about users of the website.
func PeopleStatistics() People {
	var result = People{
		ByAgeRange:    map[string]int64{},
		ByGender:      map[string]int64{"": 0},
		ByOrientation: map[string]int64{"": 0},
	}

	type record struct {
		MetricType  string
		MetricValue string
		MetricCount int64
	}
	var records []record
	res := models.DB.Raw(`
		-- Users who opt in/out of explicit content
		WITH subquery_explicit AS (
			SELECT
				SUM(CASE WHEN explicit IS TRUE THEN 1 ELSE 0 END) AS explicit_count,
				SUM(CASE WHEN explicit IS NOT TRUE THEN 1 ELSE 0 END) AS non_explicit_count
			FROM users
			WHERE users.status = 'active'
		),

		-- Users who share at least one explicit photo on public
		subquery_explicit_photo AS (
			SELECT
				COUNT(*) AS user_count
			FROM users
			WHERE users.status = 'active'
			AND EXISTS (
				SELECT 1
				FROM photos
				WHERE photos.user_id = users.id
				AND photos.explicit IS TRUE
				AND photos.visibility = 'public'
			)
		),

		-- User counts by age
		subquery_ages AS (
			SELECT
				CASE
					WHEN DATE_PART('year', AGE(birthdate)) BETWEEN 0 AND 25 THEN '18-25'
					WHEN DATE_PART('year', AGE(birthdate)) BETWEEN 26 and 35 THEN '26-35'
					WHEN DATE_PART('year', AGE(birthdate)) BETWEEN 36 and 45 THEN '36-45'
					WHEN DATE_PART('year', AGE(birthdate)) BETWEEN 46 and 55 THEN '46-55'
					WHEN DATE_PART('year', AGE(birthdate)) BETWEEN 56 and 65 THEN '56-65'
					WHEN DATE_PART('year', AGE(birthdate)) BETWEEN 66 and 75 THEN '66-75'
					WHEN DATE_PART('year', AGE(birthdate)) BETWEEN 76 and 85 THEN '76-85'
					ELSE '86+'
				END AS age_range,
				COUNT(*) AS user_count
			FROM
				users
			WHERE users.status = 'active'
			GROUP BY
				CASE
					WHEN DATE_PART('year', AGE(birthdate)) BETWEEN 0 AND 25 THEN '18-25'
					WHEN DATE_PART('year', AGE(birthdate)) BETWEEN 26 and 35 THEN '26-35'
					WHEN DATE_PART('year', AGE(birthdate)) BETWEEN 36 and 45 THEN '36-45'
					WHEN DATE_PART('year', AGE(birthdate)) BETWEEN 46 and 55 THEN '46-55'
					WHEN DATE_PART('year', AGE(birthdate)) BETWEEN 56 and 65 THEN '56-65'
					WHEN DATE_PART('year', AGE(birthdate)) BETWEEN 66 and 75 THEN '66-75'
					WHEN DATE_PART('year', AGE(birthdate)) BETWEEN 76 and 85 THEN '76-85'
					ELSE '86+'
				END
		),

		-- User counts by gender
		subquery_gender AS (
			SELECT
				profile_fields.value AS gender,
				COUNT(*) AS user_count
			FROM users
			JOIN profile_fields ON profile_fields.user_id = users.id
			WHERE users.status = 'active'
			AND profile_fields.name = 'gender'
			GROUP BY profile_fields.value
		),

		-- User counts by orientation
		subquery_orientation AS (
			SELECT
				profile_fields.value AS orientation,
				COUNT(*) AS user_count
			FROM users
			JOIN profile_fields ON profile_fields.user_id = users.id
			WHERE users.status = 'active'
			AND profile_fields.name = 'orientation'
			GROUP BY profile_fields.value
		)

		SELECT
			'ExplicitCount' AS metric_type,
			'explicit' AS metric_value,
			explicit_count AS metric_count
		FROM subquery_explicit

		UNION ALL

		SELECT
			'ExplicitPhotoCount' AS metric_type,
			'count' AS metric_value,
			user_count AS metric_count
		FROM subquery_explicit_photo

		UNION ALL

		SELECT
			'ExplicitCount' AS metric_type,
			'non_explicit' AS metric_value,
			non_explicit_count AS metric_count
		FROM subquery_explicit

		UNION ALL

		SELECT
			'AgeCounts' AS metric_type,
			age_range AS metric_value,
			user_count AS metric_count
		FROM subquery_ages

		UNION ALL

		SELECT
			'GenderCount' AS metric_type,
			gender AS metric_value,
			user_count AS metric_count
		FROM subquery_gender

		UNION ALL

		SELECT
			'OrientationCount' AS metric_type,
			orientation AS metric_value,
			user_count AS metric_count
		FROM subquery_orientation
	`).Scan(&records)
	if res.Error != nil {
		log.Error("PeopleStatistics: %s", res.Error)
		return result
	}

	// Ingest the records.
	var (
		totalWithAge         int64 // will be the total count of users since age is required
		totalWithGender      int64
		totalWithOrientation int64
	)
	for _, row := range records {
		switch row.MetricType {
		case "ExplicitCount":
			result.Total += row.MetricCount
			if row.MetricValue == "explicit" {
				result.ExplicitOptIn = row.MetricCount
			}
		case "ExplicitPhotoCount":
			result.ExplicitPhoto = row.MetricCount
		case "AgeCounts":
			if _, ok := result.ByAgeRange[row.MetricValue]; !ok {
				result.ByAgeRange[row.MetricValue] = 0
			}
			result.ByAgeRange[row.MetricValue] += row.MetricCount
			totalWithAge += row.MetricCount
		case "GenderCount":
			if _, ok := result.ByGender[row.MetricValue]; !ok {
				result.ByGender[row.MetricValue] = 0
			}
			result.ByGender[row.MetricValue] += row.MetricCount
			totalWithGender += row.MetricCount
		case "OrientationCount":
			if _, ok := result.ByOrientation[row.MetricValue]; !ok {
				result.ByOrientation[row.MetricValue] = 0
			}
			result.ByOrientation[row.MetricValue] += row.MetricCount
			totalWithOrientation += row.MetricCount
		}
	}

	// Gender and Orientation: pad out the "no answer" selection with the count of users
	// who had no profile_fields stored in the DB at all.
	result.ByOrientation[""] += (totalWithAge - totalWithOrientation)
	result.ByGender[""] += (totalWithAge - totalWithGender)

	return result
}

// PhotoStatistics gets info about photo usage on the website.
//
// Counts of Explicit vs. Non-Explicit photos.
func PhotoStatistics() Photo {
	var result Photo
	type record struct {
		Explicit bool
		C        int64
	}
	var records []record

	res := models.DB.Raw(`
		SELECT
			photos.explicit,
			count(photos.id) AS c
		FROM
			photos
		JOIN users ON (photos.user_id = users.id)
		WHERE photos.visibility = 'public'
		AND photos.gallery IS TRUE
		AND users.status = 'active'
		GROUP BY photos.explicit
		ORDER BY c DESC
	`).Scan(&records)
	if res.Error != nil {
		log.Error("PhotoStatistics: %s", res.Error)
		return result
	}

	for _, row := range records {
		result.Total += row.C
		if row.Explicit {
			result.Explicit += row.C
		} else {
			result.NonExplicit += row.C
		}
	}

	return result
}
