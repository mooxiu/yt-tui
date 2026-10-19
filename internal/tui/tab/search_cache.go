package tab

import "time"

const (
	searchCacheTTL   = 15 * time.Minute
	searchCacheLimit = 50
)

type searchCacheEntry struct {
	result    srchResultMsg
	expiresAt time.Time
}

// cacheResult runs on the Update goroutine, so the map needs no locking.
// Cached hits keep their original expiration instead of extending it forever.
func (t *Search) cacheResult(result srchResultMsg) {
	now := time.Now()
	for query, entry := range t.cache {
		if !now.Before(entry.expiresAt) {
			delete(t.cache, query)
		}
	}
	if t.cache == nil {
		t.cache = make(map[string]searchCacheEntry)
	}
	if _, exists := t.cache[result.query]; !exists && len(t.cache) >= searchCacheLimit {
		var oldestQuery string
		var oldest time.Time
		for query, entry := range t.cache {
			if oldest.IsZero() || entry.expiresAt.Before(oldest) {
				oldestQuery, oldest = query, entry.expiresAt
			}
		}
		delete(t.cache, oldestQuery)
	}
	t.cache[result.query] = searchCacheEntry{result: result, expiresAt: now.Add(searchCacheTTL)}
}
