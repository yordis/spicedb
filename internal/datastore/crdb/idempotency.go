package crdb

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/authzed/spicedb/internal/datastore/crdb/schema"
	"github.com/authzed/spicedb/pkg/datastore"
)

func (cds *crdbDatastore) CheckIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string) (*datastore.IdempotencyResult, error) {
	query := psql.Select(
		schema.ColMetadata,
		schema.ColExpiresAt,
	).
		From(schema.TableTransactionMetadata).
		Where(sq.Expr("(metadata->>?) = ?", datastore.IdempotencyKeyMetadataKey, idempotencyKey)).
		OrderBy(schema.ColExpiresAt + " DESC").
		Limit(1)

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	var metadata json.RawMessage
	var expiresAt time.Time
	rowErr := cds.readPool.QueryRowFunc(ctx, func(_ context.Context, row pgx.Row) error {
		return row.Scan(&metadata, &expiresAt)
	}, sqlQuery, args...)
	if rowErr != nil {
		if errors.Is(rowErr, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, rowErr
	}

	var metadataMap map[string]interface{}
	if err := json.Unmarshal(metadata, &metadataMap); err != nil {
		return nil, err
	}

	storedHash, ok := metadataMap[datastore.IdempotencyRequestHashMetadataKey].(string)
	if !ok {
		return nil, nil
	}

	return &datastore.IdempotencyResult{
		Revision:    datastore.NoRevision,
		RequestHash: storedHash,
		CreatedAt:   expiresAt,
	}, nil
}

func (cds *crdbDatastore) StoreIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string, revision datastore.Revision, ttl time.Duration) error {
	// CockroachDB implementation would insert into transaction_metadata table
	// For now, this is a no-op
	return nil
}
