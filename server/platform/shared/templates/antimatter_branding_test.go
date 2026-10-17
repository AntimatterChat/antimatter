// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package templates

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shippedTemplatesDir is server/templates. GetTemplateDirectory would find this package's own directory, which is also
// named templates.
func shippedTemplatesDir(t *testing.T) string {
	dir := filepath.Join("..", "..", "..", "templates")
	_, err := os.Stat(filepath.Join(dir, "welcome_body.html"))
	require.NoError(t, err, "server/templates is missing")
	return dir
}

// TestShippedTemplatesNameNoMattermostSite checks the email and error page templates the server ships: none may link
// to, or give an address at, a Mattermost site (UPSTREAM_MERGE_RULES.md, rule 10). CSS classes and data attributes
// named mattermost-* are identifiers and don't count.
func TestShippedTemplatesNameNoMattermostSite(t *testing.T) {
	dir := shippedTemplatesDir(t)

	files, err := filepath.Glob(filepath.Join(dir, "*.html"))
	require.NoError(t, err)
	partials, err := filepath.Glob(filepath.Join(dir, "partials", "*.html"))
	require.NoError(t, err)
	files = append(files, partials...)
	require.Greater(t, len(files), 10)

	mattermostSite := regexp.MustCompile(`(?i)\b([a-z0-9-]+\.)*mattermost\.(com|cloud|org)\b`)
	for _, file := range files {
		data, err := os.ReadFile(file)
		require.NoError(t, err)
		assert.Empty(t, mattermostSite.FindAllString(string(data), -1), "%s names a Mattermost site", filepath.Base(file))
	}
}

// TestNoCloudBillingTemplates: the Cloud welcome, renewal, arrears and payment emails belonged to Mattermost's Cloud
// billing, which Antimatter removed; nothing sends them, so they aren't shipped.
func TestNoCloudBillingTemplates(t *testing.T) {
	dir := shippedTemplatesDir(t)

	for _, pattern := range []string{"cloud_*", "payment_failed*"} {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		require.NoError(t, err)
		assert.Empty(t, matches)
	}
}
