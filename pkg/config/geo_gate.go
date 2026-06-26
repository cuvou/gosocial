package config

// GeoIP gating rules. TODO: make dynamically configurable.
var GeoGateEnabled bool

// GeoIP database path (standard location on Fedora/Debian)
const GeoIPPath = "/usr/share/GeoIP/GeoLite2-City.mmdb"

// US states to block.
var BlockUSStates = map[string]interface{}{
	"UT": nil, // Utah
	"LA": nil, // Louisiana
	"MS": nil, // Mississippi
	"AR": nil, // Arkansas
	"MT": nil, // Montana
	"TX": nil, // Texas
}

// Countries to block.
var BlockCountries = map[string]interface{}{
	// "US": nil, // TEST
}
