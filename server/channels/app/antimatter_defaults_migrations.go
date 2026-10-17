// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Antimatter's one-time migrations of defaults that changed: each runs once, on the first start of the version that
// changed it, and is recorded in the Systems table. They only bring what was left at the old default to the new one,
// and leave alone what an admin chose.

const (
	moderatorPresetMigrationKey          = "AntimatterModeratorPresetMigrationComplete"
	administratorRolesMigrationKey       = "AntimatterAdministratorRolesMigrationComplete"
	maxUsersPerTeamUnlimitedMigrationKey = "AntimatterMaxUsersPerTeamUnlimitedMigrationComplete"
	communityKindsMigrationKey           = "AntimatterCommunityKindsMigrationComplete"
	generalChannelMigrationKey           = "AntimatterGeneralChannelMigrationComplete"

	// oldMaxUsersPerTeamDefault is TeamSettings.MaxUsersPerTeam's default before it became 0 (no limit).
	oldMaxUsersPerTeamDefault = 50
)

// moderatorPresetPermissionsV1 is what the "Moderators" role of Fusion's layout presets (webapp/fusion/src/fusion/
// teams/templates.ts) granted at first: no "Remove people", so its moderators could neither time anyone out nor remove
// them.
var moderatorPresetPermissionsV1 = []string{
	model.PermissionDeleteOthersPosts.Id,
	model.PermissionEditOthersPosts.Id,
	model.PermissionManagePublicChannelProperties.Id,
	model.PermissionManagePrivateChannelProperties.Id,
	model.PermissionManagePublicChannelMembers.Id,
	model.PermissionManagePrivateChannelMembers.Id,
}

// ModeratorPresetPermissions is what the presets' "Moderators" role grants now: the first set, and "Remove people"
// (remove_user_from_team), which timeouts and removals ask for (CheckCanModerateMember, CheckCanRemoveTeamMember).
func ModeratorPresetPermissions() []string {
	return append(slices.Clone(moderatorPresetPermissionsV1), model.PermissionRemoveUserFromTeam.Id)
}

func sameSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(slices.Compact(a), slices.Compact(b))
}

func (s *Server) migrationDone(key string) (bool, error) {
	var nfErr *store.ErrNotFound
	if _, err := s.Store().System().GetByName(key); err == nil {
		return true, nil
	} else if !errors.As(err, &nfErr) {
		return false, fmt.Errorf("could not query migration: %w", err)
	}
	return false, nil
}

func (s *Server) markMigrationDone(key string) error {
	if err := s.Store().System().SaveOrUpdate(&model.System{Name: key, Value: "true"}); err != nil {
		return fmt.Errorf("failed to mark the migration %s as completed: %w", key, err)
	}
	return nil
}

// doModeratorPresetMigration gives "Remove people" to the Moderators roles that communities got from a layout preset
// and kept as they were: the roles whose permissions are still exactly the preset's first set. A role an admin
// edited since has other permissions, and is left alone.
func (s *Server) doModeratorPresetMigration() error {
	if done, err := s.migrationDone(moderatorPresetMigrationKey); done || err != nil {
		return err
	}

	roles, err := s.Store().Role().GetAll()
	if err != nil {
		return fmt.Errorf("failed to get the roles: %w", err)
	}
	updated := 0
	for _, role := range roles {
		if !model.IsTeamRoleName(role.Name) || role.DeleteAt != 0 || !sameSet(role.Permissions, moderatorPresetPermissionsV1) {
			continue
		}
		role.Permissions = ModeratorPresetPermissions()
		if _, err := s.Store().Role().Save(role); err != nil {
			return fmt.Errorf("failed to update the role %s: %w", role.Name, err)
		}
		updated++
	}
	if updated > 0 {
		mlog.Info("Gave the communities' preset Moderators roles Remove people", mlog.Int("roles", updated))
	}
	return s.markMigrationDone(moderatorPresetMigrationKey)
}

