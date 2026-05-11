# GoSQLBackup

🇧🇷 Português  |  [🇬🇧 English](README.en.md)

Ferramenta CLI em Go para exportação paralela de dados de **MySQL/MariaDB** e **PostgreSQL** em arquivos `.sql`. Divide cada tabela em faixas de chave primária e processa cada faixa em um worker, gerando arquivos prontos para reimportação via `psql`, `mysql`, DBeaver ou similares.

## Funcionalidades

- Suporte a **MySQL/MariaDB** e **PostgreSQL** com a mesma interface (camada de dialeto interna).
- Backup de uma tabela, várias tabelas, banco inteiro ou por padrão glob.
- Worker pool por tabela, com paralelismo entre tabelas configurável.
- Configuração via `.env`, variáveis de ambiente e flags (precedência: flag > env > `.env`).
- Setup interativo (`gosqlbackup init`) que pergunta credenciais e grava `.env` com permissão `0600`.
- Detecção automática de chave primária inteira via `INFORMATION_SCHEMA`. Tabelas sem PK inteira são puladas com warning, não crasham.
- Dump opcional de `CREATE TABLE` (`--with-schema`): nativo no MySQL (`SHOW CREATE TABLE`), via `pg_dump` no Postgres.
- Cancelamento limpo via `SIGINT`/`SIGTERM` (arquivos parciais preservados para inspeção).
- Logs estruturados (`text` ou `json`).
- ~190k linhas/segundo sustentado em hardware comum (ver [Benchmarks](#benchmarks)).

## Requisitos

- Go 1.25+ (resolvido automaticamente pela toolchain)
- MySQL/MariaDB **ou** PostgreSQL acessível
- `pg_dump` no `PATH` apenas se for usar `--with-schema` em Postgres

## Instalação

```bash
git clone <repo>
cd go-db-backup
make build
# binário em ./bin/gosqlbackup
```

## Configuração

Você tem três opções:

### Opção 1 — Setup interativo (recomendado para primeira vez)

```bash
./bin/gosqlbackup init
```

A ferramenta pergunta:
1. Tipo de banco (`mysql` ou `postgres`)
2. Host, porta, usuário, senha (mascarada), nome do banco
3. Schema e SSL mode (somente Postgres)

Grava em `.env` com permissão `0600`. Use `--config /caminho/.env` para gravar em outro lugar e `--force` para sobrescrever sem perguntar.

### Opção 2 — Manual

```bash
cp .env.example .env
$EDITOR .env
```

### Opção 3 — Prompt no ato

Se rodar `backup` ou `list-tables` sem credenciais configuradas em um terminal interativo, a ferramenta pergunta na hora (sem salvar).

### Variáveis

| Variável | Default | Descrição |
|---|---|---|
| `DB_DRIVER` | — | `mysql` ou `postgres` (obrigatório) |
| `DB_HOST` | `localhost` | Host do banco |
| `DB_PORT` | `3306`/`5432` | Porta (default depende do driver) |
| `DB_USER` | — | Usuário (obrigatório) |
| `DB_PASS` | — | Senha |
| `DB_NAME` | — | Nome do banco (obrigatório) |
| `DB_SCHEMA` | `public` | Schema (Postgres only) |
| `DB_SSLMODE` | `disable` | `disable`/`require`/`prefer`/`verify-ca`/`verify-full` (Postgres only) |
| `WORKERS` | `6` | Workers por tabela |
| `BATCH_SIZE` | `3000` | Linhas por `INSERT` |
| `TABLE_CONCURRENCY` | `1` | Tabelas processadas em paralelo |
| `OUTPUT_DIR` | `./backups/<timestamp>` | Diretório de saída |
| `DEST_SUFFIX` | `_bkp` | Sufixo da tabela destino no `INSERT` |
| `LOG_LEVEL` | `info` | `debug`/`info`/`warn`/`error` |
| `LOG_FORMAT` | `text` | `text` ou `json` |

Todas têm equivalente em flag (ex.: `--db-driver`, `--db-host`, `--workers`).

## Uso

```bash
# Primeira vez: setup interativo
./bin/gosqlbackup init

# Listar tabelas (mostra PK detectada e elegibilidade)
./bin/gosqlbackup list-tables

# Tabela única
./bin/gosqlbackup backup --table users

# Várias tabelas
./bin/gosqlbackup backup --tables users,orders,products
./bin/gosqlbackup backup --table users --table orders

# Banco inteiro
./bin/gosqlbackup backup --all

# Filtro por padrão glob
./bin/gosqlbackup backup --pattern "mdl_*"

# All com exclusões
./bin/gosqlbackup backup --all --exclude "logs_*" --exclude "tmp_*"

# Com schema (CREATE TABLE)
./bin/gosqlbackup backup --table users --with-schema

# Performance tuning
./bin/gosqlbackup backup --all --workers 8 --batch-size 5000 --table-concurrency 3
```

### Estrutura da saída

```
backups/20260511-143012/
├── users_schema.sql          # (se --with-schema)
├── users_worker_1.sql
├── users_worker_2.sql
├── orders_worker_1.sql
└── ...
```

Cada arquivo de worker contém (MySQL):

```sql
SET NAMES utf8mb4;
SET autocommit=0;
START TRANSACTION;
SET FOREIGN_KEY_CHECKS=0;

INSERT INTO `users_bkp` (`id`, `nome`, `email`) VALUES
(1, 'abc', 'xyz'),
(2, 'def', 'uvw');

SET FOREIGN_KEY_CHECKS=1;
COMMIT;
```

Em Postgres:

```sql
BEGIN;
SET session_replication_role = replica;

INSERT INTO "users_bkp" ("id", "nome", "email") VALUES
(1, E'abc', E'xyz'),
(2, E'def', E'uvw');

SET session_replication_role = DEFAULT;
COMMIT;
```

> ⚠️ O `INSERT` aponta para `<source>_bkp` por padrão (configurável via `--dest-suffix`). Crie a tabela destino antes de importar, ou ajuste `--dest-suffix ""` para importar na mesma tabela.

### Reimportar

**MySQL/MariaDB:**
```bash
mysql -h host -u user -p banco < backups/20260511-143012/users_worker_1.sql
```

**PostgreSQL:**
```bash
psql -h host -U user -d banco -f backups/20260511-143012/users_worker_1.sql
```

> 💡 Para arquivos muito grandes (centenas de MB), use sempre `psql -f`/`mysql <`. Eles processam statement-por-statement; carregar tudo num único `EXEC` de cliente pode estourar memória do servidor.

## Como funciona

1. Resolve a lista de tabelas a partir das flags (`--table`, `--tables`, `--all`, `--pattern`, `--exclude`) e valida cada nome com regex `^[A-Za-z_][A-Za-z0-9_]*$`.
2. Para cada tabela, consulta `INFORMATION_SCHEMA` para detectar a PK. Tabelas com PK não-inteira simples (uuid, text, composta, sem PK) são **puladas com warning**.
3. Lê `MIN(pk)` e `MAX(pk)` e divide o intervalo em `--workers` faixas iguais.
4. Cada worker faz `SELECT * FROM <tabela> WHERE <pk> BETWEEN ? AND ?` no seu intervalo e escreve `INSERT`s em batches de `--batch-size`.
5. Erros propagam via `errgroup` e cancelam todos os workers (incluindo `SIGINT`/`SIGTERM`).

## Benchmarks

Teste com tabela sintética de 5M linhas, 8 colunas (int, uuid, text, numeric, jsonb, bool, text[], timestamp), 1.3 GB. PG 16 local em container, Linux desktop.

| Workers | Batch | Tempo | RSS pico | Linhas/s |
|---:|---:|---:|---:|---:|
| 4 | 3000 | **26.4s** | 42 MB | **189k** |
| 8 | 3000 | 30.0s | 73 MB | 167k |
| 12 | 3000 | 41.2s | 96 MB | 121k |
| 12 | 10000 | 30.6s | 157 MB | 163k |
| 4 | 10000 | **26.0s** | 61 MB | **192k** |

**Observações:**
- Sweet spot depende do hardware do DB e do cliente — comece com `workers=4-8` e ajuste.
- `--workers` acima do ótimo degrada por contenção (workers ficam esperando o DB).
- Memória escala linearmente com `batch-size × workers`; 5M linhas em 157 MB de RSS no pior caso.
- CPU 75-83% no melhor caso — gargalo é IO/serialização, não a lógica de paralelismo.

## Limitações conhecidas

- **PK obrigatória**: requer PK inteira simples (`tinyint`…`bigint` no MySQL; `smallint`/`integer`/`bigint` no Postgres). PKs compostas, `uuid` ou `text` são puladas com warning.
- **Distribuição por range**: IDs muito esparsos podem desbalancear workers (uns terminam antes dos outros).
- **Numeric em Postgres** sai quoted (`E'6.0000'`) — funciona via cast automático na importação mas é "feio".
- **`--with-schema` em Postgres** requer `pg_dump` no `PATH`.
- **bytea/jsonb** são exportados como string escapada — colunas binárias podem precisar tratamento manual.
- **Sem compressão** de saída (próximo PR).
- **Sem resume** de backup interrompido (cancelamento gera arquivos parciais; reiniciar refaz tudo).

## Stress test e validação de integridade

Comprovado em testes contra Postgres 16:

- **40 linhas** (`PERMISSION`, lookup): roundtrip via `EXCEPT` — **0 diferenças**.
- **31.500 linhas** (`SALES_ORDER`, tipos mistos incluindo `numeric` decimal): roundtrip via `row_to_json` — **byte-a-byte idêntico**.
- **5M linhas** (sintético com jsonb, uuid, array): export em 26s, **5M linhas exatas**, banco source **intacto**.
- **Cancelamento**: `SIGINT` retorna exit 130 em ~2s, arquivos parciais preservados, sem panic/goroutines vazadas.

## Desenvolvimento

```bash
make build         # compila para ./bin/gosqlbackup
make test          # roda testes unitários (funções puras + dialetos)
make lint          # golangci-lint (requer instalado)
make fmt           # gofmt + goimports
make clean         # remove bin/ e backups/
```

### Estrutura do projeto

```
go-db-backup/
├── cmd/gosqlbackup/main.go         # entrypoint
├── internal/
│   ├── cli/                        # cobra commands (root, backup, list, init, version)
│   ├── config/                     # .env + env + flags
│   ├── db/                         # abertura/pool MySQL + Postgres (pgx/stdlib)
│   └── backup/                     # núcleo
│       ├── dialect.go              # interface Dialect + helpers
│       ├── dialect_mysql.go        # impl MySQL
│       ├── dialect_postgres.go     # impl Postgres
│       ├── runner.go               # orquestra multi-tabela
│       ├── worker.go               # workers por tabela
│       ├── tables.go               # listagem, glob, PK detection
│       └── sql.go                  # ValidateIdent, JoinCols, JoinVals
├── .env.example
├── Makefile
└── .golangci.yml
```

## Contribuindo

PRs e issues são bem-vindos. Roadmap de melhorias:

- Compressão `.sql.gz` opcional
- Resume de backup interrompido
- Particionamento alternativo (hash) para PKs esparsas
- Suporte a tipos binários (bytea) com hex encoding
- Detectar `numeric` via `ColumnTypes()` para emitir sem aspas
- CI com testes de integração via testcontainers

## Licença

(definir)
