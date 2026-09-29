// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package accesscontrol

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest/mocks"
)

// Field IDs used across tests.
var (
	testGroupID      = model.NewId()
	fieldDepartment  = model.NewId()
	fieldClearance   = model.NewId()
	fieldSkills      = model.NewId()
	fieldPrograms    = model.NewId()
	fieldChanLevel   = model.NewId()
	fieldChanProgram = model.NewId()
	fieldChanDept    = model.NewId()
)

// Graph options of the "programs" field:
//
//	all
//	├── air
//	│   └── f18
//	└── sea
var (
	optAll = "opt-all"
	optAir = "opt-air"
	optF18 = "opt-f18"
	optSea = "opt-sea"
)

var graphNames = map[string]string{"All Programs": optAll, "Air": optAir, "F-18 Program": optF18, "Sea": optSea}

var graphParents = map[string][]string{optAir: {optAll}, optF18: {optAir}, optSea: {optAll}}

func graphUp(id string) []string {
	out := []string{id}
	for _, p := range graphParents[id] {
		out = append(out, graphUp(p)...)
	}
	return out
}

func graphDown(id string) []string {
	out := []string{id}
	for child, parents := range graphParents {
		for _, p := range parents {
			if p == id {
				out = append(out, graphDown(child)...)
			}
		}
	}
	return out
}

func testFields() []*model.PropertyField {
	rankOptions := []any{
		map[string]any{"id": model.NewId(), "name": "Public", "rank": float64(1)},
		map[string]any{"id": model.NewId(), "name": "Secret", "rank": float64(2)},
		map[string]any{"id": model.NewId(), "name": "TopSecret", "rank": float64(3)},
	}
	return []*model.PropertyField{
		{ID: fieldDepartment, GroupID: testGroupID, Name: "department", Type: model.PropertyFieldTypeText, ObjectType: model.PropertyFieldObjectTypeUser},
		{ID: fieldClearance, GroupID: testGroupID, Name: "clearance", Type: model.PropertyFieldTypeRank, ObjectType: model.PropertyFieldObjectTypeUser,
			Attrs: model.StringInterface{model.PropertyFieldAttributeOptions: rankOptions}},
		{ID: fieldSkills, GroupID: testGroupID, Name: "skills", Type: model.PropertyFieldTypeMultiselect, ObjectType: model.PropertyFieldObjectTypeUser},
		{ID: fieldPrograms, GroupID: testGroupID, Name: "programs", Type: model.PropertyFieldTypeGraph, ObjectType: model.PropertyFieldObjectTypeUser},
		{ID: fieldChanLevel, GroupID: testGroupID, Name: "minClearance", Type: model.PropertyFieldTypeRank, ObjectType: model.PropertyFieldObjectTypeChannel,
			Attrs: model.StringInterface{model.PropertyFieldAttributeOptions: rankOptions}},
		{ID: fieldChanProgram, GroupID: testGroupID, Name: "channelPrograms", Type: model.PropertyFieldTypeGraph, ObjectType: model.PropertyFieldObjectTypeChannel},
		{ID: fieldChanDept, GroupID: testGroupID, Name: "department", Type: model.PropertyFieldTypeText, ObjectType: model.PropertyFieldObjectTypeChannel},
	}
}

