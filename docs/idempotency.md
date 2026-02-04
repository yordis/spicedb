# Idempotency Keys

SpiceDB supports idempotency keys for `WriteRelationships` operations, allowing clients to safely retry requests without causing duplicate effects.

## Overview

Idempotency keys ensure that repeated requests with the same key and request body produce the same effect. Responses may include a newer `WrittenAt` revision than the original request, but the state changes are not duplicated. This is particularly useful in distributed systems where network failures or timeouts may cause clients to retry requests, preventing issues like:

- Duplicate relationship creation
- Race conditions in event processing pipelines
- Inconsistent state due to retry storms

## Usage

To use idempotency keys, include the `idempotency_key` field in your `WriteRelationshipsRequest`:

```protobuf
message WriteRelationshipsRequest {
  repeated RelationshipUpdate updates = 1;
  repeated Precondition optional_preconditions = 2;
  google.protobuf.Struct optional_transaction_metadata = 3;
  string idempotency_key = 4;  // Optional idempotency key
}
```

### Example (Go)

```go
resp, err := client.WriteRelationships(ctx, &v1.WriteRelationshipsRequest{
    Updates: []*v1.RelationshipUpdate{
        {
            Operation: v1.RelationshipUpdate_OPERATION_CREATE,
            Relationship: &v1.Relationship{
                Resource: &v1.ObjectReference{
                    ObjectType: "document",
                    ObjectId:   "doc1",
                },
                Relation: "viewer",
                Subject: &v1.SubjectReference{
                    Object: &v1.ObjectReference{
                        ObjectType: "user",
                        ObjectId:   "user1",
                    },
                },
            },
        },
    },
    IdempotencyKey: "unique-event-id-12345",
})
```

## Key Format Requirements

Idempotency keys must meet the following requirements:

| Requirement | Value |
|-------------|-------|
| Maximum length | 256 characters |
| Character set | Valid UTF-8 |
| Forbidden characters | Null character (`\x00`) |

Recommended key formats include:
- UUIDs: `550e8400-e29b-41d4-a716-446655440000`
- Event IDs from message queues: `event:kafka:topic:partition:offset`
- Request identifiers: `req-2024-01-15-abc123`

## Behavior

### Same Key, Same Request

When a request is received with an idempotency key that was previously used with the same request body:
- The request succeeds
- The response contains a valid ZedToken (which may be newer than the original)
- No duplicate writes occur

### Same Key, Different Request

When a request is received with an idempotency key that was previously used with a **different** request body:
- The request fails with an error
- The error indicates an idempotency conflict
- The original operation's state is preserved

### No Key

When no idempotency key is provided:
- Each request is treated as a new operation
- Standard retry logic applies (retries may create duplicates for non-idempotent operations)

## TTL and Cleanup

Idempotency keys are retained for a configurable duration (default: 24 hours) where the datastore
supports explicit TTL cleanup. For SQL and Spanner backends, retention is governed by the datastore's
transaction metadata cleanup policy (for example, GC windows or row-deletion policies), which may be
longer than the configured TTL. A key can be safely reused only after the underlying metadata row
is removed.

Retention ensures that:
- Storage requirements remain bounded
- Old keys don't block new operations indefinitely
- Reasonable retry windows are supported

## Best Practices

### Generate Unique Keys

Use globally unique identifiers for idempotency keys:

```go
// Good: UUID
idempotencyKey := uuid.New().String()

// Good: Composite key from event source
idempotencyKey := fmt.Sprintf("kafka:%s:%d:%d", topic, partition, offset)

// Bad: Sequential counter (may conflict across instances)
idempotencyKey := fmt.Sprintf("req-%d", counter)
```

### Include Context in Keys

For event-driven systems, derive the idempotency key from the event's unique identifier:

```go
// Kafka event
idempotencyKey := fmt.Sprintf("kafka:%s:%d:%d", 
    event.Topic, event.Partition, event.Offset)

// NATS JetStream
idempotencyKey := fmt.Sprintf("nats:%s:%d", 
    msg.Subject, msg.Sequence)
```

### Handle Conflict Errors

Always handle idempotency conflict errors in your application:

```go
resp, err := client.WriteRelationships(ctx, req)
if err != nil {
    if status.Code(err) == codes.InvalidArgument {
        // Check if it's an idempotency conflict
        // Log and potentially alert on this condition
        log.Warn("idempotency conflict detected", "key", req.IdempotencyKey)
    }
    return err
}
```

### Use for At-Least-Once Delivery

Idempotency keys are ideal for systems with at-least-once message delivery:

```go
func processEvent(ctx context.Context, event Event) error {
    req := &v1.WriteRelationshipsRequest{
        Updates:        buildUpdates(event),
        IdempotencyKey: event.ID, // Use event ID as idempotency key
    }
    
    _, err := client.WriteRelationships(ctx, req)
    // Safe to retry on transient errors - idempotency key prevents duplicates
    return err
}
```

## Limitations

1. **WriteRelationships only**: Idempotency keys are only supported for `WriteRelationships` operations.

2. **Request body matching**: The entire request body (updates, preconditions, metadata) is hashed for conflict detection. Any change to the request body with the same key will result in a conflict error.

3. **No revision replay**: For safety reasons (avoiding the "new enemy problem"), replayed idempotent requests return the current head revision, not the original revision. Tokens may differ between retries even when the effect is identical.

4. **Storage overhead**: Each idempotency key requires storage until the TTL expires. High-volume systems should consider the storage implications.

## Monitoring

SpiceDB exposes the following Prometheus metrics for idempotency:

| Metric | Type | Description |
|--------|------|-------------|
| `spicedb_v1_idempotency_cache_hit_total` | Counter | Number of idempotent request replays |
| `spicedb_v1_idempotency_cache_miss_total` | Counter | Number of new idempotency keys |
| `spicedb_v1_idempotency_conflict_total` | Counter | Number of idempotency conflicts |
| `spicedb_v1_idempotency_store_error_total` | Counter | Number of idempotency storage errors |

## See Also

- [WriteRelationships API Reference](https://buf.build/authzed/api/docs/main:authzed.api.v1#authzed.api.v1.PermissionsService.WriteRelationships)
