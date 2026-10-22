// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package ldap

import (
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// fakeBackend is an in-memory implementation of backend.
type fakeBackend struct {
	mu sync.Mutex

	cfg    *model.Config
	logger *mlog.Logger
	files  map[string][]byte

	users        map[string]*model.User
	groups       map[string]*model.Group
	members      map[string]map[string]bool // group ID -> user IDs
	hasSyncables map[string]bool

	attrSyncer    *fakeAttributeSyncer
	attrSyncerErr *model.AppError

	pictures       map[string][]byte
	defaultPicture map[string]int

	defaultMemberships []model.CreateDefaultMembershipParams
	deleteConstrained  int
	syncedRoles        [][]string

	jobs       map[string]*model.Job
	lastSyncOK *model.Job
}

func newFakeBackend() *fakeBackend {
	cfg := &model.Config{}
	cfg.SetDefaults()
	ls := &cfg.LdapSettings
	*ls.Enable = true
	*ls.EnableSync = true
	*ls.LdapServer = "ldap.example.com"
	*ls.BaseDN = "dc=example,dc=com"
	*ls.BindUsername = "cn=admin,dc=example,dc=com"
	*ls.BindPassword = "adminpwd"
	*ls.IdAttribute = "uid"
	ls.LoginIdAttribute = new("uid") // SetDefaults aliases it to IdAttribute
	*ls.UsernameAttribute = "uid"
	*ls.EmailAttribute = "mail"
	*ls.FirstNameAttribute = "givenName"
	*ls.LastNameAttribute = "sn"
	*ls.NicknameAttribute = "displayName"
	*ls.PositionAttribute = "title"
	*ls.UserFilter = "(objectClass=inetOrgPerson)"
	*ls.GroupIdAttribute = "entryUUID"
	*ls.GroupDisplayNameAttribute = "cn"

	logger, _ := mlog.NewLogger()
	return &fakeBackend{
		cfg:            cfg,
		logger:         logger,
		files:          map[string][]byte{},
		users:          map[string]*model.User{},
		groups:         map[string]*model.Group{},
		members:        map[string]map[string]bool{},
		hasSyncables:   map[string]bool{},
		pictures:       map[string][]byte{},
		defaultPicture: map[string]int{},
		jobs:           map[string]*model.Job{},
	}
}

func (b *fakeBackend) rctx() request.CTX { return request.EmptyContext(b.logger) }

func (b *fakeBackend) Config() *model.Config    { return b.cfg }
func (b *fakeBackend) Logger() mlog.LoggerIFace { return b.logger }
func (b *fakeBackend) GetConfigFile(name string) ([]byte, error) {
	if f, ok := b.files[name]; ok {
		return f, nil
	}
	return nil, errNotFound
}

func (b *fakeBackend) addUser(u *model.User) *model.User {
	b.mu.Lock()
	defer b.mu.Unlock()
	if u.Id == "" {
		u.Id = model.NewId()
	}
	if u.Roles == "" {
		u.Roles = model.SystemUserRoleId
	}
	if u.Props == nil {
		u.Props = model.StringMap{}
	}
	b.users[u.Id] = u.DeepCopy()
	return u
}

func (b *fakeBackend) user(id string) *model.User {
	b.mu.Lock()
	defer b.mu.Unlock()
	if u, ok := b.users[id]; ok {
		return u.DeepCopy()
	}
	return nil
}

func (b *fakeBackend) userByAuth(authData string) *model.User {
	u, _ := b.GetUserByAuth(authData, model.UserAuthServiceLdap)
	return u
}

func (b *fakeBackend) GetUser(_ request.CTX, id string) (*model.User, error) {
	if u := b.user(id); u != nil {
		return u, nil
	}
	return nil, errNotFound
}

func (b *fakeBackend) GetUserByAuth(authData string, service string) (*model.User, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, u := range b.users {
		if u.AuthService == service && u.AuthData != nil && *u.AuthData == authData {
			return u.DeepCopy(), nil
		}
	}
	return nil, errNotFound
}

func (b *fakeBackend) GetUsersByAuthService(service string) ([]*model.User, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var res []*model.User
	for _, u := range b.users {
		if u.AuthService == service {
			res = append(res, u.DeepCopy())
		}
	}
	slices.SortFunc(res, func(a, c *model.User) int { return strings.Compare(a.Username, c.Username) })
	return res, nil
}

func (b *fakeBackend) CreateUser(_ request.CTX, user *model.User, guest bool) (*model.User, *model.AppError) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, u := range b.users {
		if u.Email == user.Email {
			return nil, model.NewAppError("CreateUser", "app.user.save.email_exists.app_error", nil, "", http.StatusBadRequest)
		}
		if u.Username == user.Username {
			return nil, model.NewAppError("CreateUser", "app.user.save.username_exists.app_error", nil, "", http.StatusBadRequest)
		}
	}
	u := user.DeepCopy()
	u.Id = model.NewId()
	u.Roles = model.SystemUserRoleId
	if guest {
		u.Roles = model.SystemGuestRoleId
	}
	if u.Props == nil {
		u.Props = model.StringMap{}
	}
	u.CreateAt = model.GetMillis()
	b.users[u.Id] = u
	return u.DeepCopy(), nil
}

