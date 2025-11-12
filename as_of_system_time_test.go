package cockroachdb

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
	"gorm.io/gorm/utils/tests"
)

func TestAsOfSystemTimeBuilder(t *testing.T) {
	db := setupTestDB(t)
	user, _ := schema.Parse(&tests.User{}, &sync.Map{}, db.NamingStrategy)

	cases := []struct {
		name     string
		modifier gorm.StatementModifier
		build    func(*gorm.Statement)
		wantSQL  string
	}{
		{
			name:     "time",
			modifier: AsOfSystemTime(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)),
			build: func(stmt *gorm.Statement) {
				stmt.AddClause(clause.Select{})
				stmt.AddClause(clause.From{Tables: []clause.Table{{Name: "users"}}})
			},
			wantSQL: "SELECT * FROM \"users\" AS OF SYSTEM TIME '2023-01-01 12:00:00.000000'",
		},
		{
			name:     "raw string",
			modifier: AsOfSystemTime("'-1h'"),
			build: func(stmt *gorm.Statement) {
				stmt.AddClause(clause.Select{})
				stmt.AddClause(clause.From{Tables: []clause.Table{{Name: "users"}}})
			},
			wantSQL: "SELECT * FROM \"users\" AS OF SYSTEM TIME '-1h'",
		},
		{
			name:     "with joins",
			modifier: AsOfSystemTime("'-1h'"),
			build: func(stmt *gorm.Statement) {
				stmt.AddClause(clause.Select{})
				stmt.AddClause(clause.From{
					Tables: []clause.Table{{Name: "users"}},
					Joins: []clause.Join{
						{
							Type:  clause.InnerJoin,
							Table: clause.Table{Name: "companies"},
							ON: clause.Where{
								Exprs: []clause.Expression{
									clause.Eq{Column: clause.Column{Table: "companies", Name: "id"}, Value: clause.Column{Table: "users", Name: "company_id"}},
								},
							},
						},
					},
				})
			},
			wantSQL: "SELECT * FROM \"users\" INNER JOIN \"companies\" ON \"companies\".\"id\" = \"users\".\"company_id\" AS OF SYSTEM TIME '-1h'",
		},
	}

	for idx, tc := range cases {
		t.Run(fmt.Sprintf("case %d: %s", idx, tc.name), func(t *testing.T) {
			stmt := gorm.Statement{
				Table:   user.Table,
				Schema:  user,
				DB:      db,
				Clauses: map[string]clause.Clause{},
			}

			tc.modifier.ModifyStatement(&stmt)
			tc.build(&stmt)

			stmt.Build("SELECT", "FROM")

			got := stmt.SQL.String()
			if got != tc.wantSQL {
				t.Fatalf("unexpected SQL: want %q, got %q", tc.wantSQL, got)
			}
		})
	}
}

func TestAsOfSystemTimeNow(t *testing.T) {
	modifier := AsOfSystemTimeNow()
	stmt := &gorm.Statement{}
	modifier.ModifyStatement(stmt)

	value, ok := stmt.Settings.Load(asOfSystemTimeKey)
	if !ok {
		t.Fatal("expected setting to be stored")
	}
	if value.(string) != "'-1µs'" {
		t.Fatalf("unexpected stored value %q", value)
	}
}

func TestAsOfSystemTimeUnsupportedType(t *testing.T) {
	modifier := AsOfSystemTime(123)
	stmt := &gorm.Statement{DB: &gorm.DB{}}

	modifier.ModifyStatement(stmt)

	if stmt.Error == nil {
		t.Fatal("expected an error to be registered on statement")
	}
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db := &gorm.DB{
		Config: &gorm.Config{
			ClauseBuilders: map[string]clause.ClauseBuilder{
				"FROM": fromClauseBuilder,
			},
			NamingStrategy: schema.NamingStrategy{},
		},
	}
	db.ClauseBuilders = db.Config.ClauseBuilders

	return db
}
