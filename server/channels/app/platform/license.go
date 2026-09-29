// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package platform

import "maps"

// compatClientLicense is served at /api/v4/license/client. This server has no
// license concept, but clients built against upstream (notably the official
// mobile and desktop apps) hide features such as SSO login unless the server
// reports them. It describes what this server supports; it is not a license,
// carries no SKU, and leaves out services this server does not provide
// (Cloud, the Mattermost-hosted push notification service).
var compatClientLicense = map[string]string{
	"IsLicensed":                "true",
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
	return maps.Clone(compatClientLicense)
}
