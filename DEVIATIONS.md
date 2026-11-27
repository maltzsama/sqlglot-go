# Deviations from upstream sqlglot

Ledger of every place sqlglot-go *intentionally* differs from pinned upstream (v30.17.0), so an
upstream bump or differential check can tell an intended divergence from a regression. One entry per
deviation: upstream's behavior → ours → the guarding test. Reasoning lives in the code comments
(grep `divergence` / `Unlike upstream`), commit history, and `ROADMAP.md`'s resolved-findings ledger.

Only **§1 changes same-dialect parse→generate output**; everything else is cross-dialect-only,
output-preserving, a not-yet-ported boundary, or a Go-only API extension. Grammar the port accepts
beyond upstream is additionally registered in `testdata/upstream_extensions.jsonl` (tripwired by
`TestUpstreamExtensionsTripwire`).

---

## 1. Behavioral deviations (same input → different output than upstream)

Each fixes a place upstream disagrees with the real engine; the port matches the engine.

### 1.1 ASCII-only identifier case-folding
Upstream folds unquoted identifiers full-Unicode (`str.lower()`): `CAFÉ` → `café`. Real engines fold
ASCII-only on multibyte encodings (PG `downcase_identifier`), so the port folds only `A-Z`↔`a-z`:
`CAFÉ` → `cafÉ`. Applies to every folding strategy. `dialects/dialect.go`;
test `identifier_casefold_test.go`.

