// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestServerCategories(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.ManagedChannelCategories = true
	}).InitBasic(t)
	team := th.BasicTeam
	route := fmt.Sprintf("/teams/%s/server_categories", team.Id)

	get := func(t *testing.T, client *model.Client4) []*model.ServerCategory {
		t.Helper()
		resp, err := client.DoAPIGet(context.Background(), route, "")
		require.NoError(t, err)
		defer resp.Body.Close()
		var categories []*model.ServerCategory
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&categories))
		return categories
	}
	put := func(client *model.Client4, update *model.ServerCategoriesUpdate) (*http.Response, error) {
		body, err := json.Marshal(update)
		require.NoError(t, err)
		return client.DoAPIPut(context.Background(), route, string(body))
	}
	mappings := func(t *testing.T) map[string]string {
		t.Helper()
		// The mappings cover the caller's channels: the basic user's, who created them.
		resp, err := th.Client.DoAPIGet(context.Background(), fmt.Sprintf("/teams/%s/channels/managed_categories", team.Id), "")
		require.NoError(t, err)
		defer resp.Body.Close()
		var m map[string]string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		return m
	}
	file := func(client *model.Client4, channelID, name string) (*model.Response, error) {
		_, resp, err := client.PatchChannel(context.Background(), channelID, &model.ChannelPatch{ManagedCategoryName: &name})
		return resp, err
	}

	t.Run("only team admins change the categories", func(t *testing.T) {
		resp, err := put(th.Client, &model.ServerCategoriesUpdate{Categories: []*model.ServerCategory{{Name: "Rules"}}})
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("members see the categories in order", func(t *testing.T) {
		resp, err := put(th.SystemAdminClient, &model.ServerCategoriesUpdate{Categories: []*model.ServerCategory{{Name: "Rules", Locked: true}, {Name: "Community"}}})
		require.NoError(t, err)
		resp.Body.Close()

		categories := get(t, th.Client)
		require.Len(t, categories, 2)
		assert.Equal(t, "Rules", categories[0].Name)
		assert.True(t, categories[0].Locked)
		assert.Equal(t, "Community", categories[1].Name)
		assert.False(t, categories[1].Locked)
	})

	t.Run("names are unique, ignoring case", func(t *testing.T) {
		resp, err := put(th.SystemAdminClient, &model.ServerCategoriesUpdate{Categories: []*model.ServerCategory{{Name: "Rules"}, {Name: "rules"}}})
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("a channel admin files their channel in an open category, not a locked or new one", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)

		_, err := file(th.Client, channel.Id, "Community")
		require.NoError(t, err)
		assert.Equal(t, "Community", mappings(t)[channel.Id])

		resp, err := file(th.Client, channel.Id, "Rules")
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		resp, err = file(th.Client, channel.Id, "Brand new")
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		// Out of an open category, back into everyone's own.
		_, err = file(th.Client, channel.Id, "")
		require.NoError(t, err)
		_, filed := mappings(t)[channel.Id]
		assert.False(t, filed)
	})

	t.Run("a locked category keeps its channels from channel admins", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)
		_, err := file(th.SystemAdminClient, channel.Id, "Rules")
		require.NoError(t, err)

		resp, err := file(th.Client, channel.Id, "")
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		resp, err = file(th.Client, channel.Id, "Community")
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("a team admin filing under a new name adds it, open, at the end", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)
		_, err := file(th.SystemAdminClient, channel.Id, "Projects")
		require.NoError(t, err)

		categories := get(t, th.Client)
		require.Len(t, categories, 3)
		assert.Equal(t, "Projects", categories[2].Name)
		assert.False(t, categories[2].Locked)
	})

	t.Run("channels follow a renamed category and leave a removed one", func(t *testing.T) {
		renamed := th.CreatePublicChannel(t)
		_, err := file(th.SystemAdminClient, renamed.Id, "Community")
		require.NoError(t, err)
		removed := th.CreatePublicChannel(t)
		_, err = file(th.SystemAdminClient, removed.Id, "Projects")
		require.NoError(t, err)

		resp, err := put(th.SystemAdminClient, &model.ServerCategoriesUpdate{
			Categories: []*model.ServerCategory{{Name: "Rules", Locked: true}, {Name: "Open space"}},
			Renames:    map[string]string{"Community": "Open space"},
			Removed:    []string{"Projects"},
		})
		require.NoError(t, err)
		resp.Body.Close()

		m := mappings(t)
		assert.Equal(t, "Open space", m[renamed.Id])
		_, filed := m[removed.Id]
		assert.False(t, filed)
	})

	t.Run("members create channels in open categories only", func(t *testing.T) {
		create := func(category string) (*model.Channel, *model.Response, error) {
			return th.Client.CreateChannel(context.Background(), &model.Channel{
				DisplayName:         "Filed " + category,
				Name:                GenerateTestChannelName(),
				Type:                model.ChannelTypeOpen,
				TeamId:              team.Id,
				ManagedCategoryName: category,
			})
		}

		created, _, err := create("open space")
		require.NoError(t, err)
		assert.Equal(t, "Open space", created.ManagedCategoryName, "the list's spelling wins")

		_, resp, err := create("Rules")
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		_, resp, err = create("Somewhere new")
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("a list made from an older one is refused, and changes nothing", func(t *testing.T) {
		before := get(t, th.Client)
		resp, err := put(th.SystemAdminClient, &model.ServerCategoriesUpdate{Categories: []*model.ServerCategory{{Name: "Rules"}}})
		require.Error(t, err)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
		assert.Equal(t, before, get(t, th.Client))
	})

	t.Run("names match ignoring case when channels follow a rename", func(t *testing.T) {
		shouty := th.CreatePublicChannel(t)
		require.Nil(t, th.App.SetChannelManagedCategory(th.Context, shouty.Id, "OPEN SPACE"))

		resp, err := put(th.SystemAdminClient, &model.ServerCategoriesUpdate{
			Categories: []*model.ServerCategory{{Name: "Rules", Locked: true}, {Name: "Hangout"}},
			Renames:    map[string]string{"Open space": "Hangout"},
		})
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, "Hangout", mappings(t)[shouty.Id])
	})

	t.Run("a category filed under but missing from the list can be renamed and removed", func(t *testing.T) {
		kept := th.CreatePublicChannel(t)
		require.Nil(t, th.App.SetChannelManagedCategory(th.Context, kept.Id, "Legacy"))
		gone := th.CreatePublicChannel(t)
		require.Nil(t, th.App.SetChannelManagedCategory(th.Context, gone.Id, "Old stuff"))

		resp, err := put(th.SystemAdminClient, &model.ServerCategoriesUpdate{
			Categories: []*model.ServerCategory{{Name: "Rules", Locked: true}, {Name: "Hangout"}, {Name: "Archive"}},
			Renames:    map[string]string{"legacy": "Archive"},
			Removed:    []string{"old STUFF"},
		})
		require.NoError(t, err)
		resp.Body.Close()

		m := mappings(t)
		assert.Equal(t, "Archive", m[kept.Id])
		_, filed := m[gone.Id]
		assert.False(t, filed)
	})

	t.Run("a channel that can't be made adds no category", func(t *testing.T) {
		taken := th.CreatePublicChannel(t)
		_, _, err := th.SystemAdminClient.CreateChannel(context.Background(), &model.Channel{
			DisplayName: "Clash", Name: taken.Name, Type: model.ChannelTypeOpen, TeamId: team.Id, ManagedCategoryName: "Ghost",
		})
		require.Error(t, err)
		for _, c := range get(t, th.Client) {
			assert.NotEqual(t, "Ghost", c.Name)
		}
	})

	t.Run("channels aren't filed through the properties API", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)
		body := []map[string]any{{"field_id": model.NewId(), "value": "Rules"}}
		resp, err := th.Client.DoAPIPatchJSON(context.Background(), "/properties/groups/"+model.ManagedCategoryPropertyGroupName+"/channel/values/"+channel.Id, body)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
}
