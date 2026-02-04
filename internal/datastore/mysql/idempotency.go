package mysql

import (
	"context"
	"database/sql"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"

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
func (md *mysqlDatastore) CheckIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string) (*datastore.IdempotencyResult, error) {
	// Query by the dedicated idempotency_key column for efficient lookup
	query := sb.Select(colMetadata, colTimestamp).
		From(md.driver.RelationTupleTransaction()).
		Where(sq.Eq{colIdempotencyKey: idempotencyKey}).
		Limit(1)

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	var metadata structpbWrapper
	var timestamp time.Time
	if err := md.db.QueryRowContext(ctx, sqlQuery, args...).Scan(&metadata, &timestamp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	storedHash, ok := metadata[datastore.IdempotencyRequestHashMetadataKey].(string)
	if !ok {
		return nil, nil
	}

	return &datastore.IdempotencyResult{
		Revision:    datastore.NoRevision,
		RequestHash: storedHash,
		CreatedAt:   timestamp,
	}, nil
}

func (md *mysqlDatastore) StoreIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string, revision datastore.Revision, ttl time.Duration) error {
	// The idempotency key is already stored when the transaction is created via createNewTransaction.
	// This method is a no-op for MySQL since the key is stored in the transaction row itself.
	return nil
}
