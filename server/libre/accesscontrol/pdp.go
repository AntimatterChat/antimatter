// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package accesscontrol

import (
	"net/http"
	"slices"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Blame sources used internally when combining lanes; they mirror the
// simulation blame sources so the simulator can reuse the evaluator.
const (
	laneDraft      = "draft"
	laneParent     = "parent"
	lanePermission = "permission"
)

// ruleExpr is one expression contributing to a decision.
type ruleExpr struct {
	policyID   string
	policyName string
	policyType string
	ruleName   string
	role       string
	expression string // as stored (may use id_<fieldID> selectors)
	lane       string
}

// evalContext evaluates expressions for one subject and resource.
type evalContext struct {
	s        *Service
	rctx     request.CTX
	cat      *catalog
	subject  *model.Subject
	user     map[string]any
	resource model.Resource

	resourceAttrs  map[string]any
	resourceLoaded bool
}

func (s *Service) newEvalContext(rctx request.CTX, cat *catalog, subject *model.Subject, resource model.Resource) *evalContext {
	return &evalContext{
		s:        s,
		rctx:     rctx,
		cat:      cat,
		subject:  subject,
		user:     s.userActivation(subject, cat),
		resource: resource,
	}
}

// convertAttributes adapts attribute values (as materialized by the attribute
// views, keyed by field name) into CEL values: ranked options become
// comparable rank values and graph values become hierarchy-aware sets.
func (s *Service) convertAttributes(raw map[string]any, cat *catalog, objectType string) map[string]any {
	out := make(map[string]any, len(raw))
	for name, val := range raw {
		f := cat.lookup(objectType, name)
		if f == nil {
			out[name] = normalizeJSONValue(val)
			continue
		}
		switch f.Type {
		case model.PropertyFieldTypeRank:
			out[name] = toRankValue(val, f)
		case model.PropertyFieldTypeGraph:
			out[name] = toGraphValue(val, f, s.catalogs)
		default:
			out[name] = normalizeJSONValue(val)
		}
	}
	return out
}

// normalizeJSONValue turns JSON numbers without a fractional part into int64
// so they compare naturally with CEL integer literals.
func normalizeJSONValue(val any) any {
	switch v := val.(type) {
	case float64:
		if i, ok := toInt64(v); ok {
			return i
		}
	case []any:
		out := make([]any, len(v))
		for i, e := range v {
			out[i] = normalizeJSONValue(e)
		}
		return out
	}
	return val
}

func toRankValue(val any, f *fieldInfo) any {
	rv := &rankValue{ranks: f.ranks}
	switch v := val.(type) {
	case map[string]any:
		rv.name, _ = v["name"].(string)
		if r, ok := toInt64(v["rank"]); ok {
			rv.rank = r
			rv.hasRank = true
		}
	case string:
		rv.name = v
	default:
		return val
	}
	if !rv.hasRank {
		if r, ok := f.ranks[rv.name]; ok {
			rv.rank = r
			rv.hasRank = true
		}
	}
	return rv
}

func toGraphValue(val any, f *fieldInfo, graph graphResolver) any {
	gv := &graphValue{field: f, graph: graph, ids: []string{}}
	switch v := val.(type) {
	case []any:
		for _, e := range v {
			if id, ok := e.(string); ok {
				gv.ids = append(gv.ids, id)
			}
		}
	case []string:
		gv.ids = append(gv.ids, v...)
	case string:
		gv.ids = append(gv.ids, v)
	case nil:
	default:
		return val
	}
	return gv
}

// userActivation builds the `user` CEL variable.
func (s *Service) userActivation(subject *model.Subject, cat *catalog) map[string]any {
	if subject == nil {
		subject = &model.Subject{}
	}
	attrs := s.convertAttributes(subject.Attributes, cat, model.PropertyFieldObjectTypeUser)
	session := subject.Session
	if session == nil {
		session = map[string]any{}
	}
	return map[string]any{
		nativeID:       subject.ID,
		nativeEmail:    subject.Email,
		nativeVerified: subject.EmailVerified,
		nativeIsBot:    subject.IsBot,
		nativeCreateAt: subject.CreateAt,
		"attributes":   attrs,
		"session":      session,
	}
}

// resourceAttributes loads the accessed channel's attributes, keyed by name.
func (s *Service) resourceAttributes(rctx request.CTX, cat *catalog, resourceID string) (map[string]any, error) {
	st := s.store()
	if st == nil || resourceID == "" || cat == nil {
		return map[string]any{}, nil
	}
	s.refreshAttributesIfStale(rctx)
	subj, err := st.Attributes().GetSubject(rctx, resourceID, cat.groupID, model.PropertyFieldObjectTypeChannel)
	if err != nil {
		if isNotFound(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if subj.Attributes == nil {
		return map[string]any{}, nil
	}
	return subj.Attributes, nil
}

func (ec *evalContext) resourceActivation() (map[string]any, error) {
	if !ec.resourceLoaded {
		raw, err := ec.s.resourceAttributes(ec.rctx, ec.cat, ec.resource.ID)
		if err != nil {
			return nil, err
		}
		ec.resourceAttrs = ec.s.convertAttributes(raw, ec.cat, model.PropertyFieldObjectTypeChannel)
		ec.resourceLoaded = true
	}
	return map[string]any{
		"id":         ec.resource.ID,
		"type":       ec.resource.Type,
		"attributes": ec.resourceAttrs,
	}, nil
}

// eval evaluates a stored or authored expression. An empty expression is an
// error: callers skip empty expressions before evaluating.
func (ec *evalContext) eval(expression string) (bool, error) {
	named := idsToNames(expression, ec.cat)
	prg, err := ec.s.program(named)
	if err != nil {
		return false, err
	}
	activation := map[string]any{celVarUser: ec.user}
	if strings.Contains(named, celVarResource) {
		res, err := ec.resourceActivation()
		if err != nil {
			return false, err
		}
		activation[celVarResource] = res
	} else {
		activation[celVarResource] = map[string]any{"id": ec.resource.ID, "type": ec.resource.Type, "attributes": map[string]any{}}
	}
	return evalBool(prg, activation)
}

// evalOr evaluates rules OR-combined. Evaluation errors count as false.
func (ec *evalContext) evalOr(rules []ruleExpr) (bool, []ruleExpr) {
	var failed []ruleExpr
	for _, r := range rules {
		ok, err := ec.eval(r.expression)
		if err != nil {
			ec.s.logger(ec.rctx).Debug("Access control expression evaluation failed; treating as deny",
				mlog.String("policy_id", r.policyID), mlog.String("rule", r.ruleName), mlog.Err(err))
		}
		if ok {
			return true, nil
		}
		failed = append(failed, r)
	}
	return false, failed
}

// evalAnd evaluates rules AND-combined. Evaluation errors count as false.
func (ec *evalContext) evalAnd(rules []ruleExpr) (bool, []ruleExpr) {
	for _, r := range rules {
		ok, err := ec.eval(r.expression)
		if err != nil {
			ec.s.logger(ec.rctx).Debug("Access control expression evaluation failed; treating as deny",
				mlog.String("policy_id", r.policyID), mlog.String("rule", r.ruleName), mlog.Err(err))
		}
		if !ok {
			return false, []ruleExpr{r}
		}
	}
	return true, nil
}

// isLegacyImportReference recognizes v0.1 child rules that merely reference
// their parent (`policies.id_<parentID>`); the parent is evaluated directly.
func isLegacyImportReference(expression string) bool {
	return strings.HasPrefix(strings.TrimSpace(expression), "policies.")
}

func ruleHasAction(rule model.AccessControlPolicyRule, action string) bool {
	if slices.Contains(rule.Actions, action) {
		return true
	}
	return action == model.AccessControlPolicyActionMembership && slices.Contains(rule.Actions, "*")
}

func usableExpression(expression string) bool {
	e := strings.TrimSpace(expression)
	return e != "" && !isLegacyImportReference(e)
}

// membershipRules returns the membership expressions of a policy and of the
// parent policies it imports, all of which must hold.
func (s *Service) membershipRules(rctx request.CTX, policy *model.AccessControlPolicy) ([]ruleExpr, error) {
	var out []ruleExpr
	for _, rule := range policy.Rules {
		if !rule.IsMembershipRule() || !usableExpression(rule.Expression) {
			continue
		}
		out = append(out, ruleExpr{policyID: policy.ID, policyName: policy.Name, policyType: policy.Type, ruleName: rule.Name, expression: rule.Expression, lane: laneDraft})
	}
	for _, importID := range policy.Imports {
		parent, err := s.loadPolicy(rctx, importID)
		if err != nil {
			return nil, err
		}
		if parent == nil {
			// Dangling import: the parent was deleted.
			continue
		}
		for _, rule := range parent.Rules {
			if !rule.IsMembershipRule() || !usableExpression(rule.Expression) {
				continue
			}
			out = append(out, ruleExpr{policyID: parent.ID, policyName: parent.Name, policyType: parent.Type, ruleName: rule.Name, expression: rule.Expression, lane: laneParent})
		}
	}
	return out, nil
}

// actionRules returns the rules of a policy governing a non-membership action
// for a subject holding role in the rule's scope. Rules without a role apply
// to every role. A channel admin falls back to channel member rules when no
// admin-specific rule exists for the action.
func actionRules(policy *model.AccessControlPolicy, action, role string) []ruleExpr {
	collect := func(wantRole string) (specific, generic []ruleExpr) {
		for _, rule := range policy.Rules {
			if !ruleHasAction(rule, action) || !usableExpression(rule.Expression) {
				continue
			}
			re := ruleExpr{policyID: policy.ID, policyName: policy.Name, policyType: policy.Type, ruleName: rule.Name, role: rule.Role, expression: rule.Expression, lane: laneDraft}
			switch rule.Role {
			case "":
				generic = append(generic, re)
			case wantRole:
				specific = append(specific, re)
			}
		}
		return specific, generic
	}

	specific, generic := collect(role)
	if len(specific) == 0 && role == model.ChannelAdminRoleId {
		specific, _ = collect(model.ChannelUserRoleId)
	}
	if role == "" {
		specific = nil
	}
	return append(specific, generic...)
}

// subjectSystemRole resolves the subject's system role.
func subjectSystemRole(subject *model.Subject) string {
	role := subject.RoleForScope(model.AccessControlSubjectScopeSystem)
	tokens := strings.Fields(role)
	switch {
	case slices.Contains(tokens, model.SystemAdminRoleId):
		return model.SystemAdminRoleId
	case slices.Contains(tokens, model.SystemGuestRoleId):
		return model.SystemGuestRoleId
	case slices.Contains(tokens, model.SystemUserRoleId):
		return model.SystemUserRoleId
	case len(tokens) == 1:
		return tokens[0]
	}
	return model.SystemUserRoleId
}

// permissionPolicyGroups returns, for a system role and action, the rules of
// each applicable permission policy. Policies are AND-ed; rules within a
// policy are OR-ed. A system admin falls back to member policies when no
// admin-specific policy covers the action.
func permissionPolicyGroups(policies []*model.AccessControlPolicy, role, action string) [][]ruleExpr {
	collect := func(wantRole string) [][]ruleExpr {
		var groups [][]ruleExpr
		for _, p := range policies {
			if p == nil || !slices.Contains(p.Roles, wantRole) {
				continue
			}
			var rules []ruleExpr
			for _, rule := range p.Rules {
				if !ruleHasAction(rule, action) || !usableExpression(rule.Expression) {
					continue
				}
				rules = append(rules, ruleExpr{policyID: p.ID, policyName: p.Name, policyType: p.Type, ruleName: rule.Name, role: wantRole, expression: rule.Expression, lane: lanePermission})
			}
			if len(rules) > 0 {
				groups = append(groups, rules)
			}
		}
		return groups
	}

	groups := collect(role)
	if len(groups) == 0 && role == model.SystemAdminRoleId {
		groups = collect(model.SystemUserRoleId)
	}
	return groups
}

func allowDecision() model.AccessDecision {
	return model.AccessDecision{Decision: true}
}

func denyDecision(policyID string) model.AccessDecision {
	d := model.AccessDecision{Decision: false}
	if policyID != "" {
		d.Context = map[string]any{"policy_id": policyID}
	}
	return d
}

// AccessEvaluation implements PolicyDecisionPointInterface.
func (s *Service) AccessEvaluation(rctx request.CTX, req model.AccessRequest) (model.AccessDecision, *model.AppError) {
	if !s.abacEnabled() {
		return model.NewNoPolicyAccessDecision(), nil
	}
	if req.Resource.ID == "" || req.Resource.Type == "" || req.Action == "" {
		return model.AccessDecision{}, model.NewAppError("AccessEvaluation", "app.pap.get_policy.app_error", nil, "resource and action are required", http.StatusBadRequest)
	}

	cat, appErr := s.catalog(rctx)
	if appErr != nil {
		return model.AccessDecision{}, appErr
	}
	flags := s.config().FeatureFlags
	ec := s.newEvalContext(rctx, cat, &req.Subject, req.Resource)

	switch {
	case req.Resource.Type == model.AccessControlPolicyTypeChannel:
		policy, err := s.loadPolicy(rctx, req.Resource.ID)
		if err != nil {
			return model.AccessDecision{}, model.NewAppError("AccessEvaluation", "app.pap.get_policy.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		if policy != nil && policy.Type != model.AccessControlPolicyTypeChannel {
			policy = nil
		}

		if req.Action == model.AccessControlPolicyActionMembership {
			return s.evaluateMembership(ec, policy)
		}

		if model.IsPermissionAction(req.Action) {
			if flags == nil || !flags.PermissionPolicies {
				return model.NewNoPolicyAccessDecision(), nil
			}
			return s.evaluatePermission(ec, policy, req.Action, flags.IsChannelPermissionPoliciesEnabled())
		}

		return evaluateActionRules(ec, policy, req.Action)

	case req.Resource.Type == model.AccessControlPolicyTypeTeam:
		if flags == nil || !flags.TeamMembershipAccessControl || req.Action != model.AccessControlPolicyActionMembership {
			return model.NewNoPolicyAccessDecision(), nil
		}
		policy, err := s.loadPolicy(rctx, req.Resource.ID)
		if err != nil {
			return model.AccessDecision{}, model.NewAppError("AccessEvaluation", "app.pap.get_policy.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		if policy != nil && policy.Type != model.AccessControlPolicyTypeTeam {
			policy = nil
		}
		return s.evaluateMembership(ec, policy)

	case model.IsPluginAccessControlPolicyType(req.Resource.Type):
		policy, err := s.loadPolicy(rctx, req.Resource.ID)
		if err != nil {
			return model.AccessDecision{}, model.NewAppError("AccessEvaluation", "app.pap.get_policy.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		if policy == nil || policy.Type != req.Resource.Type {
			return model.NewNoPolicyAccessDecision(), nil
		}
		return evaluateActionRules(ec, policy, req.Action)
	}

	return model.NewNoPolicyAccessDecision(), nil
}

func (s *Service) evaluateMembership(ec *evalContext, policy *model.AccessControlPolicy) (model.AccessDecision, *model.AppError) {
	if policy == nil {
		return model.NewNoPolicyAccessDecision(), nil
	}
	rules, err := s.membershipRules(ec.rctx, policy)
	if err != nil {
		return model.AccessDecision{}, model.NewAppError("AccessEvaluation", "app.pap.get_policy.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if len(rules) == 0 {
		return model.NewNoPolicyAccessDecision(), nil
	}
	if ok, _ := ec.evalAnd(rules); ok {
		return allowDecision(), nil
	}
	return denyDecision(policy.ID), nil
}

// evaluateActionRules evaluates the rules of a policy that declare the action,
// OR-combined, regardless of role.
func evaluateActionRules(ec *evalContext, policy *model.AccessControlPolicy, action string) (model.AccessDecision, *model.AppError) {
	if policy == nil {
		return model.NewNoPolicyAccessDecision(), nil
	}
	var rules []ruleExpr
	for _, rule := range policy.Rules {
		if !ruleHasAction(rule, action) || !usableExpression(rule.Expression) {
			continue
		}
		rules = append(rules, ruleExpr{policyID: policy.ID, policyName: policy.Name, ruleName: rule.Name, expression: rule.Expression})
	}
	if len(rules) == 0 {
		return model.NewNoPolicyAccessDecision(), nil
	}
	if ok, _ := ec.evalOr(rules); ok {
		return allowDecision(), nil
	}
	return denyDecision(policy.ID), nil
}

// evaluatePermission combines the channel lane (the channel policy's
// role-scoped rules) with the system lane (permission policies for the
// subject's system role). Every lane that governs the action must allow it.
func (s *Service) evaluatePermission(ec *evalContext, policy *model.AccessControlPolicy, action string, channelLane bool) (model.AccessDecision, *model.AppError) {
	governed := false

	if channelLane && policy != nil {
		role := ec.subject.RoleForScope(model.AccessControlSubjectScopeChannel)
		if rules := actionRules(policy, action, role); len(rules) > 0 {
			governed = true
			if ok, _ := ec.evalOr(rules); !ok {
				return denyDecision(policy.ID), nil
			}
		}
	}

	permissionPolicies, err := s.loadPermissionPolicies(ec.rctx)
	if err != nil {
		return model.AccessDecision{}, model.NewAppError("AccessEvaluation", "app.pap.get_policy.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	for _, group := range permissionPolicyGroups(permissionPolicies, subjectSystemRole(ec.subject), action) {
		governed = true
		if ok, _ := ec.evalOr(group); !ok {
			return denyDecision(group[0].policyID), nil
		}
	}

	if !governed {
		return model.NewNoPolicyAccessDecision(), nil
	}
	return allowDecision(), nil
}
