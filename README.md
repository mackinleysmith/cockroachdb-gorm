# CockroachDB Driver for GORM

This package provides a CockroachDB-specific dialector for GORM. It extends the
official PostgreSQL driver with CockroachDB features, including support for
[`AS OF SYSTEM TIME`](https://www.cockroachlabs.com/docs/stable/as-of-system-time).

## Installation

```sh
go get gorm.io/driver/cockroachdb
```

Add the driver to your application by importing the package and opening a GORM
connection with it:

```go
import (
	"gorm.io/driver/cockroachdb"
	"gorm.io/gorm"
)

db, err := gorm.Open(cockroachdb.Open("postgresql://user:pass@host:26257/db"))
```

The driver exposes the same configuration surface as `gorm.io/driver/postgres`,
so you can also call `cockroachdb.New(cockroachdb.Config{...})`.

## Temporal Queries with AS OF SYSTEM TIME

The driver adds helpers that let you build temporal queries without modifying
core GORM APIs.

```go
// Query data as it existed one hour ago.
db.Clauses(cockroachdb.AsOfSystemTime("'-1h'")).Find(&users)

// Query data at a specific timestamp.
db.Clauses(cockroachdb.AsOfSystemTime(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC))).Find(&users)

// Ask CockroachDB to serve data as close to "now" as possible without retries.
db.Clauses(cockroachdb.AsOfSystemTimeNow()).Find(&users)
```

For more fluent chaining, you can also access the helpers directly from the
dialector:

```go
asOf := db.Dialector.(interface {
	AsOfSystemTime(any) gorm.StatementModifier
}).AsOfSystemTime("'-30m'")

db.Clauses(asOf).Where("status = ?", "active").Find(&users)
```

These helpers sanitize raw string input by removing semicolons and format
`time.Time` values with microsecond precision to match CockroachDB's SQL
requirements. The generated SQL mirrors the implementation from
[`mackinleysmith/gorm`](https://github.com/mackinleysmith/gorm), ensuring that
queries place the `AS OF SYSTEM TIME` clause after any joins in the `FROM`
section.

## License

MIT

