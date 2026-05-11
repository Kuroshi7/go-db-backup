package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"go-db-backup/internal/backup"
	"go-db-backup/internal/config"
)

func Open(ctx context.Context, cfg config.DBConfig, maxOpenConns int) (*sql.DB, backup.Dialect, error) {
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}

	d, err := backup.NewDialect(cfg.Driver)
	if err != nil {
		return nil, nil, err
	}

	conn, err := sql.Open(d.Driver(), d.DSN(cfg))
	if err != nil {
		return nil, nil, fmt.Errorf("sql.Open: %w", err)
	}

	if maxOpenConns < 1 {
		maxOpenConns = 1
	}
	conn.SetMaxOpenConns(maxOpenConns)
	conn.SetMaxIdleConns(maxOpenConns)
	conn.SetConnMaxLifetime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := conn.PingContext(pingCtx); err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("ping: %w", err)
	}

	return conn, d, nil
}
