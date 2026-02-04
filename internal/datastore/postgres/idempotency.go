package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/authzed/spicedb/internal/datastore/postgres/schema"
	"github.com/authzed/spicedb/pkg/datastore"
)

func (pgd *pgDatastore) CheckIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string) (*datastore.IdempotencyResult, error) {
	// Query by the dedicated idempotency_key column for efficient lookup
	query := psql.Select(schema.ColXID, schema.ColMetadata, schema.ColTimestamp).
		From(schema.TableTransaction).
		Where(sq.Eq{schema.ColIdempotencyKey: idempotencyKey}).
		Limit(1)

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	var xid xid8
	var metadataJSON json.RawMessage
	var timestamp time.Time

	err = pgd.readPool.QueryRow(ctx, sqlQuery, args...).Scan(&xid, &metadataJSON, &timestamp)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	// Parse the metadata to extract the request hash
	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
		return nil, err
	}

	storedHash, ok := metadata[datastore.IdempotencyRequestHashMetadataKey].(string)
	if !ok {
		return nil, nil
	}

	return &datastore.IdempotencyResult{
		Revision:    postgresRevision{optionalTxID: xid},
		RequestHash: storedHash,
		CreatedAt:   timestamp,
	}, nil
}

func (pgd *pgDatastore) StoreIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string, revision datastore.Revision, ttl time.Duration) error {
	// The idempotency key is already stored when the transaction is created via createNewTransaction.
	// This method is a no-op for PostgreSQL since the key is stored in the transaction row itself.
	return nil
}
