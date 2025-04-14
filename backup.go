package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"
	"strings"
)


func RunBackupWorkers(db *sql.DB, tabela string, workers int) error {
	var minID, maxID int
	
	err := db.QueryRow(fmt.Sprintf("SELECT MIN(id), MAX(id) FROM %s", tabela)).Scan(&minID, &maxID)
	if err != nil {
		return fmt.Errorf("falha ao obter IDs: %v", err)
	}



	intervalo := (maxID - minID + 1) / workers

	var wg sync.WaitGroup

	
	for i := 0; i < workers; i++ {
		inicio := minID + i*intervalo
		fim := inicio + intervalo - 1
		if i == workers-1 {
			fim = maxID
		}

		wg.Add(1)
		go func(workerID, inicio, fim int) {
			defer wg.Done()
			if err := exportaSQL(db, tabela, workerID, inicio, fim); err != nil {
				log.Printf("[Worker %d] Erro: %v", workerID, err)
			} else {
				log.Printf("[Worker %d] Concluído: %d a %d", workerID, inicio, fim)
			}
		}(i+1, inicio, fim)
	}
	wg.Wait()
	return nil
}


func exportaSQL(db *sql.DB, tabela string, workerID, inicio, fim int) error {
	query := fmt.Sprintf("SELECT * FROM %s WHERE id BETWEEN %d AND %d", tabela, inicio, fim)
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("erro na consulta: %v", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("erro ao obter colunas: %v", err)
	}


	file, err := os.Create(fmt.Sprintf("backup_worker_%d.sql", workerID))
	if err != nil {
		return fmt.Errorf("erro ao criar arquivo: %v", err)
	}
	defer file.Close()


	writer := bufio.NewWriter(file)
	defer writer.Flush()

	file.WriteString("SET NAMES utf8mb4;\n")
	file.WriteString("SET autocommit=0;\n")
	file.WriteString("START TRANSACTION;\n")
	file.WriteString("SET FOREIGN_KEY_CHECKS=0;\n\n")


	vals := make([]interface{}, len(cols))
	valPtrs := make([]interface{}, len(cols))
	for i := range vals {
		valPtrs[i] = &vals[i]
	}

	batchSize := 3000
	insertPrefix := fmt.Sprintf("INSERT INTO mdl_logstore_standard_log_bkp (%s) VALUES\n", joinCols(cols))
	batch := []string{}
	linhaCount := 0



	for rows.Next() {
		if err := rows.Scan(valPtrs...); err != nil {
			log.Printf("[Worker %d] Erro ao scanear linha: %v", workerID, err)
			continue
		}

		valStrs := make([]string, len(vals))
		for i, val := range vals {
			switch v := val.(type) {
			case nil:
				valStrs[i] = "NULL"
			case []byte:
				valStrs[i] = fmt.Sprintf("'%s'", escapeString(string(v)))
			case string:
				valStrs[i] = fmt.Sprintf("'%s'", escapeString(v))
			default:
				valStrs[i] = fmt.Sprintf("%v", v)
			}
		}
		batch = append(batch, fmt.Sprintf("(%s)", joinVals(valStrs)))
		linhaCount++

		if len(batch) >= batchSize {
			writer.WriteString(insertPrefix + strings.Join(batch, ",\n") + ";\n")
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		writer.WriteString(insertPrefix + strings.Join(batch, ",\n") + ";\n")
	}

	writer.WriteString("SET FOREIGN_KEY_CHECKS=1;\n")
	writer.WriteString("COMMIT;\n")
	log.Printf("[Worker %d] Exportou %d linhas", workerID, linhaCount)
	return nil
}

func joinCols(cols []string) string {
	return "`" + strings.Join(cols, "`, `") + "`"
}

func joinVals(vals []string) string {
	return strings.Join(vals, ", ")
}

func escapeString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\"", "\\\"") // opcional: se tiver aspas duplas
	return s
}