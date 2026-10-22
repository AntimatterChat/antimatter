// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package oidc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClaimValues(t *testing.T) {
	claims := Claims{
		"department":                "R&D",
		"empty":                     "",
		"groups":                    []any{"admins", "", "devs", map[string]any{"x": "y"}, json.Number("7")},
		"level":                     float64(3),
		"active":                    true,
		"nothing":                   nil,
		"address":                   map[string]any{"country": "FR", "geo": map[string]any{"city": "Paris"}},
		"https://example.com/roles": []any{"owner"},
		"a.b":                       "dotted name",
		"a":                         map[string]any{"b": "nested"},
	}

	for path, want := range map[string][]string{
		"department":                {"R&D"},
		" department ":              {"R&D"},
		"groups":                    {"admins", "devs", "7"},
		"level":                     {"3"},
		"active":                    {"true"},
		"address.country":           {"FR"},
		"address.geo.city":          {"Paris"},
		"https://example.com/roles": {"owner"},
		"a.b":                       {"dotted name"},
	} {
		assert.Equal(t, want, ClaimValues(claims, path), path)
	}

	for _, path := range []string{"", "empty", "nothing", "address", "address.missing", "missing", "department.x", ".department"} {
		assert.Empty(t, ClaimValues(claims, path), path)
	}
	assert.Empty(t, ClaimValues(nil, "department"))
}
