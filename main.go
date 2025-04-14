package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	dbPass := "PASS_DB"
	dbUser := "USUARIO_SB"
	dbPort := "DB_PORT"
	dbHost := "HOST_DB"
	dbName := "NOME_DO_DB"


	dsn :=  fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", dbUser, dbPass, dbHost, dbPort, dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Erro ao conectar no banco %v", err)
	}
	defer db.Close()

	tabela := "mdl_logstore_standard_log"
	workers := 6

	start := time.Now()
	if err := RunBackupWorkers(db, tabela, workers); err != nil {
		log.Fatalf("Erro no backup: %v", err)
	}

	fmt.Println("Backup concluido em:", time.Since(start))
}