func (b *fakeBackend) SaveUser(_ request.CTX, user *model.User) (*model.User, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, u := range b.users {
		if u.Id != user.Id && (u.Username == user.Username || u.Email == user.Email) {
			return nil, model.NewAppError("SaveUser", "app.user.save.username_exists.app_error", nil, "", http.StatusBadRequest)
		}
	}
	b.users[user.Id] = user.DeepCopy()
	return user.DeepCopy(), nil
}

func (b *fakeBackend) SetUserActive(_ request.CTX, user *model.User, active bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	u := b.users[user.Id]
	if active {
		u.DeleteAt = 0
	} else {
		u.DeleteAt = model.GetMillis()
	}
	return nil
}

func (b *fakeBackend) UpdateUserRoles(_ request.CTX, userID string, roles string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.users[userID].Roles = roles
	return nil
}

func (b *fakeBackend) DemoteUserToGuest(_ request.CTX, user *model.User) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.users[user.Id].Roles = model.SystemGuestRoleId
	return nil
}

func (b *fakeBackend) PromoteGuestToUser(_ request.CTX, user *model.User) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.users[user.Id].Roles = model.SystemUserRoleId
	return nil
}

func (b *fakeBackend) UpdateAuthData(userID string, service string, authData string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	u := b.users[userID]
	u.AuthService = service
	u.AuthData = &authData
	u.Password = ""
	return nil
}

func (b *fakeBackend) SetProfileImage(_ request.CTX, userID string, data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pictures[userID] = data
	b.users[userID].LastPictureUpdate = model.GetMillis()
	return nil
}

func (b *fakeBackend) SetDefaultProfileImage(_ request.CTX, user *model.User) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.defaultPicture[user.Id]++
	delete(b.pictures, user.Id)
	b.users[user.Id].LastPictureUpdate = -model.GetMillis()
	return nil
}

func (b *fakeBackend) linkGroup(remoteID, displayName string) *model.Group {
	b.mu.Lock()
	defer b.mu.Unlock()
	g := &model.Group{Id: model.NewId(), RemoteId: &remoteID, DisplayName: displayName, Source: model.GroupSourceLdap}
	b.groups[g.Id] = g
	b.members[g.Id] = map[string]bool{}
	return g
}

func (b *fakeBackend) memberIDs(groupID string) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var ids []string
	for id := range b.members[groupID] {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func (b *fakeBackend) GetLdapGroups() ([]*model.Group, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var res []*model.Group
	for _, g := range b.groups {
		if g.DeleteAt == 0 {
			c := *g
			res = append(res, &c)
		}
	}
	slices.SortFunc(res, func(a, c *model.Group) int { return strings.Compare(a.Id, c.Id) })
	return res, nil
}

func (b *fakeBackend) GetLdapGroupByRemoteID(remoteID string) (*model.Group, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, g := range b.groups {
		if g.RemoteId != nil && *g.RemoteId == remoteID {
			c := *g
			return &c, nil
		}
	}
	return nil, errNotFound
}

func (b *fakeBackend) GetLdapGroupsForUser(userID string) ([]*model.Group, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var res []*model.Group
	for id, m := range b.members {
		if m[userID] && b.groups[id].DeleteAt == 0 {
			c := *b.groups[id]
			res = append(res, &c)
		}
	}
	return res, nil
}

func (b *fakeBackend) UpdateGroup(group *model.Group) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := *group
	b.groups[group.Id] = &c
	return nil
}

