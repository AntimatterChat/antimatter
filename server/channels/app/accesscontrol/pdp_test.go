// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package accesscontrol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func membershipRule(expr string) model.AccessControlPolicyRule {
	return model.AccessControlPolicyRule{Actions: []string{model.AccessControlPolicyActionMembership}, Expression: expr}
}

func TestAccessEvaluationMembership(t *testing.T) {
	e := newTestEnv(t)
	parentID := model.NewId()
	channelID := model.NewId()
	e.addPolicy(&model.AccessControlPolicy{
		ID: parentID, Name: "parent", Type: model.AccessControlPolicyTypeParent, Version: model.AccessControlPolicyVersionV0_3,
		Rules: []model.AccessControlPolicyRule{membershipRule(`user.attributes.id_` + fieldDepartment + ` == "Engineering"`)},
	})
	e.addPolicy(&model.AccessControlPolicy{
		ID: channelID, Type: model.AccessControlPolicyTypeChannel, Version: model.AccessControlPolicyVersionV0_3,
		Imports: []string{parentID, model.NewId()}, // second import dangles
		Rules:   []model.AccessControlPolicyRule{membershipRule(`user.attributes.clearance >= "Secret"`)},
	})

	eval := func(attrs map[string]any, resource string) model.AccessDecision {
		d, appErr := e.svc.AccessEvaluation(e.rctx, model.AccessRequest{
			Subject:  subjectWith(attrs),
			Resource: model.Resource{ID: resource, Type: model.AccessControlPolicyTypeChannel},
			Action:   model.AccessControlPolicyActionMembership,
		})
		require.Nil(t, appErr)
		return d
	}

	allowed := eval(map[string]any{"department": "Engineering", "clearance": map[string]any{"name": "TopSecret", "rank": 3.0}}, channelID)
	assert.True(t, allowed.Decision)
	assert.False(t, allowed.IsNoPolicy())

	assert.False(t, eval(map[string]any{"department": "Sales", "clearance": map[string]any{"name": "TopSecret", "rank": 3.0}}, channelID).Decision)
	assert.False(t, eval(map[string]any{"department": "Engineering", "clearance": map[string]any{"name": "Public", "rank": 1.0}}, channelID).Decision)
	assert.False(t, eval(map[string]any{}, channelID).Decision, "missing attributes deny")

	noPolicy := eval(map[string]any{}, model.NewId())
	assert.True(t, noPolicy.IsNoPolicy())

	t.Run("disabled ABAC is unregulated", func(t *testing.T) {
		e.cfg.AccessControlSettings.EnableAttributeBasedAccessControl = model.NewPointer(false)
		defer func() { e.cfg.AccessControlSettings.EnableAttributeBasedAccessControl = model.NewPointer(true) }()
		assert.True(t, eval(map[string]any{}, channelID).IsNoPolicy())
	})
}

func TestAccessEvaluationTeam(t *testing.T) {
	e := newTestEnv(t)
	teamID := model.NewId()
	e.addPolicy(&model.AccessControlPolicy{
		ID: teamID, Type: model.AccessControlPolicyTypeTeam, Version: model.AccessControlPolicyVersionV0_3,
		Rules: []model.AccessControlPolicyRule{membershipRule(`user.attributes.department == "Engineering"`)},
	})
	req := model.AccessRequest{
		Subject:  subjectWith(map[string]any{"department": "Sales"}),
		Resource: model.Resource{ID: teamID, Type: model.AccessControlPolicyTypeTeam},
		Action:   model.AccessControlPolicyActionMembership,
	}
	d, appErr := e.svc.AccessEvaluation(e.rctx, req)
	require.Nil(t, appErr)
	assert.False(t, d.Decision)

	e.cfg.FeatureFlags.TeamMembershipAccessControl = false
	d, appErr = e.svc.AccessEvaluation(e.rctx, req)
	require.Nil(t, appErr)
	assert.True(t, d.IsNoPolicy())
}

