package cockroachdb

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	asOfSystemTimeKey = "gorm:cockroachdb:as_of_system_time"
	timeLayout        = "2006-01-02 15:04:05.000000"
)

// AsOfSystemTimeClause configures a query to read at a past system time. Pass it
// to db.Clauses; the CockroachDB FROM clause builder appends it after any joins.
// On a non-CockroachDB connection it has no effect.
type AsOfSystemTimeClause struct {
	expression string
	err        error
}

var (
	_ gorm.StatementModifier = AsOfSystemTimeClause{}
	_ clause.Expression      = AsOfSystemTimeClause{}
)

// ModifyStatement stores the AS OF SYSTEM TIME expression on the statement. It
// lives in the statement settings, so it is cloned along with the statement and
// survives rebuilding the same statement (e.g. Count followed by Find).
func (c AsOfSystemTimeClause) ModifyStatement(stmt *gorm.Statement) {
	if c.err != nil {
		if stmt.DB != nil {
			stmt.AddError(c.err)
		}
		return
	}
	stmt.Settings.Store(asOfSystemTimeKey, c.expression)
}

// Build satisfies clause.Expression so the value can be passed to db.Clauses.
// Rendering happens in the FROM clause builder, never here.
func (AsOfSystemTimeClause) Build(clause.Builder) {}

// AsOfSystemTime reads data as of the given time. The input can be either a raw
// string expression (e.g. "'-1h'" or "now() - interval '1 day'") or a time.Time.
//
//	db.Clauses(cockroachdb.AsOfSystemTime("'-1h'")).Find(&users)
func AsOfSystemTime(timestamp any) AsOfSystemTimeClause {
	switch v := timestamp.(type) {
	case string:
		return AsOfSystemTimeClause{expression: sanitizeRawTimestamp(v)}
	case time.Time:
		return AsOfSystemTimeClause{expression: fmt.Sprintf("'%s'", v.Format(timeLayout))}
	case *time.Time:
		if v == nil {
			return AsOfSystemTimeClause{err: fmt.Errorf("cockroachdb: nil *time.Time provided to AsOfSystemTime")}
		}
		return AsOfSystemTimeClause{expression: fmt.Sprintf("'%s'", v.Format(timeLayout))}
	default:
		return AsOfSystemTimeClause{err: fmt.Errorf("cockroachdb: unsupported timestamp type %T for AS OF SYSTEM TIME", timestamp)}
	}
}

// AsOfSystemTimeNow reads data as close to the current time as CockroachDB
// permits without incurring retry errors.
func AsOfSystemTimeNow() AsOfSystemTimeClause {
	return AsOfSystemTime("'-1µs'")
}

func sanitizeRawTimestamp(raw string) string {
	return strings.ReplaceAll(raw, ";", "")
}
