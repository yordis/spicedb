package postgres

import (
	"context"
	"time"

	"github.com/authzed/spicedb/pkg/datastore"
)

func (pgd *pgDatastore) CheckIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string) (*datastore.IdempotencyResult, error) {
	// PostgreSQL implementation would query the transaction metadata
	// For now, this is a stub - full implementation requires database schema changes
	return nil, nil
}

func (pgd *pgDatastore) StoreIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string, revision datastore.Revision, ttl time.Duration) error {
	// PostgreSQL implementation would insert into transaction metadata
	// For now, this is a stub - full implementation requires database schema changes
	return nil
}
