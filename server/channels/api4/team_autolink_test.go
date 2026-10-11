// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestTeamAutolinks(t *testing.T) {
	th := Setup(t).InitBasic(t)
	ctx := context.Background()

	jira := `{"links":[{"name":"Jira","pattern":"LUM-(\\d+)","template":"[$0](https://jira.example/browse/$0)"}]}`
	_, resp, err := th.Client.PatchTeam(ctx, th.BasicTeam.Id, &model.TeamPatch{Autolinks: model.NewPointer(jira)})
	require.Error(t, err, "members don't set autolinks")
	CheckForbiddenStatus(t, resp)

	_, resp, err = th.SystemAdminClient.PatchTeam(ctx, th.BasicTeam.Id, &model.TeamPatch{Autolinks: model.NewPointer(`{"links":[{"pattern":"(","template":"x"}]}`)})
	require.Error(t, err, "invalid patterns are refused")
	CheckBadRequestStatus(t, resp)

	team, _, err := th.SystemAdminClient.PatchTeam(ctx, th.BasicTeam.Id, &model.TeamPatch{Autolinks: model.NewPointer(jira)})
	require.NoError(t, err)
	assert.Equal(t, jira, team.Autolinks)

	t.Run("new messages are linked", func(t *testing.T) {
		post, _, err := th.Client.CreatePost(ctx, &model.Post{ChannelId: th.BasicChannel.Id, Message: "Fixed LUM-12, see `LUM-13`"})
		require.NoError(t, err)
		assert.Equal(t, "Fixed [LUM-12](https://jira.example/browse/LUM-12), see `LUM-13`", post.Message)

		edited, _, err := th.Client.PatchPost(ctx, post.Id, &model.PostPatch{Message: model.NewPointer("Now LUM-14")})
		require.NoError(t, err)
		assert.Equal(t, "Now LUM-14", edited.Message, "edits are left alone unless the team says so")
	})

	t.Run("edits are linked when the team says so", func(t *testing.T) {
		onEdit := `{"links":[{"pattern":"LUM-(\\d+)","template":"[$0](https://jira.example/browse/$0)"}],"on_edit":true}`
		_, _, err := th.SystemAdminClient.PatchTeam(ctx, th.BasicTeam.Id, &model.TeamPatch{Autolinks: model.NewPointer(onEdit)})
		require.NoError(t, err)
		post, _, err := th.Client.CreatePost(ctx, &model.Post{ChannelId: th.BasicChannel.Id, Message: "nothing yet"})
		require.NoError(t, err)
		edited, _, err := th.Client.PatchPost(ctx, post.Id, &model.PostPatch{Message: model.NewPointer("LUM-15")})
		require.NoError(t, err)
		assert.Equal(t, "[LUM-15](https://jira.example/browse/LUM-15)", edited.Message)
	})

	t.Run("direct messages have no team autolinks", func(t *testing.T) {
		dm, _, err := th.Client.CreateDirectChannel(ctx, th.BasicUser.Id, th.BasicUser2.Id)
		require.NoError(t, err)
		post, _, err := th.Client.CreatePost(ctx, &model.Post{ChannelId: dm.Id, Message: "LUM-16"})
		require.NoError(t, err)
		assert.Equal(t, "LUM-16", post.Message)
	})

	t.Run("preview", func(t *testing.T) {
		body := `{"autolinks":{"links":[{"pattern":"PR(\\d+)","template":"[#$1](https://git.example/pull/$1)","word_match":true}]},"text":"merged PR42"}`
		res, err := th.Client.DoAPIPost(ctx, "/teams/"+th.BasicTeam.Id+"/autolinks/preview", body)
		require.Error(t, err)
		if res != nil {
			assert.Equal(t, http.StatusForbidden, res.StatusCode)
		}

		res, err = th.SystemAdminClient.DoAPIPost(ctx, "/teams/"+th.BasicTeam.Id+"/autolinks/preview", body)
		require.NoError(t, err)
		defer res.Body.Close()
		var out map[string]string
		require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
		assert.Equal(t, "merged [#42](https://git.example/pull/42)", out["text"])

		_, err = th.SystemAdminClient.DoAPIPost(ctx, "/teams/"+th.BasicTeam.Id+"/autolinks/preview", `{"autolinks":{"links":[{"pattern":"(","template":"x"}]},"text":"x"}`)
		require.Error(t, err, "invalid rules are reported")
	})
}