func (b *fakeBackend) DeleteGroup(groupID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.groups[groupID].DeleteAt = model.GetMillis()
	return nil
}

func (b *fakeBackend) GroupHasSyncables(groupID string) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.hasSyncables[groupID], nil
}

func (b *fakeBackend) GetGroupMemberIDs(groupID string) ([]string, error) {
	return b.memberIDs(groupID), nil
}

func (b *fakeBackend) AddGroupMembers(groupID string, userIDs []string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, id := range userIDs {
		b.members[groupID][id] = true
	}
	return nil
}

func (b *fakeBackend) RemoveGroupMembers(groupID string, userIDs []string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, id := range userIDs {
		delete(b.members[groupID], id)
	}
	return nil
}

func (b *fakeBackend) CreateDefaultMemberships(_ request.CTX, params model.CreateDefaultMembershipParams) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.defaultMemberships = append(b.defaultMemberships, params)
	return nil
}

func (b *fakeBackend) DeleteGroupConstrainedMemberships(_ request.CTX) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.deleteConstrained++
	return nil
}

func (b *fakeBackend) SyncGroupConstrainedRoles(_ request.CTX, groupIDs []string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.syncedRoles = append(b.syncedRoles, slices.Clone(groupIDs))
	return nil
}

func (b *fakeBackend) NewAttributeSyncer(_ request.CTX) (AttributeSyncer, *model.AppError) {
	if b.attrSyncerErr != nil {
		return nil, b.attrSyncerErr
	}
	if b.attrSyncer == nil {
		return nil, nil
	}
	b.attrSyncer.loads++
	return b.attrSyncer, nil
}

// fakeAttributeSyncer records what the AD/LDAP service hands to the
// PropertySyncer; the syncer itself is tested in the app package.
type fakeAttributeSyncer struct {
	mu         sync.Mutex
	attributes []string
	loads      int
	synced     map[string]map[string][]string // user ID -> attrs of the last sync
	syncOpts   map[string]model.PropertySyncOptions
	syncCalls  int
	prunes     int
}

func newFakeAttributeSyncer(attributes ...string) *fakeAttributeSyncer {
	return &fakeAttributeSyncer{
		attributes: attributes,
		synced:     map[string]map[string][]string{},
		syncOpts:   map[string]model.PropertySyncOptions{},
	}
}

func (f *fakeAttributeSyncer) HasMappings() bool { return len(f.attributes) > 0 }

func (f *fakeAttributeSyncer) ExternalAttributes() []string { return slices.Clone(f.attributes) }

func (f *fakeAttributeSyncer) SyncUser(_ request.CTX, userID string, attrs map[string][]string, opts model.PropertySyncOptions) (*model.PropertySyncResult, *model.AppError) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.syncCalls++
	f.synced[userID] = attrs
	f.syncOpts[userID] = opts
	return &model.PropertySyncResult{UserID: userID}, nil
}

func (f *fakeAttributeSyncer) PruneOrphanedOptions(_ request.CTX) (*model.PropertySyncPruneResult, *model.AppError) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prunes++
	return &model.PropertySyncPruneResult{}, nil
}

func (b *fakeBackend) CreateSyncJob(_ request.CTX, data map[string]string) (*model.Job, *model.AppError) {
	b.mu.Lock()
	defer b.mu.Unlock()
	job := &model.Job{Id: model.NewId(), Type: model.JobTypeLdapSync, Status: model.JobStatusSuccess, Data: data}
	b.jobs[job.Id] = job
	return job, nil
}

func (b *fakeBackend) GetJob(_ request.CTX, id string) (*model.Job, *model.AppError) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if j, ok := b.jobs[id]; ok {
		return j, nil
	}
	return nil, model.NewAppError("GetJob", "app.job.get.app_error", nil, "", http.StatusNotFound)
}

func (b *fakeBackend) GetLastSuccessfulSyncJob() (*model.Job, error) {
	return b.lastSyncOK, nil
}
