package cockroachdb

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	asOfSystemTimeKey = "gorm:cockroachdb:as_of_system_time"
	timeLayout        = "2006-01-02 15:04:05.000000"
)

type asOfSystemTimeModifier struct {
	expression string
	err        error
}

// ModifyStatement stores the computed AS OF SYSTEM TIME expression on the
// statement so that the custom FROM clause builder can append it to the final
// SQL.
func (m asOfSystemTimeModifier) ModifyStatement(stmt *gorm.Statement) {
	if m.err != nil {
		if stmt.DB != nil {
			stmt.AddError(m.err)
		}
		return
	}
	stmt.Settings.Store(asOfSystemTimeKey, m.expression)
}

// AsOfSystemTime returns a statement modifier that configures the query to use
// CockroachDB's AS OF SYSTEM TIME clause. The input can be either a raw string
// expression (e.g. "'-1h'" or "now() - interval '1 day'") or a time.Time value.
func AsOfSystemTime(timestamp interface{}) gorm.StatementModifier {
	return buildAsOfSystemTimeModifier(timestamp)
}

// AsOfSystemTimeNow configures the statement to read data as close to the
// current time as CockroachDB permits without incurring retry errors.
func AsOfSystemTimeNow() gorm.StatementModifier {
	return buildAsOfSystemTimeModifier("'-1µs'")
}

// AsOfSystemTime exposes the same helper on the dialector so callers can write
// db.Dialector.(interface{ AsOfSystemTime(any) gorm.StatementModifier }).
func (dialector Dialector) AsOfSystemTime(timestamp interface{}) gorm.StatementModifier {
	return buildAsOfSystemTimeModifier(timestamp)
}

// AsOfSystemTimeNow exposes the now helper on the dialector.
func (dialector Dialector) AsOfSystemTimeNow() gorm.StatementModifier {
	return buildAsOfSystemTimeModifier("'-1µs'")
}

func buildAsOfSystemTimeModifier(timestamp interface{}) asOfSystemTimeModifier {
	switch v := timestamp.(type) {
	case string:
		return asOfSystemTimeModifier{expression: sanitizeRawTimestamp(v)}
	case fmt.Stringer:
		return asOfSystemTimeModifier{expression: sanitizeRawTimestamp(v.String())}
	case time.Time:
		return asOfSystemTimeModifier{expression: fmt.Sprintf("'%s'", v.Format(timeLayout))}
	case *time.Time:
		if v == nil {
			return asOfSystemTimeModifier{err: fmt.Errorf("cockroachdb: nil *time.Time provided to AsOfSystemTime")}
		}
		return asOfSystemTimeModifier{expression: fmt.Sprintf("'%s'", v.Format(timeLayout))}
	default:
		return asOfSystemTimeModifier{err: fmt.Errorf("cockroachdb: unsupported timestamp type %T for AS OF SYSTEM TIME", timestamp)}
	}
}

func sanitizeRawTimestamp(raw string) string {
	return strings.ReplaceAll(raw, ";", "")
}
