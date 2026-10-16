// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app/ldap"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// ldapBackend implements ldap.Backend on top of the application layer.
type ldapBackend struct {
	a *App
}

var _ ldap.Backend = (*ldapBackend)(nil)

func (b *ldapBackend) Config() *model.Config    { return b.a.Config() }
func (b *ldapBackend) Logger() mlog.LoggerIFace { return b.a.Log() }
func (b *ldapBackend) store() store.Store       { return b.a.Srv().Store() }
func (b *ldapBackend) GetConfigFile(name string) ([]byte, error) {
	return b.a.Srv().Platform().GetConfigFile(name)
}

func (b *ldapBackend) GetUser(rctx request.CTX, id string) (*model.User, error) {
	return b.store().User().Get(rctx, id)
}

func (b *ldapBackend) GetUserByAuth(authData string, service string) (*model.User, error) {
	return b.store().User().GetByAuth(&authData, service)
}

func (b *ldapBackend) GetUsersByAuthService(service string) ([]*model.User, error) {
	return b.store().User().GetAllUsingAuthService(service)
}

func (b *ldapBackend) CreateUser(rctx request.CTX, user *model.User, guest bool) (*model.User, *model.AppError) {
	if guest {
		return b.a.CreateGuest(rctx, user)
	}
	return b.a.CreateUser(rctx, user)
}

func (b *ldapBackend) SaveUser(rctx request.CTX, user *model.User) (*model.User, error) {
	update, err := b.store().User().Update(rctx, user, true)
	if err != nil {
		return nil, err
	}
	newUser := update.New
	b.a.InvalidateCacheForUser(newUser.Id)

	if newUser.Username != update.Old.Username && newUser.LastPictureUpdate <= 0 {
		if appErr := b.a.UpdateDefaultProfileImage(rctx, newUser); appErr != nil {
			rctx.Logger().Warn("Failed to update default profile image after username change", mlog.String("user_id", newUser.Id), mlog.Err(appErr))
		}
	}

	b.publishUserUpdated(newUser)
	if syncService := b.a.Srv().GetSharedChannelSyncService(); syncService != nil && syncService.Active() {
		syncService.NotifyUserProfileChanged(newUser.Id)
	}
	return newUser, nil
}

// publishUserUpdated broadcasts the user_updated websocket event, with the
// same sanitization levels as the application layer.
func (b *ldapBackend) publishUserUpdated(user *model.User) {
	omit := map[string]bool{user.Id: true}

	adminCopy := user.DeepCopy()
	b.a.SanitizeProfile(adminCopy, true)
	adminMessage := model.NewWebSocketEvent(model.WebsocketEventUserUpdated, "", "", "", omit, "")
	adminMessage.Add("user", adminCopy)
	adminMessage.GetBroadcast().ContainsSensitiveData = true
	b.a.Publish(adminMessage)

	publicCopy := user.DeepCopy()
	b.a.SanitizeProfile(publicCopy, false)
	message := model.NewWebSocketEvent(model.WebsocketEventUserUpdated, "", "", "", omit, "")
	message.Add("user", publicCopy)
	message.GetBroadcast().ContainsSanitizedData = true
	b.a.Publish(message)

	selfCopy := user.DeepCopy()
	selfCopy.Sanitize(nil)
	selfMessage := model.NewWebSocketEvent(model.WebsocketEventUserUpdated, "", "", selfCopy.Id, nil, "")
	selfMessage.Add("user", selfCopy)
	b.a.Publish(selfMessage)
}

func (b *ldapBackend) SetUserActive(rctx request.CTX, user *model.User, active bool) error {
	if _, appErr := b.a.UpdateActive(rctx, user, active); appErr != nil {
		return appErr
	}
	return nil
}

func (b *ldapBackend) UpdateUserRoles(rctx request.CTX, userID string, roles string) error {
	if _, appErr := b.a.UpdateUserRoles(rctx, userID, roles, true); appErr != nil {
		return appErr
	}
	return nil
}

func (b *ldapBackend) DemoteUserToGuest(rctx request.CTX, user *model.User) error {
	if appErr := b.a.DemoteUserToGuest(rctx, user); appErr != nil {
		return appErr
	}
	return nil
}

func (b *ldapBackend) PromoteGuestToUser(rctx request.CTX, user *model.User) error {
	if appErr := b.a.PromoteGuestToUser(rctx, user, ""); appErr != nil {
		return appErr
	}
	return nil
}

func (b *ldapBackend) UpdateAuthData(userID string, service string, authData string) error {
	if _, err := b.store().User().UpdateAuthData(userID, service, &authData, "", false); err != nil {
		return err
	}
	b.a.InvalidateCacheForUser(userID)
	return nil
}

