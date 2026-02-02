package spanner

import (
	"context"
	"time"

	"github.com/authzed/spicedb/pkg/datastore"
)

func (sd *spannerDatastore) CheckIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string) (*datastore.IdempotencyResult, error) {
	// Spanner implementation would query the transaction_metadata table
	// For now, return nil (no caching)
	return nil, nil
}

func (sd *spannerDatastore) StoreIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string, revision datastore.Revision, ttl time.Duration) error {
	// Spanner implementation would insert into transaction_metadata table
	// For now, this is a no-op
	return nil
}
