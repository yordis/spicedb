package datastore

import (
	"fmt"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/spicedb/pkg/spiceerrors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IdempotencyKeyConflictError indicates that an idempotency key was used with a different request body.
type IdempotencyKeyConflictError struct {
	error
	idempotencyKey string
}

// NewIdempotencyKeyConflictError constructs a new idempotency key conflict error.
func NewIdempotencyKeyConflictError(idempotencyKey string) IdempotencyKeyConflictError {
	return IdempotencyKeyConflictError{
		error:          fmt.Errorf("idempotency key %q already used with different request body", idempotencyKey),
		idempotencyKey: idempotencyKey,
	}
}

// GRPCStatus implements retrieving the gRPC status for the error.
func (err IdempotencyKeyConflictError) GRPCStatus() *status.Status {
	return spiceerrors.WithCodeAndDetails(
		err,
		codes.InvalidArgument,
		spiceerrors.ForReason(
			v1.ErrorReason_ERROR_REASON_UNSPECIFIED,
			map[string]string{
				"idempotency_key": err.idempotencyKey,
				"error_type":      "idempotency_conflict",
			},
		),
	)
}
