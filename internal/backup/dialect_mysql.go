package backup

import (
	"context"
	"database/sql"
	"fmt"

	"go-db-backup/internal/config"
)

type mysqlDialect struct{}

func (mysqlDialect) Name() string                     { return config.DriverMySQL }
func (mysqlDialect) Driver() string                   { return "mysql" }
func (d mysqlDialect) DSN(cfg config.DBConfig) string { return cfg.DSN() }

func (mysqlDialect) QuoteIdent(s string) string { return "`" + s + "`" }

func (mysqlDialect) QuoteString(s string) string { return "'" + escapeBackslashStyle(s) + "'" }

func (mysqlDialect) FormatBool(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func (mysqlDialect) Placeholder(_ int) string { return "?" }

func (mysqlDialect) Prelude() string {
	return "SET NAMES utf8mb4;\n" +
		"SET autocommit=0;\n" +
		"START TRANSACTION;\n" +
		"SET FOREIGN_KEY_CHECKS=0;\n\n"
}

func (mysqlDialect) Postlude() string {
	return "\nSET FOREIGN_KEY_CHECKS=1;\nCOMMIT;\n"
}

func (mysqlDialect) ListTablesSQL() string {
	return `SELECT TABLE_NAME
	        FROM INFORMATION_SCHEMA.TABLES
	        WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE'
	        ORDER BY TABLE_NAME`
}

func (mysqlDialect) DescribePKSQL() string {
	return `SELECT k.COLUMN_NAME, c.DATA_TYPE
	        FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE k
	        JOIN INFORMATION_SCHEMA.COLUMNS c
	          ON c.TABLE_SCHEMA = k.TABLE_SCHEMA
	         AND c.TABLE_NAME = k.TABLE_NAME
	         AND c.COLUMN_NAME = k.COLUMN_NAME
	        WHERE k.TABLE_SCHEMA = ?
	          AND k.TABLE_NAME = ?
	          AND k.CONSTRAINT_NAME = 'PRIMARY'
	        ORDER BY k.ORDINAL_POSITION`
}

func (d mysqlDialect) MinMaxSQL(table, pk string) string {
	return fmt.Sprintf("SELECT MIN(%s), MAX(%s) FROM %s",
		d.QuoteIdent(pk), d.QuoteIdent(pk), d.QuoteIdent(table))
}

func (d mysqlDialect) SelectRangeSQL(table, pk string) string {
	return fmt.Sprintf("SELECT * FROM %s WHERE %s BETWEEN ? AND ?",
		d.QuoteIdent(table), d.QuoteIdent(pk))
}

func (mysqlDialect) IsIntegerType(t string) bool {
	switch t {
	case "tinyint", "smallint", "mediumint", "int", "integer", "bigint":
		return true
	}
	return false
}

func (d mysqlDialect) DumpCreateTable(ctx context.Context, db *sql.DB, _ config.DBConfig, table string) (string, error) {
	if err := ValidateIdent(table); err != nil {
		return "", err
	}
	query := fmt.Sprintf("SHOW CREATE TABLE %s", d.QuoteIdent(table))
	var name, createStmt string
	if err := db.QueryRowContext(ctx, query).Scan(&name, &createStmt); err != nil {
		return "", fmt.Errorf("SHOW CREATE TABLE %s: %w", table, err)
	}
	return createStmt + ";\n", nil
}
