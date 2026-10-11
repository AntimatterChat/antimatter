// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package utils_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost/server/public/utils"
)

func TestLookupServerEnv(t *testing.T) {
	const name = "UTILS_TEST_SETTING"

	t.Run("neither set", func(t *testing.T) {
		value, ok := utils.LookupServerEnv(name)
		assert.False(t, ok)
		assert.Empty(t, value)
		assert.Empty(t, utils.GetServerEnv(name))
	})

	t.Run("AM_ only", func(t *testing.T) {
		t.Setenv("AM_"+name, "am")
		value, ok := utils.LookupServerEnv(name)
		assert.True(t, ok)
		assert.Equal(t, "am", value)
	})

	t.Run("MM_ only", func(t *testing.T) {
		t.Setenv("MM_"+name, "mm")
		value, ok := utils.LookupServerEnv(name)
		assert.True(t, ok)
		assert.Equal(t, "mm", value)
	})

	t.Run("both set, AM_ wins", func(t *testing.T) {
		t.Setenv("AM_"+name, "am")
		t.Setenv("MM_"+name, "mm")
		assert.Equal(t, "am", utils.GetServerEnv(name))
	})

	t.Run("empty AM_ still wins", func(t *testing.T) {
		t.Setenv("AM_"+name, "")
		t.Setenv("MM_"+name, "mm")
		value, ok := utils.LookupServerEnv(name)
		assert.True(t, ok)
		assert.Empty(t, value)
	})
}
