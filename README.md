# sqlglot-go

[![CI](https://github.com/maltzsama/sqlglot-go/actions/workflows/ci.yml/badge.svg)](https://github.com/maltzsama/sqlglot-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/maltzsama/sqlglot-go.svg)](https://pkg.go.dev/github.com/maltzsama/sqlglot-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/maltzsama/sqlglot-go)](https://goreportcard.com/report/github.com/maltzsama/sqlglot-go)
[![Release](https://img.shields.io/github/v/release/maltzsama/sqlglot-go)](https://github.com/maltzsama/sqlglot-go/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-yellow.svg)](./LICENSE)

sqlglot-go is a Go library for tokenizing SQL, parsing it into a mutable AST, generating SQL, and
transpiling between dialects. It ports those components, schema handling, and the SQL optimizer from
[tobymao/sqlglot](https://github.com/tobymao/sqlglot) v30.17.0. It requires Go 1.23 or later and uses
only the Go standard library.

## Getting started

```bash
go get github.com/maltzsama/sqlglot-go
```

Parse and render a PostgreSQL query, then translate a MySQL query to PostgreSQL.

```go
package main

import (
	"fmt"
	"log"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

func main() {
	expr, err := sqlglot.ParseOne("select id from users", "postgres")
	if err != nil {
		log.Fatal(err)
	}
	query, err := sqlglot.Generate(expr, "postgres", generator.Options{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(query) // SELECT id FROM users

	queries, err := sqlglot.Transpile(
		"SELECT IFNULL(total, 0) FROM orders", "mysql", "postgres", generator.Options{},
	)
	if err != nil {
		log.Fatal(err)
	}
	for _, query := range queries {
		fmt.Println(query) // SELECT COALESCE(total, 0) FROM orders
	}
}
```

## API guide

| Task | Entry point |
| --- | --- |
| Inspect raw SQL tokens | [`sqlglot.Tokenize`](https://pkg.go.dev/github.com/maltzsama/sqlglot-go#Tokenize) |
| Parse a query or scripts with multiple statements | [`sqlglot.ParseOne`](https://pkg.go.dev/github.com/maltzsama/sqlglot-go#ParseOne) for an AST, [`sqlglot.Parse`](https://pkg.go.dev/github.com/maltzsama/sqlglot-go#Parse) for separate statement ASTs |
| Parse a supported fragment as a specific AST kind | [`sqlglot.ParseInto`](https://pkg.go.dev/github.com/maltzsama/sqlglot-go#ParseInto) |
| Inspect or build an AST | [`expressions.Expression`, `Kind`, and builders](https://pkg.go.dev/github.com/maltzsama/sqlglot-go/expressions) |
| Render a tree or translate SQL | [`sqlglot.Generate`](https://pkg.go.dev/github.com/maltzsama/sqlglot-go#Generate) or [`sqlglot.Transpile`](https://pkg.go.dev/github.com/maltzsama/sqlglot-go#Transpile), with [generator options](https://pkg.go.dev/github.com/maltzsama/sqlglot-go/generator#Options) |
| Resolve columns or inspect scopes for lineage | [`optimizer.Qualify`](https://pkg.go.dev/github.com/maltzsama/sqlglot-go/optimizer#Qualify) and [`optimizer.TraverseScope`](https://pkg.go.dev/github.com/maltzsama/sqlglot-go/optimizer#TraverseScope), with column metadata from [`schema`](https://pkg.go.dev/github.com/maltzsama/sqlglot-go/schema) |
| Apply the default rewrite pipeline | [`optimizer.Optimize`](https://pkg.go.dev/github.com/maltzsama/sqlglot-go/optimizer#Optimize) |

Use `nil` or `""` for the base dialect, a registered name such as `"postgres"`, or a resolved
`*dialects.Dialect`. Some dialects accept settings in the name string, for example
`"mysql, mysql_version=8.0.35"` to enable eligible MySQL executable comments.

`ParseOne` returns a `KindBlock` when its input produces multiple statements. Use `Parse` to
process them separately. Some unsupported statements parse as a source-preserving `KindCommand`.
Check the node kind when your application needs a structured statement. `Tokenize` can also return
tokens for input that the parser rejects. Token offsets are inclusive rune indexes, and `Token.Text`
may contain a decoded value. See [`tokens.Token`](https://pkg.go.dev/github.com/maltzsama/sqlglot-go/tokens#Token)
for the source-span contract.

### Qualify columns and inspect scopes

Provide a schema when column resolution depends on table metadata, such as when expanding
`SELECT *` or resolving an unqualified column across joins. `Qualify` rewrites the supplied AST and
can panic on invalid schema or qualification failures. Pass a copy when the original AST is still
needed. `Generate` preserves its input tree.

```go
package main

import (
	"fmt"
	"log"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
	"github.com/maltzsama/sqlglot-go/optimizer"
	"github.com/maltzsama/sqlglot-go/schema"
)

func main() {
	expr, err := sqlglot.ParseOne("SELECT id FROM users", "postgres")
	if err != nil {
		log.Fatal(err)
	}

	opts := optimizer.DefaultQualifyOpts()
	opts.Dialect = "postgres"
	opts.Schema = schema.M("users", schema.M("id", "INT"))
	qualified := optimizer.Qualify(expr.Copy(), opts)

	query, err := sqlglot.Generate(qualified, "postgres", generator.Options{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(query) // SELECT "users"."id" AS "id" FROM "users" AS "users"

	for _, scope := range optimizer.TraverseScope(qualified) {
		fmt.Println(scope.SelectedSourceNames()) // [users]
	}
}
```

The scope API exposes sources, columns, CTEs, subqueries, and set operations for consumers that
build lineage analysis. The library also provides opt-in search-path qualification, source
resolution reports, and analysis scopes for top-level `UPDATE`, `DELETE`, and `MERGE`.
See [DEVIATIONS.md](./DEVIATIONS.md) for these extensions and engine-specific behavior.

## Dialects and verified coverage

The identity corpus checks parse and generation under the same dialect. It passes **7,363 of
7,609** cases. The transpile corpus checks expected output from upstream `validate_all` read/write
pairs. These figures describe the committed test corpora. Coverage varies by construct and dialect
pair.

| Dialect | Name | Identity cases passing |
| --- | --- | ---: |
| Base | `""` | 980/980 |
| Athena | `athena` | 51/51 |
| BigQuery | `bigquery` | 689/689 |
| ClickHouse | `clickhouse` | 468/468 |
| Databricks | `databricks` | 176/176 |
| Doris | `doris` | 56/56 |
| Dremio | `dremio` | 46/46 |
| DuckDB | `duckdb` | 565/565 |
| Hive | `hive` | 141/141 |
| ksqlDB | `ksqldb` (alias `ksql`) | 57/57 (project fixtures) |
| MariaDB | `mariadb` | Dedicated tests, no upstream identity corpus |
| Materialize | `materialize` | 24/24 |
| MySQL | `mysql` | 428/428 |
| Oracle | `oracle` | 249/249 |
| PostgreSQL | `postgres` | 468/468 |
| Presto | `presto` | 170/170 |
| Redshift | `redshift` | 171/171 |
| RisingWave | `risingwave` | 7/7 |
| Snowflake | `snowflake` | 1,297/1,543 |
| Spark | `spark` | 227/227 |
| Spark 2 | `spark2` | Covered by read/write pairs |
| SQLite | `sqlite` | 136/136 |
| StarRocks | `starrocks` | 145/145 |
| Teradata | `teradata` | 84/84 |
| Trino | `trino` | 115/115 |
| T-SQL | `tsql` | 613/613 |

The transpile corpus passes **6,635 of 8,881** pairs, with **2,246** remaining gaps. Of the
recorded pairs, 6,760 cross dialects.
See [identity gaps](./testdata/parity_gaps.txt) and [transpile gaps](./testdata/transpile_gaps.txt)
for the outstanding cases. The default optimizer pipeline passes its full `optimizer.sql`,
TPC-H, and TPC-DS suites. The standalone simplification fixtures now pass **521/521** cases;
standalone type annotation gaps remain in
[the annotation ledger](./optimizer/testdata/annotate_gaps.txt).

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for checks and PR conventions,
[AGENTS.md](./AGENTS.md) for the porting rules, and [ROADMAP.md](./ROADMAP.md) for current priorities.

The [root package](./sqlglot.go) ties together [tokenization](./tokens), [parsing](./parser),
[AST expressions](./expressions), and [generation](./generator). Dialect configuration lives in
[`dialects`](./dialects), with parser and generator overrides in their respective packages.
[`optimizer`](./optimizer) contains qualification, scope analysis, and rewrite rules.

```bash
go test ./...
go vet ./...
```

The checked-in fixtures cover normal development. Fetch the pinned Python source when porting
behavior, running the upstream extension tripwire, or regenerating a corpus.

```bash
scripts/fetch-reference.sh
go test -count=1 -run '^Test(Corpus|TranspileCorpus|Fidelity|UpstreamExtensionsTripwire)$' -v .
```

For a live comparison, set `PYTHONPATH=.reference/sqlglot-v30.17.0` and invoke Python's
`sqlglot.parse_one(...)`. To re-extract the transpile corpus and refresh its gap ledger, run
`PYTHONPATH=.reference/sqlglot-v30.17.0 python3 scripts/extract_dialect_corpus.py --transpile-only`
and then `SQLGLOT_TRANSPILE_UPDATE=1 go test . -run '^TestTranspileCorpus$' -v`.

## License and attribution

MIT, see [LICENSE](./LICENSE). sqlglot-go is an independent continuation of
[ridi-oss/sqlglot-go](https://github.com/ridi-oss/sqlglot-go), based on
[tobymao/sqlglot](https://github.com/tobymao/sqlglot) (© Toby Mao, MIT). See
[CHANGELOG.md](./CHANGELOG.md) for release history.
