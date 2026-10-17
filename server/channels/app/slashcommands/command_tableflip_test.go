// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

func TestTableflipProviderDoCommand(t *testing.T) {
	th := setup(t)

	p := TableflipProvider{}

	t.Run("no message", func(t *testing.T) {
		resp := p.DoCommand(th.App, th.Context, &model.CommandArgs{}, "")
		assert.Equal(t, model.CommandResponseTypeInChannel, resp.ResponseType)
		assert.Equal(t, "(╯°□°)╯︵ ┻━┻", resp.Text)
	})

	t.Run("with a message", func(t *testing.T) {
		resp := p.DoCommand(th.App, th.Context, &model.CommandArgs{}, "the build is red again")
		assert.Equal(t, model.CommandResponseTypeInChannel, resp.ResponseType)
		assert.Equal(t, "the build is red again (╯°□°)╯︵ ┻━┻", resp.Text)
	})
}

func TestUnflipProviderDoCommand(t *testing.T) {
	th := setup(t)

	p := UnflipProvider{}

	t.Run("no message", func(t *testing.T) {
		resp := p.DoCommand(th.App, th.Context, &model.CommandArgs{}, "")
		assert.Equal(t, model.CommandResponseTypeInChannel, resp.ResponseType)
		assert.Equal(t, "┬─┬ノ( º _ ºノ)", resp.Text)
	})

	t.Run("with a message", func(t *testing.T) {
		resp := p.DoCommand(th.App, th.Context, &model.CommandArgs{}, "fixed it")
		assert.Equal(t, model.CommandResponseTypeInChannel, resp.ResponseType)
		assert.Equal(t, "fixed it ┬─┬ノ( º _ ºノ)", resp.Text)
	})
}

func TestTableflipCommandsAreRegistered(t *testing.T) {
	th := setup(t)

	for _, trigger := range []string{CmdTableflip, CmdUnflip} {
		provider := app.GetCommandProvider(trigger)
		require.NotNil(t, provider, "/%s should be a built-in command", trigger)

		cmd := provider.GetCommand(th.App, i18n.IdentityTfunc())
		assert.Equal(t, trigger, cmd.Trigger)
		assert.True(t, cmd.AutoComplete, "/%s should show in the autocomplete", trigger)
		assert.Equal(t, "api.command_"+trigger+".desc", cmd.AutoCompleteDesc)
		assert.Equal(t, "api.command_"+trigger+".hint", cmd.AutoCompleteHint)
	}
}
