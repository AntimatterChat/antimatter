// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Server categories (Antimatter) are the sidebar categories admins set up for everyone in a team. Channels are filed
// in them by name, as Mattermost's managed categories; the team's list adds their order and whether each is open or
// locked.

func (a *App) GetServerCategories(rctx request.CTX, teamID string) ([]*model.ServerCategory, *model.AppError) {
	categories, err := a.Srv().Store().ServerCategory().GetForTeam(rctx, teamID)
	if err != nil {
		return nil, model.NewAppError("GetServerCategories", "app.server_category.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return categories, nil
}

// UpdateServerCategories replaces a team's server categories. The channels filed under a renamed category follow its
// new name; those of a removed category are unfiled, back in everyone's own categories. Names match ignoring case,
// and a category filed under but missing from the list can be renamed or removed too.
func (a *App) UpdateServerCategories(rctx request.CTX, teamID string, update *model.ServerCategoriesUpdate) ([]*model.ServerCategory, *model.AppError) {
	if appErr := update.IsValid(teamID); appErr != nil {
		return nil, appErr
	}

	removed := slices.Clone(update.Removed)
	for from := range update.Renames {
		removed = append(removed, from)
	}
	previous, stored, err := a.Srv().Store().ServerCategory().ReplaceForTeam(rctx, teamID, update.Categories, removed)
	if err != nil {
		var conflict *store.ErrConflict
		if errors.As(err, &conflict) {
			return nil, model.NewAppError("UpdateServerCategories", "app.server_category.conflict.app_error", nil, "", http.StatusConflict).Wrap(err)
		}
		return nil, model.NewAppError("UpdateServerCategories", "app.server_category.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	// Where the channels filed under each name go: a new name, or nowhere ("").
	moves := map[string]string{}
	for from, to := range update.Renames {
		if c := model.FindServerCategory(stored, to); c != nil {
			moves[categoryKey(from)] = c.Name
		}
	}
	gone := slices.Clone(update.Removed)
	for _, c := range previous {
		gone = append(gone, c.Name)
	}
	for _, name := range gone {
		if _, ok := moves[categoryKey(name)]; !ok && model.FindServerCategory(stored, name) == nil {
			moves[categoryKey(name)] = ""
		}
	}
	if len(moves) > 0 {
		// The categories' overrides go with them first: the channels refiled then inherit them.
		a.renameCategoryOverrides(rctx, teamID, moves)
		if appErr := a.refileChannels(rctx, teamID, moves); appErr != nil {
			return nil, appErr
		}
	}

	a.PublishServerCategoriesUpdated(teamID)
	return stored, nil
}

// categoryKey is how category names compare: trimmed, ignoring case.
func categoryKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// refileChannels moves the team's channels filed under the categories named in moves (by categoryKey) to their new
// category, or unfiles them.
func (a *App) refileChannels(rctx request.CTX, teamID string, moves map[string]string) *model.AppError {
	channels, err := a.Srv().Store().Channel().GetTeamChannels(teamID)
	var nfErr *store.ErrNotFound
	if err != nil && !errors.As(err, &nfErr) {
		return model.NewAppError("UpdateServerCategories", "app.server_category.refile.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if len(channels) == 0 {
		return nil
	}

	channelIDs := make([]string, 0, len(channels))
	for _, ch := range channels {
		channelIDs = append(channelIDs, ch.Id)
	}
	values, appErr := a.SearchPropertyValues(rctx, a.Channels().managedCategoryGroupID, model.PropertyValueSearchOpts{
		FieldID:   a.Channels().managedCategoryFieldID,
		TargetIDs: channelIDs,
		PerPage:   len(channelIDs),
	})
	if appErr != nil {
		return model.NewAppError("UpdateServerCategories", "app.server_category.refile.app_error", nil, "", http.StatusInternalServerError).Wrap(appErr)
	}

	for _, v := range values {
		var name string
		if json.Unmarshal(v.Value, &name) != nil {
			continue
		}
		to, ok := moves[categoryKey(name)]
		if !ok || to == name {
			continue
		}
		if to == "" {
			appErr = a.ClearChannelManagedCategory(rctx, v.TargetID)
		} else {
			appErr = a.SetChannelManagedCategory(rctx, v.TargetID, to)
		}
		if appErr != nil {
			rctx.Logger().Warn("Failed to refile a channel in its server category", mlog.String("channel_id", v.TargetID), mlog.String("from", name), mlog.String("to", to), mlog.Err(appErr))
		}
	}
	return nil
}

// PublishServerCategoriesUpdated tells a team's clients to fetch its server categories and their channels again.
func (a *App) PublishServerCategoriesUpdated(teamID string) {
	message := model.NewWebSocketEvent(model.WebsocketEventServerCategoriesUpdated, teamID, "", "", nil, "")
	message.Add("team_id", teamID)
	a.Publish(message)
}

// GetChannelServerCategory gives the name of the server category a channel is filed in, or "".
func (a *App) GetChannelServerCategory(rctx request.CTX, channelID string) (string, *model.AppError) {
	values, appErr := a.SearchPropertyValues(rctx, a.Channels().managedCategoryGroupID, model.PropertyValueSearchOpts{
		FieldID:   a.Channels().managedCategoryFieldID,
		TargetIDs: []string{channelID},
		PerPage:   1,
	})
	if appErr != nil || len(values) == 0 {
		return "", appErr
	}
	var name string
	if err := json.Unmarshal(values[0].Value, &name); err != nil {
		return "", nil
	}
	return name, nil
}

// SessionCanFileChannel tells whether someone may file a channel in a server category, or take it out of one ("").
// An open category takes channels from anyone who can manage the channel's roles (Mattermost's rule for managed
// categories); a locked category, or a new one, needs a team admin.
func (a *App) SessionCanFileChannel(rctx request.CTX, session model.Session, channel *model.Channel, name string) bool {
	if a.SessionHasPermissionToTeam(session, channel.TeamId, model.PermissionManageTeam) {
		return true
	}

	categories, appErr := a.GetServerCategories(rctx, channel.TeamId)
	if appErr != nil {
		return false
	}
	current, appErr := a.GetChannelServerCategory(rctx, channel.Id)
	if appErr != nil {
		return false
	}
	for _, n := range []string{current, name} {
		if strings.TrimSpace(n) == "" {
			continue
		}
		if c := model.FindServerCategory(categories, n); c == nil || c.Locked {
			return false
		}
	}

	ok, _ := a.SessionHasPermissionToChannel(rctx, session, channel.Id, model.PermissionManageChannelRoles)
	return ok
}

// EnsureServerCategory adds a category to the end of the team's list, open, when a channel is filed under a name the
// list doesn't have yet. It gives back the name as the list spells it.
func (a *App) EnsureServerCategory(rctx request.CTX, teamID, name string) (string, *model.AppError) {
	category, added, err := a.Srv().Store().ServerCategory().AddForTeam(rctx, teamID, strings.TrimSpace(name))
	if err != nil {
		return "", model.NewAppError("EnsureServerCategory", "app.server_category.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if added {
		a.PublishServerCategoriesUpdated(teamID)
	}
	return category.Name, nil
}

// DeleteServerCategoriesForTeam removes a team's server categories, as the team goes.
func (a *App) DeleteServerCategoriesForTeam(rctx request.CTX, teamID string) *model.AppError {
	if err := a.Srv().Store().ChannelOverride().DeleteCategoriesForTeam(rctx, teamID); err != nil {
		return model.NewAppError("DeleteServerCategoriesForTeam", "app.server_category.delete.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if err := a.Srv().Store().ServerCategory().DeleteForTeam(rctx, teamID); err != nil {
		return model.NewAppError("DeleteServerCategoriesForTeam", "app.server_category.delete.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return nil
}
