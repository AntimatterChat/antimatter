// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package ldap

import (
	"github.com/mattermost/ldap"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// AttributeSyncer applies the attribute values the directory reports for a
// user to the user attributes (custom profile attributes) synced from AD/LDAP.
// The app layer's PropertySyncer implements it: it matches select and
// multiselect values to options by name, creates the missing ones, clears what
// the directory no longer reports and prunes the options nobody holds any
// more. This package only reads the directory entry.
type AttributeSyncer interface {
	// HasMappings reports whether any user attribute is synced from AD/LDAP.
	HasMappings() bool
	// ExternalAttributes returns the directory attributes the synced user
	// attributes read from.
	ExternalAttributes() []string
	// SyncUser writes the values in attrs (directory attribute name -> values)
	// to the user's synced attributes.
	SyncUser(rctx request.CTX, userID string, attrs map[string][]string, opts model.PropertySyncOptions) (*model.PropertySyncResult, *model.AppError)
	// PruneOrphanedOptions removes the options of synced select and
	// multiselect attributes that nobody holds any more.
	PruneOrphanedOptions(rctx request.CTX) (*model.PropertySyncPruneResult, *model.AppError)
}

// loadAttributeSyncer returns the syncer of the user attributes synced from
// the directory, or nil when there are none, or when they cannot be loaded
// (logged; the attributes are then left as they are).
func (l *Ldap) loadAttributeSyncer(rctx request.CTX) AttributeSyncer {
	syncer, appErr := l.b.NewAttributeSyncer(rctx)
	if appErr != nil {
		rctx.Logger().LogM(mlog.MlvlLDAPWarn, "Unable to load the user attributes synchronized with AD/LDAP", mlog.Err(appErr))
		return nil
	}
	if syncer == nil || !syncer.HasMappings() {
		return nil
	}
	return syncer
}

// syncedAttributes returns the directory attributes to read for the synced
// user attributes.
func syncedAttributes(syncer AttributeSyncer) []string {
	if syncer == nil {
		return nil
	}
	return syncer.ExternalAttributes()
}

// entryAttributes reads the values of the synced directory attributes from an
// entry. Every value of a multi-valued attribute is kept, in directory order;
// an attribute the entry lacks is left out, which clears the user's value.
func entryAttributes(syncer AttributeSyncer, entry *ldap.Entry) map[string][]string {
	attrs := map[string][]string{}
	for _, name := range syncer.ExternalAttributes() {
		if values := attributeValues(entry, name); len(values) > 0 {
			attrs[name] = values
		}
	}
	return attrs
}

// syncAttributes writes a user's synced attributes from the directory entry.
// prune removes the options the user was the last to hold: a sign-in does it
// at once, the synchronization job once at the end of the run.
func (l *Ldap) syncAttributes(rctx request.CTX, syncer AttributeSyncer, userID string, entry *ldap.Entry, prune bool) {
	if syncer == nil || entry == nil {
		return
	}
	if _, appErr := syncer.SyncUser(rctx, userID, entryAttributes(syncer, entry), model.PropertySyncOptions{PruneOrphanedOptions: prune}); appErr != nil {
		rctx.Logger().LogM(mlog.MlvlLDAPWarn, "Failed to synchronize user attributes from AD/LDAP", mlog.String("user_id", userID), mlog.Err(appErr))
	}
}

// pruneAttributeOptions removes, after a synchronization run, the options of
// synced attributes that no user or channel holds any more.
func (l *Ldap) pruneAttributeOptions(rctx request.CTX, syncer AttributeSyncer) {
	if syncer == nil {
		return
	}
	if _, appErr := syncer.PruneOrphanedOptions(rctx); appErr != nil {
		rctx.Logger().LogM(mlog.MlvlLDAPWarn, "Failed to remove the unused options of user attributes synchronized with AD/LDAP", mlog.Err(appErr))
	}
}
