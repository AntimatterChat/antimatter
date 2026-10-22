// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Package auth holds the health rules for AD/LDAP, SAML and sessions.
package auth

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/platform/shared/healthcheck"
)

const (
	ldapConsolePath = "/admin_console/authentication/ldap"
	ldapDocsURL     = "https://antimatter.chat/docs/admins/authentication.html#sso"
	samlConsolePath = "/admin_console/authentication/saml"
	samlDocsURL     = "https://antimatter.chat/docs/admins/authentication.html#sso"
)

// ldapRule resolves while gate reports LDAP off. Antimatter's LDAP service is free, so no licence is read.
func ldapRule(gate func(*healthcheck.Snapshot) (bool, bool), check func(*healthcheck.Snapshot) healthcheck.Result) func(*healthcheck.Snapshot) []healthcheck.Result {
	return healthcheck.Gate(gate, healthcheck.ReasonConfigUnavailable, check)
}

// samlRule resolves while SamlSettings.Enable is off. Antimatter's SAML service is free, so no licence is read.
func samlRule(check func(*healthcheck.Snapshot) healthcheck.Result) func(*healthcheck.Snapshot) []healthcheck.Result {
	return healthcheck.Gate(samlEnabled, healthcheck.ReasonConfigUnavailable, check)
}

// ldapEnabled matches the server's own test before probing LDAP: sync-only installs count.
func ldapEnabled(s *healthcheck.Snapshot) (bool, bool) {
	enable, ok := s.ConfigBool(func(cfg *model.Config) *bool { return cfg.LdapSettings.Enable })
	if !ok {
		return false, false
	}
	if enable {
		return true, true
	}

	return ldapSyncEnabled(s)
}

func ldapSyncEnabled(s *healthcheck.Snapshot) (bool, bool) {
	return s.ConfigBool(func(cfg *model.Config) *bool { return cfg.LdapSettings.EnableSync })
}

func samlEnabled(s *healthcheck.Snapshot) (bool, bool) {
	return s.ConfigBool(func(cfg *model.Config) *bool { return cfg.SamlSettings.Enable })
}

func unknownConfig() healthcheck.Result {
	return healthcheck.Unknown(healthcheck.ReasonConfigUnavailable)
}
