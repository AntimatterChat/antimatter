// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

func TestExportCommand(t *testing.T) {
	th := setup(t).initBasic(t)

	// The exporter (libre/messageexport's, tested there) is faked: it records what it's asked for.
	var asked []app.ChannelExportParams
	tooLarge := false
	app.RegisterChannelExporter(func(_ request.CTX, _ store.Store, params app.ChannelExportParams, w io.Writer) (int, error) {
		asked = append(asked, params)
		if tooLarge {
			return 0, app.ErrChannelExportTooLarge
		}
		_, err := fmt.Fprintf(w, "Post Id\n%s\n", params.ChannelID)
		return 1, err
	})
	t.Cleanup(func() { app.RegisterChannelExporter(nil) })

	run := func(userID, channelID, message string) string {
		resp := (&ExportProvider{}).DoCommand(th.App, th.Context, &model.CommandArgs{
			T:         i18n.IdentityTfunc(),
			UserId:    userID,
			ChannelId: channelID,
		}, message)
		assert.Equal(t, model.CommandResponseTypeEphemeral, resp.ResponseType)
		return resp.Text
	}

	t.Run("sends the export in a direct message from the system bot", func(t *testing.T) {
		assert.Equal(t, "api.command_export.sent", run(th.BasicUser.Id, th.BasicChannel.Id, ""))
		assert.Equal(t, int64(0), asked[len(asked)-1].Since)

		bot, appErr := th.App.GetSystemBot(th.Context)
		require.Nil(t, appErr)
		dm, appErr := th.App.GetOrCreateDirectChannel(th.Context, th.BasicUser.Id, bot.UserId)
		require.Nil(t, appErr)
		posts, appErr := th.App.GetPostsPage(th.Context, model.GetPostsOptions{ChannelId: dm.Id, PerPage: 10})
		require.Nil(t, appErr)
		require.Len(t, posts.Order, 1)
		post := posts.Posts[posts.Order[0]]
		assert.Equal(t, bot.UserId, post.UserId)
		assert.Contains(t, post.Message, "~"+th.BasicChannel.Name)
		require.Len(t, post.FileIds, 1)

		info, appErr := th.App.GetFileInfo(th.Context, post.FileIds[0])
		require.Nil(t, appErr)
		assert.True(t, strings.HasPrefix(info.Name, th.BasicChannel.Name+"-"), info.Name)
		assert.Equal(t, "csv", info.Extension)
		data, appErr := th.App.ReadFile(info.Path)
		require.Nil(t, appErr)
		assert.Equal(t, "Post Id\n"+th.BasicChannel.Id+"\n", string(data))
	})

	t.Run("the last days", func(t *testing.T) {
		assert.Equal(t, "api.command_export.sent", run(th.BasicUser.Id, th.BasicChannel.Id, " 30 "))
		since := asked[len(asked)-1].Since
		assert.InDelta(t, model.GetMillis()-(30*24*time.Hour).Milliseconds(), since, float64(time.Minute.Milliseconds()))

		calls := len(asked)
		assert.Equal(t, "api.command_export.days.app_error", run(th.BasicUser.Id, th.BasicChannel.Id, "lots"))
		assert.Equal(t, "api.command_export.days.app_error", run(th.BasicUser.Id, th.BasicChannel.Id, "-2"))
		assert.Len(t, asked, calls)
	})

	t.Run("too many messages", func(t *testing.T) {
		tooLarge = true
		defer func() { tooLarge = false }()
		assert.Equal(t, "api.command_export.too_large.app_error", run(th.BasicUser.Id, th.BasicChannel.Id, ""))
	})

	t.Run("not a channel the user can read", func(t *testing.T) {
		private := th.createPrivateChannel(t, th.BasicTeam)
		calls := len(asked)
		assert.Equal(t, "app.channel_export.permission.app_error", run(th.BasicUser2.Id, private.Id, ""))
		assert.Len(t, asked, calls)
	})
}
