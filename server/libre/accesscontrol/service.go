// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package accesscontrol is a free implementation of the attribute-based access
// control service (policy administration and decision points) behind
// einterfaces.AccessControlServiceInterface, plus the membership sync jobs.
//
// Policies are CEL expressions over the requesting user (user.attributes.*,
// user.session.*, and the native user.email/verified/isbot/createat/id) and,
// optionally, the accessed resource (resource.attributes.*).
package accesscontrol

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

const (
	// policyCacheTTL bounds how long a node keeps deciding from a policy row
	// another node changed. Local changes invalidate immediately.
	policyCacheTTL = 10 * time.Second
	// programCacheSize bounds the number of compiled expressions kept.
	programCacheSize = 4096
	// defaultAttributeRefreshInterval is used when the setting is unset.
	defaultAttributeRefreshInterval = 30 * time.Second
	// maxPolicySearchPage is the page size used when enumerating policies.
	maxPolicySearchPage = 1000
)

// Backend gives the service access to the server pieces it needs. Both the app
// and the platform layers can provide it.
type Backend struct {
	Store  func() store.Store
	Config func() *model.Config
	Logger func() mlog.LoggerIFace
}

// Service implements einterfaces.AccessControlServiceInterface.
type Service struct {
	backend Backend
	now     func() time.Time

	envMu sync.RWMutex
	env   *cel.Env

	catalogs *catalogCache

	programsMu sync.Mutex
	programs   map[string]cel.Program

	policiesMu sync.Mutex
	policies   map[string]cachedPolicy

	permissionMu       sync.Mutex
	permissionPolicies []*model.AccessControlPolicy
	permissionLoadedAt time.Time

	refreshMu   sync.Mutex
	lastRefresh time.Time
}

type cachedPolicy struct {
	policy   *model.AccessControlPolicy // nil when the policy does not exist
	loadedAt time.Time
}

var _ einterfaces.AccessControlServiceInterface = (*Service)(nil)

// New creates a service over the given backend.
func New(b Backend) *Service {
	s := &Service{
		backend:  b,
		now:      time.Now,
		programs: map[string]cel.Program{},
		policies: map[string]cachedPolicy{},
	}
	s.catalogs = newCatalogCache(s.store)
	return s
}

func (s *Service) store() store.Store {
	if s.backend.Store == nil {
		return nil
	}
	return s.backend.Store()
}

func (s *Service) config() *model.Config {
	if s.backend.Config == nil {
		cfg := &model.Config{}
		cfg.SetDefaults()
		return cfg
	}
	return s.backend.Config()
}

func (s *Service) logger(rctx request.CTX) mlog.LoggerIFace {
	if rctx != nil {
		return rctx.Logger()
	}
	if s.backend.Logger != nil {
		return s.backend.Logger()
	}
	return mlog.CreateConsoleTestLogger(nil)
}

// abacEnabled reports whether attribute based access control is switched on.
func (s *Service) abacEnabled() bool {
	cfg := s.config()
	return cfg.AccessControlSettings.EnableAttributeBasedAccessControl != nil &&
		*cfg.AccessControlSettings.EnableAttributeBasedAccessControl
}

