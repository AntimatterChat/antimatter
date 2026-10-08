// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"errors"
	"net/http"
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
// new name; those of a category left out are unfiled, back in everyone's own categories.
func (a *App) UpdateServerCategories(rctx request.CTX, teamID string, update *model.ServerCategoriesUpdate) ([]*model.ServerCategory, *model.AppError) {
	if appErr := update.IsValid(teamID); appErr != nil {
		return nil, appErr
	}

	previous, appErr := a.GetServerCategories(rctx, teamID)
	if appErr != nil {
		return nil, appErr
	}

	stored, err := a.Srv().Store().ServerCategory().ReplaceForTeam(rctx, teamID, update.Categories)
	if err != nil {
		return nil, model.NewAppError("UpdateServerCategories", "app.server_category.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	// Where the channels of each previous category go: a new name, or nowhere ("").
	moves := map[string]string{}
	for _, c := range previous {
		if renamed, ok := update.Renames[c.Name]; ok && model.FindServerCategory(stored, renamed) != nil {
			moves[c.Name] = model.FindServerCategory(stored, renamed).Name
		} else if model.FindServerCategory(stored, c.Name) == nil {
			moves[c.Name] = ""
		}
	}
	if len(moves) > 0 {
		if appErr := a.refileChannels(rctx, teamID, moves); appErr != nil {
			return nil, appErr
		}
	}

	a.PublishServerCategoriesUpdated(teamID)
	return stored, nil
}

// refileChannels moves the team's channels filed under the categories named in moves to their new category, or
// unfiles them.
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
		to, ok := moves[name]
		if !ok {
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
	name = strings.TrimSpace(name)
	categories, appErr := a.GetServerCategories(rctx, teamID)
	if appErr != nil {
		return "", appErr
	}
	if c := model.FindServerCategory(categories, name); c != nil {
		return c.Name, nil
	}
	categories = append(categories, &model.ServerCategory{TeamId: teamID, Name: name})
	if _, appErr := a.UpdateServerCategories(rctx, teamID, &model.ServerCategoriesUpdate{Categories: categories}); appErr != nil {
		return "", appErr
	}
	return name, nil
}
