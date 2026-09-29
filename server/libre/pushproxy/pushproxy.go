// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package pushproxy manages the authentication token that the server sends
// to the push notification proxy in the X-Mattermost-Auth header.
//
// Such tokens are issued by Mattermost Inc's hosted push notification
// service in exchange for a license-signed request; the open source
// mattermost-push-proxy that self-hosted deployments run does not
// authenticate servers and has no token endpoint. Mattermost Libre therefore
// never obtains new tokens: it keeps serving a token already stored in the
// database (for example one issued before migrating to Libre) until the push
// proxy URL changes, and otherwise sends push notifications without the
// authentication headers, which is what self-hosted push proxies expect.
package pushproxy

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/jobs"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
	ejobs "github.com/mattermost/mattermost/server/v8/einterfaces/jobs"
)

// cacheTTL bounds how long a token read from the database is reused before
// being read again (it may be changed by another cluster node).
const cacheTTL = 5 * time.Minute

func init() {
	app.RegisterPushProxyInterface(func(a *app.App) einterfaces.PushProxyInterface {
		return New(
			func() store.SystemStore { return a.Srv().Store().System() },
			func() mlog.LoggerIFace { return a.Log() },
			func() *jobs.JobServer { return a.Srv().Jobs },
		)
	})
}

// tokenCache is shared by every instance: the server creates one instance for
// the push notification path and another one for the job worker.
type tokenCache struct {
	mu       sync.Mutex
	token    string
	loadedAt time.Time
	loaded   bool
}

var sharedCache = &tokenCache{}

// PushProxy implements einterfaces.PushProxyInterface.
type PushProxy struct {
	system func() store.SystemStore
	logger func() mlog.LoggerIFace
	jobs   func() *jobs.JobServer
	cache  *tokenCache
	now    func() time.Time
}

var _ einterfaces.PushProxyInterface = (*PushProxy)(nil)

// New creates the push proxy token manager.
func New(system func() store.SystemStore, logger func() mlog.LoggerIFace, jobServer func() *jobs.JobServer) *PushProxy {
	return &PushProxy{system: system, logger: logger, jobs: jobServer, cache: sharedCache, now: time.Now}
}

// GetAuthToken returns the stored token, or "" when there is none, in which
// case push notifications are sent without authentication headers.
func (p *PushProxy) GetAuthToken() string {
	p.cache.mu.Lock()
	defer p.cache.mu.Unlock()

	if p.cache.loaded && p.now().Sub(p.cache.loadedAt) < cacheTTL {
		return p.cache.token
	}

	system, err := p.system().GetByName(model.SystemPushProxyAuthToken)
	if err != nil {
		var nfErr *store.ErrNotFound
		if !errors.As(err, &nfErr) {
			// Keep the previous value on transient errors; retry later.
			p.logger().Debug("Failed to read the push proxy auth token", mlog.Err(err))
			if p.cache.loaded {
				return p.cache.token
			}
			return ""
		}
		system = nil
	}

	p.cache.token = ""
	if system != nil {
		p.cache.token = system.Value
	}
	p.cache.loaded = true
	p.cache.loadedAt = p.now()
	return p.cache.token
}

// GenerateAuthToken is called when the push proxy URL changes and by the
// push proxy auth job. Self-hosted push proxies don't issue tokens, so the
// only thing to do is to make sure a token obtained for another push proxy
// is not sent to the new one.
func (p *PushProxy) GenerateAuthToken() *model.AppError {
	if appErr := p.DeleteAuthToken(); appErr != nil {
		return appErr
	}
	p.logger().Info("The push notification server does not require an authentication token; push notifications are sent without one")
	return nil
}

// DeleteAuthToken removes the stored token.
func (p *PushProxy) DeleteAuthToken() *model.AppError {
	if _, err := p.system().PermanentDeleteByName(model.SystemPushProxyAuthToken); err != nil {
		var nfErr *store.ErrNotFound
		if !errors.As(err, &nfErr) {
			return model.NewAppError("DeleteAuthToken", "ent.push_proxy.delete.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
	}

	p.cache.mu.Lock()
	p.cache.token = ""
	p.cache.loaded = true
	p.cache.loadedAt = p.now()
	p.cache.mu.Unlock()
	return nil
}

// MakeWorker returns the worker of the push proxy auth job.
func (p *PushProxy) MakeWorker() model.Worker {
	return jobs.NewSimpleWorker(model.JobTypePushProxyAuth, p.jobs(), func(logger mlog.LoggerIFace, _ *model.Job) error {
		if appErr := p.GenerateAuthToken(); appErr != nil {
			return appErr
		}
		return nil
	}, func(*model.Config) bool { return true })
}

// MakeScheduler returns the scheduler of the push proxy auth job. There are
// no tokens to renew, so it never schedules jobs.
func (p *PushProxy) MakeScheduler() ejobs.Scheduler {
	return &scheduler{jobs: p.jobs}
}

type scheduler struct {
	jobs func() *jobs.JobServer
}

func (s *scheduler) Enabled(*model.Config) bool {
	return false
}

func (s *scheduler) NextScheduleTime(*model.Config, time.Time, bool, *model.Job) *time.Time {
	return nil
}

func (s *scheduler) ScheduleJob(rctx request.CTX, _ *model.Config, _ bool, _ *model.Job) (*model.Job, *model.AppError) {
	return s.jobs().CreateJob(rctx, model.JobTypePushProxyAuth, nil)
}
