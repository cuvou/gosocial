package config_test

import (
	"testing"

	"github.com/cuvou/gosocial/pkg/config"
)

// TestAdminScopesCount validates that all named admin scopes are
// returned by the scope list function.
func TestAdminScopesCount(t *testing.T) {
	var scopes = config.ListAdminScopes()
	if len(scopes) != config.QuantityAdminScopes || len(scopes) != len(config.AdminScopeDescriptions)-1 {
		t.Errorf(
			"The list of scopes returned by ListAdminScopes doesn't match the expected count. "+
				"Expected %d (with %d descriptions), got %d",
			config.QuantityAdminScopes, len(config.AdminScopeDescriptions),
			len(scopes),
		)
	}
}
