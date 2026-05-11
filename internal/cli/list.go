package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"go-db-backup/internal/backup"
	"go-db-backup/internal/db"
)

func newListTablesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-tables",
		Short: "Lista tabelas do banco com PK detectada",
		RunE:  runListTables,
	}
	addDBFlags(cmd)
	return cmd
}

func runListTables(cmd *cobra.Command, _ []string) error {
	if err := loadEnv(); err != nil {
		return err
	}
	logger := buildLogger()
	_ = logger

	dbCfg, err := dbConfigFromFlags(cmd)
	if err != nil {
		return err
	}

	conn, dialect, err := db.Open(cmd.Context(), dbCfg, 2)
	if err != nil {
		return err
	}
	defer conn.Close()

	schema := dbCfg.EffectiveSchema()
	tables, err := backup.ListAllTables(cmd.Context(), conn, dialect, schema)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TABLE\tPK_COLUMN\tPK_TYPE\tELEGIVEL")
	for _, t := range tables {
		info, err := backup.DescribeTable(cmd.Context(), conn, dialect, schema, t)
		if err != nil {
			fmt.Fprintf(tw, "%s\t-\t-\terro: %v\n", t, err)
			continue
		}
		pk := info.PKColumn
		if pk == "" {
			pk = "-"
		}
		pkt := info.PKType
		if pkt == "" {
			pkt = "-"
		}
		eleg := "não"
		if info.HasIntPK {
			eleg = "sim"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", t, pk, pkt, eleg)
	}
	return tw.Flush()
}
