// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	oauthgoogle "github.com/mattermost/mattermost/server/v8/channels/app/oauthproviders/google"
	oauthoffice365 "github.com/mattermost/mattermost/server/v8/channels/app/oauthproviders/office365"
	oauthopenid "github.com/mattermost/mattermost/server/v8/channels/app/oauthproviders/openid"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

func TestAntimatterOAuthProviders(t *testing.T) {
	providers := antimatterOAuthProviders()
	require.Len(t, providers, 3)
	assert.IsType(t, &oauthgoogle.Provider{}, providers[model.ServiceGoogle])
	assert.IsType(t, &oauthoffice365.Provider{}, providers[model.ServiceOffice365])
	assert.IsType(t, &oauthopenid.Provider{}, providers[model.ServiceOpenid])

	// Starting a server registers them (other tests may have replaced one
	// with a mock since, hence only the presence check).
	registerOAuthProviders()
	for _, service := range []string{model.ServiceGoogle, model.ServiceOffice365, model.ServiceOpenid, model.ServiceGitlab} {
		assert.NotNil(t, einterfaces.GetOAuthProvider(service), service)
	}
}
