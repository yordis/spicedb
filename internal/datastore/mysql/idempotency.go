package mysql

import (
	"context"
	"time"

	"github.com/authzed/spicedb/pkg/datastore"
)

func (md *mysqlDatastore) CheckIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string) (*datastore.IdempotencyResult, error) {
	// MySQL implementation would query the transaction_metadata table
	// For now, return nil (no caching)
	return nil, nil
}

func (md *mysqlDatastore) StoreIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string, revision datastore.Revision, ttl time.Duration) error {
	// MySQL implementation would insert into transaction_metadata table
	// For now, this is a no-op
	return nil
}