func TestAccessEvaluationPermissions(t *testing.T) {
	e := newTestEnv(t)
	channelID := model.NewId()
	e.addPolicy(&model.AccessControlPolicy{
		ID: channelID, Type: model.AccessControlPolicyTypeChannel, Version: model.AccessControlPolicyVersionV0_4,
		Rules: []model.AccessControlPolicyRule{
			{Name: "eng uploads", Role: model.ChannelUserRoleId, Actions: []string{model.AccessControlPolicyActionUploadFileAttachment}, Expression: `user.attributes.department == "Engineering"`},
			{Name: "ops uploads", Role: model.ChannelUserRoleId, Actions: []string{model.AccessControlPolicyActionUploadFileAttachment}, Expression: `user.attributes.department == "Ops"`},
		},
	})
	e.addPolicy(&model.AccessControlPolicy{
		ID: model.NewId(), Name: "downloads", Type: model.AccessControlPolicyTypePermission, Version: model.AccessControlPolicyVersionV0_3,
		Roles: []string{model.SystemUserRoleId},
		Rules: []model.AccessControlPolicyRule{{Actions: []string{model.AccessControlPolicyActionDownloadFileAttachment}, Expression: `user.attributes.clearance >= "Secret"`}},
	})

	eval := func(attrs map[string]any, channelRole, systemRole, action string) model.AccessDecision {
		s := subjectWith(attrs)
		s.SetScopedRole(model.AccessControlSubjectScopeSystem, systemRole)
		s.SetScopedRole(model.AccessControlSubjectScopeChannel, channelRole)
		d, appErr := e.svc.AccessEvaluation(e.rctx, model.AccessRequest{
			Subject:  s,
			Resource: model.Resource{ID: channelID, Type: model.AccessControlPolicyTypeChannel},
			Action:   action,
		})
		require.Nil(t, appErr)
		return d
	}
	upload := model.AccessControlPolicyActionUploadFileAttachment
	download := model.AccessControlPolicyActionDownloadFileAttachment

	assert.True(t, eval(map[string]any{"department": "Ops"}, model.ChannelUserRoleId, model.SystemUserRoleId, upload).Decision, "rules OR together")
	assert.False(t, eval(map[string]any{"department": "Sales"}, model.ChannelUserRoleId, model.SystemUserRoleId, upload).Decision)
	assert.False(t, eval(map[string]any{"department": "Sales"}, model.ChannelAdminRoleId, model.SystemUserRoleId, upload).Decision, "admins fall back to member rules")
	assert.True(t, eval(map[string]any{"department": "Sales"}, model.ChannelGuestRoleId, model.SystemUserRoleId, upload).IsNoPolicy(), "no guest rule")

	assert.True(t, eval(map[string]any{"clearance": map[string]any{"name": "Secret", "rank": 2.0}}, model.ChannelUserRoleId, model.SystemUserRoleId, download).Decision)
	assert.False(t, eval(map[string]any{"clearance": map[string]any{"name": "Public", "rank": 1.0}}, model.ChannelUserRoleId, model.SystemAdminRoleId, download).Decision, "admins fall back to member policies")
	assert.True(t, eval(map[string]any{}, model.ChannelGuestRoleId, model.SystemGuestRoleId, download).IsNoPolicy())

	e.cfg.FeatureFlags.PermissionPolicies = false
	assert.True(t, eval(map[string]any{"department": "Sales"}, model.ChannelUserRoleId, model.SystemUserRoleId, upload).IsNoPolicy())
}

func TestAccessEvaluationPluginPolicy(t *testing.T) {
	e := newTestEnv(t)
	resourceID := model.NewId()
	e.addPolicy(&model.AccessControlPolicy{
		ID: resourceID, Name: "agent", Type: "com.example.plugin:agent", Version: model.AccessControlPolicyVersionV0_5,
		Rules: []model.AccessControlPolicyRule{{Actions: []string{"use"}, Expression: `user.attributes.department == "Engineering"`}},
	})
	eval := func(resourceType, action string, attrs map[string]any) model.AccessDecision {
		d, appErr := e.svc.AccessEvaluation(e.rctx, model.AccessRequest{
			Subject:  subjectWith(attrs),
			Resource: model.Resource{ID: resourceID, Type: resourceType},
			Action:   action,
		})
		require.Nil(t, appErr)
		return d
	}
	assert.True(t, eval("com.example.plugin:agent", "use", map[string]any{"department": "Engineering"}).Decision)
	assert.False(t, eval("com.example.plugin:agent", "use", map[string]any{"department": "Sales"}).Decision)
	assert.True(t, eval("com.example.plugin:agent", "other", nil).IsNoPolicy())
	assert.True(t, eval("com.other.plugin:agent", "use", nil).IsNoPolicy())
}

