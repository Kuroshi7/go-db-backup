package backup

import (
	"fmt"
	"regexp"
	"strings"
)

var identRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func ValidateIdent(name string) error {
	if name == "" {
		return fmt.Errorf("identificador vazio")
	}
	if len(name) > 64 {
		return fmt.Errorf("identificador excede 64 caracteres: %q", name)
	}
	if !identRegex.MatchString(name) {
		return fmt.Errorf("identificador inválido: %q (esperado [A-Za-z_][A-Za-z0-9_]*)", name)
	}
	return nil
}

func JoinCols(d Dialect, cols []string) string {
	quoted := make([]string, len(cols))
	for i, c := range cols {
		quoted[i] = d.QuoteIdent(c)
	}
	return strings.Join(quoted, ", ")
}

func JoinVals(vals []string) string {
	return strings.Join(vals, ", ")
}
