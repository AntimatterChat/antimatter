// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestBuildRewriteSystemPrompt(t *testing.T) {
	basePrompt := model.RewriteSystemPrompt

	t.Run("uses_user_locale", func(t *testing.T) {
		prompt := buildRewriteSystemPrompt("en_CA")
		require.True(t, strings.HasPrefix(prompt, basePrompt))
		require.Contains(t, prompt, "User locale: en_CA.")
	})

	t.Run("returns_base_prompt_when_no_locale", func(t *testing.T) {
		prompt := buildRewriteSystemPrompt("")
		require.Equal(t, basePrompt, prompt)
	})
}

func TestRewriteMessage(t *testing.T) {
	t.Run("sets_structured_output_schema_on_the_model_request", func(t *testing.T) {
		th, llm, _ := setupAIFeatureModel(t, `{"rewritten_text":"Rewritten message"}`)
		ctx := th.Context.WithSession(&model.Session{UserId: th.BasicUser.Id})

		response, appErr := th.App.RewriteMessage(ctx, model.NewId(), "original message", model.RewriteActionImproveWriting, "", "")
		require.Nil(t, appErr)
		require.NotNil(t, response)
		assert.Equal(t, "Rewritten message", response.RewrittenText)
		require.Len(t, llm.calls(), 1)
		assert.Equal(t, "rewrite", llm.calls()[0].schemaName())
		format := llm.calls()[0].Body["response_format"].(map[string]any)["json_schema"].(map[string]any)
		schema, err := json.Marshal(format["schema"])
		require.NoError(t, err)
		assert.JSONEq(t, string(rewriteResponseJSONSchemaRaw), string(schema), "the rewrite's own schema")
		assert.Contains(t, llm.calls()[0].messages()[1]["content"], "original message")
	})
}
