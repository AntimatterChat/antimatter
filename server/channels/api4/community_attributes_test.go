// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// callJSON makes an API call and decodes its answer into out (when given), returning the status.
func callJSON(t *testing.T, client *model.Client4, method, url string, body, out any) int {
	t.Helper()
	data := ""
	if body != nil {
		buf, err := json.Marshal(body)
		require.NoError(t, err)
		data = string(buf)
	}
	resp, _ := client.DoAPIRequestWithHeaders(context.Background(), method, url, data, nil)
	require.NotNil(t, resp)
	defer resp.Body.Close()
	if out != nil && resp.StatusCode < 300 {
		raw, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, out))
	}
	return resp.StatusCode
}

// Community attributes are the user attributes a team's admins define for its members.
func TestCommunityAttributes(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	team := th.BasicTeam

	loggedIn := func(t *testing.T, user *model.User) *model.Client4 {
		t.Helper()
		client := th.CreateClient()
		_, _, err := client.Login(context.Background(), user.Email, user.Password)
		require.NoError(t, err)
		return client
	}
	admin := th.CreateUser(t)
	th.LinkUserToTeam(t, admin, team)
	th.UpdateUserToTeamAdmin(t, admin, team)
	adminClient := loggedIn(t, admin)
	member, member2 := th.BasicUser, th.BasicUser2
	memberClient := th.Client
	member2Client := loggedIn(t, member2)
	outsider := th.CreateUser(t)
	outsiderClient := loggedIn(t, outsider)

	fieldsURL := "/teams/" + team.Id + "/user_attributes/fields"
	valuesURL := func(userID string) string { return "/teams/" + team.Id + "/members/" + userID + "/user_attributes" }
	searchURL := "/teams/" + team.Id + "/user_attributes/values/search"

	create := func(t *testing.T, field map[string]any) *model.PropertyField {
		t.Helper()
		var created model.PropertyField
		require.Equal(t, http.StatusCreated, callJSON(t, adminClient, http.MethodPost, fieldsURL, field, &created))
		return &created
	}
	values := func(t *testing.T, client *model.Client4, userID string) map[string]any {
		t.Helper()
		var got map[string]any
		require.Equal(t, http.StatusOK, callJSON(t, client, http.MethodGet, valuesURL(userID), nil, &got))
		return got
	}

	t.Run("team admins define attributes, members only read them", func(t *testing.T) {
		require.Equal(t, http.StatusForbidden, callJSON(t, memberClient, http.MethodPost, fieldsURL, map[string]any{"name": "Rebels", "type": "text"}, nil))

		field := create(t, map[string]any{"name": "  Location ", "type": "text", "attrs": map[string]any{"bogus": 1}})
		assert.Equal(t, "Location", field.Name)
		assert.Equal(t, model.PropertyFieldObjectTypeUser, field.ObjectType)
		assert.Equal(t, "team", field.TargetType)
		assert.Equal(t, team.Id, field.TargetID)
		assert.Equal(t, "when_set", field.Attrs["visibility"])
		assert.NotContains(t, field.Attrs, "bogus")

		var listed []*model.PropertyField
		require.Equal(t, http.StatusOK, callJSON(t, memberClient, http.MethodGet, fieldsURL, nil, &listed))
		assert.Contains(t, fieldNames(listed), "Location")
		require.Equal(t, http.StatusForbidden, callJSON(t, outsiderClient, http.MethodGet, fieldsURL, nil, nil))

		// System admins administer every team.
		var made model.PropertyField
		require.Equal(t, http.StatusCreated, callJSON(t, th.SystemAdminClient, http.MethodPost, fieldsURL, map[string]any{"name": "Badge number", "type": "text"}, &made))
	})

	t.Run("an attribute is checked", func(t *testing.T) {
		for name, field := range map[string]map[string]any{
			"no name":            {"name": " ", "type": "text"},
			"a long name":        {"name": strings.Repeat("x", 65), "type": "text"},
			"an unknown type":    {"name": "Rank", "type": "rank"},
			"a choice of none":   {"name": "Shift", "type": "select"},
			"twice the same":     {"name": "Shift", "type": "select", "attrs": map[string]any{"options": []any{map[string]any{"name": "Day"}, map[string]any{"name": "day"}}}},
			"a wrong visibility": {"name": "Shift", "type": "text", "attrs": map[string]any{"visibility": "nobody"}},
			"a wrong value type": {"name": "Shift", "type": "text", "attrs": map[string]any{"value_type": "fax"}},
		} {
			assert.Equal(t, http.StatusBadRequest, callJSON(t, adminClient, http.MethodPost, fieldsURL, field, nil), name)
		}
	})

	t.Run("members set their own values, admins anyone's", func(t *testing.T) {
		company := create(t, map[string]any{"name": "Company", "type": "select", "attrs": map[string]any{
			"member_list": true,
			"options":     []any{map[string]any{"name": "Lumen"}, map[string]any{"name": "Acme Subcontracting"}},
		}})
		options := company.Attrs["options"].([]any)
		require.Len(t, options, 2)
		acme := options[1].(map[string]any)["id"].(string)
		require.True(t, model.IsValidId(acme))
		assert.Equal(t, true, company.Attrs["member_list"])
		badge := create(t, map[string]any{"name": "Badge", "type": "text", "attrs": map[string]any{"managed": "admin"}})

		require.Equal(t, http.StatusOK, callJSON(t, memberClient, http.MethodPatch, valuesURL(member.Id), map[string]any{company.ID: acme}, nil))
		assert.Equal(t, acme, values(t, member2Client, member.Id)[company.ID])

		require.Equal(t, http.StatusForbidden, callJSON(t, memberClient, http.MethodPatch, valuesURL(member.Id), map[string]any{badge.ID: "B-12"}, nil), "admin-managed")
		require.Equal(t, http.StatusForbidden, callJSON(t, memberClient, http.MethodPatch, valuesURL(member2.Id), map[string]any{company.ID: acme}, nil), "someone else's")
		require.Equal(t, http.StatusOK, callJSON(t, adminClient, http.MethodPatch, valuesURL(member.Id), map[string]any{badge.ID: "B-12"}, nil))
		assert.Equal(t, "B-12", values(t, memberClient, member.Id)[badge.ID])

		// An empty value clears one.
		require.Equal(t, http.StatusOK, callJSON(t, memberClient, http.MethodPatch, valuesURL(member.Id), map[string]any{company.ID: nil}, nil))
		assert.Equal(t, "", values(t, memberClient, member.Id)[company.ID])
	})

	t.Run("values are checked against their attribute", func(t *testing.T) {
		email := create(t, map[string]any{"name": "Work email", "type": "text", "attrs": map[string]any{"value_type": "email"}})
		since := create(t, map[string]any{"name": "On site since", "type": "date"})
		shifts := create(t, map[string]any{"name": "Shifts", "type": "multiselect", "attrs": map[string]any{"options": []any{map[string]any{"name": "Day"}, map[string]any{"name": "Night"}}}})
		manager := create(t, map[string]any{"name": "Manager", "type": "user"})
		buddies := create(t, map[string]any{"name": "Buddies", "type": "multiuser"})
		day := shifts.Attrs["options"].([]any)[0].(map[string]any)["id"].(string)

		for name, update := range map[string]map[string]any{
			"not an email":  {email.ID: "nope"},
			"not a date":    {since.ID: "13/10/2026"},
			"no such shift": {shifts.ID: []string{model.NewId()}},
			"a string list": {shifts.ID: "Day"},
			"no such user":  {manager.ID: model.NewId()},
			"no such field": {model.NewId(): "x"},
			"too long":      {email.ID: strings.Repeat("x", 80)},
		} {
			status := callJSON(t, memberClient, http.MethodPatch, valuesURL(member.Id), update, nil)
			assert.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound}, status, name)
		}

		var set map[string]any
		require.Equal(t, http.StatusOK, callJSON(t, memberClient, http.MethodPatch, valuesURL(member.Id), map[string]any{
			email.ID:   "dev@example.com",
			since.ID:   "2026-10-13",
			shifts.ID:  []string{day, day},
			manager.ID: admin.Id,
			buddies.ID: []string{member2.Id, admin.Id},
		}, &set))
		assert.Equal(t, []any{day}, set[shifts.ID], "deduplicated")
		assert.Equal(t, "2026-10-13", set[since.ID])
	})

	t.Run("hidden and admin-only values are for the admins and the member only", func(t *testing.T) {
		clearance := create(t, map[string]any{"name": "Clearance", "type": "text", "attrs": map[string]any{"visibility": "admin"}})
		internal := create(t, map[string]any{"name": "Internal ID", "type": "text", "attrs": map[string]any{"visibility": "hidden"}})
		public := create(t, map[string]any{"name": "Desk", "type": "text"})
		require.Equal(t, http.StatusOK, callJSON(t, adminClient, http.MethodPatch, valuesURL(member.Id), map[string]any{clearance.ID: "level 2", internal.ID: "X-1", public.ID: "4B"}, nil))

		seen := values(t, member2Client, member.Id)
		assert.NotContains(t, seen, clearance.ID)
		assert.NotContains(t, seen, internal.ID)
		assert.Equal(t, "4B", seen[public.ID])

		for _, client := range []*model.Client4{memberClient, adminClient, th.SystemAdminClient} {
			seen := values(t, client, member.Id)
			assert.Equal(t, "level 2", seen[clearance.ID])
			assert.Equal(t, "X-1", seen[internal.ID])
		}

		var bulk map[string]map[string]any
		require.Equal(t, http.StatusOK, callJSON(t, member2Client, http.MethodPost, searchURL, map[string]any{"user_ids": []string{member.Id, outsider.Id}}, &bulk))
		assert.NotContains(t, bulk[member.Id], clearance.ID)
		assert.Equal(t, "4B", bulk[member.Id][public.ID])
		assert.NotContains(t, bulk, outsider.Id, "not in the team")
	})

	t.Run("only the team's members, in their team", func(t *testing.T) {
		desk := create(t, map[string]any{"name": "Desk number", "type": "text"})
		require.Equal(t, http.StatusNotFound, callJSON(t, adminClient, http.MethodPatch, valuesURL(outsider.Id), map[string]any{desk.ID: "1"}, nil))
		require.Equal(t, http.StatusForbidden, callJSON(t, outsiderClient, http.MethodGet, valuesURL(member.Id), nil, nil))
		require.Equal(t, http.StatusForbidden, callJSON(t, outsiderClient, http.MethodPost, searchURL, map[string]any{"user_ids": []string{member.Id}}, nil))

		// Another team's admin can't reach this team's attribute through their own team.
		other := th.CreateTeam(t)
		th.LinkUserToTeam(t, admin, other)
		th.UpdateUserToTeamAdmin(t, admin, other)
		require.Equal(t, http.StatusNotFound, callJSON(t, adminClient, http.MethodPatch, "/teams/"+other.Id+"/user_attributes/fields/"+desk.ID, map[string]any{"name": "Mine"}, nil))
		require.Equal(t, http.StatusNotFound, callJSON(t, adminClient, http.MethodDelete, "/teams/"+other.Id+"/user_attributes/fields/"+desk.ID, nil, nil))
	})

	t.Run("the generic properties API doesn't serve community attributes", func(t *testing.T) {
		require.Equal(t, http.StatusNotFound, callJSON(t, memberClient, http.MethodGet, "/properties/groups/"+model.CommunityAttributesPropertyGroupName+"/user/values/"+member.Id, nil, nil))
	})

	t.Run("an attribute is renamed, keeps its type, and is deleted", func(t *testing.T) {
		site := create(t, map[string]any{"name": "Site", "type": "text"})
		require.Equal(t, http.StatusOK, callJSON(t, memberClient, http.MethodPatch, valuesURL(member.Id), map[string]any{site.ID: "Paris"}, nil))

		var patched model.PropertyField
		require.Equal(t, http.StatusOK, callJSON(t, adminClient, http.MethodPatch, fieldsURL+"/"+site.ID, map[string]any{"name": "Work site", "attrs": map[string]any{"visibility": "always"}}, &patched))
		assert.Equal(t, "Work site", patched.Name)
		assert.Equal(t, "always", patched.Attrs["visibility"])
		require.Equal(t, http.StatusBadRequest, callJSON(t, adminClient, http.MethodPatch, fieldsURL+"/"+site.ID, map[string]any{"type": "date"}, nil))
		require.Equal(t, http.StatusForbidden, callJSON(t, memberClient, http.MethodPatch, fieldsURL+"/"+site.ID, map[string]any{"name": "Mine"}, nil))

		require.Equal(t, http.StatusForbidden, callJSON(t, memberClient, http.MethodDelete, fieldsURL+"/"+site.ID, nil, nil))
		require.Equal(t, http.StatusOK, callJSON(t, adminClient, http.MethodDelete, fieldsURL+"/"+site.ID, nil, nil))
		assert.NotContains(t, values(t, memberClient, member.Id), site.ID)
	})

	t.Run("a team has a limited number of attributes", func(t *testing.T) {
		var listed []*model.PropertyField
		require.Equal(t, http.StatusOK, callJSON(t, adminClient, http.MethodGet, fieldsURL, nil, &listed))
		for i := len(listed); i < model.CommunityAttributesMaxFields; i++ {
			create(t, map[string]any{"name": fmt.Sprintf("Extra %d", i), "type": "text"})
		}
		require.Equal(t, http.StatusBadRequest, callJSON(t, adminClient, http.MethodPost, fieldsURL, map[string]any{"name": "One too many", "type": "text"}, nil))
	})
}

