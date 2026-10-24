// Copyright (c) 2016-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// extractCommand builds a command carrying the same flags init() registers on
// ExtractCmd, pointed at dir and with the optional source trees left absent.
func extractCommand(dir string) *cobra.Command {
	cmd := &cobra.Command{RunE: extractCmdF}
	cmd.Flags().Bool("skip-dynamic", true, "")
	cmd.Flags().String("portal-dir", "", "")
	cmd.Flags().String("enterprise-dir", filepath.Join(dir, "absent-enterprise"), "")
	cmd.Flags().String("server-dir", dir, "")
	cmd.Flags().String("model-dir", filepath.Join(dir, "absent-model"), "")
	cmd.Flags().String("plugin-dir", filepath.Join(dir, "absent-plugin"), "")
	cmd.Flags().Bool("contributor", false, "")

	return cmd
}

// The description is the whole reason a person edits en.json by hand, so
// extraction has to carry it across rather than rewrite it away. An id the
// source no longer references goes, and a newly referenced one arrives with an
// empty description to fill in.
func TestExtractPreservesDescriptions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "i18n"), 0700))

	const en = `[
  {
    "id": "dropped.id",
    "translation": "Gone",
    "description": "No longer referenced by any source file."
  },
  {
    "id": "kept.id",
    "translation": "Kept",
    "description": "Where this string appears and how it is used."
  }
]
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "i18n", "en.json"), []byte(en), 0600))

	const src = `package fixture

func f() {
	i18n.T("kept.id")
	i18n.T("added.id")
}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fixture.go"), []byte(src), 0600))

	require.NoError(t, extractCmdF(extractCommand(dir), nil))

	raw, err := os.ReadFile(filepath.Join(dir, "i18n", "en.json"))
	require.NoError(t, err)

	var got []Translation
	require.NoError(t, json.Unmarshal(raw, &got))

	descriptions := map[string]string{}
	for _, translation := range got {
		descriptions[translation.Id] = translation.Description
	}

	assert.Equal(t, map[string]string{
		"added.id": "",
		"kept.id":  "Where this string appears and how it is used.",
	}, descriptions)
}

// A description is metadata for translators, so it belongs in en.json and
// nowhere else. Emitting it into a locale file would hand Weblate a field it
// would then have to round-trip.
func TestExtractWritesDescriptionForEveryID(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "i18n"), 0700))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "i18n", "en.json"), []byte("[]\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fixture.go"), []byte("package fixture\n\nfunc f() { i18n.T(\"added.id\") }\n"), 0600))

	require.NoError(t, extractCmdF(extractCommand(dir), nil))

	raw, err := os.ReadFile(filepath.Join(dir, "i18n", "en.json"))
	require.NoError(t, err)

	assert.JSONEq(t, `[{"id":"added.id","translation":"","description":""}]`, string(raw))
}

// Several Antimatter error-builders pass a translation id through a small local
// helper (one id argument per call site, built into an AppError once) rather than
// a literal id at the NewAppError call itself. extractByFuncName already special-
// cases exactly this for a helper named newAppError, matching the id-only shape of
// model.NewAppError's own id argument — see UploadFileTask.newAppError in
// channels/app/file.go for the real-world precedent. A helper under any other
// name is invisible to the extractor: its call sites' literal ids are dropped as
// unused on the next `make i18n-extract`, which is what happened to roughly
// 30 Antimatter ids (model.ai_agent_task.*, model.ai_mcp_server.*,
// app.ai.agent.task.*, ent.intune.*, ent.outgoing_oauth_connections.*,
// ent.autotranslation.*) before those helpers were renamed to newAppError.
func TestExtractRecognizesLocalNewAppErrorHelper(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "i18n"), 0700))

	const en = `[
  {
    "id": "fixture.helper.one.app_error",
    "translation": "First case.",
    "description": "Reached through the local newAppError helper, first call site."
  },
  {
    "id": "fixture.helper.two.app_error",
    "translation": "Second case.",
    "description": "Reached through the local newAppError helper, second call site."
  }
]
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "i18n", "en.json"), []byte(en), 0600))

	// Mirrors the shape used across the server (e.g. channels/app/intune, …/
	// outgoingoauth, …/autotranslation, public/model/ai_agent_task.go,
	// public/model/ai_mcp.go): a package-level or closure helper literally named
	// newAppError, taking the id as its own first parameter, wrapping
	// model.NewAppError once for every call site.
	const src = `package fixture

func newAppError(id string, status int) *model.AppError {
	return model.NewAppError("fixture", id, nil, "", status)
}

func f() {
	newAppError("fixture.helper.one.app_error", http.StatusBadRequest)
	newAppError("fixture.helper.two.app_error", http.StatusNotFound)
}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fixture.go"), []byte(src), 0600))

	require.NoError(t, extractCmdF(extractCommand(dir), nil))

	raw, err := os.ReadFile(filepath.Join(dir, "i18n", "en.json"))
	require.NoError(t, err)

	var got []Translation
	require.NoError(t, json.Unmarshal(raw, &got))

	ids := map[string]bool{}
	for _, translation := range got {
		ids[translation.Id] = true
	}

	assert.True(t, ids["fixture.helper.one.app_error"], "id reached only through the local newAppError helper was dropped")
	assert.True(t, ids["fixture.helper.two.app_error"], "id reached only through the local newAppError helper was dropped")
}
