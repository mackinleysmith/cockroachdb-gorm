package cockroachdb

import (
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type user struct {
	ID        uint
	Name      string
	CompanyID uint
}

type company struct {
	ID   uint
	Name string
}

// dryRunDB opens a dialector without connecting; queries only render SQL.
func dryRunDB(t *testing.T, dialector gorm.Dialector) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(dialector, &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

const dsn = "postgres://user@localhost:26257/db"

func TestAsOfSystemTimeSQL(t *testing.T) {
	db := dryRunDB(t, Open(dsn))

	cases := []struct {
		name  string
		query func(*gorm.DB) *gorm.DB
		want  string
	}{
		{
			name: "time",
			query: func(tx *gorm.DB) *gorm.DB {
				return tx.Clauses(AsOfSystemTime(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC))).Find(&[]user{})
			},
			want: `SELECT * FROM "users" AS OF SYSTEM TIME '2023-01-01 12:00:00.000000'`,
		},
		{
			name:  "raw string",
			query: func(tx *gorm.DB) *gorm.DB { return tx.Clauses(AsOfSystemTime("'-1h'")).Find(&[]user{}) },
			want:  `SELECT * FROM "users" AS OF SYSTEM TIME '-1h'`,
		},
		{
			name:  "now",
			query: func(tx *gorm.DB) *gorm.DB { return tx.Clauses(AsOfSystemTimeNow()).Find(&[]user{}) },
			want:  `SELECT * FROM "users" AS OF SYSTEM TIME '-1µs'`,
		},
		{
			name: "after joins and before where",
			query: func(tx *gorm.DB) *gorm.DB {
				return tx.Clauses(AsOfSystemTimeNow()).
					Joins(`JOIN "companies" ON "companies"."id" = "users"."company_id"`).
					Where(`"users"."name" = ?`, "x").
					Find(&[]user{})
			},
			want: `SELECT "users"."id","users"."name","users"."company_id" FROM "users" JOIN "companies" ON "companies"."id" = "users"."company_id" AS OF SYSTEM TIME '-1µs' WHERE "users"."name" = $1`,
		},
		{
			name: "semicolons stripped",
			query: func(tx *gorm.DB) *gorm.DB {
				return tx.Clauses(AsOfSystemTime("'-1h'; DROP TABLE users")).Find(&[]user{})
			},
			want: `SELECT * FROM "users" AS OF SYSTEM TIME '-1h' DROP TABLE users`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := db.ToSQL(tc.query)
			if got != strings.ReplaceAll(tc.want, "$1", "'x'") {
				t.Fatalf("\nwant %s\n got %s", tc.want, got)
			}
		})
	}
}

// A statement reused across finishers must keep AS OF SYSTEM TIME on each build.
func TestAsOfSystemTimeSurvivesRebuild(t *testing.T) {
	db := dryRunDB(t, Open(dsn))
	q := db.Model(&user{}).Clauses(AsOfSystemTimeNow()).Where("name = ?", "x")

	var count int64
	if sql := q.Session(&gorm.Session{}).Count(&count).Statement.SQL.String(); !strings.Contains(sql, "AS OF SYSTEM TIME") {
		t.Fatalf("count missing AS OF SYSTEM TIME: %s", sql)
	}
	if sql := q.Session(&gorm.Session{}).Find(&[]user{}).Statement.SQL.String(); !strings.Contains(sql, "AS OF SYSTEM TIME") {
		t.Fatalf("find missing AS OF SYSTEM TIME: %s", sql)
	}
}

func TestAsOfSystemTimeIgnoredOnPostgres(t *testing.T) {
	db := dryRunDB(t, postgres.Open(dsn))
	got := db.ToSQL(func(tx *gorm.DB) *gorm.DB { return tx.Clauses(AsOfSystemTimeNow()).Find(&[]company{}) })
	if want := `SELECT * FROM "companies"`; got != want {
		t.Fatalf("want %s, got %s", want, got)
	}
}

func TestAsOfSystemTimeUnsupportedType(t *testing.T) {
	db := dryRunDB(t, Open(dsn))
	if err := db.Clauses(AsOfSystemTime(123)).Find(&[]user{}).Error; err == nil {
		t.Fatal("expected an error for an unsupported timestamp type")
	}
}

func TestNameStaysPostgres(t *testing.T) {
	if got := Open(dsn).Name(); got != "postgres" {
		t.Fatalf("want postgres, got %s", got)
	}
}
