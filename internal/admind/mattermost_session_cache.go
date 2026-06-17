package admind

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

const (
	mattermostSessionFreshTTL      = time.Minute
	mattermostSessionStaleTTL      = 10 * time.Minute
	mattermostSessionLookupTimeout = 3 * time.Second
	mattermostSessionPruneSize     = 256
)

type mattermostSessionCacheEntry struct {
	userRecord  mattermostUserRecord
	refreshedAt time.Time
}

type mattermostSessionCache struct {
	mutex   sync.Mutex
	entries map[string]mattermostSessionCacheEntry
}

func newMattermostSessionCache() *mattermostSessionCache {
	return &mattermostSessionCache{entries: map[string]mattermostSessionCacheEntry{}}
}

func mattermostSessionCacheKey(cookieHeader string) string {
	digest := sha256.Sum256([]byte(cookieHeader))
	return hex.EncodeToString(digest[:])
}

func (cache *mattermostSessionCache) lookup(key string, now time.Time, maximumAge time.Duration) (mattermostUserRecord, bool) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	entry, found := cache.entries[key]
	if !found || now.Sub(entry.refreshedAt) > maximumAge {
		return mattermostUserRecord{}, false
	}
	return entry.userRecord, true
}

func (cache *mattermostSessionCache) store(key string, userRecord mattermostUserRecord, now time.Time) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	if len(cache.entries) > mattermostSessionPruneSize {
		cache.pruneExpired(now)
	}
	cache.entries[key] = mattermostSessionCacheEntry{userRecord: userRecord, refreshedAt: now}
}

func (cache *mattermostSessionCache) pruneExpired(now time.Time) {
	for key, entry := range cache.entries {
		if now.Sub(entry.refreshedAt) > mattermostSessionStaleTTL {
			delete(cache.entries, key)
		}
	}
}
