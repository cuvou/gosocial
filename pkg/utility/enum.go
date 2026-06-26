package utility

import (
	"slices"

	"github.com/cuvou/gosocial/pkg/config"
)

// StringIn narrows a string to one of the allowable options.
func StringIn(v string, options []string, orDefault string) string {
	if slices.Contains(options, v) {
		return v
	}
	return orDefault
}

// StringInOptions constrains a string value (posted by the user) to only be one
// of the available values in the Option list enum. Returns the string if OK, or
// else the default string.
func StringInOptions(v string, options []config.Option, orDefault string) string {
	for _, option := range options {
		if v == option.Value {
			return v
		}
	}
	return orDefault
}

// StringInOptGroup constrains a string value (posted by the user) to only be one
// of the available values in the Option list enum. Returns the string if OK, or
// else the default string.
func StringInOptGroup(v string, options []config.OptGroup, orDefault string) string {
	for _, group := range options {
		for _, option := range group.Options {
			if v == option.Value {
				return v
			}
		}
	}
	return orDefault
}
