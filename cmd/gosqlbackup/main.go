package main

import (
	"fmt"
	"os"

	"go-db-backup/internal/backup"
	"go-db-backup/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		if backup.IsContextCanceled(err) {
			fmt.Fprintln(os.Stderr, "interrompido")
			os.Exit(130)
		}
		os.Exit(1)
	}
}
