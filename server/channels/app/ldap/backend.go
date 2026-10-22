// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package ldap

import (
	"errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Backend is the set of server operations used by the LDAP implementation.
// The app layer implements it (channels/app/ldap_backend.go); the tests use an
// in-memory fake, which keeps the directory logic testable without a database.
type Backend interface {
	Config() *model.Config
	Logger() mlog.LoggerIFace
	GetConfigFile(name string) ([]byte, error)

	// Users
	GetUser(rctx request.CTX, id string) (*model.User, error)
	GetUserByAuth(authData string, service string) (*model.User, error)
	GetUsersByAuthService(service string) ([]*model.User, error)
	CreateUser(rctx request.CTX, user *model.User, guest bool) (*model.User, *model.AppError)
	// SaveUser persists all the profile fields of the given user, including
	// username and email, bypassing the restrictions applied to changes
	// requested by the users themselves.
	SaveUser(rctx request.CTX, user *model.User) (*model.User, error)
	SetUserActive(rctx request.CTX, user *model.User, active bool) error
	UpdateUserRoles(rctx request.CTX, userID string, roles string) error
	DemoteUserToGuest(rctx request.CTX, user *model.User) error
	PromoteGuestToUser(rctx request.CTX, user *model.User) error
	UpdateAuthData(userID string, service string, authData string) error
	SetProfileImage(rctx request.CTX, userID string, data []byte) error
	SetDefaultProfileImage(rctx request.CTX, user *model.User) error

	// Groups
	GetLdapGroups() ([]*model.Group, error)
	GetLdapGroupByRemoteID(remoteID string) (*model.Group, error)
	GetLdapGroupsForUser(userID string) ([]*model.Group, error)
	UpdateGroup(group *model.Group) error
	DeleteGroup(groupID string) error
	GroupHasSyncables(groupID string) (bool, error)
	GetGroupMemberIDs(groupID string) ([]string, error)
	AddGroupMembers(groupID string, userIDs []string) error
	RemoveGroupMembers(groupID string, userIDs []string) error

	// Group syncables
	CreateDefaultMemberships(rctx request.CTX, params model.CreateDefaultMembershipParams) error
	DeleteGroupConstrainedMemberships(rctx request.CTX) error
	SyncGroupConstrainedRoles(rctx request.CTX, groupIDs []string) error

	// User attributes (custom profile attributes)
	//
	// NewAttributeSyncer returns the syncer of the user attributes synced from
	// AD/LDAP, loaded at the time of the call: one per synchronization run or
	// sign-in.
	NewAttributeSyncer(rctx request.CTX) (AttributeSyncer, *model.AppError)

	// Jobs
	CreateSyncJob(rctx request.CTX, data map[string]string) (*model.Job, *model.AppError)
	GetJob(rctx request.CTX, id string) (*model.Job, *model.AppError)
	GetLastSuccessfulSyncJob() (*model.Job, error)
}

// errNotFound is returned by the backend lookups when nothing matches.
var errNotFound = errors.New("not found")

func isNotFound(err error) bool {
	if errors.Is(err, errNotFound) {
		return true
	}
	var nfErr *store.ErrNotFound
	return errors.As(err, &nfErr)
}
