// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildConversationText(t *testing.T) {
	t.Run("build conversation with posts", func(t *testing.T) {
		posts := []*model.Post{
			{
				Id:       model.NewId(),
				Message:  "Hello world",
				UserId:   "user1",
				CreateAt: 1234567890000,
				Props: model.StringInterface{
					"username": "john_doe",
				},
			},
			{
				Id:       model.NewId(),
				Message:  "How are you?",
				UserId:   "user2",
				CreateAt: 1234567895000,
				Props: model.StringInterface{
					"username": "jane_smith",
				},
			},
		}

		result, _ := buildConversationTextWithIDs(posts)
		assert.Contains(t, result, "john_doe")
		assert.Contains(t, result, "jane_smith")
		assert.Contains(t, result, "Hello world")
		assert.Contains(t, result, "How are you?")
	})

	t.Run("build conversation with posts without username", func(t *testing.T) {
		posts := []*model.Post{
			{
				Id:       model.NewId(),
				Message:  "Test message",
				UserId:   "user123",
				CreateAt: 1234567890000,
				Props:    model.StringInterface{},
			},
		}

		result, _ := buildConversationTextWithIDs(posts)
		// Should fallback to user ID when no username prop
		assert.Contains(t, result, "user123")
		assert.Contains(t, result, "Test message")
	})

	t.Run("build conversation with empty posts", func(t *testing.T) {
		posts := []*model.Post{}
		result, _ := buildConversationTextWithIDs(posts)
		assert.Equal(t, "", result)
	})
}

func TestSummarizePosts(t *testing.T) {
	t.Run("successful recap completion parsing", func(t *testing.T) {
		th, llm, _ := setupAIFeatureModel(t, `{"highlights":["Highlight 1"],"action_items":["Action 1"]}`)
		ctx := th.Context.WithSession(&model.Session{UserId: th.BasicUser.Id})
		posts := []*model.Post{{
			Id:       model.NewId(),
			UserId:   th.BasicUser.Id,
			Message:  "Important update",
			CreateAt: model.GetMillis(),
			Props: model.StringInterface{
				"username": th.BasicUser.Username,
			},
		}}

		summary, appErr := th.App.SummarizePosts(ctx, th.BasicUser.Id, posts, th.BasicChannel.DisplayName, th.BasicTeam.Name, model.NewId())
		require.Nil(t, appErr)
		require.NotNil(t, summary)
		assert.Equal(t, []string{"Highlight 1"}, summary.Highlights)
		assert.Equal(t, []string{"Action 1"}, summary.ActionItems)
		require.Len(t, llm.calls(), 1)
		assert.Equal(t, "recap", llm.calls()[0].schemaName(), "the recap asks for its JSON schema")
		assertAIUsageFor(t, th, th.BasicUser.Id)
	})

	t.Run("null arrays normalize to empty slices", func(t *testing.T) {
		th, _, _ := setupAIFeatureModel(t, `{"highlights":null,"action_items":null}`)
		ctx := th.Context.WithSession(&model.Session{UserId: th.BasicUser.Id})
		posts := []*model.Post{{
			Id:       model.NewId(),
			UserId:   th.BasicUser.Id,
			Message:  "Need to follow up",
			CreateAt: model.GetMillis(),
			Props: model.StringInterface{
				"username": th.BasicUser.Username,
			},
		}}

		summary, appErr := th.App.SummarizePosts(ctx, th.BasicUser.Id, posts, th.BasicChannel.DisplayName, th.BasicTeam.Name, model.NewId())
		require.Nil(t, appErr)
		require.NotNil(t, summary)
		assert.Empty(t, summary.Highlights)
		assert.Empty(t, summary.ActionItems)
		assert.NotNil(t, summary.Highlights)
		assert.NotNil(t, summary.ActionItems)
	})

	t.Run("a failing model returns agent call failed", func(t *testing.T) {
		th, llm, _ := setupAIFeatureModel(t, "")
		llm.fail(http.StatusInternalServerError, "model failed")
		ctx := th.Context.WithSession(&model.Session{UserId: th.BasicUser.Id})
		posts := []*model.Post{{
			Id:       model.NewId(),
			UserId:   th.BasicUser.Id,
			Message:  "Need help",
			CreateAt: model.GetMillis(),
			Props: model.StringInterface{
				"username": th.BasicUser.Username,
			},
		}}

		summary, appErr := th.App.SummarizePosts(ctx, th.BasicUser.Id, posts, th.BasicChannel.DisplayName, th.BasicTeam.Name, model.NewId())
		require.Nil(t, summary)
		require.NotNil(t, appErr)
		assert.Equal(t, "app.ai.summarize.agent_call_failed", appErr.Id)
	})

	t.Run("invalid json returns parse failed", func(t *testing.T) {
		th, _, _ := setupAIFeatureModel(t, "{invalid json")
		ctx := th.Context.WithSession(&model.Session{UserId: th.BasicUser.Id})
		posts := []*model.Post{{
			Id:       model.NewId(),
			UserId:   th.BasicUser.Id,
			Message:  "Broken payload",
			CreateAt: model.GetMillis(),
			Props: model.StringInterface{
				"username": th.BasicUser.Username,
			},
		}}

		summary, appErr := th.App.SummarizePosts(ctx, th.BasicUser.Id, posts, th.BasicChannel.DisplayName, th.BasicTeam.Name, model.NewId())
		require.Nil(t, summary)
		require.NotNil(t, appErr)
		assert.Equal(t, "app.ai.summarize.parse_failed", appErr.Id)
	})

	t.Run("empty posts short circuit without a model call", func(t *testing.T) {
		th, llm, _ := setupAIFeatureModel(t, "")
		ctx := th.Context.WithSession(&model.Session{UserId: th.BasicUser.Id})

		summary, appErr := th.App.SummarizePosts(ctx, th.BasicUser.Id, []*model.Post{}, th.BasicChannel.DisplayName, th.BasicTeam.Name, model.NewId())
		require.Nil(t, appErr)
		require.NotNil(t, summary)
		assert.Empty(t, summary.Highlights)
		assert.Empty(t, summary.ActionItems)
		assert.Len(t, llm.calls(), 0)
	})

	t.Run("custom instructions are included in prompt", func(t *testing.T) {
		th, llm, _ := setupAIFeatureModel(t, `{"highlights":["Highlight 1"],"action_items":[]}`)
		ctx := th.Context.WithSession(&model.Session{UserId: th.BasicUser.Id})
		posts := []*model.Post{{
			Id:        model.NewId(),
			UserId:    th.BasicUser.Id,
			ChannelId: th.BasicChannel.Id,
			Message:   "Important update",
			CreateAt:  model.GetMillis(),
			Props: model.StringInterface{
				"username": th.BasicUser.Username,
			},
		}}

		_, appErr := th.App.SummarizePostsWithInstructions(ctx, th.BasicUser.Id, posts, th.BasicChannel.DisplayName, th.BasicTeam.Name, model.NewId(), "Focus on launch risks")
		require.Nil(t, appErr)
		require.Len(t, llm.calls(), 1)
		messages := llm.calls()[0].messages()
		require.Len(t, messages, 2)
		assert.Contains(t, messages[1]["content"], "Additional user instructions:")
		assert.Contains(t, messages[1]["content"], "Focus on launch risks")
	})
}