// doAdministratorRolesMigration gives the roles that have "Administrator" on (manage_team) everything else their
// community's admins may do that a role may hold, as SetTeamRolePermissions does now: the switch promised it, but
// only granted the community's settings. It runs after the permissions migrations, so that the admins' role is the
// current one.
func (s *Server) doAdministratorRolesMigration() error {
	if done, err := s.migrationDone(administratorRolesMigrationKey); done || err != nil {
		return err
	}

	a := New(ServerConnector(s.Channels()))
	rctx := request.EmptyContext(s.Log())
	roles, err := s.Store().Role().GetAll()
	if err != nil {
		return fmt.Errorf("failed to get the roles: %w", err)
	}
	updated := 0
	for _, role := range roles {
		if !model.IsTeamRoleName(role.Name) || role.DeleteAt != 0 || !slices.Contains(role.Permissions, model.PermissionManageTeam.Id) {
			continue
		}
		group, err := s.Store().Group().Get(strings.TrimPrefix(role.Name, model.TeamRoleName("")))
		if err != nil || group.TeamId == "" {
			continue
		}
		adminPermissions, appErr := a.TeamAdminRolePermissions(rctx, group.TeamId)
		if appErr != nil {
			mlog.Warn("Failed to get what a community's admins may do", mlog.String("team_id", group.TeamId), mlog.Err(appErr))
			continue
		}
		permissions := append(slices.Clone(role.Permissions), adminPermissions...)
		slices.Sort(permissions)
		permissions = slices.Compact(permissions)
		if sameSet(permissions, role.Permissions) {
			continue
		}
		role.Permissions = permissions
		if _, err := s.Store().Role().Save(role); err != nil {
			return fmt.Errorf("failed to update the role %s: %w", role.Name, err)
		}
		updated++
	}
	if updated > 0 {
		mlog.Info("Gave the roles with Administrator on what their community's admins may do", mlog.Int("roles", updated))
	}
	return s.markMigrationDone(administratorRolesMigrationKey)
}

// doMaxUsersPerTeamUnlimitedMigration lifts TeamSettings.MaxUsersPerTeam from 50, its old default, to 0 (no limit),
// its default now. Config files store every setting, defaults included, so 50 there is almost always the former
// default rather than an admin's choice; any other value is kept, and so is one set through the environment
// (AM_ or MM_TEAMSETTINGS_MAXUSERSPERTEAM).
func (s *Server) doMaxUsersPerTeamUnlimitedMigration() error {
	if done, err := s.migrationDone(maxUsersPerTeamUnlimitedMigrationKey); done || err != nil {
		return err
	}

	if limit := s.Config().TeamSettings.MaxUsersPerTeam; limit != nil && *limit == oldMaxUsersPerTeamDefault && !s.maxUsersPerTeamSetFromEnvironment() {
		s.platform.UpdateConfig(func(config *model.Config) {
			config.TeamSettings.MaxUsersPerTeam = model.NewPointer(model.TeamSettingsDefaultMaxUsersPerTeam)
		})
		mlog.Info("Lifted the limit of members per community, which was the old default", mlog.Int("old_limit", oldMaxUsersPerTeamDefault))
	}
	return s.markMigrationDone(maxUsersPerTeamUnlimitedMigrationKey)
}

func (s *Server) maxUsersPerTeamSetFromEnvironment() bool {
	teamSettings, ok := s.platform.GetEnvironmentOverrides()["TeamSettings"].(map[string]any)
	if !ok {
		return false
	}
	_, ok = teamSettings["MaxUsersPerTeam"]
	return ok
}

// doCommunityKindsMigration gives every community that has no kind yet (all of them, on the first start of the
// version that brought kinds) TeamSettings.DefaultCommunityKind, as it is when it runs: a server set to workplace
// before upgrading makes all its communities workplaces. Each community's row is written explicitly, as for new ones,
// so that apps reading the Systems table (Calls) find it; a kind already set is kept. Admins change each community's
// kind afterwards.
func (s *Server) doCommunityKindsMigration() error {
	if done, err := s.migrationDone(communityKindsMigrationKey); done || err != nil {
		return err
	}

	kind := model.TeamKindCommunity
	if k := s.Config().TeamSettings.DefaultCommunityKind; k != nil && *k == model.TeamKindWorkplace {
		kind = model.TeamKindWorkplace
	}
	teams, err := s.Store().Team().GetAll()
	if err != nil {
		return fmt.Errorf("failed to get the teams: %w", err)
	}
	written := 0
	for _, team := range teams {
		name := model.TeamKindSystemKeyPrefix + team.Id
		var nfErr *store.ErrNotFound
		if _, err := s.Store().System().GetByName(name); err == nil {
			continue
		} else if !errors.As(err, &nfErr) {
			return fmt.Errorf("failed to get the kind of team %s: %w", team.Id, err)
		}
		if err := s.Store().System().SaveOrUpdate(&model.System{Name: name, Value: kind}); err != nil {
			return fmt.Errorf("failed to save the kind of team %s: %w", team.Id, err)
		}
		written++
	}
	if written > 0 {
		s.Store().Team().ClearCaches()
		mlog.Info("Gave the communities without a kind the default one", mlog.Int("teams", written), mlog.String("kind", kind))
	}
	return s.markMigrationDone(communityKindsMigrationKey)
}

// The default channel's display names. General is what communities get now; Town Square is what they got before, in
// the server's language of the time: its translations stay in the catalogs for doGeneralChannelMigration only.
const generalChannelDisplayNameID = "api.channel.create_default_channels.general"

var legacyDefaultChannelDisplayNameID = i18n.TranslationId("api.channel.create_default_channels.town_square")

