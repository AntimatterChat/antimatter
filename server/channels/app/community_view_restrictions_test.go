// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// Community attribute values and profiles follow the view restrictions the user routes do: a guest only sees the
// people of the channels they're in.
func TestCommunityAttributeValuesRespectGuestViewRestrictions(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })

	field, appErr := th.App.CreateCommunityAttribute(th.Context, th.BasicTeam.Id, th.SystemAdminUser.Id,
		&model.PropertyField{Name: "Desk", Type: model.PropertyFieldTypeText})
	require.Nil(t, appErr)

	// The guest shares a channel with BasicUser, none with unseen.
	guest := th.CreateGuest(t)
	th.LinkUserToTeam(t, guest, th.BasicTeam)
	shared := th.CreateChannel(t, th.BasicTeam)
	th.AddUserToChannel(t, guest, shared)
	unseen := th.CreateUser(t)
	th.LinkUserToTeam(t, unseen, th.BasicTeam)

	setDesk := func(user *model.User, desk string) {
		t.Helper()
		session := model.Session{UserId: user.Id, Roles: user.Roles}
		_, appErr := th.App.PatchCommunityAttributeValues(th.Context, session, th.BasicTeam.Id, user.Id,
			map[string]json.RawMessage{field.ID: json.RawMessage(`"` + desk + `"`)})
		require.Nil(t, appErr)
	}
	setDesk(th.BasicUser, "B-12")
	setDesk(unseen, "C-07")
	setDesk(guest, "G-01")

	guestSession := model.Session{UserId: guest.Id, Roles: model.SystemGuestRoleId}
	values, appErr := th.App.GetCommunityAttributeValues(th.Context, guestSession, th.BasicTeam.Id, []string{th.BasicUser.Id, unseen.Id, guest.Id})
	require.Nil(t, appErr)
	assert.Contains(t, values, th.BasicUser.Id, "someone the guest shares a channel with")
	assert.NotContains(t, values, unseen.Id, "someone the guest may not see")
	assert.Contains(t, values, guest.Id, "the guest's own")

	t.Run("a member sees everyone", func(t *testing.T) {
		session := model.Session{UserId: th.BasicUser2.Id, Roles: model.SystemUserRoleId}
		values, appErr := th.App.GetCommunityAttributeValues(th.Context, session, th.BasicTeam.Id, []string{th.BasicUser.Id, unseen.Id, guest.Id})
		require.Nil(t, appErr)
		assert.Len(t, values, 3)
	})

	t.Run("the community's profiles", func(t *testing.T) {
		for _, user := range []*model.User{th.BasicUser, unseen} {
			_, appErr := th.App.PatchCommunityProfile(th.Context, th.BasicTeam.Id, user.Id, &model.CommunityProfilePatch{Nickname: model.NewPointer("nick " + user.Username)})
			require.Nil(t, appErr)
		}
		profiles, appErr := th.App.GetCommunityProfilesForUser(th.Context, guest.Id, th.BasicTeam.Id)
		require.Nil(t, appErr)
		ids := make([]string, 0, len(profiles))
		for _, profile := range profiles {
			ids = append(ids, profile.UserId)
		}
		assert.Contains(t, ids, th.BasicUser.Id)
		assert.NotContains(t, ids, unseen.Id)

		all, appErr := th.App.GetCommunityProfilesForUser(th.Context, th.BasicUser2.Id, th.BasicTeam.Id)
		require.Nil(t, appErr)
		assert.Len(t, all, 2)
	})
}
