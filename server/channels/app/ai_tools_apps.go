// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// The apps' tools (Antimatter): an app declares its tools with the plugin API's RegisterAITools, named
// "<app id>.<name>", and runs them in its ExecuteAITool hook for the person the server verified. Each node keeps what
// the apps running on it declared; an app's tools count only while that version of it is running.

const aiAppToolTimeout = 2 * time.Minute

type aiAppToolsState struct {
	mu       sync.RWMutex
	byPlugin map[string]*aiAppTools
}

type aiAppTools struct {
	version string
	tools   []*model.AITool
}

// aiReservedNamespace tells whether an app ID would name its tools like the server's own or an MCP server's.
func aiReservedNamespace(pluginID string) bool {
	return pluginID == model.AIToolBuiltInNamespace || strings.HasPrefix(pluginID, model.AIToolMCPNamespacePrefix)
}

// RegisterAIAppTools keeps the tools an app declares, replacing those it declared before.
func (a *App) RegisterAIAppTools(pluginID, version string, definitions []*model.AIToolDefinition) *model.AppError {
	const where = "RegisterAIAppTools"
	if pluginID == "" || aiReservedNamespace(pluginID) {
		return model.NewAppError(where, "app.ai.tools.app_namespace.app_error", nil, "plugin_id="+pluginID, http.StatusBadRequest)
	}
	if len(definitions) > model.AIToolMaxPerSource {
		return model.NewAppError(where, "app.ai.tools.too_many.app_error", map[string]any{"Max": model.AIToolMaxPerSource}, "", http.StatusBadRequest)
	}
	tools := make([]*model.AITool, 0, len(definitions))
	seen := map[string]bool{}
	for _, d := range definitions {
		if appErr := d.IsValid(); appErr != nil {
			return appErr
		}
		if seen[d.Name] {
			return model.NewAppError(where, "model.ai_tool.definition.app_error", nil, "twice: "+d.Name, http.StatusBadRequest)
		}
		seen[d.Name] = true
		name := model.AIToolFullName(pluginID, d.Name)
		if len(name) > model.AIAgentToolNameMaxLength || !model.IsValidAIAgentToolName(name) {
			return model.NewAppError(where, "model.ai_tool.definition.app_error", nil, "name="+name, http.StatusBadRequest)
		}
		tools = append(tools, &model.AITool{
			Name:        name,
			Namespace:   pluginID,
			LocalName:   d.Name,
			Source:      model.AIToolSourceApp,
			SourceId:    pluginID,
			Title:       d.Title,
			Description: d.Description,
			InputSchema: d.InputSchema,
			Kind:        d.Kind,
			Permissions: slices.Clone(d.Permissions),
		})
	}
	state := &a.Srv().ai.appTools
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.byPlugin == nil {
		state.byPlugin = map[string]*aiAppTools{}
	}
	state.byPlugin[pluginID] = &aiAppTools{version: version, tools: tools}
	return nil
}

// aiAppToolSource is an app with tools, running.
type aiAppToolSource struct {
	pluginID string
	name     string
	tools    []*model.AITool
}

// aiRunningAppTools gives the tools of the apps running now, in the version that declared them.
func (a *App) aiRunningAppTools() []*aiAppToolSource {
	env := a.GetPluginsEnvironment()
	if env == nil {
		return nil
	}
	state := &a.Srv().ai.appTools
	state.mu.RLock()
	defer state.mu.RUnlock()
	var out []*aiAppToolSource
	for _, info := range env.Active() {
		if info.Manifest == nil {
			continue
		}
		registered := state.byPlugin[info.Manifest.Id]
		if registered == nil || registered.version != info.Manifest.Version || len(registered.tools) == 0 {
			continue
		}
		name := info.Manifest.Name
		if name == "" {
			name = info.Manifest.Id
		}
		out = append(out, &aiAppToolSource{pluginID: info.Manifest.Id, name: name, tools: registered.tools})
	}
	slices.SortFunc(out, func(x, y *aiAppToolSource) int {
		return strings.Compare(strings.ToLower(x.name), strings.ToLower(y.name))
	})
	return out
}

// runAIAppTool runs an app's tool through its ExecuteAITool hook, for the person in the call.
func (a *App) runAIAppTool(rctx request.CTX, tool *model.AITool, call *model.AIToolExecution) (string, error) {
	env := a.GetPluginsEnvironment()
	if env == nil || !env.IsActive(tool.SourceId) {
		return "", errors.New("the app isn't running")
	}
	hooks, err := env.HooksForPlugin(tool.SourceId)
	if err != nil {
		return "", errors.New("the app isn't running")
	}
	done := make(chan *model.AIToolResult, 1)
	pc := pluginContext(rctx)
	a.Srv().Go(func() {
		done <- hooks.ExecuteAITool(pc, call)
	})
	ctx, cancel := context.WithTimeout(rctx.Context(), aiAppToolTimeout)
	defer cancel()
	select {
	case result := <-done:
		if result == nil {
			return "", errors.New("the app gave no result")
		}
		if result.Error != "" {
			return "", errors.New(result.Error)
		}
		return result.Text, nil
	case <-ctx.Done():
		return "", errors.New("the app took too long")
	}
}
