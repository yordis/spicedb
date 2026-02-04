package migrations

import (
	"context"

	"github.com/jackc/pgx/v5"
)

const (
	addIdempotencyKeyColumn = `
		ALTER TABLE transaction_metadata
		ADD COLUMN IF NOT EXISTS idempotency_key STRING;
	`

	addIdempotencyKeyIndex = `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_idempotency_key
		ON transaction_metadata (idempotency_key)
		WHERE idempotency_key IS NOT NULL;
	`
)

func init() {
	err := CRDBMigrations.Register("add-idempotency-key-column", "add-expiration-support", addIdempotencyKeyColumnMigration, noAtomicMigration)
	if err != nil {
		panic("failed to register migration: " + err.Error())
	}
}

func addIdempotencyKeyColumnMigration(ctx context.Context, conn *pgx.Conn) error {
	if _, err := conn.Exec(ctx, addIdempotencyKeyColumn); err != nil {
		return err
	}

	if _, err := conn.Exec(ctx, addIdempotencyKeyIndex); err != nil {
		return err
	}

	return nil
}
