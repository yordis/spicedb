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

// CheckIdempotencyKey checks if an idempotency key exists and returns the stored result.
// Returns nil if the key is not found or if the key has expired.
//
// NOTE: This intentionally returns datastore.NoRevision for the Revision field.
// The service layer will use HeadRevision() when NoRevision is returned. This design
// choice avoids the "new enemy problem" where returning an older cached revision could
// cause consistency issues. By always returning the latest revision, clients get a
// safe, consistent view of the data.
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

	// Return NoRevision intentionally - service layer will use HeadRevision()
	return &datastore.IdempotencyResult{
		Revision:    datastore.NoRevision,
		RequestHash: entry.result.RequestHash,
		CreatedAt:   entry.result.CreatedAt,
	}, nil
}

// StoreIdempotencyKey stores an idempotency key with a TTL.
// For memdb, this stores the result in an in-memory cache with expiration.
func (mds *memdbDatastore) StoreIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string, revision datastore.Revision, ttl time.Duration) error {
	mds.idempotencyMutex.Lock()
	defer mds.idempotencyMutex.Unlock()

	if mds.idempotencyCache == nil {
		mds.idempotencyCache = make(map[string]idempotencyCacheEntry)
	}

	mds.idempotencyCache[idempotencyKey] = idempotencyCacheEntry{
		result: &datastore.IdempotencyResult{
			Revision:    datastore.NoRevision,
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
