// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package plugin

import (
	"net"
	"net/http"
	"net/rpc"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/ai"
	"github.com/mattermost/mattermost/server/public/model"
)

// fakeAIStreamAPI is a server API with only AIChatStream: it streams the events given, or refuses.
type fakeAIStreamAPI struct {
	API
	events  []ai.StreamEvent
	refuse  *model.AppError
	gotUser string
	closed  chan struct{}
	block   bool
}

func (f *fakeAIStreamAPI) AIChatStream(actingUserID, channelID, teamID string, request *ai.ChatRequest) (*ai.Stream, *model.AppError) {
	f.gotUser = actingUserID
	if f.refuse != nil {
		return nil, f.refuse
	}
	ch := make(chan ai.StreamEvent, len(f.events)+1)
	for _, e := range f.events {
		ch <- e
	}
	stop := make(chan struct{})
	go func() {
		if f.block {
			<-stop
		}
		close(ch)
	}()
	return ai.NewStream(ch, func() {
		close(stop)
		if f.closed != nil {
			close(f.closed)
		}
	}), nil
}

func newAIStreamRPC(t *testing.T, api API) *apiRPCClient {
	broker := newTestHooksRPCMuxBroker()
	serverConn, clientConn := net.Pipe()
	server := rpc.NewServer()
	require.NoError(t, server.RegisterName("Plugin", &apiRPCServer{impl: api, muxBroker: broker}))
	go server.ServeConn(serverConn)
	client := rpc.NewClient(clientConn)
	t.Cleanup(func() { client.Close() })
	return &apiRPCClient{client: client, muxBroker: broker}
}

func TestAIChatStreamRPC(t *testing.T) {
	t.Run("events cross to the plugin", func(t *testing.T) {
		api := &fakeAIStreamAPI{events: []ai.StreamEvent{
			{Type: ai.EventText, Text: "Hel"},
			{Type: ai.EventText, Text: "lo"},
			{Type: ai.EventUsage, Usage: &ai.Usage{InputTokens: 3, OutputTokens: 2}},
			{Type: ai.EventEnd, FinishReason: ai.FinishReasonStop},
		}}
		client := newAIStreamRPC(t, api)
		stream, appErr := client.AIChatStream("user-id", "", "", &ai.ChatRequest{ConnectionID: "c"})
		require.Nil(t, appErr)
		resp, err := stream.ReadAll()
		require.NoError(t, err)
		assert.Equal(t, "Hello", resp.Message.Content)
		assert.Equal(t, int64(5), resp.Usage.Total())
		assert.Equal(t, "user-id", api.gotUser)
	})

	t.Run("a refusal comes back as the server's error", func(t *testing.T) {
		client := newAIStreamRPC(t, &fakeAIStreamAPI{refuse: model.NewAppError("AIChatStream", "app.ai.call.audience.app_error", nil, "", http.StatusForbidden)})
		_, appErr := client.AIChatStream("user-id", "", "", &ai.ChatRequest{})
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusForbidden, appErr.StatusCode)
	})

	t.Run("closing on the plugin's side stops the call", func(t *testing.T) {
		api := &fakeAIStreamAPI{events: []ai.StreamEvent{{Type: ai.EventText, Text: "first"}}, block: true, closed: make(chan struct{})}
		client := newAIStreamRPC(t, api)
		stream, appErr := client.AIChatStream("user-id", "", "", &ai.ChatRequest{})
		require.Nil(t, appErr)
		first := <-stream.Events
		assert.Equal(t, "first", first.Text)
		stream.Close()
		select {
		case <-api.closed:
		case <-time.After(5 * time.Second):
			require.FailNow(t, "the server's stream wasn't closed")
		}
	})
}
