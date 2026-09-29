// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestRevealBurnOnReadPostsForUser(t *testing.T) {
	th := Setup(t).InitBasic(t)

	// Enable BurnOnRead feature
	th.App.UpdateConfig(func(cfg *model.Config) {
		cfg.FeatureFlags.BurnOnRead = true
		cfg.ServiceSettings.EnableBurnOnRead = new(true)
	})

	t.Run("skips deleted burn-on-read post", func(t *testing.T) {
		deletedPost := &model.Post{
			Id:        model.NewId(),
			UserId:    th.BasicUser.Id,
			ChannelId: th.BasicChannel.Id,
			Message:   "deleted burn on read message",
			Type:      model.PostTypeBurnOnRead,
			DeleteAt:  model.GetMillis(),
			CreateAt:  model.GetMillis(),
		}

		postList := model.NewPostList()
		postList.AddPost(deletedPost)

		resultList, appErr := th.App.revealBurnOnReadPostsForUser(th.Context, postList, th.BasicUser2.Id)

		require.Nil(t, appErr)
		require.NotNil(t, resultList)
		// The deleted post should remain in BurnOnReadPosts but not be processed
		assert.Contains(t, resultList.BurnOnReadPosts, deletedPost.Id)
		// Verify the post was not modified (still has DeleteAt set)
		assert.Equal(t, deletedPost.DeleteAt, resultList.BurnOnReadPosts[deletedPost.Id].DeleteAt)
		assert.Equal(t, deletedPost.Message, resultList.BurnOnReadPosts[deletedPost.Id].Message)
	})
}
