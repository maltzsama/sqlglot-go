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
port with TSQL, ClickHouse, Oracle, StarRocks, Doris, Presto, Trino, Hive, and Athena.

## Scope — what this is (and isn't)

This is **not** a port of all of sqlglot. It ports the front half of the pipeline in full and a
targeted slice of the optimizer:

**Ported (faithful, ~1:1):**
- **Tokenizer, AST/node model, parser, generator** — the full round-trip. Anything upstream sqlglot
  parses and regenerates for **base + MySQL + Postgres**, this port does identically:
  **100% round-trip parity — 1876/1876 corpus cases** (base 980/980, MySQL 428/428, Postgres 468/468),
  checked against the ported upstream test suite. No statement fakes a round-trip via a raw-text
  fallback where upstream builds a structured node (guarded by a committed fidelity test).
- **Schema** — `MappingSchema`, `DataType.build`, type-category sets.
- **`qualify` optimizer pass** — qualify_tables → normalize_identifiers → qualify_columns →
  quote_identifiers → validate.
- **Rewrite passes** — `eliminate_ctes` (with the mutation `Journal`), `eliminate_subqueries`,
  `optimize_joins`, `unnest_subqueries` and `merge_subqueries`, each checked against its upstream
  fixture file, on top of a port of upstream's query builder API.
- **`annotate_types`** — type inference with the per-dialect expression metadata generated from
  upstream. Functions the port still parses as `Anonymous` annotate as `UNKNOWN` (tracked in #44).
- **`simplify`** — boolean, arithmetic, string and date simplification, folding constants with
  upstream's value semantics; the same `Anonymous` gap leaves some function folds undone (#44).
- **`scope` / `traverse_scope` + the full `Scope` API** — sources, columns, unions, CTEs, subqueries:
  the foundation for **column lineage**.

**Not ported (yet):**
- The rest of sqlglot's **optimizer**: `normalize`, `pushdown_predicates`,
  `pushdown_projections`, `eliminate_joins`, `canonicalize`, and the top-level `optimize()` rule
  orchestrator.
- **Cross-dialect transpilation** — the generator can target a dialect, but only **same-dialect**
  round-trip is a goal and is tested; reading one dialect and writing another is not verified.
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
| Schema | `schema` | `MappingSchema`, `DataType.build`, type-category sets |
| Qualify | `optimizer` | `qualify` (qualify_tables, normalize_identifiers, qualify_columns, quote_identifiers, validate) |
| Rewrite passes | `optimizer` | `EliminateCTEs`, `EliminateSubqueries`, `OptimizeJoins`, `UnnestSubqueries`, `MergeSubqueries` |
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
| Presto | `presto` | 170/170 |
| StarRocks | `starrocks` | 145/145 |
| Hive | `hive` | 141/141 |
| Trino | `trino` | 115/115 |
| Doris | `doris` | 56/56 |
| Athena | `athena` | 51/51 |

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
verbatim) and is the correctness oracle. A round-trip parity corpus and an AST-fidelity gate keep the
port honest — both are monotonic (they can only tighten). For a live differential check against the
pinned Python source, `scripts/fetch-reference.sh` fetches sqlglot 30.17.0 into `.reference/`, then e.g.
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
