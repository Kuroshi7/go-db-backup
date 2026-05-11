package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Exibe a versão do binário",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), Version)
		},
	}
}
