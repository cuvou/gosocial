package flairs

// Flair adds an optional custom icon and decoration to users on chat (e.g. like VIP but no special features).
type Flair struct {
	// Plain text of the flair, e.g. "Newbie"
	Name string `json:"name,omitempty"`

	// Bulma CSS color class for the flair icon and tag color in Profile Cards.
	// One of: is-info, is-success, is-warning, is-danger, ...
	// Default: is-info.
	Color string `json:"color,omitempty"`

	// FontAwesome CSS classes for the icon.
	// e.g.: "fa fa-star"
	// Default: "fa fa-star"
	Icon string `json:"icon,omitempty"`

	// NoVIP boolean: if set on a user with VIP status, their VIP flair is hidden from display.
	NoVIP bool `json:"noVIP,omitempty"`
}
