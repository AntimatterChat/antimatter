// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Antimatter: creating a community. Its address (Team.Name) can be left out: the server makes one from its name. The
// kind of community decides whether everyone is in all its channels: a guild has them, a workspace doesn't, on
// creation and whenever its kind changes (the admins can still change the setting on its own afterwards).

// freeTeamNameTries is how many numbered addresses are tried before a random one.
const freeTeamNameTries = 20

// teamNameFromDisplayName is the address a community's name suggests: in lower case, accents dropped, dashes for the
// rest; "" when the name gives none (no Latin letters or digits, or a reserved word first).
func teamNameFromDisplayName(displayName string) string {
	var b strings.Builder
	for _, r := range norm.NFKD.String(strings.ToLower(displayName)) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	name := b.String()
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}
	name = strings.Trim(name, "-")
	// Room for a number after it.
	if len(name) > model.TeamNameMaxLength-4 {
		name = strings.Trim(name[:model.TeamNameMaxLength-4], "-")
	}
	if !model.IsValidTeamName(name) || model.IsReservedTeamName(name) {
		return ""
	}
	return name
}

// FreeTeamName gives an address no community has, for a community named displayName created without one: its name's
// suggestion, else that with a number after it, else a random one.
func (a *App) FreeTeamName(displayName string) (string, error) {
	base := teamNameFromDisplayName(displayName)
	if base == "" {
		return model.NewId(), nil
	}
	free := func(name string) (bool, error) {
		_, err := a.Srv().Store().Team().GetByName(name)
		if err == nil {
			return false, nil
		}
		var nfErr *store.ErrNotFound
		if errors.As(err, &nfErr) {
			return true, nil
		}
		return false, err
	}
	for i := 1; i <= freeTeamNameTries; i++ {
		name := base
		if i > 1 {
			name = fmt.Sprintf("%s-%d", base, i)
		}
		ok, err := free(name)
		if err != nil {
			return "", err
		}
		if ok {
			return name, nil
		}
	}
	return base + "-" + model.NewId()[:6], nil
}

// allChannelsForKind tells whether a community of a kind has everyone in all its channels: guilds do, unless the
// server turns it off for new communities (TeamSettings.AllChannelsMembershipForNewTeams); workspaces don't.
func (a *App) allChannelsForKind(kind string) bool {
	if model.NormalizeTeamKind(kind) != model.TeamKindGuild {
		return false
	}
	setting := a.Config().TeamSettings.AllChannelsMembershipForNewTeams
	return setting == nil || *setting
}

// PrepareNewTeam fills in what a community created through the API leaves to the server: its address when it has
// none, its kind (the server's default) and, from its kind, whether everyone is in all its channels.
func (a *App) PrepareNewTeam(team *model.Team) *model.AppError {
	if team.Name == "" {
		name, err := a.FreeTeamName(team.DisplayName)
		if err != nil {
			return model.NewAppError("PrepareNewTeam", "app.team.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		team.Name = name
	}
	team.Kind = model.NormalizeTeamKind(team.Kind)
	if team.Kind == "" {
		team.Kind = model.NormalizeTeamKind(*a.Config().TeamSettings.DefaultCommunityKind)
	}
	team.AllChannelsMembership = a.allChannelsForKind(team.Kind)
	team.DefaultChannelId = ""
	team.WelcomeChannelId = ""
	team.DefaultRoleId = ""
	return nil
}
