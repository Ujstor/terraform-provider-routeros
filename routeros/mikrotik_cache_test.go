package routeros

import (
	"testing"
)

func TestNormalizeBulkReadPath(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"/ip/route", "/ip/route"},
		{"ip/route", "/ip/route"},
		{"/ip/route/", "/ip/route"},
		{"  /ipv6/route  ", "/ipv6/route"},
	}
	for _, tt := range tests {
		if got := normalizeBulkReadPath(tt.in); got != tt.want {
			t.Errorf("normalizeBulkReadPath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPathCacheEntry_lookupAndInvalidate(t *testing.T) {
	e := &pathCacheEntry{
		byID:   make(map[string]MikrotikItem),
		byName: make(map[string]MikrotikItem),
	}
	e.load([]MikrotikItem{
		{".id": "*1", "name": "a"},
		{".id": "*2", "name": "b"},
	})

	if _, ok := e.lookup(Id, "*1"); !ok {
		t.Fatal("expected hit by id")
	}
	if _, ok := e.lookup(Name, "b"); !ok {
		t.Fatal("expected hit by name")
	}

	e.invalidateByID("*1")
	if _, ok := e.lookup(Id, "*1"); ok {
		t.Fatal("expected miss after invalidate")
	}
	if _, ok := e.lookup(Name, "a"); ok {
		t.Fatal("expected name index removed with id invalidate")
	}
	if _, ok := e.lookup(Id, "*2"); !ok {
		t.Fatal("expected other entry to remain")
	}
}

func TestBulkReadStore_enabledPaths(t *testing.T) {
	store := newBulkReadStore(bulkReadConfig{
		enabled: true,
		paths: map[string]struct{}{
			"/ip/route": {},
		},
	})
	if !store.enabled("/ip/route") || store.enabled("/interface/vlan") {
		t.Fatal("enabled path check failed")
	}
}

func TestBulkReadStore_missUpsert(t *testing.T) {
	store := newBulkReadStore(bulkReadConfig{
		enabled: true,
		paths:   map[string]struct{}{"/ip/route": {}},
	})
	e := store.entry("/ip/route")
	e.load([]MikrotikItem{{".id": "*1"}})

	if _, ok := e.lookup(Id, "*99"); ok {
		t.Fatal("expected miss before upsert")
	}
	e.upsert(MikrotikItem{".id": "*99", "gateway": "1.1.1.1"})
	item, ok := e.lookup(Id, "*99")
	if !ok || item["gateway"] != "1.1.1.1" {
		t.Fatalf("got %#v, ok=%v", item, ok)
	}
}

func TestNewBulkReadStore_disabled(t *testing.T) {
	if newBulkReadStore(bulkReadConfig{enabled: false}) != nil {
		t.Fatal("expected nil store when disabled")
	}
}
