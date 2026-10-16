// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package accesscontrol

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// catalogTTL bounds how long a node may keep deciding from field metadata
// (names, types, option ranks) another node changed.
const catalogTTL = 30 * time.Second

// Attribute scopes of CEL selectors.
const (
	scopeUser     = "user"     // user.attributes.<name>
	scopeResource = "resource" // resource.attributes.<name>
	scopeSession  = "session"  // user.session.<name>
	scopeNative   = "native"   // user.<name>
)

// objectTypeForScope maps a CEL attribute scope to the property field object
// type backing it.
func objectTypeForScope(scope string) string {
	switch scope {
	case scopeUser:
		return model.PropertyFieldObjectTypeUser
	case scopeResource:
		return model.PropertyFieldObjectTypeChannel
	case scopeSession:
		return model.PropertyFieldObjectTypeSession
	}
	return ""
}

// fieldInfo is the per-field metadata the engine needs.
type fieldInfo struct {
	ID         string
	Name       string
	ObjectType string
	Type       model.PropertyFieldType
	Field      *model.PropertyField
	// ranks maps option name to rank for `rank` fields.
	ranks map[string]int64
}

// catalog indexes the access control property group's fields.
type catalog struct {
	groupID  string
	byName   map[string]map[string]*fieldInfo
	byID     map[string]*fieldInfo
	loadedAt time.Time
}

func (c *catalog) lookup(objectType, name string) *fieldInfo {
	if c == nil {
		return nil
	}
	if byName, ok := c.byName[objectType]; ok {
		if f, ok := byName[name]; ok {
			return f
		}
	}
	return nil
}

func (c *catalog) lookupScope(scope, name string) *fieldInfo {
	return c.lookup(objectTypeForScope(scope), name)
}

func (c *catalog) lookupID(id string) *fieldInfo {
	if c == nil {
		return nil
	}
	return c.byID[id]
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		if n != math.Trunc(n) {
			return 0, false
		}
		return int64(n), true
	case float32:
		return int64(n), true
	}
	return 0, false
}

// optionMaps returns the inline options of a field as (name→id, name→rank).
func optionMaps(field *model.PropertyField) (map[string]string, map[string]int64) {
	ids := map[string]string{}
	ranks := map[string]int64{}
	if field == nil || field.Attrs == nil {
		return ids, ranks
	}
	raw, ok := field.Attrs[model.PropertyFieldAttributeOptions]
	if !ok || raw == nil {
		return ids, ranks
	}

	var list []map[string]any
	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				list = append(list, m)
			}
		}
	case []map[string]any:
		list = v
	case []map[string]string:
		for _, item := range v {
			m := make(map[string]any, len(item))
			for k, val := range item {
				m[k] = val
			}
			list = append(list, m)
		}
	}

	for _, opt := range list {
		name, _ := opt["name"].(string)
		if name == "" {
			continue
		}
		if id, ok := opt["id"].(string); ok {
			ids[name] = id
		}
		if rank, ok := toInt64(opt["rank"]); ok {
			ranks[name] = rank
		}
	}
	return ids, ranks
}

func buildCatalog(groupID string, fields []*model.PropertyField) *catalog {
	c := &catalog{
		groupID:  groupID,
		byName:   map[string]map[string]*fieldInfo{},
		byID:     map[string]*fieldInfo{},
		loadedAt: time.Now(),
	}
	for _, f := range fields {
		if f == nil || f.DeleteAt != 0 {
			continue
		}
		objectType := f.ObjectType
		if objectType == "" {
			// Legacy (PSAv1) custom profile attributes carry no object type.
			objectType = model.PropertyFieldObjectTypeUser
		}
		info := &fieldInfo{
			ID:         f.ID,
			Name:       f.Name,
			ObjectType: objectType,
			Type:       f.Type,
			Field:      f,
		}
		if f.Type == model.PropertyFieldTypeRank {
			_, info.ranks = optionMaps(f)
		}
		c.byID[f.ID] = info
		if c.byName[objectType] == nil {
			c.byName[objectType] = map[string]*fieldInfo{}
		}
		// A typed (PSAv2) field wins over a legacy field of the same name.
		if existing, ok := c.byName[objectType][f.Name]; ok && existing.Field.ObjectType != "" && f.ObjectType == "" {
			continue
		}
		c.byName[objectType][f.Name] = info
	}
	return c
}

