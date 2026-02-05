package spanner

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/spanner"
	"google.golang.org/api/iterator"

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
func (sd *spannerDatastore) CheckIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string) (*datastore.IdempotencyResult, error) {
	// Query by the dedicated idempotency_key column for efficient lookup
	stmt := spanner.Statement{
		SQL: fmt.Sprintf(
			"SELECT %s, %s FROM %s WHERE %s = @idempotencyKey LIMIT 1",
			colMetadata,
			colCreatedAt,
			tableTransactionMetadata,
			colIdempotencyKey,
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
	var createdAt time.Time
	if err := row.Columns(&metadataJSON, &createdAt); err != nil {
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
		CreatedAt:   createdAt,
	}, nil
}

func (sd *spannerDatastore) StoreIdempotencyKey(ctx context.Context, idempotencyKey, requestHash string, revision datastore.Revision, ttl time.Duration) error {
	// The idempotency key is already stored when the transaction metadata is inserted.
	// This method is a no-op for Spanner since the key is stored in the transaction_metadata row.
	return nil
}
