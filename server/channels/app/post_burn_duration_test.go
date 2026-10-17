// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// readDurationOf is a post's read_duration prop: an int64 as the server sets it, a float64 once read back from JSON.
func readDurationOf(t *testing.T, post *model.Post) int64 {
	t.Helper()
	switch v := post.GetProp(model.PostPropsReadDurationSeconds).(type) {
	case int64:
		return v
	case float64:
		return int64(v)
	}
	require.FailNow(t, "the post has no read_duration")
	return 0
}

// The sender of a burn-on-read message picks how long it lives once read (its read_duration prop, in
// milliseconds): the server keeps it within the administrator's maximum, and uses the default when none was picked.
func TestBurnOnReadSenderDuration(t *testing.T) {
	th := Setup(t).InitBasic(t)
	th.App.UpdateConfig(func(cfg *model.Config) {
		cfg.FeatureFlags.BurnOnRead = true
		cfg.ServiceSettings.EnableBurnOnRead = new(true)
		cfg.ServiceSettings.BurnOnReadDurationSeconds = new(600)
		cfg.ServiceSettings.BurnOnReadMaximumDurationSeconds = new(3600)
	})

	reader := th.CreateUser(t)
	th.LinkUserToTeam(t, reader, th.BasicTeam)
	th.AddUserToChannel(t, reader, th.BasicChannel)

	send := func(t *testing.T, readDuration any) (*model.Post, *model.AppError) {
		t.Helper()
		post := &model.Post{
			ChannelId: th.BasicChannel.Id,
			UserId:    th.BasicUser.Id,
			Message:   "burn on read message",
			Type:      model.PostTypeBurnOnRead,
		}
		if readDuration != nil {
			post.AddProp(model.PostPropsReadDurationSeconds, readDuration)
		}
		created, _, appErr := th.App.CreatePost(th.Context, post, th.BasicChannel, model.CreatePostFlags{})
		return created, appErr
	}

	// reveal opens the message as the reader, and returns the time the reader's copy burns, with the times the
	// reveal started and ended.
	reveal := func(t *testing.T, post *model.Post) (expireAt, before, after int64) {
		t.Helper()
		before = model.GetMillis()
		revealed, appErr := th.App.RevealPost(th.Context, post, reader.Id, "")
		after = model.GetMillis()
		require.Nil(t, appErr)
		require.NotNil(t, revealed)
		receipt, err := th.App.Srv().Store().ReadReceipt().Get(th.Context, post.Id, reader.Id)
		require.NoError(t, err)
		return receipt.ExpireAt, before, after
	}

	t.Run("none picked: the default duration", func(t *testing.T) {
		post, appErr := send(t, nil)
		require.Nil(t, appErr)
		require.Equal(t, int64(600_000), readDurationOf(t, post))
	})

	t.Run("a picked duration within the maximum is kept", func(t *testing.T) {
		post, appErr := send(t, float64(15*60*1000))
		require.Nil(t, appErr)
		require.Equal(t, int64(900_000), readDurationOf(t, post))

		stored, err := th.App.Srv().Store().Post().GetSingle(th.Context, post.Id, false)
		require.NoError(t, err)
		require.Equal(t, int64(900_000), readDurationOf(t, stored))
	})

	t.Run("a duration over the maximum is clamped to it", func(t *testing.T) {
		post, appErr := send(t, float64(30*24*60*60*1000))
		require.Nil(t, appErr)
		require.Equal(t, int64(3_600_000), readDurationOf(t, post))
	})

	t.Run("a duration under a minute becomes a minute", func(t *testing.T) {
		post, appErr := send(t, float64(1000))
		require.Nil(t, appErr)
		require.Equal(t, int64(60_000), readDurationOf(t, post))
	})

	t.Run("what isn't a duration is refused", func(t *testing.T) {
		for _, bad := range []any{"forever", float64(0), float64(-60_000), true, map[string]any{"ms": 1}} {
			post, appErr := send(t, bad)
			require.Nil(t, post, "%v", bad)
			require.NotNil(t, appErr, "%v", bad)
			require.Equal(t, "api.post.fill_in_post_props.burn_on_read.duration.app_error", appErr.Id)
			require.Equal(t, http.StatusBadRequest, appErr.StatusCode)
		}
	})

	t.Run("the reader's copy burns after the picked duration", func(t *testing.T) {
		post, appErr := send(t, float64(15*60*1000))
		require.Nil(t, appErr)

		expireAt, before, after := reveal(t, post)
		require.GreaterOrEqual(t, expireAt, before+900_000)
		require.LessOrEqual(t, expireAt, after+900_000)
	})

	t.Run("without a picked duration, it burns after the default", func(t *testing.T) {
		post, appErr := send(t, nil)
		require.Nil(t, appErr)

		expireAt, before, after := reveal(t, post)
		require.GreaterOrEqual(t, expireAt, before+600_000)
		require.LessOrEqual(t, expireAt, after+600_000)
	})

	// Security: what decides when a message burns is checked again when it's read, so that a duration raised after
	// sending (or a maximum lowered since) never lets it outlive the administrator's maximum.
	t.Run("reading clamps a duration raised after sending to the maximum", func(t *testing.T) {
		post, appErr := send(t, float64(15*60*1000))
		require.Nil(t, appErr)
		post.AddProp(model.PostPropsReadDurationSeconds, float64(30*24*60*60*1000))

		expireAt, _, after := reveal(t, post)
		require.LessOrEqual(t, expireAt, after+3_600_000)
	})

	t.Run("reading applies the maximum as it is now", func(t *testing.T) {
		post, appErr := send(t, float64(30*60*1000))
		require.Nil(t, appErr)

		th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.BurnOnReadMaximumDurationSeconds = new(120) })
		defer th.App.UpdateConfig(func(cfg *model.Config) { cfg.ServiceSettings.BurnOnReadMaximumDurationSeconds = new(3600) })

		expireAt, _, after := reveal(t, post)
		require.LessOrEqual(t, expireAt, after+120_000)
	})

	t.Run("a broken duration on a stored message falls back to the default when read", func(t *testing.T) {
		post, appErr := send(t, nil)
		require.Nil(t, appErr)
		post.AddProp(model.PostPropsReadDurationSeconds, "forever")

		expireAt, before, after := reveal(t, post)
		require.GreaterOrEqual(t, expireAt, before+600_000)
		require.LessOrEqual(t, expireAt, after+600_000)
	})
}