func (b *ldapBackend) SetProfileImage(rctx request.CTX, userID string, data []byte) error {
	if appErr := b.a.SetProfileImageFromFile(rctx, userID, bytes.NewReader(data)); appErr != nil {
		return appErr
	}
	return nil
}

func (b *ldapBackend) SetDefaultProfileImage(rctx request.CTX, user *model.User) error {
	if appErr := b.a.SetDefaultProfileImage(rctx, user); appErr != nil {
		return appErr
	}
	return nil
}

func (b *ldapBackend) GetLdapGroups() ([]*model.Group, error) {
	return b.store().Group().GetAllBySource(model.GroupSourceLdap)
}

func (b *ldapBackend) GetLdapGroupByRemoteID(remoteID string) (*model.Group, error) {
	return b.store().Group().GetByRemoteID(remoteID, model.GroupSourceLdap)
}

func (b *ldapBackend) GetLdapGroupsForUser(userID string) ([]*model.Group, error) {
	groups, err := b.store().Group().GetByUser(userID, model.GroupSearchOpts{})
	if err != nil {
		return nil, err
	}
	res := make([]*model.Group, 0, len(groups))
	for _, g := range groups {
		if g.Source == model.GroupSourceLdap && g.DeleteAt == 0 {
			res = append(res, g)
		}
	}
	return res, nil
}

func (b *ldapBackend) UpdateGroup(group *model.Group) error {
	if _, appErr := b.a.UpdateGroup(group); appErr != nil {
		return appErr
	}
	return nil
}

func (b *ldapBackend) DeleteGroup(groupID string) error {
	if _, appErr := b.a.DeleteGroup(groupID); appErr != nil {
		return appErr
	}
	return nil
}

func (b *ldapBackend) GroupHasSyncables(groupID string) (bool, error) {
	for _, syncableType := range []model.GroupSyncableType{model.GroupSyncableTypeTeam, model.GroupSyncableTypeChannel} {
		syncables, err := b.store().Group().GetAllGroupSyncablesByGroupId(groupID, syncableType)
		if err != nil {
			return false, err
		}
		if len(syncables) > 0 {
			return true, nil
		}
	}
	return false, nil
}

func (b *ldapBackend) GetGroupMemberIDs(groupID string) ([]string, error) {
	users, err := b.store().Group().GetMemberUsers(groupID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.Id)
	}
	return ids, nil
}

func (b *ldapBackend) AddGroupMembers(groupID string, userIDs []string) error {
	if _, appErr := b.a.UpsertGroupMembers(groupID, userIDs); appErr != nil {
		return appErr
	}
	return nil
}

func (b *ldapBackend) RemoveGroupMembers(groupID string, userIDs []string) error {
	if _, appErr := b.a.DeleteGroupMembers(groupID, userIDs); appErr != nil {
		return appErr
	}
	return nil
}

func (b *ldapBackend) CreateDefaultMemberships(rctx request.CTX, params model.CreateDefaultMembershipParams) error {
	return b.a.CreateDefaultMemberships(rctx, params)
}

func (b *ldapBackend) DeleteGroupConstrainedMemberships(rctx request.CTX) error {
	return b.a.DeleteGroupConstrainedMemberships(rctx)
}

// SyncGroupConstrainedRoles synchronizes the team/channel admin roles of the
// group-constrained teams and channels linked to the given groups.
func (b *ldapBackend) SyncGroupConstrainedRoles(rctx request.CTX, groupIDs []string) error {
	teams := map[string]bool{}
	channels := map[string]bool{}
	for _, groupID := range groupIDs {
		for _, syncableType := range []model.GroupSyncableType{model.GroupSyncableTypeTeam, model.GroupSyncableTypeChannel} {
			syncables, err := b.store().Group().GetAllGroupSyncablesByGroupId(groupID, syncableType)
			if err != nil {
				return err
			}
			for _, gs := range syncables {
				if syncableType == model.GroupSyncableTypeTeam {
					teams[gs.SyncableId] = true
				} else {
					channels[gs.SyncableId] = true
				}
			}
		}
	}

	var errs []error
	for teamID := range teams {
		team, err := b.store().Team().Get(teamID)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if team.GroupConstrained == nil || !*team.GroupConstrained {
			continue
		}
		if appErr := b.a.SyncSyncableRoles(rctx, teamID, model.GroupSyncableTypeTeam); appErr != nil {
			errs = append(errs, appErr)
		}
	}
	for channelID := range channels {
		channel, err := b.store().Channel().Get(channelID, true)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if channel.GroupConstrained == nil || !*channel.GroupConstrained {
			continue
		}
		if appErr := b.a.SyncSyncableRoles(rctx, channelID, model.GroupSyncableTypeChannel); appErr != nil {
			errs = append(errs, appErr)
		}
	}
	return errors.Join(errs...)
}

