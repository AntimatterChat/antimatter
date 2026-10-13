// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPropertyFieldValueRestricted(t *testing.T) {
	field := func(visibility any) *PropertyField {
		return &PropertyField{Attrs: StringInterface{PropertyFieldAttrVisibility: visibility}}
	}
	value := json.RawMessage(`"Paris"`)

	for visibility, restricted := range map[any]bool{
		PropertyFieldVisibilityAdmin:   true,
		PropertyFieldVisibilityHidden:  true,
		" admin ":                      true,
		PropertyFieldVisibilityAlways:  false,
		PropertyFieldVisibilityWhenSet: false,
		"":                             false,
		42:                             false,
	} {
		assert.Equal(t, restricted, IsPropertyFieldValueRestricted(field(visibility)), visibility)
		if restricted {
			assert.JSONEq(t, PropertyValueWithheldJSON, string(BroadcastUserAttributeValue(field(visibility), value)), visibility)
		} else {
			assert.Equal(t, value, BroadcastUserAttributeValue(field(visibility), value), visibility)
		}
	}
	assert.False(t, IsPropertyFieldValueRestricted(nil))
	assert.False(t, IsPropertyFieldValueRestricted(&PropertyField{}))
	assert.True(t, IsValidPropertyFieldVisibility(PropertyFieldVisibilityAdmin))
}