type testEnv struct {
	t     *testing.T
	rctx  request.CTX
	cfg   *model.Config
	st    *mocks.Store
	acp   *mocks.AccessControlPolicyStore
	attrs *mocks.AttributesStore
	pf    *mocks.PropertyFieldStore
	pg    *mocks.PropertyGroupStore
	svc   *Service

	mu       sync.Mutex
	policies map[string]*model.AccessControlPolicy
	// channelAttrs are raw channel attribute values by channel ID.
	channelAttrs map[string]map[string]any
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	cfg := &model.Config{}
	cfg.SetDefaults()
	cfg.AccessControlSettings.EnableAttributeBasedAccessControl = model.NewPointer(true)
	cfg.FeatureFlags.PermissionPolicies = true
	cfg.FeatureFlags.ChannelPermissionPolicies = true
	cfg.FeatureFlags.TeamMembershipAccessControl = true
	cfg.FeatureFlags.ResourceAttributesInPolicies = true
	cfg.FeatureFlags.PolicySimulation = true

	e := &testEnv{
		t:            t,
		rctx:         request.TestContext(t),
		cfg:          cfg,
		st:           &mocks.Store{},
		acp:          &mocks.AccessControlPolicyStore{},
		attrs:        &mocks.AttributesStore{},
		pf:           &mocks.PropertyFieldStore{},
		pg:           &mocks.PropertyGroupStore{},
		policies:     map[string]*model.AccessControlPolicy{},
		channelAttrs: map[string]map[string]any{},
	}

	e.st.On("AccessControlPolicy").Return(e.acp)
	e.st.On("Attributes").Return(e.attrs)
	e.st.On("PropertyField").Return(e.pf)
	e.st.On("PropertyGroup").Return(e.pg)

	e.pg.On("Get", model.AccessControlPropertyGroupName).Return(&model.PropertyGroup{ID: testGroupID, Name: model.AccessControlPropertyGroupName}, nil)
	e.pf.On("GetForGroup", mock.Anything, testGroupID).Return(testFields(), nil)
	e.pf.On("GetOptionsByName", mock.Anything, mock.Anything).Return(func(field *model.PropertyField, names []string) ([]*model.PropertyFieldOption, error) {
		var out []*model.PropertyFieldOption
		for _, n := range names {
			if id, ok := graphNames[n]; ok {
				out = append(out, &model.PropertyFieldOption{ID: id, Name: n})
			}
		}
		return out, nil
	})
	e.pf.On("GetOptionsByID", mock.Anything, mock.Anything).Return(func(field *model.PropertyField, ids []string) ([]*model.PropertyFieldOption, error) {
		var out []*model.PropertyFieldOption
		for n, id := range graphNames {
			for _, want := range ids {
				if want == id {
					out = append(out, &model.PropertyFieldOption{ID: id, Name: n})
				}
			}
		}
		return out, nil
	})
	e.pf.On("GetOptionAncestorsOrSelf", mock.Anything, mock.Anything).Return(func(field *model.PropertyField, ids []string) (map[string][]string, error) {
		out := map[string][]string{}
		for _, id := range ids {
			out[id] = graphUp(id)
		}
		return out, nil
	})
	e.pf.On("GetOptionDescendantsOrSelf", mock.Anything, mock.Anything).Return(func(field *model.PropertyField, ids []string) (map[string][]string, error) {
		out := map[string][]string{}
		for _, id := range ids {
			out[id] = graphDown(id)
		}
		return out, nil
	})

	e.acp.On("Get", mock.Anything, mock.Anything).Return(func(_ request.CTX, id string) (*model.AccessControlPolicy, error) {
		e.mu.Lock()
		defer e.mu.Unlock()
		if p, ok := e.policies[id]; ok {
			return clonePolicy(p), nil
		}
		return nil, store.NewErrNotFound("AccessControlPolicy", id)
	})
	e.acp.On("SearchPolicies", mock.Anything, mock.Anything).Return(func(_ request.CTX, opts model.AccessControlPolicySearch) ([]*model.AccessControlPolicy, int64, error) {
		e.mu.Lock()
		defer e.mu.Unlock()
		var out []*model.AccessControlPolicy
		for _, p := range e.policies {
			if opts.Type != "" && p.Type != opts.Type {
				continue
			}
			if opts.ParentID != "" {
				found := false
				for _, imp := range p.Imports {
					if imp == opts.ParentID {
						found = true
					}
				}
				if !found {
					continue
				}
			}
			out = append(out, clonePolicy(p))
		}
		return out, int64(len(out)), nil
	})
	e.acp.On("Save", mock.Anything, mock.Anything).Return(func(_ request.CTX, p *model.AccessControlPolicy) (*model.AccessControlPolicy, error) {
		if appErr := p.IsValid(); appErr != nil {
			return nil, appErr
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		saved := clonePolicy(p)
		saved.Revision++
		e.policies[p.ID] = saved
		return clonePolicy(saved), nil
	})
	e.acp.On("Delete", mock.Anything, mock.Anything).Return(func(_ request.CTX, id string) error {
		e.mu.Lock()
		defer e.mu.Unlock()
		delete(e.policies, id)
		return nil
	})

	e.attrs.On("RefreshAttributes").Return(nil)
	e.attrs.On("GetSubject", mock.Anything, mock.Anything, testGroupID, model.PropertyFieldObjectTypeChannel).Return(
		func(_ request.CTX, id, _, _ string) (*model.Subject, error) {
			e.mu.Lock()
			defer e.mu.Unlock()
			if a, ok := e.channelAttrs[id]; ok {
				return &model.Subject{ID: id, Type: "channel", Attributes: a}, nil
			}
			return nil, store.NewErrNotFound("Attributes", id)
		})

	e.svc = New(Backend{
		Store:  func() store.Store { return e.st },
		Config: func() *model.Config { return e.cfg },
		Logger: func() mlog.LoggerIFace { return mlog.CreateConsoleTestLogger(t) },
	})
	e.svc.now = func() time.Time { return time.UnixMilli(1_700_000_000_000) }
	return e
}

func (e *testEnv) addPolicy(p *model.AccessControlPolicy) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.policies[p.ID] = clonePolicy(p)
}

func subjectWith(attrs map[string]any) model.Subject {
	s := model.Subject{ID: model.NewId(), Type: "user", Attributes: attrs, Role: model.SystemUserRoleId}
	s.SetScopedRole(model.AccessControlSubjectScopeSystem, model.SystemUserRoleId)
	return s
}
