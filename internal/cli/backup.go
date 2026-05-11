package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"go-db-backup/internal/backup"
	"go-db-backup/internal/config"
	"go-db-backup/internal/db"
)

func newBackupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Exporta dados de tabelas para arquivos .sql",
		Long: `Exporta dados de uma, várias ou todas as tabelas para arquivos .sql.

Cada tabela é dividida em N faixas de chave primária e processada por
workers em paralelo. Tabelas sem PK inteira simples são puladas com warning.`,
		Example: `  gosqlbackup backup --table users
  gosqlbackup backup --tables users,orders,products
  gosqlbackup backup --all --exclude "logs_*"
  gosqlbackup backup --pattern "mdl_*" --with-schema --workers 12`,
		RunE: runBackup,
	}

	addDBFlags(cmd)

	cmd.Flags().StringSlice("table", nil, "tabela(s) a exportar (flag repetível)")
	cmd.Flags().String("tables", "", "lista de tabelas separadas por vírgula")
	cmd.Flags().Bool("all", false, "exportar todas as tabelas do banco")
	cmd.Flags().String("pattern", "", "padrão glob de inclusão (ex.: \"mdl_*\")")
	cmd.Flags().StringSlice("exclude", nil, "padrão glob de exclusão (flag repetível)")

	cmd.Flags().Int("workers", config.GetEnvInt("WORKERS", 6), "workers por tabela")
	cmd.Flags().Int("batch-size", config.GetEnvInt("BATCH_SIZE", 3000), "linhas por INSERT")
	cmd.Flags().Int("table-concurrency", config.GetEnvInt("TABLE_CONCURRENCY", 1), "quantas tabelas processar em paralelo")
	cmd.Flags().String("output-dir", config.GetEnvString("OUTPUT_DIR", ""), "diretório de saída (default: ./backups/<timestamp>)")
	cmd.Flags().String("dest-suffix", config.GetEnvString("DEST_SUFFIX", "_bkp"), "sufixo do nome da tabela destino no INSERT")
	cmd.Flags().Bool("with-schema", false, "prepende SHOW CREATE TABLE em um arquivo *_schema.sql por tabela")

	return cmd
}

func runBackup(cmd *cobra.Command, _ []string) error {
	if err := loadEnv(); err != nil {
		return err
	}
	logger := buildLogger()

	dbCfg, err := dbConfigFromFlags(cmd)
	if err != nil {
		return err
	}

	sel, err := selectionFromFlags(cmd)
	if err != nil {
		return err
	}

	workers, _ := cmd.Flags().GetInt("workers")
	batchSize, _ := cmd.Flags().GetInt("batch-size")
	tableConc, _ := cmd.Flags().GetInt("table-concurrency")
	outputDir, _ := cmd.Flags().GetString("output-dir")
	destSuffix, _ := cmd.Flags().GetString("dest-suffix")
	withSchema, _ := cmd.Flags().GetBool("with-schema")

	maxConns := workers*tableConc + 2
	conn, dialect, err := db.Open(cmd.Context(), dbCfg, maxConns)
	if err != nil {
		return err
	}
	defer conn.Close()

	runner := backup.NewRunner(conn, dialect, logger)
	results, err := runner.Run(cmd.Context(), backup.RunConfig{
		DBConfig:         dbCfg,
		Selection:        sel,
		Workers:          workers,
		BatchSize:        batchSize,
		TableConcurrency: tableConc,
		OutputDir:        outputDir,
		DestSuffix:       destSuffix,
		WithSchema:       withSchema,
	})

	printSummary(cmd, results)
	if err != nil {
		if backup.IsContextCanceled(err) {
			logger.Warn("backup interrompido", "err", err)
			return err
		}
		return err
	}
	return nil
}

func selectionFromFlags(cmd *cobra.Command) (backup.TableSelection, error) {
	var sel backup.TableSelection
	sel.All, _ = cmd.Flags().GetBool("all")
	sel.Pattern, _ = cmd.Flags().GetString("pattern")
	sel.Excludes, _ = cmd.Flags().GetStringSlice("exclude")

	repeated, _ := cmd.Flags().GetStringSlice("table")
	csv, _ := cmd.Flags().GetString("tables")
	names := append([]string{}, repeated...)
	if csv != "" {
		for _, p := range strings.Split(csv, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				names = append(names, p)
			}
		}
	}
	sel.Names = names

	if err := sel.Validate(); err != nil {
		return sel, err
	}
	return sel, nil
}

func printSummary(cmd *cobra.Command, results []backup.Result) {
	out := cmd.OutOrStdout()
	if len(results) == 0 {
		return
	}
	fmt.Fprintln(out, "\nResumo:")
	for _, r := range results {
		switch {
		case r.Err != nil:
			fmt.Fprintf(out, "  ✗ %s: erro: %v\n", r.Table, r.Err)
		case r.Skipped:
			fmt.Fprintf(out, "  - %s: pulada (%s)\n", r.Table, r.Reason)
		default:
			fmt.Fprintf(out, "  ✓ %s: ok em %s\n", r.Table, r.Elapsed.Round(1e6))
		}
	}
}
