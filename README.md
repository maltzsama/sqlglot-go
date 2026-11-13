# sqlglot-go

[![CI](https://github.com/maltzsama/sqlglot-go/actions/workflows/ci.yml/badge.svg)](https://github.com/maltzsama/sqlglot-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/maltzsama/sqlglot-go.svg)](https://pkg.go.dev/github.com/maltzsama/sqlglot-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/maltzsama/sqlglot-go)](https://goreportcard.com/report/github.com/maltzsama/sqlglot-go)
[![Release](https://img.shields.io/github/v/release/maltzsama/sqlglot-go)](https://github.com/maltzsama/sqlglot-go/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)

A Go port of the **parse → AST → generate** core of
[tobymao/sqlglot](https://github.com/tobymao/sqlglot) v30.17.0, plus the `qualify` and `scope`
optimizer passes that column **qualification and lineage** are built on. Ported file-by-file from the
pinned Python source, as close to 1:1 as Go allows, so upstream tests port across directly. **Zero
third-party dependencies** (Go stdlib only).

This repository is a maintained, independent continuation of `ridi-oss/sqlglot-go` (forked at its
v0.38.0 release). It tracks upstream sqlglot **v30.17.0** and extends the original base/MySQL/Postgres
port with TSQL, ClickHouse, Oracle, StarRocks, Doris, Presto, Trino, Hive, Athena, Redshift,
Materialize, RisingWave, Spark, Databricks, BigQuery, DuckDB, Snowflake, SQLite, MariaDB, and Dremio.

## Scope — what this is (and isn't)

This is **not** a port of all of sqlglot. It ports the front half of the pipeline in full and a
targeted slice of the optimizer:

**Ported (faithful, ~1:1):**
- **Tokenizer, AST/node model, parser, generator** — the full round-trip. Anything upstream sqlglot
  parses and regenerates for **base + MySQL + Postgres**, this port does identically:
  **100% round-trip parity — 1876/1876 corpus cases** (base 980/980, MySQL 428/428, Postgres 468/468),
  checked against the ported upstream test suite. No statement fakes a round-trip via a raw-text
  fallback where upstream builds a structured node (guarded by a committed fidelity test).
- **Cross-dialect transpilation** — reading one registered dialect and writing another, verified
  against upstream's `validate_all` read/write pairs: `TestTranspileCorpus` runs every pair whose
  dialects are all registered — **8758 pairs total, 6658 of them crossing dialects, 6074 passing
  today** — with monotonic per-target-dialect
  floors, and the open pairs are the burndown list in `testdata/transpile_gaps.txt` (mostly the
  per-dialect generator `TRANSFORMS`/`TYPE_MAPPING` remaps).
- **Schema** — `MappingSchema`, `DataType.build`, type-category sets.
- **`qualify` optimizer pass** — qualify_tables → normalize_identifiers → qualify_columns →
  quote_identifiers → validate.
- **Rewrite passes** — `eliminate_ctes` (with the mutation `Journal`), `eliminate_subqueries`,
  `optimize_joins`, `unnest_subqueries` and `merge_subqueries`, each checked against its upstream
  fixture file, on top of a port of upstream's query builder API.
- **`annotate_types`** — type inference with the per-dialect expression metadata generated from
  upstream. The function nodes upstream parses structurally are ported (#44); the remaining
  fixture gaps are recorded in `optimizer/testdata/annotate_gaps.txt`.
- **`simplify`** — boolean, arithmetic, string and date simplification, folding constants with
  upstream's value semantics; remaining fixture gaps are in `optimizer/testdata/simplify_gaps.txt`.
- **`normalize`** and **`eliminate_joins`** — CNF/DNF conversion and unused-join removal.
- **`pushdown_predicates`** and **`pushdown_projections`** — predicate pushdown and unused
  projection pruning.
- **`canonicalize`** — type-driven canonical rewrites (CONCAT, date coercion, boolean predicates).
- **`scope` / `traverse_scope` + the full `Scope` API** — sources, columns, unions, CTEs, subqueries:
  the foundation for **column lineage**.

**Not ported (yet):**
- `canonicalize_internal_names`, which is not one of `optimize()`'s default rules.
- **Dialects beyond the ones listed under [Dialects](#dialects)** (upstream ships 30+).

**Beyond upstream — opt-in, additive (default output unchanged):**
- **Search-path table qualification** — `QualifyOpts.SearchPath` resolves an unqualified table
  against an ordered schema list by proven existence (fail-closed).
- **Top-level `UPDATE`/`DELETE`/`MERGE` scopes** — `TraverseScope`/`BuildScope` bind DML targets and
  their `FROM`/`USING`/`JOIN` sources for analysis (fail-closed; upstream yields none).
- **Qualify resolution report** — `QualifyOpts.ResolutionReport` surfaces each source's `SourceKind`
  (Physical / CTE / Derived / Subquery / Unresolved) + identity.
- **MySQL version/executable comments** — `mysql_version=<MYSQL_VERSION_ID>` activates `/*!… */`
  bodies into the token stream; default-off strips them as upstream does.

See [CHANGELOG.md](./CHANGELOG.md) for the per-version history, [ROADMAP.md](./ROADMAP.md) for the
remaining work and resolved-findings ledger, and [DEVIATIONS.md](./DEVIATIONS.md) for every place the
port *intentionally* behaves differently from upstream sqlglot (headline: ASCII-only identifier
case-folding, to match real engines).

## What works today

| Capability | Package | Notes |
|---|---|---|
| Tokenize | `tokens` | base tokenizer plus per-dialect overrides for every registered dialect |
| Parse → AST | `parser`, `expressions` | full: SELECT/set-ops/CTE/subqueries, all query clauses, predicates, functions, DML + DDL (INSERT/UPDATE/DELETE/MERGE/CREATE + properties), CAST/DataType, PIVOT/LATERAL/VALUES/TABLESAMPLE, INTERVAL, JSON/JSONB ops, window functions |
| Generate (AST → SQL) | `generator` | **100% round-trip** on the upstream identity corpus for base (980/980) and every registered dialect (see [Dialects](#dialects)) |
| Transpile (read one dialect, write another) | `parser`, `generator` | every registered-dialect pair from upstream's `validate_all` corpus runs green or is tracked — 6658 of the 8758 pairs cross dialects, 6074/8758 pass overall; open pairs in `testdata/transpile_gaps.txt` |
| Schema | `schema` | `MappingSchema`, `DataType.build`, type-category sets |
| Qualify | `optimizer` | `qualify` (qualify_tables, normalize_identifiers, qualify_columns, quote_identifiers, validate) |
| Rewrite passes | `optimizer` | `EliminateCTEs`, `EliminateSubqueries`, `OptimizeJoins`, `UnnestSubqueries`, `MergeSubqueries`, `Normalize`, `EliminateJoins`, `PushdownPredicates`, `PushdownProjections`, `Canonicalize`, `Simplify`, `AnnotateTypes` |
| Optimize | `optimizer` | `Optimize` runs upstream's default rule sequence (`Rules`); TPC-H and TPC-DS fixtures pass |
| Scope / lineage | `optimizer` | `traverse_scope` + full `Scope` API (sources, columns, unions, CTEs) |
| Dialects | `dialects` | see [Dialects](#dialects) (tokenizer, normalization, quoting, per-dialect functions/flags) |

## Dialects

Same-dialect round-trip parity against the ported upstream identity corpus
(`testdata/dialect_identity.jsonl`), on top of the base corpus:

| Dialect | Name | Corpus cases |
|---|---|---|
| MySQL | `mysql` | 428/428 |
| Postgres | `postgres` | 468/468 |
| TSQL (SQL Server) | `tsql` | 613/613 |
| ClickHouse | `clickhouse` | 468/468 |
| Oracle | `oracle` | 249/249 |
| Redshift | `redshift` | 171/171 |
| Presto | `presto` | 170/170 |
| StarRocks | `starrocks` | 145/145 |
| Hive | `hive` | 141/141 |
| Trino | `trino` | 115/115 |
| Doris | `doris` | 56/56 |
| Athena | `athena` | 51/51 |
| Materialize | `materialize` | 24/24 |
| Spark | `spark` | 227/227 |
| Databricks | `databricks` | 175/176 |
| BigQuery | `bigquery` | 683/689 |
| DuckDB | `duckdb` | 529/565 |
| Snowflake | `snowflake` | 1209/1543 |
| SQLite | `sqlite` | 136/136 |
| MariaDB | `mariadb` | n/a (no upstream fixture; beyond-upstream identity over MySQL) |
| Dremio | `dremio` | 46/46 |
| Spark 2 | `spark2` | n/a (no upstream fixture; exercised through Spark) |
| RisingWave | `risingwave` | 7/7 |

## Quick start

```bash
go get github.com/maltzsama/sqlglot-go
```

```go
package main

import (
	"fmt"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
	"github.com/maltzsama/sqlglot-go/optimizer"
	"github.com/maltzsama/sqlglot-go/schema"
)

func main() {
	// Parse
	expr, _ := sqlglot.ParseOne("SELECT id, name FROM users WHERE id = 1", "postgres")

	// Qualify against a schema (bind columns to sources, expand *, validate)
	sch := schema.M("users", schema.M("id", "INT", "name", "TEXT"))
	opts := optimizer.DefaultQualifyOpts()
	opts.Schema = sch
	opts.Dialect = "postgres"
	qualified := optimizer.Qualify(expr, opts)

	// Generate SQL back out
	sql, _ := sqlglot.Generate(qualified, "postgres", generator.Options{})
	fmt.Println(sql)
	// SELECT "users"."id" AS "id", "users"."name" AS "name" FROM "users" AS "users" WHERE "users"."id" = 1

	// Walk scopes (sources, columns, unions, …) for lineage
	for _, s := range optimizer.TraverseScope(qualified) {
		fmt.Printf("scope: %d columns, union=%v\n", len(s.Columns()), s.IsUnion())
	}
}
```

## Development

```bash
go test ./...          # green
gofmt -l . && go vet ./...
```

The upstream test suite is ported alongside the code (`*_test.go` + `testdata/*.sql` fixtures reused
verbatim) and is the correctness oracle. A round-trip parity corpus, a cross-dialect transpile corpus
and an AST-fidelity gate keep the port honest — all are monotonic (they can only tighten). The
cross-dialect corpus is re-extracted with
`PYTHONPATH=.reference/sqlglot-v30.17.0 python3 scripts/extract_dialect_corpus.py --transpile-only`,
and its gaps file regenerates with
`SQLGLOT_TRANSPILE_UPDATE=1 go test . -run TestTranspileCorpus -v`. For a live differential check
against the pinned Python source, `scripts/fetch-reference.sh` fetches sqlglot 30.17.0 into
`.reference/`, then e.g.
`PYTHONPATH=.reference/sqlglot-v30.17.0 python3 -c "import sqlglot; print(sqlglot.parse_one('…','postgres').sql())"`.

## Continuing the port

Read [AGENTS.md](./AGENTS.md) and [ROADMAP.md](./ROADMAP.md). The reference Python source
(`.reference/`, fetched via `scripts/fetch-reference.sh`) is the source of truth — port from it 1:1,
port the matching upstream tests, keep `go test ./...` green. `ROADMAP.md` records the remaining
slices, every known divergence, and resolved-findings so settled decisions aren't re-litigated.

## License & attribution

MIT — see [LICENSE](./LICENSE). This is a derivative work: a Go translation of
[tobymao/sqlglot](https://github.com/tobymao/sqlglot) (© Toby Mao, MIT). The upstream MIT license is
preserved. sqlglot-go is not affiliated with or endorsed by the upstream project.
