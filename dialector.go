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
//
// Name() is inherited and stays "postgres": CockroachDB speaks the PostgreSQL
// dialect, and libraries such as gorm.io/datatypes pick column types and SQL by
// matching on that name.
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

// fromClauseBuilder renders the FROM clause as usual, then appends AS OF SYSTEM
// TIME if the statement carries one, so it lands after any joins.
func fromClauseBuilder(c clause.Clause, builder clause.Builder) {
	c.Build(builder)

	stmt, ok := builder.(*gorm.Statement)
	if !ok {
		return
	}
	if asOf, ok := stmt.Settings.Load(asOfSystemTimeKey); ok {
		builder.WriteString(" AS OF SYSTEM TIME ")
		builder.WriteString(asOf.(string))
	}
}
