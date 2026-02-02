package crdb

import (
	"context"
	"time"

	"github.com/authzed/spicedb/pkg/datastore"
)

func (cds *crdbDatastore) CheckIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string) (*datastore.IdempotencyResult, error) {
	// CockroachDB implementation would query the transaction_metadata table
	// For now, return nil (no caching)
	return nil, nil
}

func (cds *crdbDatastore) StoreIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string, revision datastore.Revision, ttl time.Duration) error {
	// CockroachDB implementation would insert into transaction_metadata table
	// For now, this is a no-op
	return nil
}
