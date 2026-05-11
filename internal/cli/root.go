package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"go-db-backup/internal/config"
)

var (
	flagEnvPath   string
	flagLogLevel  string
	flagLogFormat string
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "gosqlbackup",
		Short: "Exporta dados de MySQL/MariaDB em arquivos .sql de forma paralela",
		Long: `gosqlbackup é uma ferramenta de backup para MySQL/MariaDB que usa
worker pool para paralelizar a exportação por faixas de chave primária.

Suporta backup de uma tabela, várias tabelas ou do banco inteiro,
com filtros por padrão glob.`,
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVar(&flagEnvPath, "config", ".env", "caminho do arquivo .env")
	root.PersistentFlags().StringVar(&flagLogLevel, "log-level", "info", "nível de log (debug|info|warn|error)")
	root.PersistentFlags().StringVar(&flagLogFormat, "log-format", "text", "formato do log (text|json)")

	root.AddCommand(newBackupCmd())
	root.AddCommand(newListTablesCmd())
	root.AddCommand(newInitCmd())
	root.AddCommand(newVersionCmd())

	return root
}

func Execute() error {
	root := newRootCmd()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return root.ExecuteContext(ctx)
}

func loadEnv() error {
	return config.Load(flagEnvPath)
}

func buildLogger() *slog.Logger {
	level := parseLevel(flagLogLevel)
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if strings.EqualFold(flagLogFormat, "json") {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}
	return slog.New(handler)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func dbConfigFromFlags(cmd *cobra.Command) (config.DBConfig, error) {
	driver := config.NormalizeDriver(config.GetEnvString("DB_DRIVER", ""))
	if v, _ := cmd.Flags().GetString("db-driver"); v != "" {
		driver = config.NormalizeDriver(v)
	}

	defaultPort := 3306
	if driver == config.DriverPostgres {
		defaultPort = 5432
	}

	cfg := config.DBConfig{
		Driver:  driver,
		Host:    config.GetEnvString("DB_HOST", "localhost"),
		Port:    config.GetEnvInt("DB_PORT", defaultPort),
		User:    config.GetEnvString("DB_USER", ""),
		Pass:    config.GetEnvString("DB_PASS", ""),
		Name:    config.GetEnvString("DB_NAME", ""),
		Schema:  config.GetEnvString("DB_SCHEMA", ""),
		SSLMode: config.GetEnvString("DB_SSLMODE", ""),
	}

	if v, _ := cmd.Flags().GetString("db-host"); v != "" {
		cfg.Host = v
	}
	if cmd.Flags().Changed("db-port") {
		v, _ := cmd.Flags().GetInt("db-port")
		cfg.Port = v
	}
	if v, _ := cmd.Flags().GetString("db-user"); v != "" {
		cfg.User = v
	}
	if v, _ := cmd.Flags().GetString("db-pass"); v != "" {
		cfg.Pass = v
	}
	if v, _ := cmd.Flags().GetString("db-name"); v != "" {
		cfg.Name = v
	}
	if v, _ := cmd.Flags().GetString("db-schema"); v != "" {
		cfg.Schema = v
	}
	if v, _ := cmd.Flags().GetString("db-sslmode"); v != "" {
		cfg.SSLMode = v
	}

	if err := cfg.Validate(); err != nil {
		if promptErr := promptMissingCredentials(cmd, &cfg); promptErr != nil {
			return cfg, promptErr
		}
		if err := cfg.Validate(); err != nil {
			return cfg, fmt.Errorf("config de banco inválida: %w (dica: rode `gosqlbackup init`)", err)
		}
	}
	return cfg, nil
}

func addDBFlags(cmd *cobra.Command) {
	cmd.Flags().String("db-driver", "", "tipo do banco: mysql|postgres (env DB_DRIVER)")
	cmd.Flags().String("db-host", "", "host do banco (env DB_HOST)")
	cmd.Flags().Int("db-port", 0, "porta do banco (env DB_PORT; default 3306 mysql, 5432 postgres)")
	cmd.Flags().String("db-user", "", "usuário (env DB_USER)")
	cmd.Flags().String("db-pass", "", "senha (env DB_PASS)")
	cmd.Flags().String("db-name", "", "nome do banco (env DB_NAME)")
	cmd.Flags().String("db-schema", "", "schema (Postgres only; env DB_SCHEMA; default \"public\")")
	cmd.Flags().String("db-sslmode", "", "SSL mode Postgres (disable|require|prefer|verify-ca|verify-full; env DB_SSLMODE)")
}