### 1.2 MySQL-exact identifier folding (opt-in strategies)
Upstream's MySQL is `CASE_SENSITIVE` (folds nothing) — unchanged by default. Two opt-in
`NormalizationStrategy` values model real MySQL resolution:
- `MySQLCaseInsensitive` (lctn=1/2): folds everything with MySQL's `utf8mb3_general_ci` map
  (`dialects.MySQLLower`, generated table in `dialects/mysql_casefold_table.go` — accent-preserving,
  not Go's `strings.ToLower`).
- `MySQLCaseSensitiveTableNames` (lctn=0): role-aware — table/db names, table aliases, CTE names, and
  column qualifiers stay case-sensitive; column names fold. `information_schema` relations fold
  regardless (MySQL matches that schema case-insensitively).

`schema.NewMappingSchema(normalize=true)` normalizes keys role-aware and **fails closed** on two raw
keys folding to one normalized key (upstream `nested_set` is last-wins) — every folding dialect.
Tests: `optimizer/qualify_tables_mysql_test.go`, `schema/` tests.

### 1.3 Dialect settings string + DialectType polymorphism
`dialects.GetOrRaise` accepts upstream's settings-string form (`"mysql, normalization_strategy=…"`)
and the dialect-accepting entry points take `nil | string | *Dialect`, mirroring upstream
`DialectType`. Gap-closure, not a divergence. Only `normalization_strategy` (plus the Go-only
`mysql_version`/`mysql_ansi_quotes`) is supported; upstream's `version` is not.
Athena's Hive/Trino sub-parsers inherit the outer dialect's `opaque_functions` and
`normalization_strategy` (upstream constructs bare sub-dialects).

### 1.4 MySQL `--` comment requires trailing space
Upstream comments out `SELECT 1--2` in every dialect. MySQL requires ASCII whitespace/control (or
EOF) after `--`, so under mysql the port tokenizes `1--2` as `1 - -2`. Postgres/base unchanged.
Test `tokenizer_mysql_comment_test.go`.

### 1.5 MySQL executable comment activation (opt-in `mysql_version`)
Default matches upstream: `/*! … */` bodies stay comments. With `mysql_version=<MYSQL_VERSION_ID>`
set, a bare `/*! … */` body — and a gated `/*!NNNNN … */` body when version ≥ gate — is tokenized as
SQL. Activation is semantic, not byte-preserving (`SELECT 1 /*!50000 + 100 */` regenerates as
`SELECT 1 + 100`). Optimizer hints `/*+ … */` stay comments. Test `mysql_version_comment_test.go`.

### 1.6 MySQL `RESET …` → `Command`
Upstream parses `RESET MASTER` as `Alias(RESET AS MASTER)`. The port degrades all MySQL `RESET …` to
`exp.Command`. `reset` as an identifier unaffected. `dialects/mysql.go`.

### 1.7 Postgres `U&'…'` / `U&"…"` decoded
Upstream mis-tokenizes `U&'\0067'` as `U & '…'` and errors on the identifier form. The port decodes
the SQL-standard escapes into an ordinary `Literal`/`Identifier`. A custom `UESCAPE 'c'` clause fails
closed. Ledger `pg-unicode-identifier`; tests `unicode_escape_test.go`, `tokens/unicode_escape_test.go`.

### 1.8 `SAVEPOINT` / `RELEASE SAVEPOINT` → `Savepoint`
Upstream parses `SAVEPOINT s` as an `Alias` and errors on `RELEASE SAVEPOINT s`. The port builds
`exp.Savepoint{this, kind}`; bare `RELEASE <name>` (no keyword) is Postgres-only. Output normalizes
release to `RELEASE SAVEPOINT <name>`. Ledger `release-savepoint`; test `savepoint_test.go`.

### 1.9 Postgres/MySQL reject a bare string as table name/alias
Upstream folds `FROM 'foo'` and `FROM t 'x'` into identifiers in every dialect; real PG/MySQL reject
both. Gated by `Dialect.StringTableIdentifiers` (default true = upstream; false for postgres/mysql —
parse error, or `Command` inside tryParse'd DDL). Explicit `AS '…'` table alias is still accepted
(matches upstream; a pre-existing quirk). Test `parser/string_aliases_test.go`.

### 1.10 Postgres `pg_catalog.<builtin>` resolves to the builtin
Upstream leaves `pg_catalog.int4` USER-DEFINED. For postgres, a two-part `pg_catalog.<name>` whose
tail is in a pinned allowlist of real pg_catalog type names resolves to the same node as the bare
spelling (`pg_catalog.int4` → `INT`), across `::`/CAST/space-typed-literal spellings. Aliases PG
rejects (`pg_catalog.integer`), other schemas, `pg_catalog.char`/`bit` (bare keyword is a different
type), and modifier-bearing `oid(5)`-style names stay USER-DEFINED. `parser/parser_types.go`;
test `TestParsePgCatalogBuiltinType`.

### 1.11 MySQL `ANSI_QUOTES` (opt-in `mysql_ansi_quotes`)
Upstream has no support; default MySQL treats `"…"` as a string. With `mysql_ansi_quotes=true`,
`"…"` is a quoted identifier. Parse-only; the generator already emits `'`/backtick, valid in both
modes. Test `mysql_ansi_quotes_test.go`.

### 1.12 Postgres `SET var = "quoted"` keeps the quotes
Upstream rewrites a quoted-identifier SET value to a bare `Var` (`SET search_path = "$user"` →
`= $user`, invalid PG). The port keeps a quoted value verbatim. `parser/stmt_set.go`;
test `TestPostgresSetMultiValue`.

### 1.13 MySQL `START REPLICA|SLAVE|GROUP_REPLICATION` → `Command`
Upstream parses these as `Transaction{modes:[REPLICA]}` (renders invalid `BEGIN REPLICA`). The port
degrades them to `Command` at top level, MySQL-only. `parser/stmt_transaction.go`;
tests `TestMySQLStartReplicationIsNotTransaction`, `TestStartReplicationDivertIsMySQLOnly`.

### 1.14 MySQL admin command leaders → `Command`
Upstream mis-parses `STOP …`/`FLUSH …`/`UNLOCK INSTANCE`/`XA …`/`BINLOG`/`HELP`/`RESTART`/`SHUTDOWN`
as `Alias`/`Column` (some `XA` forms error). The port degrades the whole statement to `Command` —
top-level only, so the words stay usable as identifiers and nested values.
`parser/stmt_mysql_command.go`; tests `TestMySQLCommandLeaders*`.

### 1.15 `TABLE tbl` → `SELECT * FROM tbl` in MySQL/MariaDB and Trino
Upstream parses `TABLE users` as an `Alias` in every dialect. The port builds the equivalent
`Select` only where the grammar defines the constructor — MySQL 8.0.19+ and MariaDB through the
mysql dialect chain, plus Trino's `TABLE table_name` query — top-level, plain table identifier
only; trailer forms (`ORDER BY`/`UNION`/…) fail closed. Every other dialect keeps upstream's
`Alias` mis-parse (and its parse error for the qualified form), so the port stays faithful.
Qualified `TABLE db.users` (MySQL/MariaDB) and `TABLE cat.schema.table` (Trino) are grammar
beyond upstream and ledgered (`mysql-table-value`, `trino-table-value`).
`parser/stmt_mysql_table.go`; tests `TestMySQLTableStatement*`,
`TestTableStatementQualifiedAcrossDialects`.

### 1.16 MySQL `DROP INDEX idx ON db.users`
Upstream errors on the db-qualified target (single-id `_parse_on_property`). The port parses the ON
target with `parseTableParts` into `OnProperty{this: Table}` — the qualifier survives; the target is
a `Table`, not upstream's bare `Identifier`. Ledger `mysql-drop-index-on-qualified`;
test `TestParseDropIndexOnTable`.

### 1.17 Postgres dollar-quote tag preserved
Upstream drops a named heredoc tag on parse and always regenerates `$$…$$`; real PostgreSQL rejects
that whenever the body itself contains `$$` (the reason a named tag exists). The port keeps the tag
on `Heredoc.tag` (`$fn$…$fn$` round-trips verbatim). `parser_ddl.go` `heredocTag`;
test `TestHeredocTagPreserved`.

### 1.18 MySQL compound-statement grammar; routine bodies are one statement
Upstream parses no procedural grammar: `DECLARE` mangles into an Alias, a routine's
`END; SELECT 2` folds the trailing statement into the routine's Block, and truncated
definitions leave dangling `EndStatement` fragments. This port parses MySQL's compound
statements STRUCTURALLY (`parser/mysql_compound.go`, reference manual §15.6, verified on real
MySQL 8.0.46): `BEGIN…END`, `DECLARE` variables (exp.Declare)/handlers/conditions/cursors,
`IF…THEN…ELSEIF…ELSE…END IF` (exp.IfBlock), `CASE…WHEN…END CASE` (exp.CaseBlock),
`LOOP`/`REPEAT…UNTIL`/`WHILE…DO` (exp.LoopBlock/RepeatBlock/WhileBlock), statement labels,
ITERATE/LEAVE/DO, SIGNAL/RESIGNAL/GET DIAGNOSTICS, and routine parameter modes
(`IN`/`OUT`/`INOUT`). Grammar extension (upstream parses none of it).

Consequences, all engine-verified:
- Every routine definition is exactly ONE statement; trailing batch statements are separate.
  PG `BEGIN ATOMIC` bodies parse structurally too (`begin="ATOMIC"` on the Create).
- Terminators match exactly: `END IF` against a BEGIN, a mismatched `END <kw>`, an
  unterminated body, and any MySQL top-level statement leading with `END` are parse errors
  (real MySQL 1064) — never truncated fragments or silently merged statements.
- A bare top-level `ELSE` chunk is a parse error (upstream silently DROPS all later
  statements). Under postgres a bare `END` chunk is COMMIT wherever it appears (real PG 16).
- Every nonempty body statement requires its terminating `;` before the block's `END`
  (`BEGIN SELECT 1 END` is real MySQL 1064). With the `;` present, an unquoted identifier
  spelled `end` is an ordinary column reference (`SET @a = end;`), matching the engine.
- The merge invariant (a statement following a routine is never inside the routine's span)
  is enforced by `TestBlockExtentInvariantFuzz` (3000 generated adversarial bodies; fails on
  v0.28.3, which had a live WHILE merge).
Tests `procedure_block_test.go`, `statement_span_test.go`, `block_stack_fuzz_test.go`.

---

### 1.20 CONCAT_WS parenthesizes compound NULL checks
Upstream renders the NULL guard of a NULL-skipping CONCAT_WS as `arg IS NULL` with the argument
unwrapped, so `CONCAT_WS(',', NOT TRUE, 'x')` becomes `NOT TRUE IS NULL`. Trino parses that as
`NOT (TRUE IS NULL)`, which is TRUE, so the guard wrongly returns NULL for a non-NULL argument. The
port wraps a compound argument in parentheses: `(NOT TRUE) IS NULL`. Applies to dialects with
CONCAT_WS_COALESCE (Trino, Postgres, DuckDB, Hive, T-SQL, ...). `generator/concat_ws.go`;
test `concat_ws_guard_test.go`.

### 1.19 Athena folds quoted identifiers

Upstream's outer `Athena` dialect inherits base `LOWERCASE` (unquoted folds, quoted verbatim), an
oversight: Athena is Trino, and Trino folds every identifier, quoted or not (`presto.py:35`
`CASE_INSENSITIVE`). `dialects.Athena()` sets `CaseInsensitive`, so `"MyCol"` and `mycol` are one
column under Qualify and `FoldIdentifierName`, matching the Athena docs (names are
case-insensitive and stored lowercase). Athena also folds full-Unicode (Trino's Java
`toLowerCase`), so §1.1's ASCII-only rule does not apply to it: `"CAFÉ"` → `café`.

### 1.20 Athena routes each statement of a batch separately

Upstream (`athena.py:89-111`) classifies the whole input once and re-tokenizes all of it as Hive or
Trino, so `SHOW TABLES; SELECT "$path" FROM t` turns `"$path"` into a string literal. The port
splits the classifier stream on `;` and routes/re-tokenizes per statement, offsetting token
positions so spans stay exact. Single statements tokenize as upstream does, except the ledgered
`athena-*` grammar extensions below, which need real tokens where upstream packs a raw tail.

### 1.21 PostgreSQL rejects `UNNEST ... WITH OFFSET`

Upstream's shared `_parse_unnest` (`parser.py:5209`) accepts BigQuery's `WITH OFFSET [AS n]` in every
dialect and regenerates it as `WITH ORDINALITY` (folding the name into the alias columns, or dropping
it when there is no alias). PostgreSQL only has
`WITH ORDINALITY`, so under postgres the port raises a parse error. Other dialects unchanged. The
AST cannot tell a parsed `WITH OFFSET` from `WITH ORDINALITY AS u(v, n)`, whose surplus alias column
upstream pops into the same `Identifier` offset, so the check is parser-only. Test
`parser/postgres_unnest_test.go`.

### 1.22 T-SQL `GO` batch separator is a statement boundary

Upstream lexes `GO` as a Command keyword, so `SELECT 1\nGO` aliases the Select as `GO`,
`SELECT 1;\nGO\nSELECT 2` hides the second batch inside a `Command "GO\nSELECT 2"`, and
`SELECT 1\nGO\nSELECT 2` fails to parse. sqlcmd and SSMS strip a line holding only `GO [count]`
and end the batch there, which is what a script splitter has to match. Under `tsql` the tokenizer
(`TokenizerConfig.BatchSeparator`) lexes such a line, case-insensitive with surrounding spaces/tabs,
an optional repeat count, and an optional trailing `--` comment, as a `SEMICOLON` token whose
`Text` is the separator (`GO 5`). The token is emitted for every separator line — including after a
`;` or another separator — so a token consumer can locate each boundary; the chunk builder drops
the empty statement slot such a separator would otherwise produce. Every consumer then splits on it
like `;`, and a Command's packed tail stops before it. The separator's trailing `--` comment is
stripped with the line instead of surfacing as an orphan `Semicolon` statement, and the separator
also terminates a routine body: `CREATE PROCEDURE p AS SELECT 1` followed by GO keeps the Create
and gives the next batch's statement back as its own statement (a plain `;` still folds into the
body, as upstream's `_parse_block` does). The repeat count is not modeled. `GO` inside a string, a
block comment, or sharing its line with other SQL is unchanged. An input holding nothing but the
separator line keeps upstream's `Command "GO"`, because there is no batch to separate.
Tests `TestTSQLGoBatchSeparator`, `TestTSQLGoBatchSeparatorToken` (`tsql_test.go`).

### 1.23 Oracle `/` terminator line is a statement boundary

SQL*Plus and SQLcl treat a line holding only `/` as "run the buffer", which ends the statement (and
is the only terminator for a PL/SQL unit). Upstream lexes it as division, so
`SELECT 1 FROM dual\n/\nSELECT 2 FROM dual` fails to parse and `SELECT 1 FROM dual;\n/` fails with
`'this' missing for Div`. Under `oracle` the tokenizer lexes such a line (surrounding spaces/tabs
allowed) as a `SEMICOLON` token with `Text` `/`, through the same `TokenizerConfig.BatchSeparator`
mechanism as §1.22: the token is emitted for every separator line — after a `;` too — so a token
consumer can find the boundary, the chunk builder drops the empty statement slot, and it splits
like `;`. A `/` that shares its line with anything else, including a comment, stays division.
Test `TestOracleSlashTerminator` (`oracle_test.go`).

### 1.24 Oracle PL/SQL units are one statement

Upstream splits every input on `;`, so a PL/SQL unit is cut at the first `;` inside its body:
`CREATE OR REPLACE FUNCTION f RETURN NUMBER IS BEGIN RETURN 1; END;` becomes a truncated `Command`
plus a stray `EndStatement`, anonymous `BEGIN ... END;` / `DECLARE ... END;` blocks fail to parse,
and a `CREATE PROCEDURE` body is mangled (`IS BEGIN NULL; END` regenerates as `AS IS AS BEGIN; END`,
dropping the body). Oracle compiles the whole unit as one statement. Under `oracle`, the parser's
`;` chunking (`splitOracleChunks`, `parser/dialect_oracle_plsql.go`) keeps a unit in one chunk: a
stored `CREATE [OR REPLACE] [EDITIONABLE|NONEDITIONABLE] {PROCEDURE|FUNCTION|PACKAGE [BODY]|TRIGGER|
TYPE BODY}` or an anonymous block runs until its block nesting closes (`BEGIN`/`DECLARE`, a
subprogram or package `IS|AS`, a compound trigger, and `CASE`/`IF`/`LOOP` open a level; `END [IF|
LOOP|CASE] [name]` closes one) and the next `;`, or a `/` line (§1.23), ends it. Strings and quoted
identifiers never count. The body is not modeled: a stored unit becomes `Create{kind, this:
Command}` with the verbatim text (`kind` is `PROCEDURE`, `PACKAGE BODY`, ...), and an anonymous
block becomes `Command{this: BEGIN|DECLARE}`. A unit still open at the end of input or at a `/`
line fails closed with `Unterminated PL/SQL block`. Ledger rows `oracle-plsql-*`; test
`TestOraclePLSQLUnitBoundaries` (`oracle_test.go`).

### 1.25 `merge_subqueries` keeps an alias column list when it renames a source

When an inner source's alias conflicts with an outer one, upstream `_rename_inner_sources`
(`optimizer/merge_subqueries.py:353-358`) sets a fresh `TableAlias` with only the new name, dropping
the alias column list. `SELECT s.x FROM (SELECT q.x FROM t1 AS q(x, y)) AS s JOIN t2 AS q ON ...`
merges into `... FROM t1 AS q_2 JOIN t2 AS q ON q_2.x = q.a`, which references a column `t1` doesn't
have. The port renames only the alias name, so the result keeps `t1 AS q_2(x, y)` and stays valid.
Test `TestMergeSubqueriesKeepsAliasColumns` (`optimizer/merge_subqueries_test.go`).

### 1.26 `unnest_subqueries` skips a scalar set-operation subquery it can't place

Upstream `unnest` (`optimizer/unnest_subqueries.py:61-100`) rewraps a set-operation subquery in a
new `SELECT`, which has no parent. When the scalar path then takes the `MAX` branch (a grouped or
aggregating outer query, or a `HAVING` predicate), it calls `select.parent.replace` on `None` and
raises `AttributeError`, e.g. for `SELECT x.a + (SELECT 1 UNION ALL SELECT 2) FROM x GROUP BY x.a`.
The port returns without rewriting, leaving the query unchanged. Test
`TestUnnestSubqueriesSetOperationScalar` (`optimizer/unnest_subqueries_test.go`).

## Opt-in behavioral extensions beyond upstream

Additive analysis features; default behavior and fixture output unchanged.

- **Search-path table qualification** (`QualifyOpts.SearchPath`): resolves the schema part by proven
  existence against the supplied schema, in order; no fallback to `DefaultSchema`, no catalog stamp,
  unproven stays unqualified (fail-closed). Names fold role-aware (§1.2). Empty path = upstream path.
- **Qualify resolution report** (`QualifyOpts.ResolutionReport` → `SourceKind` per source): exposes
  the source classification the qualify scope pass already computes. `Unresolved` is the zero value.
  DML roots populate from the §6.1 analysis traversal. A SEMI or ANTI JOIN source, read but never
  selected, is classified from the scope's sources like a selected one.
- **Opaque function calls** (`opaque_functions` dialect setting, resolve-time like
  `mysql_ansi_quotes`): name-lookup builtin calls (FunctionByName + dialect overlays + convenience
  functionParsers + `IF(`) parse as `Anonymous` with the name as written; the generator renders
  Anonymous names verbatim (no upcasing) so parse∘generate is idempotent. The call *form* decides,
  not the name: keyword-grammar forms (CAST, `EXTRACT(f FROM x)`, `TRIM(BOTH … FROM …)`,
  `SUBSTRING(x FROM … FOR …)`, `POSITION(a IN b)`, `CHAR(… USING cs)`, `CEIL(x TO u)`) keep their
  structured nodes and render form-faithfully (never the STR_POSITION/CHR/comma↔FROM-FOR
  canonicalizations); comma forms of the same names go opaque. Bare no-paren niladics
  (`CURRENT_DATE`, …) stay structured. Tests `opaque_functions_test.go`.
- **Implicit (system) columns** (`schema.NewMappingSchemaWithImplicit`): marks in-mapping columns
  as engine-implicit (PG `ctid`/`xmin`/`xmax`/`cmin`/`cmax`/`tableoid`), populating the upstream
  `visible` mechanism as the marking's complement. Explicit refs resolve/lineage like any column;
  implicit columns are excluded from `SELECT *`/`t.*` (upstream-ported onlyVisible path) and from
  NATURAL JOIN / USING candidate sets (`expandUsing`/`orderedColumnNames` now pass onlyVisible —
  upstream passes no filter there, but upstream also cannot mark a column implicit; engine-verified:
  PG system columns are not part of the row type). Derived scopes no longer carry them via `*`, so
  an outer `ctid` over a subquery fails resolution exactly like the engine. Markings for unknown
  tables/columns fail closed at construction. Tests `schema/implicit_columns_test.go`,
  `optimizer/implicit_columns_test.go`.
- **Engine identity resolution** (`QualifyOpts.EngineCatalog` + `CallReport`/`RelationReport`):
  classifies each `Anonymous` call as Builtin|UDF|Unknown and each table as
  SystemRelation|UserRelation|Unknown with a canonical `schema.name` identity, per the verified
  engine algorithms (MySQL native-priority + current-database scoping; PG search_path with
  pg_catalog implicitly first unless explicitly listed; Athena `catalog.database.table` with the
  consumer-supplied `DefaultCatalog` + `CurrentDatabase`, calls a single global builtin set with a
  bare-name identity, qualified calls Unknown; ambiguity fails closed to Unknown). The
  catalog is a pure consumer input introspected from the live target — no name sets ship with
  sqlglot-go — and resolution is report-only: the AST is never rewritten. Tests
  `optimizer/engine_catalog_test.go`.
- **MariaDB dialect** (`mariadb`): upstream has no MariaDB dialect and raises `unknown dialect`.
  The port gives it its own identity (`Name` is `mariadb`) that keeps every MySQL behavior —
  the parser and generator override chains treat `mariadb` as a child of `mysql`, and the
  ~60 former `dialect.Name == "mysql"` checks are family checks, so MySQL grammar, settings
  (`mysql_version`, `mysql_ansi_quotes`), `REPLACE`, `SHOW CREATE`, `||`-as-OR and the rest
  apply unchanged. MariaDB-only grammar is gated on the dialect:
  `NEXT VALUE FOR <seq>` / `PREVIOUS VALUE FOR <seq>` (the latter a new `PrevValueFor` node,
  since upstream has none) and the `/*M! ... */` / `/*M!100100 ... */` executable comment,
  activated alongside `/*!` by the `mysql_version` setting. MariaDB's lexer semantics apply:
  either marker reads five gate digits plus an optional sixth, and MySQL-style comments gated
  to 50700-99999 are ignored (MariaDB's own numbering starts at 100000). The grammar
  extensions are tracked in `testdata/upstream_extensions.jsonl`.
  Engine differences beyond plain inheritance: `XOR` binds between `AND` and `OR` (MySQL keeps
  upstream's grouping), `SHOW SLAVE HOSTS` keeps its MariaDB spelling instead of MySQL's
  `SHOW REPLICAS`, and `MEMBER OF` renders as MariaDB's `JSON_CONTAINS(array, value)` since
  MariaDB only gains `MEMBER OF` in 13.3. Engine identity resolution applies the MySQL rules.
  Tests `mariadb_test.go`, dialect identity in `dialects/dialecttype_test.go`, engine catalog
  in `optimizer/engine_catalog_test.go`.

---

## Grammar extensions beyond upstream

Constructs upstream Commands or parse-errors that this port structures. Each is registered in
`testdata/upstream_extensions.jsonl` by the id below (the ledger row records upstream's fallback and
reconciliation note); malformed/engine-invalid forms fail closed to `Command` or a parse error.

| ledger id | construct → node |
|---|---|
| `mysql-table-value`, `trino-table-value` | Qualified `TABLE db.users` (MySQL/MariaDB) and `TABLE cat.schema.table` (Trino) → `Select{Table}` (§1.15); the unqualified form is the same-dialect correctness fix, these are the beyond-upstream qualified spellings |
| `oracle-qquote` | Oracle `q'X...X'` / `nq'...'` alternative string literals → `Literal` with the verbatim payload (no escape processing); the generator re-emits a standard escaped single-quoted string |
| `mysql-explain-modifier-table`, `pg-explain-table` | `TABLE t` as an EXPLAIN target with modifiers (MySQL manual §15.8.2 explainable_stmt; PG accepts the SELECT-shorthand — both engine-verified). All EXPLAIN forms share the `Describe` node. Discriminators differ per dialect: MySQL → `Describe{kind:"TABLE", this:Table}`; PG → `Describe{kind:"EXPLAIN"}` with `this.Kind()==Table` (kind stays the EXPLAIN label — key off the target's Kind, not the string). Generator keeps the `TABLE` keyword (upstream's `EXPLAIN TABLE t` → `DESCRIBE t` conflates a query-explain with a metadata describe) |
| `oracle-plsql-function`, `oracle-plsql-trigger`, `oracle-plsql-package`, `oracle-plsql-anonymous-block` | Oracle PL/SQL units kept whole (§1.24): stored units → `Create{kind, this: Command}` with the verbatim unit text; anonymous blocks → `Command{this: BEGIN\|DECLARE}` |
| `pg-explain` | Postgres `EXPLAIN` → `Describe{kind:"EXPLAIN"}` with structured options + parsed inner statement |
| `mysql-replace` | `REPLACE INTO …` → `Insert{replace:true}` (MySQL generator renders `REPLACE`) |
| `mysql-describe-column` | `DESCRIBE tbl col\|'wild'` → `Describe{this:Table, column}` (single identifier/wild only) |
| `pg-set-role`, `pg-set-session-authorization`, `pg-set-time-zone`, `pg-set-names`, `pg-set-constraints`, `pg-set-session-characteristics` | Postgres SET special forms → `Set{SetItem{kind:…}}` |
| `pg-set-transaction-deferrable` | PG `[NOT] DEFERRABLE` transaction mode (PG-only mode table) |
| `pg-set-multi-value` | `SET search_path = a, b` → one `SetItem`, extra values in `expressions` (PG-only; strict `var_value` list) |
| `pg-set-scoped-role` | `SET [SESSION\|LOCAL] ROLE\|SESSION AUTHORIZATION` → same kinds + `SetItem.scope` |
| `mysql-set-password` | `SET PASSWORD [FOR user] = v` → `SetItem{kind:"PASSWORD"}` |
| `mysql-set-role`, `mysql-set-default-role` | MySQL `SET [DEFAULT] ROLE …` → `SetItem{kind:"ROLE"\|"DEFAULT ROLE"}`; strict role-name/CSV grammar, engine-invalid forms fail closed |
| `mysql-show-create-user` | `SHOW CREATE USER u` → `Show{this:"CREATE USER"}` |
| `mysql-show-grants-user-host`, `mysql-show-grants-using` | `SHOW GRANTS [FOR user [USING roles]]` → structured `Show` |
| `pg-show-guc`, `pg-reset` | PG `SHOW`/`RESET {name\|ALL\|special}` → `Show`/`Reset` with **canonical** `this` (lowercased, unquoted; `TIME ZONE`→`timezone`, `SESSION AUTHORIZATION`→`session_authorization`) |
| `mysql-into-outfile`, `mysql-into-dumpfile` | `SELECT … INTO OUTFILE\|DUMPFILE '/path'` → `Into{kind, this:path}` + structured export options; placement/tail rules match MySQL, violations fail closed |
| `pg-user-type-typed-literal` | PG `<type-name> 'str'` (space typed-literal) → the same `Cast` as `'str'::type`; also ports the `STRING_ALIASES` flag properly (base/PG reject implicit string aliases, MySQL accepts) |
| `pg-start-transaction`, `mysql-start-transaction-snapshot` | `START TRANSACTION [modes]` → `Transaction` (PG `START`→BEGIN token; MySQL `WITH CONSISTENT SNAPSHOT`) |
| `athena-show-columns`, `athena-show-create-table`, `athena-show-create-view`, `athena-show-databases`, `athena-show-schemas`, `athena-show-partitions`, `athena-show-tables`, `athena-show-tblproperties`, `athena-show-views` | Athena (Hive-routed) `SHOW …` → `Show{this:<canonical UPPER form>, target:Table, db, like}`; the SHOW token leaves Hive's tokenizer `Commands` set under athena only |
| `athena-explain` | Athena `EXPLAIN [ANALYZE] [(FORMAT\|TYPE …)] stmt` per the Athena docs (ANALYZE allows only FORMAT TEXT\|JSON; targets SELECT, CTAS, INSERT) → `Describe{kind:"EXPLAIN"}` (same shape as `pg-explain`); inner statement fully parsed |
| `athena-unload` | `UNLOAD (query) TO 's3://…' WITH (format = …, …)` → `Unload{this:Subquery, files, params}` (a dedicated Kind so athena/trino `COPY` keeps upstream's rendering); documented properties only, `format` required and validated, inner query fully parsed |
| `athena-rename-partition` | `ALTER TABLE t PARTITION (k = v) RENAME TO PARTITION (k = v)` → `Alter{actions:[AlterRename{this:Partition}]}` (upstream parse-errors) |
| `alter-add-constraint-repeated` | `ALTER TABLE t ADD CONSTRAINT a …, ADD CONSTRAINT b …` → `Alter` with one `AddConstraint` per `ADD` (upstream's constraint CSV eats the `, ADD` comma and the statement degrades to `Command`); `ADD CONSTRAINT a …, CONSTRAINT b …` keeps both in one `AddConstraint` |
| `pg-comment-is-null` | PG `COMMENT ON … IS NULL` (drops a comment) → `Comment{expression:Null}` (upstream parse-errors) |
| `pg-returns-setof` | PG `RETURNS SETOF <type>` → `ReturnsProperty{this:DataType, setof:true}` (an added `setof` arg; upstream leaves `SETOF` unparsed and the CREATE degrades to `Command`) |
| `mysql-create-user`, `mysql-create-role`, `mysql-alter-user`, `mysql-drop-user`, `mysql-drop-role` | account DDL → structured `Create`/`Alter`/`Drop` root with `kind:"USER"\|"ROLE"`; body kept verbatim in a `Command` child |
| `snowflake-copy-without-from` | `COPY INTO t` with the file location omitted (loads from the table's own stage) → `Copy` with no `files`; upstream parse-errors at EOF |
| `snowflake-show-empty-scope` | `SHOW … IN SCHEMA [<name>] [LIMIT n]` with the optional name omitted → `Show{scope_kind, limit}`; upstream consumes `LIMIT` as the scope name and parse-errors |

Cross-cutting rules for the SET/SHOW family:
- **Quoted dispatch keywords never match** — `SET "ROLE" x`, `` SET PASSWORD `=` 'x' ``,
  `RESET "all"` fail closed rather than regenerate as the unquoted (different) statement.
  Upstream's `_find_parser`/`_match_texts` match quoted tokens by text; the port does not.
- The privileged keyword forms are disambiguated from same-named GUC assignments by the following
  `=`/`:=`/`TO` delimiter (`SET role = x` parses as an assignment `EQ`, readable LHS).
- `parseMySQLUserSpec` enforces MySQL's `user` grammar (`name[@host]`, `CURRENT_USER[()]`); a dotted
  unquoted host under-accepts to `Command`.
- Known approximation: the port has no PG reserved-word table (upstream leaves it empty), so a few
  keyword-shaped names over/under-accept — all fail-safe (documented per-site in the parsers).

### 1.27 JSONPath tokenizer and extraction paths

`sqlglot/jsonpath.py` and `Dialect.to_json_path` are ported (`jsonpath/`, `Dialect.ToJSONPath`),
including the per-dialect tokenizer deltas — bigquery's DASH/NUMBER var tokens, its
dash-tolerant `SAFE_JSON_PATH_KEY_RE` and single-quote escaping; hive's DASH var tokens and
dash-tolerant key regex; snowflake's `$`-less single-token table; databricks' backtick
identifier and base key regex; and duckdb's JSON-pointer short-circuit. Function-form paths are
therefore rewritten exactly as upstream does: `JSON_EXTRACT(doc, '$. a b c .d')` →
`JSON_EXTRACT(doc, '$[\' a b c \'].d')`, `JSON_QUERY(...)` → `JSON_QUERY(doc, '$." a b c ".d')`
(double-quote escaping), `GET_JSON_OBJECT(c, '$.x-y')` → `GET_JSON_OBJECT(c, '$["x-y"]')`.

`build_json_extract_path` is ported for Postgres-family arrow operators and
`JSON_EXTRACT_PATH[_TEXT]` function builders (`dialects/dialect.py:2205-2247`,
`parsers/postgres.py:115-116`). Literal path segments become `JSONPathRoot` plus key/subscript
nodes; any non-literal function path falls back to the original positional arguments, matching
upstream. Postgres-family generation ports `_json_extract_sql` and `json_extract_segments`
(`generators/postgres.py:183-197`, `dialects/dialect.py:2250-2284`); Redshift uses its
always-function transform (`generators/redshift.py:87-88`). DuckDB's JSONPath operator
construction and same-parent wrapping are also covered. Guarding tests include
`jsonpath/jsonpath_test.go`, `TestJSONQueryRenderingIsTrinoFamilyOnly`,
`TestBigQueryJSONFunctions`, `TestDatabricksJSON`, `TestHiveJSONFunctions`, and the parity
corpora. The remaining Snowflake JSON extraction transforms and other cross-dialect gaps remain
tracked in `testdata/transpile_gaps.txt`; they are not implied complete by this entry.

### 1.28 SQLite `0x..` literals are integers, not BLOBs

Real SQLite parses a `0x`-prefixed token as a hexadecimal **integer**: `SELECT 0xCC` returns
204. Upstream sqlglot's tokenizer treats `0x..` as a hex string for every dialect and generates
`x'CC'` for SQLite — a BLOB, a different value (`0xCC = 204` is true; `x'CC' = 204` is false).
The port emits the decimal integer for SQLite (`HexIntegerLiterals` tokenizer flag), matching
the engine. `x'CC'`/`X'CC'` remain the BLOB literals. Guarding test:
`TestSQLiteHexIntegerLiteral`. Upstream's 0x expectations are excluded from the extracted
identity and transpile corpora by the extractor, since they encode the wrong value.

### 1.29 SQLite `LIKE`/`GLOB` function forms parenthesize binary operands

SQLite provides `like(pattern, string)`/`glob(pattern, string)` as functions. The infix rewrite
runs as `string LIKE pattern`, so a binary operand (`LIKE(a OR b, 'x')`) must be parenthesized
or SQL re-associates it (`a OR b LIKE 'x'` parses as `a OR (b LIKE 'x')`). Upstream emits the
unparenthesized form; the port wraps binary operands, matching the engine's evaluation.
Guarding test: `TestSQLiteLikeGlobParentheses`.

### 1.30 SQLite `RANGE CURRENT ROW` abbreviates only the bare frame form

Upstream's `windowspec_sql` returns `RANGE CURRENT ROW` for any `RANGE` frame whose start is
`CURRENT ROW`, dropping an explicit end (`RANGE BETWEEN CURRENT ROW AND UNBOUNDED FOLLOWING`)
and any `EXCLUDE` clause — which changes the window aggregate. The port abbreviates only when
no end and no EXCLUDE are present. Guarding test: `TestSQLiteWindowFrames`.

### 1.31 Teradata date-add amount is not simplified, and a non-literal never crashes

Upstream's `_date_add_sql` runs `_simplify_unless_literal` on the amount before type-checking
it (generators/teradata.py:25, generator.py:5380-5386 — a lazy in-method import of the
optimizer, there to break the same cycle Go forbids outright: `optimizer` imports `generator`,
so `generator` cannot import `optimizer`). Two consequences:

- **A foldable amount stays unfolded.** `DATE_ADD(d, INTERVAL 1 + 1 DAY)` generates
  `d + INTERVAL (INTERVAL 2 DAY)` upstream (the `1 + 1` folds to `2`, quoted by the
  `is_string` set) but `d + INTERVAL (INTERVAL (1 + 1) DAY)` here; read from bigquery the
  split is `d + INTERVAL '2' DAY` vs `d + INTERVAL (1 + 1) DAY`. Non-foldable amounts
  (`n`, `n + 1`) are identical to upstream.
- **A `Neg` over a non-literal does not crash.** Upstream takes `value.this.to_py()` on the
  negated operand and raises `ValueError: n cannot be converted to a Python object` for
  `INTERVAL -n DAY`; the port reports it unsupported like upstream does, keeps the operand
  bare and flips the operator (`d - INTERVAL n DAY`), which is semantically the interval
  upstream failed to emit. The literal side of `Neg` is unreachable from the parser in both
  implementations (`INTERVAL -5 DAY` parses to a `-5` literal), so only this crash-for-
  output case differs.

Guarding test: `TestTeradataDateAdd`.

### 1.32 DuckDB transpile follows the real engines over upstream

The DuckDB generator matches the real source/target engine where the pinned upstream v30.17.0
disagrees, even though these are cross-dialect-only (the matching corpus expectations are kept
in `testdata/transpile_corrections.jsonl` rather than editing the upstream-generated `want`):

- **`ARRAY_REMOVE`** keeps NULL elements (`IS DISTINCT FROM`, collision-free lambda) instead of
  upstream's `<>`. Upstream v30.35.0 adopted the same fix; the pin stays at v30.17.0.
- **Base64 custom alphabets** use one `TRANSLATE` instead of cascading `REPLACE` calls, so
  swapped symbols (e.g. `/+`) are not rewritten twice. Also fixed in upstream v30.35.0.
- **`RIGHT SEMI/ANTI JOIN`** emits DuckDB's typed join (`JOIN BY (TYPE RIGHT_ANTI)`) instead of
  silently rewriting it as a left-sided join. Also fixed in upstream v30.35.0.
- **`MONTHS_BETWEEN`** includes the time-of-day component in the fraction, matching Snowflake.
- **`TIMESTAMP_TRUNC`** honors an explicit zone for sub-day units, matching BigQuery.
- **`DATE(..., zone)`** converts a `TIMESTAMPTZ` instant directly in the target zone, so the
  result no longer depends on DuckDB's session time zone.
- **`JSON_VALUE_ARRAY`** returns SQL NULL when any element is an object or array, matching
  BigQuery's scalar-only contract.
- **Safe `PARSE_JSON`** uses one `TRY_CAST(... AS JSON)` evaluation instead of `JSON_VALID` plus
  `CAST`, which could evaluate a volatile operand twice.

Guarding tests: `TestDuckDBEngineCorrections`; corpus expectations in
`testdata/transpile_corrections.jsonl` (loaded by `TestTranspileCorpus`).

### 1.33 Cross-dialect transpile follows the real engines (review follow-ups)

Further real-engine corrections exercised by `testdata/transpile_corrections.jsonl`
(upstream v30.17.0 disagrees; upstream v30.35.0 has already adopted several):

- `TIMESTAMP_DIFF` uses complete-interval `DATE_SUB` arithmetic instead of boundary-counting
  `DATE_DIFF` (also upstream v30.35.0), so a half-hour difference is not one hour.
- MySQL/TSQL NULL-ordering simulation uses the resolved expression only for the rank key and
  keeps the original key second, and groups connector operands in the `IS NULL` test.
- Postgres `ARRAY_CONTAINS` returns NULL for a NULL input array.
- Presto `ARRAY_SORT` comparators handle NULL elements explicitly; `ARRAY_JOIN` forwards the
  null-replacement argument.
- Snowflake filtered `COUNT` uses `COUNT(IFF(cond, 1, NULL))` (0 on empty input),
  `JSON_ARRAY(NULL)` emits a JSON null, ordered DISTINCT aggregates rewrite matching order
  keys, and naive `AT TIME ZONE` reads the input in the supplied zone.
- Hive/Spark exclusive-end `RANGE` converts the bound to the last term inside the range.
- T-SQL `ARRAY_TO_STRING` has no row-preserving form, so it is reported unsupported and left
  as the source call.

Guarding test: `TestTranspileCorrectionsMatch`.

### 1.34 Real-engine correctness fixes from the DuckDB drain review

Corrections where pinned upstream v30.17.0 emits SQL with different semantics than the real
engine. Only the `UNIX_SECONDS` case appears in the validated corpus, so it carries an overlay
entry; the rest are exercised by unit tests or reachable only through untested argument forms.

- BigQuery `UNIX_SECONDS` floors fractional seconds (`FLOOR(EPOCH(...))`); a plain `CAST`
  rounds half away from zero, so `…:01.8` became 2 instead of 1.
- T-SQL `CONVERT` styles 10–13 use real `%` directives. Upstream's table contains literal
  `mm-dd-yy` / `yy/mm/dd` / `yymmdd` / `…ss…`, which DuckDB's `STRPTIME` cannot read as fields.
- `CONVERT(..., NULL)` returns NULL (SQL Server semantics) rather than falling back to a cast.
- T-SQL `OFFSET` under `DISTINCT` orders by the first projected expression; SQL Server rejects
  `ORDER BY (SELECT NULL)` there because the key is not in the DISTINCT projection.
- DuckDB `XOR` expansion parenthesizes connector operands, so a nested AND/OR keeps its
  binding instead of being re-associated by SQL precedence.

Guarding tests: `TestTranspileCorrectionsMatch`, `TestTSQLReviewFixes`, and
`TestDuckDBReviewRegressions`.

### 1.35 Postgres `LATERAL GENERATE_SERIES` keeps its source column name

Pinned upstream wraps a set-returning `GENERATE_SERIES` in `LATERAL` with `UNNEST`, so the
DuckDB relation's column is named `unnest` and a projection that references the original
`generate_series` name no longer binds. This port keeps the bare call for a `LATERAL` table
source (projections still get `UNNEST`), which preserves the column name and matches DuckDB's
own lateral table-source behavior.

Guarding test: `TestDuckDBTailTranspileFixes`.

### 1.36 Snowflake zone and MySQL epoch readings

- BigQuery `TIMESTAMP(value, zone)` with a naive value uses Snowflake's three-argument
  `CONVERT_TIMEZONE(zone, 'UTC', CAST(value AS TIMESTAMP))`. Pinned upstream emits the
  two-argument form, which Snowflake reads in the session zone instead of the supplied one.
- MySQL `UNIX_SECONDS(x)` renders `UNIX_TIMESTAMP(x)`. The base TimestampDiff fallback orders
  its arguments for the generic `TIMESTAMPDIFF(unit, start, end)` shape, which MySQL does not
  accept, and the native function is the correct mapping.

Guarding tests: `TestTranspileCorrectionsMatch` and `TestBigQuerySnowflakeTail`.

## Beyond-upstream dialects

A dialect upstream does not ship at all. There is no Python oracle for these — pinned v30.17.0
raises `Unknown dialect` on every read — so each construct is registered in
`testdata/upstream_extensions.jsonl` with `upstream: "parse_error"`, and the pin-tripwire
(`TestUpstreamExtensionsTripwire`) fires the day upstream adds the dialect. The parse/generate
identity gate is `testdata/dialect_identity.jsonl` (`dialect: ksqldb`, floors in `corpus_test.go`).

### ksqlDB (`ksqldb`, alias `ksql`)

Ported from Confluent `ksqldb-parser/src/main/antlr4/io/confluent/ksql/parser/SqlBase.g4`
(verified against the ksqlDB source tree). Tokenizer/flags overlay in `dialects/ksqldb.go`,
statement and query-modifier parsers in `parser/dialect_ksqldb.go`, rendering in
`generator/dialect_ksqldb.go` (plus four guarded splices in the shared generators: the query
clause order, the join's `WITHIN` window, `INSERT`'s `WITH` clause and `DROP`'s `DELETE TOPIC`).

| ksqlDB construct | node |
|---|---|
| `CREATE [OR REPLACE] [SOURCE] {STREAM\|TABLE} [IF NOT EXISTS] name [(cols)] [WITH (…)] [AS query]`, `CREATE {SINK\|SOURCE} CONNECTOR`, `CREATE TYPE` | `Create` with `kind` `"STREAM"`/`"TABLE"`/`"SOURCE STREAM"`/`"SINK CONNECTOR"`/`"SOURCE CONNECTOR"`/`"TYPE"` |
| `INSERT INTO t [(cols)] VALUES (…)`, `INSERT INTO t [WITH (…)] query` | `Insert` + `properties` |
| `DROP {STREAM\|TABLE} [IF EXISTS] name [DELETE TOPIC]`, `DROP {CONNECTOR\|TYPE}` | `Drop` + `delete_topic` |
| `(LIST\|SHOW) … [EXTENDED]` | `Show{this: "<VERB> <PHRASE>"}` — the verb is carried in `this` so `LIST` and `SHOW` stay distinct; `EXTENDED` → `full` |
| `DESCRIBE … [EXTENDED]`, `DESCRIBE {FUNCTION\|CONNECTOR} x` | `Describe` — ksqlDB's `EXTENDED` is a **suffix**, so it lands in `style`, not the prefix `style` the base generator renders |
| `EXPLAIN (stmt\|identifier)` | `Describe{kind: "EXPLAIN"}` (same shape as `pg-explain`) |
| `ALTER {STREAM\|TABLE} t ADD [COLUMN] name type [, …]` | `Alter{kind, actions: [ColumnDef]}` |
| `TERMINATE\|PAUSE\|RESUME (id\|ALL)` | `Kill{kind: <verb>}` |
| `SET 'k' = 'v'`, `UNSET 'k'`, `DEFINE x = 'v'`, `UNDEFINE x` | `Set` — `tag` is the DEFINE/UNDEFINE discriminator, `unset` the UNSET/UNDEFINE one; a plain `SET` renders through base `setSQL` |
| `WINDOW <name>? {TUMBLING\|HOPPING\|SESSION} (…)` | one `KindKsqlWindow` in `Select.windows`; the `WINDOW ` keyword is prefixed by `ksqlQueryModifiers`, so the node renders its body only |
| `PARTITION BY …` | `Select.partition_by` |
| `EMIT {CHANGES\|FINAL}` | `Select.emit` as a raw `Var` |
| join `WITHIN <size> [(before, after)] [GRACE PERIOD <size>]` | `Join.within` (`Interval`, or a two-`Interval` `Tuple`) + `Join.grace` |
| window/duration sizes (`1 HOUR`) | bare `Interval{this, unit}` — ksqlDB has no `INTERVAL` keyword here |
| `a -> b` | `JSONExtract` rendered as the `->` operator |
| `STRING` type | `DTypeText` mapped back to `STRING` (base would emit `TEXT`, which ksqlDB has no keyword for) |

**Deliberately not modeled** (verbatim `Command`, still round-trip-stable): `PRINT … printClause`,
`ASSERT … timeout`, `RUN SCRIPT`. Their trailing clauses are not worth a node.

**Fail closed** — ksqlDB's grammar rejects these, so the port refuses to structure them and the
statement degrades to a raw `Command` instead of admitting SQL the engine would refuse:
`CREATE … (cols) … AS query` (the two CREATE forms are disjoint, SqlBase.g4:74-83),
`INSERT INTO t (cols) SELECT …` and `INSERT INTO t WITH (…) VALUES …` (the two INSERT forms are
disjoint, SqlBase.g4:86-87), and a dangling `EMIT` / `WINDOW` / `PARTITION BY` clause (each is
peeked before it is consumed, so a malformed clause leaves its tokens in place and the statement
errors out rather than silently dropping the clause).

**Cross-cutting base change:** ksqlDB's `EMIT` is rejected as an implicit alias in the two alias
positions — `parseTableAlias` (table alias in `FROM s EMIT CHANGES`) and `parseAlias` (expression
alias in `PARTITION BY a EMIT CHANGES`) — via a `dialectIs("ksqldb")` guard in each. The guard
fires only on the modifier spelling (EMIT followed by CHANGES/FINAL): EMIT is nonReserved
(SqlBase.g4:428), so a bare `FROM s emit` / `SELECT a emit` still aliases exactly as ksqlDB's
`aliasedRelation: relationPrimary (AS? sourceName)?` allows. Upstream computes
`TABLE_ALIAS_TOKENS`/`ALIAS_TOKENS` once from the base `ID_VAR_TOKENS` (parsers/base.py:771), so
a dialect's `ID_VAR_TOKENS` additions are never implicit-alias candidates; merging them here
made `EMIT` swallow the preceding expression. The additions still apply everywhere else
(`SELECT emit FROM t`, `SELECT a AS emit FROM t`).
Guarding tests: `TestKsqlDBRoundTrip`, `TestKsqlDBFailClosed`.

Guarding tests: `ksqldb_test.go` (`TestKsqlDBRoundTrip`, `TestKsqlDBAST`, `TestKsqlDBFailClosed`,
`TestKsqlDBPrintStaysCommand`, `TestKsqlDBTypes`, `TestKsqlDBBaseDegrades`).

## 2. Cross-dialect-only deviations (never affect same-dialect round-trip)

Cross-dialect transpilation between the registered dialects is exercised and gap-tracked by
`TestTranspileCorpus` (issue #33); the deviations below only manifest there and never in a
same-dialect round trip. SQLite's `DATEDIFF(..., 'nanosecond')` multiplier uses the real
86,400,000,000,000 ns/day instead of upstream's ten-times-too-small value, so the transpiled
result matches the engine. The Presto/Trino/Hive/Athena generators port the
same-dialect surface of upstream's `TRANSFORMS`/`TYPE_MAPPING`/`PROPERTIES_LOCATION`
(`generator/dialect_{presto,trino,hive,athena}.go`); the transpile-only `Select` preprocess chain
(eliminate_qualify/distinct_on, explode_projection_to_unnest, semi/anti joins) and transforms whose
source nodes only other dialects build are not. The Spark 2, Spark and Databricks generators follow
the same rule: their shared `Select` preprocess chain (eliminate_qualify, eliminate_distinct_on,
unnest_to_explode, any_to_exists) stays unported, and no identity-corpus case observes it. Several Presto functions still parse as `Anonymous`
(`DATE_FORMAT`, `DATE_PARSE`, `TO_CHAR`, `DATE_TRUNC`, `REGEXP_*`, `LOCALTIME[STAMP]`, `CONCAT_WS`)
— lineage still sees column args; same-dialect `.sql()` echoes them.

---

## 3. Output-preserving, Go-necessitated divergences

None change `.sql()`:
- `newNode` orders args by per-Kind `argTypes` declaration order, not insertion order.
- `parseTable` fast-path skip (parse-order optimization).
- `IsWrapper` uses the `truthy` helper (no nil args stored).
- `matchTextSeq` retreat has no debug logger.
- `bracket_to_element_at_sql` / the Spark `TRY_ELEMENT_AT` path shift only a literal integer index;
  upstream's `apply_index_offset` additionally annotates types and simplifies. The parsed safe
  brackets carry offset=1, which cancels the shift, so `.sql()` is unchanged.

---

## 4. Cosmetic AST-shape divergences (output identical, `.ToS()` differs)

- Comment bubbling: a trailing comment can attach to a slightly different node.

---

## 5. Deferred / fail-closed (`Command` where a future slice would structure)

- Hive CREATE-DDL property callbacks (kept in Hive's `PropertyParsers` overlay; base/mysql/postgres
  fail them closed).
- Upstream `_parse_heredoc`'s `$`-textseq fallback (parser.py:9372-9399, for dialects that don't lex
  dollar-quotes): pinned upstream's own base/mysql round-trip of it is broken (`… END AS $$`), so
  only the HEREDOC_STRING branch is ported (Postgres bodies parse to `exp.Heredoc`).

---

## 6. Go-only analysis API extensions

### 6.1 Top-level UPDATE/DELETE/MERGE scopes
Upstream `traverse_scope` yields no scope for a DML root. `TraverseScope`/`BuildScope` additionally
yield a root `Scope` (target + FROM/USING/JOIN sources), complete-or-none (a malformed source omits
the root scope, never emits a partial one). The optimizer passes use a separate compatibility
traversal reproducing upstream exactly. Tests `optimizer/scope_dml_test.go`.

### 6.2 Projection, statement + bind-parameter source spans
Upstream discards token positions at parse time. The port stamps each SELECT projection (inside any
`Alias`) and every bind parameter (`Placeholder`/`Parameter`: `?`, `:name`, `$1`, `%(name)s`, `@var`,
`${a:b}`) with `meta["span"]` (rune offsets) and `meta["spanText"]` (verbatim text — `1 +    1` keeps
its spacing), both-or-neither, via the span helpers (`parseSpanned`/`spanned`). Meta survives `Copy()`, never affects output/
`HashKey`/`Equal`; a rewrite that replaces the node drops it. Accessors in `expressions/span.go`.
Every top-level statement `Parse` returns is stamped the same way (a multi-chunk statement — a
procedure `BEGIN…END` body — spans all its chunks, inner semicolons included), so a batch consumer
can split a multi-statement input into verbatim per-statement slices without a `Generate()`
rewrite. A statement span covers the statement's tokens **plus its attached comments**: leading
line/block comments attached to the first token, and trailing comments on the statement's same
line, before its `;`, or at end of input — so a `;` inside a comment is never mistaken for a
boundary. An Oracle PL/SQL unit's terminating `;` is part of the unit (an anonymous block or a
stored unit needs it to execute) and is included in the span; a call specification with no END
keeps its `;` outside, as an ordinary statement terminator. Inter-statement separators and
surrounding whitespace are not covered. A boundary token spliced from an activated MySQL
executable comment widens to the `/*!NNNNN … */` delimiters — statement spans only — so the slice
stays executable MySQL.
Bind-parameter spans let a consumer bind ordered parameters lexically; tree walk order is not lexical
across CTEs, joins and WHERE. Tests `expressions/span_test.go`, `parser/parser_span_test.go`,
`parser/placeholder_span_test.go`, `optimizer/qualify_span_test.go`, `statement_span_test.go`.

### 6.3 `Token.Keyword`
Upstream tokens carry no record of where their type came from. `Token.Keyword` is set when the
type was looked up in the scanning tokenizer's `Keywords` table — under Athena that is the Hive or
Trino table the statement was routed to — so a token-only consumer can tell keywords from
identifiers without re-deriving the routing. Operators in that table (`::`, `<=`, `->`) are flagged
too. Quoted identifiers, strings, and a word after a
`PARAMETER` are never keywords. `Token.Synthetic` marks the one token with no source text,
Athena's `HIVE_TOKEN_STREAM` routing marker, so such a consumer can skip it without naming Hive.
Tests `tokens/tokens_test.go`, `dialects/athena_test.go`.

---

## 7. Qualifier arg renamed `db` → `schema`

Upstream calls the middle table/column qualifier `db` — a misnomer for the ANSI **schema**. The port
renames that one qualifier everywhere: arg-key `"schema"`, `SchemaName()`,
`TablePartKeys = {this, schema, catalog}`, builders `Table_(table, schema, catalog, …)`,
`QualifyOpts.DB` → `QualifyOpts.DefaultSchema`. `.sql()` output unchanged; `.ToS()` renders
`schema=` where Python renders `db=`.

**NOT renamed** (genuine databases): `Show.db` (`SHOW … FROM <db>`), `Use`, `CREATE DATABASE`,
`TruncateTable.is_database`, the `DATABASE` token, schema-mapping data keys, and 1:1-ported flag
names like `Dialect.RenameTableWithDB`. The `is_db_reference` construct reuses the (now-`schema`)
slot for a database name, as upstream reuses `db` — discriminated by `is_database`.

**Porting rule:** upstream code touching a Table/Column `db` arg / `.db` → translate to `schema`;
leave genuine-database `db` alone. Python-captured `fidelity_cases.txt` oracles: apply
`s/\bdb=/schema=/` (qualifier only).

---

## Not deviations

Where a reviewer flags an "upstream bug", the port generally keeps upstream 1:1 (faithfulness);
§1 entries are the deliberate exceptions, each matching the real engine.
