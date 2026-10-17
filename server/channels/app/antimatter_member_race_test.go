// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// racingChannelStore adds the member itself right before the app saves them, once: as the background fill of a
// community with everyone in every channel does when it adds someone between addUserToChannel's membership check and
// its save.
type racingChannelStore struct {
	store.ChannelStore
	once  sync.Once
	raced bool
}

func (s *racingChannelStore) SaveMember(rctx request.CTX, member *model.ChannelMember) (*model.ChannelMember, error) {
	s.once.Do(func() {
		racer := *member
		racer.NotifyProps = model.GetDefaultChannelNotifyProps()
		if _, err := s.ChannelStore.SaveMember(rctx, &racer); err == nil {
			s.raced = true
		}
	})
	return s.ChannelStore.SaveMember(rctx, member)
}

type racingStore struct {
	store.Store
	channels *racingChannelStore
}

func (s *racingStore) Channel() store.ChannelStore {
	return s.channels
}

// raceChannelMembers makes the next member saved be saved by someone else first.
func raceChannelMembers(t *testing.T, th *TestHelper) *racingChannelStore {
	t.Helper()
	original := th.App.Srv().Store()
	racing := &racingChannelStore{ChannelStore: original.Channel()}
	th.App.Srv().SetStore(&racingStore{Store: original, channels: racing})
	t.Cleanup(func() { th.App.Srv().SetStore(original) })
	return racing
}

// Adding someone to a channel while they're being added by someone else (the background fill of a community with
// everyone in every channel) succeeds: they're a member, once, instead of a 500 on the duplicate key.
func TestAddMemberRacedByAnotherAdd(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	member := func(t *testing.T, channel *model.Channel, userID string) {
		t.Helper()
		got, err := th.App.Srv().Store().Channel().GetMember(RequestContextWithMaster(th.Context), channel.Id, userID)
		require.NoError(t, err)
		require.Equal(t, userID, got.UserId)
		count, err := th.App.Srv().Store().Channel().GetMemberCount(channel.Id, false)
		require.NoError(t, err)
		require.EqualValues(t, 2, count, "the creator and them, once")
	}

	t.Run("AddChannelMember, as the API adds people", func(t *testing.T) {
		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, th.BasicTeam)
		channel := th.CreateChannel(t, th.BasicTeam)
		racing := raceChannelMembers(t, th)

		cm, appErr := th.App.AddChannelMember(th.Context, user.Id, channel, ChannelMemberOpts{UserRequestorID: th.BasicUser.Id})
		require.True(t, racing.raced, "the race happened")
		require.Nil(t, appErr)
		require.Equal(t, user.Id, cm.UserId)
		member(t, channel, user.Id)
	})

	t.Run("AddUserToChannel", func(t *testing.T) {
		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, th.BasicTeam)
		channel := th.CreateChannel(t, th.BasicTeam)
		racing := raceChannelMembers(t, th)

		cm, appErr := th.App.AddUserToChannel(th.Context, user, channel, false)
		require.True(t, racing.raced, "the race happened")
		require.Nil(t, appErr)
		require.Equal(t, user.Id, cm.UserId)
		member(t, channel, user.Id)
	})

	t.Run("the community's background fill", func(t *testing.T) {
		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, th.BasicTeam)
		channel := th.CreateChannel(t, th.BasicTeam)
		racing := raceChannelMembers(t, th)

		require.Nil(t, th.App.joinChannelForCommunity(th.Context, user, channel, false))
		require.True(t, racing.raced, "the race happened")
		member(t, channel, user.Id)
	})

	t.Run("other failures to save still fail", func(t *testing.T) {
		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, th.BasicTeam)
		channel := th.CreateChannel(t, th.BasicTeam)
		original := th.App.Srv().Store()
		th.App.Srv().SetStore(&racingStore{Store: original, channels: &racingChannelStore{ChannelStore: &failingSaveMemberStore{ChannelStore: original.Channel()}}})
		t.Cleanup(func() { th.App.Srv().SetStore(original) })

		_, appErr := th.App.AddUserToChannel(th.Context, user, channel, false)
		require.NotNil(t, appErr)
		require.Equal(t, "api.channel.add_user.to.channel.failed.app_error", appErr.Id)
		require.Equal(t, http.StatusInternalServerError, appErr.StatusCode)
	})
}

type failingSaveMemberStore struct {
	store.ChannelStore
}

func (s *failingSaveMemberStore) SaveMember(request.CTX, *model.ChannelMember) (*model.ChannelMember, error) {
	return nil, store.NewErrInvalidInput("ChannelMember", "UserId", "")
}
