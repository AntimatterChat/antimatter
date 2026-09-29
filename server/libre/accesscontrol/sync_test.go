// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package accesscontrol

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest/mocks"
)

type fakeMembershipApp struct {
	channels map[string]*model.Channel
	teams    map[string]*model.Team
	added    []string
	removed  []string
}

func (f *fakeMembershipApp) GetChannel(_ request.CTX, id string) (*model.Channel, *model.AppError) {
	if c, ok := f.channels[id]; ok {
		return c, nil
	}
	return nil, model.NewAppError("GetChannel", "not_found", nil, "", http.StatusNotFound)
}

func (f *fakeMembershipApp) GetTeam(id string) (*model.Team, *model.AppError) {
	if t, ok := f.teams[id]; ok {
		return t, nil
	}
	return nil, model.NewAppError("GetTeam", "not_found", nil, "", http.StatusNotFound)
}

func (f *fakeMembershipApp) AddChannelMemberByAccessPolicy(_ request.CTX, c *model.Channel, userID, _ string, _ int) *model.AppError {
	f.added = append(f.added, c.Id+"/"+userID)
	return nil
}

func (f *fakeMembershipApp) RemoveChannelMemberByAccessPolicy(_ request.CTX, c *model.Channel, userID, _ string, _ int) *model.AppError {
	f.removed = append(f.removed, c.Id+"/"+userID)
	return nil
}

func (f *fakeMembershipApp) AddTeamMemberByAccessPolicy(_ request.CTX, t *model.Team, _ *model.Bot, userID, _ string, _ int) *model.AppError {
	f.added = append(f.added, t.Id+"/"+userID)
	return nil
}

func (f *fakeMembershipApp) RemoveTeamMemberByAccessPolicy(_ request.CTX, t *model.Team, _ *model.Bot, userID, _ string, _ int) *model.AppError {
	f.removed = append(f.removed, t.Id+"/"+userID)
	return nil
}

func TestQueriesAndSync(t *testing.T) {
	e := newTestEnv(t)
	parentID := model.NewId()
	privateID := model.NewId()
	publicID := model.NewId()
	teamID := model.NewId()
	stay, leave, join := model.NewId(), model.NewId(), model.NewId()

	e.addPolicy(&model.AccessControlPolicy{
		ID: parentID, Name: "p", Type: model.AccessControlPolicyTypeParent, Version: model.AccessControlPolicyVersionV0_3,
		Rules: []model.AccessControlPolicyRule{membershipRule(`user.attributes.id_` + fieldDepartment + ` == "Engineering"`)},
	})
	private := &model.AccessControlPolicy{
		ID: privateID, Type: model.AccessControlPolicyTypeChannel, Version: model.AccessControlPolicyVersionV0_3, Imports: []string{parentID},
	}
	private.SetAutoAddMode(model.AccessControlAutoAddAlways)
	e.addPolicy(private)
	e.addPolicy(&model.AccessControlPolicy{
		ID: publicID, Type: model.AccessControlPolicyTypeChannel, Version: model.AccessControlPolicyVersionV0_3, Imports: []string{parentID},
	})
	e.addPolicy(&model.AccessControlPolicy{
		ID: teamID, Type: model.AccessControlPolicyTypeTeam, Version: model.AccessControlPolicyVersionV0_3,
		Rules: []model.AccessControlPolicyRule{membershipRule(`user.attributes.department == "Engineering"`)},
	})

	wantQuery := `((UserAttributeView.Attributes ->> $1::text) = $2::text)`
	e.attrs.On("GetChannelMembersToRemove", mock.Anything, mock.Anything, mock.MatchedBy(func(o model.SubjectSearchOptions) bool {
		return o.Query == wantQuery && len(o.Args) == 2 && o.Args[1] == "Engineering"
	})).Return(func(_ request.CTX, channelID string, _ model.SubjectSearchOptions) ([]*model.ChannelMember, error) {
		return []*model.ChannelMember{{ChannelId: channelID, UserId: leave}}, nil
	})
	e.attrs.On("GetTeamMembersToRemove", mock.Anything, teamID, mock.MatchedBy(func(o model.SubjectSearchOptions) bool {
		return o.Query == wantQuery
	})).Return([]*model.TeamMember{{TeamId: teamID, UserId: leave}}, nil)
	e.attrs.On("SearchUsers", mock.Anything, mock.MatchedBy(func(o model.SubjectSearchOptions) bool {
		return o.Query == wantQuery
	})).Return(func(_ request.CTX, o model.SubjectSearchOptions) ([]*model.User, int64, error) {
		if o.ExcludeChannelMembers != "" && o.ExcludeChannelMembers != privateID {
			return nil, 0, nil
		}
		return []*model.User{{Id: join}, {Id: stay}}, 2, nil
	})

	t.Run("query users for resource", func(t *testing.T) {
		users, total, appErr := e.svc.QueryUsersForResource(e.rctx, privateID, model.AccessControlPolicyActionMembership, model.SubjectSearchOptions{})
		require.Nil(t, appErr)
		assert.Equal(t, int64(2), total)
		assert.Len(t, users, 2)
	})

	t.Run("members to remove", func(t *testing.T) {
		members, appErr := e.svc.GetChannelMembersToRemove(e.rctx, privateID)
		require.Nil(t, appErr)
		require.Len(t, members, 1)
		assert.Equal(t, leave, members[0].UserId)

		none, appErr := e.svc.GetChannelMembersToRemove(e.rctx, model.NewId())
		require.Nil(t, appErr)
		assert.Empty(t, none)
	})

	t.Run("channel sync", func(t *testing.T) {
		fake := &fakeMembershipApp{channels: map[string]*model.Channel{
			privateID: {Id: privateID, Type: model.ChannelTypePrivate, TeamId: teamID},
			publicID:  {Id: publicID, Type: model.ChannelTypeOpen, TeamId: teamID},
		}}
		sy := &syncer{store: e.st, acs: e.svc, app: fake, logger: mlog.CreateConsoleTestLogger(t)}
		require.NoError(t, sy.syncChannels(e.rctx, &model.Job{Id: model.NewId(), Data: model.StringMap{"policy_id": parentID}}))
		assert.Equal(t, []string{privateID + "/" + leave}, fake.removed, "public channels are not pruned")
		assert.ElementsMatch(t, []string{privateID + "/" + join, privateID + "/" + stay}, fake.added, "only auto-add policies add")
		assert.Equal(t, 2, sy.synced)
	})

	t.Run("team sync", func(t *testing.T) {
		teamMembers := &mocks.TeamStore{}
		e.st.On("Team").Return(teamMembers)
		teamMembers.On("GetMembersByIds", teamID, mock.Anything, mock.Anything).Return([]*model.TeamMember{{TeamId: teamID, UserId: stay}}, nil)

		p := clonePolicy(e.policies[teamID])
		p.SetAutoAddMode(model.AccessControlAutoAddAlways)
		e.addPolicy(p)

		fake := &fakeMembershipApp{teams: map[string]*model.Team{teamID: {Id: teamID, AllowOpenInvite: false}}}
		sy := &syncer{store: e.st, acs: e.svc, app: fake, logger: mlog.CreateConsoleTestLogger(t)}
		e.svc.invalidateAllPolicies()
		require.NoError(t, sy.syncTeams(e.rctx, &model.Job{Id: model.NewId(), Data: model.StringMap{}}))
		assert.Equal(t, []string{teamID + "/" + leave}, fake.removed)
		assert.Equal(t, []string{teamID + "/" + join}, fake.added)
	})
}

