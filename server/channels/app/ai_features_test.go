// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/ai"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/app/autotranslation"
)

// The server's own AI features run on the connections of AISettings, through the AI service: not the AI chat app.

func useAIFeatureConnection(t *testing.T, th *TestHelper, set func(cfg *model.Config, connectionID string), c *model.AIConnection) {
	t.Helper()
	th.App.UpdateConfig(func(cfg *model.Config) { set(cfg, c.Id) })
}

func TestRewriteUsesItsConnection(t *testing.T) {
	th := setupAITest(t)
	ctx := th.Context.WithSession(&model.Session{UserId: th.BasicUser.Id})

	_, appErr := th.App.RewriteMessage(ctx, "", "original message", model.RewriteActionImproveWriting, "", "")
	require.NotNil(t, appErr, "no connection chosen")
	assert.Equal(t, "app.ai.feature.not_configured.app_error", appErr.Id)

	f := newFakeLLM(t, `{"rewritten_text":"A better message"}`)
	c := createTestAIConnection(t, th, f, model.AIConnectionScopeServer, func(req *model.AIConnectionRequest) {
		// Built-in features don't depend on who the connection is shared with.
		req.Sharing = model.AIConnectionSharing{}
	})
	useAIFeatureConnection(t, th, func(cfg *model.Config, id string) {
		cfg.AISettings.Rewrite.ConnectionId = model.NewPointer(id)
		cfg.AISettings.Rewrite.Model = model.NewPointer("rewrite-model")
	}, c)

	response, appErr := th.App.RewriteMessage(ctx, "", "original message", model.RewriteActionImproveWriting, "", "")
	require.Nil(t, appErr)
	assert.Equal(t, "A better message", response.RewrittenText)

	calls := f.calls()
	require.Len(t, calls, 1)
	assert.Equal(t, "/v1/chat/completions", calls[0].Path)
	assert.Equal(t, "Bearer "+aiTestKey, calls[0].Authorization)
	assert.Equal(t, "rewrite-model", calls[0].Body["model"])
	format := calls[0].Body["response_format"].(map[string]any)
	assert.Equal(t, "json_schema", format["type"], "the rewrite asks for its JSON schema")
	messages := calls[0].Body["messages"].([]any)
	assert.Equal(t, "system", messages[0].(map[string]any)["role"])
	assert.Contains(t, messages[1].(map[string]any)["content"], "original message")

	usage, appErr := th.App.GetAIConnectionUsage(th.Context, c)
	require.Nil(t, appErr)
	assert.Equal(t, int64(1), usage.ByPurpose[string(ai.PurposeRewrite)].Requests)
	require.NotEmpty(t, usage.Days)
	assert.Equal(t, th.BasicUser.Id, usage.Days[0].UserId, "counted against the person rewriting")

	t.Run("not on a community's connection", func(t *testing.T) {
		community := createTestAIConnection(t, th, f, th.BasicTeam.Id, nil)
		useAIFeatureConnection(t, th, func(cfg *model.Config, id string) { cfg.AISettings.Rewrite.ConnectionId = model.NewPointer(id) }, community)
		_, appErr := th.App.RewriteMessage(ctx, "", "original message", model.RewriteActionShorten, "", "")
		require.NotNil(t, appErr)
		assert.Equal(t, "app.ai.call.builtin_server_only.app_error", appErr.Id)
	})
}

