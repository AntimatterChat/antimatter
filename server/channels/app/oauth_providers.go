// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"sync"

	"github.com/mattermost/mattermost/server/public/model"
	oauthgoogle "github.com/mattermost/mattermost/server/v8/channels/app/oauthproviders/google"
	oauthoffice365 "github.com/mattermost/mattermost/server/v8/channels/app/oauthproviders/office365"
	oauthopenid "github.com/mattermost/mattermost/server/v8/channels/app/oauthproviders/openid"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

// antimatterOAuthProviders returns the SSO providers Antimatter ships besides
// GitLab's (oauthproviders/gitlab registers itself from init(), as upstream).
func antimatterOAuthProviders() map[string]einterfaces.OAuthProvider {
	return map[string]einterfaces.OAuthProvider{
		model.ServiceGoogle:    oauthgoogle.New(nil),
		model.ServiceOffice365: oauthoffice365.New(nil),
		model.ServiceOpenid:    oauthopenid.New(nil),
	}
}

var registerOAuthProvidersOnce sync.Once

// registerOAuthProviders adds them to einterfaces' provider registry when the
// first server starts. The registry is global to the process, so they are
// registered once: a later server must not replace the mocks tests register.
func registerOAuthProviders() {
	registerOAuthProvidersOnce.Do(func() {
		for name, provider := range antimatterOAuthProviders() {
			einterfaces.RegisterOAuthProvider(name, provider)
		}
	})
}