func TestSaveAndGetPolicy(t *testing.T) {
	e := newTestEnv(t)
	parentID := model.NewId()

	saved, appErr := e.svc.SavePolicy(e.rctx, &model.AccessControlPolicy{
		ID: parentID, Name: "Engineers", Type: model.AccessControlPolicyTypeParent,
		Rules: []model.AccessControlPolicyRule{membershipRule(`user.attributes.department == "Engineering"`)},
	})
	require.Nil(t, appErr)
	assert.Equal(t, `user.attributes.department == "Engineering"`, saved.Rules[0].Expression)
	assert.Equal(t, model.AccessControlPolicyVersionV0_3, saved.Version)

	// Stored by field ID.
	assert.Equal(t, `user.attributes.id_`+fieldDepartment+` == "Engineering"`, e.policies[parentID].Rules[0].Expression)

	got, appErr := e.svc.GetPolicy(e.rctx, parentID)
	require.Nil(t, appErr)
	assert.Equal(t, `user.attributes.department == "Engineering"`, got.Rules[0].Expression)

	_, appErr = e.svc.GetPolicy(e.rctx, model.NewId())
	require.NotNil(t, appErr)
	assert.Equal(t, 404, appErr.StatusCode)

	t.Run("invalid expression", func(t *testing.T) {
		_, appErr := e.svc.SavePolicy(e.rctx, &model.AccessControlPolicy{
			Name: "bad", Type: model.AccessControlPolicyTypeParent,
			Rules: []model.AccessControlPolicyRule{membershipRule(`user.attributes.department ==`)},
		})
		require.NotNil(t, appErr)
		assert.Equal(t, 400, appErr.StatusCode)
	})

	t.Run("unknown attribute", func(t *testing.T) {
		_, appErr := e.svc.SavePolicy(e.rctx, &model.AccessControlPolicy{
			Name: "bad", Type: model.AccessControlPolicyTypeParent,
			Rules: []model.AccessControlPolicyRule{membershipRule(`user.attributes.nope == "x"`)},
		})
		require.NotNil(t, appErr)
		assert.Equal(t, 400, appErr.StatusCode)
	})

	t.Run("channel attributes need the flag and a channel", func(t *testing.T) {
		teamPolicy := &model.AccessControlPolicy{
			ID: model.NewId(), Type: model.AccessControlPolicyTypeTeam,
			Rules: []model.AccessControlPolicyRule{membershipRule(`user.attributes.department == resource.attributes.department`)},
		}
		_, appErr := e.svc.SavePolicy(e.rctx, teamPolicy)
		require.NotNil(t, appErr)
		assert.Equal(t, "app.pap.save_policy.team_resource_attributes", appErr.Id)

		e.cfg.FeatureFlags.ResourceAttributesInPolicies = false
		defer func() { e.cfg.FeatureFlags.ResourceAttributesInPolicies = true }()
		teamPolicy.Type = model.AccessControlPolicyTypeChannel
		_, appErr = e.svc.SavePolicy(e.rctx, teamPolicy)
		require.NotNil(t, appErr)
		assert.Equal(t, "app.pap.save_policy.resource_attributes_disabled", appErr.Id)
	})

	t.Run("imports must be parents", func(t *testing.T) {
		_, appErr := e.svc.SavePolicy(e.rctx, &model.AccessControlPolicy{
			ID: model.NewId(), Type: model.AccessControlPolicyTypeChannel, Imports: []string{model.NewId()},
		})
		require.NotNil(t, appErr)
		assert.Equal(t, 400, appErr.StatusCode)

		child, appErr := e.svc.SavePolicy(e.rctx, &model.AccessControlPolicy{
			ID: model.NewId(), Type: model.AccessControlPolicyTypeChannel, Imports: []string{parentID},
		})
		require.Nil(t, appErr)
		assert.Equal(t, []string{parentID}, child.Imports)
	})

	t.Run("role scoped rules are v0.4", func(t *testing.T) {
		p, appErr := e.svc.SavePolicy(e.rctx, &model.AccessControlPolicy{
			ID: model.NewId(), Type: model.AccessControlPolicyTypeChannel, Version: model.AccessControlPolicyVersionV0_3,
			Rules: []model.AccessControlPolicyRule{{Name: "up", Role: model.ChannelUserRoleId, Actions: []string{model.AccessControlPolicyActionUploadFileAttachment}, Expression: `user.attributes.department == "x"`}},
		})
		require.Nil(t, appErr)
		assert.Equal(t, model.AccessControlPolicyVersionV0_4, p.Version)
	})

	t.Run("policy rule attributes", func(t *testing.T) {
		channelID := model.NewId()
		e.addPolicy(&model.AccessControlPolicy{
			ID: channelID, Type: model.AccessControlPolicyTypeChannel, Version: model.AccessControlPolicyVersionV0_3,
			Imports: []string{parentID},
			Rules:   []model.AccessControlPolicyRule{membershipRule(`user.attributes.id_` + fieldSkills + `.hasAnyOf(["Go", "SQL"]) && user.verified == true`)},
		})
		attrs, appErr := e.svc.GetPolicyRuleAttributes(e.rctx, channelID, model.AccessControlPolicyActionMembership)
		require.Nil(t, appErr)
		assert.Equal(t, map[string][]string{
			"skills":     {"Go", "SQL"},
			"verified":   {"true"},
			"department": {"Engineering"},
		}, attrs)
	})

	t.Run("delete", func(t *testing.T) {
		require.Nil(t, e.svc.DeletePolicy(e.rctx, parentID))
		_, appErr := e.svc.GetPolicy(e.rctx, parentID)
		require.NotNil(t, appErr)
	})
}