func TestRecapsUseTheirConnection(t *testing.T) {
	th := setupAITest(t)
	ctx := th.Context.WithSession(&model.Session{UserId: th.BasicUser.Id})
	posts := []*model.Post{{
		Id: model.NewId(), UserId: th.BasicUser.Id, ChannelId: th.BasicChannel.Id, Message: "We ship on Friday",
		CreateAt: model.GetMillis(), Props: model.StringInterface{"username": th.BasicUser.Username},
	}}

	_, appErr := th.App.SummarizePosts(ctx, th.BasicUser.Id, posts, th.BasicChannel.DisplayName, th.BasicTeam.Name, "")
	require.NotNil(t, appErr)
	assert.Equal(t, "app.ai.feature.not_configured.app_error", appErr.Id)

	f := newFakeLLM(t, `{"highlights":["Shipping on Friday"],"action_items":["Prepare the release"]}`)
	c := createTestAIConnection(t, th, f, model.AIConnectionScopeServer, nil)
	useAIFeatureConnection(t, th, func(cfg *model.Config, id string) { cfg.AISettings.Recaps.ConnectionId = model.NewPointer(id) }, c)

	summary, appErr := th.App.SummarizePosts(ctx, th.BasicUser.Id, posts, th.BasicChannel.DisplayName, th.BasicTeam.Name, "")
	require.Nil(t, appErr)
	assert.Equal(t, []string{"Shipping on Friday"}, summary.Highlights)
	assert.Equal(t, []string{"Prepare the release"}, summary.ActionItems)
	assert.Equal(t, "test-model", f.calls()[0].Body["model"], "the connection's chat model when the setting has none")

	usage, appErr := th.App.GetAIConnectionUsage(th.Context, c)
	require.Nil(t, appErr)
	assert.Equal(t, int64(1), usage.ByPurpose[string(ai.PurposeRecaps)].Requests)
}

func TestAutoTranslationUsesItsConnection(t *testing.T) {
	th := setupAITest(t)
	f := newFakeLLM(t, `{"translations":["Hola"],"source_language":"en","confidence":0.9}`)
	c := createTestAIConnection(t, th, f, model.AIConnectionScopeServer, nil)
	host := newAutoTranslationHost(th.App.Srv())
	req := autotranslation.CompletionRequest{
		Messages:         []autotranslation.CompletionMessage{{Role: "system", Message: "Translate"}, {Role: "user", Message: "Hello"}},
		JSONOutputFormat: map[string]any{"type": "object"},
	}

	out, err := host.AIChat(context.Background(), c.Id, "hy-mt", req)
	require.NoError(t, err)
	assert.JSONEq(t, `{"translations":["Hola"],"source_language":"en","confidence":0.9}`, out)
	call := f.calls()[0]
	assert.Equal(t, "hy-mt", call.Body["model"])
	assert.NotNil(t, call.Body["response_format"])

	usage, appErr := th.App.GetAIConnectionUsage(th.Context, c)
	require.Nil(t, appErr)
	assert.Equal(t, int64(1), usage.ByPurpose[string(ai.PurposeAutoTranslation)].Requests)

	community := createTestAIConnection(t, th, f, th.BasicTeam.Id, nil)
	_, err = host.AIChat(context.Background(), community.Id, "", req)
	require.Error(t, err, "a community's connection doesn't serve the server's translation")
}

func TestTextToSpeechUsesItsConnection(t *testing.T) {
	th := setupAITest(t)
	f := newFakeLLM(t, "")
	c := createTestAIConnection(t, th, f, model.AIConnectionScopeServer, nil)
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.TextToSpeechSettings.Enable = true
		*cfg.TextToSpeechSettings.Model = "tts-1"
		*cfg.TextToSpeechSettings.Voice = "alloy"
		*cfg.TextToSpeechSettings.URL = ""
		cfg.AISettings.TextToSpeech.ConnectionId = model.NewPointer(c.Id)
	})
	post := th.CreateMessagePost(t, th.BasicChannel, "Read me aloud "+model.NewId())
	ctx := th.Context.WithSession(&model.Session{UserId: th.BasicUser.Id})

	audio, contentType, appErr := th.App.GetPostSpeech(ctx, post)
	require.Nil(t, appErr)
	assert.Equal(t, []byte("mp3"), audio)
	assert.Equal(t, "audio/mpeg", contentType)
	call := f.calls()[0]
	assert.Equal(t, "/v1/audio/speech", call.Path)
	assert.Equal(t, "Bearer "+aiTestKey, call.Authorization)
	assert.Equal(t, "tts-1", call.Body["model"])
	assert.Equal(t, "alloy", call.Body["voice"])
}
