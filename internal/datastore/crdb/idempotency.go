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

// CheckIdempotencyKey checks if an idempotency key exists and returns the stored result.
// Returns nil if the key is not found.
//
// NOTE: This intentionally returns datastore.NoRevision for the Revision field.
// The service layer will use HeadRevision() when NoRevision is returned. This design
// choice avoids the "new enemy problem" where returning an older cached revision could
// cause consistency issues. By always returning the latest revision, clients get a
// safe, consistent view of the data.
func (cds *crdbDatastore) CheckIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string) (*datastore.IdempotencyResult, error) {
	// Query by the dedicated idempotency_key column for efficient lookup
	query := psql.Select(
		schema.ColMetadata,
	).
		From(schema.TableTransactionMetadata).
		Where(sq.Eq{schema.ColIdempotencyKey: idempotencyKey}).
		Limit(1)

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	var metadata json.RawMessage
	rowErr := cds.readPool.QueryRowFunc(ctx, func(_ context.Context, row pgx.Row) error {
		return row.Scan(&metadata)
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
		CreatedAt:   time.Now(),
	}, nil
}

func (cds *crdbDatastore) StoreIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string, revision datastore.Revision, ttl time.Duration) error {
	// The idempotency key is already stored when the transaction metadata is inserted.
	// This method is a no-op for CockroachDB since the key is stored in the transaction_metadata row.
	return nil
}
