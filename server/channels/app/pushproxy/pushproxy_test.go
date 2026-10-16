// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package pushproxy

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/jobs"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest/mocks"
)

func newTest(t *testing.T) (*PushProxy, *mocks.SystemStore, *time.Time) {
	sys := &mocks.SystemStore{}
	now := time.Now()
	p := New(func() store.SystemStore { return sys }, func() mlog.LoggerIFace { return mlog.CreateConsoleTestLogger(t) }, func() *jobs.JobServer { return nil })
	p.cache = &tokenCache{}
	p.now = func() time.Time { return now }
	return p, sys, &now
}

func TestGetAuthToken(t *testing.T) {
	p, sys, now := newTest(t)

	sys.On("GetByName", model.SystemPushProxyAuthToken).Return(&model.System{Name: model.SystemPushProxyAuthToken, Value: "tok"}, nil).Once()
	assert.Equal(t, "tok", p.GetAuthToken())
	// cached
	assert.Equal(t, "tok", p.GetAuthToken())

	// transient errors keep the cached value
	*now = now.Add(cacheTTL + time.Second)
	sys.On("GetByName", model.SystemPushProxyAuthToken).Return(nil, errors.New("db down")).Once()
	assert.Equal(t, "tok", p.GetAuthToken())

	sys.On("GetByName", model.SystemPushProxyAuthToken).Return(nil, store.NewErrNotFound("System", "x")).Once()
	assert.Equal(t, "", p.GetAuthToken())
	sys.AssertExpectations(t)
}

func TestGenerateAndDeleteAuthToken(t *testing.T) {
	p, sys, _ := newTest(t)

	sys.On("GetByName", model.SystemPushProxyAuthToken).Return(&model.System{Value: "old"}, nil).Once()
	assert.Equal(t, "old", p.GetAuthToken())

	// A token issued for another push proxy is dropped when the URL changes.
	sys.On("PermanentDeleteByName", model.SystemPushProxyAuthToken).Return(&model.System{}, nil).Once()
	require.Nil(t, p.GenerateAuthToken())
	assert.Equal(t, "", p.GetAuthToken())

	sys.On("PermanentDeleteByName", model.SystemPushProxyAuthToken).Return(nil, errors.New("db down")).Once()
	appErr := p.DeleteAuthToken()
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.push_proxy.delete.app_error", appErr.Id)
	sys.AssertExpectations(t)
}

func TestScheduler(t *testing.T) {
	p, _, _ := newTest(t)
	s := p.MakeScheduler()
	assert.False(t, s.Enabled(&model.Config{}))
	assert.Nil(t, s.NextScheduleTime(&model.Config{}, time.Now(), false, nil))
}
