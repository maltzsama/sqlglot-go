package sqlglot_test

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// transpileRecord is one cross-dialect case from upstream's
// validate_all(sql, read={...}, write={...}) pairs (tests/dialects/test_dialect.py:74,
// captured by scripts/extract_dialect_corpus.py --transpile-only): parse Sql under the
// Read dialect, generate with Pretty/Identify under the Write dialect, and expect the
// output to equal Want. "" is the base dialect on either side. A read pair contributes
// (read=<read_dialect>, write=<test dialect>, sql=<read_sql>, want=<main sql>); a write
// pair contributes (read=<test dialect>, write=<write_dialect>, sql=<main sql>,
// want=<write_sql>).
//
// JSON field names match the extractor contract:
// {"read":..,"write":..,"sql":..,"want":..,"pretty":..,"identify":..}.
type transpileRecord struct {
	Read  string `json:"read"`
	Write string `json:"write"`
	Sql   string `json:"sql"`
	Want  string `json:"want"`
	// Pretty/Identify mirror validate_all(pretty=..., identify=...): they apply to the
	// generated side only, exactly as upstream's .sql(..., pretty=pretty, identify=identify).
	Pretty bool `json:"pretty"`
	// Identify quotes every identifier on output.
	Identify bool `json:"identify,omitempty"`
}

// transpileGapKey identifies a tracked cross-dialect gap. Like gapKey (corpus_test.go) it
// intentionally excludes the category: re-labeling a gap never trips the subset check. It
// also excludes Pretty/Identify, so two records differing only in flags share one entry.
type transpileGapKey struct {
	Read  string
	Write string
	Sql   string
}

// transpileGapRecord is the on-disk shape of one line of testdata/transpile_gaps.txt.
type transpileGapRecord struct {
	Read     string `json:"read"`
	Write    string `json:"write"`
	Category string `json:"category"`
	Sql      string `json:"sql"`
}

const (
	transpileCorpusPath      = "testdata/transpile_corpus.jsonl"
	transpileGapsPath        = "testdata/transpile_gaps.txt"
	transpileCorrectionsPath = "testdata/transpile_corrections.jsonl"
)

// transpileCorrection overrides the upstream-generated Want for one corpus pair. It records
// documented real-engine fixes (DEVIATIONS.md §1.32) that intentionally differ from the
// pinned upstream, so the corpus still asserts the corrected output without hiding gaps.
type transpileCorrection struct {
	Read   string `json:"read"`
	Write  string `json:"write"`
	Sql    string `json:"sql"`
	Want   string `json:"want"`
	Reason string `json:"reason"`
}

// loadTranspileCorrections reads the committed correction overlay keyed by (read, write, sql).
func loadTranspileCorrections(t *testing.T) map[transpileGapKey]string {
	t.Helper()
	corrections := map[transpileGapKey]string{}
	data, err := os.ReadFile(transpileCorrectionsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return corrections
		}
		t.Fatalf("read %s: %v", transpileCorrectionsPath, err)
	}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		var rec transpileCorrection
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("parse %s line %q: %v", transpileCorrectionsPath, line, err)
		}
		key := transpileGapKey{Read: rec.Read, Write: rec.Write, Sql: rec.Sql}
		if _, dup := corrections[key]; dup {
			t.Fatalf("duplicate correction for [%s -> %s] %s", rec.Read, rec.Write, rec.Sql)
		}
		corrections[key] = rec.Want
	}
	return corrections
}

// applyTranspileCorrections replaces Want for the corrected pairs and fails closed when an
// overlay entry no longer matches a corpus record (a stale correction must not linger).
func applyTranspileCorrections(t *testing.T, records []transpileRecord, corrections map[transpileGapKey]string) []transpileRecord {
	t.Helper()
	seen := make(map[transpileGapKey]bool, len(records))
	for i := range records {
		key := transpileGapKey{Read: records[i].Read, Write: records[i].Write, Sql: records[i].Sql}
		seen[key] = true
		if want, ok := corrections[key]; ok {
			records[i].Want = want
		}
	}
	for key := range corrections {
		if !seen[key] {
			t.Errorf("stale transpile correction (no corpus record): [%s -> %s] %s", key.Read, key.Write, key.Sql)
		}
	}
	return records
}

// parseTranspileJSONL parses one transpileRecord per non-empty line of JSONL data. It is
// factored out of loadTranspileCorpus so the loader can be exercised directly without
// depending on the committed fixture.
func parseTranspileJSONL(data []byte) ([]transpileRecord, error) {
	var records []transpileRecord
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		var rec transpileRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return nil, fmt.Errorf("parse transpile corpus line %q: %w", line, err)
		}
		records = append(records, rec)
	}
	return records, nil
}

