# 🛠️ GoSQLBackup

**GoSQLBackup** é uma ferramenta em Golang para exportação eficiente de dados de bancos MySQL/MariaDB em formato `.sql`, com foco em alta performance, uso de concorrência e escalabilidade para grandes volumes de dados.

---

## 🚀 Funcionalidades

- 🔁 Processamento paralelo com **Goroutines** para máxima performance.
- 🧩 Exportação de dados em múltiplos arquivos `.sql`, prontos para importação.
- 💾 Suporte a **batches** de `INSERT INTO`, otimizando o tempo de importação.
- 🛠️ Totalmente configurável: escolha a tabela, quantidade de workers e faixas de ID.
- ✅ Compatível com **MySQL** e **MariaDB**.
- 🧃 Ideal para backups, migrações ou análises externas.

---

## 📦 Como funciona

A ferramenta divide a tabela em blocos por ID e processa cada intervalo de forma paralela, exportando os dados em comandos `INSERT INTO` em arquivos `.sql`.

### Exemplo de exportação:

```bash
backup_worker_1.sql
backup_worker_2.sql
backup_worker_3.sql
...
```

Cada arquivo conterá:

```sql
INSERT INTO nova_tabela (col1, col2, col3, ...) VALUES
(1, 'abc', 'xyz'),
(2, 'def', 'uvw'),
...
```

---

## 💡 Exemplo de Caso Real (Case de Sucesso)

Durante os testes da ferramenta, foi realizada a exportação de uma tabela com aproximadamente **60 milhões de linhas**.

### 🔹 Configuração utilizada:
- **Workers**: 12
- **Batch de Inserts**: 3000
- **Tamanho por arquivo**: ~5 milhões de linhas
- **Total de arquivos**: 12 arquivos `.sql`

### 🔹 Resultado:
- Tempo médio de **exportação** de cada arquivo: ~20 segundos
- Tempo total de **inserção dos dados** no banco via **DBeaver**: **Aproximadamente 5 minutos**
- Todos os dados foram exportados e inseridos com sucesso, sem perda de integridade.

✅ **Conclusão**: A ferramenta se mostrou extremamente eficiente, escalando bem com grandes volumes e reduzindo drasticamente o tempo de backup e importação.

---

## 🧪 Requisitos

- Go 1.20 ou superior
- Banco MySQL/MariaDB
- Permissões de leitura no banco

---

## ⚙️ Como usar

```bash
go run main.go
```

Ou importe a função `RunBackupWorkers()` no seu projeto Go para uso programático.

---

## 📝 Contribuições

Pull Requests são bem-vindos! 💙  
Relate bugs, sugira melhorias ou abra issues.

---
