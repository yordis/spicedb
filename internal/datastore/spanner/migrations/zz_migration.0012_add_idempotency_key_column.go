package migrations

import (
	"context"

	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
)

const (
	addIdempotencyKeyColumn = `
		ALTER TABLE transaction_metadata ADD COLUMN idempotency_key STRING(256)
	`

	addIdempotencyKeyIndex = `
		CREATE UNIQUE NULL_FILTERED INDEX idx_idempotency_key ON transaction_metadata (idempotency_key)
	`
)

func init() {
	if err := SpannerMigrations.Register("add-idempotency-key-column", "add-expiration-support", func(ctx context.Context, w Wrapper) error {
		updateOp, err := w.adminClient.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
			Database: w.client.DatabaseName(),
			Statements: []string{
				addIdempotencyKeyColumn,
				addIdempotencyKeyIndex,
			},
		})
		if err != nil {
			return err
		}
		return updateOp.Wait(ctx)
	}, nil); err != nil {
		panic("failed to register migration: " + err.Error())
	}
}