func TestSimulatePolicyForUsers(t *testing.T) {
	e := newTestEnv(t)
	channelID := model.NewId()
	eng := &model.User{Id: model.NewId(), Roles: model.SystemUserRoleId}
	sales := &model.User{Id: model.NewId(), Roles: model.SystemUserRoleId}

	users := &mocks.UserStore{}
	channels := &mocks.ChannelStore{}
	e.st.On("User").Return(users)
	e.st.On("Channel").Return(channels)
	users.On("Get", mock.Anything, eng.Id).Return(eng, nil)
	users.On("Get", mock.Anything, sales.Id).Return(sales, nil)
	channels.On("GetMember", mock.Anything, channelID, mock.Anything).Return(&model.ChannelMember{SchemeUser: true}, nil)
	e.attrs.On("GetSubject", mock.Anything, eng.Id, testGroupID, model.PropertyFieldObjectTypeUser).Return(&model.Subject{ID: eng.Id, Attributes: map[string]any{"department": "Engineering"}}, nil)
	e.attrs.On("GetSubject", mock.Anything, sales.Id, testGroupID, model.PropertyFieldObjectTypeUser).Return(&model.Subject{ID: sales.Id, Attributes: map[string]any{"department": "Sales"}}, nil)

	draft := &model.AccessControlPolicy{
		ID: channelID, Type: model.AccessControlPolicyTypeChannel, Version: model.AccessControlPolicyVersionV0_4,
		Rules: []model.AccessControlPolicyRule{
			{Name: "eng", Role: model.ChannelUserRoleId, Actions: []string{model.AccessControlPolicyActionUploadFileAttachment}, Expression: `user.attributes.department == "Engineering"`},
		},
	}
	resp, appErr := e.svc.SimulatePolicyForUsers(e.rctx, model.PolicySimulationByUsersParams{
		Policy:    draft,
		Actions:   []string{model.AccessControlPolicyActionUploadFileAttachment},
		RuleName:  "eng",
		ChannelID: channelID,
		Users:     []model.PolicySimulationUserOverride{{UserID: eng.Id}, {UserID: sales.Id}},
	})
	require.Nil(t, appErr)
	require.Len(t, resp.Results, 2)

	allowed := resp.Results[0].Decisions[model.AccessControlPolicyActionUploadFileAttachment]
	assert.True(t, allowed.Decision)

	denied := resp.Results[1].Decisions[model.AccessControlPolicyActionUploadFileAttachment]
	assert.False(t, denied.Decision)
	require.Len(t, denied.Blame, 1)
	assert.Equal(t, model.PolicySimulationBlameSourceThisRule, denied.Blame[0].Source)
	require.NotNil(t, denied.Blame[0].EvaluationTree)
	tree := denied.Blame[0].EvaluationTree
	assert.Equal(t, model.PolicySimulationEvaluationKindCompare, tree.Kind)
	assert.Equal(t, model.PolicySimulationEvaluationOutcomeFalse, tree.Outcome)
	assert.Equal(t, "Sales", tree.ActualValue)
	assert.Equal(t, "Engineering", tree.ExpectedValue)
	assert.Equal(t, "Sales", resp.Results[1].Attributes["department"])

	_, appErr = e.svc.SimulatePolicyForUsers(e.rctx, model.PolicySimulationByUsersParams{Policy: draft, Users: []model.PolicySimulationUserOverride{{UserID: eng.Id}}})
	require.NotNil(t, appErr)
	assert.Equal(t, "app.pap.simulate.missing_actions", appErr.Id)
}
