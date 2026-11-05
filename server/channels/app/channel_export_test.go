// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// The channel export leaves out the email addresses their authors hide from the person exporting (profile
// privacy), as everything else the server sends out does.
func TestExportChannelProfilePrivacy(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.PrivacySettings.ShowEmailAddress = true
		*cfg.TeamSettings.RestrictChannelExportToAdmins = false
	})

	// Sofia hides her email address from the community; Bob shows his.
	sofia := th.CreateUser(t)
	th.LinkUserToTeam(t, sofia, th.BasicTeam)
	th.AddUserToChannel(t, sofia, th.BasicChannel)
	privacy, err := json.Marshal(model.ProfilePrivacy{th.BasicTeam.Id: {model.ProfileFieldEmail}})
	require.NoError(t, err)
	sofia.Props = model.StringMap{model.UserPropsKeyProfilePrivacy: string(privacy)}
	sofia, appErr := th.App.UpdateUser(th.Context, sofia, false)
	require.Nil(t, appErr)

	by := func(user *model.User) PostOptions {
		return func(post *model.Post) { post.UserId = user.Id }
	}
	th.CreatePost(t, th.BasicChannel, by(sofia))
	th.CreatePost(t, th.BasicChannel, by(th.BasicUser2))

	// emailsSeenBy exports the channel as viewerID and gives the email address of each author, by user id.
	emailsSeenBy := func(t *testing.T, viewerID string) map[string]string {
		t.Helper()
		var buf bytes.Buffer
		_, appErr := th.App.ExportChannel(th.Context, viewerID, th.BasicChannel.Id, ChannelExportOptions{}, func(*ChannelExportFile) (io.Writer, error) {
			return &buf, nil
		})
		require.Nil(t, appErr)
		rows, err := csv.NewReader(&buf).ReadAll()
		require.NoError(t, err)
		require.Greater(t, len(rows), 1)
		require.Equal(t, "User Id", rows[0][3])
		require.Equal(t, "User Email", rows[0][5])
		emails := map[string]string{}
		for _, row := range rows[1:] {
			emails[row[3]] = row[5]
		}
		return emails
	}

	t.Run("a member of the community doesn't get the address she hides", func(t *testing.T) {
		emails := emailsSeenBy(t, th.BasicUser.Id)
		require.Contains(t, emails, sofia.Id)
		assert.Empty(t, emails[sofia.Id])
		assert.Equal(t, th.BasicUser2.Email, emails[th.BasicUser2.Id], "the others' addresses are still there")
	})

	t.Run("nor does a member who reads the public channel without being in it", func(t *testing.T) {
		reader := th.CreateUser(t)
		th.LinkUserToTeam(t, reader, th.BasicTeam)
		emails := emailsSeenBy(t, reader.Id)
		assert.Empty(t, emails[sofia.Id])
	})

	t.Run("she gets her own", func(t *testing.T) {
		emails := emailsSeenBy(t, sofia.Id)
		assert.Equal(t, sofia.Email, emails[sofia.Id])
	})

	t.Run("system admins get every address", func(t *testing.T) {
		emails := emailsSeenBy(t, th.SystemAdminUser.Id)
		assert.Equal(t, sofia.Email, emails[sofia.Id])
		assert.Equal(t, th.BasicUser2.Email, emails[th.BasicUser2.Id])
	})
}
