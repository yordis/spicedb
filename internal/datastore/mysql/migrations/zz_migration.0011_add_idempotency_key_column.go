package migrations

import "fmt"

func addIdempotencyKeyToTransactionTable(t *tables) string {
	return fmt.Sprintf(`ALTER TABLE %s
		ADD COLUMN idempotency_key VARCHAR(256) NULL DEFAULT NULL;`,
		t.RelationTupleTransaction(),
	)
}

func addIdempotencyKeyIndex(t *tables) string {
	return fmt.Sprintf(`CREATE UNIQUE INDEX idx_idempotency_key ON %s (idempotency_key);`,
		t.RelationTupleTransaction(),
	)
}

func init() {
	mustRegisterMigration("add_idempotency_key_column", "add_expiration_to_relation_tuple", noNonatomicMigration,
		newStatementBatch(
			addIdempotencyKeyToTransactionTable,
			addIdempotencyKeyIndex,
		).execute,
	)
}
