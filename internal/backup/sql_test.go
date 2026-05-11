package backup

import "testing"

func TestValidateIdent(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"simples", "users", false},
		{"underscore", "mdl_logstore_standard_log", false},
		{"sublinha inicial", "_internal", false},
		{"alfanumerico", "tabela123", false},
		{"vazio", "", true},
		{"com espaco", "minha tabela", true},
		{"com ponto-virgula", "users; DROP TABLE users", true},
		{"com backtick", "users`", true},
		{"com hifen", "user-data", true},
		{"comeca com digito", "1users", true},
		{"acima de 64 chars", string(make([]byte, 65)), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateIdent(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateIdent(%q) erro = %v, want erro = %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

func TestMySQLQuoteIdent(t *testing.T) {
	d := mysqlDialect{}
	if got := d.QuoteIdent("foo"); got != "`foo`" {
		t.Errorf("MySQL QuoteIdent(foo) = %q", got)
	}
}

func TestPostgresQuoteIdent(t *testing.T) {
	d := postgresDialect{}
	if got := d.QuoteIdent("foo"); got != `"foo"` {
		t.Errorf("Postgres QuoteIdent(foo) = %q", got)
	}
	if got := d.QuoteIdent(`a"b`); got != `"a""b"` {
		t.Errorf("Postgres QuoteIdent quoteEscape = %q", got)
	}
}

func TestJoinColsMySQL(t *testing.T) {
	got := JoinCols(mysqlDialect{}, []string{"id", "nome", "email"})
	want := "`id`, `nome`, `email`"
	if got != want {
		t.Errorf("JoinCols mysql = %q, want %q", got, want)
	}
}

func TestJoinColsPostgres(t *testing.T) {
	got := JoinCols(postgresDialect{}, []string{"id", "nome"})
	want := `"id", "nome"`
	if got != want {
		t.Errorf("JoinCols pg = %q, want %q", got, want)
	}
}

func TestJoinVals(t *testing.T) {
	got := JoinVals([]string{"1", "'abc'", "NULL"})
	want := "1, 'abc', NULL"
	if got != want {
		t.Errorf("JoinVals = %q, want %q", got, want)
	}
}

func TestEscapeBackslashStyle(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"simples", "hello", "hello"},
		{"aspas simples", "it's", `it\'s`},
		{"barra invertida", `a\b`, `a\\b`},
		{"aspas duplas", `say "hi"`, `say \"hi\"`},
		{"newline", "line1\nline2", `line1\nline2`},
		{"carriage return", "line1\rline2", `line1\rline2`},
		{"tab", "a\tb", `a\tb`},
		{"null byte", "a\x00b", `a\0b`},
		{"misturado", "a'\\\nb", `a\'\\\nb`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := escapeBackslashStyle(tc.input)
			if got != tc.want {
				t.Errorf("escapeBackslashStyle(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestMySQLQuoteString(t *testing.T) {
	got := mysqlDialect{}.QuoteString("it's")
	want := `'it\'s'`
	if got != want {
		t.Errorf("mysql QuoteString = %q, want %q", got, want)
	}
}

func TestPostgresQuoteString(t *testing.T) {
	got := postgresDialect{}.QuoteString("it's")
	want := `E'it\'s'`
	if got != want {
		t.Errorf("postgres QuoteString = %q, want %q", got, want)
	}
}

func TestFormatBool(t *testing.T) {
	my := mysqlDialect{}
	if my.FormatBool(true) != "1" || my.FormatBool(false) != "0" {
		t.Error("mysql FormatBool inesperado")
	}
	pg := postgresDialect{}
	if pg.FormatBool(true) != "TRUE" || pg.FormatBool(false) != "FALSE" {
		t.Error("postgres FormatBool inesperado")
	}
}

func TestPlaceholder(t *testing.T) {
	my := mysqlDialect{}
	if my.Placeholder(1) != "?" || my.Placeholder(7) != "?" {
		t.Error("mysql Placeholder deveria ser ?")
	}
	pg := postgresDialect{}
	if pg.Placeholder(1) != "$1" || pg.Placeholder(7) != "$7" {
		t.Error("postgres Placeholder deveria ser $N")
	}
}

func TestIsIntegerType(t *testing.T) {
	mysql := mysqlDialect{}
	for _, ty := range []string{"tinyint", "smallint", "mediumint", "int", "integer", "bigint"} {
		if !mysql.IsIntegerType(ty) {
			t.Errorf("mysql: esperava %q como integer", ty)
		}
	}
	for _, ty := range []string{"varchar", "text", "decimal", "double"} {
		if mysql.IsIntegerType(ty) {
			t.Errorf("mysql: %q não deveria ser integer", ty)
		}
	}

	pg := postgresDialect{}
	for _, ty := range []string{"smallint", "integer", "bigint"} {
		if !pg.IsIntegerType(ty) {
			t.Errorf("pg: esperava %q como integer", ty)
		}
	}
	for _, ty := range []string{"tinyint", "mediumint", "int", "varchar", "text"} {
		if pg.IsIntegerType(ty) {
			t.Errorf("pg: %q não deveria ser integer", ty)
		}
	}
}
