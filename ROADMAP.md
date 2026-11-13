# sqlglot-go — roadmap

Goal: a faithful Go port of sqlglot v30.17.0's **parse → AST → generate** core (tokenizer, AST,
parser, generator, schema) plus the `qualify` + `scope` optimizer passes that column qualification and
**lineage** build on, for **base + MySQL + Postgres**. This is deliberately **not** a full port of
sqlglot: the other 30+ dialects are out of scope for now, while cross-dialect transpilation between
the registered dialects is verified against upstream's `validate_all` corpus (#33). Port 1:1 from
.reference/sqlglot-v30.17.0/ file-by-file; port the matching upstream tests as the oracle.

Status: the parse → generate round-trip is at **100% parity on the ported upstream identity corpus** —
1847/1847 cases (base 955/955, MySQL 424/424, Postgres 468/468), enforced by a monotonic corpus floor
(`corpus_test.go` + `testdata/parity_gaps.txt`, now empty) and an AST-fidelity gate (`fidelity_test.go`
+ `testdata/fidelity_cases.txt`: no statement fakes a round-trip via a raw-text `exp.Command` where
upstream builds a structured node). The initial slices below stood up the parse→qualify→scope pipeline;
the residual-tail and CREATE-properties fidelity slices then closed the last round-trip + fidelity gaps.
Slices 0–5 are done and committed; each landed `go test ./...` green. **The CURRENT remaining work is
the optimizer** — see "Remaining work" after the slice ledger.

Slices (historical ledger; each landed `go test ./...` green before the next):

0. FOUNDATION (this run) — DONE when green:
   errors, trie, tokens (TokenType/Token/TokenizerCore/Tokenizer), expressions core
   (Node model + core/query nodes needed for SELECT), minimal SELECT parser, base Dialect.
   Tests: test_tokens.py (all but test_jinja), subset of test_expressions.py/test_parser.py.

