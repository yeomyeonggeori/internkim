package admind

import (
	"testing"
	"time"
)

func TestMattermostSessionCacheFreshAndStaleWindows(t *testing.T) {
	cache := newMattermostSessionCache()
	key := mattermostSessionCacheKey("MMAUTHTOKEN=abc")
	base := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	cache.store(key, mattermostUserRecord{Email: "lee@example.com"}, base)

	if _, found := cache.lookup(key, base.Add(30*time.Second), mattermostSessionFreshTTL); !found {
		t.Fatal("entry within fresh TTL should be served")
	}
	if _, found := cache.lookup(key, base.Add(2*time.Minute), mattermostSessionFreshTTL); found {
		t.Fatal("entry past fresh TTL must not be served as fresh")
	}
	record, found := cache.lookup(key, base.Add(2*time.Minute), mattermostSessionStaleTTL)
	if !found || record.Email != "lee@example.com" {
		t.Fatal("entry within stale TTL should survive a mattermost outage")
	}
	if _, found := cache.lookup(key, base.Add(11*time.Minute), mattermostSessionStaleTTL); found {
		t.Fatal("entry past stale TTL must expire")
	}
}

func TestMattermostSessionCachePrunesExpiredOnStore(t *testing.T) {
	cache := newMattermostSessionCache()
	base := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	for index := 0; index <= mattermostSessionPruneSize; index++ {
		cache.store(mattermostSessionCacheKey(string(rune(index))), mattermostUserRecord{Email: "stale"}, base)
	}
	freshKey := mattermostSessionCacheKey("fresh")
	cache.store(freshKey, mattermostUserRecord{Email: "fresh"}, base.Add(2*mattermostSessionStaleTTL))
	if len(cache.entries) != 1 {
		t.Fatalf("expected expired entries pruned on store, got %d entries", len(cache.entries))
	}
}