// catalogCache caches the catalog and graph lookups for a service.
type catalogCache struct {
	mu      sync.Mutex
	current *catalog

	graphMu   sync.Mutex
	nameToID  map[string]map[string]string   // fieldID -> name -> optionID
	idToName  map[string]map[string]string   // fieldID -> optionID -> name
	up        map[string]map[string][]string // fieldID -> optionID -> ancestors or self
	down      map[string]map[string][]string // fieldID -> optionID -> descendants or self
	graphTime time.Time

	storeFn func() store.Store
}

func newCatalogCache(storeFn func() store.Store) *catalogCache {
	c := &catalogCache{storeFn: storeFn}
	c.resetGraph()
	return c
}

func (c *catalogCache) resetGraph() {
	c.nameToID = map[string]map[string]string{}
	c.idToName = map[string]map[string]string{}
	c.up = map[string]map[string][]string{}
	c.down = map[string]map[string][]string{}
	c.graphTime = time.Now()
}

func (c *catalogCache) invalidateAll() {
	c.mu.Lock()
	c.current = nil
	c.mu.Unlock()

	c.graphMu.Lock()
	c.resetGraph()
	c.graphMu.Unlock()
}

func (c *catalogCache) invalidateField(fieldID string) {
	c.mu.Lock()
	c.current = nil
	c.mu.Unlock()

	c.graphMu.Lock()
	delete(c.nameToID, fieldID)
	delete(c.idToName, fieldID)
	delete(c.up, fieldID)
	delete(c.down, fieldID)
	c.graphMu.Unlock()
}

// get returns the (possibly cached) catalog of the access control property group.
func (c *catalogCache) get(rctx request.CTX) (*catalog, error) {
	c.mu.Lock()
	if c.current != nil && time.Since(c.current.loadedAt) < catalogTTL {
		cur := c.current
		c.mu.Unlock()
		return cur, nil
	}
	c.mu.Unlock()

	st := c.storeFn()
	if st == nil {
		return nil, errors.New("store is not available")
	}
	group, err := st.PropertyGroup().Get(model.AccessControlPropertyGroupName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the access control property group")
	}
	fields, err := st.PropertyField().GetForGroup(rctx, group.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the access control property fields")
	}
	cat := buildCatalog(group.ID, fields)

	c.mu.Lock()
	c.current = cat
	c.mu.Unlock()
	return cat, nil
}

func (c *catalogCache) checkGraphTTL() {
	if time.Since(c.graphTime) > catalogTTL {
		c.resetGraph()
	}
}

// optionIDsByName implements graphResolver.
func (c *catalogCache) optionIDsByName(field *fieldInfo, names []string) (map[string]string, error) {
	if field == nil || field.Field == nil {
		return nil, fmt.Errorf("unknown field")
	}
	c.graphMu.Lock()
	c.checkGraphTTL()
	cached := c.nameToID[field.ID]
	result := make(map[string]string, len(names))
	var missing []string
	for _, n := range names {
		if cached != nil {
			if id, ok := cached[n]; ok {
				if id != "" {
					result[n] = id
				}
				continue
			}
		}
		missing = append(missing, n)
	}
	c.graphMu.Unlock()

	if len(missing) == 0 {
		return result, nil
	}

	st := c.storeFn()
	if st == nil {
		return nil, errors.New("store is not available")
	}
	opts, err := st.PropertyField().GetOptionsByName(field.Field, missing)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve option names")
	}

	c.graphMu.Lock()
	defer c.graphMu.Unlock()
	if c.nameToID[field.ID] == nil {
		c.nameToID[field.ID] = map[string]string{}
	}
	if c.idToName[field.ID] == nil {
		c.idToName[field.ID] = map[string]string{}
	}
	for _, n := range missing {
		// Remember misses too, so an unknown name costs one query per TTL.
		c.nameToID[field.ID][n] = ""
	}
	for _, o := range opts {
		if o == nil {
			continue
		}
		c.nameToID[field.ID][o.Name] = o.ID
		c.idToName[field.ID][o.ID] = o.Name
		result[o.Name] = o.ID
	}
	return result, nil
}

