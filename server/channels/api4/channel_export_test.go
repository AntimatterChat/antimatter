// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
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
	mut      sync.Mutex
	params   []app.ChannelExportParams
	tooLarge bool

	// When block is set, exports wait for it to be closed, after telling started.
	block   chan struct{}
	started chan struct{}
}

func (f *fakeChannelExporter) export(_ request.CTX, _ store.Store, params app.ChannelExportParams, open func() (io.Writer, error)) (int, error) {
	f.mut.Lock()
	f.params = append(f.params, params)
	tooLarge, block, started := f.tooLarge, f.block, f.started
	f.mut.Unlock()

	if tooLarge {
		return 0, app.ErrChannelExportTooLarge
	}
	if block != nil {
		started <- struct{}{}
		<-block
	}
	w, err := open()
	if err != nil {
		return 0, err
	}
	_, err = fmt.Fprintf(w, "Post Id\n%s\n", params.ChannelID)
	return 1, err
}

func (f *fakeChannelExporter) calls() int {
	f.mut.Lock()
	defer f.mut.Unlock()
	return len(f.params)
}

func (f *fakeChannelExporter) last() app.ChannelExportParams {
	f.mut.Lock()
	defer f.mut.Unlock()
	return f.params[len(f.params)-1]
}

func registerFakeChannelExporter(t *testing.T) *fakeChannelExporter {
	f := &fakeChannelExporter{}
	previous := app.ChannelExportExporter()
	app.RegisterChannelExporter(f.export)
	t.Cleanup(func() { app.RegisterChannelExporter(previous) })
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
		assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
		assert.Regexp(t, `^attachment; filename="`+th.BasicChannel.Name+`-\d{4}-\d{2}-\d{2}\.csv"$`, resp.Header.Get("Content-Disposition"))

		params := exporter.last()
		assert.NotNil(t, params.ConsumePosts, "the plugins' hooks see the posts")
		params.ConsumePosts = nil
		assert.Equal(t, app.ChannelExportParams{
			ChannelID:     th.BasicChannel.Id,
			Since:         1000,
			Until:         2000,
			MaxPosts:      model.ChannelExportMaxPosts,
			IncludeEmails: false,
		}, params)
	})

	t.Run("an open range ends now", func(t *testing.T) {
		before := model.GetMillis()
		_, resp, err := th.Client.ExportChannel(ctx, th.BasicChannel.Id, 0, 0)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		params := exporter.last()
		assert.Zero(t, params.Since)
		assert.GreaterOrEqual(t, params.Until, before)
		assert.LessOrEqual(t, params.Until, model.GetMillis())
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

	t.Run("a public channel the user reads without being a member", func(t *testing.T) {
		public := th.CreatePublicChannel(t)
		isMember, appErr := th.App.CanExportChannel(th.Context, th.BasicUser2.Id, public)
		require.Nil(t, appErr)
		assert.False(t, isMember, "the export is audited as a non member's")

		client := th.CreateClient()
		th.LoginBasic2WithClient(t, client)
		_, resp, err := client.ExportChannel(ctx, public.Id, 0, 0)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	t.Run("not a channel the user can read", func(t *testing.T) {
		private := th.CreatePrivateChannel(t)
		client := th.CreateClient()
		th.LoginBasic2WithClient(t, client)
		calls := exporter.calls()
		_, resp, err := client.ExportChannel(ctx, private.Id, 0, 0)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		assert.Equal(t, calls, exporter.calls(), "nothing is exported")
	})

	t.Run("only a POST exports", func(t *testing.T) {
		// So that an image or a link in a message can't make its readers export.
		calls := exporter.calls()
		r, err := th.Client.DoAPIGet(ctx, "/channels/"+th.BasicChannel.Id+"/export", "")
		require.Error(t, err)
		assert.Contains(t, []int{http.StatusMethodNotAllowed, http.StatusNotFound}, r.StatusCode)
		assert.Equal(t, calls, exporter.calls(), "nothing is exported")
	})

	t.Run("a bad request", func(t *testing.T) {
		_, resp, err := th.Client.ExportChannel(ctx, th.BasicChannel.Id, 2000, 1000)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		for _, query := range []string{"since=yesterday", "format=pdf"} {
			r, err := th.Client.DoAPIPost(ctx, "/channels/"+th.BasicChannel.Id+"/export?"+query, "")
			require.Error(t, err, query)
			assert.Equal(t, http.StatusBadRequest, r.StatusCode, query)
		}
	})

	t.Run("too many messages", func(t *testing.T) {
		exporter.mut.Lock()
		exporter.tooLarge = true
		exporter.mut.Unlock()
		defer func() {
			exporter.mut.Lock()
			exporter.tooLarge = false
			exporter.mut.Unlock()
		}()
		_, resp, err := th.Client.ExportChannel(ctx, th.BasicChannel.Id, 0, 0)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		CheckErrorID(t, err, "app.channel_export.too_large.app_error")
	})

	t.Run("two exports at once at most", func(t *testing.T) {
		block := make(chan struct{})
		started := make(chan struct{})
		exporter.mut.Lock()
		exporter.block, exporter.started = block, started
		exporter.mut.Unlock()
		defer func() {
			exporter.mut.Lock()
			exporter.block, exporter.started = nil, nil
			exporter.mut.Unlock()
		}()

		var wg sync.WaitGroup
		for range 2 {
			wg.Go(func() {
				_, resp, err := th.Client.ExportChannel(ctx, th.BasicChannel.Id, 0, 0)
				assert.NoError(t, err)
				CheckOKStatus(t, resp)
			})
		}
		<-started
		<-started

		_, resp, err := th.SystemAdminClient.ExportChannel(ctx, th.BasicChannel.Id, 0, 0)
		require.Error(t, err)
		assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
		CheckErrorID(t, err, "app.channel_export.busy.app_error")

		close(block)
		wg.Wait()

		// The slots are free again.
		exporter.mut.Lock()
		exporter.block = nil
		exporter.mut.Unlock()
		_, resp, err = th.Client.ExportChannel(ctx, th.BasicChannel.Id, 0, 0)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
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
