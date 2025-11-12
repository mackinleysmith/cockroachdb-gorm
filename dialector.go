package cockroachdb

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Config re-exports postgres.Config so callers can reuse the same configuration
// structure when initializing the CockroachDB dialector.
type Config = postgres.Config

// Dialector implements gorm.Dialector for CockroachDB by wrapping the existing
// PostgreSQL dialector and extending it with Cockroach-specific behaviour.
type Dialector struct {
	*postgres.Dialector
}

// Open creates a new CockroachDB dialector using the given DSN.
func Open(dsn string) gorm.Dialector {
	if pgDialector, ok := postgres.Open(dsn).(*postgres.Dialector); ok {
		return &Dialector{Dialector: pgDialector}
	}
	return &Dialector{Dialector: &postgres.Dialector{Config: &postgres.Config{DSN: dsn}}}
}

// New creates a new CockroachDB dialector from the provided configuration.
func New(config Config) gorm.Dialector {
	if pgDialector, ok := postgres.New(config).(*postgres.Dialector); ok {
		return &Dialector{Dialector: pgDialector}
	}
	return &Dialector{Dialector: &postgres.Dialector{Config: &config}}
}

// Name returns the dialect name used by GORM.
func (dialector Dialector) Name() string {
	return "cockroachdb"
}

// Initialize wires the wrapped PostgreSQL dialector and registers the
// Cockroach-specific clause builders.
func (dialector *Dialector) Initialize(db *gorm.DB) error {
	if err := dialector.Dialector.Initialize(db); err != nil {
		return err
	}

	for name, builder := range dialector.clauseBuilders() {
		db.ClauseBuilders[name] = builder
	}

	return nil
}

// clauseBuilders returns the Cockroach-specific clause builders that augment
// the default ones provided by GORM / the PostgreSQL dialector.
func (dialector Dialector) clauseBuilders() map[string]clause.ClauseBuilder {
	return map[string]clause.ClauseBuilder{
		"FROM": fromClauseBuilder,
	}
}

// fromClauseBuilder renders the FROM clause and appends the AS OF SYSTEM TIME
// modifier if one has been configured via the CockroachDB helpers.
func fromClauseBuilder(c clause.Clause, builder clause.Builder) {
	if from, ok := c.Expression.(clause.From); ok {
		from.Build(builder)

		if stmt, ok := builder.(*gorm.Statement); ok {
			if raw, ok := stmt.Settings.Load(asOfSystemTimeKey); ok {
				if asOf, ok := raw.(string); ok && asOf != "" {
					builder.WriteByte(' ')
					builder.WriteString("AS OF SYSTEM TIME ")
					builder.WriteString(asOf)
				}
				stmt.Settings.Delete(asOfSystemTimeKey)
			}
		}
		return
	}

	c.Build(builder)
}