// optionNamesByID implements graphResolver.
func (c *catalogCache) optionNamesByID(field *fieldInfo, ids []string) (map[string]string, error) {
	if field == nil || field.Field == nil {
		return nil, fmt.Errorf("unknown field")
	}
	c.graphMu.Lock()
	c.checkGraphTTL()
	cached := c.idToName[field.ID]
	result := make(map[string]string, len(ids))
	var missing []string
	for _, id := range ids {
		if cached != nil {
			if n, ok := cached[id]; ok {
				result[id] = n
				continue
			}
		}
		missing = append(missing, id)
	}
	c.graphMu.Unlock()
	if len(missing) == 0 {
		return result, nil
	}

	st := c.storeFn()
	if st == nil {
		return nil, errors.New("store is not available")
	}
	opts, err := st.PropertyField().GetOptionsByID(field.Field, missing)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve option ids")
	}
	c.graphMu.Lock()
	defer c.graphMu.Unlock()
	if c.idToName[field.ID] == nil {
		c.idToName[field.ID] = map[string]string{}
	}
	for _, o := range opts {
		if o == nil {
			continue
		}
		c.idToName[field.ID][o.ID] = o.Name
		result[o.ID] = o.Name
	}
	return result, nil
}

func (c *catalogCache) closure(field *fieldInfo, ids []string, up bool) (map[string][]string, error) {
	if field == nil || field.Field == nil {
		return nil, fmt.Errorf("unknown field")
	}
	c.graphMu.Lock()
	c.checkGraphTTL()
	cacheMap := c.down
	if up {
		cacheMap = c.up
	}
	cached := cacheMap[field.ID]
	result := make(map[string][]string, len(ids))
	var missing []string
	for _, id := range ids {
		if cached != nil {
			if set, ok := cached[id]; ok {
				result[id] = set
				continue
			}
		}
		missing = append(missing, id)
	}
	c.graphMu.Unlock()
	if len(missing) == 0 {
		return result, nil
	}

	st := c.storeFn()
	if st == nil {
		return nil, errors.New("store is not available")
	}
	var fetched map[string][]string
	var err error
	if up {
		fetched, err = st.PropertyField().GetOptionAncestorsOrSelf(field.Field, missing)
	} else {
		fetched, err = st.PropertyField().GetOptionDescendantsOrSelf(field.Field, missing)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to walk the option hierarchy")
	}

	c.graphMu.Lock()
	defer c.graphMu.Unlock()
	cacheMap = c.down
	if up {
		cacheMap = c.up
	}
	if cacheMap[field.ID] == nil {
		cacheMap[field.ID] = map[string][]string{}
	}
	for _, id := range missing {
		set := fetched[id] // absent option: nil, which answers "no" to every question
		cacheMap[field.ID][id] = set
		result[id] = set
	}
	return result, nil
}

// ancestorsOrSelf implements graphResolver.
func (c *catalogCache) ancestorsOrSelf(field *fieldInfo, ids []string) (map[string][]string, error) {
	return c.closure(field, ids, true)
}

// descendantsOrSelf implements graphResolver.
func (c *catalogCache) descendantsOrSelf(field *fieldInfo, ids []string) (map[string][]string, error) {
	return c.closure(field, ids, false)
}
