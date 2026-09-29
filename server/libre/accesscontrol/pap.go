// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package accesscontrol

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	celast "github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/operators"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

func isNotFound(err error) bool {
	var nfErr *store.ErrNotFound
	return errors.As(err, &nfErr)
}

func clonePolicy(p *model.AccessControlPolicy) *model.AccessControlPolicy {
	if p == nil {
		return nil
	}
	c := *p
	c.Roles = slices.Clone(p.Roles)
	c.Imports = slices.Clone(p.Imports)
	c.Rules = make([]model.AccessControlPolicyRule, len(p.Rules))
	for i, r := range p.Rules {
		c.Rules[i] = r
		c.Rules[i].Actions = slices.Clone(r.Actions)
		if r.Metadata != nil {
			c.Rules[i].Metadata = make(map[string]any, len(r.Metadata))
			for k, v := range r.Metadata {
				c.Rules[i].Metadata[k] = v
			}
		}
	}
	if p.Props != nil {
		c.Props = make(map[string]any, len(p.Props))
		for k, v := range p.Props {
			c.Props[k] = v
		}
	}
	return &c
}

// normalize returns a copy of the policy whose expressions use attribute names.
func normalize(p *model.AccessControlPolicy, cat *catalog) *model.AccessControlPolicy {
	c := clonePolicy(p)
	if c == nil {
		return nil
	}
	for i := range c.Rules {
		c.Rules[i].Expression = idsToNames(c.Rules[i].Expression, cat)
	}
	return c
}

// NormalizePolicy implements PolicyAdministrationPointInterface.
func (s *Service) NormalizePolicy(rctx request.CTX, policy *model.AccessControlPolicy) (*model.AccessControlPolicy, *model.AppError) {
	if policy == nil {
		return nil, model.NewAppError("NormalizePolicy", "app.pap.normalize_policy.app_error", nil, "policy is nil", http.StatusBadRequest)
	}
	cat, appErr := s.catalog(rctx)
	if appErr != nil {
		return nil, model.NewAppError("NormalizePolicy", "app.pap.normalize_policy.app_error", nil, "", http.StatusInternalServerError).Wrap(appErr)
	}
	return normalize(policy, cat), nil
}

