// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package platform

import (
	"maps"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

// compatClientLicense is served at /api/v4/license/client. This server has no
// license concept, but clients built against upstream (notably the official
// mobile and desktop apps) hide features such as SSO login unless the server
// reports them. It describes what this server supports; it is not a license,
// and leaves out services this server does not provide (Cloud, the
// Mattermost-hosted push notification service). Its SKU is the highest tier
// (Enterprise Advanced) so those apps turn on every client feature it gates,
// all of which this server supports, and the company is the site's name (see
// clientLicense). Plugins get no license at all (PluginAPI.GetLicense).
var compatClientLicense = map[string]string{
	"IsLicensed":                "true",
	"SkuShortName":              model.LicenseShortSkuEnterpriseAdvanced,
	"LDAP":                      "true",
	"LDAPGroups":                "true",
	"MFA":                       "true",
	"SAML":                      "true",
	"Cluster":                   "true",
	"Metrics":                   "true",
	"GoogleOAuth":               "true",
	"Office365OAuth":            "true",
	"OpenId":                    "true",
	"Compliance":                "true",
	"Announcement":              "true",
	"Elasticsearch":             "true",
	"DataRetention":             "true",
	"IDLoadedPushNotifications": "true",
	"EmailNotificationContents": "true",
	"MessageExport":             "true",
	"CustomPermissionsSchemes":  "true",
	"GuestAccounts":             "true",
	"GuestAccountsPermissions":  "true",
	"CustomTermsOfService":      "true",
	"LockTeammateNameDisplay":   "true",
	"SharedChannels":            "true",
	"RemoteClusterService":      "true",
	"OutgoingOAuthConnections":  "true",
	"MHPNS":                     "false",
	"Cloud":                     "false",
	"IsTrial":                   "false",
	"IsGovSku":                  "false",
	"IsNonProduction":           "false",
}

// ClientLicense returns the feature map served to clients that still read
// /api/v4/license/client.
func (ps *PlatformService) ClientLicense() map[string]string {
	return clientLicense(ps.Config().TeamSettings.SiteName)
}

// clientLicense is compatClientLicense with the site's name as the company the
// official apps show it "licensed to", Antimatter when it has none.
func clientLicense(siteName *string) map[string]string {
	license := maps.Clone(compatClientLicense)
	license["Company"] = model.TeamSettingsDefaultSiteName
	if siteName != nil && strings.TrimSpace(*siteName) != "" {
		license["Company"] = strings.TrimSpace(*siteName)
	}
	return license
}
