package memdb

import (
	"context"
	"sync"
	"time"

	"github.com/authzed/spicedb/pkg/datastore"
)

type idempotencyCacheEntry struct {
	result    *datastore.IdempotencyResult
	expiresAt time.Time
}

func (mds *memdbDatastore) CheckIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string) (*datastore.IdempotencyResult, error) {
	mds.idempotencyMutex.RLock()
	defer mds.idempotencyMutex.RUnlock()

	if mds.idempotencyCache == nil {
		return nil, nil
	}

	entry, exists := mds.idempotencyCache[idempotencyKey]
	if !exists {
		return nil, nil
	}

	// Check if expired
	if time.Now().After(entry.expiresAt) {
		return nil, nil
	}

	return entry.result, nil
}

func (mds *memdbDatastore) StoreIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string, revision datastore.Revision, ttl time.Duration) error {
	mds.idempotencyMutex.Lock()
	defer mds.idempotencyMutex.Unlock()

	if mds.idempotencyCache == nil {
		mds.idempotencyCache = make(map[string]idempotencyCacheEntry)
	}

	mds.idempotencyCache[idempotencyKey] = idempotencyCacheEntry{
		result: &datastore.IdempotencyResult{
			Revision:    revision,
			RequestHash: requestHash,
			CreatedAt:   time.Now(),
		},
		expiresAt: time.Now().Add(ttl),
	}

	return nil
}

func (mds *memdbDatastore) initIdempotencyCache() {
	mds.idempotencyMutex = sync.RWMutex{}
	mds.idempotencyCache = make(map[string]idempotencyCacheEntry)
}
