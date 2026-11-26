package sqlglot_test

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestTranspileCorrectionsMatch pins every documented real-engine correction: the pair must
// transpile to the corrected Want in testdata/transpile_corrections.jsonl (DEVIATIONS §1.32
// and §1.33). TestTranspileCorpus applies the same overlay, so a stale entry fails both.
func TestTranspileCorrectionsMatch(t *testing.T) {
	f, err := os.Open("testdata/transpile_corrections.jsonl")
	if err != nil {
		t.Fatalf("open corrections: %v", err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec struct {
			Read, Write, Sql, Want string
		}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("parse correction %q: %v", line, err)
		}
		out, err := sqlglot.Transpile(rec.Sql, rec.Read, rec.Write, generator.Options{})
		if err != nil {
			t.Fatalf("[%s -> %s] %s: %v", rec.Read, rec.Write, rec.Sql, err)
		}
		if len(out) != 1 || out[0] != rec.Want {
			t.Fatalf("[%s -> %s] %s\n got: %v\nwant: %s", rec.Read, rec.Write, rec.Sql, out, rec.Want)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan corrections: %v", err)
	}
	if count == 0 {
		t.Fatal("no corrections exercised")
	}
}
