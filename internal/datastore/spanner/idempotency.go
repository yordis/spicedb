package spanner

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/spanner"
	"google.golang.org/api/iterator"

	"github.com/authzed/spicedb/pkg/datastore"
)

func (sd *spannerDatastore) CheckIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string) (*datastore.IdempotencyResult, error) {
	stmt := spanner.Statement{
		SQL: fmt.Sprintf(
			"SELECT %s FROM %s WHERE JSON_VALUE(%s, '$.%s') = @idempotencyKey LIMIT 1",
			colMetadata,
			tableTransactionMetadata,
			colMetadata,
			datastore.IdempotencyKeyMetadataKey,
		),
		Params: map[string]interface{}{
			"idempotencyKey": idempotencyKey,
		},
	}

	iter := sd.client.Single().Query(ctx, stmt)
	defer iter.Stop()

	row, err := iter.Next()
	if err != nil {
		if err == iterator.Done {
			return nil, nil
		}
		return nil, err
	}

	var metadataJSON spanner.NullJSON
	if err := row.Columns(&metadataJSON); err != nil {
		return nil, err
	}

	if !metadataJSON.Valid || metadataJSON.Value == nil {
		return nil, nil
	}

	metadataMap, ok := metadataJSON.Value.(map[string]any)
	if !ok {
		return nil, nil
	}

	storedHash, ok := metadataMap[datastore.IdempotencyRequestHashMetadataKey].(string)
	if !ok {
		return nil, nil
	}

	return &datastore.IdempotencyResult{
		Revision:    datastore.NoRevision,
		RequestHash: storedHash,
		CreatedAt:   time.Time{},
	}, nil
}

func (sd *spannerDatastore) StoreIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string, revision datastore.Revision, ttl time.Duration) error {
	// Spanner implementation would insert into transaction_metadata table
	// For now, this is a no-op
	return nil
}
