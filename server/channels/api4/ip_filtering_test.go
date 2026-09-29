// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// loopbackRanges allow the test client, which connects over loopback.
var loopbackRanges = model.AllowedIPRanges{
	{CIDRBlock: "127.0.0.0/8", Description: "loopback v4", Enabled: true, Action: model.IPFilterActionAllow},
	{CIDRBlock: "::1", Description: "loopback v6", Enabled: true, Action: model.IPFilterActionAllow},
}

func TestGetIPFilters(t *testing.T) {
	th := Setup(t).InitBasic(t)
	th.App.UpdateConfig(func(cfg *model.Config) {
		cfg.IPFilteringSettings.Rules = model.AllowedIPRanges{
			{CIDRBlock: "192.0.2.0/24", Description: "test", Enabled: false, Action: model.IPFilterActionDeny},
		}
	})

	t.Run("requires permission", func(t *testing.T) {
		_, resp, err := th.Client.GetIPFilters(context.Background())
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("returns the configured rules", func(t *testing.T) {
		ranges, _, err := th.SystemAdminClient.GetIPFilters(context.Background())
		require.NoError(t, err)
		require.Len(t, *ranges, 1)
		assert.Equal(t, "192.0.2.0/24", (*ranges)[0].CIDRBlock)
		assert.True(t, (*ranges)[0].IsDeny())
	})
}

func TestApplyIPFilters(t *testing.T) {
	th := Setup(t).InitBasic(t)
	t.Cleanup(func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.IPFilteringSettings.Rules = model.AllowedIPRanges{} })
	})

	t.Run("requires permission", func(t *testing.T) {
		_, resp, err := th.Client.ApplyIPFilters(context.Background(), &loopbackRanges)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("saves allow and deny rules", func(t *testing.T) {
		ranges := append(model.AllowedIPRanges{
			{CIDRBlock: " 192.0.2.0/24 ", Description: "documentation range", Enabled: true, Action: model.IPFilterActionDeny},
			{CIDRBlock: "198.51.100.7", Description: "no action given", Enabled: false},
		}, loopbackRanges...)

		applied, _, err := th.SystemAdminClient.ApplyIPFilters(context.Background(), &ranges)
		require.NoError(t, err)
		require.Len(t, *applied, 4)

		saved := th.App.Config().IPFilteringSettings.Rules
		require.Len(t, saved, 4)
		assert.Equal(t, "192.0.2.0/24", saved[0].CIDRBlock, "the range is trimmed")
		assert.True(t, saved[0].IsDeny())
		assert.Equal(t, model.IPFilterActionAllow, saved[1].Action, "a missing action defaults to allow")
		assert.Equal(t, th.SystemAdminUser.Id, saved[0].OwnerID)

		// The server still answers the admin, who is allowed by the loopback rules.
		_, _, err = th.SystemAdminClient.GetIPFilters(context.Background())
		require.NoError(t, err)
	})

	t.Run("refuses rules that lock the requester out", func(t *testing.T) {
		before := th.App.Config().IPFilteringSettings.Rules

		ranges := model.AllowedIPRanges{
			{CIDRBlock: "192.0.2.0/24", Enabled: true, Action: model.IPFilterActionAllow},
		}
		_, resp, err := th.SystemAdminClient.ApplyIPFilters(context.Background(), &ranges)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		CheckErrorID(t, err, "app.ip_filtering.apply.lockout.app_error")
		assert.Equal(t, before, th.App.Config().IPFilteringSettings.Rules)
	})

	t.Run("refuses invalid rules", func(t *testing.T) {
		for _, bad := range []model.AllowedIPRange{
			{CIDRBlock: "192.0.2.0/33", Enabled: true},
			{CIDRBlock: "not-an-ip", Enabled: false},
			{CIDRBlock: "192.0.2.0/24", Enabled: true, Action: "maybe"},
		} {
			ranges := append(model.AllowedIPRanges{bad}, loopbackRanges...)
			_, resp, err := th.SystemAdminClient.ApplyIPFilters(context.Background(), &ranges)
			require.Error(t, err, "rule %+v", bad)
			CheckBadRequestStatus(t, resp)
		}
	})
}

func TestIPFilteringEnforcement(t *testing.T) {
	th := Setup(t).InitBasic(t)
	t.Cleanup(func() {
		th.App.UpdateConfig(func(cfg *model.Config) { cfg.IPFilteringSettings.Rules = model.AllowedIPRanges{} })
	})

	_, resp, err := th.Client.GetMe(context.Background(), "")
	require.NoError(t, err)
	CheckOKStatus(t, resp)

	// Deny the loopback addresses the test client connects from.
	th.App.UpdateConfig(func(cfg *model.Config) {
		cfg.IPFilteringSettings.Rules = model.AllowedIPRanges{
			{CIDRBlock: "127.0.0.0/8", Enabled: true, Action: model.IPFilterActionDeny},
			{CIDRBlock: "::1", Enabled: true, Action: model.IPFilterActionDeny},
		}
	})

	_, resp, err = th.Client.GetMe(context.Background(), "")
	require.Error(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)

	// Disabling the rules lifts the filter without a restart.
	th.App.UpdateConfig(func(cfg *model.Config) {
		for i := range cfg.IPFilteringSettings.Rules {
			cfg.IPFilteringSettings.Rules[i].Enabled = false
		}
	})

	_, resp, err = th.Client.GetMe(context.Background(), "")
	require.NoError(t, err)
	CheckOKStatus(t, resp)
}

func TestGetMyIP(t *testing.T) {
	th := Setup(t).InitBasic(t)

	resp, _, err := th.Client.GetMyIP(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, resp.IP)
}