// loadTranspileCorpus reads testdata/transpile_corpus.jsonl, produced by
// scripts/extract_dialect_corpus.py --transpile-only. Unlike the scope-B identity corpus
// this file is committed as part of #33, so its absence is a hard failure: the harness
// must never silently pass by exercising zero cross-dialect cases.
func loadTranspileCorpus(t *testing.T) []transpileRecord {
	t.Helper()
	data, err := os.ReadFile(transpileCorpusPath)
	if err != nil {
		t.Fatalf("read transpile corpus: %v", err)
	}
	records, err := parseTranspileJSONL(data)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return records
}

// assertNoDuplicateTranspileRecords enforces the extractor's dedup invariant
// (transpile_corpus.jsonl is deduped on the full record). It hardens the record-count
// floors: without it, the floors could be satisfied by padding the corpus with duplicate
// lines while real pairs were dropped. transpileRecord is all comparable fields.
func assertNoDuplicateTranspileRecords(t *testing.T, records []transpileRecord) {
	t.Helper()
	seen := make(map[transpileRecord]struct{}, len(records))
	for _, rec := range records {
		if _, dup := seen[rec]; dup {
			t.Errorf("duplicate transpile corpus record (transpile_corpus.jsonl must be deduped): %+v", rec)
		}
		seen[rec] = struct{}{}
	}
}

// runTranspile parses rec.Sql under rec.Read and generates the result under rec.Write
// with rec.Pretty/Identify. A case passes iff both steps succeed and the output equals
// rec.Want; the individual errors are kept so categorizeTranspile can bucket the reason.
func runTranspile(rec transpileRecord) (got string, perr, gerr error) {
	expression, perr := sqlglot.ParseOne(rec.Sql, rec.Read)
	if perr != nil {
		return "", perr, nil
	}
	opts := generator.Options{Pretty: rec.Pretty, Identify: rec.Identify}
	got, gerr = sqlglot.Generate(expression, rec.Write, opts)
	return got, nil, gerr
}

// categorizeTranspile buckets a failing case for the transpile_gaps.txt worklist. It is
// coarse and deterministic; "parse:" rows are read-side gaps (the source dialect fails to
// parse), the "gen:" rows are write-side gaps (the target dialect's TRANSFORMS/
// TYPE_MAPPING are incomplete). Categories are not part of the subset-check identity
// (see transpileGapKey), so relabeling never breaks the test.
func categorizeTranspile(rec transpileRecord, got string, perr, gerr error) string {
	if perr != nil {
		return "parse: " + firstPhrase(perr.Error())
	}
	if gerr != nil {
		return "generate: error"
	}
	if strings.Contains(rec.Want, "CAST(") || strings.Contains(rec.Sql, "::") {
		return "gen: cast/type"
	}
	if strings.HasPrefix(rec.Sql, "CREATE ") || strings.HasPrefix(rec.Sql, "ALTER ") {
		return "gen: ddl/properties"
	}
	return "gen: transpile mismatch"
}

// loadTranspileGaps reads the committed worklist of known/expected cross-dialect failures.
// Only (read, write, sql) is used as the map key — the category column is metadata for
// humans, not part of the identity.
func loadTranspileGaps(t *testing.T) map[transpileGapKey]bool {
	t.Helper()
	gaps := map[transpileGapKey]bool{}
	data, err := os.ReadFile(transpileGapsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return gaps
		}
		t.Fatalf("read %s: %v", transpileGapsPath, err)
	}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		var rec transpileGapRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("parse %s line %q: %v", transpileGapsPath, line, err)
		}
		gaps[transpileGapKey{Read: rec.Read, Write: rec.Write, Sql: rec.Sql}] = true
	}
	return gaps
}

// writeTranspileGaps rewrites testdata/transpile_gaps.txt from the current failure set,
// sorted by (read, write, category, sql) for a stable, diffable file.
func writeTranspileGaps(fails map[transpileGapKey]string) error {
	entries := make([]transpileGapRecord, 0, len(fails))
	for k, category := range fails {
		entries = append(entries, transpileGapRecord{Read: k.Read, Write: k.Write, Category: category, Sql: k.Sql})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Read != entries[j].Read {
			return entries[i].Read < entries[j].Read
		}
		if entries[i].Write != entries[j].Write {
			return entries[i].Write < entries[j].Write
		}
		if entries[i].Category != entries[j].Category {
			return entries[i].Category < entries[j].Category
		}
		return entries[i].Sql < entries[j].Sql
	})
	var b strings.Builder
	// Encoder with HTML escaping disabled so operators like "->" and "<" stay literal,
	// keeping the file readable and line-diffable (same as writeGaps in corpus_test.go).
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return os.WriteFile(transpileGapsPath, []byte(b.String()), 0o644)
}

