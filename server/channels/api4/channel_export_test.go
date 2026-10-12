// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// fakeChannelExporter stands for libre/messageexport's exporter (tested there): it records what it's asked for.
type fakeChannelExporter struct {
	params   []app.ChannelExportParams
	tooLarge bool
}

func (f *fakeChannelExporter) export(_ request.CTX, _ store.Store, params app.ChannelExportParams, w io.Writer) (int, error) {
	f.params = append(f.params, params)
	if f.tooLarge {
		return 0, app.ErrChannelExportTooLarge
	}
	_, err := fmt.Fprintf(w, "Post Id\n%s\n", params.ChannelID)
	return 1, err
}

func (f *fakeChannelExporter) last() app.ChannelExportParams {
	return f.params[len(f.params)-1]
}

func registerFakeChannelExporter(t *testing.T) *fakeChannelExporter {
	f := &fakeChannelExporter{}
	app.RegisterChannelExporter(f.export)
	t.Cleanup(func() { app.RegisterChannelExporter(nil) })
	return f
}

func TestExportChannel(t *testing.T) {
	th := Setup(t).InitBasic(t)
	exporter := registerFakeChannelExporter(t)
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.PrivacySettings.ShowEmailAddress = false
	})
	ctx := context.Background()

	t.Run("a member downloads the channel's export", func(t *testing.T) {
		data, resp, err := th.Client.ExportChannel(ctx, th.BasicChannel.Id, 1000, 2000)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		assert.Equal(t, "Post Id\n"+th.BasicChannel.Id+"\n", string(data))
		assert.Equal(t, "text/csv; charset=utf-8", resp.Header.Get("Content-Type"))
		assert.Regexp(t, `^attachment; filename="`+th.BasicChannel.Name+`-\d{4}-\d{2}-\d{2}\.csv"$`, resp.Header.Get("Content-Disposition"))

		assert.Equal(t, app.ChannelExportParams{
			ChannelID:     th.BasicChannel.Id,
			Since:         1000,
			Until:         2000,
			MaxPosts:      model.ChannelExportMaxPosts,
			IncludeEmails: false,
		}, exporter.last())
	})

	t.Run("system admins get the emails", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.ExportChannel(ctx, th.BasicChannel.Id, 0, 0)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		assert.True(t, exporter.last().IncludeEmails)
	})

	t.Run("a direct message conversation", func(t *testing.T) {
		dm := th.CreateDmChannel(t, th.BasicUser2)
		_, resp, err := th.Client.ExportChannel(ctx, dm.Id, 0, 0)
		require.NoError(t, err)
		assert.Regexp(t, `filename="direct-message-`, resp.Header.Get("Content-Disposition"))
	})

	t.Run("not a channel the user can read", func(t *testing.T) {
		private := th.CreatePrivateChannel(t)
		client := th.CreateClient()
		th.LoginBasic2WithClient(t, client)
		calls := len(exporter.params)
		_, resp, err := client.ExportChannel(ctx, private.Id, 0, 0)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		assert.Len(t, exporter.params, calls, "nothing is exported")
	})

	t.Run("a bad request", func(t *testing.T) {
		_, resp, err := th.Client.ExportChannel(ctx, th.BasicChannel.Id, 2000, 1000)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		for _, query := range []string{"since=yesterday", "format=pdf"} {
			r, err := th.Client.DoAPIGet(ctx, "/channels/"+th.BasicChannel.Id+"/export?"+query, "")
			require.Error(t, err, query)
			assert.Equal(t, http.StatusBadRequest, r.StatusCode, query)
		}
	})

	t.Run("too many messages", func(t *testing.T) {
		exporter.tooLarge = true
		defer func() { exporter.tooLarge = false }()
		_, resp, err := th.Client.ExportChannel(ctx, th.BasicChannel.Id, 0, 0)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		CheckErrorID(t, err, "app.channel_export.too_large.app_error")
	})

	t.Run("restricted to admins", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.TeamSettings.RestrictChannelExportToAdmins = true
		})
		defer th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.TeamSettings.RestrictChannelExportToAdmins = false
		})

		client := th.CreateClient()
		th.LoginBasic2WithClient(t, client)
		_, resp, err := client.ExportChannel(ctx, th.BasicChannel.Id, 0, 0)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "app.channel_export.admins_only.app_error")

		// The channel's admins, the team's and the system's can.
		_, appErr := th.App.UpdateChannelMemberSchemeRoles(th.Context, th.BasicChannel.Id, th.BasicUser.Id, false, true, true)
		require.Nil(t, appErr)
		_, resp, err = th.Client.ExportChannel(ctx, th.BasicChannel.Id, 0, 0)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		_, appErr = th.App.UpdateTeamMemberSchemeRoles(th.Context, th.BasicTeam.Id, th.BasicUser2.Id, false, true, true)
		require.Nil(t, appErr)
		_, resp, err = client.ExportChannel(ctx, th.BasicChannel.Id, 0, 0)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		_, resp, err = th.SystemAdminClient.ExportChannel(ctx, th.BasicChannel.Id, 0, 0)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})
}
