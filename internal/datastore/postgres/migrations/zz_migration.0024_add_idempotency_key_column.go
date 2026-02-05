package migrations

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const addIdempotencyKeyColumn = `
	ALTER TABLE relation_tuple_transaction
	ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(256) DEFAULT NULL;
`

const addIdempotencyKeyIndex = `
	CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_idempotency_key
	ON relation_tuple_transaction (idempotency_key)
	WHERE idempotency_key IS NOT NULL;
`

func init() {
	if err := DatabaseMigrations.Register("add-idempotency-key-column", "add-index-for-transaction-gc",
		func(ctx context.Context, conn *pgx.Conn) error {
			if _, err := conn.Exec(ctx, addIdempotencyKeyColumn); err != nil {
				return fmt.Errorf("failed to add idempotency_key column to relation_tuple_transaction table: %w", err)
			}

			if _, err := conn.Exec(ctx, addIdempotencyKeyIndex); err != nil {
				return fmt.Errorf("failed to create unique index on idempotency_key: %w", err)
			}

			return nil
		},
		noTxMigration); err != nil {
		panic("failed to register migration: " + err.Error())
	}
}