// translateOr translates a string, giving fallback when the catalogs aren't loaded.
func translateOr(t i18n.TranslateFunc, id, fallback string) string {
	if translated := t(id); translated != "" && translated != id {
		return translated
	}
	return fallback
}

// generalChannelDisplayName gives the display name of a default channel the migration renames: General, in the
// language of its old name, when its admins never renamed it, that is when it is still Town Square in one of the
// server's languages. ok is false for a name the admins chose, which the channel keeps. The server's language is tried
// first, then English, as several languages share a name.
func (s *Server) generalChannelDisplayName(displayName string) (string, bool) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return "", false
	}
	locales := []string{}
	if locale := s.Config().LocalizationSettings.DefaultServerLocale; locale != nil && *locale != "" {
		locales = append(locales, *locale)
	}
	locales = append(locales, "en")
	others := []string{}
	for locale := range i18n.GetSupportedLocales() {
		others = append(others, locale)
	}
	slices.Sort(others)
	locales = append(locales, others...)

	for _, locale := range locales {
		t := i18n.GetUserTranslations(locale)
		if translateOr(t, legacyDefaultChannelDisplayNameID, "Town Square") == displayName {
			return translateOr(t, generalChannelDisplayNameID, "General"), true
		}
	}
	return "", false
}

// doGeneralChannelMigration renames every community's default channel from town-square (Town Square) to general
// (General), the name communities get now. It keeps the channel's ID, so links by ID, members, posts and settings stay.
// The display name changes only while it is still the old default (in any of the server's languages); one the admins
// chose is kept. A community that has a channel named general of its own, even archived, keeps its town-square as it
// is, which stays its default channel (Team.DefaultChannelId is set to it unless the community chose another): its
// links keep working and nothing is merged. An archived town-square is kept too, as the store can't update it.
//
// Old links and lookups by town-square still find the default channel (getChannelByNameOrLegacy, and the web apps'
// channel routes). A community that failed to be renamed is logged and tried again on the next start.
func (s *Server) doGeneralChannelMigration() error {
	if done, err := s.migrationDone(generalChannelMigrationKey); done || err != nil {
		return err
	}

	rctx := request.EmptyContext(s.Log())
	teams, err := s.Store().Team().GetAll()
	if err != nil {
		return fmt.Errorf("failed to get the teams: %w", err)
	}
	renamed, kept, failed := 0, 0, 0
	for _, team := range teams {
		var nfErr *store.ErrNotFound
		channel, err := s.Store().Channel().GetByNameIncludeDeleted(team.Id, model.LegacyDefaultChannelName, false)
		if errors.As(err, &nfErr) {
			continue
		} else if err != nil {
			mlog.Warn("Failed to get a community's town-square channel", mlog.String("team_id", team.Id), mlog.Err(err))
			failed++
			continue
		}
		if channel.DeleteAt != 0 {
			mlog.Info("Kept the archived town-square channel of a community", mlog.String("team_id", team.Id), mlog.String("channel_id", channel.Id))
			kept++
			continue
		}

		if _, err = s.Store().Channel().GetByNameIncludeDeleted(team.Id, model.DefaultChannelName, false); err == nil {
			if team.DefaultChannelId == "" {
				team.DefaultChannelId = channel.Id
				if _, err = s.Store().Team().Update(team); err != nil {
					mlog.Warn("Failed to keep town-square as a community's default channel", mlog.String("team_id", team.Id), mlog.Err(err))
					failed++
					continue
				}
			}
			mlog.Warn("Kept the town-square channel of a community that has a general channel of its own; its admins may rename them",
				mlog.String("team_id", team.Id), mlog.String("team_name", team.Name), mlog.String("channel_id", channel.Id))
			kept++
			continue
		} else if !errors.As(err, &nfErr) {
			mlog.Warn("Failed to look for a community's general channel", mlog.String("team_id", team.Id), mlog.Err(err))
			failed++
			continue
		}

		channel.Name = model.DefaultChannelName
		if displayName, ok := s.generalChannelDisplayName(channel.DisplayName); ok {
			channel.DisplayName = displayName
		}
		if _, err = s.Store().Channel().Update(rctx, channel); err != nil {
			mlog.Warn("Failed to rename a community's town-square channel to general", mlog.String("team_id", team.Id), mlog.String("channel_id", channel.Id), mlog.Err(err))
			failed++
			continue
		}
		renamed++
	}

	if renamed > 0 || kept > 0 {
		s.Store().Channel().ClearCaches()
		s.Store().Team().ClearCaches()
		mlog.Info("Renamed the communities' default channel from town-square to general", mlog.Int("renamed", renamed), mlog.Int("kept", kept))
	}
	if failed > 0 {
		mlog.Warn("Some communities' default channel couldn't be renamed to general; trying again on the next start", mlog.Int("failed", failed))
		return nil
	}
	return s.markMigrationDone(generalChannelMigrationKey)
}
