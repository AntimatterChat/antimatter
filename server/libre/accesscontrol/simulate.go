// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package accesscontrol

import (
	"fmt"
	"math"
	"net/http"
	"slices"
	"sort"
	"strings"

	celast "github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/parser"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// maxSimulationUsers bounds the users one simulation request may evaluate.
const maxSimulationUsers = 100

// SimulatePolicyForUsers implements PolicyAdministrationPointInterface.
func (s *Service) SimulatePolicyForUsers(rctx request.CTX, params model.PolicySimulationByUsersParams) (*model.PolicySimulationResponse, *model.AppError) {
	const where = "SimulatePolicyForUsers"
	st := s.store()
	if st == nil {
		return nil, model.NewAppError(where, "app.pap.simulate.unavailable", nil, "", http.StatusNotImplemented)
	}
	if flags := s.config().FeatureFlags; flags == nil || !flags.IsPolicySimulationEnabled() {
		return nil, model.NewAppError(where, "app.pap.simulate.feature_disabled", nil, "", http.StatusForbidden)
	}
	if params.Policy == nil {
		return nil, model.NewAppError(where, "app.pap.simulate.missing_policy", nil, "", http.StatusBadRequest)
	}
	if len(params.Actions) == 0 {
		return nil, model.NewAppError(where, "app.pap.simulate.missing_actions", nil, "", http.StatusBadRequest)
	}
	if len(params.Users) == 0 {
		return nil, model.NewAppError(where, "app.pap.simulate.missing_users", nil, "", http.StatusBadRequest)
	}
	if len(params.Users) > maxSimulationUsers {
		return nil, model.NewAppError(where, "app.pap.simulate.too_many_users", map[string]any{"Max": maxSimulationUsers}, "", http.StatusBadRequest)
	}

	cat, appErr := s.catalog(rctx)
	if appErr != nil {
		return nil, appErr
	}
	draft := normalize(params.Policy, cat)

	resourceType := draft.Type
	resourceID := params.ChannelID
	switch {
	case draft.Type == model.AccessControlPolicyTypeChannel:
		if resourceID == "" {
			resourceID = draft.ID
		}
		if resourceID == "" {
			return nil, model.NewAppError(where, "app.pap.simulate.missing_channel_id", nil, "", http.StatusBadRequest)
		}
	case draft.Type == model.AccessControlPolicyTypeTeam:
		if resourceID == "" {
			resourceID = params.TeamID
		}
		if resourceID == "" {
			resourceID = draft.ID
		}
	case draft.Type == model.AccessControlPolicyTypeParent, draft.Type == model.AccessControlPolicyTypePermission:
		resourceType = model.AccessControlPolicyTypeChannel
	case model.IsPluginAccessControlPolicyType(draft.Type):
		if resourceID == "" {
			resourceID = draft.ID
		}
	default:
		return nil, model.NewAppError(where, "app.pap.simulate.unsupported_type", map[string]any{"Type": draft.Type}, "", http.StatusBadRequest)
	}

	usesResource := false
	for _, r := range draft.Rules {
		if !usableExpression(r.Expression) {
			continue
		}
		if _, err := s.program(r.Expression); err != nil {
			return nil, model.NewAppError(where, "app.pap.simulate.compile_failed", nil, "", http.StatusBadRequest).Wrap(err)
		}
		if strings.Contains(r.Expression, "resource.attributes.") {
			usesResource = true
		}
	}
	if usesResource && params.ChannelID == "" && draft.Type != model.AccessControlPolicyTypeChannel {
		return nil, model.NewAppError(where, "app.pap.simulate.channel_required_for_resource", nil, "", http.StatusBadRequest)
	}

	s.refreshAttributes(rctx)

	scope := params.EvaluationScope
	if scope == "" {
		scope = model.PolicyEvaluationScopeThisRule
	}

	resp := &model.PolicySimulationResponse{Results: []model.PolicySimulationUserResult{}}
	for _, u := range params.Users {
		user, err := st.User().Get(rctx, u.UserID)
		if err != nil {
			continue
		}
		subject, appErr := s.buildSimulationSubject(rctx, cat, user, params.ChannelID, u.SessionOverrides)
		if appErr != nil {
			return nil, appErr
		}
		ec := s.newEvalContext(rctx, cat, subject, model.Resource{ID: resourceID, Type: resourceType})

		result := model.PolicySimulationUserResult{
			User:       user,
			Decisions:  map[string]model.PolicySimulationActionDecision{},
			Attributes: displayAttributes(ec, subject),
		}
		for _, action := range params.Actions {
			result.Decisions[action] = s.simulateAction(ec, draft, action, scope, params.RuleName)
		}
		resp.Results = append(resp.Results, result)
	}
	resp.Total = int64(len(resp.Results))
	return resp, nil
}

func (s *Service) buildSimulationSubject(rctx request.CTX, cat *catalog, user *model.User, channelID string, session map[string]any) (*model.Subject, *model.AppError) {
	subject := &model.Subject{
		ID:            user.Id,
		Type:          "user",
		Attributes:    map[string]any{},
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
		IsBot:         user.IsBot,
		CreateAt:      user.CreateAt,
		Role:          user.Roles,
		Session:       session,
	}
	st := s.store()
	stored, err := st.Attributes().GetSubject(rctx, user.Id, cat.groupID, model.PropertyFieldObjectTypeUser)
	if err != nil && !isNotFound(err) {
		return nil, model.NewAppError("SimulatePolicyForUsers", "app.pap.simulate.attribute_refresh", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if err == nil && stored.Attributes != nil {
		subject.Attributes = stored.Attributes
	}
	subject.SetScopedRole(model.AccessControlSubjectScopeSystem, subjectSystemRole(subject))

	if channelID != "" {
		member, err := st.Channel().GetMember(rctx, channelID, user.Id)
		if err == nil {
			role := ""
			tokens := strings.Fields(member.Roles)
			switch {
			case member.SchemeAdmin || slices.Contains(tokens, model.ChannelAdminRoleId):
				role = model.ChannelAdminRoleId
			case member.SchemeGuest || slices.Contains(tokens, model.ChannelGuestRoleId):
				role = model.ChannelGuestRoleId
			case member.SchemeUser || slices.Contains(tokens, model.ChannelUserRoleId):
				role = model.ChannelUserRoleId
			}
			subject.SetScopedRole(model.AccessControlSubjectScopeChannel, role)
		} else if user.IsGuest() {
			// Not a member yet: simulate the role they would join with.
			subject.SetScopedRole(model.AccessControlSubjectScopeChannel, model.ChannelGuestRoleId)
		} else {
			subject.SetScopedRole(model.AccessControlSubjectScopeChannel, model.ChannelUserRoleId)
		}
	}
	return subject, nil
}

func vacuous(source string) model.PolicySimulationActionDecision {
	return model.PolicySimulationActionDecision{
		Decision: true,
		Blame:    []model.PolicySimulationBlame{{Source: source, Outcome: model.PolicySimulationBlameOutcomeAllow}},
	}
}

func referencesSession(rules []ruleExpr) bool {
	for _, r := range rules {
		if strings.Contains(r.expression, "user.session.") {
			return true
		}
	}
	return false
}

// simulateAction evaluates one action of the draft for one subject.
func (s *Service) simulateAction(ec *evalContext, draft *model.AccessControlPolicy, action, scope, editingRule string) model.PolicySimulationActionDecision {
	thisRuleOnly := scope != model.PolicyEvaluationScopeAll

	// Collect the draft's contributing rules.
	var draftRules []ruleExpr
	membership := action == model.AccessControlPolicyActionMembership
	switch {
	case membership:
		for _, r := range draft.Rules {
			if r.IsMembershipRule() && usableExpression(r.Expression) {
				draftRules = append(draftRules, ruleExpr{policyID: draft.ID, policyName: draft.Name, ruleName: r.Name, expression: r.Expression, lane: laneDraft})
			}
		}
	case draft.Type == model.AccessControlPolicyTypePermission:
		role := subjectSystemRole(ec.subject)
		applies := slices.Contains(draft.Roles, role) || (role == model.SystemAdminRoleId && slices.Contains(draft.Roles, model.SystemUserRoleId))
		if applies {
			for _, r := range draft.Rules {
				if ruleHasAction(r, action) && usableExpression(r.Expression) {
					draftRules = append(draftRules, ruleExpr{policyID: draft.ID, policyName: draft.Name, ruleName: r.Name, expression: r.Expression, lane: laneDraft})
				}
			}
		}
	case draft.Type == model.AccessControlPolicyTypeChannel:
		draftRules = actionRules(draft, action, ec.subject.RoleForScope(model.AccessControlSubjectScopeChannel))
	default:
		for _, r := range draft.Rules {
			if ruleHasAction(r, action) && usableExpression(r.Expression) {
				draftRules = append(draftRules, ruleExpr{policyID: draft.ID, policyName: draft.Name, ruleName: r.Name, expression: r.Expression, lane: laneDraft})
			}
		}
	}

	if thisRuleOnly && editingRule != "" {
		var only []ruleExpr
		for _, r := range draftRules {
			if r.ruleName == editingRule {
				only = append(only, r)
			}
		}
		if len(only) == 0 {
			return vacuous(model.PolicySimulationBlameSourceNoApplicableRule)
		}
		draftRules = only
	}

	// Rules outside the draft, only in the "all" scope.
	var parentRules []ruleExpr
	var permissionGroups [][]ruleExpr
	if !thisRuleOnly {
		if membership {
			for _, importID := range draft.Imports {
				parent, err := s.loadPolicy(ec.rctx, importID)
				if err != nil || parent == nil {
					continue
				}
				for _, r := range parent.Rules {
					if r.IsMembershipRule() && usableExpression(r.Expression) {
						parentRules = append(parentRules, ruleExpr{policyID: parent.ID, policyName: parent.Name, policyType: parent.Type, expression: r.Expression, lane: laneParent})
					}
				}
			}
		} else if model.IsPermissionAction(action) && (draft.Type == model.AccessControlPolicyTypeChannel || draft.Type == model.AccessControlPolicyTypePermission) {
			policies, _ := s.loadPermissionPolicies(ec.rctx)
			others := make([]*model.AccessControlPolicy, 0, len(policies))
			for _, p := range policies {
				if p.ID != draft.ID {
					others = append(others, p)
				}
			}
			permissionGroups = permissionPolicyGroups(others, subjectSystemRole(ec.subject), action)
		}
	}

	all := slices.Clone(draftRules)
	all = append(all, parentRules...)
	for _, g := range permissionGroups {
		all = append(all, g...)
	}
	if len(all) == 0 {
		return vacuous(model.PolicySimulationBlameSourceNoApplicablePolicy)
	}
	if referencesSession(all) && len(ec.subject.Session) == 0 {
		return vacuous(model.PolicySimulationBlameSourceNoSessionData)
	}

	decision := model.PolicySimulationActionDecision{Decision: true}
	sourceFor := func(r ruleExpr) string {
		if editingRule == "" || r.ruleName == editingRule {
			return model.PolicySimulationBlameSourceThisRule
		}
		return model.PolicySimulationBlameSourceSiblingRule
	}

	// The draft's own rules: AND for membership, OR otherwise.
	if len(draftRules) > 0 {
		if membership {
			if ok, failed := ec.evalAnd(draftRules); !ok {
				decision.Decision = false
				for _, r := range failed {
					decision.Blame = append(decision.Blame, s.draftBlame(ec, r, sourceFor(r), model.PolicySimulationBlameOutcomeDeny))
				}
			}
		} else {
			ok, failed := ec.evalOr(draftRules)
			if !ok {
				decision.Decision = false
				for _, r := range failed {
					decision.Blame = append(decision.Blame, s.draftBlame(ec, r, sourceFor(r), model.PolicySimulationBlameOutcomeDeny))
				}
			} else if editingRule != "" {
				for _, r := range draftRules {
					if r.ruleName != editingRule {
						continue
					}
					if editingOK, _ := ec.eval(r.expression); !editingOK {
						decision.Blame = append(decision.Blame, s.draftBlame(ec, r, model.PolicySimulationBlameSourceSiblingSaved, model.PolicySimulationBlameOutcomeAllow))
					}
				}
			}
		}
	}

	if ok, failed := ec.evalAnd(parentRules); !ok {
		decision.Decision = false
		for _, r := range failed {
			decision.Blame = append(decision.Blame, model.PolicySimulationBlame{
				Source:     model.PolicySimulationBlameSourceChannelPolicy,
				Outcome:    model.PolicySimulationBlameOutcomeDeny,
				PolicyID:   r.policyID,
				PolicyName: r.policyName,
			})
		}
	}

	for _, group := range permissionGroups {
		if ok, _ := ec.evalOr(group); !ok {
			decision.Decision = false
			decision.Blame = append(decision.Blame, model.PolicySimulationBlame{
				Source:     model.PolicySimulationBlameSourceSystemPermission,
				Outcome:    model.PolicySimulationBlameOutcomeDeny,
				PolicyID:   group[0].policyID,
				PolicyName: group[0].policyName,
				Role:       group[0].role,
			})
		}
	}

	return decision
}

func (s *Service) draftBlame(ec *evalContext, r ruleExpr, source, outcome string) model.PolicySimulationBlame {
	named := idsToNames(r.expression, ec.cat)
	return model.PolicySimulationBlame{
		Source:         source,
		Outcome:        outcome,
		PolicyID:       r.policyID,
		PolicyName:     r.policyName,
		RuleName:       r.ruleName,
		Role:           r.role,
		Expression:     named,
		EvaluationTree: s.evaluationTree(ec, named),
	}
}

// evaluationTree explains an expression's verdict node by node.
func (s *Service) evaluationTree(ec *evalContext, expression string) *model.PolicySimulationEvaluationNode {
	parsed, celErrs, err := s.parseExpression(expression)
	if err != nil || len(celErrs) > 0 || parsed == nil {
		return nil
	}
	node := s.treeNode(ec, parsed, parsed.Expr())
	return &node
}

func (s *Service) subExpression(a *celast.AST, e celast.Expr) string {
	out, err := parser.Unparse(e, a.SourceInfo(), parser.WrapOnColumn(math.MaxInt32))
	if err != nil {
		return ""
	}
	return out
}

func outcomeOf(ok bool, err error) (string, string) {
	switch {
	case err != nil:
		return model.PolicySimulationEvaluationOutcomeError, err.Error()
	case ok:
		return model.PolicySimulationEvaluationOutcomeTrue, ""
	}
	return model.PolicySimulationEvaluationOutcomeFalse, ""
}

func (s *Service) treeNode(ec *evalContext, a *celast.AST, e celast.Expr) model.PolicySimulationEvaluationNode {
	text := s.subExpression(a, e)
	node := model.PolicySimulationEvaluationNode{Expression: text}

	if call, ok := callOf(e); ok && !call.IsMemberFunction() {
		switch call.FunctionName() {
		case operators.LogicalAnd, operators.LogicalOr:
			isAnd := call.FunctionName() == operators.LogicalAnd
			node.Kind = model.PolicySimulationEvaluationKindOr
			if isAnd {
				node.Kind = model.PolicySimulationEvaluationKindAnd
			}
			sawError, sawTrue, sawFalse := false, false, false
			for _, operand := range flatten(e, call.FunctionName()) {
				child := s.treeNode(ec, a, operand)
				switch child.Outcome {
				case model.PolicySimulationEvaluationOutcomeTrue:
					sawTrue = true
				case model.PolicySimulationEvaluationOutcomeFalse:
					sawFalse = true
				default:
					sawError = true
				}
				node.Children = append(node.Children, child)
			}
			switch {
			case isAnd && sawFalse, !isAnd && sawTrue:
				node.Outcome = model.PolicySimulationEvaluationOutcomeFalse
				if !isAnd {
					node.Outcome = model.PolicySimulationEvaluationOutcomeTrue
				}
			case sawError:
				node.Outcome = model.PolicySimulationEvaluationOutcomeError
			case isAnd:
				node.Outcome = model.PolicySimulationEvaluationOutcomeTrue
			default:
				node.Outcome = model.PolicySimulationEvaluationOutcomeFalse
			}
			return node
		case operators.LogicalNot:
			node.Kind = model.PolicySimulationEvaluationKindNot
			child := s.treeNode(ec, a, call.Args()[0])
			node.Children = []model.PolicySimulationEvaluationNode{child}
			switch child.Outcome {
			case model.PolicySimulationEvaluationOutcomeTrue:
				node.Outcome = model.PolicySimulationEvaluationOutcomeFalse
			case model.PolicySimulationEvaluationOutcomeFalse:
				node.Outcome = model.PolicySimulationEvaluationOutcomeTrue
			default:
				node.Outcome = model.PolicySimulationEvaluationOutcomeError
			}
			return node
		}
	}

	ok, err := ec.eval(text)
	node.Outcome, node.Error = outcomeOf(ok, err)

	cond, cErr := extractCondition(e)
	if cErr != nil {
		if r, v, isMember := memberTest(e); isMember {
			cond = &condition{attr: r, operator: opIn, values: []any{v}}
			cErr = nil
		}
	}
	if cErr != nil {
		node.Kind = model.PolicySimulationEvaluationKindOther
		return node
	}

	node.Kind = model.PolicySimulationEvaluationKindCompare
	if cond.form == formCall {
		node.Kind = model.PolicySimulationEvaluationKindFunction
	}
	node.Operator = cond.operator
	if cond.memberChain {
		node.Operator = opIn
	}
	node.Attribute = cond.attr.path()
	node.ActualValue = displayRefValue(ec, cond.attr)
	if cond.target != nil {
		return node
	}
	parts := make([]string, 0, len(cond.values))
	for _, v := range cond.values {
		parts = append(parts, fmt.Sprint(v))
	}
	node.ExpectedValue = strings.Join(parts, ", ")
	return node
}

// displayValue renders an attribute value for display.
func displayValue(ec *evalContext, v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case *rankValue:
		return val.name
	case *graphValue:
		names, err := ec.s.catalogs.optionNamesByID(val.field, val.ids)
		out := make([]string, 0, len(val.ids))
		for _, id := range val.ids {
			if err == nil && names[id] != "" {
				out = append(out, names[id])
			} else {
				out = append(out, id)
			}
		}
		return strings.Join(out, ", ")
	case []any:
		out := make([]string, 0, len(val))
		for _, e := range val {
			out = append(out, fmt.Sprint(e))
		}
		return strings.Join(out, ", ")
	case []string:
		return strings.Join(val, ", ")
	}
	return fmt.Sprint(v)
}

func displayRefValue(ec *evalContext, r attrRef) string {
	switch r.scope {
	case scopeNative:
		return displayValue(ec, ec.user[r.name])
	case scopeUser:
		if attrs, ok := ec.user["attributes"].(map[string]any); ok {
			return displayValue(ec, attrs[r.name])
		}
	case scopeSession:
		if sess, ok := ec.user["session"].(map[string]any); ok {
			return displayValue(ec, sess[r.name])
		}
	case scopeResource:
		if res, err := ec.resourceActivation(); err == nil {
			if attrs, ok := res["attributes"].(map[string]any); ok {
				return displayValue(ec, attrs[r.name])
			}
		}
	}
	return ""
}

func displayAttributes(ec *evalContext, subject *model.Subject) map[string]string {
	attrs, _ := ec.user["attributes"].(map[string]any)
	if len(attrs) == 0 {
		return nil
	}
	names := make([]string, 0, len(attrs))
	for n := range attrs {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make(map[string]string, len(attrs))
	for _, n := range names {
		out[n] = displayValue(ec, attrs[n])
	}
	return out
}
