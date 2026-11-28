package generator_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/dialects"
	"github.com/maltzsama/sqlglot-go/generator"
)

// A projection that contains its own ORDER BY must not be used as the null-ordering key, or the
// generator recurses forever. The bare column is the key instead, as the pinned oracle renders it.
func TestNullOrderingSelfReferenceTerminates(t *testing.T) {
	out, err := sqlglot.Transpile("SELECT ARRAY_AGG(v ORDER BY v) AS v FROM t", dialects.DialectType("trino"), dialects.DialectType("mysql"), generator.Options{})
	if err != nil || len(out) != 1 {
		t.Fatalf("transpile: %v %v", out, err)
	}
	want := "SELECT GROUP_CONCAT(v ORDER BY CASE WHEN v IS NULL THEN 1 ELSE 0 END, v) AS v FROM t"
	if out[0] != want {
		t.Fatalf("got %q\nwant %q", out[0], want)
	}
}
