// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommunityRulesIsActive(t *testing.T) {
	var none *CommunityRules
	assert.False(t, none.IsActive())
	assert.False(t, (&CommunityRules{Version: 0, Enabled: true}).IsActive(), "rules never saved hold nobody")
	assert.False(t, (&CommunityRules{Version: 2, Enabled: false}).IsActive(), "rules turned off hold nobody")
	assert.True(t, (&CommunityRules{Version: 1, Enabled: true}).IsActive())
}

func TestCommunityRulesRequestIsValid(t *testing.T) {
	request := &CommunityRulesRequest{Enabled: true, Text: "  Be kind.\n"}
	request.PreSave()
	assert.Equal(t, "Be kind.", request.Text)
	require.Nil(t, request.IsValid())

	off := &CommunityRulesRequest{Enabled: false, Text: ""}
	assert.Nil(t, off.IsValid(), "rules may be turned off empty")

	empty := &CommunityRulesRequest{Enabled: true, Text: "   "}
	empty.PreSave()
	appErr := empty.IsValid()
	require.NotNil(t, appErr, "rules aren't turned on without text")
	assert.Equal(t, "model.community_rules.empty.app_error", appErr.Id)

	long := &CommunityRulesRequest{Text: strings.Repeat("é", CommunityRulesMaxRunes+1)}
	appErr = long.IsValid()
	require.NotNil(t, appErr)
	assert.Equal(t, "model.community_rules.text.app_error", appErr.Id)
	assert.Nil(t, (&CommunityRulesRequest{Text: strings.Repeat("é", CommunityRulesMaxRunes)}).IsValid(), "the bound counts characters, not bytes")
}
