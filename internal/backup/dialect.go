package backup

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"go-db-backup/internal/config"
)

type Dialect interface {
	Name() string
	Driver() string
	DSN(cfg config.DBConfig) string

	QuoteIdent(s string) string
	QuoteString(s string) string
	FormatBool(b bool) string

	Placeholder(n int) string

	Prelude() string
	Postlude() string

	ListTablesSQL() string
	DescribePKSQL() string
	MinMaxSQL(table, pk string) string
	SelectRangeSQL(table, pk string) string

	IsIntegerType(t string) bool

	DumpCreateTable(ctx context.Context, db *sql.DB, cfg config.DBConfig, table string) (string, error)
}

func NewDialect(driver string) (Dialect, error) {
	switch driver {
	case config.DriverMySQL:
		return mysqlDialect{}, nil
	case config.DriverPostgres:
		return postgresDialect{}, nil
	default:
		return nil, fmt.Errorf("driver não suportado: %q", driver)
	}
}

func formatValue(d Dialect, val interface{}) string {
	switch v := val.(type) {
	case nil:
		return "NULL"
	case time.Time:
		return d.QuoteString(v.UTC().Format("2006-01-02 15:04:05.999999"))
	case []byte:
		return d.QuoteString(string(v))
	case string:
		return d.QuoteString(v)
	case bool:
		return d.FormatBool(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func escapeBackslashStyle(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case 0:
			b.WriteString(`\0`)
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
