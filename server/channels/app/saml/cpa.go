// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package saml

import (
	"net/http"

	saml2 "github.com/mattermost/gosaml2"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// AttributeSyncer applies the attribute values a SAML assertion carries to the
// user attributes (custom profile attributes) synced from SAML. The app
// layer's PropertySyncer implements it: text, select and multiselect
// attributes are synced, select and multiselect values are matched to options
// by name and missing options are created, and an attribute also synced from
// AD/LDAP is left to the AD/LDAP synchronization.
type AttributeSyncer interface {
	// HasMappings reports whether any user attribute is synced from SAML.
	HasMappings() bool
	// ExternalAttributes returns the SAML attributes the synced user
	// attributes read from.
	ExternalAttributes() []string
	// SyncUser writes the values in attrs (SAML attribute name -> values) to
	// the user's synced attributes.
	SyncUser(rctx request.CTX, userID string, attrs map[string][]string, opts model.PropertySyncOptions) (*model.PropertySyncResult, *model.AppError)
}

// syncCustomProfileAttributes copies the SAML attributes mapped on user
// attributes (attrs "saml": "<attribute name>") into the user's values. A
// mapped attribute absent from the assertion clears the value, so that revoked
// attributes do not linger (they may drive access control policies), and the
// options the user was the last to hold are removed.
func (s *Service) syncCustomProfileAttributes(rctx request.CTX, user *model.User, info *saml2.AssertionInfo) *model.AppError {
	syncer, appErr := s.b.NewAttributeSyncer(rctx)
	if appErr != nil {
		return appErr
	}
	if syncer == nil || !syncer.HasMappings() {
		return nil
	}

	// An assertion without any attribute statement says nothing about the
	// user's attributes (usually an Identity Provider misconfiguration):
	// rather than clearing every synced value, leave them as they are.
	if !hasAttributeStatement(info) {
		return model.NewAppError("syncCustomProfileAttributes", "ent.saml.update_cpa.empty_attribute_statement", nil, "", http.StatusBadRequest)
	}

	_, appErr = syncer.SyncUser(rctx, user.Id, assertionAttributes(syncer, info), model.PropertySyncOptions{PruneOrphanedOptions: true})
	return appErr
}

// assertionAttributes reads every value of the synced SAML attributes from the
// assertion, in assertion order. An attribute the assertion lacks is left out,
// which clears the user's value.
func assertionAttributes(syncer AttributeSyncer, info *saml2.AssertionInfo) map[string][]string {
	attrs := map[string][]string{}
	for _, name := range syncer.ExternalAttributes() {
		if values, ok := getAttributeValues(info, name); ok && len(values) > 0 {
			attrs[name] = values
		}
	}
	return attrs
}

func hasAttributeStatement(info *saml2.AssertionInfo) bool {
	if info == nil {
		return false
	}
	if len(info.Values) > 0 {
		return true
	}
	for _, a := range info.Assertions {
		if a.AttributeStatement != nil {
			return true
		}
	}
	return false
}