1. PARSER CORE — split into 1a/1b so each lands green:
   - 1a: DONE (committed on branch sjcho/sqlglot-go/parser-core; 52 tests green). Grammar
     green, function long-tail + CAST deferred. Includes set operations,
     Subquery/derived tables/scalar subqueries, CTE/WITH, GROUP/HAVING/ORDER/LIMIT/
     OFFSET/FETCH/QUALIFY/DISTINCT/WINDOW/FILTER, predicates (IN/EXISTS/ANY/ALL/
     BETWEEN/IS DISTINCT FROM), CASE/IF/Paren/Tuple, function-call dispatch,
     FUNCTION_BY_NAME, Anonymous fallback, and a curated common-function set.
   - 1b: DONE when green. Landed DML/DDL statement roots, minimal CREATE/Command,
     CAST/`::`/DataType coordination, specialized FUNCTION_PARSERS, bracket
     literals/indexing, LATERAL/UNNEST/VALUES/PIVOT, and the M1 root probes.
   - 1c: DONE (committed on branch; 73 tests green). Landed LOCK/FOR, CLUSTER/DISTRIBUTE/
     SORT BY, PREWHERE, window extras (WITHIN GROUP / IGNORE-RESPECT NULLS / frame EXCLUDE),
     full parseTypes (nested/UDT/parameterized/MAP/STRUCT/ARRAY/enum/NULLABLE/COLLATE),
     INTERVAL literals, full PIVOT/UNPIVOT (Any/Alias), JSON column operators
     (-> ->> #> #>> → JSONExtract*/JSONBExtract*), SELECT TOP wiring, and a function batch.
   - 1d: defer CREATE detail (properties, column + table CONSTRAINT_PARSERS, indexes,
     clone/sequence/materialized), remaining STATEMENT_PARSERS + parse_into(into=),
     CONNECT BY / START WITH, angle-bracket inline STRUCT constructor, and the remaining
     long function tail. (None probe-critical; probe's KNOWN_ROOTS already parse.)

2. GENERATOR CORE — DONE (branch sjcho/sqlglot-go/generator; 81 tests green). Base Generator
   in a new generator/ package (table-driven Kind→sql dispatch, query-modifier clause order,
   identifier quoting for quote_identifiers, ANSI string escaping, pretty/compact). identity.sql
   round-trip: 732/955 pass, 203 need deferred 1d grammar, 20 gen-mismatch (tracked). Public
   API: Generate / Transpile + Expression.SQL. MySQL/Postgres generator overrides → slice 5.

3. SCHEMA — DONE (branch sjcho/sqlglot-go/schema; 93 tests green). New schema/ package:
   Schema iface + MappingSchema + EnsureSchema (nested-mapping normalization, ordered Mapping
   for deterministic iteration, string-part trie, dialect identifier normalization, column_names/
   get_column_type/find/supported_table_args). Completed DataType semantics in expressions/
   datatype.go: DataTypeBuild/FromStr (via a new ParseIntoFunc hook — no parser import cycle),
   DataTypeIsType + category sets (TEXT/INTEGER/FLOAT/NUMERIC/TEMPORAL/NESTED/…). time.py NOT
   needed (zero refs from schema/datatypes). UDF machinery deferred (probe/tests don't use it).
   A nested dict {table:{col:type}} → MappingSchema → column_names/get_column_type verified.

4. OPTIMIZER PASSES — split into 4a/4b so each lands green:
   - 4a: DONE (branch sjcho/sqlglot-go/scope; 103 tests green). New optimizer/ package:
     scope.py (Scope + ScopeType + build_scope + traverse_scope + walk_in_scope +
     find_all_in_scope; sources/selected_sources/cte_sources/external_columns/columns/
     is_union/union_scopes/subquery_scopes/... with lazy caching), plus the pre-passes
     qualify_tables, normalize_identifiers, isolate_table_selects. traverse_scope verified on
     JOIN/UNION/CTE/correlated-subquery. Ported scope + qualify_tables/normalize_identifiers/
     isolate_table_selects fixtures.
   - 4b: DONE (branch sjcho/sqlglot-go/qualify; 106 tests green). resolver + qualify_columns
     + validate_qualify_columns + quote_identifiers + expand_stars + the qualify() driver +
     simplify_parens. ALL 165 in-scope qualify_columns.sql fixtures pass (exact .sql() + AST
     invariant), 18 invalid cases raise. qualify() runs end-to-end (JOIN with unqualified cols
     → qualified + stars expanded + validated). Carried 4a items done: NamedSelects KindSubquery
     (+ Selects()); AST-invariant assertion added to the optimizer harness (assertASTInvariants).
   - 4c (DEFERRED, OFF probe's critical path): full TypeAnnotator/annotate_types (coercion
     tables, per-node type rules). KEY FINDING: annotate_scope is NEVER called for base/mysql/
     postgres — both call sites are gated by ANNOTATE_ALL_SCOPES / SUPPORTS_STRUCT_STAR_EXPANSION,
     both false in base (qualify_columns.py:112,788). A minimal constructible TypeAnnotator +
     no-op AnnotateScope suffices for qualify(); probe never triggers annotation. Port the full
     machinery only for annotate_types.sql test fidelity, after probe e2e. Also deferred:
     canonicalize/normalize (not on qualify's path).

5. MYSQL + POSTGRES WIRING — DONE (branch sjcho/sqlglot-go/dialects; 114 tests green).
   GetOrRaise("mysql"/"postgres") return real *Dialect values: per-dialect TokenizerConfig
   (MySQL backtick identifiers + # comments + backslash/bit/hex + keyword remaps; Postgres $$
   dollar-quotes + HSTORE [moved here from base per slice-3 TODO] + keyword/op deltas),
   NormalizationStrategy (MySQL = CASE_SENSITIVE per mysql.py:25 — NOT case-insensitive;
   Postgres = LOWERCASE), and dialect quoting (MySQL ` / Postgres "). VERIFIED end-to-end:
   parse → qualify → traverse_scope runs under BOTH dialects with correct identifier
   normalization (postgres lowercases unquoted; mysql keeps case) + dialect-correct quoting.
   Ported curated same-dialect validate_identity round-trips from test_mysql/test_postgres.
   - 5b (DEFERRED, not probe-critical): per-dialect parser FUNCTIONS ↔ generator TRANSFORMS/
     TYPE_MAPPING override tables (function + type-name remaps must land paired to avoid
     round-trip regressions; the base tables are package-global singletons). Includes MySQL
     ||/&&/XOR logical operators (DPipeIsStringConcat=false currently errors — safer than
     misparse), MySQL CAST(x AS TIMESTAMP/BLOB) round-trip, and the cross-dialect
     transpilation test cases (need the other 32 dialects — out of M1 scope).

(An external consumer's lineage analyzer was ported and parity-tested against Python during the
initial slices to drive/validate the API surface; it has since been removed — this repo is the SQL
engine only. That validation confirmed the qualify/scope pipeline matches upstream on ~94 real
queries across MySQL/Postgres.)

## Remaining work

The parse → generate round-trip is done for the ported corpus (see Status). The remaining work is
mostly the **optimizer** (port 1:1 from `.reference/`, port the matching upstream tests,
differential-check against the pinned Python):

- THE OPTIMIZER is ported, with the top-level `optimize()` rule pipeline (`Optimize`, `Rules`);
  only `canonicalize_internal_names`, which is not a default rule, is left. Ported: `qualify` +
  `scope`/`traverse_scope` (+ `isolate_table_selects`),
  `eliminate_ctes` + `journal`, `eliminate_subqueries`, `optimize_joins`, `unnest_subqueries`,
  `merge_subqueries`, `annotate_types`, `simplify`, `normalize`, `eliminate_joins`,
  `pushdown_predicates`, `pushdown_projections` and `canonicalize`, plus the query builder API
  they share. `optimizer.sql` passes with only the six tracked `optimizeGaps` rows skipped
  (explode folding, the Spark decorrelation/TRANSFORM cases, and the two Snowflake #40 rows),
  and the TPC-H and TPC-DS fixture sets pass in full.
- annotate_types is ported (#27), and the function nodes it previously parsed as `Anonymous` are
  now structured (#44). The fixture cases that still do not pass are recorded in
  `optimizer/testdata/annotate_gaps.txt`.
- CROSS-DIALECT transpilation: verified against upstream's `validate_all` read/write pairs —
  `TestTranspileCorpus` runs every pair whose dialects are all registered (`testdata/transpile_corpus.jsonl`,
  8497 pairs, 5854 passing), with the open pairs burndown-listed in `testdata/transpile_gaps.txt`.
  What remains are the per-dialect override tables that make cross-dialect output correct — per-dialect
  parser `FUNCTIONS` ↔ generator `TRANSFORMS`/`TYPE_MAPPING` remaps (must land paired to avoid
  round-trip regressions), MySQL `||`/`&&`/`XOR` logical operators, MySQL `CAST(x AS TIMESTAMP/BLOB)`.
- DIALECTS: base, MySQL, Postgres, TSQL, ClickHouse, StarRocks, Doris, Presto, Trino, Hive, Athena,
  Oracle, Redshift, Materialize, RisingWave, Spark 2, Spark, Databricks, BigQuery (#41), DuckDB (#39)
  and Snowflake (#40) are ported; upstream ships 30+ and the rest are out of scope for now (queued:
  SQLite #38, MariaDB #37).
- TIME-TRAVEL clauses (`FOR SYSTEM_TIME AS OF`, hive `TIMESTAMP AS OF`): the v30.17.0 bump removed
  the FOR VERSION/TIMESTAMP and hive AS OF compound keywords but did not port `VERSION_PHRASES` +
  `_parse_version` (parser.py:1738-1745, 5120-5146); the constructs now fail closed (parse error)
  instead of building `Table.version`. Port with a dialect that needs them.
- PARSER coverage is bounded by the ported corpus: constructs upstream parses that are NOT exercised
  by the identity fixtures may still be gaps — e.g. any long `FUNCTIONS`/`FUNCTION_PARSERS` tail or
  DDL detail not hit by a fixture. Treat a not-yet-parsed construct upstream parses as a gap to
  close. (Closed this way so far: `exp.Heredoc` postgres dollar-quoted bodies, and the
  `exp.Block`/`WhileBlock`/`EndStatement` procedure-body machinery.)

## MySQL grammar + tokenizer correctness + typed dialect + Node.Meta — DONE (PR #2)

**DONE (2026-07).** Branch `sjincho/parity/mysql-grammar-tokenizer-meta`, PR
[ridi-oss/sqlglot-go#2](https://github.com/ridi-oss/sqlglot-go/pull/2) — reviewed (Codex gpt-5.6-sol xhigh +
Opus, each verified vs the pinned reference), `go test ./...` / `go vet` / `gofmt` green. Motivated by the
proxy-monster suggestions review; five slices, each a faithful port, an API restoration, or a documented
correctness fix:

- **P1 — MySQL/base `MATCH(cols) AGAINST('x' [modifier])`.** Registered the base `FUNCTION_PARSERS["MATCH"]`
  entry (parser.py:1508 → `_parse_match_against`, parser.py:8181). The `exp.MatchAgainst` node + base/PG
  generators already existed (reachable only via PG `@@`); this closed the parser gap. Ports
  `tests/dialects/test_mysql.py::test_match_against`.
- **P2 — MySQL `GROUP_CONCAT(expr [, …] [ORDER BY …] [SEPARATOR s])`.** Ported `_parse_group_concat`
  (parser.py:10074, MySQL `FUNCTION_PARSERS`, parsers/mysql.py:156) **and** the MySQL generator
  (generators/mysql.py:174) together — parser-only would re-emit the comma form (per-row concat under the
  default separator), a re-generation correctness bug. Multi-arg → `CONCAT`, `ORDER BY` → `exp.Order`.
- **N1 — typed dialect (restores upstream `DialectType`).** `dialects.GetOrRaise`,
  `optimizer.NormalizeIdentifiers`, and `optimizer.QualifyOpts.Dialect` accept `nil | string |
  *dialects.Dialect`; the dialect is threaded through the optimizer + `EnsureSchema` as an `any` so a passed
  `*Dialect` instance flows through **unchanged** (matches upstream `qualify.py:78` — verified by pointer
  identity that `EnsureSchema(...).Dialect()` is the caller's instance). The string/settings form is
  unchanged. Added `(*Dialect).SettingsString()` / `dialects.CanonicalString`.
- **N3 — port `Node.Meta` (upstream `Expression.meta`, core.py:991/996).** `Node` gains a `meta
  map[string]any` with `Meta()` (lazy) / `MetaGet()` (no-alloc) + a deep-copy in `Copy` (core.py:1013);
  wired the `is_table` (qualify_tables.py:55/59, schema.py:704) and `case_sensitive`
  (normalize_identifiers.py:66-67) consumers. **Inert** for base/MySQL/Postgres — only BigQuery reads
  `is_table`, and `case_sensitive` has no producer until `sqlglot.meta` comment parsing is ported — so the
  corpus/fidelity are unchanged; the value is upstream parity + a foundation (resolved the `Node.Meta` TODOs
  in `schema.go`, `normalize_identifiers.go`, `qualify_tables.go`).

**Deviation added:** T1 — MySQL `--` line comment requires a trailing whitespace/control/EOF; otherwise it
is two `-` operators (`1--2` == `1 - -2`), matching the real server. Upstream mis-tokenizes it. Recorded in
**DEVIATIONS.md §1.4** (a same-dialect behavioral deviation, same class as the §1.1 ASCII fold), with a
regression test. Base/Postgres unchanged.

## M1 + P6 — extension mechanism + structural Postgres EXPLAIN — DONE (branch sjincho/parity/extension-mechanism-explain)

**M1 (Class-A divergence mechanism):** `testdata/upstream_extensions.jsonl` ledgers each construct
sqlglot-go parses structurally that pinned upstream does not; `extension_tripwire_test.go` enforces both
sides — `TestUpstreamExtensionsGoSide` (always-on: we parse each row to its `go_kind`) and the
`.reference`-gated `TestUpstreamExtensionsTripwire` (re-asserts pinned upstream still returns
`command`/`parse_error`, failing at a future reference bump with the row's reconcile note). See AGENTS.md
"How deviations are tracked" + DEVIATIONS.md "Grammar extensions beyond upstream".

## P6 — structural Postgres EXPLAIN grammar extension — DONE (same branch)

Postgres `EXPLAIN` now builds a `Describe` root with `kind = "EXPLAIN"`, structured `CopyParameter`
options, a parsed inner statement, and the option-list `wrapped` flag. The Postgres generator preserves
parenthesized comma-separated and legacy space-separated forms; unsupported forms still fail closed to a
verbatim `Command`. This is the first extension governed by the `pg-explain` row in
`testdata/upstream_extensions.jsonl`: pinned v30.12.0 remains `Command`, Go remains `Describe`, and
`TestUpstreamExtensionsGoSide` plus the `.reference`-gated `TestUpstreamExtensionsTripwire` enforce both
sides until a future upstream implementation is reconciled. Dedicated AST/round-trip/fallback tests and
the full build/test/vet/gofmt gate verify the slice. No identity-corpus fixture was added, so the corpus
floors remain **1847/1847** (base 955/955, MySQL 424/424, Postgres 468/468).

## P4 — structural MySQL INSERT SET + REPLACE grammar extensions — DONE

MySQL `INSERT INTO t SET a = 1, b = 2` now parses as `Insert` and intentionally canonicalizes to
`INSERT INTO t (a, b) VALUES (1, 2)`: the target is the existing `Schema(Table, columns)` shape and the
source is one-row `Values(Tuple(values))`, making the generated form idempotent. Supported MySQL
`REPLACE` statements also use the ordinary `Insert` target/source shape plus the Go-only optional
`replace = true` marker; for example, `REPLACE INTO t VALUES (1)` remains that canonical output through
the MySQL generator. The tokenizer no longer packs `REPLACE` as a raw
`Command`; a MySQL-only optional-`TABLE` lookahead preserves a target literally named `table`, a leading
`REPLACE(` retreats to ordinary expression parsing, and unsupported or partially consumed forms fail
closed to a source-preserving `Command`. Only MySQL generation observes the marker, so other dialects
and ordinary `Insert` nodes remain unchanged.

The Class-A ledger rows `mysql-insert-set` (`parse_error` upstream, Go `Insert`) and `mysql-replace`
(`command` upstream, Go `Insert`) lock the extensions to pinned v30.12.0 and define their reconciliation
lifecycle. No corpus or fidelity fixtures or floors were changed: the existing **1847/1847** identity
corpus (base 955/955, MySQL 424/424, Postgres 468/468), including the preexisting
`REPLACE INTO table SELECT id FROM table2 WHERE cnt > 100` row, remains the ratchet.

## R1 / R3 / A1′ / R2 — opt-in analysis + tokenizer enablers — DONE (v0.5.0)

**DONE (2026-07-12, main; released as v0.5.0.)** Four PRs of opt-in, additive enabler APIs for the
downstream lineage/gating consumer — none changes default same-dialect output; each is in DEVIATIONS.md
and (where it goes beyond upstream grammar) the Class-A ledger. Corpus/fidelity floors unchanged.

- **R1 — search-path table qualification (PR #3).** `QualifyOpts.SearchPath []string` stamps an
  unqualified table's `db` from the first schema in the path that _proves_ the table exists; fail-closed
  (a flat/db-incapable schema never stamps), resolves a 2-part probe against a 3-level
  `catalog.schema.table` schema, and leaves `catalog` to the caller. `QualifyTables` gained
  `searchPath, schema.Schema` trailing args.
- **R3 — top-level UPDATE/DELETE/MERGE scopes (PR #4).** A Go-only **analysis** traversal
  (`TraverseScope`/`BuildScope`) yields a DML-root scope binding the target + `FROM`/`USING`/`JOIN`
  sources + columns; complete-or-none (a malformed DML omits the root scope rather than emitting a
  partial one). The optimizer/qualify/validate passes keep the upstream-faithful
  `traverseScopeForOptimizer`, which omits these scopes — so no same-dialect drift. Security-sensitive:
  analysis-only, fail-closed. DEVIATIONS §6.
- **A1′ + V1/V2/A2 — MySQL version comments + tokenizer-sharing enablers (PR #6).** Opt-in
  `mysql_version=<MYSQL_VERSION_ID>` activates `/*!… */` and gated `/*!NNNNN … */` bodies into the token
  stream (default-off strips, DEVIATIONS §1.5). Plus `(*Dialect).IsReservedKeyword`, and documented
  fail-closed contracts for `Tokenize` (errors, never truncates) and `Token.Start`/`End` (byte-exact
  lexeme spans).
- **R2 — Qualify resolution report (PR #7, closes #5).** `QualifyOpts.ResolutionReport` surfaces the
  per-source `SourceKind` (Physical/CTE/Derived/Subquery/Unresolved) Qualify's scope pass already
  computes, classified from the resolved source's dynamic type (never the node's `KindTable`).
  `Unresolved` is the zero value (fail-closed); nil report is a strict no-op. Composes with R3: a DML
  root classifies its target _and_ read-sources from the analysis traversal.

## Athena enabler parity (slice A of the Athena-for-proxy-monster plan) — DONE

Brings dialect `athena` to feature parity with the mysql/postgres enabler surface pm consumes:
settings (`opaque_functions`, `normalization_strategy`) reach the Hive/Trino sub-parsers;
per-statement Hive/Trino routing for batches (DEVIATIONS §1.20); `CaseInsensitive` folding
(§1.19); `ResolveEngineIdentities` athena algorithm (`DefaultCatalog` + `CurrentDatabase`,
`catalog.database.table` identities, global builtins); structured `SHOW *`, `EXPLAIN`, `UNLOAD`,
`RENAME TO PARTITION` (ledger `athena-*`); and the upstream identity corpus for
athena/trino/presto/hive wired into `corpus_test.go` with monotonic floors. The remaining
`parity_gaps.txt` rows for those four dialects are the burndown list for the next slices:

- **B + C — DONE.** Presto/Trino/Hive generators + the Athena Hive/Trino generator router
  (`generator/dialect_dispatch.go` seam: per-dialect dispatch/type-mapping/property-location
  overlays walked through `trino → presto`, `athena-trino → trino`, `athena-hive → hive`).
  Corpus (validate_identity + same-dialect validate_all writes): athena 51/51, presto 170/170,
  hive 141/141, trino 115/115; every remaining parser gap is closed and
  `testdata/parity_gaps.txt` is empty.
- **D — DONE.** Parser drift vs v30.17: `FOR TIMESTAMP|VERSION AS OF` (Version node),
  MATCH_RECOGNIZE, `U&'…' UESCAPE`, Presto formatted-time/regexp/`LOCALTIME[STAMP]`/`JSON '…'`
  functions, Hive `${a:b}` parameters, brace structs, `DISTRIBUTE BY`/`SORT BY` windows, strict time
  tokens, `INSERT … DIRECTORY … STORED AS`, Trino `DECLARE`/`SQL SECURITY`. The only remaining
  Trino inline `WITH FUNCTION` routines also landed (48 cases): `With.udfs`,
  `FunctionSpecification`, and the DECLARE/SET/IF/CASE/WHILE/LOOP/REPEAT/LEAVE/ITERATE routine
  statements, with `trino_udf_test.go` ported from `test_trino.py`.

## Athena support (Presto/Trino/Hive parser chain), scoped to lineage — DONE

**DONE (2026-07, main @ e428a54).** All 4 slices landed + merged, each `go test ./...` green with
base/MySQL/Postgres byte-identical throughout: (1) per-dialect parser-override seam `d9474c2`,
(2) Presto `22c7fbd`, (3) Hive `a1d322e`, (4) Trino + Athena router `e428a54`. `GetOrRaise` now
returns real dialects for **presto / trino / hive / athena** (parser + tokenizer only; generators
skipped as planned). Validation on the real RIDI Athena corpus (1,974 raw queries): `athena` parses
**1909 structured / 0 Command** (= the presto baseline); a `CREATE EXTERNAL TABLE … STORED AS PARQUET
LOCATION …` routes to Hive → structured `Create`; and `ParseOne → Qualify → TraverseScope` runs under
`athena` and resolves columns (proxy-monster's exact pipeline). The seam gained `PropertyParsers`
(slice 3) + token-keyed `NoParenFunctions` + an override-key indirection (slice 4). Known deferrals
(rare, lineage-safe, Anonymous): Presto DATE_FORMAT/DATE_PARSE/REGEXP_*/LOCALTIME family,
MATCH_RECOGNIZE, U&'…' UESCAPE; the 7 Hive CREATE-DDL property callbacks live in Hive's overlay (not
the shared registry) until a future paired parser+generator slice. The original plan follows.

Motivation: proxy-monster (RIDI's query-gating proxy) is adding an Athena proxy alongside its
MySQL/Postgres ones (planned, near-term). Its probe uses `parse` + `qualify` + `traverse_scope`
(+ a same-dialect `.sql()` ONLY for `SELECT *` expansion) — it does NOT canonicalize functions or
transpile cross-dialect. Empirical basis (6,072 unique RIDI Athena queries sampled from Redash;
1,974 raw + 4,098 `{{templated}}`): the CURRENT engine (postgres dialect, zero Athena code) already
structures 1,908/1,974 = 96.7% of the raw queries — matching the pinned Python `athena` dialect
(1,915), with ZERO Command fallbacks. So Athena support is a faithfulness/robustness investment for
the last ~3% + upstream-correct semantics, not a "can't parse it" necessity.

Approach: import the **parser + tokenizer** side of upstream's Athena chain 1:1 — athena→trino→presto
for queries, hive for DDL/CTAS lineage. That is ~570 LOC of `parsers/*.py` (presto 137, hive 298,
trino 63, athena 74) + the tokenizer deltas in each `dialects/*.py`. **Skip the generators** (~1,438
LOC of TRANSFORMS/TYPE_MAPPING) — the probe never canonicalizes or transpiles; the lone same-dialect
`SELECT *`-expansion `.sql()` round-trips via the base generator + `Anonymous` fallback (functions
like URL_DECODE/FORMAT_DATETIME stay Anonymous, which lineage sees through and the generator echoes
verbatim). Athena's parser is a ROUTER: `AthenaParser.parse()` sends DDL to the Hive sub-parser and
queries to the Trino(→Presto) sub-parser.

PREREQUISITE — the per-dialect parser-override seam (parser-side of the deferred 5b): the parser's
`functionParsers` / `statementParsers` / `noParenFunctionParsers` maps and type-token sets are
package-global singletons today; only `d.Functions` (a FUNCTIONS overlay) + a few bool flags are
per-dialect. Presto's parser overrides `FUNCTION_PARSERS` (drops TRIM), adds `STATEMENT_PARSERS`
(Athena's `USING`→command), and brings ARRAY/MAP/ROW type tokens — none layerable per-dialect today.
Generalize the seam so a dialect supplies parser-registry overlays (dialect overlay ∪ base singleton).
This is reusable for every future dialect, so it is foundational, not throwaway.

Slices (each lands `go test ./...` green; regressed against the committed RIDI Athena query corpus):
1. PER-DIALECT PARSER-OVERRIDE SEAM (5b, parser-side): extend Dialect with FunctionParsers /
   StatementParsers / type-token overlays; thread them through the parser's registry lookups as
   (dialect overlay ∪ base singleton). Zero behavior change for base/MySQL/Postgres (empty overlays).
2. PRESTO: parser (FUNCTIONS + FUNCTION_PARSERS + the `_parse` methods for ARRAY/MAP/ROW, UNNEST,
   TRY, lambda `->`, casts/type tokens) + tokenizer deltas. The query-engine bulk — most of the value.
3. HIVE: parser — only the DDL surface Athena routes to Hive (CREATE [EXTERNAL] TABLE etc.) that
   CTAS/DDL lineage needs.
4. TRINO + ATHENA: thin Trino-over-Presto layer + the AthenaParser router (DDL→Hive, query→Trino) +
   athena tokenizer/dialect registration.

Out of scope (same as base/MySQL/Postgres): generator TRANSFORMS/TYPE_MAPPING, cross-dialect
transpilation, the other 30+ dialects. Corpus: RIDI's real Athena queries (sampled 2026-07 from Redash
`platform-athena` + siblings) — kept OUT of the public repo (internal SQL); used as a local
regression oracle only.

Landed (2026-07) — Presto parser + tokenizer (part of Slice 2, atop the committed parser-override
seam): `dialects.Presto()` (class flags, tokenizer config, `d.Functions` overlay of 34 entries), the
`ZONE_AWARE_TIMESTAMP_CONSTRUCTOR` cast promotion (`TIMESTAMP '<zoned literal>'` → TIMESTAMPTZ, Presto-
gated, zero base impact), the `U&'...'` UNICODE_STRING path, and the TRIM function-parser disable. On
the RIDI corpus (2,002 raw non-templated queries) Presto structures 1,935 with 0 Command fallbacks vs
Postgres 1,934/0 — matches-and-beats the baseline. Known divergences for this slice (differential-
tested vs Python 30.12.0):
- Generator TRANSFORMS/TYPE_MAPPING deferred (out of scope), so a structured function whose canonical
  class name differs from its Presto spelling round-trips to the CANONICAL name via the base generator
  (faithful to upstream presto-read + base-write): `ARBITRARY`→`ANY_VALUE`, `CARDINALITY`→`ARRAY_LENGTH`,
  `ROW`→`STRUCT`, `DAY_OF_WEEK`→`DAYOFWEEK_ISO`, `MD5`→`MD5_DIGEST`, `SHA256`→`SHA2(x, 256)`,
  `BITWISE_AND(a,b)`→`a & b`. The four acronym class names (`JSON_FORMAT`/`MD5_DIGEST`/`SHA2`/
  `DAYOFWEEK_ISO`) render via `generator/name.go` sqlNameOverrides mirroring upstream `_sql_names`
  (the camelToSnake split would otherwise emit `J_S_O_N_FORMAT` etc.).
- FUNCTIONS entries needing unported helpers stay Anonymous (fail-closed, lineage-safe, round-trip
  verbatim): DATE_FORMAT/DATE_PARSE/DATE_TRUNC/TO_CHAR (need build_formatted_time / TIME_MAPPING /
  date_trunc_to_time) and REGEXP_EXTRACT/REGEXP_EXTRACT_ALL/REGEXP_REPLACE (need build_regexp_extract
  + the default-group arg). LOCALTIME/LOCALTIMESTAMP, MATCH_RECOGNIZE grammar, TABLE_ALIAS_TOKENS
  |= {ANTI,SEMI}, and the `U&'...'` UESCAPE clause are also deferred (see plan).
- `TRIM(BOTH x FROM y)` fails to parse under Presto (Expecting `)`) — FAITHFUL to upstream, which drops
  TRIM from FUNCTION_PARSERS (presto.py:137), making the special TRIM grammar unreachable. This is the
  single corpus query Presto errors on where Postgres (which keeps the TRIM parser) succeeds.

Historical note: earlier entries below tagged items "off probe's critical path" / "fail-closed" —
that framing referred to a since-removed external consumer. For this repo they are simply parity gaps.

Cross-cutting deferred from foundation (tracked as TODOs in code):
- Expr→SQL (generator) — blocks all .sql() asserts.
- Reflection registries EXPR_CLASSES / FUNCTION_BY_NAME (expressions/__init__.py:47-51) →
  explicit Go registration tables (slice 1).
- Full schema/type annotation hierarchy beyond the parser's minimal DataType/DType nodes (slice 3).
- highlight_sql-rich parse errors already ported in foundation; parse_into(into=) deferred.

**All intentional deviations from upstream are consolidated in [DEVIATIONS.md](./DEVIATIONS.md).** The
headline behavioral one: ASCII-only identifier case-folding (`dialects/dialect.go` NormalizeIdentifier /
CaseSensitive) — a deliberate divergence from upstream's full-Unicode `str.lower()`/`str.upper()`
(dialect.py:1042-1050,1055-1064) to match how real engines fold on multibyte encodings (PostgreSQL
`downcase_identifier`, scansup.c: ASCII-only). See DEVIATIONS.md §1.1.

Known divergences from the r1–r3 adversarial review (differential-tested vs Python 30.12.0;
non-blocking for the foundation, must be resolved by the noted slice):
- arg ordering: newNode orders args by argTypes declaration order, not caller insertion
  order (expressions/core.go newNode). Cosmetic now — HashKey sorts keys, and Expression-
  valued children traverse in the same relative order, so equality/find/walk are unaffected.
  GENERATOR (slice 2): verified NOT needed — the only generic-iteration emit path,
  function_fallback_sql, iterates arg_types (class-declaration order), which argTypesFor(kind)
  already provides independent of Node.argOrder. Still MUST fix before serde (slice 6), which
  serializes the live args in order. (Upstream preserves insertion order via a dict.)
- parser-level comment bubbling: `SELECT a FROM t /* after */` attaches the trailing comment
  to the inner Identifier(t) rather than the Table node; and `_parse_alias` does not yet move
  a mid-expression comment next to the alias (upstream parser.py:8499-8501). Tokenizer-level
  attachment is correct. Cosmetic (AST-shape only) — round-trip output is unaffected since the
  comment still renders in the same textual position either way, so this doesn't fail the
  corpus; still worth fixing for AST fidelity. NOT the same bug as matchRParen's dropped
  expression-hint (see the residual-tail resolved-findings entry below, now fixed) - that one
  DID cause round-trip mismatches (`CAST(x AS INT) /* c */` bubbling all the way to the
  enclosing Select) and is closed.
- deferred-feature parse divergences (expected, un-skip as features land): `/*+ HINT */` errors
  instead of being ignored (slice 1); int64 overflow in ToPy/IsInt (latent until slice 4).
- Slice 1a intentionally drops `_parse_table`'s fast path so subquery detection runs before
  table-part parsing. This is a pure optimization divergence; revisit if parser profiling
  shows it matters.
- `IsWrapper` uses the Go AST's `truthy` helper rather than Python's `v is None` check because
  `newNode` does not store nil args. The wrapper semantics are equivalent for stored args.
- Full DataType semantics remain deferred to schema/DataType slice 3. Slice 1b only adds the
  parser-visible DType enum and DataType nodes needed for CAST/`::` and column definitions;
  generator `.sql()` and rich `.type` assertions stay deferred.

Resolved in the alias-role review pass (PM-escalated, now fixed + regression-tested):
- QualifyTables' setAlias folded the injected default alias as a PARENTLESS identifier
  (NormalizeIdentifiersString) BEFORE attaching it under the TableAlias — a faithful port of
  upstream's `normalize_identifiers(new_alias_name).name` (qualify_tables.py:93), which also omits
  the alias's role (it sets `is_table` only on db/catalog). Harmless upstream, but it starved the
  port's non-upstream role-aware MySQLCaseSensitiveTableNames strategy (lctn=0, §1.2) of the parent
  it reads: the alias for an unaliased mixed-case table was misclassified as column-level and
  lowercased (`FROM Users` → `AS users`) while the column qualifier stayed `Users`, so scope
  resolution could not bind and lineage came back empty. Fixed in optimizer/qualify_tables.go by
  normalizing the alias AFTER `alias.Set("this", ...)`, giving it the TableAlias("this") parent
  (isRelationLevelIdentifier). Output-identical to upstream for every strategy except the role-aware
  one; quoting still honored. Test: TestQualifyTablesDefaultAliasRelationRole. (Rejected the initial
  FoldIdentifierName(name, true) proposal: it folds unconditionally and would over-fold a quoted
  mixed-case alias under the ASCII strategies, diverging from upstream.)

Resolved in the foundation review pass (were latent, now fixed + regression-tested):
- Replace()/Pop() silently no-op'd on single-value (non-list) args — the core tree-rewrite
  primitive every optimizer pass depends on. Fixed in expressions/core.go Replace (route
  index<0 through Set, the index-nil path). Tests: TestReplaceSingleValueArg, TestPopSingleValueArg.
- _parse_alias built an invalid exp.Tuple{this:...} (Tuple has no `this` arg) → ArgError.
  Added exp.Aliases (this+expressions) and use it. Test: TestParseAliases.

Resolved in the slice-2 review pass:
- unnestSQL nil'd the local `offset` after folding WITH OFFSET AS <col> into the alias, so it
  dropped WITH ORDINALITY (and turned an ordinality column into a plain data column). Upstream
  clears only the offset ARG (generator.py:3444-3447 vs 3456-3457). Fixed in generator/sql.go.
  Test: TestUnnestWithOrdinality.

Resolved in the slice-1c review pass:
- parseWindow parsed the frame EXCLUDE option with raise_unmatched=false; upstream
  _parse_window (parser.py:8405) uses the default True, so a malformed EXCLUDE option must
  raise "Unknown option". Fixed in parser/parser.go. Test: TestWindowExtras (malformed case).

Resolved in the slice-4b review pass:
- expandStarsInScope reset the EXCEPT/RENAME/REPLACE maps per selection, so a modifier on a
  leading full `*` did not leak into a later bare `*` (upstream keys by id(table): full stars
  share the stable selected_sources key → leak; qualified stars use fresh keys → no leak).
  Fixed: maps declared once outside the loop; full-star keyed by source name (stable),
  qualified-star keyed by a per-selection-unique token. Test: TestExpandStarsFullStarLeak.
  All 165 in-scope fixtures still pass.

Slice-1b review disposition:
- Reviewer flagged parseValue ignoring its `values` param, claiming upstream has an
  `if not values and self._curr: return None` guard. VERIFIED against the pinned source:
  v30.12.0 `_parse_value` (parser.py:3783) declares `values=True` but never references it —
  the Go port is faithful; that guard exists in a different sqlglot version. No change.
- Genuine minor gap (deferred to dialect slice 5): parseValue does not yet honor
  SUPPORTS_VALUES_DEFAULT (`VALUES (DEFAULT)` → exp.var), a dialect flag; base is unaffected.

Resolved in the slice-1a review pass:
- parseLimit dropped upstream's `isinstance(expression, exp.Mod)` retreat (parser.py:5576-5579),
  so `LIMIT 10 % 3` built Mod(10,3) instead of erroring on the trailing operand. Restored the
  retreat in parser/parser.go parseLimit. Test: TestLimitPercentModRetreat.

Resolved in the residual-tail parity slice (closed the last 25 round-trip gaps in
testdata/parity_gaps.txt, now empty; corpus floors raised to 955/424/468 base/mysql/postgres,
all 1847 records passing):
- New node families ported 1:1 to expressions/kinds.go + expressions/residual_tail.go: BitString/
  HexString/ByteString (query.py:471-491, is_primitive=True — mysql `0x..`/`x'..'`/`b'..'`/`0b..`,
  postgres `e'..'`), SessionParameter (core.py:1837, mysql `@@GLOBAL.x`), PropertyEQ (core.py:2150,
  the `:=` ASSIGNMENT operator), Distance/DistanceNd (core.py:2154-2159, postgres `<->`/`<<->>`),
  Lag/Lead (aggregate.py:150-163), Concat (string.py:29-31, adjacent-string-literal rewrite only).
- tokens/tokenizer.go compileConfig now auto-populates UnescapedSequences from the default table
  (dialects/dialect.py:297-306) whenever StringEscapes['\\'] or ByteStringEscapes['\\'] is set —
  fixes mysql's `'\\"a'` round-trip (the tokenizer wasn't collapsing `\\` to one backslash) and
  makes postgres `e'\n'`/`e'\t'`/etc. scan to real control chars for the byte-string family above.
- generator/sql.go escapeStr split into escapeStr (unchanged default) + escapeStrOpts (full
  escape_str signature: escapeBackslash/delimiter/escapedDelimiter/isByteString,
  generator.py:2983-3005), needed by bytestringSQL's escape_backslash=False behavior (a literal
  backslash, e.g. postgres `e'\176'`'s octal escape, must round-trip unchanged).
- parseType (mysql only) ports parsers/mysql.py:545-558's BINARY-as-cast special case
  (`ORDER BY BINARY a` -> `ORDER BY CAST(a AS BINARY)`); parseFunctionCall's func-token gate
  gained CharsetIsFunction (mysql `CHARSET(...)`, parsers/mysql.py:69 FUNC_TOKENS).
  hexstringSQL/CHAR(0x.. USING ..) needed the HexString kind above to close (mysql
  `CHAR(0xC3A9 USING utf8mb4)`).
- parseAssignment now implements the ASSIGNMENT `:=` loop (parser.py:5790-5810, was a bare
  pass-through to parseDisjunction); primaryParsers[SESSION_PARAMETER] + parseSessionParameter
  added (parser.py:7168-7176) — together close mysql `@var1 := 1`/`@@GLOBAL.x`.
  factorTokens gained LR_ARROW/LLRR_ARROW -> Distance/DistanceNd (parser.py:917-918), closing
  postgres `<->`/`<<->>` and (as a side effect, per the plan's predicted prerequisite chain) the
  `LATERAL VERTICES(...) v1 <-> v2` gap, which needed no other grammar change.
- parseWrappedSelect was missing _parse_wrapped_select's Values-into-Table rewrap
  (parser.py:3828-3832): a parenthesized `(VALUES (1)) AS v(id) LEFT JOIN t ON ...` couldn't
  attach the trailing JOIN. Fixed.
- parseDatePart + a dialect-gated FUNCTION_PARSERS["DATE_PART"] entry port postgres's
  `_parse_date_part` (parsers/postgres.py:303-311): `DATE_PART(<part>, <value>)` desugars to
  `EXTRACT(<part> FROM <value>)`.
- parsePrimary's STRING dispatch was missing _parse_primary's adjacent-string-literal rewrite
  (parser.py:6871-6885): `'a' 'b' 'c'` now builds Concat (coalesce=dialect.CONCAT_COALESCE,
  a new Dialect field — dialects/dialect.py:404, postgres.py:15 override) instead of falling
  through to plain alias handling.
- parseBracket was missing _parse_bracket's ARRAY_CONSTRUCTORS swap (parser.py:7713-7721,
  table at :787-790): a bracket subscripting a bare "ARRAY" reference now builds a real
  exp.Array instead of a Bracket workaround, fixing `ARRAY[]::type[]` (empty array — Bracket's
  "expressions" arg is required, so an empty list tripped the same "Required keyword" check
  upstream's own error_messages has; Array's is optional). Needed a new arraySQL generator
  method (postgres bracket-notation via generators/postgres.py:502-509, `ARRAY(<subquery>)` for
  a query-typed sole element; base/mysql fall back to the pre-existing functionFallbackSQL paren
  form, verified against the oracle to be base's real behavior - the OLD Bracket-based
  `TestArraySizeDimDroppedBase` expectation was actually wrong, not identity-preserving, and is
  corrected). The old isArrayLiteralBracket helper + its bracketSQL special-case branch were
  removed during integration (parseBracket never produces that Bracket-with-"ARRAY"-column shape
  again; the postgres ARRAY[...] pretty-wrap path is now arraySQL -> inlineArraySQL).
- KindLag/KindLead registered (were unregistered, falling through to Anonymous, which has no
  AggFunc trait): this - not a parser change - is what actually closes
  `(LEAD(foo1, 1, 0)) OVER (...)`, since parseParen's existing `this.This().Is(TraitAggFunc)`
  window-reparse gate already handled the parenthesized-aggfunc case correctly once LEAD/LAG
  build a real AggFunc-trait node instead of Anonymous.
- matchRParen silently dropped its `expression` parameter (a no-op wrapping bare `p.match`),
  unlike upstream's `_match_r_paren(expression)` -> `_match(.., expression=expression)`, which
  attaches a same-line trailing comment after the just-matched `)` directly to that expression
  (parser.py:9432-9434, 1926-1938). Instead the comment lingered in p.prevComments and bubbled
  up to whatever outer node's next p.expression(...) call happened to consume it - e.g.
  `SELECT CAST(x AS INT) /* c */ FROM foo` attached the comment to the outer Select instead of
  the Cast. Fixed generically (matchRParen now calls p.addComments(expression) after a
  successful match); this single fix, at its ~11 non-nil call sites across the parser, also
  closed the separate `SELECT FOO(x /* c */) /* FOO */, b /* b */` gap with no further change.

Closed in the integration / review-findings pass (dialect-divergence findings that the
same-read==write corpus can't reach — cross-dialect transpile, bare/underscore literals, and
hand-built ASTs; each verified against the pinned reference and guarded by
TestReviewFindingsFixes):
- The CONCAT(...) function is now a registered builder (parser.parseConcat, porting the FUNCTIONS
  "CONCAT" lambda at parser.py:345-349) instead of an Anonymous call, so it composes with the
  generator work below: `CONCAT(a, b)` base->postgres -> `a || b`, postgres->mysql ->
  `CONCAT(COALESCE(a, ''), COALESCE(b, ''))`, same-dialect stays `CONCAT(a, b)`. It lives in the
  dialect-aware FUNCTION_PARSERS map (not FunctionByName) because safe/coalesce depend on the
  dialect. CONCAT_WS is intentionally left Anonymous: it needs a distinct exp.ConcatWs node whose
  concatws_sql wraps a NULL-coalescing dialect in a CASE (generator.py:3683+) - out of this port's
  scope, and it only diverges cross-dialect (never in the same-read==write corpus), so leaving it
  Anonymous is no regression.
- concatSQL now ports the full concat_sql (generator.py:3667-3682): the CONCAT_COALESCE branch
  transpiles a coalesce=false Concat to a `||` DPipe chain (concat_to_dpipe_sql, dialect.py:1804)
  so e.g. `'a' 'b'` base->postgres becomes `'a' || 'b'` (NULL-propagating) instead of CONCAT(...),
  and convert_concat_args (generator.py:3636-3665) wraps each non-string arg in COALESCE(e, '')
  when the write dialect's CONCAT doesn't coalesce but the node asks it to. The per-arg
  string/ARRAY-type skip still defers full annotate_types (ROADMAP 4c): a statically-known string
  literal is left unwrapped (matching upstream), any other arg is wrapped.
- hexstringSQL ports the full is_integer_type condition (generator.py:1578-1586): a
  HexString{is_integer:true} renders as its integer value even where the write dialect has a
  HEX_START (HEX_STRING_IS_INTEGER_TYPE is false for base/mysql/postgres). bytestringSQL's
  is_bytes cast now wraps the re-parsed byte-string node (exp.cast semantics) rather than the
  rendered literal, so `ByteString{is_bytes:true}` -> `CAST(e'abc' AS BYTEA)`, not
  CAST of a doubled-quote string. (No in-scope parser sets either flag; these guard the exposed
  node contract.)
- Bit/hex literal validation is now one CPython-faithful int() parser, tokens.ParseIntPython
  (*big.Int, bool), shared by the tokenizer and the generator's integer fallback so the accepted
  forms match exactly. It honors a leading +/- sign, a matching base prefix (0b/0o/0x), and single
  '_' separators between digits or right after the prefix (never leading/trailing/doubled). The
  quoted forms validate int(payload, base), so `x'0xA'`/`x'+A'`/`x'-FF'`/`x'A_B'` round-trip and
  `x'GG'` still errors; the bare forms validate int(fullText, base), so `0x_FF` tokenizes (payload
  "_FF" -> x'_FF') while `0x`/`0xGG` fall back to an identifier. The generator fallback panics
  (recovered by Generate into an error) on an invalid payload, matching CPython's ValueError - e.g.
  folding a bare `0x_FF` to base runs int("_FF", 16), which raises upstream too (an upstream
  asymmetry the port mirrors). An empty `x''`/`b''` is left as-is (upstream skips int() on it).
- The postgres pyformat placeholder name is parsed with parseIdVar(any_token=true), matching
  upstream `_parse_wrapped(self._parse_id_var)` (parsers/postgres.py:294-301), so a non-reserved
  keyword or a number is a valid name: `%(from)s` / `%(1)s`, not only a bare identifier `%(name)s`.
- postgres now sets has_bit_strings/has_hex_strings (= bool(BIT_STRINGS)/bool(HEX_STRINGS),
  tokens.py:581-582; postgres tables are non-empty, dialects/postgres.py:65-66), enabling the
  number scanner's bare `0b`/`0x` forms (`SELECT 0xFF` -> `SELECT x'FF'`), which base does not have.
- VARIADIC's postgres-only NO_PAREN_FUNCTION_PARSERS entry is now filtered per-dialect at both
  lookup sites (noParenFunctionParserFor) instead of self-gating inside the closure. The old
  self-gate returned nil after the caller had already committed to the no-paren path, so base/mysql
  `VARIADIC(x)` degraded to `VARIADIC AS (x)`; it now parses as an ordinary function call, and a
  bare `VARIADIC` stays a column in base/mysql (parsers/postgres.py:142; per-dialect table deferred
  to slice 5b).

Resolved in the CREATE-properties fidelity slice (drove a 95-row Python-oracle AST/SQL gate,
testdata/fidelity_cases.txt + fidelity_test.go; corpus stays 955/424/468 base/mysql/postgres, all
1847 records passing, parity_gaps.txt empty; the full-corpus Go-vs-Python Command audit went from
94 Go-only degradations to 1 — see the deferred Heredoc item below):
- AST (expressions/kinds.go + expressions/fidelity_*.go): added the property/query/constraint node
  families needed to model the worklist — Algorithm/AutoIncrement/Collate/Definer/Engine/Inherits/
  Like/Lock/Locking/Materialized/NoPrimaryIndex/OnCommit/PartitionedBy/PartitionByRange/
  PartitionByList/PartitionedOf/SchemaComment/SqlReadWrite/Temporary/Unlogged/WithData properties,
  PartitionList/PartitionBoundSpec/PartitionRange, AnalyzeHistogram/AnalyzeWith/UsingData, and the
  Compress/DateFormat/Exclude/InlineLength/Title/Uppercase/WithOperator/InOut column constraints.
  This is the worklist subset, NOT the full upstream 80+ PROPERTIES / all DDL constraints.
- Parser: real _parse_properties loop + POST_CREATE/NAME/SCHEMA/WITH/ALIAS/EXPRESSION/INDEX property
  placement in _parse_create; CREATE TYPE enum/composite; bare UNIQUE/PRIMARY/AMP indexes; postgres
  function parameter modes (InOutColumnConstraint); MySQL RANGE/LIST partition DDL; MySQL ALTER
  options + AUTO_INCREMENT; EXCLUDE / WITH-storage / reserved WITH-operator constraints; MySQL
  ANALYZE ... HISTOGRAM and structured COMMENT ON FUNCTION/PROCEDURE.
- Generator: PROPERTIES_LOCATION bucketing + dedicated renderers, full CREATE assembly order, MySQL
  partition rendering, postgres columndef_sql placing the parameter mode before the name (side-
  effect-free, does not pop() the constraint), and the new constraint/analyze renderers.
- Integration ToS-fidelity fixes so repr()==ToS() exactly (verified generation + Equal/hash are
  untouched — they use truthy()/isFalse and skip false/empty): arg-order rows for Create/Alter/
  UniqueColumnConstraint/IndexParameters now follow the parser's constructor-kwarg order (what
  repr's dict-insertion order reflects), not the class arg_types order; DType renders as its enum
  member name "DType.<NAME>" (USERDEFINED name != value); _parse_types sets nested on the scalar
  branch; parseExists returns the bool False (not nil) like upstream's and-chain, so Alter/Drop/
  Insert/Truncate/ColumnDef carry exists=False; Node.IsLeaf/isEmptyList handle a non-empty/empty
  []string (AnalyzeWith, UniqueColumnConstraint.options); UNIQUE index_type defaults to False;
  CollateProperty.default is set only under a leading DEFAULT (unlike CharacterSetProperty).
- Gate floors (fidelity_test.go): >=95 cases, >=93 command-free. Two command_exception rows are
  legitimate upstream nested Commands — `CREATE FUNCTION ... SET search_path TO 'public' AS ...` and
  the sibling SET-config row — whose SetConfigProperty.this is itself a Command in the pinned tree
  (_parse_set returns Command mid-CREATE, parser.py:9265-9275), so a Command-free port is impossible
  there; the root stays Create.
- Two ast_divergence rows (MySQL `PARTITION BY RANGE (YEAR|MONTH(col))`): upstream wraps all 11 MySQL
  date functions in TsOrDsToDate in the parser and removes it in the generator; this port elides
  both consistently (no exp.TsOrDsToDate Kind — generator/dialect_funcs.go). The CREATE parses to a
  full PartitionByRangeProperty and round-trips to identical SQL; only the incidental partition-
  expression arg shape differs, so want_ast stays the honest Python oracle and only the exact ToS()
  match is relaxed (capped at maxASTDivergences=2). Closing it belongs to the function-parity slice.
- Resolved later (was: the 1 remaining full-corpus Go-only Command): postgres
  `CREATE FUNCTION ... AS $$ ... $$` dollar-quoted UDF bodies now parse to exp.Heredoc, and
  procedure BEGIN...END bodies to exp.Block (+ exp.WhileBlock/EndStatement), matching upstream
  parser.py:2114-2135/2463/9368-9370. See procedure_block_test.go + the fidelity oracle rows.

## TSQL (SQL Server) dialect — DONE (issue #118)

Ported 1:1 from `.reference/sqlglot-v30.17.0/sqlglot/{dialects,parsers,generators}/tsql.py`.
`dialects.GetOrRaise("tsql")` returns a real dialect: CASE_INSENSITIVE normalization, `[bracket]`
identifiers, `N'...'` national strings, `@`/`@@`/`#` single tokens, `GO` batches, `0x` hex literals,
`TYPED_DIVISION`, `CONCAT_COALESCE`, `LIMIT_IS_TOP`, `LIMIT_FETCH=FETCH`, `ENSURE_BOOLS`, and the
TSQL `FUNCTIONS` overlay (CONVERT, DATEADD/DATEDIFF/DATENAME/DATEPART/DATETRUNC, ISNULL, LEN/LEFT/
RIGHT, GETDATE/SYSDATETIME/SYSDATETIMEOFFSET/NEWID, EOMONTH, FORMAT, HASHBYTES, PARSENAME, ...).

Parsed end to end: `SELECT TOP n [PERCENT] [WITH TIES]`, `OFFSET ... FETCH`, `CROSS/OUTER APPLY`,
`FOR XML|JSON|BROWSE`, `OPTION (...)`, `DECLARE`/`EXECUTE`/`sp_executesql`, `BEGIN/COMMIT/ROLLBACK`
transactions, `NEXT VALUE FOR`, `#temp`/`##global` tables, `IDENTITY`, `CLUSTERED`/`NONCLUSTERED`
constraints, `x::y` scope resolution, and the TSQL DDL/DML forms exercised by the corpus.

Corpus: `testdata/dialect_identity.jsonl` carries the 613 `validate_identity` cases extracted
from `tests/dialects/test_tsql.py` (`scripts/extract_dialect_corpus.py --dialects tsql`), and the
dialect is at **613/613** — floors in `corpus_test.go` raised to 613 and no TSQL rows remain in
`testdata/parity_gaps.txt`. The final burndown (fork issue `maltzsama/sqlglot-go#3`) closed the
last 13 rows: CREATE TRIGGER bodies, `OPENJSON ... WITH` column definitions, MERGE nested in a
derived table (`_parse_wrapped_select` override), temporal `PERIOD FOR SYSTEM_TIME` /
`SYSTEM_VERSIONING` / `DATA_DELETION` DDL (base `PERIOD` constraint registry, the
`PROPERTIES_LOCATION` entries, and the None-not-`""` optional-arg rendering), `ALTER TABLE ... SET`
interval option lists, `CREATE TABLE ... ON <filegroup>`, COPY `CREDENTIAL = (...)` paren option
lists (with the `SequenceProperties` comma-splitter), statement-leading `::FOO(...)` range-parser
scope resolution, the TSQL ensure-bools render-time rewrite (incl. width-aware `connector_sql`
breaking), and the pretty UDF parameter-list indent. Regression tests:
`TestTSQLParserGeneratorGaps` in `tsql_test.go`.
## ClickHouse dialect — DONE (issue #3)

Ported 1:1 from `.reference/sqlglot-v30.17.0/sqlglot/{dialects,parsers,generators}/clickhouse.py`.
`dialects.GetOrRaise("clickhouse")` returns a real dialect: CASE_SENSITIVE normalization,
`NORMALIZE_FUNCTIONS=False` (original function spelling preserved through
`PRESERVE_ORIGINAL_NAMES`), `NULL_ORDERING=nulls_are_last`, `INDEX_OFFSET=1`,
`NUMBERS_CAN_BE_UNDERSCORE_SEPARATED`, `IDENTIFIERS_CAN_START_WITH_DIGIT`, `0b`/`0x` literals,
`$tag$...$tag$` heredocs, `#`/`#!` comments, `FLOAT32`/`FLOAT64`/`UINT*`/`DATE32`/`DATETIME64`/
`ENUM8/16`/`LowCardinality`/`Map`/`Nested`/`Tuple` types, `.^`/`.:` column operators, `SAMPLE`,
`PREWHERE`, `FINAL`, `GLOBAL IN`, `ON CLUSTER`, `SETTINGS`/`FORMAT` query modifiers, the
aggregate-combinator resolver (`sumIf`/`sumMergeState`/`quantiles(0.5)(x)`), `COLUMNS(...) APPLY`
chains, brace query parameters, JSON type arguments (`SKIP`, `max_dynamic_paths`), and the
`FUNCTIONS` overlay (TOSTARTOF*/TOMONDAY, DATE_ADD/DATE_DIFF/DATE_SUB, splitBy*, map, has, uniq,
countIf, ...).

Parsed end to end: `SELECT ... PREWHERE ... FINAL`, `ARRAY JOIN`, `GLOBAL ANY LEFT JOIN`,
`ORDER BY ... WITH FILL [FROM/TO/STEP/INTERPOLATE]`, `GROUP BY ... WITH TOTALS`,
`INSERT INTO FUNCTION ... PARTITION BY ... SETTINGS ... SELECT`, `DETACH`/`ATTACH`/`EXCHANGE`,
`CREATE TABLE` with `ENGINE`/`ORDER BY`/`PARTITION BY`/`TTL`/`SETTINGS`/`SAMPLE BY`/`CODEC`/
`ASSUME`/`PROJECTION`, `CREATE DICTIONARY` (`SOURCE`/`LAYOUT`/`LIFETIME`/`RANGE`), `ALTER TABLE
... MODIFY COLUMN|TTL|SQL SECURITY`, `DROP ... ON CLUSTER`, `DELETE ... ON CLUSTER`, and
`CREATE FUNCTION ... AS lambda`.

Corpus: `testdata/dialect_identity.jsonl` carries the 468 `validate_identity` cases extracted from
`tests/dialects/test_clickhouse.py` (`scripts/extract_dialect_corpus.py --dialects clickhouse`,
now part of the extractor's dialect list). The dialect is at **468/468**, with no ClickHouse rows
in `testdata/parity_gaps.txt`. Shared fixes made while closing it: `opExpressions` renders flat
when the node is a Properties child, join `global_`, `IN [..]` bracket lists, multi-character
heredoc escape handling, per-dialect `ESCAPED_SEQUENCES`, and the `{**default, **override}`
UNESCAPED_SEQUENCES merge. Regression tests: `clickhouse_test.go`
(`TestClickhouseIdentity`, `TestClickhouseASTShapes`, `TestClickhouseDialectFlags`,
`TestClickhouseQualifyAndScope`).

## StarRocks dialect — DONE (issue #3)

Ported 1:1 from `.reference/sqlglot-v30.17.0/sqlglot/{dialects,parsers,generators}/starrocks.py`.
StarRocks extends MySQL upstream, so `dialects.StarRocks()` starts from `MySQL()` and applies the
deltas: the parser-override registry now has a parent chain (`starrocks` → `mysql`) and the
generator dispatch reuses the existing `dialectParents` chain; `INDEX_OFFSET=1`; the tokenizer
adds `LARGEINT`/`REFRESH`; the generator flags `PROPERTIES` prefixes, `INSERT OVERWRITE`, the
reserved keyword list, `IS_BOOL_ALLOWED=false`, and `RENAME_TABLE_WITH_DB=false`.

Parsed end to end: `CREATE TABLE` with `PRIMARY KEY`/`UNIQUE KEY`/`ROLLUP`/`DISTRIBUTED BY`/
`PARTITION BY RANGE|LIST` (including the dynamic `START/END/EVERY` form)/`PROPERTIES`,
materialized views with the full `REFRESH [DEFERRED|IMMEDIATE] [ASYNC|MANUAL] [START] [EVERY]`
clause, `REFRESH EXTERNAL TABLE`, `UNNEST` default aliases, `LEFT SEMI/ANTI JOIN`, and the
MySQL-inherited grammar.

Generator preprocess steps ported from `transforms.py`: `DELETE ... BETWEEN` expansion,
`SELECT DISTINCT ON` elimination into a `ROW_NUMBER()` subquery, and `UNNEST(GENERATE_DATE_ARRAY)`
rewriting into a recursive CTE.

Corpus: 145 `validate_identity` cases from `tests/dialects/test_starrocks.py`
(`scripts/extract_dialect_corpus.py --dialects starrocks`), at **145/145**, with no StarRocks rows
in `testdata/parity_gaps.txt`. Regression tests: `starrocks_test.go`.

## Doris dialect — DONE (issue #3)

Ported 1:1 from `.reference/sqlglot-v30.17.0/sqlglot/{dialects,parsers,generators}/doris.py`.
Doris extends MySQL upstream, so `dialects.Doris()` starts from `MySQL()` and joins the same
parser/generator inheritance chains as StarRocks.

Parsed end to end: `CREATE TABLE` with `UNIQUE KEY`/`DUPLICATE KEY`/plain `KEY`/
`DISTRIBUTED BY HASH|RANDOM`/`PROPERTIES`, `PARTITION BY (...)` keys, `PARTITION BY RANGE|LIST`
with static `VALUES LESS THAN`, Doris's `VALUES [(...), (...))` bracket groups and the dynamic
`FROM (...) TO (...) INTERVAL n UNIT` form, materialized views with the Doris
`BUILD [DEFERRED|IMMEDIATE]` and `REFRESH [AUTO|COMPLETE] ON [MANUAL|COMMIT|SCHEDULE] [EVERY n UNIT]
[STARTS '...']` clauses, plus the MySQL-inherited DML.

Generator details: the Doris type mappings, the `PROPERTIES` prefix, the materialized-view `KEY`
prefix for `UniqueKeyProperty`, the Doris `PARTITION RANGE`/dynamic-partition renderings, the
UPDATE/DELETE table alias without `AS`, and the function renames (`MONTHS_ADD`, `L2_DISTANCE`,
`COLLECT_LIST`/`COLLECT_SET`, `SPLIT_BY_STRING`, `TO_DATE`, ...).

Corpus: 56 `validate_identity` cases from `tests/dialects/test_doris.py`
(`scripts/extract_dialect_corpus.py --dialects doris`), at **56/56**, with no Doris rows in
`testdata/parity_gaps.txt`. Regression tests: `doris_test.go`.

## Oracle dialect — DONE (issue #3)

Ported 1:1 from `.reference/sqlglot-v30.17.0/sqlglot/{dialects,parsers,generators}/oracle.py`.
Oracle extends the base dialect (not MySQL), with UPPERCASE normalization,
`SUPPORTS_COLUMN_JOIN_MARKS` derived from the `(+)` keyword, the Oracle tokenizer (`MINUS` ->
EXCEPT, `START` -> BEGIN, `VARCHAR2`/`NVARCHAR2`, `U+...'` strings, `@`/`$`/`#` vars,
`ORDER SIBLINGS BY`, `SAMPLE`), the Oracle `TIME_MAPPING`/`INVERSE_TIME_MAPPING` format
conversion, and pseudocolumns (`ROWNUM`/`ROWID`/`LEVEL`/...).

Parsed end to end: `DATE`/`TIMESTAMP` typed literals, `SYSDATE`/`SYSTIMESTAMP`, `NVL`,
`TO_DATE`/`TO_TIMESTAMP`/`TO_CHAR`/`TO_NUMBER`, the overloaded `TRUNC` with its default `DD`
unit, `LISTAGG ... ON OVERFLOW`, `KEEP (DENSE_RANK ...)`, `CONNECT BY`/`START WITH`/`PRIOR`,
`CONNECT_BY_ROOT`, `(+)` outer-join marks, `SELECT UNIQUE`, `BULK COLLECT INTO`,
`INSERT ALL/FIRST` with `WHEN`/`ELSE` branches, `PIVOT`/`UNPIVOT` with Oracle alias rules,
`SAMPLE`, `JSON_EXISTS`/`JSON_ARRAY`/`JSON_ARRAYAGG`, `CREATE OR REPLACE FORCE VIEW`, and the
`ALTER TABLE ... ADD (...)` form without the COLUMNS keyword.

Generator details: the Oracle type mappings, `TZ_TO_WITH_TIME_ZONE`, the `TO_CHAR`/`TO_DATE`/
`TO_TIMESTAMP` format conversion, `SYSDATE`/`SYSTIMESTAMP`, `INSTR`, `SUBSTR`, `MOD`,
`L2_DISTANCE`, `DBMS_RANDOM.VALUE`, hint re-rendering with `QUERY_HINT_SEP`, the UPDATE/DELETE
table alias without `AS`, and the PIVOT/UNPIVOT alias rules.

Corpus: 249 `validate_identity` cases from `tests/dialects/test_oracle.py`
(`scripts/extract_dialect_corpus.py --dialects oracle`), at **249/249**, with no Oracle rows in
`testdata/parity_gaps.txt`. Regression tests: `oracle_test.go`.

## Redshift dialect — DONE (issue #34)

Ported 1:1 from `.reference/sqlglot-v30.17.0/sqlglot/{dialects,parsers,generators}/redshift.py`.
Redshift extends Postgres (the parser and generator override chains fall back to the postgres
tables), with case-insensitive identifier normalization, `NormalizeNotNull`, the `(+)` outer-join
marker enabled through `SUPPORTS_COLUMN_JOIN_MARKS`, `LockingReadsSupported = False`, no derived
table needed for star `EXCLUDE`, distinct array constructors, and the Redshift `TIME_MAPPING`
additions (including the `MON`/`MONTH` conversions layered on Postgres's table).

Parsed end to end: `SUPER`/`HLLSKETCH`/`VARBYTE` types with the Redshift type mapping, `UNPIVOT`
over SUPER objects (`UNPIVOT foo.obj[0] AS val AT attr`), `AT` array indexing, select-level
`EXCLUDE`, `ALTER TABLE ... DISTKEY / DISTSTYLE / SORTKEY`, `COPY` and `UNLOAD`, `GRANT`/`REVOKE`,
the `CONVERT` parse override, `DATEADD`/`DATEDIFF` builders mapped through `DATE_PART_MAPPING`,
`IGNORE NULLS`/`RESPECT NULLS` hoisting for window functions, `ENCODE` column properties and the
`DISTKEY`/`SORTKEY`/`DISTSTYLE` table properties, and `CURRENT_USER_ID` as a no-paren function.

Generator details: the Redshift type mappings (`VARCHAR(max)`, `TIMESTAMPTZ`), the shared
`tzToWithTimeZone` datatype transform (renamed out of the Oracle generator so both dialects render
`WITH TIME ZONE`), the no-op cast to `JSON`, and the property rendering for the table/column
properties above.

Corpus: 171 `validate_identity` cases from `tests/dialects/test_redshift.py`
(`scripts/extract_dialect_corpus.py --dialects redshift`), at **171/171**, with no Redshift rows in
`testdata/parity_gaps.txt`. Regression tests: `redshift_test.go` (the full upstream test file is
ported there: identity, cross-dialect, AST shapes, dialect flags, qualify/scope, cast,
TO_TIMESTAMP annotation and the Command-fallback GRANT/REVOKE/UNLOAD rows).

Known divergences for this slice:

- The implicit-unnest conversion landed after the dialect port: `FROM a, a.b` joins whose leftmost
  part names an already-introduced relation now become explicit `Unnest` nodes, matching upstream
  `SUPPORTS_IMPLICIT_UNNEST` (parsers/redshift.py:84 → parser.py:4305-4330), so cross-dialect writes
  emit `UNNEST(...)` instead of a bare column join. Asserted by `TestRedshiftColumnUnnesting`.
- 23 of the 106 `validate_all` arms of `tests/dialects/test_redshift.py` are omitted from
  `TestRedshiftCrossDialect`: cross-dialect generator output this port does not reproduce yet. They
  group as — `JSON_EXTRACT_PATH_TEXT` → presto; `ADD_MONTHS` → trino, tsql; `IS TRUE`/`IS
  FALSE`/`IS NOT FALSE` → presto; `DISTINCT ON` → hive, mysql, oracle, presto, starrocks, trino,
  tsql; `DATEDIFF` day → hive, presto; `DATEADD` month → hive, mysql, postgres, presto, tsql;
  `DATEDIFF` week → hive, postgres, presto. The same statements' Redshift identity rows all pass;
  each group is a shared-generator rendering gap, not a Redshift parse gap.
- Pre-existing (not introduced by this branch): the hive generator's `unnest_to_explode` transform
  (upstream generators/hive.py:339; also databricks.py:44, spark2.py:188) is not ported, so an
  explicit Unnest written to hive renders `SELECT * FROM x, EXPLODE(a.b, _t0(c))` where upstream
  renders `SELECT * FROM x LATERAL VIEW EXPLODE(a.b) _t0 AS c`. Base-read of that query produced the
  same wrong hive output before the implicit-unnest fix, which is how pre-existence was proven.
  Still open after #35 landed: the whole hive `Select` preprocess chain (eliminate_qualify,
  eliminate_distinct_on, unnest_to_explode, any_to_exists) remains a documented cross-dialect-only
  deferral in DEVIATIONS §2, and no identity-corpus case depends on it.

## Materialize dialect — DONE (issue #36)

Ported 1:1 from `.reference/sqlglot-v30.17.0/sqlglot/{dialects,parsers,generators}/materialize.py`.
Materialize extends Postgres with `NormalizeNotNull` re-enabled, `SupportsCreateTableLike` and
`SupportsBetweenFlags` back at their base defaults, and `TYPED_LAMBDA_ARGS = True`.

Parsed end to end: the `LIST` and `MAP` constructors in both bracket and call notation
(`LIST[1, 2, 3]`, `MAP{k: v}`), named arguments through the `FARROW` (`=>`) lambda entry with
Materialize's lambda overrides (a typed single-argument head like `x INT -> ...`, and the
field-based lambda argument parser), `ON CONFLICT ... DO NOTHING`, `MZ_NOW()`, the `mz_timestamp`
cast, and `SERIAL`/`AUTO_INCREMENT` columns that render without `NOT NULL`.

Generator details: the `ToMap` rendering (the base `MAP {this}` transform is dropped so the
parenthesized form wins), the `mz_timestamp`/`mz_now` type and function output, and the Materialize
`TIME_MAPPING` and type mappings.

Corpus: 24 `validate_identity` cases from `tests/dialects/test_materialize.py`
(`scripts/extract_dialect_corpus.py --dialects materialize`), at **24/24**, with no Materialize rows
in `testdata/parity_gaps.txt`. Regression tests: `materialize_test.go`.

## RisingWave dialect — DONE (issue #36)

Ported 1:1 from `.reference/sqlglot-v30.17.0/sqlglot/{dialects,parsers,generators}/risingwave.py`.
RisingWave extends Postgres with `NormalizeNotNull`, parenthesized struct access required, struct
star expansion supported, `LockingReadsSupported = False`, `SupportsBetweenFlags` back at the base
default, unwrapped transform column constraints (`col INT AS expr` without parentheses), and
`ExpressionPrecedesPropertiesCreatables` for `CREATE SINK`.

Parsed end to end: `CREATE SINK`/`CREATE SOURCE` with their connector options, the `ENCODE`/`KEY
ENCODE`/`INCLUDE` create properties as new `EncodeProperty`/`IncludeProperty` nodes, the `WATERMARK
FOR <col> AS <expr>` column constraint at column and schema level, computed columns without
parentheses, `MAP` datatype constructors with angle-bracket or parenthesized fields, and
`MATERIALIZED`/`NOT MATERIALIZED` CTE markers; table hints are disabled so connector properties are
not swallowed as a table alias.

Generator details: the body-before-properties `CREATE` order for sinks, the `ENCODE`/`INCLUDE`
property rendering, `PROPERTIES_LOCATION` moving `FileFormatProperty` after the CREATE body, the
`watermarkcolumnconstraint` transform, and the watermark/computed-column output.

Corpus: 7 `validate_identity` cases from `tests/dialects/test_risingwave.py`
(`scripts/extract_dialect_corpus.py --dialects risingwave`), at **7/7**, with no RisingWave rows in
`testdata/parity_gaps.txt`. Regression tests: `risingwave_test.go`.

## Spark 2, Spark 3 and Databricks dialects — DONE (issue #35)

Ported 1:1 from
`.reference/sqlglot-v30.17.0/sqlglot/{dialects,parsers,generators}/{spark2,spark,databricks}.py`
plus the shared `transforms.py` preprocess helpers. The dialects form the upstream chain
`Spark2(Hive) → Spark(Spark2) → Databricks(Spark)`.

Spark 2 adds the `X'..'` hex strings, the TIMESTAMP→TIMESTAMPTZ keyword remap, the AND/OR
connector functions, the Spark join hints, APPROX_PERCENTILE, TRIM_PATTERN_FIRST,
CHANGE COLUMN ... TYPE, the DROP COLUMNS form, the PIVOT column-naming rule, and the whole
Spark2Generator transform table: FROM_UTC_TIMESTAMP/TO_UTC_TIMESTAMP, TO_TIMESTAMP, TRUNC,
AGGREGATE, FORMAT_STRING, SHIFTLEFT/SHIFTRIGHT, MAP_FROM_ARRAYS, ELEMENT_AT, the FROM_JSON/TO_JSON
casts, `USING`/`STORED AS` file formats, ALTER COLUMN rendering and the safe-bracket ELEMENT_AT.

Spark 3 adds raw strings, the DECLARE keyword, the Spark FUNCTIONS overlay, widget placeholders,
SET VAR/SET VARIABLE, non-aggregate PIVOT entries, computed GENERATED columns, DECLARE and REFRESH
statements, LISTAGG, the datasource CREATE partition fold, TRY_ELEMENT_AT, TRY_CAST and the Spark
TYPE_MAPPING.

Databricks adds STRICT_CAST, the colon VARIANT accessor with its JSONPath node support, the
STREAM/VOID keywords, the FUNCTIONS overlay for IFF/GETDATE/NOW/DATEDIFF/TO_DATE/UNIFORM, CURDATE
with optional parens, the REGR distinct-argument functions, `?::`, TIMESERIES primary keys,
CLUSTER BY AUTO/NONE, and the generator's CURRENT_VERSION/CURRENT_CATALOG/VARIANT/LISTAGG
overrides plus the RETURNS TABLE wrapping and generated-identity BIGINT rule.

Supporting base pieces: the EXPLODE/EXPLODE_OUTER, STRING, JSONPath, Stream, ClusterProperty and
EnviromentProperty nodes; the hive IF rendering; top-level CTE hoisting; faithful DECLARE
rendering; the INSERT TABLE source; LIKE/ILIKE ESCAPE; and the STRING/VOID type mappings.

Corpus: 227 `spark` and 176 `databricks` `validate_identity` cases from
`tests/dialects/test_spark.py` and `tests/dialects/test_databricks.py`
(`scripts/extract_dialect_corpus.py`), at **227/227** and **175/176**. The remaining Databricks
row is the `GET_JSON_OBJECT` path rewrite, which needs the deferred JSONPath tokenizer
(DEVIATIONS §1.27). Regression tests: `spark2_test.go`, `spark_test.go`, `databricks_test.go`.
The optimizer fixtures that do not pass yet are recorded in `optimizeGaps`.

## BigQuery dialect — DONE (issue #41)

Ported 1:1 from
`.reference/sqlglot-v30.17.0/sqlglot/{dialects,parsers,generators}/bigquery.py`: the camel-case and
`SUPPORTS_USER_DEFINED_TYPES=False` flags, the backtick/`"""` tokenizer with `0x` hex strings,
`CURRENT_DATETIME`, and the TIMESTAMP→TIMESTAMPTZ / DATETIME / BYTES keyword remaps.

Parser: dashed and numeric table names, the wildcard table suffix (`x.*`, `x.y*`), quoted-path
table/column splitting with `quoted_table`/`quoted_column`, INFORMATION_SCHEMA view merging,
`OPTIONS (...)` properties, so the generated output is `OPTIONS` rather than `WITH`, the no-paren CSV
`CLUSTER BY` property, `NOT DETERMINISTIC`, the SAFE./NET. qualified-call re-parse, AI./ML. custom
signatures (GAP_FILL, FORECAST, VECTOR_SEARCH, GENERATE_*), EXPORT DATA, FOR ... DO, LOAD DATA ...
FROM FILES, `SELECT AS STRUCT`, set-op `BY NAME`/`CORRESPONDING`/`ON`, `UNNEST ... WITH OFFSET`, the
`HAVING MAX`/`HAVING MIN` aggregate modifier, and CAST `FORMAT`/`AT TIME ZONE`.

Generator: the INTERVAL forms (`DATE_ADD(x, INTERVAL '1' DAY)`), `TO_HEX`/`LOWER(TO_HEX(·))`,
`LOG` base ordering, `STRING_AGG`, sorted `JSON_OBJECT` pairs with `,`, the `_alias_ordered_group`
GROUP BY alias rewrite, `IGNORE NULLS` inside aggregates, UNPIVOT alias literals,
`IGNORE_NULLS_IN_FUNC`, `EXCEPT/INTERSECT ... DISTINCT` always explicit, `RENAME TO` without the
dataset, precision stripping on CAST, the reserved-keyword quoting set, and the BigQuery type map.

Corpus: 689 `validate_identity` cases from `tests/dialects/test_bigquery.py`
(`scripts/extract_dialect_corpus.py --dialects bigquery`), at **683/689**. The six open rows are
JSON-path rewrites (e.g. `JSON_EXTRACT(doc, '$. a b c .d')` → `$[' a b c '].d`), which need the
deferred JSONPath tokenizer (DEVIATIONS §1.27). Regression tests: `bigquery_test.go`. The newly
running annotate fixtures that do not pass yet are recorded in `optimizer/testdata/annotate_gaps.txt`.

## DuckDB dialect — DONE (issue #39)

Ported from `.reference/sqlglot-v30.17.0/sqlglot/{dialects,parsers,generators}/duckdb.py`: the
case-insensitive identifier strategy, `e'...'` byte strings, `$` heredocs and parameters, `//`/`**`/
`^@`/`@>`/`<@` operators, the `ATTACH`/`DETACH`/`INSTALL`/`FORCE`/`SUMMARIZE`/`PIVOT`/`UNPIVOT`
statement parsers, prefix aliases (`foo: 1`, `FROM foo: t`), `LAMBDA x : body`, `#N` positional
columns, `MAP` constructors, `SET VARIABLE`, the simplified `PIVOT ... ON ... USING ...` and
`UNPIVOT ... INTO NAME ... VALUE ...` forms, leading `FROM` selects, and the `EXCLUDE` star modifier.

Generator: the `DECIMAL(18, 3)` default and TEXT precision strip, `STRUCT(...)`/`MAP(...)`/`T[n]`
type rendering, the interval binary-op canonicalization (`CAST(...) + INTERVAL '1' DAY`),
`LIST_SORT`/`ARRAY_REVERSE_SORT`, `LIST_FILTER`/`LIST_TRANSFORM`, `LISTAGG`, `STR_SPLIT`,
`REGEXP_EXTRACT` argument order, `SHA256`, `POWER`/`REGEXP_FULL_MATCH`/`STARTS_WITH`/`XOR` operator
functions, `USING SAMPLE ... REPEATABLE`, `COPY` without `INTO`, JSON arrow extraction with
parenthesization, `SET VARIABLE name = value`, and the bit-aggregate integer coercion.

Corpus: 565 `validate_identity` cases from `tests/dialects/test_duckdb.py`, at **528/565**. The
remaining rows are tracked in `testdata/parity_gaps.txt`: `COLUMNS(*)` star expansion, `VALUES`
query modifiers, multi-clause `CREATE MACRO`, list comprehensions, `AT (VERSION => ...)` table
versions, `USING KEY`, JSON-path canonicalization, map-literal keys, and a few DDL property
rewrites. Newly active optimizer fixture cases are recorded in
`optimizer/testdata/annotate_gaps.txt`. Regression tests: `duckdb_test.go`.


## Snowflake dialect — PARTIAL (issue #40)

Ported the identity-relevant core of
`.reference/sqlglot-v30.17.0/sqlglot/{dialects,parsers,generators}/snowflake.py`: the uppercase
identifier normalization, `x'...'` hex strings, `$$` raw strings, `//` comments, `$` parameters,
the Snowflake keyword remaps (FLOAT→DOUBLE, TIMESTAMP_TZ, MINUS→EXCEPT, SAMPLE, …), `DIRECTED`
joins, `UNDROP`, the `DATEADD`/`DATEDIFF`/`TIMESTAMPADD` argument order, `DATE_PART`/`EXTRACT`
normalization, `TO_DATE`/`IFF`/`LISTAGG`, array constructors and list functions, bitwise/aggregate
function spellings, the Snowflake type map (VARIANT/OBJECT/VARCHAR/TIMESTAMPNTZ), `STRUCT(...)` /
`MAP(...)` / `ARRAY(...)` type delimiters, `OBJECT_CONSTRUCT`/`OBJECT_KEYS`, the `$` parameter token,
and `COPY` without a `WITH` wrapper.

Corpus: 1543 `validate_identity` cases from `tests/dialects/test_snowflake.py`, at **1209/1543**. The
remaining rows are tracked in `testdata/parity_gaps.txt`: stage paths and `GET`/`PUT`, `AT`/`BEFORE`
time travel, `SEMANTIC_VIEW`, `MATCH_RECOGNIZE`/`MATCH_CONDITION`, `IDENTIFIER` calls, JSON colon
paths and `GET_PATH`/`FLATTEN` implicit columns, object/map literal keys, COPY parameter metadata,
ALTER TABLE property statements, and the remaining DDL describe qualifiers. Newly active optimizer
fixtures are recorded in `optimizeGaps`, `simplify_gaps.txt`, and `annotate_gaps.txt`. Regression
tests: `snowflake_test.go`.

## Star expansion through PIVOT / UNPIVOT — DONE (issue #46)

`SELECT *` over a pivoted or unpivoted source now expands to the operator's output columns instead
of the pre-pivot columns, matching upstream `qualify` (`_expand_stars` + `Scope.pivots`,
qualify_columns.py:843-1035):

- `expandStarsInScope` resolves the chain's final alias (the `(UN)PIVOT ... AS _0` name), which is
  not registered in `scope.sources`, through the chain's parent, then folds each operator with
  `Pivot.output_columns` (expressions/query.py:1780): the pre-pivot columns minus the consumed
  ones, plus the operator's outputs — honoring `value_columns_first` (T-SQL), the dialect's
  inferred `columns`, and the alias's positional renames. `qualifyColumnsInScope` validates the
  projections against the post-pivot set (upstream's `single_chain`/`produced`/`available` block).
- `Resolver.GetSourceColumns` zips a table alias's column list over the source columns
  `zip_longest`-style (resolver.py:217), so `m_sales AS m_sales(empid, dept, jan, feb)` exposes its
  alias columns even when the table is absent from the schema.
- Parser: the inferred PIVOT output columns honor `IDENTIFY_PIVOT_STRINGS` (snowflake: the IN-list
  literal `'x'` names the column `'x'`) and `PREFIXED_PIVOT_COLUMNS` (bigquery: the aggregation
  alias leads, `sum_x` not `x_sum`), and all three naming flags are recorded on the Pivot node
  (parser.py:5463-5506).
- Generators: snowflake unqualifies a UNPIVOT's columns deeply and a PIVOT's `ANY ORDER BY` field
  only (`_unqualify_pivot_columns`), while duckdb unqualifies the whole operator — DuckDB only
  accepts simple column names inside a (UN)PIVOT; `SetOperation.named_selects` honors `BY NAME`
  so DuckDB's `UNION BY NAME` fixtures expand. `TestPivotAnnotation` now runs raw `SELECT *`
  through qualify + annotate_types under snowflake/duckdb exactly like upstream
  (tests/test_optimizer.py:869-955).

The `optimizeGaps` and `qualifyColumnsInvalidGaps` #46 rows are cleared; the fixture harness now
runs duckdb and snowflake (`dialectInScope`), deferring only the unrelated snowflake `CONNECT BY
LEVEL` and `sqlglot.meta case_sensitive` cases. Not ported (cross-dialect only): the generator's
`_pivot_in_value_aliases` rewrite (generator.py:2524), which renames stored pivot columns when the
write dialect names them differently — same-dialect output is unaffected.

## Cross-dialect transpile corpus — DONE (issue #33)

Reading one dialect and writing another is now verified against the oracle upstream already keeps
for it: `validate_all(sql, read={...}, write={...})` (tests/dialects/test_dialect.py:74-113), where
each read pair asserts `parse_one(read_sql, read_dialect).sql(self.dialect) == sql` and each write
pair asserts `expression.sql(write_dialect) == write_sql`.

- **Extraction** — `scripts/extract_dialect_corpus.py --transpile-only` records every pair whose
  dialects are all registered into `testdata/transpile_corpus.jsonl`: **8497 records across 284
  dialect pairs** (6427 cross-dialect, 2070 same-dialect), deduped and sorted for a byte-stable
  rerun (verified: two consecutive runs are identical). `REGISTERED` mirrors `dialects.GetOrRaise`,
  so it also holds `spark2` — which upstream's test classes only ever name as a read/write key,
  111 of them, with no `TestSpark2` class of its own to run — and the `mariadb` alias, though
  upstream ships no MariaDB dialect and yields zero keys. Unregistered-dialect pairs and
  `UnsupportedError` wants are skipped. The issue's ~1.1k figure counted `validate_all` call sites
  in 12 test files; the extractor runs every registered dialect's test class plus `TestDialect`
  (base, `dialect = None` → `""`) and records each read/write *entry*, which reconciles to the
  larger pair count. The new `--transpile-only` flag leaves `testdata/dialect_identity.jsonl`
  byte-identical: a plain full run rewrites every identity section and would drop the hand-added
  MySQL/Postgres/base records that per-dialect `--dialects` runs preserve verbatim. Four
  log-asserting methods are allowlisted in `KNOWN_FAILURES`, each verified to fail at/after its
  last capture call, so no record is silently lost: `TestPostgres.test_array_offset`,
  `TestClickhouse.test_array_offset`, `TestDuckDB.test_array_index`, `TestBigQuery.test_warnings`.
- **Harness** — `transpile_corpus_test.go` (`TestTranspileCorpus`) mirrors `corpus_test.go`: parse
  under `read`, generate under `write`, and require (a) every failure be listed in
  `testdata/transpile_gaps.txt`, (b) per-write-dialect pass floors (bucketed by the generation
  target, where the TRANSFORMS/TYPE_MAPPING gaps live; read-side parse gaps show up with a
  `parse:` category), (c) per-write-dialect total floors against dropped records. Regenerate the
  gaps file and print observed counts with
  `SQLGLOT_TRANSPILE_UPDATE=1 go test . -run TestTranspileCorpus -v`.
- **Initial state** — **5854/8497 pass (69%)**, 2643 tracked gaps: 1743 `gen: transpile mismatch`,
  790 `gen: cast/type`, 67 `gen: ddl/properties`, 43 parse/generate errors. Closing them slice by
  slice is the per-dialect generator work listed under Remaining work.
- **Bug found by the first run** — `SELECT e'Hello\nworld'` (duckdb → bigquery) crashed the test
  binary with a stack overflow: `bytestringSQL`'s cast-wrap branch rebuilt its child ByteString
  without the `is_bytes` flag upstream's `maybe_parse` carries (parser.py:1223,
  `is_bytes = BYTE_STRING_IS_BYTES_TYPE or None`), so BigQuery's `CAST(b'...' AS STRING)` re-entered
  the branch forever. Fixed in `generator/residual_tail.go` by mirroring the parser's flag
  (parser.go:2847) on the fresh node — no output change on any previously passing case.
- **Docs** — README moves cross-dialect transpilation out of "Not ported (yet)" into the ported
  list, with the verified pair counts and the gaps file as the burndown record.
