package backup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type TableInfo struct {
	Name     string
	PKColumn string
	PKType   string
	HasIntPK bool
}

func ListAllTables(ctx context.Context, db *sql.DB, d Dialect, schema string) ([]string, error) {
	rows, err := db.QueryContext(ctx, d.ListTablesSQL(), schema)
	if err != nil {
		return nil, fmt.Errorf("listar tabelas: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func DescribeTable(ctx context.Context, db *sql.DB, d Dialect, schema, tableName string) (TableInfo, error) {
	info := TableInfo{Name: tableName}

	rows, err := db.QueryContext(ctx, d.DescribePKSQL(), schema, tableName)
	if err != nil {
		return info, fmt.Errorf("describe %s: %w", tableName, err)
	}
	defer rows.Close()

	type col struct {
		Name string
		Type string
	}
	var cols []col
	for rows.Next() {
		var c col
		if err := rows.Scan(&c.Name, &c.Type); err != nil {
			return info, err
		}
		cols = append(cols, c)
	}
	if err := rows.Err(); err != nil {
		return info, err
	}

	if len(cols) != 1 {
		return info, nil
	}
	info.PKColumn = cols[0].Name
	info.PKType = strings.ToLower(cols[0].Type)
	info.HasIntPK = d.IsIntegerType(info.PKType)
	return info, nil
}

func ResolveTables(ctx context.Context, db *sql.DB, d Dialect, schema string, sel TableSelection) ([]string, error) {
	if err := sel.Validate(); err != nil {
		return nil, err
	}

	var candidates []string
	switch {
	case sel.All || sel.Pattern != "":
		all, err := ListAllTables(ctx, db, d, schema)
		if err != nil {
			return nil, err
		}
		candidates = all
	default:
		candidates = sel.Names
	}

	if err := validatePatterns(sel.Pattern, sel.Excludes); err != nil {
		return nil, err
	}
	candidates = applyFilters(candidates, sel.Pattern, sel.Excludes)

	for _, t := range candidates {
		if err := ValidateIdent(t); err != nil {
			return nil, fmt.Errorf("tabela %q rejeitada: %w", t, err)
		}
	}

	candidates = dedup(candidates)
	sort.Strings(candidates)

	if len(candidates) == 0 {
		return nil, errors.New("nenhuma tabela selecionada após aplicar filtros")
	}
	return candidates, nil
}

type TableSelection struct {
	Names    []string
	All      bool
	Pattern  string
	Excludes []string
}

func (s TableSelection) Validate() error {
	if !s.All && s.Pattern == "" && len(s.Names) == 0 {
		return errors.New("informe ao menos uma seleção: --table, --tables, --all ou --pattern")
	}
	return nil
}

func dedup(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func validatePatterns(pattern string, excludes []string) error {
	if pattern != "" {
		if _, err := filepath.Match(pattern, ""); err != nil {
			return fmt.Errorf("padrão inválido %q: %w", pattern, err)
		}
	}
	for _, ex := range excludes {
		if _, err := filepath.Match(ex, ""); err != nil {
			return fmt.Errorf("padrão de exclusão inválido %q: %w", ex, err)
		}
	}
	return nil
}

func applyFilters(tables []string, pattern string, excludes []string) []string {
	out := make([]string, 0, len(tables))
	for _, t := range tables {
		if pattern != "" {
			ok, _ := filepath.Match(pattern, t)
			if !ok {
				continue
			}
		}
		skip := false
		for _, ex := range excludes {
			ok, _ := filepath.Match(ex, t)
			if ok {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		out = append(out, t)
	}
	return out
}
