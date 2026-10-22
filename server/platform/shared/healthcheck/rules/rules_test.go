// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package rules

import (
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/v8/platform/shared/healthcheck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltinRegistry(t *testing.T) {
	registry := healthcheck.Builtin()
	require.NoError(t, registry.Validate())

	for _, code := range []string{"PUSH_EMPTY_URL", "PUSH_BAD_SCHEME", "PUSH_TEST_PROXY", "SITE_URL_EMPTY", "SITE_URL_HTTP", "CLUSTER_SINGLE_NODE", "CLUSTER_VERSION_SKEW", "RATELIMIT_OFF_LARGE"} {
		_, ok := registry.Get(code)
		assert.True(t, ok, code)
	}
}

func TestBuiltinRulesOnCloud(t *testing.T) {
	var cloud []string
	for _, rule := range healthcheck.Builtin().Rules() {
		if rule.AppliesTo(healthcheck.Deployment{IsCloud: true}) {
			cloud = append(cloud, rule.Code)
		}
	}

	assert.ElementsMatch(t, []string{
		"PLUGIN_NOT_RUNNING",
		"PUSH_BAD_SCHEME",
		"PUSH_EMPTY_URL",
		"PUSH_ID_ONLY",
		"PUSH_TEST_PROXY",
	}, cloud)
}

// TestBuiltinRulesLinkAntimatterDocs: the "learn more" link of every rule leads to Antimatter's documentation or
// repositories, never to a Mattermost site (UPSTREAM_MERGE_RULES.md, rule 10).
func TestBuiltinRulesLinkAntimatterDocs(t *testing.T) {
	for _, rule := range healthcheck.Builtin().Rules() {
		if rule.DocsURL == "" {
			continue
		}
		assert.True(t, strings.HasPrefix(rule.DocsURL, "https://antimatter.chat/docs/") ||
			strings.HasPrefix(rule.DocsURL, "https://github.com/AntimatterChat/"), "%s links to %s", rule.Code, rule.DocsURL)
	}
}
