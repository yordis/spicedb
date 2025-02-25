package common

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/exaring/otelpgx"
	zerologadapter "github.com/jackc/pgx-zerolog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/authzed/spicedb/internal/datastore/common"
	log "github.com/authzed/spicedb/internal/logging"
	corev1 "github.com/authzed/spicedb/pkg/proto/core/v1"
)

const errUnableToQueryTuples = "unable to query tuples: %w"

// NewPGXExecutor creates an executor that uses the pgx library to make the specified queries.
func NewPGXExecutor(querier DBFuncQuerier) common.ExecuteQueryFunc {
	return func(ctx context.Context, sql string, args []any) ([]*corev1.RelationTuple, error) {
		span := trace.SpanFromContext(ctx)
		return queryTuples(ctx, sql, args, span, querier)
	}
}

// queryTuples queries tuples for the given query and transaction.
func queryTuples(ctx context.Context, sqlStatement string, args []any, span trace.Span, tx DBFuncQuerier) ([]*corev1.RelationTuple, error) {
	var tuples []*corev1.RelationTuple
	err := tx.QueryFunc(ctx, func(ctx context.Context, rows pgx.Rows) error {
		span.AddEvent("Query issued to database")

		for rows.Next() {
			nextTuple := &corev1.RelationTuple{
				ResourceAndRelation: &corev1.ObjectAndRelation{},
				Subject:             &corev1.ObjectAndRelation{},
			}
			var caveatName sql.NullString
			var caveatCtx map[string]any
			err := rows.Scan(
				&nextTuple.ResourceAndRelation.Namespace,
				&nextTuple.ResourceAndRelation.ObjectId,
				&nextTuple.ResourceAndRelation.Relation,
				&nextTuple.Subject.Namespace,
				&nextTuple.Subject.ObjectId,
				&nextTuple.Subject.Relation,
				&caveatName,
				&caveatCtx,
			)
			if err != nil {
				return fmt.Errorf(errUnableToQueryTuples, fmt.Errorf("scan err: %w", err))
			}

			nextTuple.Caveat, err = common.ContextualizedCaveatFrom(caveatName.String, caveatCtx)
			if err != nil {
				return fmt.Errorf(errUnableToQueryTuples, fmt.Errorf("unable to fetch caveat context: %w", err))
			}
			tuples = append(tuples, nextTuple)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf(errUnableToQueryTuples, fmt.Errorf("rows err: %w", err))
		}

		span.AddEvent("Tuples loaded", trace.WithAttributes(attribute.Int("tupleCount", len(tuples))))
		return nil
	}, sqlStatement, args...)
	if err != nil {
		return nil, err
	}

	return tuples, nil
}

// ConfigurePGXLogger sets zerolog global logger into the connection pool configuration, and maps
// info level events to debug, as they are rather verbose for SpiceDB's info level
func ConfigurePGXLogger(connConfig *pgx.ConnConfig) {
	levelMappingFn := func(logger tracelog.Logger) tracelog.LoggerFunc {
		return func(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]interface{}) {
			if level == tracelog.LogLevelInfo {
				level = tracelog.LogLevelDebug
			}

			// do not log cancelled queries as errors
			// do not log serialization failues as errors
			if errArg, ok := data["err"]; ok {
				err, ok := errArg.(error)
				if ok && errors.Is(err, context.Canceled) {
					return
				}

				var pgerr *pgconn.PgError
				if errors.As(err, &pgerr) && pgerr.SQLState() == pgSerializationFailure {
					return
				}
			}

			logger.Log(ctx, level, msg, data)
		}
	}

	l := zerologadapter.NewLogger(log.Logger, zerologadapter.WithSubDictionary("pgx"))
	addTracer(connConfig, &tracelog.TraceLog{Logger: levelMappingFn(l), LogLevel: tracelog.LogLevelInfo})
}

// ConfigureOTELTracer adds OTEL tracing to a pgx.ConnConfig
func ConfigureOTELTracer(connConfig *pgx.ConnConfig) {
	addTracer(connConfig, otelpgx.NewTracer(otelpgx.WithTrimSQLInSpanName()))
}

func addTracer(connConfig *pgx.ConnConfig, tracer pgx.QueryTracer) {
	composedTracer := addComposedTracer(connConfig)
	composedTracer.Tracers = append(composedTracer.Tracers, tracer)
}

func addComposedTracer(connConfig *pgx.ConnConfig) *ComposedTracer {
	var composedTracer *ComposedTracer
	if connConfig.Tracer == nil {
		composedTracer = &ComposedTracer{}
		connConfig.Tracer = composedTracer
	} else {
		var ok bool
		composedTracer, ok = connConfig.Tracer.(*ComposedTracer)
		if !ok {
			composedTracer.Tracers = append(composedTracer.Tracers, connConfig.Tracer)
			connConfig.Tracer = composedTracer
		}
	}
	return composedTracer
}

