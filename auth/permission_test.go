package auth

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPermissionRequirement_FulfillsRequirement(t *testing.T) {
	var testCases = []struct {
		requirement string
		permission  string
		expected    bool
	}{
		{"namespace.service.resource.verb", "namespace.service.resource.verb", true},
		{"namespace.service.resource.verb", "namespace.service.resource.other", false},
		{"namespace.service.resource.verb", "namespace.service.other.verb", false},
		{"namespace.service.resource.verb", "namespace.other.resource.verb", false},
		{"namespace.service.resource.verb", "other.service.resource.verb", false},
		{"namespace.service.resource.verb", "namespace.service.resource.*", true},
		{"namespace.service.resource.verb", "namespace.service.*.verb", true},
		{"namespace.service.resource.verb", "namespace.*.resource.verb", true},
		{"namespace.service.resource.verb", "*.service.resource.verb", true},
		{"namespace.service.resource.verb", "*.*.*.*", true},
	}

	for _, c := range testCases {
		t.Run(fmt.Sprintf("%v_%v", c.requirement, c.permission), func(t *testing.T) {
			permission, err := ParsePermissionString(c.permission)
			require.NoError(t, err)
			require.Equal(t, c.expected, ParsePermissionRequirementOrDie(c.requirement).FulfillsRequirement(permission))
		})
	}
}
