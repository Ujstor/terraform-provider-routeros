package routeros

import (
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Default menus for bulk read when bulk_read is enabled and bulk_read_paths is empty.
var defaultBulkReadPaths = []string{
	"/ip/route",
	"/ipv6/route",
	"/interface/bridge/vlan",
}

type bulkReadConfig struct {
	enabled bool
	paths   map[string]struct{}
}

type bulkReadStore struct {
	cfg   bulkReadConfig
	paths map[string]*pathCacheEntry
	mu    sync.Mutex
}

type pathCacheEntry struct {
	warm   bool
	byID   map[string]MikrotikItem
	byName map[string]MikrotikItem
	mu     sync.RWMutex
	warmMu sync.Mutex
}

func parseBulkReadConfig(d *schema.ResourceData) bulkReadConfig {
	cfg := bulkReadConfig{
		enabled: d.Get("bulk_read").(bool),
		paths:   map[string]struct{}{},
	}
	if !cfg.enabled {
		return cfg
	}

	raw, _ := d.Get("bulk_read_paths").([]interface{})
	if len(raw) == 0 {
		for _, p := range defaultBulkReadPaths {
			cfg.paths[normalizeBulkReadPath(p)] = struct{}{}
		}
		return cfg
	}

	for _, v := range raw {
		p, ok := v.(string)
		if !ok || p == "" {
			continue
		}
		cfg.paths[normalizeBulkReadPath(p)] = struct{}{}
	}
	return cfg
}

func normalizeBulkReadPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimSuffix(path, "/")
}

func newBulkReadStore(cfg bulkReadConfig) *bulkReadStore {
	if !cfg.enabled || len(cfg.paths) == 0 {
		return nil
	}
	return &bulkReadStore{
		cfg:   cfg,
		paths: make(map[string]*pathCacheEntry),
	}
}

func (s *bulkReadStore) enabled(path string) bool {
	_, ok := s.cfg.paths[normalizeBulkReadPath(path)]
	return ok
}

func clientBulkReadStore(c Client) (*bulkReadStore, bool) {
	switch cl := c.(type) {
	case *ApiClient:
		if cl.bulkRead != nil {
			return cl.bulkRead, true
		}
	case *RestClient:
		if cl.bulkRead != nil {
			return cl.bulkRead, true
		}
	}
	return nil, false
}

func invalidateBulkReadCache(c Client, resourcePath, mikrotikID string) {
	store, ok := clientBulkReadStore(c)
	if !ok || mikrotikID == "" {
		return
	}
	store.invalidateByID(resourcePath, mikrotikID)
}

func (s *bulkReadStore) entry(path string) *pathCacheEntry {
	path = normalizeBulkReadPath(path)
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.paths[path]
	if !ok {
		e = &pathCacheEntry{
			byID:   make(map[string]MikrotikItem),
			byName: make(map[string]MikrotikItem),
		}
		s.paths[path] = e
	}
	return e
}

func copyMikrotikItem(item MikrotikItem) MikrotikItem {
	if item == nil {
		return nil
	}
	cp := make(MikrotikItem, len(item))
	for k, v := range item {
		cp[k] = v
	}
	return cp
}

func (e *pathCacheEntry) load(items []MikrotikItem) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.byID = make(map[string]MikrotikItem, len(items))
	e.byName = make(map[string]MikrotikItem, len(items))
	for _, item := range items {
		indexMikrotikItem(e, item)
	}
	e.warm = true
}

func indexMikrotikItem(e *pathCacheEntry, item MikrotikItem) {
	item = copyMikrotikItem(item)
	if id := item.GetID(Id); id != "" {
		e.byID[id] = item
	}
	if name := item.GetID(Name); name != "" {
		e.byName[name] = item
	}
}

func (e *pathCacheEntry) lookup(idType IdType, value string) (MikrotikItem, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	switch idType {
	case Id:
		item, ok := e.byID[value]
		return copyMikrotikItem(item), ok
	case Name:
		item, ok := e.byName[value]
		return copyMikrotikItem(item), ok
	default:
		return nil, false
	}
}

func (e *pathCacheEntry) upsert(item MikrotikItem) {
	e.mu.Lock()
	defer e.mu.Unlock()
	indexMikrotikItem(e, item)
}

func (e *pathCacheEntry) invalidateByID(mikrotikID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if item, ok := e.byID[mikrotikID]; ok {
		if name := item.GetID(Name); name != "" {
			delete(e.byName, name)
		}
	}
	delete(e.byID, mikrotikID)
}

func (s *bulkReadStore) invalidateByID(resourcePath, mikrotikID string) {
	if !s.enabled(resourcePath) {
		return
	}
	s.entry(resourcePath).invalidateByID(mikrotikID)
}

func (s *bulkReadStore) ensureWarm(resourcePath string, c Client) error {
	if !s.enabled(resourcePath) {
		return nil
	}
	e := s.entry(resourcePath)

	e.mu.RLock()
	if e.warm {
		e.mu.RUnlock()
		return nil
	}
	e.mu.RUnlock()

	e.warmMu.Lock()
	defer e.warmMu.Unlock()

	e.mu.RLock()
	if e.warm {
		e.mu.RUnlock()
		return nil
	}
	e.mu.RUnlock()

	items, err := readItemsUncached(nil, resourcePath, c)
	if err != nil {
		return err
	}
	if items == nil {
		e.load(nil)
		return nil
	}
	e.load(*items)
	return nil
}

func (s *bulkReadStore) readItems(id *ItemId, resourcePath string, c Client) (*[]MikrotikItem, error) {
	path := normalizeBulkReadPath(resourcePath)

	// Full list: warm cache and return (datasources, explicit print).
	if id == nil {
		items, err := readItemsUncached(nil, resourcePath, c)
		if err != nil {
			return nil, err
		}
		if items != nil {
			s.entry(path).load(*items)
		}
		return items, nil
	}

	if err := s.ensureWarm(resourcePath, c); err != nil {
		return nil, err
	}

	if item, ok := s.entry(path).lookup(id.Type, id.Value); ok {
		return &[]MikrotikItem{item}, nil
	}

	// Miss after warm: per-id lookup (cache-aside).
	items, err := readItemsUncached(id, resourcePath, c)
	if err != nil {
		return nil, err
	}
	if items != nil && len(*items) == 1 {
		s.entry(path).upsert((*items)[0])
	}
	return items, nil
}

// CachedPaths returns allowlisted paths that currently have a cache entry (for debugging).
func (s *bulkReadStore) CachedPaths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.paths))
	for p := range s.paths {
		out = append(out, p)
	}
	return out
}

func (s *bulkReadStore) cachedPathsString() string {
	if s == nil {
		return ""
	}
	allowlist := make([]string, 0, len(s.cfg.paths))
	for p := range s.cfg.paths {
		allowlist = append(allowlist, p)
	}
	msg := fmt.Sprintf("bulk_read enabled for: %s", strings.Join(allowlist, ", "))
	loaded := s.CachedPaths()
	if len(loaded) > 0 {
		msg += fmt.Sprintf("; loaded: %s", strings.Join(loaded, ", "))
	}
	return msg
}
