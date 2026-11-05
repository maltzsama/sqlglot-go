package dialects_test

import (
	"testing"

	"github.com/ridi-oss/sqlglot-go/dialects"
)

// TestDialectResolutionIsIsolated pins the dialect-template cache contract: resolving a
// settings variant must not leak state into the cached template or into later plain
// resolutions of the same dialect.
func TestDialectResolutionIsIsolated(t *testing.T) {
	// Resolve the ANSI_QUOTES variant first, then the plain dialect.
	ansi, err := dialects.GetOrRaise("mysql, mysql_ansi_quotes=true")
	if err != nil {
		t.Fatalf("GetOrRaise ansi: %v", err)
	}
	plain, err := dialects.GetOrRaise("mysql")
	if err != nil {
		t.Fatalf("GetOrRaise plain: %v", err)
	}
	if ansi.MySQLAnsiQuotes == nil || !*ansi.MySQLAnsiQuotes {
		t.Fatal("ansi variant did not set MySQLAnsiQuotes")
	}
	if plain.MySQLAnsiQuotes != nil {
		t.Fatal("plain mysql inherited MySQLAnsiQuotes from the settings variant")
	}
	// In the plain dialect '"' is still a string quote; under ANSI_QUOTES it is an identifier
	// delimiter. Tokenizing the same input must therefore differ.
	plainToks, err := plain.NewTokenizer().Tokenize(`SELECT "x"`)
	if err != nil {
		t.Fatalf("plain tokenize: %v", err)
	}
	ansiToks, err := ansi.NewTokenizer().Tokenize(`SELECT "x"`)
	if err != nil {
		t.Fatalf("ansi tokenize: %v", err)
	}
	if len(plainToks) == 0 || len(ansiToks) == 0 {
		t.Fatal("unexpected empty token stream")
	}
	plainStr := plainToks[len(plainToks)-1].TokenType.String()
	ansiStr := ansiToks[len(ansiToks)-1].TokenType.String()
	if plainStr == ansiStr {
		t.Fatalf(`"x" tokenized identically in both modes (%s); the cached tokenizer config leaked`, plainStr)
	}
}

// TestDialectTemplatesAreSharedButCopied checks that two resolutions return distinct
// pointers (so field writes are isolated) while the dialect name stays right.
func TestDialectTemplatesAreSharedButCopied(t *testing.T) {
	a, err := dialects.GetOrRaise("postgres")
	if err != nil {
		t.Fatalf("GetOrRaise: %v", err)
	}
	b, err := dialects.GetOrRaise("postgres")
	if err != nil {
		t.Fatalf("GetOrRaise: %v", err)
	}
	if a == b {
		t.Fatal("expected distinct dialect pointers per resolution")
	}
	if a.Name != "postgres" || b.Name != "postgres" {
		t.Fatalf("names = %q/%q, want postgres", a.Name, b.Name)
	}
	// A field write on one copy must not affect the other.
	original := b.NullOrdering
	a.NullOrdering = "test-only"
	if b.NullOrdering != original {
		t.Fatal("field write leaked between resolved copies")
	}
}

// TestGetOrRaiseSharedReusesTemplate pins the read-only resolution used by the parse and
// generate entry points: bare names return the cached pointer, while settings variants still
// get an isolated copy because they mutate fields.
func TestGetOrRaiseSharedReusesTemplate(t *testing.T) {
	first, err := dialects.GetOrRaiseShared("postgres")
	if err != nil {
		t.Fatalf("GetOrRaiseShared: %v", err)
	}
	second, err := dialects.GetOrRaiseShared("postgres")
	if err != nil {
		t.Fatalf("GetOrRaiseShared: %v", err)
	}
	if first != second {
		t.Fatal("GetOrRaiseShared should return the shared template pointer")
	}
	if plain, err := dialects.GetOrRaise("postgres"); err != nil {
		t.Fatalf("GetOrRaise: %v", err)
	} else if plain == first {
		t.Fatal("GetOrRaise must keep returning an isolated copy")
	}

	sharedVariant, err := dialects.GetOrRaiseShared("mysql, mysql_ansi_quotes=true")
	if err != nil {
		t.Fatalf("GetOrRaiseShared settings variant: %v", err)
	}
	sharedPlain, err := dialects.GetOrRaiseShared("mysql")
	if err != nil {
		t.Fatalf("GetOrRaiseShared plain: %v", err)
	}
	if sharedVariant == sharedPlain {
		t.Fatal("settings variant must not share the bare template")
	}
	if sharedVariant.MySQLAnsiQuotes == nil || !*sharedVariant.MySQLAnsiQuotes {
		t.Fatal("settings variant did not set MySQLAnsiQuotes")
	}
	if sharedPlain.MySQLAnsiQuotes != nil {
		t.Fatal("plain mysql inherited MySQLAnsiQuotes from the settings variant")
	}
}