// Cross-dialect pass/total floors, bucketed by the WRITE dialect (the generation target,
// where the per-dialect generator TRANSFORMS/TYPE_MAPPING gaps live — issue #33 step 3).
// Read-side parse gaps surface under their target's bucket with a "parse:" category.
// Pass and total floors are monotonic: raise them, never lower them.
const (
	minPassTranspileBase        = 941
	minPassTranspileAthena      = 9
	minPassTranspileBigQuery    = 728
	minPassTranspileClickhouse  = 226
	minPassTranspileDatabricks  = 250
	minPassTranspileDoris       = 99
	minPassTranspileDremio      = 31
	minPassTranspileDuckDB      = 1428
	minPassTranspileHive        = 513
	minPassTranspileMaterialize = 32
	minPassTranspileMySQL       = 347
	minPassTranspileOracle      = 176
	minPassTranspilePostgres    = 425
	minPassTranspilePresto      = 597
	minPassTranspileRedshift    = 184
	minPassTranspileRisingWave  = 6
	minPassTranspileSnowflake   = 979
	minPassTranspileSpark       = 841
	minPassTranspileSpark2      = 87
	minPassTranspileSQLite      = 146
	minPassTranspileStarrocks   = 99
	minPassTranspileTeradata    = 74
	minPassTranspileTrino       = 137
	minPassTranspileTSQL        = 375
)

// Total floors catch dropped failing records that pass floors cannot detect.
const (
	minTotalTranspileBase        = 945
	minTotalTranspileAthena      = 11
	minTotalTranspileBigQuery    = 730
	minTotalTranspileClickhouse  = 241
	minTotalTranspileDatabricks  = 262
	minTotalTranspileDoris       = 104
	minTotalTranspileDremio      = 31
	minTotalTranspileDuckDB      = 1428
	minTotalTranspileHive        = 513
	minTotalTranspileMaterialize = 32
	minTotalTranspileMySQL       = 350
	minTotalTranspileOracle      = 190
	minTotalTranspilePostgres    = 445
	minTotalTranspilePresto      = 615
	minTotalTranspileRedshift    = 190
	minTotalTranspileRisingWave  = 6
	minTotalTranspileSnowflake   = 991
	minTotalTranspileSpark       = 841
	minTotalTranspileSpark2      = 89
	minTotalTranspileSQLite      = 153
	minTotalTranspileStarrocks   = 101
	minTotalTranspileTeradata    = 74
	minTotalTranspileTrino       = 150
	minTotalTranspileTSQL        = 389
)

