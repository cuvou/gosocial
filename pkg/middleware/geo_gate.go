package middleware

import (
	"net"
	"net/http"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/controller/index"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/utility"
	"github.com/oschwald/geoip2-golang"
)

// GeoGate: block access to the site based on the user's location (due to local laws or regulations).
func GeoGate(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !config.GeoGateEnabled {
			handler.ServeHTTP(w, r)
			return
		}

		// Flash errors to admins.
		onError := func(err error) {
			session.FlashError(w, r, "GeoIP: %s", err)
			handler.ServeHTTP(w, r)
		}

		// See where they're coming from.
		db, err := geoip2.Open(config.GeoIPPath)
		if err != nil {
			onError(err)
			return
		}
		defer db.Close()

		// If you are using strings that may be invalid, check that ip is not nil
		addr := utility.IPAddress(r)
		ip := net.ParseIP(addr)
		if ip != nil {
			record, err := db.City(ip)
			if err != nil {
				onError(err)
				return
			}

			// Blocked by US states
			if record.Country.IsoCode == "US" {
				for _, sub := range record.Subdivisions {
					if _, ok := config.BlockUSStates[sub.IsoCode]; ok {
						session.LogoutUser(w, r)
						page := index.StaticTemplate("errors/geo_gate.html")()
						page(w, r)
						return
					}
				}
			}

			// Blocked by country code
			if _, ok := config.BlockCountries[record.Country.IsoCode]; ok {
				session.LogoutUser(w, r)
				page := index.StaticTemplate("errors/geo_gate.html")()
				page(w, r)
				return
			}

			// Debug info
			/*
				fmt.Printf("Portuguese (BR) city name: %v\n", record.City.Names["pt-BR"])
				if len(record.Subdivisions) > 0 {
					fmt.Printf("English subdivision name: %v\n", record.Subdivisions[0].Names["en"])
				}
				fmt.Printf("Russian country name: %v\n", record.Country.Names["ru"])
				fmt.Printf("ISO country code: %v\n", record.Country.IsoCode)
				fmt.Printf("Time zone: %v\n", record.Location.TimeZone)
				fmt.Printf("Coordinates: %v, %v\n", record.Location.Latitude, record.Location.Longitude)
				// Output:
				// Portuguese (BR) city name: Londres
				// English subdivision name: England
				// Russian country name: Великобритания
				// ISO country code: GB
				// Time zone: Europe/London
				// Coordinates: 51.5142, -0.0931
			*/
		}

		handler.ServeHTTP(w, r)
	})
}
