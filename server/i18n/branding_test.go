// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package i18n

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The server's user-visible strings say Antimatter, never Mattermost, and link to Antimatter rather than to
// mattermost.com (UPSTREAM_MERGE_RULES.md, rules 10 and 14). The repository-wide branding check in CI covers the other
// files; this test runs with `go test` on the catalogue itself.

// brandingAllowed are the strings that may name Mattermost: they speak of Mattermost, Inc. or its services as such,
// or name an identifier that keeps its name.
var brandingAllowed = map[string]string{
	"health.rule.push_test_proxy.title":                "warns that a configured push service is Mattermost's",
	"health.rule.push_test_proxy.message":              "explains that Mattermost's push service only serves Mattermost's apps",
	"mattermost.bulletin.subject":                      "security bulletins from Mattermost, which Antimatter is based on",
	"model.oauth.is_valid.mattermost_app_id.app_error": "MattermostAppID is a field name",
}

// brandingToFix are the strings that still name Mattermost or link to mattermost.com and should not: each needs
// rewording (and its 37 translations) or its link pointed at https://antimatter.chat/docs/ or dropped. Remove an entry
// once it's fixed; the test fails until you do, so this list only shrinks. It is empty: keep it that way.
var brandingToFix = []string{}

var mattermostWord = regexp.MustCompile(`(?i)mattermost`)

type catalogueEntry struct {
	ID          string          `json:"id"`
	Translation json.RawMessage `json:"translation"`
}

func TestEnglishStringsSayAntimatter(t *testing.T) {
	var entries []catalogueEntry
	require.NoError(t, json.Unmarshal(English, &entries))
	require.Greater(t, len(entries), 1000, "en.json looks truncated")

	var found []string
	for _, e := range entries {
		// Plural forms are objects; matching the raw JSON covers both forms.
		if !mattermostWord.Match(e.Translation) {
			continue
		}
		if _, ok := brandingAllowed[e.ID]; ok {
			continue
		}
		found = append(found, e.ID)
		assert.Contains(t, brandingToFix, e.ID,
			"%s names Mattermost: say Antimatter, link to https://antimatter.chat, or add it to brandingAllowed with the reason: %s",
			e.ID, string(e.Translation))
	}

	for _, id := range brandingToFix {
		assert.Contains(t, found, id, "%s no longer names Mattermost: remove it from brandingToFix", id)
	}
	for id := range brandingAllowed {
		assert.True(t, slices.ContainsFunc(entries, func(e catalogueEntry) bool { return e.ID == id }),
			"%s is in brandingAllowed but no longer in en.json", id)
	}
}

// TestEnglishStringsLinkNoMattermostSite catches links even in allowed strings: none of them may send a user to a
// Mattermost site, apart from the ones still listed to fix.
func TestEnglishStringsLinkNoMattermostSite(t *testing.T) {
	var entries []catalogueEntry
	require.NoError(t, json.Unmarshal(English, &entries))
	link := regexp.MustCompile(`(?i)https?://([a-z0-9.-]*\.)?mattermost\.(com|cloud)\b`)
	for _, e := range entries {
		if link.Match(e.Translation) && !slices.Contains(brandingToFix, e.ID) {
			t.Errorf("%s links to a Mattermost site: %s", e.ID, strings.TrimSpace(string(e.Translation)))
		}
	}
}

// TestTranslationsDoNotBringMattermostBack checks the 40 other catalogues: where the English string doesn't name
// Mattermost, its translation must not either (stale upstream translations, UPSTREAM_MERGE_RULES.md rule 26).
func TestTranslationsDoNotBringMattermostBack(t *testing.T) {
	var english []catalogueEntry
	require.NoError(t, json.Unmarshal(English, &english))
	rebranded := make(map[string]bool, len(english))
	for _, e := range english {
		rebranded[e.ID] = !mattermostWord.Match(e.Translation)
	}

	files, err := filepath.Glob("*.json")
	require.NoError(t, err)
	require.Greater(t, len(files), 30, "the translations are missing")
	for _, file := range files {
		if file == "en.json" {
			continue
		}
		data, err := os.ReadFile(file)
		require.NoError(t, err)
		var entries []catalogueEntry
		require.NoError(t, json.Unmarshal(data, &entries), file)
		for _, e := range entries {
			if rebranded[e.ID] && mattermostWord.Match(e.Translation) {
				t.Errorf("%s: %s says Mattermost where English doesn't: %s", file, e.ID, string(e.Translation))
			}
		}
	}
}
