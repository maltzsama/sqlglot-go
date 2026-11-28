package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
)

// Postgres `U&'...'` string literals are kept as written, as upstream does: they parse to a
// UnicodeString and round-trip unchanged (the pinned and latest upstream both do this). The
// `U&"..."` quoted identifiers are still decoded into the names they denote (DEVIATIONS §1.7),
// so a name-based authorization check sees the real identifier.

func genPG(t *testing.T, e exp.Expression) string {
	t.Helper()
	out, err := sqlglot.Generate(e, "postgres", generator.Options{})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return out
}

func TestPostgresUnicodeString(t *testing.T) {
	// Expected round-trips are the pinned oracle's output (upstream keeps the escape text).
	cases := []struct {
		sql     string
		wantRaw string
		wantSQL string
	}{
		{`SELECT U&'\0067\0072\0061\0064\0065'`, `\0067\0072\0061\0064\0065`, `SELECT U&'\0067\0072\0061\0064\0065'`},
		{`SELECT u&'\0067rade'`, `\0067rade`, `SELECT U&'\0067rade'`},
		{`SELECT U&'a''b\0063'`, `a'b\0063`, `SELECT U&'a''b\0063'`},
		{`SELECT U&'plain'`, "plain", "SELECT U&'plain'"},
		{`SELECT U&'\+01F600'`, `\+01F600`, `SELECT U&'\+01F600'`},
	}
	for _, tc := range cases {
		t.Run(tc.sql, func(t *testing.T) {
			e, err := sqlglot.ParseOne(tc.sql, "postgres")
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			lit := e.Expressions()[0]
			if lit.Kind() != exp.KindUnicodeString {
				t.Fatalf("expr = %v, want UnicodeString:\n%s", exp.ClassName(lit.Kind()), e.ToS())
			}
			if got := lit.Text("this"); got != tc.wantRaw {
				t.Fatalf("text = %q, want %q", got, tc.wantRaw)
			}
			if got := genPG(t, e); got != tc.wantSQL {
				t.Fatalf("round-trip = %q, want %q", got, tc.wantSQL)
			}
		})
	}
}

func TestPostgresUnicodeIdentifier(t *testing.T) {
	t.Run("schema identifier resolves to the real name", func(t *testing.T) {
		// The headline: a schema spelled with escapes must surface as its decoded name so a
		// name-based check cannot be bypassed.
		e, err := sqlglot.ParseOne(`SELECT count(*) FROM U&"inf\006Frmation_schema".tables`, "postgres")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		table := e.Arg("from_").(exp.Expression).Arg("this").(exp.Expression)
		schema := table.Arg("schema").(exp.Expression)
		if schema.Kind() != exp.KindIdentifier || schema.Arg("quoted") != true {
			t.Fatalf("schema = %v, want quoted Identifier:\n%s", exp.ClassName(schema.Kind()), e.ToS())
		}
		if got := schema.Text("this"); got != "information_schema" {
			t.Fatalf("decoded schema = %q, want information_schema", got)
		}
		if got := genPG(t, e); got != `SELECT COUNT(*) FROM "information_schema".tables` {
			t.Fatalf("round-trip = %q", got)
		}
	})

	t.Run("function name resolves to the real name", func(t *testing.T) {
		e, err := sqlglot.ParseOne(`SELECT U&"set_confi\0067"('search_path', 'x', false)`, "postgres")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		fn := e.Expressions()[0]
		name := fn.Arg("this").(exp.Expression)
		if got := name.Text("this"); got != "set_config" {
			t.Fatalf("decoded function name = %q, want set_config:\n%s", got, e.ToS())
		}
	})

	t.Run("quoted identifier preserves case", func(t *testing.T) {
		e, err := sqlglot.ParseOne(`SELECT U&"Mixed\0043ase"`, "postgres")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		col := e.Expressions()[0].Arg("this").(exp.Expression)
		if got := col.Text("this"); got != "MixedCase" {
			t.Fatalf("decoded identifier = %q, want MixedCase", got)
		}
	})
}

func TestPostgresUnicodeFailClosedAndNoRegression(t *testing.T) {
	// A custom `UESCAPE 'c'` clause is not decoded (the tokenizer assumes the default '\'),
	// so such forms must fail closed (parse error), never silently decode against the wrong
	// escape character.
	t.Run("custom UESCAPE round-trips as upstream", func(t *testing.T) {
		e, err := sqlglot.ParseOne(`SELECT U&'\0441' UESCAPE '!'`, "postgres")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if got := genPG(t, e); got != `SELECT U&'\0441' UESCAPE '!'` {
			t.Fatalf("round-trip = %q", got)
		}
	})

	// Identifier escapes PostgreSQL rejects must fail closed (parse error), so a name-based check
	// is never fooled by a garbage decoded identifier (DEVIATIONS §1.7).
	for _, sql := range []string{
		`SELECT U&"info\D800"`, // invalid escape in an identifier (identifier decoding is kept)
	} {
		t.Run("invalid escape fails closed/"+sql, func(t *testing.T) {
			if _, err := sqlglot.ParseOne(sql, "postgres"); err == nil {
				t.Fatalf("expected a parse error for %q, got none", sql)
			}
		})
	}

	// Spaced `U & 'x'` is genuine bitwise-AND of a column named U — the U& binding requires the
	// prefix to be immediately followed by the quote, matching Postgres.
	t.Run("spaced form stays bitwise", func(t *testing.T) {
		e, err := sqlglot.ParseOne(`SELECT U & 'x'`, "postgres")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if e.Expressions()[0].Kind() != exp.KindBitwiseAnd {
			t.Fatalf("expr = %v, want BitwiseAnd:\n%s", exp.ClassName(e.Expressions()[0].Kind()), e.ToS())
		}
	})

	// Only Postgres wires U&; base/MySQL have no such syntax, so U& stays split (unchanged).
	for _, dialect := range []string{"", "mysql"} {
		t.Run("non-postgres unaffected/"+dialect, func(t *testing.T) {
			e, err := sqlglot.ParseOne(`SELECT U&'\0067'`, dialect)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if e.Expressions()[0].Kind() != exp.KindBitwiseAnd {
				t.Fatalf("dialect %q expr = %v, want BitwiseAnd (no U& support):\n%s", dialect, exp.ClassName(e.Expressions()[0].Kind()), e.ToS())
			}
		})
	}
}