// Init implements PolicyAdministrationPointInterface. It builds the CEL
// environment; it is idempotent.
func (s *Service) Init(rctx request.CTX) *model.AppError {
	if _, err := s.celEnv(); err != nil {
		return model.NewAppError("Init", "app.pap.init.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return nil
}

func (s *Service) celEnv() (*cel.Env, error) {
	s.envMu.RLock()
	env := s.env
	s.envMu.RUnlock()
	if env != nil {
		return env, nil
	}

	s.envMu.Lock()
	defer s.envMu.Unlock()
	if s.env != nil {
		return s.env, nil
	}
	env, err := newCELEnv(func() time.Time { return s.now() })
	if err != nil {
		return nil, err
	}
	s.env = env
	return env, nil
}

// program compiles (and caches) an expression written with attribute names.
func (s *Service) program(expression string) (cel.Program, error) {
	s.programsMu.Lock()
	if prg, ok := s.programs[expression]; ok {
		s.programsMu.Unlock()
		return prg, nil
	}
	s.programsMu.Unlock()

	env, err := s.celEnv()
	if err != nil {
		return nil, err
	}
	checked, iss := env.Compile(expression)
	if iss != nil && iss.Err() != nil {
		return nil, iss.Err()
	}
	if !checked.OutputType().IsAssignableType(cel.BoolType) {
		return nil, errors.New("expression does not evaluate to a boolean")
	}
	prg, err := env.Program(checked)
	if err != nil {
		return nil, err
	}

	s.programsMu.Lock()
	if len(s.programs) >= programCacheSize {
		s.programs = map[string]cel.Program{}
	}
	s.programs[expression] = prg
	s.programsMu.Unlock()
	return prg, nil
}

// evalBool evaluates a compiled expression. Any evaluation error (missing
// attribute, type mismatch) is returned; callers treat it as a deny.
func evalBool(prg cel.Program, activation map[string]any) (bool, error) {
	out, _, err := prg.Eval(activation)
	if err != nil {
		return false, err
	}
	b, ok := out.(types.Bool)
	if !ok {
		return false, errors.New("expression did not evaluate to a boolean")
	}
	return bool(b), nil
}

// catalog returns the property field catalog.
func (s *Service) catalog(rctx request.CTX) (*catalog, *model.AppError) {
	cat, err := s.catalogs.get(rctx)
	if err != nil {
		return nil, model.NewAppError("AccessControl", "app.access_control.build_subject.group_id.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return cat, nil
}

// refreshAttributesIfStale refreshes the attribute materialized views at most
// once per AttributeRefreshIntervalSeconds, without blocking concurrent callers.
func (s *Service) refreshAttributesIfStale(rctx request.CTX) {
	interval := defaultAttributeRefreshInterval
	if v := s.config().AccessControlSettings.AttributeRefreshIntervalSeconds; v != nil && *v > 0 {
		interval = time.Duration(*v) * time.Second
	}
	if !s.refreshMu.TryLock() {
		return
	}
	defer s.refreshMu.Unlock()
	if time.Since(s.lastRefresh) < interval {
		return
	}
	s.forceRefreshLocked(rctx)
}

// refreshAttributes refreshes the attribute materialized views now.
func (s *Service) refreshAttributes(rctx request.CTX) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	s.forceRefreshLocked(rctx)
}

func (s *Service) forceRefreshLocked(rctx request.CTX) {
	st := s.store()
	if st == nil {
		return
	}
	if err := st.Attributes().RefreshAttributes(); err != nil {
		s.logger(rctx).Warn("Failed to refresh attribute views", mlog.Err(err))
		return
	}
	s.lastRefresh = time.Now()
}

// ---------------------------------------------------------------------------
// Policy loading and caching.
// ---------------------------------------------------------------------------

// loadPolicy returns the stored policy (expressions as stored, i.e. with field
// IDs) or nil when it does not exist.
func (s *Service) loadPolicy(rctx request.CTX, id string) (*model.AccessControlPolicy, error) {
	s.policiesMu.Lock()
	if c, ok := s.policies[id]; ok && time.Since(c.loadedAt) < policyCacheTTL {
		s.policiesMu.Unlock()
		return c.policy, nil
	}
	s.policiesMu.Unlock()

	st := s.store()
	if st == nil {
		return nil, errors.New("store is not available")
	}
	policy, err := st.AccessControlPolicy().Get(rctx, id)
	if err != nil {
		var nfErr *store.ErrNotFound
		if !errors.As(err, &nfErr) {
			return nil, err
		}
		policy = nil
	}

	s.policiesMu.Lock()
	if len(s.policies) > 50000 {
		s.policies = map[string]cachedPolicy{}
	}
	s.policies[id] = cachedPolicy{policy: policy, loadedAt: time.Now()}
	s.policiesMu.Unlock()
	return policy, nil
}

// loadPermissionPolicies returns every system permission policy.
func (s *Service) loadPermissionPolicies(rctx request.CTX) ([]*model.AccessControlPolicy, error) {
	s.permissionMu.Lock()
	defer s.permissionMu.Unlock()
	if s.permissionPolicies != nil && time.Since(s.permissionLoadedAt) < policyCacheTTL {
		return s.permissionPolicies, nil
	}
	policies, err := s.searchAllPolicies(rctx, model.AccessControlPolicySearch{Type: model.AccessControlPolicyTypePermission})
	if err != nil {
		return nil, err
	}
	if policies == nil {
		policies = []*model.AccessControlPolicy{}
	}
	s.permissionPolicies = policies
	s.permissionLoadedAt = time.Now()
	return policies, nil
}

// searchAllPolicies pages through SearchPolicies.
func (s *Service) searchAllPolicies(rctx request.CTX, opts model.AccessControlPolicySearch) ([]*model.AccessControlPolicy, error) {
	st := s.store()
	if st == nil {
		return nil, errors.New("store is not available")
	}
	var out []*model.AccessControlPolicy
	opts.Limit = maxPolicySearchPage
	opts.Cursor = model.AccessControlPolicyCursor{}
	for {
		page, _, err := st.AccessControlPolicy().SearchPolicies(rctx, opts)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if len(page) < maxPolicySearchPage {
			return out, nil
		}
		opts.Cursor.ID = page[len(page)-1].ID
	}
}

// invalidatePolicy drops cached state for a policy.
func (s *Service) invalidatePolicy(id string) {
	s.policiesMu.Lock()
	delete(s.policies, id)
	s.policiesMu.Unlock()

	s.permissionMu.Lock()
	s.permissionPolicies = nil
	s.permissionMu.Unlock()
}

// invalidateAllPolicies drops every cached policy and compiled program.
func (s *Service) invalidateAllPolicies() {
	s.policiesMu.Lock()
	s.policies = map[string]cachedPolicy{}
	s.policiesMu.Unlock()

	s.permissionMu.Lock()
	s.permissionPolicies = nil
	s.permissionMu.Unlock()

	s.programsMu.Lock()
	s.programs = map[string]cel.Program{}
	s.programsMu.Unlock()
}

// OnPropertyFieldOptionsChanged implements PolicyAdministrationPointInterface.
func (s *Service) OnPropertyFieldOptionsChanged(rctx request.CTX, fieldID string) {
	s.catalogs.invalidateField(fieldID)
}

// InvalidateAllPolicyCaches implements PolicyAdministrationPointInterface.
// Caches are node local; other nodes converge within their cache TTLs.
func (s *Service) InvalidateAllPolicyCaches(rctx request.CTX) {
	s.catalogs.invalidateAll()
	s.invalidateAllPolicies()
}