// GetPolicy implements PolicyAdministrationPointInterface.
func (s *Service) GetPolicy(rctx request.CTX, id string) (*model.AccessControlPolicy, *model.AppError) {
	st := s.store()
	if st == nil {
		return nil, model.NewAppError("GetPolicy", "app.pap.is_ready.app_error", nil, "", http.StatusInternalServerError)
	}
	policy, err := st.AccessControlPolicy().Get(rctx, id)
	if err != nil {
		if isNotFound(err) {
			return nil, model.NewAppError("GetPolicy", "app.pap.get_policy.app_error", nil, "", http.StatusNotFound).Wrap(err)
		}
		return nil, model.NewAppError("GetPolicy", "app.pap.get_policy.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	cat, appErr := s.catalog(rctx)
	if appErr != nil {
		return nil, appErr
	}
	return normalize(policy, cat), nil
}

// DeletePolicy implements PolicyAdministrationPointInterface.
func (s *Service) DeletePolicy(rctx request.CTX, id string) *model.AppError {
	st := s.store()
	if st == nil {
		return model.NewAppError("DeletePolicy", "app.pap.is_ready.app_error", nil, "", http.StatusInternalServerError)
	}
	if err := st.AccessControlPolicy().Delete(rctx, id); err != nil {
		if isNotFound(err) {
			return model.NewAppError("DeletePolicy", "app.pap.delete_policy.app_error", nil, "", http.StatusNotFound).Wrap(err)
		}
		return model.NewAppError("DeletePolicy", "app.pap.delete_policy.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	s.invalidatePolicy(id)
	return nil
}

// GetPoliciesForFieldIDs implements PolicyAdministrationPointInterface.
func (s *Service) GetPoliciesForFieldIDs(rctx request.CTX, fieldIDs []string) ([]*model.AccessControlPolicy, *model.AppError) {
	st := s.store()
	if st == nil {
		return nil, model.NewAppError("GetPoliciesForFieldIDs", "app.pap.is_ready.app_error", nil, "", http.StatusInternalServerError)
	}
	cat, appErr := s.catalog(rctx)
	if appErr != nil {
		return nil, appErr
	}
	seen := map[string]bool{}
	out := []*model.AccessControlPolicy{}
	for _, fieldID := range fieldIDs {
		policies, err := st.AccessControlPolicy().GetPoliciesByFieldID(rctx, fieldID)
		if err != nil {
			return nil, model.NewAppError("GetPoliciesForFieldIDs", "app.pap.get_policies_for_field_ids.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		for _, p := range policies {
			if p == nil || seen[p.ID] {
				continue
			}
			seen[p.ID] = true
			out = append(out, normalize(p, cat))
		}
	}
	return out, nil
}

// SavePolicy implements PolicyAdministrationPointInterface. It validates the
// rules' expressions, stores attribute references by field ID and returns the
// saved policy with attribute names.
func (s *Service) SavePolicy(rctx request.CTX, policy *model.AccessControlPolicy) (*model.AccessControlPolicy, *model.AppError) {
	const where = "SavePolicy"
	if policy == nil {
		return nil, model.NewAppError(where, "app.pap.save_policy.app_error", nil, "policy is nil", http.StatusBadRequest)
	}
	st := s.store()
	if st == nil {
		return nil, model.NewAppError(where, "app.pap.is_ready.app_error", nil, "", http.StatusInternalServerError)
	}
	cat, appErr := s.catalog(rctx)
	if appErr != nil {
		return nil, appErr
	}

	p := clonePolicy(policy)
	if p.ID == "" {
		p.ID = model.NewId()
	}
	if p.Version == "" {
		switch {
		case model.IsPluginAccessControlPolicyType(p.Type):
			p.Version = model.AccessControlPolicyVersionV0_5
		default:
			p.Version = model.AccessControlPolicyVersionV0_3
		}
	}
	// Channel policies carrying role-scoped permission rules are v0.4 policies.
	if p.Version == model.AccessControlPolicyVersionV0_3 && p.Type == model.AccessControlPolicyTypeChannel {
		for _, r := range p.Rules {
			if r.Role != "" {
				p.Version = model.AccessControlPolicyVersionV0_4
				break
			}
		}
	}

	flags := s.config().FeatureFlags
	resourceAttributesAllowed := flags != nil && flags.ResourceAttributesInPolicies
	usesResourceAttributes := false

	for i := range p.Rules {
		expr := strings.TrimSpace(p.Rules[i].Expression)
		p.Rules[i].Expression = expr
		if expr == "" || isLegacyImportReference(expr) {
			continue
		}
		named := idsToNames(expr, cat)

		if strings.Contains(named, model.MaskingTokenValue) {
			parsed, _, _ := s.parseExpression(named)
			if parsed != nil && containsMaskingToken(parsed.Expr()) {
				return nil, model.NewAppError(where, "app.pap.save_policy.masked_token_in_expression", nil, "", http.StatusBadRequest)
			}
		}

		celErrs, appErr := s.CheckExpression(rctx, named)
		if appErr != nil {
			return nil, appErr
		}
		// CheckExpression reloads the catalog when it meets an attribute it
		// does not know; use the reloaded one from here on.
		if cat, appErr = s.catalog(rctx); appErr != nil {
			return nil, appErr
		}
		if len(celErrs) > 0 {
			details := make([]string, 0, len(celErrs))
			for _, e := range celErrs {
				details = append(details, fmt.Sprintf("%d:%d: %s", e.Line, e.Column+1, e.Message))
			}
			return nil, model.NewAppError(where, "app.pap.save_policy.app_error", nil, "invalid expression: "+strings.Join(details, "; "), http.StatusBadRequest)
		}

		parsed, _, _ := s.parseExpression(named)
		if parsed != nil && referencesScope(parsed.Expr(), scopeResource) {
			usesResourceAttributes = true
		}

		var unknown []string
		p.Rules[i].Expression = namesToIDs(named, cat, func(scope, name string) {
			unknown = append(unknown, scope+".attributes."+name)
		})
		if len(unknown) > 0 {
			return nil, model.NewAppError(where, "app.pap.missing_attribute.app_error", nil, "unknown attributes: "+strings.Join(unknown, ", "), http.StatusBadRequest)
		}
	}

	if usesResourceAttributes {
		if !resourceAttributesAllowed {
			return nil, model.NewAppError(where, "app.pap.save_policy.resource_attributes_disabled", nil, "", http.StatusBadRequest)
		}
		switch p.Type {
		case model.AccessControlPolicyTypeTeam:
			return nil, model.NewAppError(where, "app.pap.save_policy.team_resource_attributes", nil, "", http.StatusBadRequest)
		case model.AccessControlPolicyTypeParent:
			teams, _, err := st.AccessControlPolicy().SearchPolicies(rctx, model.AccessControlPolicySearch{
				Type:     model.AccessControlPolicyTypeTeam,
				ParentID: p.ID,
				Limit:    1,
			})
			if err != nil {
				return nil, model.NewAppError(where, "app.pap.save_policy.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
			}
			if len(teams) > 0 {
				return nil, model.NewAppError(where, "app.pap.save_policy.parent_resource_attributes_team_assigned", nil, "", http.StatusBadRequest)
			}
		}
	}

	// Imports must name existing parent policies; a team policy may not
	// import a parent that uses channel attributes.
	for _, importID := range p.Imports {
		parent, err := st.AccessControlPolicy().Get(rctx, importID)
		if err != nil {
			if isNotFound(err) {
				return nil, model.NewAppError(where, "app.pap.save_policy.app_error", nil, "imported policy not found: "+importID, http.StatusBadRequest)
			}
			return nil, model.NewAppError(where, "app.pap.save_policy.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		if parent.Type != model.AccessControlPolicyTypeParent {
			return nil, model.NewAppError(where, "app.pap.save_policy.app_error", nil, "imports must reference parent policies", http.StatusBadRequest)
		}
		if p.Type == model.AccessControlPolicyTypeTeam {
			for _, r := range parent.Rules {
				if strings.Contains(r.Expression, "resource.attributes.") {
					return nil, model.NewAppError(where, "app.pap.save_policy.team_resource_attributes", nil, "", http.StatusBadRequest)
				}
			}
		}
	}

	if appErr := p.IsValid(); appErr != nil {
		return nil, appErr
	}

	saved, err := st.AccessControlPolicy().Save(rctx, p)
	if err != nil {
		var appErr *model.AppError
		var conflict *store.ErrConflict
		switch {
		case errors.As(err, &appErr):
			return nil, appErr
		case errors.As(err, &conflict):
			return nil, model.NewAppError(where, "app.pap.save_policy.name_exists.app_error", nil, "", http.StatusBadRequest).Wrap(err)
		}
		return nil, model.NewAppError(where, "app.pap.save_policy.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	s.invalidatePolicy(saved.ID)
	return normalize(saved, cat), nil
}

func containsMaskingToken(e celast.Expr) bool {
	found := false
	walk(e, func(n celast.Expr) bool {
		if found {
			return false
		}
		if v, ok := literalOf(n); ok {
			if str, isStr := v.(string); isStr && str == model.MaskingTokenValue {
				found = true
			}
		}
		return true
	})
	return found
}

// GetPolicyRuleAttributes implements PolicyAdministrationPointInterface. It
// returns, for the rules of the policy (and its parents) governing action, the
// attribute names they test and the literal values they test against.
func (s *Service) GetPolicyRuleAttributes(rctx request.CTX, policyID string, action string) (map[string][]string, *model.AppError) {
	policy, err := s.loadPolicy(rctx, policyID)
	if err != nil {
		return nil, model.NewAppError("GetPolicyRuleAttributes", "app.pap.get_policy_attributes.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if policy == nil {
		return nil, model.NewAppError("GetPolicyRuleAttributes", "app.pap.get_policy.app_error", nil, "", http.StatusNotFound)
	}
	cat, appErr := s.catalog(rctx)
	if appErr != nil {
		return nil, appErr
	}

	var rules []ruleExpr
	if action == "" || action == model.AccessControlPolicyActionMembership {
		rules, err = s.membershipRules(rctx, policy)
		if err != nil {
			return nil, model.NewAppError("GetPolicyRuleAttributes", "app.pap.get_policy_attributes.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
	} else {
		for _, r := range policy.Rules {
			if ruleHasAction(r, action) && usableExpression(r.Expression) {
				rules = append(rules, ruleExpr{expression: r.Expression})
			}
		}
	}

	out := map[string][]string{}
	add := func(name string, v any) {
		str := fmt.Sprint(v)
		if s, ok := v.(string); ok {
			str = s
		}
		if str == model.MaskingTokenValue {
			return
		}
		if !slices.Contains(out[name], str) {
			out[name] = append(out[name], str)
		}
	}

	for _, r := range rules {
		parsed, celErrs, perr := s.parseExpression(idsToNames(r.expression, cat))
		if perr != nil || len(celErrs) > 0 || parsed == nil {
			continue
		}
		collectAttributeValues(parsed.Expr(), func(ref attrRef, values []any) {
			if ref.scope != scopeUser && ref.scope != scopeNative {
				return
			}
			if _, ok := out[ref.name]; !ok {
				out[ref.name] = []string{}
			}
			for _, v := range values {
				add(ref.name, v)
			}
		})
	}
	return out, nil
}

// collectAttributeValues reports, for every test of an attribute against
// literal values, the attribute and the literals.
func collectAttributeValues(root celast.Expr, report func(attrRef, []any)) {
	literals := func(e celast.Expr) []any {
		if v, ok := literalOf(e); ok {
			return []any{v}
		}
		if e.Kind() == celast.ListKind {
			var out []any
			for _, el := range e.AsList().Elements() {
				if v, ok := literalOf(el); ok {
					out = append(out, v)
				}
			}
			return out
		}
		return nil
	}

	walk(root, func(n celast.Expr) bool {
		if n.Kind() != celast.CallKind {
			if r, ok := refOf(n); ok {
				report(r, nil)
				return false
			}
			return true
		}
		call := n.AsCall()
		fn := call.FunctionName()
		if call.IsMemberFunction() {
			if r, ok := refOf(call.Target()); ok && len(call.Args()) == 1 {
				report(r, literals(call.Args()[0]))
			}
			return true
		}
		args := call.Args()
		_, isCmp := comparisonOperators[fn]
		if (isCmp || fn == operators.In) && len(args) == 2 {
			if r, ok := refOf(args[0]); ok {
				report(r, literals(args[1]))
			}
			if r, ok := refOf(args[1]); ok {
				report(r, literals(args[0]))
			}
		}
		return true
	})
}

// ---------------------------------------------------------------------------
// Querying users.
// ---------------------------------------------------------------------------

// buildQuery translates an expression (names or IDs) into SQL for the
// attribute store.
func (s *Service) buildQuery(rctx request.CTX, expression, resourceID string, excludeNative bool) (string, []any, *model.AppError) {
	const where = "QueryUsersForExpression"
	cat, appErr := s.catalog(rctx)
	if appErr != nil {
		return "", nil, appErr
	}
	named := idsToNames(expression, cat)
	parsed, celErrs, err := s.parseExpression(named)
	if err != nil {
		return "", nil, model.NewAppError(where, "app.pap.query_expression.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if len(celErrs) > 0 {
		return "", nil, model.NewAppError(where, "app.pap.query_expression.app_error", nil, celErrs[0].Message, http.StatusBadRequest)
	}

	b := &sqlBuilder{cat: cat, graph: s.catalogs, now: s.now(), excludeNative: excludeNative}
	if referencesScope(parsed.Expr(), scopeResource) {
		if resourceID == "" {
			return "", nil, model.NewAppError(where, "app.pap.simulate.channel_required_for_resource", nil, "", http.StatusBadRequest)
		}
		raw, err := s.resourceAttributes(rctx, cat, resourceID)
		if err != nil {
			return "", nil, model.NewAppError(where, "app.pap.query_expression.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		b.resource = raw
		b.hasResource = true
	}

	q, err := b.build(parsed.Expr())
	if err != nil {
		var unsupportedErr *errUnsupportedSQL
		if errors.As(err, &unsupportedErr) {
			return "", nil, model.NewAppError(where, "app.pap.query_expression.app_error", nil, err.Error(), http.StatusBadRequest)
		}
		return "", nil, model.NewAppError(where, "app.pap.query_expression.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return q, b.args, nil
}

// QueryUsersForExpression implements PolicyAdministrationPointInterface.
func (s *Service) QueryUsersForExpression(rctx request.CTX, expression string, opts model.SubjectSearchOptions) ([]*model.User, int64, *model.AppError) {
	const where = "QueryUsersForExpression"
	st := s.store()
	if st == nil {
		return nil, 0, model.NewAppError(where, "app.pap.is_ready.app_error", nil, "", http.StatusInternalServerError)
	}
	if strings.TrimSpace(expression) == "" {
		return nil, 0, model.NewAppError(where, "app.pap.query_expression.app_error", nil, "expression is empty", http.StatusBadRequest)
	}

	celErrs, appErr := s.CheckExpression(rctx, expression)
	if appErr != nil {
		return nil, 0, appErr
	}
	if len(celErrs) > 0 {
		return nil, 0, model.NewAppError(where, "app.pap.query_expression.app_error", nil, celErrs[0].Message, http.StatusBadRequest)
	}

	s.refreshAttributesIfStale(rctx)
	q, args, appErr := s.buildQuery(rctx, expression, opts.ResourceID, opts.ExcludeNativeAttributes)
	if appErr != nil {
		return nil, 0, appErr
	}
	opts.Query = q
	opts.Args = args

	users, total, err := st.Attributes().SearchUsers(rctx, opts)
	if err != nil {
		return nil, 0, model.NewAppError(where, "app.pap.query_expression.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return users, total, nil
}

// joinExpressions AND-combines expressions, parenthesizing each.
func joinExpressions(rules []ruleExpr, op string) string {
	if len(rules) == 1 {
		return rules[0].expression
	}
	parts := make([]string, 0, len(rules))
	for _, r := range rules {
		parts = append(parts, "("+r.expression+")")
	}
	return strings.Join(parts, " "+op+" ")
}

// resourceExpression returns the combined expression governing action on the
// resource's policy, or "" when nothing governs it.
func (s *Service) resourceExpression(rctx request.CTX, policy *model.AccessControlPolicy, action string) (string, error) {
	if policy == nil {
		return "", nil
	}
	if action == "" || action == model.AccessControlPolicyActionMembership {
		rules, err := s.membershipRules(rctx, policy)
		if err != nil {
			return "", err
		}
		if len(rules) == 0 {
			return "", nil
		}
		return joinExpressions(rules, "&&"), nil
	}
	var rules []ruleExpr
	for _, r := range policy.Rules {
		if ruleHasAction(r, action) && usableExpression(r.Expression) {
			rules = append(rules, ruleExpr{expression: r.Expression})
		}
	}
	if len(rules) == 0 {
		return "", nil
	}
	return joinExpressions(rules, "||"), nil
}

// QueryUsersForResource implements PolicyAdministrationPointInterface. For a
// channel, users already in the channel are excluded unless the caller asked
// for a different exclusion.
func (s *Service) QueryUsersForResource(rctx request.CTX, resourceID, action string, opts model.SubjectSearchOptions) ([]*model.User, int64, *model.AppError) {
	const where = "QueryUsersForResource"
	st := s.store()
	if st == nil {
		return nil, 0, model.NewAppError(where, "app.pap.is_ready.app_error", nil, "", http.StatusInternalServerError)
	}
	policy, err := s.loadPolicy(rctx, resourceID)
	if err != nil {
		return nil, 0, model.NewAppError(where, "app.pap.get_policy.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	if policy != nil && policy.Type == model.AccessControlPolicyTypeChannel && opts.ExcludeChannelMembers == "" {
		opts.ExcludeChannelMembers = resourceID
	}

	expression, err := s.resourceExpression(rctx, policy, action)
	if err != nil {
		return nil, 0, model.NewAppError(where, "app.pap.get_policy.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	opts.Query, opts.Args = "", nil
	if expression != "" {
		s.refreshAttributesIfStale(rctx)
		resource := ""
		if policy.Type == model.AccessControlPolicyTypeChannel {
			resource = resourceID
		}
		q, args, appErr := s.buildQuery(rctx, expression, resource, false)
		if appErr != nil {
			return nil, 0, appErr
		}
		opts.Query, opts.Args = q, args
	}

	users, total, err := st.Attributes().SearchUsers(rctx, opts)
	if err != nil {
		return nil, 0, model.NewAppError(where, "app.pap.query_expression.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return users, total, nil
}

// GetChannelMembersToRemove implements PolicyAdministrationPointInterface.
func (s *Service) GetChannelMembersToRemove(rctx request.CTX, channelID string) ([]*model.ChannelMember, *model.AppError) {
	const where = "GetChannelMembersToRemove"
	st := s.store()
	if st == nil {
		return nil, model.NewAppError(where, "app.pap.is_ready.app_error", nil, "", http.StatusInternalServerError)
	}
	policy, err := s.loadPolicy(rctx, channelID)
	if err != nil {
		return nil, model.NewAppError(where, "app.pap.get_channel_members_to_remove.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if policy == nil || policy.Type != model.AccessControlPolicyTypeChannel {
		return []*model.ChannelMember{}, nil
	}
	expression, err := s.resourceExpression(rctx, policy, model.AccessControlPolicyActionMembership)
	if err != nil {
		return nil, model.NewAppError(where, "app.pap.get_channel_members_to_remove.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if expression == "" {
		return []*model.ChannelMember{}, nil
	}

	s.refreshAttributesIfStale(rctx)
	q, args, appErr := s.buildQuery(rctx, expression, channelID, false)
	if appErr != nil {
		return nil, appErr
	}
	members, err := st.Attributes().GetChannelMembersToRemove(rctx, channelID, model.SubjectSearchOptions{Query: q, Args: args})
	if err != nil {
		return nil, model.NewAppError(where, "app.pap.get_channel_members_to_remove.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return members, nil
}

// GetTeamMembersToRemove implements PolicyAdministrationPointInterface.
func (s *Service) GetTeamMembersToRemove(rctx request.CTX, teamID string) ([]*model.TeamMember, *model.AppError) {
	const where = "GetTeamMembersToRemove"
	st := s.store()
	if st == nil {
		return nil, model.NewAppError(where, "app.pap.is_ready.app_error", nil, "", http.StatusInternalServerError)
	}
	policy, err := s.loadPolicy(rctx, teamID)
	if err != nil {
		return nil, model.NewAppError(where, "app.pap.get_team_members_to_remove.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if policy == nil || policy.Type != model.AccessControlPolicyTypeTeam {
		return []*model.TeamMember{}, nil
	}
	expression, err := s.resourceExpression(rctx, policy, model.AccessControlPolicyActionMembership)
	if err != nil {
		return nil, model.NewAppError(where, "app.pap.get_team_members_to_remove.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if expression == "" {
		return []*model.TeamMember{}, nil
	}

	s.refreshAttributesIfStale(rctx)
	q, args, appErr := s.buildQuery(rctx, expression, "", false)
	if appErr != nil {
		return nil, appErr
	}
	members, err := st.Attributes().GetTeamMembersToRemove(rctx, teamID, model.SubjectSearchOptions{Query: q, Args: args})
	if err != nil {
		return nil, model.NewAppError(where, "app.pap.get_team_members_to_remove.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return members, nil
}
