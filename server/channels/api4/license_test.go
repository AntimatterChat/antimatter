// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestGetOldClientLicense(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t)
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.SiteName = "Lumen Labs" })

	check := func(t *testing.T, license map[string]string) {
		t.Helper()
		require.Equal(t, "true", license["IsLicensed"])
		require.Equal(t, "true", license["LDAP"])
		require.Equal(t, "true", license["SAML"])
		require.Equal(t, "false", license["Cloud"])
		require.Equal(t, "false", license["MHPNS"])
		// The official apps' Enterprise Advanced features need a known SKU; "Licensed to" shows the site's name.
		require.Equal(t, "advanced", license["SkuShortName"])
		require.Equal(t, "Lumen Labs", license["Company"])
	}

	t.Run("anonymous", func(t *testing.T) {
		client := th.CreateClient()
		license, _, err := client.GetOldClientLicense(context.Background(), "")
		require.NoError(t, err)
		check(t, license)
	})

	t.Run("system admin", func(t *testing.T) {
		license, _, err := th.SystemAdminClient.GetOldClientLicense(context.Background(), "")
		require.NoError(t, err)
		check(t, license)
	})
}
