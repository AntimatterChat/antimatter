// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// Antimatter's limits on group messages and community attributes.

func TestGroupMessageSizeLimit(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	require.Equal(t, 10, model.ChannelGroupMaxUsers, "a group message holds up to 10 people")

	ids := []string{th.BasicUser.Id}
	for len(ids) < model.ChannelGroupMaxUsers+1 {
		ids = append(ids, th.CreateUser(t).Id)
	}

	t.Run("a group message of the largest size can be created", func(t *testing.T) {
		gm, appErr := th.App.CreateGroupChannel(th.Context, ids[:model.ChannelGroupMaxUsers], th.BasicUser.Id)
		require.Nil(t, appErr)
		require.NotNil(t, gm)
	})

	t.Run("one more person is refused", func(t *testing.T) {
		gm, appErr := th.App.CreateGroupChannel(th.Context, ids, th.BasicUser.Id)
		require.NotNil(t, appErr)
		require.Nil(t, gm)
	})
}

func TestCommunityAttributesLimit(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	require.Equal(t, 50, model.CommunityAttributesMaxFields, "a community defines up to 50 attributes")

	for i := range model.CommunityAttributesMaxFields {
		_, appErr := th.App.CreateCommunityAttribute(th.Context, th.BasicTeam.Id, th.BasicUser.Id,
			&model.PropertyField{Name: fmt.Sprintf("attribute %d", i), Type: model.PropertyFieldTypeText})
		require.Nil(t, appErr, "attribute %d", i)
	}

	_, appErr := th.App.CreateCommunityAttribute(th.Context, th.BasicTeam.Id, th.BasicUser.Id,
		&model.PropertyField{Name: "one too many", Type: model.PropertyFieldTypeText})
	require.NotNil(t, appErr)
	require.Equal(t, "app.community_attributes.too_many.app_error", appErr.Id)
}
