package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const (
	DriverMySQL    = "mysql"
	DriverPostgres = "postgres"
)

type Config struct {
	DB      DBConfig
	Logging LoggingConfig
	Backup  BackupConfig
}

type DBConfig struct {
	Driver  string // "mysql" | "postgres"
	Host    string
	Port    int
	User    string
	Pass    string
	Name    string
	Schema  string // Postgres only (default "public"); ignorado em MySQL
	SSLMode string // Postgres only (disable|require|prefer|verify-ca|verify-full)
}

type LoggingConfig struct {
	Level  string
	Format string
}

type BackupConfig struct {
	Workers          int
	BatchSize        int
	TableConcurrency int
	OutputDir        string
	DestSuffix       string
	WithSchema       bool

	Tables   []string
	All      bool
	Pattern  string
	Excludes []string
}

func Load(envPath string) error {
	if envPath == "" {
		envPath = ".env"
	}
	if _, err := os.Stat(envPath); err == nil {
		if err := godotenv.Overload(envPath); err != nil {
			return fmt.Errorf("erro carregando %s: %w", envPath, err)
		}
	}
	return nil
}

func (c DBConfig) Validate() error {
	switch c.Driver {
	case DriverMySQL, DriverPostgres:
	case "":
		return errors.New("DB_DRIVER é obrigatório (mysql|postgres)")
	default:
		return fmt.Errorf("DB_DRIVER inválido: %q (esperado: mysql|postgres)", c.Driver)
	}
	if c.Host == "" {
		return errors.New("DB_HOST é obrigatório")
	}
	if c.User == "" {
		return errors.New("DB_USER é obrigatório")
	}
	if c.Name == "" {
		return errors.New("DB_NAME é obrigatório")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("DB_PORT inválido: %d", c.Port)
	}
	if c.Driver == DriverPostgres {
		switch c.SSLMode {
		case "", "disable", "require", "prefer", "allow", "verify-ca", "verify-full":
		default:
			return fmt.Errorf("DB_SSLMODE inválido: %q", c.SSLMode)
		}
	}
	return nil
}

func (c DBConfig) DSN() string {
	switch c.Driver {
	case DriverPostgres:
		schema := c.Schema
		if schema == "" {
			schema = "public"
		}
		ssl := c.SSLMode
		if ssl == "" {
			ssl = "disable"
		}
		u := url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(c.User, c.Pass),
			Host:   fmt.Sprintf("%s:%d", c.Host, c.Port),
			Path:   "/" + c.Name,
		}
		q := url.Values{}
		q.Set("sslmode", ssl)
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		return u.String()
	default:
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
			c.User, c.Pass, c.Host, c.Port, c.Name)
	}
}

func (c DBConfig) EffectiveSchema() string {
	if c.Driver == DriverPostgres {
		if c.Schema == "" {
			return "public"
		}
		return c.Schema
	}
	return c.Name
}

func (c DBConfig) DefaultPort() int {
	if c.Driver == DriverPostgres {
		return 5432
	}
	return 3306
}

func GetEnvString(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func GetEnvInt(key string, def int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func NormalizeDriver(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "postgres", "postgresql", "pg":
		return DriverPostgres
	case "mysql", "mariadb":
		return DriverMySQL
	}
	return s
}
