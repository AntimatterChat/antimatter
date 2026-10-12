// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOnTopPost(t *testing.T) {
	t.Run("IsActive", func(t *testing.T) {
		assert.True(t, (&OnTopPost{}).IsActive(1000), "until removed")
		assert.True(t, (&OnTopPost{ExpiresAt: 2000}).IsActive(1000))
		assert.False(t, (&OnTopPost{ExpiresAt: 1000}).IsActive(1000))
		assert.False(t, (*OnTopPost)(nil).IsActive(1000))
	})

	t.Run("IsValid", func(t *testing.T) {
		assert.Nil(t, (&OnTopPostRequest{Duration: 0}).IsValid())
		assert.Nil(t, (&OnTopPostRequest{Duration: OnTopPostMaxDuration.Milliseconds()}).IsValid())
		assert.NotNil(t, (&OnTopPostRequest{Duration: -1}).IsValid())
		assert.NotNil(t, (&OnTopPostRequest{Duration: OnTopPostMaxDuration.Milliseconds() + 1}).IsValid())
	})
}
