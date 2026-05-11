package backup

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sync/errgroup"
)

type TableJob struct {
	Source    string
	Dest      string
	PKColumn  string
	MinID     int64
	MaxID     int64
	OutputDir string
	Workers   int
	BatchSize int
}

func RunTableWorkers(ctx context.Context, db *sql.DB, d Dialect, job TableJob, logger *slog.Logger) error {
	if err := ValidateIdent(job.Source); err != nil {
		return err
	}
	if err := ValidateIdent(job.Dest); err != nil {
		return err
	}
	if err := ValidateIdent(job.PKColumn); err != nil {
		return err
	}
	if job.Workers < 1 {
		job.Workers = 1
	}
	if job.BatchSize < 1 {
		job.BatchSize = 1000
	}

	if err := os.MkdirAll(job.OutputDir, 0o755); err != nil {
		return fmt.Errorf("criar output-dir: %w", err)
	}

	totalRange := job.MaxID - job.MinID + 1
	if totalRange < 1 {
		logger.Info("tabela vazia, nada a exportar", "table", job.Source)
		return nil
	}
	step := totalRange / int64(job.Workers)
	if step < 1 {
		step = 1
	}

	g, gctx := errgroup.WithContext(ctx)
	for i := 0; i < job.Workers; i++ {
		workerID := i + 1
		inicio := job.MinID + int64(i)*step
		fim := inicio + step - 1
		if i == job.Workers-1 {
			fim = job.MaxID
		}
		if inicio > job.MaxID {
			break
		}

		g.Go(func() error {
			return exportRange(gctx, db, d, job, workerID, inicio, fim, logger)
		})
	}
	return g.Wait()
}

func exportRange(ctx context.Context, db *sql.DB, d Dialect, job TableJob, workerID int, inicio, fim int64, logger *slog.Logger) error {
	wlog := logger.With("table", job.Source, "worker_id", workerID, "range", fmt.Sprintf("%d-%d", inicio, fim))

	rows, err := db.QueryContext(ctx, d.SelectRangeSQL(job.Source, job.PKColumn), inicio, fim)
	if err != nil {
		return fmt.Errorf("worker %d: %w", workerID, err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("worker %d colunas: %w", workerID, err)
	}

	fileName := filepath.Join(job.OutputDir, fmt.Sprintf("%s_worker_%d.sql", job.Source, workerID))
	file, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("worker %d criar arquivo: %w", workerID, err)
	}
	defer file.Close()

	writer := bufio.NewWriterSize(file, 1<<20)
	defer func() { _ = writer.Flush() }()

	if _, err := writer.WriteString(d.Prelude()); err != nil {
		return err
	}

	insertPrefix := fmt.Sprintf("INSERT INTO %s (%s) VALUES\n",
		d.QuoteIdent(job.Dest), JoinCols(d, cols))

	vals := make([]interface{}, len(cols))
	valPtrs := make([]interface{}, len(cols))
	for i := range vals {
		valPtrs[i] = &vals[i]
	}

	batch := make([]string, 0, job.BatchSize)
	var rowCount int64

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if _, err := writer.WriteString(insertPrefix); err != nil {
			return err
		}
		if _, err := writer.WriteString(strings.Join(batch, ",\n")); err != nil {
			return err
		}
		if _, err := writer.WriteString(";\n"); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := rows.Scan(valPtrs...); err != nil {
			return fmt.Errorf("worker %d scan: %w", workerID, err)
		}
		valStrs := make([]string, len(vals))
		for i, val := range vals {
			valStrs[i] = formatValue(d, val)
		}
		batch = append(batch, "("+JoinVals(valStrs)+")")
		rowCount++

		if len(batch) >= job.BatchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("worker %d rows.Err: %w", workerID, err)
	}
	if err := flush(); err != nil {
		return err
	}
	if _, err := writer.WriteString(d.Postlude()); err != nil {
		return err
	}

	wlog.Info("worker concluído", "rows", rowCount, "file", fileName)
	return nil
}
