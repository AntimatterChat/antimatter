// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package platform

import (
	"github.com/mattermost/mattermost/server/v8/einterfaces"
	"github.com/mattermost/mattermost/server/v8/platform/services/searchengine"
)

var elasticsearchInterface func(*PlatformService) searchengine.SearchEngineInterface

func RegisterElasticsearchInterface(f func(*PlatformService) searchengine.SearchEngineInterface) {
	elasticsearchInterface = f
}

var ldapDiagnosticInterface func(*PlatformService) einterfaces.LdapDiagnosticInterface

func RegisterLdapDiagnosticInterface(f func(*PlatformService) einterfaces.LdapDiagnosticInterface) {
	ldapDiagnosticInterface = f
}

var samlDiagnosticInterface func(*PlatformService) einterfaces.SamlDiagnosticInterface

func RegisterSamlDiagnosticInterface(f func(*PlatformService) einterfaces.SamlDiagnosticInterface) {
	samlDiagnosticInterface = f
}

var accessControlServiceInterface func(*PlatformService) einterfaces.AccessControlServiceInterface

func RegisterAccessControlServiceInterface(f func(*PlatformService) einterfaces.AccessControlServiceInterface) {
	accessControlServiceInterface = f
}
