// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// Security regression: the sender of a burn-on-read message picks how long it lives once read (props.read_duration,
// in milliseconds), but a client can never make it live longer than the administrator's maximum, whatever it sends.
func TestBurnOnReadDurationFromClient(t *testing.T) {
	th := SetupEnterprise(t).InitBasic(t)
	th.App.UpdateConfig(func(cfg *model.Config) {
		cfg.FeatureFlags.BurnOnRead = true
		cfg.ServiceSettings.EnableBurnOnRead = new(true)
		cfg.ServiceSettings.BurnOnReadDurationSeconds = new(600)
		cfg.ServiceSettings.BurnOnReadMaximumDurationSeconds = new(3600)
	})
	th.LinkUserToTeam(t, th.SystemAdminUser, th.BasicTeam)
	th.AddUserToChannel(t, th.SystemAdminUser, th.BasicChannel)

	send := func(t *testing.T, readDuration any) (*model.Post, *model.Response, error) {
		t.Helper()
		post := &model.Post{
			ChannelId: th.BasicChannel.Id,
			Message:   "burn on read message",
			Type:      model.PostTypeBurnOnRead,
		}
		if readDuration != nil {
			post.AddProp(model.PostPropsReadDurationSeconds, readDuration)
		}
		return th.Client.CreatePost(context.Background(), post)
	}
	durationOf := func(t *testing.T, post *model.Post) float64 {
		t.Helper()
		ms, ok := post.GetProp(model.PostPropsReadDurationSeconds).(float64)
		require.True(t, ok, "read_duration is a number")
		return ms
	}

	t.Run("the client config tells the maximum", func(t *testing.T) {
		config, _, err := th.Client.GetClientConfig(context.Background(), "")
		require.NoError(t, err)
		require.Equal(t, "3600", config["BurnOnReadMaximumDurationSeconds"])
		require.Equal(t, "600", config["BurnOnReadDurationSeconds"])
	})

	t.Run("a picked duration within the maximum is kept", func(t *testing.T) {
		created, resp, err := send(t, 15*60*1000)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		require.Equal(t, float64(900_000), durationOf(t, created))
	})

	t.Run("a duration over the maximum is clamped, and the reader's copy burns within the maximum", func(t *testing.T) {
		created, resp, err := send(t, 30*24*60*60*1000)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		require.Equal(t, float64(3_600_000), durationOf(t, created))

		// As the sender's client gets it again.
		fetched, _, err := th.Client.GetPost(context.Background(), created.Id, "")
		require.NoError(t, err)
		require.Equal(t, float64(3_600_000), durationOf(t, fetched))

		revealed, resp, err := th.SystemAdminClient.RevealPost(context.Background(), created.Id)
		after := model.GetMillis()
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotZero(t, revealed.Metadata.ExpireAt)
		require.LessOrEqual(t, revealed.Metadata.ExpireAt, after+3_600_000)
	})

	t.Run("a huge number doesn't wrap around", func(t *testing.T) {
		created, resp, err := send(t, 1e300)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		require.Equal(t, float64(3_600_000), durationOf(t, created))
	})

	t.Run("what isn't a duration is refused", func(t *testing.T) {
		for _, bad := range []any{"forever", 0, -60_000} {
			_, resp, err := send(t, bad)
			require.Error(t, err, "%v", bad)
			CheckBadRequestStatus(t, resp)
			CheckErrorID(t, err, "api.post.fill_in_post_props.burn_on_read.duration.app_error")
		}
	})

	t.Run("none picked: the default", func(t *testing.T) {
		created, resp, err := send(t, nil)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		require.Equal(t, float64(600_000), durationOf(t, created))
	})
}
