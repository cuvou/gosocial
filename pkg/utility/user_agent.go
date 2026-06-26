package utility

import (
	"fmt"

	"github.com/mileusna/useragent"
)

// UserAgent contains parsed User-Agent properties used by the website, mainly
// to enhance the Login Sessions page.
//
// It is a thin wrapper for mileusna/useragent with fixed/known fields to
// ensure reliability for the HTML templates.
type UserAgent struct {
	ua useragent.UserAgent
}

// ParseUserAgent parses a browser UA string.
func ParseUserAgent(userAgent string) UserAgent {
	ua := useragent.Parse(userAgent)
	return UserAgent{
		ua: ua,
	}
}

// DeviceType returns whether it is a: Mobile, Tablet, Desktop, Bot.
func (ua UserAgent) DeviceType() string {
	if ua.ua.Mobile {
		return "Mobile"
	} else if ua.ua.Tablet {
		return "Tablet"
	} else if ua.ua.Desktop {
		return "Desktop"
	} else if ua.ua.Bot {
		return "Bot"
	} else {
		return "Unknown Device"
	}
}

// DeviceIcon returns a FontAwesome icon class for the device type.
func (ua UserAgent) DeviceIcon() string {
	if ua.ua.Mobile {
		return "fa fa-mobile-screen-button"
	} else if ua.ua.Tablet {
		return "fa fa-tablet-screen-button"
	} else if ua.ua.Desktop {
		return "fa fa-desktop"
	} else if ua.ua.Bot {
		return "fa fa-robot"
	} else {
		return "fa-solid fa-computer"
	}
}

// OSIcon returns a FontAwesome icon class representing the operating system (Android, iOS, Windows, etc.)
func (ua UserAgent) OSIcon() string {
	if ua.ua.IsAndroid() {
		return "fab fa-android"
	} else if ua.ua.IsIOS() || ua.ua.IsMacOS() {
		return "fab fa-apple"
	} else if ua.ua.IsWindows() {
		return "fab fa-windows"
	} else if ua.ua.IsLinux() {
		return "fab fa-linux"
	} else if ua.ua.IsChromeOS() {
		return "fab fa-chrome"
	} else if ua.ua.IsBlackBerry() || ua.ua.IsBlackberryOS() {
		return "fab fa-blackberry"
	} else {
		// Unknown.
		return "fa-solid fa-computer"
	}
}

// BrowserName returns the name and version of the browser, like "Firefox v105"
func (ua UserAgent) BrowserName() string {
	return fmt.Sprintf("%s %s", ua.ua.Name, ua.ua.Version)
}

// OS returns the OS name and version.
func (ua UserAgent) OS() string {
	return fmt.Sprintf("%s %s", ua.ua.OS, ua.ua.OSVersion)
}

// Device returns the device name.
func (ua UserAgent) Device() string {
	return ua.ua.Device
}

// BrowserIcon returns a FontAwesome icon class for the web browser type.
func (ua UserAgent) BrowserIcon() string {
	if ua.ua.IsChrome() {
		return "fab fa-chrome"
	} else if ua.ua.IsEdge() {
		return "fab fa-edge"
	} else if ua.ua.IsFirefox() {
		return "fab fa-firefox"
	} else if ua.ua.IsSafari() {
		return "fab fa-safari"
	} else if ua.ua.IsOpera() || ua.ua.IsOperaMini() {
		return "fab fa-opera"
	} else if ua.ua.IsBlackBerry() || ua.ua.IsBlackberryOS() {
		return "fab fa-blackberry"
	} else {
		return "fa fa-globe"
	}
}
