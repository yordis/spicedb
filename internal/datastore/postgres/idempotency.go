package postgres

import (
	"context"
	"encoding/json"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/authzed/spicedb/internal/datastore/postgres/schema"
	"github.com/authzed/spicedb/pkg/datastore"
)

func (pgd *pgDatastore) CheckIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string) (*datastore.IdempotencyResult, error) {
	query := sq.Select(
		schema.ColSnapshot,
		schema.ColMetadata,
		schema.ColTimestamp,
	).
		From(schema.TableTransaction).
		Where(sq.And{
			sq.Expr("(metadata->>'idempotency_key') = ?", idempotencyKey),
		}).
		OrderBy(schema.ColTimestamp + " DESC").
		Limit(1)

	row, err := pgd.execute(ctx, query)
	if err != nil {
		return nil, err
	}
	defer row.Close()

	if !row.Next() {
		return nil, nil
	}

	var snapshot string
	var metadata json.RawMessage
	var timestamp time.Time

	if err := row.Scan(&snapshot, &metadata, &timestamp); err != nil {
		return nil, err
	}

	// Parse stored request hash from metadata
	var metadataMap map[string]interface{}
	if err := json.Unmarshal(metadata, &metadataMap); err != nil {
		return nil, err
	}

	storedHash, ok := metadataMap["request_hash"].(string)
	if !ok {
		return nil, nil
	}

	// Create revision from snapshot
	revision, err := pgd.revisionFromSnapshot(snapshot)
	if err != nil {
		return nil, err
	}

	return &datastore.IdempotencyResult{
		Revision:    revision,
		RequestHash: storedHash,
		CreatedAt:   timestamp,
	}, nil
}

func (pgd *pgDatastore) StoreIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string, revision datastore.Revision, ttl time.Duration) error {
	// For PostgreSQL, we store the idempotency information in the metadata
	// during the transaction creation. This is a no-op since metadata is
	// already stored as part of ReadWriteTx options.
	return nil
}
