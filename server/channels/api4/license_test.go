// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"io"
	"net/http"
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

// Mattermost's mobile app asks for the licence load on its About screen; a 404 there is logged as an error.
func TestGetLicenseLoadMetric(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	t.Run("logged in", func(t *testing.T) {
		load, resp, err := th.Client.GetLicenseLoadMetric(context.Background())
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.Equal(t, map[string]int{"load": 0}, load)

		raw, err := th.Client.DoAPIGet(context.Background(), "/license/load_metric", "")
		require.NoError(t, err)
		defer raw.Body.Close()
		body, err := io.ReadAll(raw.Body)
		require.NoError(t, err)
		require.JSONEq(t, `{"load":0}`, string(body))
		require.Equal(t, "application/json", raw.Header.Get("Content-Type"))
	})

	t.Run("logged out", func(t *testing.T) {
		_, resp, err := th.CreateClient().GetLicenseLoadMetric(context.Background())
		require.Error(t, err)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}