// TestTranspileCorpus is the cross-dialect harness over upstream's validate_all read/write
// pairs (testdata/transpile_corpus.jsonl). It mirrors TestCorpus (corpus_test.go) with the
// same three checks, so the cross-dialect corpus can only grow and its gaps can only shrink:
//   - subset check: every failing pair must already be listed in
//     testdata/transpile_gaps.txt, so a new/untracked failure fails the build.
//   - per-write-dialect pass-floor ratchet: the pass count per target dialect must stay
//     >= its recorded minimum, so a regression can't be hidden by ALSO adding it to
//     transpile_gaps.txt (the pass count would still drop).
//   - per-write-dialect completeness floor: the number of cases exercised per target must
//     stay >= its recorded minimum, so dropping a *failing* case — which lowers neither the
//     pass count nor trips the subset check — still fails.
//
// Regenerate the gaps file and print the observed counts with:
//
//	SQLGLOT_TRANSPILE_UPDATE=1 go test . -run TestTranspileCorpus -v
func TestTranspileCorpus(t *testing.T) {
	records := loadTranspileCorpus(t)
	assertNoDuplicateTranspileRecords(t, records)
	records = applyTranspileCorrections(t, records, loadTranspileCorrections(t))

	total := map[string]int{}
	pass := map[string]int{}
	fails := map[transpileGapKey]string{}

	for _, rec := range records {
		total[rec.Write]++
		got, perr, gerr := runTranspile(rec)
		if perr == nil && gerr == nil && got == rec.Want {
			pass[rec.Write]++
			continue
		}
		fails[transpileGapKey{Read: rec.Read, Write: rec.Write, Sql: rec.Sql}] = categorizeTranspile(rec, got, perr, gerr)
	}

	floors := []struct {
		dialect  string
		minPass  int
		minTotal int
	}{
		{"", minPassTranspileBase, minTotalTranspileBase},
		{"athena", minPassTranspileAthena, minTotalTranspileAthena},
		{"bigquery", minPassTranspileBigQuery, minTotalTranspileBigQuery},
		{"clickhouse", minPassTranspileClickhouse, minTotalTranspileClickhouse},
		{"databricks", minPassTranspileDatabricks, minTotalTranspileDatabricks},
		{"doris", minPassTranspileDoris, minTotalTranspileDoris},
		{"dremio", minPassTranspileDremio, minTotalTranspileDremio},
		{"duckdb", minPassTranspileDuckDB, minTotalTranspileDuckDB},
		{"hive", minPassTranspileHive, minTotalTranspileHive},
		{"materialize", minPassTranspileMaterialize, minTotalTranspileMaterialize},
		{"mysql", minPassTranspileMySQL, minTotalTranspileMySQL},
		{"oracle", minPassTranspileOracle, minTotalTranspileOracle},
		{"postgres", minPassTranspilePostgres, minTotalTranspilePostgres},
		{"presto", minPassTranspilePresto, minTotalTranspilePresto},
		{"redshift", minPassTranspileRedshift, minTotalTranspileRedshift},
		{"risingwave", minPassTranspileRisingWave, minTotalTranspileRisingWave},
		{"snowflake", minPassTranspileSnowflake, minTotalTranspileSnowflake},
		{"spark", minPassTranspileSpark, minTotalTranspileSpark},
		{"spark2", minPassTranspileSpark2, minTotalTranspileSpark2},
		{"sqlite", minPassTranspileSQLite, minTotalTranspileSQLite},
		{"starrocks", minPassTranspileStarrocks, minTotalTranspileStarrocks},
		{"teradata", minPassTranspileTeradata, minTotalTranspileTeradata},
		{"trino", minPassTranspileTrino, minTotalTranspileTrino},
		{"tsql", minPassTranspileTSQL, minTotalTranspileTSQL},
	}
	var counts []string
	for _, floor := range floors {
		name := floor.dialect
		if name == "" {
			name = "base"
		}
		counts = append(counts, fmt.Sprintf("%s pass=%d/%d fail=%d", name,
			pass[floor.dialect], total[floor.dialect], total[floor.dialect]-pass[floor.dialect]))
	}

	if os.Getenv("SQLGLOT_TRANSPILE_UPDATE") == "1" {
		if err := writeTranspileGaps(fails); err != nil {
			t.Fatalf("write %s: %v", transpileGapsPath, err)
		}
		// Emit the observed pass counts on stderr (not t.Log): t.Log needs -v and
		// package-list mode buffers a passing test's output, so run:
		//   SQLGLOT_TRANSPILE_UPDATE=1 go test . -run TestTranspileCorpus -v
		fmt.Fprintf(os.Stderr,
			"updated %s: N=%d; %s\n", transpileGapsPath, len(records), strings.Join(counts, "; "))
		return
	}

	gaps := loadTranspileGaps(t)
	var untracked []string
	for k, category := range fails {
		if !gaps[k] {
			untracked = append(untracked, fmt.Sprintf("[%s -> %s] %s (%s)", k.Read, k.Write, k.Sql, category))
		}
	}
	if len(untracked) > 0 {
		sort.Strings(untracked)
		t.Errorf("untracked cross-dialect failures (fix them, or record as known gaps in %s with\n"+
			"  SQLGLOT_TRANSPILE_UPDATE=1 go test . -run TestTranspileCorpus -v\n):\n%s",
			transpileGapsPath, strings.Join(untracked, "\n"))
	}

	for _, floor := range floors {
		name := floor.dialect
		if name == "" {
			name = "base"
		}
		if pass[floor.dialect] < floor.minPass {
			t.Errorf("%s transpile pass regressed: pass=%d, want >= %d", name, pass[floor.dialect], floor.minPass)
		}
		if total[floor.dialect] < floor.minTotal {
			t.Errorf("%s transpile corpus shrank: %d records exercised, want >= %d (a corpus case was dropped)", name, total[floor.dialect], floor.minTotal)
		}
	}
	t.Logf("transpile totals: N=%d; %s", len(records), strings.Join(counts, "; "))
}

// TestParseTranspileJSONL validates the JSONL loader independently of the committed
// fixture: write a couple of inline records to a temp file and assert
// loadTranspileCorpus-style parsing round-trips them faithfully.
func TestParseTranspileJSONL(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/transpile_corpus.jsonl"
	content := "" +
		`{"read":"mysql","write":"tsql","sql":"SELECT 1","want":"SELECT 1","pretty":false}` + "\n" +
		`{"read":"hive","write":"presto","sql":"SELECT CAST(1 AS INT)","want":"SELECT 1","pretty":true,"identify":true}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp jsonl: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read temp jsonl: %v", err)
	}
	got, err := parseTranspileJSONL(data)
	if err != nil {
		t.Fatalf("parseTranspileJSONL: %v", err)
	}

	want := []transpileRecord{
		{Read: "mysql", Write: "tsql", Sql: "SELECT 1", Want: "SELECT 1", Pretty: false},
		{Read: "hive", Write: "presto", Sql: "SELECT CAST(1 AS INT)", Want: "SELECT 1", Pretty: true, Identify: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseTranspileJSONL = %#v, want %#v", got, want)
	}
}
