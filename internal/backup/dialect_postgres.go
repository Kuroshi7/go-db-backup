package backup

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"go-db-backup/internal/config"
)

type postgresDialect struct{}

func (postgresDialect) Name() string                     { return config.DriverPostgres }
func (postgresDialect) Driver() string                   { return "pgx" }
func (d postgresDialect) DSN(cfg config.DBConfig) string { return cfg.DSN() }

func (postgresDialect) QuoteIdent(s string) string {
	return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
}

func (postgresDialect) QuoteString(s string) string {
	return "E'" + escapeBackslashStyle(s) + "'"
}

func (postgresDialect) FormatBool(b bool) string {
	if b {
		return "TRUE"
	}
	return "FALSE"
}

func (postgresDialect) Placeholder(n int) string {
	return "$" + strconv.Itoa(n)
}

func (postgresDialect) Prelude() string {
	return "BEGIN;\n" +
		"SET session_replication_role = replica;\n\n"
}

func (postgresDialect) Postlude() string {
	return "\nSET session_replication_role = DEFAULT;\nCOMMIT;\n"
}

func (postgresDialect) ListTablesSQL() string {
	return `SELECT table_name
	        FROM information_schema.tables
	        WHERE table_schema = $1 AND table_type = 'BASE TABLE'
	        ORDER BY table_name`
}

func (postgresDialect) DescribePKSQL() string {
	return `SELECT k.column_name, c.data_type
	        FROM information_schema.table_constraints t
	        JOIN information_schema.key_column_usage k
	          ON k.constraint_name = t.constraint_name
	         AND k.table_schema = t.table_schema
	         AND k.table_name = t.table_name
	        JOIN information_schema.columns c
	          ON c.table_schema = k.table_schema
	         AND c.table_name = k.table_name
	         AND c.column_name = k.column_name
	        WHERE t.constraint_type = 'PRIMARY KEY'
	          AND t.table_schema = $1
	          AND t.table_name = $2
	        ORDER BY k.ordinal_position`
}

func (d postgresDialect) MinMaxSQL(table, pk string) string {
	return fmt.Sprintf("SELECT MIN(%s), MAX(%s) FROM %s",
		d.QuoteIdent(pk), d.QuoteIdent(pk), d.QuoteIdent(table))
}

func (d postgresDialect) SelectRangeSQL(table, pk string) string {
	return fmt.Sprintf("SELECT * FROM %s WHERE %s BETWEEN $1 AND $2",
		d.QuoteIdent(table), d.QuoteIdent(pk))
}

func (postgresDialect) IsIntegerType(t string) bool {
	switch t {
	case "smallint", "integer", "bigint":
		return true
	}
	return false
}

func (postgresDialect) DumpCreateTable(ctx context.Context, _ *sql.DB, cfg config.DBConfig, table string) (string, error) {
	if err := ValidateIdent(table); err != nil {
		return "", err
	}
	schema := cfg.EffectiveSchema()
	if err := ValidateIdent(schema); err != nil {
		return "", err
	}
	bin, err := exec.LookPath("pg_dump")
	if err != nil {
		return "", fmt.Errorf("--with-schema em Postgres requer pg_dump no PATH: %w", err)
	}
	args := []string{
		"--schema-only",
		"--no-owner",
		"--no-privileges",
		"--table", schema + "." + table,
		"--host", cfg.Host,
		"--port", strconv.Itoa(cfg.Port),
		"--username", cfg.User,
		"--dbname", cfg.Name,
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = append(cmd.Environ(), "PGPASSWORD="+cfg.Pass)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pg_dump %s.%s: %w (%s)", schema, table, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
