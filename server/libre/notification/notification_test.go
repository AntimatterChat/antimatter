// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package notification

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest/mocks"
	"github.com/mattermost/mattermost/server/v8/channels/testlib"
	"github.com/mattermost/mattermost/server/v8/config"
)

type fixture struct {
	app      *app.App
	users    *mocks.UserStore
	posts    *mocks.PostStore
	channels *mocks.ChannelStore
}

func setup(t *testing.T) *fixture {
	t.Helper()
	setupStore := testlib.GetMockStoreForSetupFunctions()
	preferences := &mocks.PreferenceStore{}
	preferences.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("not found"))
	setupStore.On("Preference").Return(preferences)

	configStore := config.NewTestMemoryStore()
	cfg := configStore.Get()
	dir := t.TempDir()
	*cfg.FileSettings.Directory = filepath.Join(dir, "data")
	*cfg.PluginSettings.Directory = filepath.Join(dir, "plugins")
	*cfg.PluginSettings.ClientDirectory = filepath.Join(dir, "webapp")
	*cfg.PluginSettings.Enable = false
	*cfg.AnnouncementSettings.AdminNoticesEnabled = false
	*cfg.AnnouncementSettings.UserNoticesEnabled = false
	*cfg.LogSettings.EnableConsole = false
	*cfg.ServiceSettings.CollapsedThreads = model.CollapsedThreadsDisabled
	*cfg.PrivacySettings.ShowFullName = false
	*cfg.EmailSettings.PushNotificationContents = model.IdLoadedNotification
	_, _, err := configStore.Set(cfg)
	require.NoError(t, err)

	s, err := app.NewServer(app.ConfigStore(configStore), app.StoreOverride(setupStore), app.SkipPostInitialization())
	if err != nil {
		t.Skipf("cannot create a server without a database: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	users := setupStore.User().(*mocks.UserStore)
	posts := setupStore.Post().(*mocks.PostStore)
	channels := setupStore.Channel().(*mocks.ChannelStore)

	return &fixture{app: app.New(app.ServerConnector(s.Channels())), users: users, posts: posts, channels: channels}
}

func TestGetNotificationMessage(t *testing.T) {
	f := setup(t)
	rctx := request.TestContext(t)

	receiver := &model.User{Id: model.NewId(), Username: "receiver", Locale: "en"}
	sender := &model.User{Id: model.NewId(), Username: "sender", Locale: "en"}
	channel := &model.Channel{Id: model.NewId(), TeamId: model.NewId(), Type: model.ChannelTypeOpen, DisplayName: "Town Square"}
	post := &model.Post{Id: model.NewId(), ChannelId: channel.Id, UserId: sender.Id, Message: "hello **world**"}

	f.posts.On("GetSingle", mock.Anything, post.Id, false).Return(post, nil)
	f.posts.On("GetSingle", mock.Anything, mock.Anything, false).Return(nil, store.NewErrNotFound("Post", "x"))
	f.channels.On("Get", channel.Id, true).Return(channel, nil)
	f.users.On("Get", mock.Anything, receiver.Id).Return(receiver, nil)
	f.users.On("Get", mock.Anything, sender.Id).Return(sender, nil)
	f.users.On("GetUnreadCount", receiver.Id, false).Return(int64(3), nil)

	n := &Notification{app: f.app}
	ack := &model.PushNotificationAck{Id: "ack-id", PostId: post.Id, ClientPlatform: "ios", NotificationType: model.PushTypeMessage, IsIdLoaded: true}
	msg, appErr := n.GetNotificationMessage(rctx, ack, receiver.Id)
	require.Nil(t, appErr)

	assert.Equal(t, "@sender: hello world", msg.Message)
	assert.Equal(t, "Town Square", msg.ChannelName)
	assert.Equal(t, "@sender", msg.SenderName)
	assert.Equal(t, post.Id, msg.PostId)
	assert.Equal(t, channel.Id, msg.ChannelId)
	assert.Equal(t, channel.TeamId, msg.TeamId)
	assert.Equal(t, 3, msg.Badge)
	assert.Equal(t, "ack-id", msg.AckId)
	assert.Equal(t, "ios", msg.Platform)
	assert.False(t, msg.IsIdLoaded)

	_, appErr = n.GetNotificationMessage(rctx, &model.PushNotificationAck{PostId: model.NewId()}, receiver.Id)
	require.NotNil(t, appErr)
	_, appErr = n.GetNotificationMessage(rctx, &model.PushNotificationAck{}, receiver.Id)
	require.NotNil(t, appErr)
}