// ComposedTracer allows adding multiple tracers to a pgx.ConnConfig
type ComposedTracer struct {
	Tracers []pgx.QueryTracer
}

func (m *ComposedTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	for _, t := range m.Tracers {
		ctx = t.TraceQueryStart(ctx, conn, data)
	}

	return ctx
}

func (m *ComposedTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	for _, t := range m.Tracers {
		t.TraceQueryEnd(ctx, conn, data)
	}
}

// DBFuncQuerier is satisfied by RetryPool and QuerierFuncs (which can wrap a pgxpool or transaction)
type DBFuncQuerier interface {
	ExecFunc(ctx context.Context, tagFunc func(ctx context.Context, tag pgconn.CommandTag, err error) error, sql string, arguments ...any) error
	QueryFunc(ctx context.Context, rowsFunc func(ctx context.Context, rows pgx.Rows) error, sql string, optionsAndArgs ...any) error
	QueryRowFunc(ctx context.Context, rowFunc func(ctx context.Context, row pgx.Row) error, sql string, optionsAndArgs ...any) error
}

// PoolOptions is the set of configuration used for a pgx connection pool.
type PoolOptions struct {
	ConnMaxIdleTime         *time.Duration
	ConnMaxLifetime         *time.Duration
	ConnMaxLifetimeJitter   *time.Duration
	ConnHealthCheckInterval *time.Duration
	MinOpenConns            *int
	MaxOpenConns            *int
}

// ConfigurePgx applies PoolOptions to a pgx connection pool confiugration.
func (opts PoolOptions) ConfigurePgx(pgxConfig *pgxpool.Config) {
	if opts.MaxOpenConns != nil {
		pgxConfig.MaxConns = int32(*opts.MaxOpenConns)
	}

	if opts.MinOpenConns != nil {
		// Default to keeping the pool maxed out at all times.
		pgxConfig.MinConns = pgxConfig.MaxConns
	}

	if pgxConfig.MaxConns > 0 && pgxConfig.MinConns > 0 && pgxConfig.MaxConns < pgxConfig.MinConns {
		log.Warn().Int32("max-connections", pgxConfig.MaxConns).Int32("min-connections", pgxConfig.MinConns).Msg("maximum number of connections configured is less than minimum number of connections; minimum will be used")
	}

	if opts.ConnMaxIdleTime != nil {
		pgxConfig.MaxConnIdleTime = *opts.ConnMaxIdleTime
	}

	if opts.ConnMaxLifetime != nil {
		pgxConfig.MaxConnLifetime = *opts.ConnMaxLifetime
	}

	if opts.ConnHealthCheckInterval != nil {
		pgxConfig.HealthCheckPeriod = *opts.ConnHealthCheckInterval
	}

	if opts.ConnMaxLifetimeJitter != nil {
		pgxConfig.MaxConnLifetimeJitter = *opts.ConnMaxLifetimeJitter
	} else if opts.ConnMaxLifetime != nil {
		pgxConfig.MaxConnLifetimeJitter = time.Duration(0.2 * float64(*opts.ConnMaxLifetime))
	}

	ConfigurePGXLogger(pgxConfig.ConnConfig)
	ConfigureOTELTracer(pgxConfig.ConnConfig)
}

type QuerierFuncs struct {
	d Querier
}

func (t *QuerierFuncs) ExecFunc(ctx context.Context, tagFunc func(ctx context.Context, tag pgconn.CommandTag, err error) error, sql string, arguments ...any) error {
	tag, err := t.d.Exec(ctx, sql, arguments...)
	return tagFunc(ctx, tag, err)
}

func (t *QuerierFuncs) QueryFunc(ctx context.Context, rowsFunc func(ctx context.Context, rows pgx.Rows) error, sql string, optionsAndArgs ...any) error {
	rows, err := t.d.Query(ctx, sql, optionsAndArgs...)
	if err != nil {
		return err
	}
	defer rows.Close()
	return rowsFunc(ctx, rows)
}

func (t *QuerierFuncs) QueryRowFunc(ctx context.Context, rowFunc func(ctx context.Context, row pgx.Row) error, sql string, optionsAndArgs ...any) error {
	return rowFunc(ctx, t.d.QueryRow(ctx, sql, optionsAndArgs...))
}

func QuerierFuncsFor(d Querier) DBFuncQuerier {
	return &QuerierFuncs{d: d}
}
