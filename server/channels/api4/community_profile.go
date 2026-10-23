// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// Antimatter community profiles (model.CommunityProfile): someone's nickname and picture in a community.
//
// Who may do what: the community's members read its profiles (one request for all of them; changes come with the
// community_profile_updated event). Only the person changes their own profile, system admins included: nobody else
// sets a nickname or a picture for someone. The community's moderators (who may time the person out, as Discord's
// hierarchy: CheckCanModerateMember) may take off a nickname or a picture that breaks the community's rules, and
// nothing else.

func (api *API) InitCommunityProfile() {
	api.BaseRoutes.Team.Handle("/profiles", api.APISessionRequired(getCommunityProfiles)).Methods(http.MethodGet)
	api.BaseRoutes.TeamMember.Handle("/profile", api.APISessionRequired(patchCommunityProfile)).Methods(http.MethodPatch)
	api.BaseRoutes.TeamMember.Handle("/profile/reset", api.APISessionRequired(resetCommunityProfile)).Methods(http.MethodPost)
	api.BaseRoutes.TeamMember.Handle("/profile/image", api.APISessionRequired(setCommunityAvatar)).Methods(http.MethodPost)
	api.BaseRoutes.TeamMember.Handle("/profile/image", api.APISessionRequiredTrustRequester(getCommunityAvatar)).Methods(http.MethodGet)
}

// getCommunityProfiles lists a community's profiles, for those who may see the community.
func getCommunityProfiles(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId()
	if c.Err != nil {
		return
	}
	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamId, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}
	profiles, appErr := c.App.GetCommunityProfiles(c.AppContext, c.Params.TeamId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, profiles)
}

// requireOwnCommunityProfile lets only the person themselves change their profile.
func requireOwnCommunityProfile(c *Context) bool {
	c.RequireTeamId().RequireUserId()
	if c.Err != nil {
		return false
	}
	if c.AppContext.Session().UserId != c.Params.UserId {
		c.Err = model.NewAppError("requireOwnCommunityProfile", "api.community_profile.not_own.app_error", nil, "", http.StatusForbidden)
		return false
	}
	return true
}

// patchCommunityProfile changes the person's nickname in the community.
func patchCommunityProfile(c *Context, w http.ResponseWriter, r *http.Request) {
	if !requireOwnCommunityProfile(c) {
		return
	}
	var patch model.CommunityProfilePatch
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&patch); err != nil {
		c.SetInvalidParamWithErr("profile", err)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventPatchCommunityProfile, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "team_id", c.Params.TeamId)
	model.AddEventParameterToAuditRec(auditRec, "nickname", patch.Nickname != nil)

	profile, appErr := c.App.PatchCommunityProfile(c.AppContext, c.Params.TeamId, c.Params.UserId, &patch)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	writeJSON(c, w, profile)
}

// resetCommunityProfile takes a nickname or a picture off a profile: the person's own, or a member's, for the
// community's moderators.
func resetCommunityProfile(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId().RequireUserId()
	if c.Err != nil {
		return
	}
	var reset model.CommunityProfileReset
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&reset); err != nil {
		c.SetInvalidParamWithErr("reset", err)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventResetCommunityProfile, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "team_id", c.Params.TeamId)
	model.AddEventParameterToAuditRec(auditRec, "user_id", c.Params.UserId)
	model.AddEventParameterToAuditRec(auditRec, "nickname", reset.Nickname)
	model.AddEventParameterToAuditRec(auditRec, "avatar", reset.Avatar)

	profile, appErr := c.App.ResetCommunityProfile(c.AppContext, *c.AppContext.Session(), c.Params.TeamId, c.Params.UserId, &reset)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	writeJSON(c, w, profile)
}

// setCommunityAvatar sets the person's picture in the community, from an image of at most the server's file size.
func setCommunityAvatar(c *Context, w http.ResponseWriter, r *http.Request) {
	defer func() {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			c.Logger.Warn("Error discarding request body", mlog.Err(err))
		}
	}()
	if !requireOwnCommunityProfile(c) {
		return
	}
	if *c.App.Config().FileSettings.DriverName == "" {
		c.Err = model.NewAppError("setCommunityAvatar", "api.user.upload_profile_user.storage.app_error", nil, "", http.StatusNotImplemented)
		return
	}
	maxSize := *c.App.Config().FileSettings.MaxFileSize
	if r.ContentLength > maxSize {
		c.Err = model.NewAppError("setCommunityAvatar", "api.user.upload_profile_user.too_large.app_error", nil, "", http.StatusRequestEntityTooLarge)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)
	if err := r.ParseMultipartForm(maxSize); err != nil {
		c.Err = model.NewAppError("setCommunityAvatar", "api.user.upload_profile_user.parse.app_error", nil, "", http.StatusBadRequest).Wrap(err)
		return
	}
	images := r.MultipartForm.File["image"]
	if len(images) == 0 || images[0] == nil {
		c.Err = model.NewAppError("setCommunityAvatar", "api.user.upload_profile_user.no_file.app_error", nil, "", http.StatusBadRequest)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventSetCommunityAvatar, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "team_id", c.Params.TeamId)
	model.AddEventParameterToAuditRec(auditRec, "filename", images[0].Filename)

	profile, appErr := c.App.SetCommunityAvatar(c.AppContext, c.Params.TeamId, c.Params.UserId, images[0])
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	writeJSON(c, w, profile)
}

// getCommunityAvatar gives someone's picture in a community to those who may see the community; 404 when they have
// none (their profile picture shows instead).
func getCommunityAvatar(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId().RequireUserId()
	if c.Err != nil {
		return
	}
	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamId, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}
	img, profile, appErr := c.App.GetCommunityAvatar(c.Params.TeamId, c.Params.UserId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	if img == nil {
		c.Err = model.NewAppError("getCommunityAvatar", "api.community_profile.no_avatar.app_error", nil, "", http.StatusNotFound)
		return
	}
	etag := strconv.FormatInt(profile.AvatarUpdateAt, 10)
	if c.HandleEtag(etag, "Get Community Avatar", w, r) {
		return
	}
	w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%v, private", model.DayInSeconds))
	w.Header().Set(model.HeaderEtagServer, etag)
	w.Header().Set("Content-Type", "image/png")
	if _, err := w.Write(img); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}
