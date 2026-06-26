package utility

import (
	"fmt"
	"math"
)

// HarversineDistanceString formats a pretty distance like "3.2km / 2.0mi" in friendly units.
func HaversineDistanceString(lat1, lon1, lat2, lon2 float64) string {
	km, mi := HaversineDistance(lat1, lon1, lat2, lon2)
	return fmt.Sprintf("%skm / %smi",
		FormatFloatCommas(km, 1),
		FormatFloatCommas(mi, 1),
	)
}

// HaversineDistance returns the distance (in kilometers, miles) between
// two points of latitude and longitude pairs.
func HaversineDistance(lat1, lon1, lat2, lon2 float64) (float64, float64) {
	lat1 *= piRad
	lon1 *= piRad
	lat2 *= piRad
	lon2 *= piRad
	var r = earthRadius

	// Calculate.
	h := hsin(lat2-lat1) + math.Cos(lat1)*math.Cos(lat2)*hsin(lon2-lon1)

	meters := 2 * r * math.Asin(math.Sqrt(h))
	kilometers := meters / 1000
	miles := kilometers * 0.621371
	return kilometers, miles
}

// adapted from: https://gist.github.com/cdipaolo/d3f8db3848278b49db68
// haversin(θ) function
func hsin(theta float64) float64 {
	return math.Pow(math.Sin(theta/2), 2)
}

const (
	piRad       = math.Pi / 180
	earthRadius = 6378100.0 // Earth radius in meters
)
