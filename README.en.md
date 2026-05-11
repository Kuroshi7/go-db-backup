# GoSQLBackup

[🇧🇷 Português](README.md)  |  🇬🇧 English

A Go CLI tool for parallel data export from **MySQL/MariaDB** and **PostgreSQL** into `.sql` files. It splits each table into primary-key ranges and processes each range in a worker, producing files ready to re-import via `psql`, `mysql`, DBeaver or similar tools.

## Features

- Supports **MySQL/MariaDB** and **PostgreSQL** through the same interface (internal dialect layer).
- Back up a single table, multiple tables, the whole database, or by glob pattern.
- Worker pool per table, with configurable cross-table concurrency.
- Configuration via `.env`, environment variables and flags (precedence: flag > env > `.env`).
- Interactive setup (`gosqlbackup init`) that asks for credentials and writes `.env` with `0600` permission.
- Automatic integer primary key detection via `INFORMATION_SCHEMA`. Tables without an integer PK are skipped with a warning instead of crashing.
- Optional `CREATE TABLE` dump (`--with-schema`): native in MySQL (`SHOW CREATE TABLE`), via `pg_dump` in Postgres.
- Clean cancellation via `SIGINT`/`SIGTERM` (partial files preserved for inspection).
- Structured logs (`text` or `json`).
- Sustains ~190k rows/second on commodity hardware (see [Benchmarks](#benchmarks)).

## Requirements

- Go 1.25+ (resolved automatically by the toolchain)
- MySQL/MariaDB **or** PostgreSQL reachable
- `pg_dump` on `PATH` only if you want to use `--with-schema` on Postgres

## Install

```bash
git clone <repo>
cd go-db-backup
make build
# binary at ./bin/gosqlbackup
```

## Configuration

Three options:

### Option 1 — Interactive setup (recommended for first use)

```bash
./bin/gosqlbackup init
```

The tool asks:
1. Database type (`mysql` or `postgres`)
2. Host, port, user, password (masked), database name
3. Schema and SSL mode (Postgres only)

It writes to `.env` with `0600` permission. Use `--config /path/.env` to target a different file and `--force` to overwrite without prompting.

### Option 2 — Manual

```bash
cp .env.example .env
$EDITOR .env
```

### Option 3 — Prompt at runtime

If you run `backup` or `list-tables` without credentials in an interactive terminal, the tool will ask on the fly (no saving).

### Variables

| Variable | Default | Description |
|---|---|---|
| `DB_DRIVER` | — | `mysql` or `postgres` (required) |
| `DB_HOST` | `localhost` | DB host |
| `DB_PORT` | `3306`/`5432` | Port (default depends on driver) |
| `DB_USER` | — | User (required) |
| `DB_PASS` | — | Password |
| `DB_NAME` | — | Database name (required) |
| `DB_SCHEMA` | `public` | Schema (Postgres only) |
| `DB_SSLMODE` | `disable` | `disable`/`require`/`prefer`/`verify-ca`/`verify-full` (Postgres only) |
| `WORKERS` | `6` | Workers per table |
| `BATCH_SIZE` | `3000` | Rows per `INSERT` |
| `TABLE_CONCURRENCY` | `1` | Tables processed in parallel |
| `OUTPUT_DIR` | `./backups/<timestamp>` | Output directory |
| `DEST_SUFFIX` | `_bkp` | Suffix appended to destination table in `INSERT` |
| `LOG_LEVEL` | `info` | `debug`/`info`/`warn`/`error` |
| `LOG_FORMAT` | `text` | `text` or `json` |

Every variable has a matching flag (e.g. `--db-driver`, `--db-host`, `--workers`).

## Usage

```bash
# First time: interactive setup
./bin/gosqlbackup init

# List tables (shows detected PK and eligibility)
./bin/gosqlbackup list-tables

# Single table
./bin/gosqlbackup backup --table users

# Multiple tables
./bin/gosqlbackup backup --tables users,orders,products
./bin/gosqlbackup backup --table users --table orders

# Whole database
./bin/gosqlbackup backup --all

# Glob pattern filter
./bin/gosqlbackup backup --pattern "mdl_*"

# All with exclusions
./bin/gosqlbackup backup --all --exclude "logs_*" --exclude "tmp_*"

# With schema (CREATE TABLE)
./bin/gosqlbackup backup --table users --with-schema

# Performance tuning
./bin/gosqlbackup backup --all --workers 8 --batch-size 5000 --table-concurrency 3
```

### Output layout

```
backups/20260511-143012/
├── users_schema.sql          # (if --with-schema)
├── users_worker_1.sql
├── users_worker_2.sql
├── orders_worker_1.sql
└── ...
```

Each worker file contains (MySQL):

```sql
SET NAMES utf8mb4;
SET autocommit=0;
START TRANSACTION;
SET FOREIGN_KEY_CHECKS=0;

INSERT INTO `users_bkp` (`id`, `name`, `email`) VALUES
(1, 'abc', 'xyz'),
(2, 'def', 'uvw');

SET FOREIGN_KEY_CHECKS=1;
COMMIT;
```

PostgreSQL:

```sql
BEGIN;
SET session_replication_role = replica;

INSERT INTO "users_bkp" ("id", "name", "email") VALUES
(1, E'abc', E'xyz'),
(2, E'def', E'uvw');

SET session_replication_role = DEFAULT;
COMMIT;
```

> ⚠️ The `INSERT` targets `<source>_bkp` by default (configurable via `--dest-suffix`). Create the destination table beforehand, or pass `--dest-suffix ""` to insert back into the original table.

### Re-import

**MySQL/MariaDB:**
```bash
mysql -h host -u user -p dbname < backups/20260511-143012/users_worker_1.sql
```

**PostgreSQL:**
```bash
psql -h host -U user -d dbname -f backups/20260511-143012/users_worker_1.sql
```

> 💡 For very large files (hundreds of MB), always use `psql -f`/`mysql <`. They stream statements one-by-one; loading everything as a single client `EXEC` may exhaust server memory.

## How it works

1. Resolves the table list from flags (`--table`, `--tables`, `--all`, `--pattern`, `--exclude`) and validates each name against `^[A-Za-z_][A-Za-z0-9_]*$`.
2. For each table, queries `INFORMATION_SCHEMA` to detect the PK. Tables with non-simple-integer PK (uuid, text, composite, none) are **skipped with a warning**.
3. Reads `MIN(pk)` and `MAX(pk)`, then splits the range into `--workers` equal slices.
4. Each worker runs `SELECT * FROM <table> WHERE <pk> BETWEEN ? AND ?` for its slice and writes `INSERT`s in batches of `--batch-size`.
5. Errors propagate through `errgroup` and cancel every worker (including on `SIGINT`/`SIGTERM`).

## Benchmarks

Synthetic test against a 5M-row table with 8 mixed-type columns (int, uuid, text, numeric, jsonb, bool, text[], timestamp), 1.3 GB. PG 16 in a local container, Linux desktop.

| Workers | Batch | Time | Peak RSS | Rows/s |
|---:|---:|---:|---:|---:|
| 4 | 3000 | **26.4s** | 42 MB | **189k** |
| 8 | 3000 | 30.0s | 73 MB | 167k |
| 12 | 3000 | 41.2s | 96 MB | 121k |
| 12 | 10000 | 30.6s | 157 MB | 163k |
| 4 | 10000 | **26.0s** | 61 MB | **192k** |

**Takeaways:**
- Sweet spot depends on DB and client hardware — start with `workers=4-8` and tune.
- `--workers` above the optimum degrades due to DB contention (workers wait on the server).
- Memory scales linearly with `batch-size × workers`; 5M rows fit in 157 MB RSS in the worst case.
- CPU at 75-83% on best runs — the bottleneck is IO/serialization, not the parallelism logic.

## Known limitations

- **PK required**: needs a simple integer PK (`tinyint`…`bigint` on MySQL; `smallint`/`integer`/`bigint` on Postgres). Composite, `uuid` or `text` PKs are skipped with a warning.
- **Range-based distribution**: very sparse IDs may unbalance workers (some finish before others).
- **Postgres numeric** is emitted quoted (`E'6.0000'`) — works via automatic cast on import but is "ugly".
- **`--with-schema` on Postgres** requires `pg_dump` on `PATH`.
- **bytea/jsonb** are emitted as escaped strings — binary columns may need manual handling.
- **No output compression** (planned).
- **No resume** of interrupted backups (cancel leaves partial files; restart redoes everything).

## Stress test and integrity validation

Verified against PostgreSQL 16:

- **40 rows** (`PERMISSION` lookup): `EXCEPT` roundtrip — **0 differences**.
- **31,500 rows** (`SALES_ORDER`, mixed types including `numeric` decimal): `row_to_json` roundtrip — **byte-identical**.
- **5M rows** (synthetic with jsonb, uuid, array): exported in 26s, **5M rows exactly**, source DB **untouched**.
- **Cancellation**: `SIGINT` returns exit 130 in ~2s, partial files preserved, no panic / leaked goroutines.

## Development

```bash
make build         # compiles to ./bin/gosqlbackup
make test          # runs unit tests (pure functions + dialects)
make lint          # golangci-lint (must be installed)
make fmt           # gofmt + goimports
make clean         # removes bin/ and backups/
```

### Project layout

```
go-db-backup/
├── cmd/gosqlbackup/main.go         # entrypoint
├── internal/
│   ├── cli/                        # cobra commands (root, backup, list, init, version)
│   ├── config/                     # .env + env + flags
│   ├── db/                         # MySQL + Postgres pool open (pgx/stdlib)
│   └── backup/                     # core
│       ├── dialect.go              # Dialect interface + helpers
│       ├── dialect_mysql.go        # MySQL impl
│       ├── dialect_postgres.go     # Postgres impl
│       ├── runner.go               # multi-table orchestrator
│       ├── worker.go               # per-table workers
│       ├── tables.go               # listing, glob, PK detection
│       └── sql.go                  # ValidateIdent, JoinCols, JoinVals
├── .env.example
├── Makefile
└── .golangci.yml
```

## Contributing

PRs and issues welcome. Roadmap:

- Optional `.sql.gz` compression
- Resume from interrupted backups
- Alternative partitioning (hash) for sparse PKs
- Binary type support (bytea) with hex encoding
- Detect `numeric` via `ColumnTypes()` to emit unquoted
- CI with integration tests via testcontainers

## License

(TBD)