func fieldNames(fields []*model.PropertyField) []string {
	names := make([]string, 0, len(fields))
	for _, f := range fields {
		names = append(names, f.Name)
	}
	return names
}

// The server's hidden and admin-only user attributes are for those who administer users and the user only.
func TestUserAttributeVisibility(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()

	field := func(t *testing.T, visibility string) *model.PropertyField {
		t.Helper()
		created, _, err := th.SystemAdminClient.CreateCPAField(ctx, &model.PropertyField{
			Name:  celSafeName(),
			Type:  model.PropertyFieldTypeText,
			Attrs: model.StringInterface{"visibility": visibility},
		})
		require.NoError(t, err)
		return created
	}
	adminOnly := field(t, model.PropertyFieldVisibilityAdmin)
	hidden := field(t, model.PropertyFieldVisibilityHidden)
	public := field(t, model.PropertyFieldVisibilityAlways)

	user2 := th.BasicUser2
	_, _, err := th.SystemAdminClient.PatchCPAValuesForUser(ctx, user2.Id, map[string]json.RawMessage{
		adminOnly.ID: json.RawMessage(`"secret"`),
		hidden.ID:    json.RawMessage(`"X-1"`),
		public.ID:    json.RawMessage(`"Paris"`),
	})
	require.NoError(t, err)

	t.Run("others don't read them", func(t *testing.T) {
		got, _, err := th.Client.ListCPAValues(ctx, user2.Id)
		require.NoError(t, err)
		assert.NotContains(t, got, adminOnly.ID)
		assert.NotContains(t, got, hidden.ID)
		assert.JSONEq(t, `"Paris"`, string(got[public.ID]))

		var generic []*model.PropertyValue
		require.Equal(t, http.StatusOK, callJSON(t, th.Client, http.MethodGet, "/properties/groups/"+model.AccessControlPropertyGroupName+"/user/values/"+user2.Id, nil, &generic))
		for _, v := range generic {
			assert.NotEqual(t, adminOnly.ID, v.FieldID)
			assert.NotEqual(t, hidden.ID, v.FieldID)
		}
	})

	t.Run("the user and the admins do", func(t *testing.T) {
		client2 := th.CreateClient()
		_, _, err := client2.Login(ctx, user2.Email, user2.Password)
		require.NoError(t, err)
		for _, client := range []*model.Client4{client2, th.SystemAdminClient} {
			got, _, err := client.ListCPAValues(ctx, user2.Id)
			require.NoError(t, err)
			assert.JSONEq(t, `"secret"`, string(got[adminOnly.ID]))
			assert.JSONEq(t, `"X-1"`, string(got[hidden.ID]))
		}
	})
}
