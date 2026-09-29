// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package accesscontrol

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/jobs"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
	ejobs "github.com/mattermost/mattermost/server/v8/einterfaces/jobs"
)

const (
	syncAddPageSize        = 200
	defaultSyncJobInterval = time.Hour
)

// membershipSyncer is the part of the app the sync jobs drive. *app.App
// satisfies it; tests use a fake.
type membershipSyncer interface {
	GetChannel(rctx request.CTX, channelID string) (*model.Channel, *model.AppError)
	GetTeam(teamID string) (*model.Team, *model.AppError)
	AddChannelMemberByAccessPolicy(rctx request.CTX, channel *model.Channel, userID, jobID string, policyRevision int) *model.AppError
	RemoveChannelMemberByAccessPolicy(rctx request.CTX, channel *model.Channel, userID, jobID string, policyRevision int) *model.AppError
	AddTeamMemberByAccessPolicy(rctx request.CTX, team *model.Team, systemBot *model.Bot, userID, jobID string, policyRevision int) *model.AppError
	RemoveTeamMemberByAccessPolicy(rctx request.CTX, team *model.Team, systemBot *model.Bot, userID, jobID string, policyRevision int) *model.AppError
}

// syncer holds what one sync job run needs.
type syncer struct {
	store  store.Store
	acs    einterfaces.AccessControlServiceInterface
	app    membershipSyncer
	logger mlog.LoggerIFace

	removed int
	added   int
	synced  int
	failed  int
}

func syncEnabled(cfg *model.Config) bool {
	return cfg.AccessControlSettings.EnableAttributeBasedAccessControl != nil &&
		*cfg.AccessControlSettings.EnableAttributeBasedAccessControl
}

func teamSyncEnabled(cfg *model.Config) bool {
	return syncEnabled(cfg) && cfg.FeatureFlags != nil && cfg.FeatureFlags.TeamMembershipAccessControl
}

func syncInterval(cfg *model.Config) time.Duration {
	if cfg != nil && cfg.AccessControlSettings.SyncJobIntervalSeconds != nil && *cfg.AccessControlSettings.SyncJobIntervalSeconds >= 60 {
		return time.Duration(*cfg.AccessControlSettings.SyncJobIntervalSeconds) * time.Second
	}
	return defaultSyncJobInterval
}

// syncJob implements ejobs.AccessControlSyncJobInterface for both the channel
// and the team membership sync.
type syncJob struct {
	srv  *app.Server
	team bool
}

var _ ejobs.AccessControlSyncJobInterface = (*syncJob)(nil)

func (j *syncJob) jobType() string {
	if j.team {
		return model.JobTypeAccessControlTeamSync
	}
	return model.JobTypeAccessControlSync
}

func (j *syncJob) enabled(cfg *model.Config) bool {
	if j.team {
		return teamSyncEnabled(cfg)
	}
	return syncEnabled(cfg)
}

// MakeWorker implements ejobs.AccessControlSyncJobInterface.
func (j *syncJob) MakeWorker() model.Worker {
	name := "AccessControlSync"
	if j.team {
		name = "AccessControlTeamSync"
	}
	return jobs.NewSimpleWorker(name, j.srv.Jobs, j.execute, j.enabled)
}

// MakeScheduler implements ejobs.AccessControlSyncJobInterface.
func (j *syncJob) MakeScheduler() ejobs.Scheduler {
	return jobs.NewPeriodicScheduler(j.srv.Jobs, j.jobType(), syncInterval(j.srv.Config()), j.enabled)
}

