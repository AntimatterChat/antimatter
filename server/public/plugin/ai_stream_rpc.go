// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/mattermost/mattermost/server/public/ai"
	"github.com/mattermost/mattermost/server/public/model"
)

// AIChatStream crosses the plugin RPC boundary by hand (it is excluded from the generator, like PluginHTTP): the
// plugin opens a MuxBroker stream, the server dials it once the call passed its checks and writes the events to it
// as JSON lines; closing the stream on the plugin's side closes the connection, which stops the call.

type Z_AIChatStreamArgs struct {
	EventStream  uint32
	ActingUserID string
	ChannelID    string
	TeamID       string
	Request      *ai.ChatRequest
}

type Z_AIChatStreamReturns struct {
	A bool
	B *model.AppError
}

type acceptedConn struct {
	conn net.Conn
	err  error
}

func (g *apiRPCClient) AIChatStream(actingUserID, channelID, teamID string, request *ai.ChatRequest) (*ai.Stream, *model.AppError) {
	streamID := g.muxBroker.NextId()
	accepted := make(chan acceptedConn, 1)
	go func() {
		conn, err := g.muxBroker.Accept(streamID)
		accepted <- acceptedConn{conn: conn, err: err}
	}()

	args := &Z_AIChatStreamArgs{EventStream: streamID, ActingUserID: actingUserID, ChannelID: channelID, TeamID: teamID, Request: request}
	returns := &Z_AIChatStreamReturns{}
	if err := g.client.Call("Plugin.AIChatStream", args, returns); err != nil {
		log.Printf("RPC call to AIChatStream API failed: %s", err.Error())
		return nil, model.NewAppError("AIChatStream", "plugin.api.ai_chat_stream.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	if returns.B != nil {
		// The server refused the call before dialling: the Accept above times out by itself.
		return nil, returns.B
	}
	result := <-accepted
	if result.err != nil {
		return nil, model.NewAppError("AIChatStream", "plugin.api.ai_chat_stream.app_error", nil, result.err.Error(), http.StatusInternalServerError)
	}
	return readAIStream(result.conn), nil
}

// readAIStream turns the JSON lines of a connection into a stream; closing the stream closes the connection.
func readAIStream(conn net.Conn) *ai.Stream {
	events := make(chan ai.StreamEvent, 16)
	done := make(chan struct{})
	var closeOnce sync.Once
	stop := func() {
		closeOnce.Do(func() {
			close(done)
			conn.Close()
		})
	}
	go func() {
		defer close(events)
		defer stop()
		decoder := json.NewDecoder(conn)
		for {
			var event ai.StreamEvent
			if err := decoder.Decode(&event); err != nil {
				select {
				case <-done:
				case events <- ai.StreamEvent{Type: ai.EventError, Error: fmt.Sprintf("the AI stream broke: %v", err)}:
				}
				return
			}
			select {
			case events <- event:
			case <-done:
				return
			}
			if event.Type == ai.EventEnd || event.Type == ai.EventError {
				return
			}
		}
	}()
	return ai.NewStream(events, stop)
}

func (s *apiRPCServer) AIChatStream(args *Z_AIChatStreamArgs, returns *Z_AIChatStreamReturns) error {
	hook, ok := s.impl.(interface {
		AIChatStream(actingUserID, channelID, teamID string, request *ai.ChatRequest) (*ai.Stream, *model.AppError)
	})
	if !ok {
		return encodableError(fmt.Errorf("API AIChatStream called but not implemented"))
	}
	stream, appErr := hook.AIChatStream(args.ActingUserID, args.ChannelID, args.TeamID, args.Request)
	if appErr != nil {
		returns.B = appErr
		return nil
	}
	conn, err := s.muxBroker.Dial(args.EventStream)
	if err != nil {
		stream.Close()
		return encodableError(fmt.Errorf("can't connect to the plugin's AI stream: %w", err))
	}
	returns.A = true
	go writeAIStream(conn, stream)
	return nil
}

// writeAIStream sends a stream's events as JSON lines; when the plugin goes away, the stream is closed, which stops
// the call.
func writeAIStream(conn net.Conn, stream *ai.Stream) {
	defer conn.Close()
	defer stream.Close()
	// The plugin never writes: its end closing ends this read, even while the model is still thinking.
	go func() {
		_, _ = io.Copy(io.Discard, conn)
		stream.Close()
	}()
	encoder := json.NewEncoder(conn)
	for event := range stream.Events {
		if err := encoder.Encode(event); err != nil {
			return
		}
	}
}
