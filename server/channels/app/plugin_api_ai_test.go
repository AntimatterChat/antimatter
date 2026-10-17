// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/ai"
)

// An app calls the AI service for a person it names, and the server holds the call to that person's access.
func TestPluginAPIAI(t *testing.T) {
	th := setupAITest(t)
	f := newFakeLLM(t, "Hello from the model")
	community := createTestAIConnection(t, th, f, th.BasicTeam.Id, nil)
	api := th.SetupPluginAPI()
	request := &ai.ChatRequest{ConnectionID: community.Id, Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hi"}}}

	resp, appErr := api.AIChat(th.BasicUser.Id, th.BasicChannel.Id, "", request)
	require.Nil(t, appErr)
	assert.Equal(t, "Hello from the model", resp.Message.Content)

	_, appErr = api.AIChat("", th.BasicChannel.Id, "", request)
	require.NotNil(t, appErr, "no call without a person")

	outsider := th.CreateUser(t)
	_, appErr = api.AIChat(outsider.Id, th.BasicChannel.Id, "", request)
	require.NotNil(t, appErr, "a person outside the community can't use its connection through an app")

	stream, appErr := api.AIChatStream(th.BasicUser.Id, th.BasicChannel.Id, "", request)
	require.Nil(t, appErr)
	streamed, err := stream.ReadAll()
	require.NoError(t, err)
	assert.Equal(t, "Hello from the model", streamed.Message.Content)

	infos, appErr := api.AIConnectionsForUser(th.BasicUser.Id, th.BasicChannel.Id, "")
	require.Nil(t, appErr)
	found := false
	for _, info := range infos {
		found = found || info.Id == community.Id
	}
	assert.True(t, found)

	usage, appErr := th.App.GetAIConnectionUsage(th.Context, community)
	require.Nil(t, appErr)
	assert.Contains(t, usage.ByPurpose, "plugin:"+api.id, "usage is counted against the app")

	_, appErr = api.AIChat(th.BasicUser.Id, "", "", nil)
	require.NotNil(t, appErr, "no request")
}
