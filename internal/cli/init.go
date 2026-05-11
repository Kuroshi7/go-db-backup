package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"go-db-backup/internal/config"
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Setup interativo: pergunta credenciais e grava em .env",
		Long: `Walkthrough interativo para configurar a ferramenta.

Pergunta host, porta, usuário, senha e banco, então grava no arquivo
indicado por --config (default: ./.env). Se o arquivo já existir, pergunta
antes de sobrescrever.`,
		RunE: runInit,
	}
	cmd.Flags().Bool("force", false, "sobrescreve o .env existente sem perguntar")
	return cmd
}

func runInit(cmd *cobra.Command, _ []string) error {
	out := cmd.OutOrStdout()
	in := bufio.NewReader(cmd.InOrStdin())

	target := flagEnvPath
	if target == "" {
		target = ".env"
	}

	if _, err := os.Stat(target); err == nil {
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Fprintf(out, "Arquivo %s já existe. Sobrescrever? [s/N]: ", target)
			ans, _ := in.ReadString('\n')
			ans = strings.ToLower(strings.TrimSpace(ans))
			if ans != "s" && ans != "sim" && ans != "y" && ans != "yes" {
				fmt.Fprintln(out, "abortado")
				return nil
			}
		}
	}

	fmt.Fprintln(out, "Configuração do GoSQLBackup")
	fmt.Fprintln(out, "Pressione Enter para aceitar o valor entre [].")
	fmt.Fprintln(out)

	driver := config.NormalizeDriver(promptLine(out, in, "Tipo do banco (mysql|postgres)", "mysql"))
	if driver != config.DriverMySQL && driver != config.DriverPostgres {
		return fmt.Errorf("tipo de banco inválido: %q (use mysql ou postgres)", driver)
	}

	defaultPort := "3306"
	defaultUser := "root"
	if driver == config.DriverPostgres {
		defaultPort = "5432"
		defaultUser = "postgres"
	}

	host := promptLine(out, in, "Host", "localhost")
	port := promptLine(out, in, "Porta", defaultPort)
	if _, err := strconv.Atoi(port); err != nil {
		return fmt.Errorf("porta inválida: %q", port)
	}
	user := promptLine(out, in, "Usuário", defaultUser)
	pass, err := promptSecret(out, in, "Senha")
	if err != nil {
		return err
	}
	dbName := promptLine(out, in, "Nome do banco", "")
	if dbName == "" {
		return fmt.Errorf("nome do banco é obrigatório")
	}

	var schema, sslmode string
	if driver == config.DriverPostgres {
		schema = promptLine(out, in, "Schema", "public")
		sslmode = promptLine(out, in, "SSL mode (disable|require|prefer)", "disable")
	}

	content := fmt.Sprintf(`# Gerado por gosqlbackup init
DB_DRIVER=%s
DB_HOST=%s
DB_PORT=%s
DB_USER=%s
DB_PASS=%s
DB_NAME=%s
`, driver, host, port, user, pass, dbName)
	if driver == config.DriverPostgres {
		content += fmt.Sprintf("DB_SCHEMA=%s\nDB_SSLMODE=%s\n", schema, sslmode)
	}

	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		return fmt.Errorf("gravar %s: %w", target, err)
	}
	fmt.Fprintf(out, "\n✓ Configuração gravada em %s (modo 0600)\n", target)
	fmt.Fprintln(out, "Próximo passo: ./bin/gosqlbackup list-tables")
	return nil
}

func promptLine(out io.Writer, in *bufio.Reader, label, def string) string {
	if def != "" {
		fmt.Fprintf(out, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(out, "%s: ", label)
	}
	line, _ := in.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func promptSecret(out io.Writer, in *bufio.Reader, label string) (string, error) {
	fmt.Fprintf(out, "%s: ", label)
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		s, err := in.ReadString('\n')
		if err != nil && s == "" {
			return "", err
		}
		return strings.TrimRight(s, "\r\n"), nil
	}
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func promptMissingCredentials(cmd *cobra.Command, cfg *config.DBConfig) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil
	}
	out := cmd.OutOrStderr()
	in := bufio.NewReader(os.Stdin)

	if cfg.Driver == "" {
		drv := config.NormalizeDriver(promptLine(out, in, "Tipo do banco (mysql|postgres)", "mysql"))
		if drv != config.DriverMySQL && drv != config.DriverPostgres {
			return fmt.Errorf("tipo de banco inválido: %q", drv)
		}
		cfg.Driver = drv
	}
	defaultPort := "3306"
	defaultUser := "root"
	if cfg.Driver == config.DriverPostgres {
		defaultPort = "5432"
		defaultUser = "postgres"
	}
	if cfg.Host == "" {
		cfg.Host = promptLine(out, in, "Host", "localhost")
	}
	if cfg.Port == 0 {
		v := promptLine(out, in, "Porta", defaultPort)
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("porta inválida: %q", v)
		}
		cfg.Port = n
	}
	if cfg.User == "" {
		cfg.User = promptLine(out, in, "Usuário", defaultUser)
	}
	if cfg.Pass == "" {
		p, err := promptSecret(out, in, "Senha")
		if err != nil {
			return err
		}
		cfg.Pass = p
	}
	if cfg.Name == "" {
		cfg.Name = promptLine(out, in, "Nome do banco", "")
	}
	if cfg.Driver == config.DriverPostgres {
		if cfg.Schema == "" {
			cfg.Schema = promptLine(out, in, "Schema", "public")
		}
		if cfg.SSLMode == "" {
			cfg.SSLMode = promptLine(out, in, "SSL mode (disable|require|prefer)", "disable")
		}
	}
	return nil
}