func (j *syncJob) execute(logger mlog.LoggerIFace, job *model.Job) error {
	acs := j.srv.Channels().AccessControl
	if acs == nil {
		return model.NewAppError("AccessControlSync", "ent.access_control.sync_job.app_error", nil, "access control service is not available", http.StatusNotImplemented)
	}
	rctx := request.EmptyContext(logger)
	if st := j.srv.Store(); st != nil {
		if err := st.Attributes().RefreshAttributes(); err != nil {
			logger.Warn("Failed to refresh attribute views before membership sync", mlog.Err(err))
		}
	}

	sy := &syncer{
		store:  j.srv.Store(),
		acs:    acs,
		app:    app.New(app.ServerConnector(j.srv.Channels())),
		logger: logger,
	}

	var err error
	if j.team {
		err = sy.syncTeams(rctx, job)
	} else {
		err = sy.syncChannels(rctx, job)
	}

	if job.Data == nil {
		job.Data = model.StringMap{}
	}
	job.Data["synced"] = strconv.Itoa(sy.synced)
	job.Data["members_added"] = strconv.Itoa(sy.added)
	job.Data["members_removed"] = strconv.Itoa(sy.removed)
	job.Data["failures"] = strconv.Itoa(sy.failed)

	if err != nil {
		return model.NewAppError("AccessControlSync", "ent.access_control.sync_job.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return nil
}

func (sy *syncer) searchAll(rctx request.CTX, opts model.AccessControlPolicySearch) ([]*model.AccessControlPolicy, error) {
	var out []*model.AccessControlPolicy
	opts.Limit = maxPolicySearchPage
	opts.Cursor = model.AccessControlPolicyCursor{}
	for {
		page, _, err := sy.store.AccessControlPolicy().SearchPolicies(rctx, opts)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if len(page) < maxPolicySearchPage {
			return out, nil
		}
		opts.Cursor.ID = page[len(page)-1].ID
	}
}

// resolvePolicies returns the resource policies of the given type a job run
// covers, from its data: a policy (or, for a parent, the resources importing
// it), a list of channels, a team's channels, or everything.
func (sy *syncer) resolvePolicies(rctx request.CTX, data model.StringMap, policyType string) ([]*model.AccessControlPolicy, error) {
	getPolicy := func(id string) (*model.AccessControlPolicy, error) {
		p, err := sy.store.AccessControlPolicy().Get(rctx, id)
		if err != nil {
			if isNotFound(err) {
				return nil, nil
			}
			return nil, err
		}
		return p, nil
	}

	if policyID := data["policy_id"]; policyID != "" {
		p, err := getPolicy(policyID)
		if err != nil || p == nil {
			return nil, err
		}
		switch p.Type {
		case policyType:
			return []*model.AccessControlPolicy{p}, nil
		case model.AccessControlPolicyTypeParent:
			return sy.searchAll(rctx, model.AccessControlPolicySearch{Type: policyType, ParentID: p.ID})
		}
		return nil, nil
	}

	if policyType == model.AccessControlPolicyTypeChannel {
		if ids := strings.TrimSpace(data["channel_ids"]); ids != "" {
			var out []*model.AccessControlPolicy
			for _, id := range strings.Split(ids, ",") {
				id = strings.TrimSpace(id)
				if !model.IsValidId(id) {
					continue
				}
				p, err := getPolicy(id)
				if err != nil {
					return nil, err
				}
				if p != nil && p.Type == policyType {
					out = append(out, p)
				}
			}
			return out, nil
		}
		if teamID := data["team_id"]; teamID != "" {
			return sy.searchAll(rctx, model.AccessControlPolicySearch{Type: policyType, TeamID: teamID})
		}
	}

	return sy.searchAll(rctx, model.AccessControlPolicySearch{Type: policyType})
}

func (sy *syncer) syncChannels(rctx request.CTX, job *model.Job) error {
	policies, err := sy.resolvePolicies(rctx, job.Data, model.AccessControlPolicyTypeChannel)
	if err != nil {
		return errors.Wrap(err, "failed to resolve channel policies")
	}
	for _, p := range policies {
		if err := sy.syncChannel(rctx, job.Id, p); err != nil {
			sy.failed++
			sy.logger.Warn("Failed to sync channel membership with its access policy", mlog.String("channel_id", p.ID), mlog.Err(err))
			continue
		}
		sy.synced++
	}
	return nil
}

func (sy *syncer) syncChannel(rctx request.CTX, jobID string, policy *model.AccessControlPolicy) error {
	channel, appErr := sy.app.GetChannel(rctx, policy.ID)
	if appErr != nil {
		if appErr.StatusCode == http.StatusNotFound {
			return nil
		}
		return appErr
	}
	if channel.DeleteAt != 0 {
		return nil
	}

	// Removals. Join is only gated on private channels, so membership of a
	// public channel is not revoked either.
	if channel.Type == model.ChannelTypePrivate {
		members, appErr := sy.acs.GetChannelMembersToRemove(rctx, channel.Id)
		if appErr != nil {
			return appErr
		}
		for _, m := range members {
			if appErr := sy.app.RemoveChannelMemberByAccessPolicy(rctx, channel, m.UserId, jobID, policy.Revision); appErr != nil {
				sy.logger.Warn("Failed to remove channel member no longer matching the access policy",
					mlog.String("channel_id", channel.Id), mlog.String("user_id", m.UserId), mlog.Err(appErr))
				continue
			}
			sy.removed++
		}
	}

	if !policy.AutoAddMembers() {
		return nil
	}

	cursor := ""
	for {
		users, _, appErr := sy.acs.QueryUsersForResource(rctx, channel.Id, model.AccessControlPolicyActionMembership, model.SubjectSearchOptions{
			TeamID:                channel.TeamId,
			ExcludeChannelMembers: channel.Id,
			Limit:                 syncAddPageSize,
			IgnoreCount:           true,
			Cursor:                model.SubjectCursor{TargetID: cursor},
		})
		if appErr != nil {
			return appErr
		}
		for _, u := range users {
			if appErr := sy.app.AddChannelMemberByAccessPolicy(rctx, channel, u.Id, jobID, policy.Revision); appErr != nil {
				sy.logger.Warn("Failed to add user matching the channel access policy",
					mlog.String("channel_id", channel.Id), mlog.String("user_id", u.Id), mlog.Err(appErr))
				continue
			}
			sy.added++
		}
		if len(users) < syncAddPageSize {
			return nil
		}
		cursor = users[len(users)-1].Id
	}
}

func (sy *syncer) syncTeams(rctx request.CTX, job *model.Job) error {
	policies, err := sy.resolvePolicies(rctx, job.Data, model.AccessControlPolicyTypeTeam)
	if err != nil {
		return errors.Wrap(err, "failed to resolve team policies")
	}
	for _, p := range policies {
		if err := sy.syncTeam(rctx, job.Id, p); err != nil {
			sy.failed++
			sy.logger.Warn("Failed to sync team membership with its access policy", mlog.String("team_id", p.ID), mlog.Err(err))
			continue
		}
		sy.synced++
	}
	return nil
}

func (sy *syncer) syncTeam(rctx request.CTX, jobID string, policy *model.AccessControlPolicy) error {
	team, appErr := sy.app.GetTeam(policy.ID)
	if appErr != nil {
		if appErr.StatusCode == http.StatusNotFound {
			return nil
		}
		return appErr
	}
	if team.DeleteAt != 0 {
		return nil
	}

	// On open teams the policy is advisory: members are not removed.
	if !team.AllowOpenInvite {
		members, appErr := sy.acs.GetTeamMembersToRemove(rctx, team.Id)
		if appErr != nil {
			return appErr
		}
		for _, m := range members {
			if appErr := sy.app.RemoveTeamMemberByAccessPolicy(rctx, team, nil, m.UserId, jobID, policy.Revision); appErr != nil {
				sy.logger.Warn("Failed to remove team member no longer matching the access policy",
					mlog.String("team_id", team.Id), mlog.String("user_id", m.UserId), mlog.Err(appErr))
				continue
			}
			sy.removed++
		}
	}

	if !policy.AutoAddMembers() {
		return nil
	}

	cursor := ""
	for {
		users, _, appErr := sy.acs.QueryUsersForResource(rctx, team.Id, model.AccessControlPolicyActionMembership, model.SubjectSearchOptions{
			Limit:       syncAddPageSize,
			IgnoreCount: true,
			Cursor:      model.SubjectCursor{TargetID: cursor},
		})
		if appErr != nil {
			return appErr
		}
		if len(users) > 0 {
			ids := make([]string, 0, len(users))
			for _, u := range users {
				ids = append(ids, u.Id)
			}
			members, err := sy.store.Team().GetMembersByIds(team.Id, ids, nil)
			if err != nil {
				return errors.Wrap(err, "failed to load team members")
			}
			current := make(map[string]bool, len(members))
			for _, m := range members {
				if m.DeleteAt == 0 {
					current[m.UserId] = true
				}
			}
			for _, u := range users {
				if current[u.Id] {
					continue
				}
				if appErr := sy.app.AddTeamMemberByAccessPolicy(rctx, team, nil, u.Id, jobID, policy.Revision); appErr != nil {
					sy.logger.Warn("Failed to add user matching the team access policy",
						mlog.String("team_id", team.Id), mlog.String("user_id", u.Id), mlog.Err(appErr))
					continue
				}
				sy.added++
			}
		}
		if len(users) < syncAddPageSize {
			return nil
		}
		cursor = users[len(users)-1].Id
	}
}