// withLdapSyncCaller identifies the AD/LDAP synchronization as the caller of
// property operations, which is required to write synced attribute values.
func withLdapSyncCaller(rctx request.CTX) request.CTX {
	return RequestContextWithCallerID(rctx, model.CallerIDLDAPSync)
}

func (b *ldapBackend) GetCPAGroupID(rctx request.CTX) (string, error) {
	rctx = withLdapSyncCaller(rctx)
	group, appErr := b.a.GetPropertyGroup(rctx, model.AccessControlPropertyGroupName)
	if appErr != nil {
		return "", appErr
	}
	return group.ID, nil
}

func (b *ldapBackend) GetCPAFields(rctx request.CTX, groupID string) ([]*model.PropertyField, error) {
	rctx = withLdapSyncCaller(rctx)
	fields, appErr := b.a.SearchPropertyFields(rctx, groupID, model.PropertyFieldSearchOpts{
		GroupID:    groupID,
		ObjectType: model.PropertyFieldObjectTypeUser,
		PerPage:    model.AccessControlGroupFieldLimit + 5,
	})
	if appErr != nil {
		return nil, appErr
	}
	return fields, nil
}

func (b *ldapBackend) GetCPAValues(rctx request.CTX, groupID string, userID string) ([]*model.PropertyValue, error) {
	rctx = withLdapSyncCaller(rctx)
	values, appErr := b.a.SearchPropertyValues(rctx, groupID, model.PropertyValueSearchOpts{
		GroupID:    groupID,
		TargetType: model.PropertyFieldObjectTypeUser,
		TargetIDs:  []string{userID},
		PerPage:    model.AccessControlGroupFieldLimit + 5,
	})
	if appErr != nil {
		return nil, appErr
	}
	return values, nil
}

func (b *ldapBackend) UpsertCPAValues(rctx request.CTX, userID string, values []*model.PropertyValue) error {
	rctx = withLdapSyncCaller(rctx)
	upserted, appErr := b.a.UpsertPropertyValues(rctx, values, model.PropertyFieldObjectTypeUser, userID, "")
	if appErr != nil {
		return appErr
	}
	b.publishCPAValues(rctx, userID, upserted)
	return nil
}

// publishCPAValues sends the legacy custom profile attributes websocket
// event, withholding the values of the fields that are not public.
func (b *ldapBackend) publishCPAValues(rctx request.CTX, userID string, values []*model.PropertyValue) {
	if len(values) == 0 {
		return
	}
	fieldIDs := make([]string, 0, len(values))
	for _, v := range values {
		fieldIDs = append(fieldIDs, v.FieldID)
	}
	fieldByID := map[string]*model.PropertyField{}
	if fields, appErr := b.a.GetPropertyFields(rctx, values[0].GroupID, fieldIDs); appErr == nil {
		for _, f := range fields {
			fieldByID[f.ID] = f
		}
	}

	res := make(map[string]json.RawMessage, len(values))
	for _, v := range values {
		res[v.FieldID] = model.BroadcastValue(fieldByID[v.FieldID], v.Value)
	}
	message := model.NewWebSocketEvent(model.WebsocketEventCPAValuesUpdated, "", "", "", nil, "")
	message.Add("user_id", userID)
	message.Add("values", res)
	b.a.Publish(message)
}

func (b *ldapBackend) DeleteCPAValue(rctx request.CTX, groupID string, valueID string) error {
	rctx = withLdapSyncCaller(rctx)
	if appErr := b.a.DeletePropertyValue(rctx, groupID, valueID); appErr != nil {
		return appErr
	}
	return nil
}

func (b *ldapBackend) CreateSyncJob(rctx request.CTX, data map[string]string) (*model.Job, *model.AppError) {
	jobServer := b.a.Srv().Jobs
	if jobServer == nil {
		return nil, model.NewAppError("CreateSyncJob", "app.job.save.app_error", nil, "job server not available", http.StatusInternalServerError)
	}
	return jobServer.CreateJob(rctx, model.JobTypeLdapSync, data)
}

func (b *ldapBackend) GetJob(rctx request.CTX, id string) (*model.Job, *model.AppError) {
	jobServer := b.a.Srv().Jobs
	if jobServer == nil {
		return nil, model.NewAppError("GetJob", "app.job.get.app_error", nil, "job server not available", http.StatusInternalServerError)
	}
	return jobServer.GetJob(rctx, id)
}

func (b *ldapBackend) GetLastSuccessfulSyncJob() (*model.Job, error) {
	job, err := b.store().Job().GetNewestJobByStatusAndType(model.JobStatusSuccess, model.JobTypeLdapSync)
	if err != nil {
		var nfErr *store.ErrNotFound
		if errors.As(err, &nfErr) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}
