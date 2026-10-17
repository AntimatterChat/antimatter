// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/v8/channels/app/accesscontrol"
	"github.com/mattermost/mattermost/server/v8/channels/app/ldap"
	"github.com/mattermost/mattermost/server/v8/channels/app/saml"
	"github.com/mattermost/mattermost/server/v8/platform/services/searchengine/elasticsearch"
)

// TestPlatformServicesWired checks that the platform builds the Antimatter
// services it holds (they are no longer registered from init()).
func TestPlatformServicesWired(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t)
	ps := th.Service

	require.NotNil(t, ps.SearchEngine)
	assert.IsType(t, &elasticsearch.Engine{}, ps.SearchEngine.ElasticsearchEngine)
	assert.IsType(t, &ldap.Diagnostic{}, ps.LdapDiagnostic())
	assert.IsType(t, &saml.Diagnostic{}, ps.SamlDiagnostic())
	assert.IsType(t, &accesscontrol.Service{}, ps.pdpService)
}
