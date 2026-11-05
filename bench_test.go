package sqlglot_test

// Parse/generate micro-benchmarks. They are not part of the normal test run (only -bench
// executes them); they exist so future optimizer/dialect work can diff ns/op and allocs/op.

import (
	"testing"

	sqlglot "github.com/ridi-oss/sqlglot-go"
	"github.com/ridi-oss/sqlglot-go/generator"
)

const benchBase = "SELECT a, b, COUNT(*) FROM t1 JOIN t2 ON t1.id = t2.id WHERE a > 1 AND b <> 'x' GROUP BY a, b ORDER BY 1 LIMIT 10"
const benchCH = "SELECT a, uniq(x), countIf(y) FROM t FINAL PREWHERE a > 1 WHERE b = 'x' GROUP BY a ORDER BY a SETTINGS max_threads = 4"
const benchCreateSR = "CREATE TABLE t (c1 INT, c2 STRING) UNIQUE KEY (c1) PARTITION BY RANGE (`c2`) (FROM ('2000-01-01') TO ('2021-01-01') INTERVAL 1 YEAR) DISTRIBUTED BY HASH (c1) BUCKETS 10 PROPERTIES ('replication_num'='1')"
const benchOracle = "SELECT last_name, LEVEL FROM employees START WITH employee_id = 100 CONNECT BY PRIOR employee_id = manager_id ORDER SIBLINGS BY last_name"

func benchParseGenerate(b *testing.B, sql, dialect string) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		expr, err := sqlglot.ParseOne(sql, dialect)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := sqlglot.Generate(expr, dialect, generator.Options{}); err != nil {
			b.Fatal(err)
		}
	}
}

func benchParse(b *testing.B, sql, dialect string) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sqlglot.ParseOne(sql, dialect); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseGenerateBase(b *testing.B) { benchParseGenerate(b, benchBase, "") }
func BenchmarkParseBase(b *testing.B)         { benchParse(b, benchBase, "") }
func BenchmarkParseGenerateCH(b *testing.B)   { benchParseGenerate(b, benchCH, "clickhouse") }
func BenchmarkParseCreateSR(b *testing.B)     { benchParseGenerate(b, benchCreateSR, "starrocks") }
func BenchmarkParseOracle(b *testing.B)       { benchParseGenerate(b, benchOracle, "oracle") }
