package backup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sync/errgroup"

	"go-db-backup/internal/config"
)

type RunConfig struct {
	DBConfig         config.DBConfig
	Selection        TableSelection
	Workers          int
	BatchSize        int
	TableConcurrency int
	OutputDir        string
	DestSuffix       string
	WithSchema       bool
}

type Runner struct {
	db      *sql.DB
	dialect Dialect
	logger  *slog.Logger
}

func NewRunner(db *sql.DB, d Dialect, logger *slog.Logger) *Runner {
	return &Runner{db: db, dialect: d, logger: logger}
}

type Result struct {
	Table   string
	Skipped bool
	Reason  string
	Rows    int64
	Elapsed time.Duration
	Err     error
}

func (r *Runner) Run(ctx context.Context, cfg RunConfig) ([]Result, error) {
	if cfg.Workers < 1 {
		cfg.Workers = 1
	}
	if cfg.BatchSize < 1 {
		cfg.BatchSize = 1000
	}
	if cfg.TableConcurrency < 1 {
		cfg.TableConcurrency = 1
	}
	if cfg.DestSuffix == "" {
		cfg.DestSuffix = "_bkp"
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = filepath.Join("backups", time.Now().Format("20060102-150405"))
	}

	schema := cfg.DBConfig.EffectiveSchema()

	tables, err := ResolveTables(ctx, r.db, r.dialect, schema, cfg.Selection)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("criar output-dir %q: %w", cfg.OutputDir, err)
	}
	r.logger.Info("backup iniciado",
		"driver", r.dialect.Name(),
		"tables", len(tables),
		"workers_per_table", cfg.Workers,
		"table_concurrency", cfg.TableConcurrency,
		"output_dir", cfg.OutputDir)

	results := make([]Result, len(tables))
	g, gctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, cfg.TableConcurrency)

	for i, t := range tables {
		i, t := i, t
		g.Go(func() error {
			select {
			case sem <- struct{}{}:
			case <-gctx.Done():
				return gctx.Err()
			}
			defer func() { <-sem }()

			res := r.processTable(gctx, t, cfg, schema)
			results[i] = res
			if res.Err != nil {
				return fmt.Errorf("tabela %s: %w", t, res.Err)
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return results, err
	}
	return results, nil
}

func (r *Runner) processTable(ctx context.Context, table string, cfg RunConfig, schema string) Result {
	start := time.Now()
	res := Result{Table: table}
	tlog := r.logger.With("table", table)

	info, err := DescribeTable(ctx, r.db, r.dialect, schema, table)
	if err != nil {
		res.Err = err
		return res
	}
	if !info.HasIntPK {
		reason := "tabela sem PK inteira simples — pulada"
		tlog.Warn(reason, "pk_column", info.PKColumn, "pk_type", info.PKType)
		res.Skipped = true
		res.Reason = reason
		res.Elapsed = time.Since(start)
		return res
	}

	minID, maxID, hasRows, err := minMaxPK(ctx, r.db, r.dialect, table, info.PKColumn)
	if err != nil {
		res.Err = err
		return res
	}
	if !hasRows {
		tlog.Info("tabela vazia, nada a exportar")
		res.Skipped = true
		res.Reason = "tabela vazia"
		res.Elapsed = time.Since(start)
		return res
	}

	if cfg.WithSchema {
		stmt, err := r.dialect.DumpCreateTable(ctx, r.db, cfg.DBConfig, table)
		if err != nil {
			res.Err = err
			return res
		}
		schemaFile := filepath.Join(cfg.OutputDir, table+"_schema.sql")
		if err := os.WriteFile(schemaFile, []byte(stmt), 0o644); err != nil {
			res.Err = fmt.Errorf("gravar schema %s: %w", schemaFile, err)
			return res
		}
		tlog.Info("schema exportado", "file", schemaFile)
	}

	job := TableJob{
		Source:    table,
		Dest:      table + cfg.DestSuffix,
		PKColumn:  info.PKColumn,
		MinID:     minID,
		MaxID:     maxID,
		OutputDir: cfg.OutputDir,
		Workers:   cfg.Workers,
		BatchSize: cfg.BatchSize,
	}
	if err := RunTableWorkers(ctx, r.db, r.dialect, job, r.logger); err != nil {
		res.Err = err
		return res
	}

	res.Elapsed = time.Since(start)
	tlog.Info("tabela concluída", "elapsed_ms", res.Elapsed.Milliseconds())
	return res
}

func minMaxPK(ctx context.Context, db *sql.DB, d Dialect, table, pk string) (int64, int64, bool, error) {
	if err := ValidateIdent(table); err != nil {
		return 0, 0, false, err
	}
	if err := ValidateIdent(pk); err != nil {
		return 0, 0, false, err
	}
	var minVal, maxVal sql.NullInt64
	if err := db.QueryRowContext(ctx, d.MinMaxSQL(table, pk)).Scan(&minVal, &maxVal); err != nil {
		return 0, 0, false, fmt.Errorf("min/max %s.%s: %w", table, pk, err)
	}
	if !minVal.Valid || !maxVal.Valid {
		return 0, 0, false, nil
	}
	return minVal.Int64, maxVal.Int64, true, nil
}

func IsContextCanceled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
