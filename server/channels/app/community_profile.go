// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"database/sql"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Community profiles (model.CommunityProfile): a nickname and a picture for one community. Only the person sets
// theirs; the community's moderators (those who may time them out, see CheckCanModerateMember) may take off a
// nickname or a picture that breaks the community's rules, nothing else.

// communityAvatarPath is where someone's picture for a community is kept, next to their profile picture.
func communityAvatarPath(teamID, userID string) string {
	return filepath.Join("users", userID, "communities", teamID+".png")
}

// GetCommunityProfiles lists the profiles set in a community, in one read of the preferences (by their name, the
// community's id). The profiles of people who left the community stay: their old messages keep showing them.
func (a *App) GetCommunityProfiles(rctx request.CTX, teamID string) ([]*model.CommunityProfile, *model.AppError) {
	prefs, err := a.Srv().Store().Preference().GetCategoryAndName(model.PreferenceCategoryCommunityProfile, teamID)
	if err != nil {
		return nil, model.NewAppError("GetCommunityProfiles", "app.community_profile.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	profiles := make([]*model.CommunityProfile, 0, len(prefs))
	for i := range prefs {
		if profile := model.CommunityProfileFromPreference(&prefs[i]); profile != nil && !profile.IsEmpty() {
			profiles = append(profiles, profile)
		}
	}
	return profiles, nil
}

// GetCommunityProfile gives someone's profile in a community, an empty one when they have none.
func (a *App) GetCommunityProfile(teamID, userID string) (*model.CommunityProfile, *model.AppError) {
	pref, err := a.Srv().Store().Preference().Get(userID, model.PreferenceCategoryCommunityProfile, teamID)
	if err != nil {
		// The preference store tells a missing preference by sql.ErrNoRows.
		var nfErr *store.ErrNotFound
		if errors.As(err, &nfErr) || errors.Is(err, sql.ErrNoRows) {
			return &model.CommunityProfile{TeamId: teamID, UserId: userID}, nil
		}
		return nil, model.NewAppError("GetCommunityProfile", "app.community_profile.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	profile := model.CommunityProfileFromPreference(pref)
	if profile == nil {
		return &model.CommunityProfile{TeamId: teamID, UserId: userID}, nil
	}
	return profile, nil
}

// checkCommunityMember refuses a profile to someone who isn't in the community.
func (a *App) checkCommunityMember(rctx request.CTX, teamID, userID string) *model.AppError {
	member, err := a.Srv().Store().Team().GetMember(rctx, teamID, userID)
	if err != nil || member.DeleteAt != 0 {
		return model.NewAppError("checkCommunityMember", "app.community_profile.not_member.app_error", nil, "", http.StatusBadRequest)
	}
	return nil
}

// saveCommunityProfile keeps a profile (or drops it when it changes nothing), and tells the community's members.
func (a *App) saveCommunityProfile(rctx request.CTX, profile *model.CommunityProfile) *model.AppError {
	profile.UpdateAt = model.GetMillis()
	if invalid := profile.IsValid(); invalid != nil {
		return invalid
	}
	if profile.IsEmpty() {
		if err := a.Srv().Store().Preference().Delete(profile.UserId, model.PreferenceCategoryCommunityProfile, profile.TeamId); err != nil {
			return model.NewAppError("saveCommunityProfile", "app.community_profile.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
	} else {
		pref, err := profile.ToPreference()
		if err != nil {
			return model.NewAppError("saveCommunityProfile", "app.community_profile.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		if err := a.Srv().Store().Preference().Save(model.Preferences{*pref}); err != nil {
			return model.NewAppError("saveCommunityProfile", "app.community_profile.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
	}

	// The profile itself goes in the event: the community's members don't read the profiles again for each change.
	message := model.NewWebSocketEvent(model.WebsocketEventCommunityProfileUpdated, profile.TeamId, "", "", nil, "")
	message.Add("team_id", profile.TeamId)
	message.Add("user_id", profile.UserId)
	message.Add("profile", profile)
	a.Publish(message)
	return nil
}

// PatchCommunityProfile changes the person's own nickname in a community.
func (a *App) PatchCommunityProfile(rctx request.CTX, teamID, userID string, patch *model.CommunityProfilePatch) (*model.CommunityProfile, *model.AppError) {
	if appErr := a.checkCommunityMember(rctx, teamID, userID); appErr != nil {
		return nil, appErr
	}
	profile, appErr := a.GetCommunityProfile(teamID, userID)
	if appErr != nil {
		return nil, appErr
	}
	if patch.Nickname != nil {
		profile.Nickname = model.NormalizeCommunityNickname(*patch.Nickname)
	}
	if appErr := a.saveCommunityProfile(rctx, profile); appErr != nil {
		return nil, appErr
	}
	return profile, nil
}

// SetCommunityAvatar sets the person's picture for a community, from an image they upload: checked (resolution,
// that it is an image), made upright and cut to a square as profile pictures are, kept as PNG.
func (a *App) SetCommunityAvatar(rctx request.CTX, teamID, userID string, imageData *multipart.FileHeader) (*model.CommunityProfile, *model.AppError) {
	if appErr := a.checkCommunityMember(rctx, teamID, userID); appErr != nil {
		return nil, appErr
	}
	file, err := imageData.Open()
	if err != nil {
		return nil, model.NewAppError("SetCommunityAvatar", "api.user.upload_profile_user.open.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}
	defer file.Close()
	if limitErr := checkImageLimits(file, *a.Config().FileSettings.MaxImageResolution); limitErr != nil {
		return nil, model.NewAppError("SetCommunityAvatar", "api.user.upload_profile_user.check_image_limits.app_error", nil, "", http.StatusBadRequest).Wrap(limitErr)
	}
	if _, seekErr := file.Seek(0, io.SeekStart); seekErr != nil {
		return nil, model.NewAppError("SetCommunityAvatar", "api.user.upload_profile_user.open.app_error", nil, "", http.StatusBadRequest).Wrap(seekErr)
	}
	buf, appErr := a.AdjustImage(rctx, file)
	if appErr != nil {
		return nil, appErr
	}
	if _, writeErr := a.WriteFile(buf, communityAvatarPath(teamID, userID)); writeErr != nil {
		return nil, model.NewAppError("SetCommunityAvatar", "api.user.upload_profile_user.upload_profile.app_error", nil, "", http.StatusInternalServerError).Wrap(writeErr)
	}

	profile, appErr := a.GetCommunityProfile(teamID, userID)
	if appErr != nil {
		return nil, appErr
	}
	profile.AvatarUpdateAt = model.GetMillis()
	if appErr := a.saveCommunityProfile(rctx, profile); appErr != nil {
		return nil, appErr
	}
	return profile, nil
}

// GetCommunityAvatar gives someone's picture for a community; nil when they have none (their profile picture
// shows).
func (a *App) GetCommunityAvatar(teamID, userID string) ([]byte, *model.CommunityProfile, *model.AppError) {
	profile, appErr := a.GetCommunityProfile(teamID, userID)
	if appErr != nil {
		return nil, nil, appErr
	}
	if profile.AvatarUpdateAt == 0 {
		return nil, profile, nil
	}
	img, appErr := a.ReadFile(communityAvatarPath(teamID, userID))
	if appErr != nil {
		return nil, profile, nil
	}
	return img, profile, nil
}

// ResetCommunityProfile takes a nickname and/or a picture off someone's profile in a community: the person
// themselves, or the community's moderators, for those below them (CheckCanModerateMember), when the nickname or
// the picture breaks the community's rules. Moderators can't set anything.
func (a *App) ResetCommunityProfile(rctx request.CTX, session model.Session, teamID, userID string, reset *model.CommunityProfileReset) (*model.CommunityProfile, *model.AppError) {
	if session.UserId == userID {
		if appErr := a.checkCommunityMember(rctx, teamID, userID); appErr != nil {
			return nil, appErr
		}
	} else if appErr := a.CheckCanModerateMember(rctx, session, teamID, userID); appErr != nil {
		return nil, appErr
	}
	profile, appErr := a.GetCommunityProfile(teamID, userID)
	if appErr != nil {
		return nil, appErr
	}
	changed := false
	if reset.Nickname && profile.Nickname != "" {
		profile.Nickname = ""
		changed = true
	}
	if reset.Avatar && profile.AvatarUpdateAt != 0 {
		profile.AvatarUpdateAt = 0
		changed = true
		if removeErr := a.RemoveFile(communityAvatarPath(teamID, userID)); removeErr != nil {
			rctx.Logger().Warn("Failed to remove a community picture", mlog.String("team_id", teamID), mlog.String("user_id", userID), mlog.Err(removeErr))
		}
	}
	if !changed {
		return profile, nil
	}
	if appErr := a.saveCommunityProfile(rctx, profile); appErr != nil {
		return nil, appErr
	}
	return profile, nil
}

// AddCommunityNicknameMatches adds to an autocomplete of a community's people those whose nickname there matches the
// term (each word of it starting the nickname or one of its words), who are still in the community and whom the one
// asking may see (options.ViewRestrictions): in the channel's list or out of it, as the others. It gives their ids:
// whatever then takes out of the results the people found only by a profile detail they hide (the per-community
// privacy) must keep these, found by a nickname they chose to show there.
func (a *App) AddCommunityNicknameMatches(rctx request.CTX, teamID, channelID, term string, options *model.UserSearchOptions, autocomplete *model.UserAutocomplete) map[string]bool {
	if strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(term), "@")) == "" {
		return nil
	}
	profiles, appErr := a.GetCommunityProfiles(rctx, teamID)
	if appErr != nil {
		rctx.Logger().Warn("Failed to read a community's profiles for an autocomplete", mlog.String("team_id", teamID), mlog.Err(appErr))
		return nil
	}
	listed := map[string]bool{}
	for _, user := range append(slices.Clone(autocomplete.Users), autocomplete.OutOfChannel...) {
		listed[user.Id] = true
	}
	limit := options.Limit
	if limit <= 0 {
		limit = model.UserSearchDefaultLimit
	}
	var ids []string
	for _, profile := range profiles {
		if profile.Nickname != "" && !listed[profile.UserId] && nicknameMatches(profile.Nickname, term) {
			ids = append(ids, profile.UserId)
			if len(ids) >= limit {
				break
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}

	members, err := a.Srv().Store().Team().GetMembersByIds(teamID, ids, options.ViewRestrictions)
	if err != nil {
		rctx.Logger().Warn("Failed to read a community's members for an autocomplete", mlog.String("team_id", teamID), mlog.Err(err))
		return nil
	}
	inTeam := map[string]bool{}
	for _, member := range members {
		if member.DeleteAt == 0 {
			inTeam[member.UserId] = true
		}
	}
	inChannel := map[string]bool{}
	if channelID != "" {
		channelMembers, channelErr := a.Srv().Store().Channel().GetMembersByIds(channelID, ids)
		if channelErr != nil {
			rctx.Logger().Warn("Failed to read a channel's members for an autocomplete", mlog.String("channel_id", channelID), mlog.Err(channelErr))
			return nil
		}
		for _, member := range channelMembers {
			inChannel[member.UserId] = true
		}
	}
	users, appErr := a.GetUsersByIds(rctx, ids, &store.UserGetByIdsOpts{IsAdmin: options.IsAdmin, ViewRestrictions: options.ViewRestrictions})
	if appErr != nil {
		rctx.Logger().Warn("Failed to read the users of an autocomplete", mlog.String("team_id", teamID), mlog.Err(appErr))
		return nil
	}
	matched := map[string]bool{}
	for _, user := range users {
		if user.DeleteAt != 0 || !inTeam[user.Id] {
			continue
		}
		matched[user.Id] = true
		if channelID == "" || inChannel[user.Id] {
			autocomplete.Users = append(autocomplete.Users, user)
		} else {
			autocomplete.OutOfChannel = append(autocomplete.OutOfChannel, user)
		}
	}
	return matched
}

// nicknameMatches tells whether every word of a search term starts the nickname or one of its words.
func nicknameMatches(nickname, term string) bool {
	nickname = strings.ToLower(nickname)
	words := strings.Fields(nickname)
	found := false
	for _, word := range strings.Fields(strings.ToLower(term)) {
		word = strings.TrimPrefix(word, "@")
		if word == "" {
			continue
		}
		if !strings.HasPrefix(nickname, word) && !slices.ContainsFunc(words, func(w string) bool { return strings.HasPrefix(w, word) }) {
			return false
		}
		found = true
	}
	return found
}

// communityNotificationNames gives how a notification of a community's message names its sender to a recipient whose
// own choice is nameFormat: as the community shows people, with the sender's nickname there. Direct and group
// messages keep the recipient's choice and the global names. (The sender's names the recipient may not see are
// taken off by the profile privacy, before or after: a community nickname isn't one of them.)
func (a *App) communityNotificationNames(notification *PostNotification, nameFormat string) (string, *PostNotification) {
	// Without full names on the server (PrivacySettings.ShowFullName), everyone shows by username, as the recipient's
	// format already says.
	if notification == nil || notification.Channel == nil || notification.Channel.TeamId == "" || notification.Sender == nil || !*a.Config().PrivacySettings.ShowFullName {
		return nameFormat, notification
	}
	teamID := notification.Channel.TeamId
	if team, appErr := a.GetTeam(teamID); appErr == nil && team.NameDisplay != "" {
		nameFormat = team.NameDisplay
	}
	if nameFormat != model.ShowNicknameFullName {
		return nameFormat, notification
	}
	profile, appErr := a.GetCommunityProfile(teamID, notification.Sender.Id)
	if appErr != nil || profile.Nickname == "" {
		return nameFormat, notification
	}
	adjusted := *notification
	adjusted.Sender = notification.Sender.DeepCopy()
	adjusted.Sender.Nickname = profile.Nickname
	return nameFormat, &adjusted
}
