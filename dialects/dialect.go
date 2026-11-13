package dialects

import (
	"fmt"
	"strings"
	"sync"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

type NormalizationStrategy int

const (
	Lowercase NormalizationStrategy = iota
	Uppercase
	CaseSensitive
	CaseInsensitive
	CaseInsensitiveUppercase
	// The two MySQL strategies below are NON-UPSTREAM (DEVIATIONS.md §1.2). They fold with
	// MySQL's exact identifier lowercase (MySQLLower — my_unicase_default .tolower, Unicode
	// simple + accent-preserving: É->é but Ñ!=N), NOT the ASCII fold used by the other
	// strategies. That fold is exported (MySQLLower) so a consumer can call the same
	// implementation via a native binding, guaranteeing a byte-identical normalized identifier.

	// MySQLCaseInsensitive folds EVERY identifier (columns AND table/schema names), regardless of
	// quoting, with MySQLLower. Models MySQL with lower_case_table_names=1/2 (all names
	// case-insensitive).
	MySQLCaseInsensitive
	// MySQLCaseSensitiveTableNames is role-aware: table/database name identifiers are
	// case-sensitive (never folded); every other identifier (columns, aliases, CTE names, ...)
	// is folded with MySQLLower, regardless of quoting. Models MySQL's default on a
	// case-sensitive filesystem (lower_case_table_names=0): columns case-insensitive on every
	// platform, database/table names case-sensitive on Linux.
	MySQLCaseSensitiveTableNames
)

type Dialect struct {
	Name            string
	QuoteStart      string
	QuoteEnd        string
	IdentifierStart string
	IdentifierEnd   string
	// TokenizerFactory supports Athena's classify-and-re-tokenize router while preserving
	// Dialect.NewTokenizer's concrete *tokens.Tokenizer return type. A nil factory uses the
	// ordinary compiled TokenizerConfig path.
	TokenizerFactory func() *tokens.Tokenizer
	TokenizerConfig  tokens.TokenizerConfig
	// MySQLVersion is NON-UPSTREAM tokenizer state (DEVIATIONS.md §1.5): the parsed
	// MySQL five-digit comparable value. Nil is the default-off sentinel.
	MySQLVersion *int
	// MySQLAnsiQuotes is NON-UPSTREAM, opt-in MySQL tokenizer state (DEVIATIONS): when true, the
	// MySQL `sql_mode=ANSI_QUOTES` behavior is active — `"` is a QUOTED IDENTIFIER delimiter (like the
	// backtick), not a string literal (which must then use `'`). Nil/false is the default-off MySQL
	// behavior (`"` is a string). Set via the `mysql_ansi_quotes` dialect setting; applied by
	// applyMySQLAnsiQuotes (dialects/mysql.go) at settings-resolution time. Mutable per statement on
	// the real server, so a consumer re-resolves the dialect when the backend's live sql_mode changes.
	MySQLAnsiQuotes *bool
	// OpaqueFunctions is NON-UPSTREAM, opt-in (DEVIATIONS): when true, name-lookup builtin calls
	// parse as exp.Anonymous with the name as written and generate back verbatim; keyword-grammar
	// forms (CAST, EXTRACT ... FROM, TRIM ... FROM, SUBSTRING ... FROM/FOR, POSITION ... IN,
	// CHAR ... USING, CEIL ... TO) keep their structured nodes and render form-faithfully.
	// Set via the `opaque_functions` dialect setting at GetOrRaise resolve time.
	OpaqueFunctions         bool
	NormalizationStrategy   NormalizationStrategy
	DPipeIsStringConcat     bool
	StrictStringConcat      bool
	TypedDivision           bool
	SafeDivision            bool
	SupportsColumnJoinMarks bool
	ColonIsVariantExtract   bool
	// ColonChainIsSingleExtract ports COLON_CHAIN_IS_SINGLE_EXTRACT (parser.py:1880, base True;
	// databricks overrides to False): whether x:y:z is one extraction with a merged path.
	ColonChainIsSingleExtract bool
	// The JSON_PATH_* generator flags (generator.py:505-561) control how a JSONPath renders.
	// QuoteJSONPath wraps the whole path in the dialect's quote characters (base True).
	QuoteJSONPath bool
	// JSONPathKeyQuotedForcesBrackets renders a quoted key as ["key"] even when it would be a
	// safe identifier (base False; databricks True).
	JSONPathKeyQuotedForcesBrackets bool
	// JSONPathSingleQuoteEscape uses backslash-escaped single quotes for a bracketed key
	// (base False).
	JSONPathSingleQuoteEscape bool
	// JSONPathBracketedKeySupported allows the ["key"] form (base True).
	JSONPathBracketedKeySupported bool
	NullOrdering                  string
	SupportsOrderByAll            bool
	TryCastRequiresString         *bool
	DatePartMapping               map[string]string
	// TimeMapping / InverseTimeMapping port TIME_MAPPING (dialects/dialect.py:447) and the
	// metaclass-generated INVERSE_TIME_MAPPING (dialect.py:261-271): the dialect's TO_CHAR-style
	// pattern table (parse side, consumed by build_formatted_time) and its inverse (generate
	// side, consumed by Generator.format_time). nil for dialects upstream leaves at the {}
	// default; the Postgres family populates both (redshift overriding with its MON/MONTH
	// additions, dialects/redshift.py:30-34).
	TimeMapping              map[string]string
	InverseTimeMapping       map[string]string
	ValidIntervalUnits       map[string]bool
	SupportsUserDefinedTypes bool
	SupportsFixedSizeArrays  bool
	SupportsLimitAll         bool
	SupportsValuesDefault    bool
	// SupportsImplicitUnnest ports the parser flag SUPPORTS_IMPLICIT_UNNEST
	// (parser.py:1887): when true, `_parse_query_modifiers` rewrites a `FROM a, a.b` join
	// whose leftmost table part matches an already-introduced relation into an explicit
	// Unnest (parser.py:4305-4330). Base defaults to false (parser.py:1887); upstream sets
	// it true for Redshift (parsers/redshift.py:84) and BigQuery (parsers/bigquery.py:192).
	SupportsImplicitUnnest bool
	// SupportsSelectInto ports the generator flag SUPPORTS_SELECT_INTO (generator.py:466):
	// when false (base/mysql), the generator rewrites `SELECT ... INTO x` into
	// `CREATE TABLE x AS SELECT ...`; postgres (and tsql) keep the inline INTO.
	SupportsSelectInto bool
	// ValuesIsFunction ports the MySQL parser's FUNC_TOKENS addition of TokenType.VALUES
	// (parsers/mysql.py:63-70), paired upstream with its FUNCTION_PARSERS["VALUES"] override
	// (parsers/mysql.py:158-160): `VALUES(col)` in `ON DUPLICATE KEY UPDATE` is a function
	// call, not the VALUES table-constructor keyword. Callback overrides now live in the
	// parser-side dialect registry, but FUNC_TOKENS admission remains a separate,
	// not-yet-generalized problem: this flag lets parseFunctionCall admit VALUES before
	// callback dispatch.
	ValuesIsFunction bool
	// CharsetIsFunction likewise ports MySQL's FUNC_TOKENS addition of
	// TokenType.CHARACTER_SET (parsers/mysql.py:63-70): `CHARSET(...)`/`CHARACTER SET(...)`
	// is an ordinary (Anonymous) function call, not the CHARACTER SET/CHARSET keyword.
	// The parser-side callback registry does not control that earlier admission decision,
	// so this flag remains until FUNC_TOKENS itself is generalized per dialect.
	CharsetIsFunction bool
	// DuplicateKeyUpdateWithSet ports the generator flag of the same name
	// (generator.py:374, generators/mysql.py:137): base emits `ON DUPLICATE KEY UPDATE SET
	// ...`, MySQL's own generator omits the SET keyword (`ON DUPLICATE KEY UPDATE ...`).
	DuplicateKeyUpdateWithSet bool
	// WrapDerivedValues ports the generator flag of the same name (generator.py:320,
	// generators/mysql.py:148): base wraps an aliased/derived VALUES table constructor in
	// parentheses (`(VALUES (1, 2)) AS t`), MySQL emits it bare (`VALUES (1, 2) AS t`).
	WrapDerivedValues bool
	// CreateFunctionReturnAs ports the generator flag of the same name (generator.py:324,
	// default True; generators/spark2.py:214 overrides to False): whether a CREATE FUNCTION
	// body is introduced by `AS`. Upstream gates it on the body being an exp.Return
	// (generator.py:1363), a node this port's parser never builds, so the gate is currently
	// unreachable - it is kept so the flag itself stays a faithful port.
	CreateFunctionReturnAs bool
	// ValuesAsTable ports the generator flag of the same name (generator.py:397,
	// generators/mysql.py:139): base renders a table-source VALUES as a VALUES constructor,
	// MySQL rewrites it into a series of SELECT unions (`(SELECT 1 AS a, ...) AS t`). A VALUES
	// used as an INSERT source (not under FROM/JOIN) is unaffected by this flag.
	ValuesAsTable                      bool
	IntervalSpans                      bool
	NormalizeFunctions                 string
	DefaultFunctionsColumnNames        map[exp.Kind][]string
	AliasPostTablesample               bool
	AliasPostVersion                   bool
	UnnestColumnOnly                   bool
	Pseudocolumns                      map[string]bool
	PreferCTEAliasColumn               bool
	ForceEarlyAliasRefExpansion        bool
	ExpandOnlyGroupAliasRef            bool
	AnnotateAllScopes                  bool
	DisablesAliasRefExpansion          bool
	SupportsAliasRefsInJoinConditions  bool
	ProjectionAliasesShadowSourceNames bool
	TablesReferenceableAsColumns       bool
	SupportsStructStarExpansion        bool
	ExcludesPseudocolumnsFromStar      bool
	QueryResultsAreStructs             bool
	// PromoteToInferredDatetimeType ports PROMOTE_TO_INFERRED_DATETIME_TYPE
	// (dialects/dialect.py:700): canonicalize compares a temporal value with a date-like string
	// at the type the string looks like. False by default, true for MySQL.
	PromoteToInferredDatetimeType bool
	// SafeToEliminateDoubleNegation ports SAFE_TO_ELIMINATE_DOUBLE_NEGATION
	// (dialects/dialect.py:754): simplify may rewrite NOT NOT x to a BOOLEAN x. True by default.
	SafeToEliminateDoubleNegation bool
	// CoalesceComparisonNonStandard ports COALESCE_COMPARISON_NON_STANDARD
	// (dialects/dialect.py:660): simplify must not rewrite comparisons with COALESCE.
	CoalesceComparisonNonStandard bool
	// WeekOffset ports WEEK_OFFSET (dialects/dialect.py:363): the first day of the week for week
	// truncation, 0 for Monday and -1 for Sunday.
	WeekOffset int
	// UnpivotValueColumnsFirst ports the parser flag UNPIVOT_VALUE_COLUMNS_FIRST (parser.py:1841):
	// an UNPIVOT emits its value columns ahead of its name column (T-SQL).
	UnpivotValueColumnsFirst bool
	// PrioritizeNonLiteralTypes ports PRIORITIZE_NON_LITERAL_TYPES (dialects/dialect.py:801): when
	// annotate_types combines a literal and a non-literal of the same integer or real family, the
	// non-literal's type wins instead of the coerced one.
	PrioritizeNonLiteralTypes         bool
	RequiresParenthesizedStructAccess bool
	IndexOffset                       int
	// RenameTableWithDB ports the generator flag of the same name (generator.py:344): base
	// allows `ALTER TABLE db.a RENAME TO db.b`, some dialects (e.g. postgres, generators/
	// postgres.py:234) only allow renaming within the same db and drop the db qualifier.
	RenameTableWithDB bool
	// SupportsModifyColumn ports the generator flag of the same name (generator.py:475):
	// whether `ALTER TABLE ... MODIFY COLUMN <column-redefinition>` syntax is supported.
	SupportsModifyColumn bool
	// SupportsChangeColumn ports the generator flag of the same name (generator.py:478):
	// whether `ALTER TABLE ... CHANGE COLUMN <rename-and-redefine>` syntax is supported.
	SupportsChangeColumn bool
	// SupportsCreateTableLike ports the generator flag of the same name (generator.py:480):
	// whether the CREATE TABLE LIKE statement is supported (base True; Materialize overrides
	// False, generators/materialize.py:15). Consumed by likeproperty_sql.
	SupportsCreateTableLike bool
	// SupportsBetweenFlags ports the generator flag of the same name (generator.py:608):
	// whether SYMMETRIC/ASYMMETRIC flags are supported with BETWEEN (base False; postgres
	// True, generators/postgres.py:280; Materialize and RisingWave re-override False).
	SupportsBetweenFlags bool
	// ExpressionPrecedesPropertiesCreatables ports Generator.EXPRESSION_PRECEDES_PROPERTIES_
	// CREATABLES (generator.py:684, base empty set): creatable kinds whose `AS <expression>`
	// renders before the POST_SCHEMA/POST_WITH properties instead of after them (risingwave
	// SINK, generators/risingwave.py:22).
	ExpressionPrecedesPropertiesCreatables map[string]bool
	// WrappedTransformColumnConstraint ports Parser.WRAPPED_TRANSFORM_COLUMN_CONSTRAINT
	// (parser.py:1896): whether the `name AS expr` column transform requires the expression
	// to be parenthesized (base True; risingwave overrides False, parsers/risingwave.py:11,
	// so `col INT AS expr` parses without the parens).
	WrappedTransformColumnConstraint bool
	// VarcharRequiresSize ports the MySQL-generator-family flag of the same name
	// (generators/mysql.py:149): MySQL requires VARCHAR to carry an explicit size.
	VarcharRequiresSize bool
	// AlterTableSupportsCascade ports the dialect flag of the same name (dialects/dialect.py:
	// 701): whether ALTER TABLE ... CASCADE is supported to propagate a column change to
	// existing partitions (Hive-family only).
	AlterTableSupportsCascade bool
	// AlterTableAddRequiredForEachColumn ports the dialect flag of the same name (dialects/
	// dialect.py:709): whether ADD must be repeated for each column added by ALTER TABLE.
	AlterTableAddRequiredForEachColumn bool
	// AlterRenameRequiresColumn ports the parser flag of the same name (parser.py:1819):
	// whether renaming a column with ALTER requires the presence of the COLUMN keyword.
	AlterRenameRequiresColumn bool
	// AlterTablePartitions ports the parser flag of the same name (parser.py:1822): whether
	// ALTER statements are allowed to contain PARTITION specifications.
	AlterTablePartitions bool
	// AlterTableIncludeColumnKeyword ports the generator flag of the same name (generator.py:
	// 400): whether the word COLUMN is included when adding a column with ALTER TABLE.
	AlterTableIncludeColumnKeyword bool
	// ComputedColumnWithType ports the generator flag of the same name (generator.py:412):
	// whether to include the type of a computed column in the CREATE DDL.
	ComputedColumnWithType bool
	// AlterSetWrapped ports the generator flag of the same name (generator.py:568): whether
	// to wrap AlterSet's properties in parens, e.g. `ALTER ... SET (<props>)`.
	AlterSetWrapped bool
	// AlterSetType ports the generator flag of the same name (generator.py:582): the syntax
	// to use when altering the type of a column via ALTER SET.
	AlterSetType string
	// InitcapDefaultDelimiterChars ports INITCAP_DEFAULT_DELIMITER_CHARS (dialects/dialect.py:
	// 762): the delimiter set an INITCAP call is considered to be using the default with, so
	// the generator can drop a redundant delimiters argument (generator.py:6246-6260). hive
	// (dialects/hive.py:35) and spark2 (dialects/spark2.py:29) override it.
	InitcapDefaultDelimiterChars string
	// castTypeMapping is the slice of this dialect's TYPE_MAPPING (generators/<dialect>.py, a
	// Generator class attribute upstream) that a parse-time exp.cast call can observe: the
	// DTypes the dialect renders as "TIMESTAMP". Upstream's cast() reaches into
	// generator_class.TYPE_MAPPING for its types_are_equivalent half
	// (expressions/builders.py:494-503); this port keeps TYPE_MAPPING in the generator
	// package because that is where upstream declares it, so the parse side carries the folds
	// its own cast builders can ask about. Only the spark2 FROM_UTC/TO_UTC_TIMESTAMP builders
	// call exp.cast today, and they always ask for TIMESTAMP, so that is the only fold family
	// recorded. nil means "no folds", i.e. an exact target match only - which is what base
	// does for every type a parse-side builder can request.
	castTypeMapping map[exp.DType]string
	// SupportsDropAlterIcebergProperty ports the generator flag of the same name (generator.py:
	// 619): whether DROP/ALTER can carry the ICEBERG keyword (e.g. Snowflake's `DROP ICEBERG
	// TABLE a.b`; DuckDB overrides to False and just emits `DROP TABLE a.b`).
	SupportsDropAlterIcebergProperty bool
	// SingleStringInterval ports the generator flag of the same name (generator.py:335); postgres
	// overrides to True (generators/postgres.py:233): render INTERVAL as a single quoted
	// "<value> <unit>" string instead of the base's separate `INTERVAL <this> <unit>` tokens.
	SingleStringInterval bool
	// IntervalAllowsPluralForm ports the generator flag of the same name (generator.py:335);
	// mysql overrides to False (generators/mysql.py:132): whether a plural interval unit (e.g.
	// "DAYS") is kept as-is, or singularized via TIME_PART_SINGULARS first.
	IntervalAllowsPluralForm bool
	// ParameterToken ports the generator flag PARAMETER_TOKEN (generator.py:667); postgres
	// overrides to "$" (generators/postgres.py:240): the sigil parameterSQL prefixes a
	// Parameter's name with (base/mysql use "@", postgres uses "$" for its positional $1/$2/...
	// placeholders).
	ParameterToken string
	// LockingReadsSupported ports the generator flag LOCKING_READS_SUPPORTED (generator.py:314);
	// mysql (generators/mysql.py:133) and postgres (generators/postgres.py:235) override to True:
	// whether row-locking read modifiers (FOR UPDATE/SHARE/...) are rendered instead of raising
	// "unsupported".
	LockingReadsSupported bool
	// TablesampleSizeIsPercent ports the dialect flag of the same name (dialects/dialect.py:342);
	// postgres overrides to true (dialects/postgres.py:20): whether a bare numeric size in
	// TABLESAMPLE means percent (rather than rows).
	TablesampleSizeIsPercent bool
	// TablesampleSizeIsRows ports the generator flag TABLESAMPLE_SIZE_IS_ROWS (generator.py:421);
	// postgres overrides to false (generators/postgres.py:242): whether a rendered size needs a
	// trailing ROWS keyword.
	TablesampleSizeIsRows bool
	// TablesampleSeedKeyword ports the generator flag TABLESAMPLE_SEED_KEYWORD (generator.py:430);
	// postgres overrides to "REPEATABLE" (generators/postgres.py:243): the keyword preceding a
	// TABLESAMPLE clause's seed value.
	TablesampleSeedKeyword string
	// TablesampleRequiresParens ports the generator flag TABLESAMPLE_REQUIRES_PARENS
	// (generator.py:418): whether parentheses are required around the table sample's expression.
	TablesampleRequiresParens bool
	// TablesampleWithMethod ports the generator flag TABLESAMPLE_WITH_METHOD (generator.py:427):
	// whether the TABLESAMPLE clause supports a method name, like BERNOULLI.
	TablesampleWithMethod bool
	// TablesampleKeywords ports the generator flag TABLESAMPLE_KEYWORDS (generator.py:424): the
	// keyword(s) to use when generating a sample clause.
	TablesampleKeywords string
	// CopyHasIntoKeyword ports the generator flag COPY_HAS_INTO_KEYWORD (generator.py:526);
	// postgres overrides to False (generators/postgres.py:251): whether copySQL renders
	// `COPY INTO <this>` (base) or `COPY <this>` (postgres).
	CopyHasIntoKeyword bool
	// CopyParamsAreWrapped ports the generator flag COPY_PARAMS_ARE_WRAPPED (generator.py:
	// 520): whether copySQL's WITH-clause params are wrapped in parens (`WITH (...)`,
	// indented when pretty) or emitted bare.
	CopyParamsAreWrapped bool
	// CopyParamsAreCsv ports the dialect flag COPY_PARAMS_ARE_CSV (dialects/dialect.py:363):
	// whether COPY's WITH-clause params are comma-separated (parser: the separator consumed
	// between options; generator: the separator emitted between rendered options).
	CopyParamsAreCsv bool
	// CopyParamsEqRequired ports the generator flag COPY_PARAMS_EQ_REQUIRED (generator.py:
	// 523): whether copyparameterSQL always renders `option = value` (True) or `option value`
	// (False) when a param has a scalar value.
	CopyParamsEqRequired bool
	// JSONArrowsRequireJSONType ports the parser flag JSON_ARROWS_REQUIRE_JSON_TYPE
	// (parsers/postgres.py:191); base/mysql leave it False. When set, a JSON `->`/`->>` RHS
	// that is a literal marks the built JSONExtract/JSONExtractScalar node's only_json_types
	// arg (mirroring build_json_extract_path's arrow_req_json_type branch, dialect.py:
	// 2092-2124), which the generator uses to choose operator-form (`->`/`->>`) over
	// function-form (JSON_EXTRACT_PATH/JSON_EXTRACT_PATH_TEXT) rendering.
	JSONArrowsRequireJSONType bool
	// JSONOperatorsAreBinary ports the v30.17.0 COLUMN_OPERATORS/JSON_OPERATORS split
	// (parsers/postgres.py:192-219, parser.py:6312-6313): when set, the JSON operators
	// (`->` `->>` `#>` `#>>` `?`) leave the accessor tier and bind at the bitwise tier with
	// a term RHS — `1 + j -> 'a'` parses as JSONExtract(Add(1, j), 'a'), matching real PG
	// operator precedence. base/mysql keep the accessor-tier behavior.
	JSONOperatorsAreBinary bool
	// NormalizeNotNull ports NORMALIZE_NOT_NULL (dialect.py:757, v30.17.0; postgres.py:16
	// overrides False): whether a postfix NOTNULL builds the normalized NOT(Is(x, NULL))
	// (true) or Is(x, NULL, negate=True), rendering `x IS NOT NULL` (false, postgres).
	NormalizeNotNull bool

	// SUPPORTS_ALTER_COLUMN_NULLABILITY (generator.py:489, v30.17.0): ALTER COLUMN can set
	// nullability together with its type (mysql true, generators/mysql.py:118).
	SupportsAlterColumnNullability bool
	// SUPPORTS_ALTER_COLUMN_IF_EXISTS (generator.py:492, v30.17.0).
	SupportsAlterColumnIfExists bool
	// JSONTypeRequiredForExtraction ports the generator flag JSON_TYPE_REQUIRED_FOR_EXTRACTION
	// (generator.py:488); mysql (generators/mysql.py:142) and postgres (generators/postgres.py:
	// 245) both override to True. Consumed by arrow_json_extract_sql (dialect.py:1210-1215):
	// when set, a string-literal `this` is wrapped in `CAST(... AS JSON)` before rendering the
	// JSON_EXTRACT_SCALAR arrow form.
	JSONTypeRequiredForExtraction bool
	// StrictCast ports parser.py:1755 STRICT_CAST: whether CAST and :: build Cast (true)
	// or the dialect's permissive TryCast form (false).
	StrictCast bool
	// LogDefaultsToLn ports parser.py:1765 LOG_DEFAULTS_TO_LN: whether one-argument LOG(x)
	// canonicalizes to Ln instead of Log.
	LogDefaultsToLn bool
	// JoinsHaveEqualPrecedence ports parser.py:1824-1828: whether keyword and comma joins build
	// the same left-deep tree instead of keyword JOIN binding more tightly.
	JoinsHaveEqualPrecedence bool
	// AddJoinOnTrue ports parser.py:1842-1844: whether a JOIN without criteria receives ON TRUE
	// so its semantics remain explicit when transpiled to dialects that require a condition.
	AddJoinOnTrue bool
	// RegexpExtractDefaultGroup ports dialect.py:679 and build_regexp_extract at dialect.py:
	// 2343-2360: the capture group inserted when REGEXP_EXTRACT omits its group argument.
	RegexpExtractDefaultGroup int
	// RegexpExtractPositionOverflowReturnsNull ports dialect.py:682-683 and
	// build_regexp_extract at dialect.py:2343-2360: the canonical RegexpExtract node records
	// whether an overflowing position returns NULL rather than an empty string.
	RegexpExtractPositionOverflowReturnsNull bool
	// Functions ports the per-dialect FUNCTIONS class attribute (parsers/mysql.py,
	// parsers/postgres.py, etc.): a dialect-specific overlay of function-name -> builder
	// entries, merged OVER exp.FunctionByName at parse time (see parser/parser.go's
	// functions==nil merge site in parseFunctionCall). Unlike upstream - where every
	// dialect's FUNCTIONS is `{**parser.Parser.FUNCTIONS, ...overrides}`, a single
	// pre-merged class-level dict - this port keeps exp.FunctionByName as the one base map
	// and each Dialect only carries its OWN additions/overrides here, merged in on demand.
	// A nil/empty map (the base Dialect's zero value) means "no per-dialect overrides",
	// so p.dialect.Functions is safe to index even when unset.
	Functions map[string]func([]exp.Expression) exp.Expression
	// ValuesFollowedByParen ports the parser flag of the same name (parser.py:1801): whether a
	// bare VALUES keyword not immediately followed by "(" can be reparsed as a plain identifier
	// (e.g. `SELECT values`, `values.c`). MySQL overrides to False (parsers/mysql.py:303),
	// since MySQL treats VALUES specially even without a following paren.
	ValuesFollowedByParen bool
	// TrimPatternFirst ports the parser flag of the same name (parser.py:1860, default False;
	// parsers/spark2.py:22 overrides to True): in `TRIM(<pattern> FROM <expr>)` the FROM token,
	// not the position, decides whether the first argument is the pattern. Consumed by
	// parseTrim (parser/parser_functions.go).
	TrimPatternFirst bool
	// ChangeColumnAlterSyntax ports the parser flag of the same name (parsers/hive.py:60,
	// default False; parsers/spark2.py:23 overrides to True): whether Hive's
	// `CHANGE COLUMN a b <type>` accepts the `TYPE` keyword before the new type. Consumed by
	// parseHiveAlterTableChange (parser/stmt_alter.go).
	ChangeColumnAlterSyntax bool
	// PivotColumnNaming ports the parser flag of the same name (parser.py:1846, default
	// "agg_name_if_aliased"; parsers/spark2.py:24 overrides to "agg_name_if_multiple"): which
	// aggregation-name rule the PIVOT output-column inference uses. Recorded on the Pivot node
	// so the generator can rename the columns when a statement is written under a dialect that
	// names them differently (generator.py:2534-2546).
	PivotColumnNaming string
	// PrefixedPivotColumns ports PREFIXED_PIVOT_COLUMNS (base False; parsers/bigquery.py:190
	// True): the aggregation alias moves to the front of each inferred PIVOT output column
	// name, so `SUM(f) AS sum ... IN ('x')` yields sum_x instead of x_sum. Recorded on the
	// Pivot node alongside PivotColumnNaming (parser.py:5504).
	PrefixedPivotColumns bool
	// ConcatWsCoalesce ports CONCAT_WS_COALESCE (base False; duckdb True): CONCAT_WS ignores
	// NULL arguments.
	ConcatWsCoalesce bool
	// StrictJSONPathSyntax ports STRICT_JSON_PATH_SYNTAX (base True; duckdb False): whether a
	// bare `$` path may omit its brackets/quotes.
	StrictJSONPathSyntax bool
	// NumbersCanBeUnderscoreSeparated ports NUMBERS_CAN_BE_UNDERSCORE_SEPARATED (duckdb True).
	NumbersCanBeUnderscoreSeparated bool
	// UUIDIsStringType ports UUID_IS_STRING_TYPE (base True; duckdb False): annotate_types treats
	// UUID as a UUID type rather than text.
	UUIDIsStringType bool
	// MapKeysAreArbitraryExpressions ports MAP_KEYS_ARE_ARBITRARY_EXPRESSIONS (duckdb True): a
	// MAP constructor accepts arbitrary key/value expressions.
	MapKeysAreArbitraryExpressions bool
	// NormalizeExtractDateParts ports NORMALIZE_EXTRACT_DATE_PARTS (base False; duckdb True):
	// EXTRACT's date part is folded through DATE_PART_MAPPING before rendering.
	NormalizeExtractDateParts bool
	// ExponentOperators ports a non-empty EXPONENT table (duckdb's `**`/`^` -> Pow).
	ExponentOperators bool
	// RegexpRangeOperators ports duckdb's RANGE_PARSERS[TILDE] = RegexpFullMatch.
	RegexpRangeOperators bool
	// CaretAtStartsWith ports duckdb's RANGE_PARSERS[CARET_AT] = StartsWith.
	CaretAtStartsWith bool
	// ArrayOverlapsDAMP ports duckdb's RANGE_PARSERS[DAMP] = ArrayOverlaps.
	ArrayOverlapsDAMP bool
	// PrefixAlias ports DuckDB's `alias: expression` / `FROM alias: table` prefix aliases
	// (parsers/duckdb.py:341-364).
	PrefixAlias bool
	// LambdaKeyword ports DuckDB's `LAMBDA x : expr` lambda form (parsers/duckdb.py:327-338).
	LambdaKeyword bool
	// ZoneAwareTimestampConstructor ports the parser flag ZONE_AWARE_TIMESTAMP_CONSTRUCTOR
	// (parser.py:1831, default False; parsers/presto.py:61 overrides to True). Consumed at
	// parser.py:6186-6191 when parsing a typed-literal CAST: for a TIMESTAMP-typed literal whose
	// text carries a time-zone offset (TIME_ZONE_RE), the target type is promoted to TIMESTAMPTZ
	// (`TIMESTAMP '2020-01-01 00:00:00+00'` -> CAST(... AS TIMESTAMPTZ)). Presto is the first
	// in-scope override; base/mysql/postgres leave it at the False default.
	ZoneAwareTimestampConstructor bool
	// SupportsPartitionSelection ports the parser flag SUPPORTS_PARTITION_SELECTION
	// (parser.py:1810, default False): whether a table source in FROM may carry a trailing
	// PARTITION(...) selector, e.g. `SELECT * FROM t1 PARTITION(p0)`. MySQL overrides to True
	// (parsers/mysql.py:304); base/postgres leave it False.
	SupportsPartitionSelection bool
	// StringAliases ports the parser flag STRING_ALIASES (parser.py:1780, default False):
	// whether a trailing string constant is folded into an identifier alias in _parse_alias,
	// e.g. the SELECT projection `SELECT 1 'x'` -> `SELECT 1 AS "x"` (parser.py:8490-8492).
	// MySQL/tsql/sqlite override to True (parsers/mysql.py:302, tsql.py:324, sqlite.py:19);
	// base/postgres leave it False, so `SELECT 1 'x'` fails closed there - matching both
	// upstream and the real engines (PG rejects it, MySQL accepts it). This gates only the
	// projection/expression alias in parseAlias; the table-name and table-alias string forms
	// are gated separately by StringTableIdentifiers below.
	StringAliases bool
	// StringTableIdentifiers gates whether a bare string constant may stand in for a table NAME
	// (`FROM 'foo'`) or table ALIAS (`FROM t 'x'`) - _parse_table_part (parser.py:4668) and
	// _parse_table_alias (parser.py:4111), both of which upstream calls UNCONDITIONALLY (there is
	// no upstream flag for this). This port-introduced per-dialect gate defaults True (accept,
	// matching upstream's permissive default so base + the presto/trino/hive/athena partial
	// dialects are unchanged) but is False for postgres and mysql, whose real engines BOTH reject a
	// bare string in either position (verified PG 17.6 / MySQL 8.0.33) - a §1 correctness deviation
	// (follow the engine, not upstream). NB this is orthogonal to StringAliases: MySQL accepts the
	// projection string alias `SELECT 1 'x'` (StringAliases=True) yet rejects the table forms
	// (StringTableIdentifiers=False).
	StringTableIdentifiers bool
	// ReservedKeywords ports the generator class var RESERVED_KEYWORDS (generator.py:790,
	// empty for base/postgres; generators/mysql.py:339 for MySQL): unquoted identifiers whose
	// lowercased name is in this set are quoted on generation (identifier_sql, generator.py:
	// 1983). A nil/empty map (base's zero value) means "quote nothing extra".
	ReservedKeywords map[string]bool
	// StringEscape is STRING_ESCAPES[0] when the dialect lists an escape ahead of the quote
	// (Redshift's ["\\", "'"]); the generator escapes a quote with it. Empty means the
	// tokenizer's escapes decide, as before.
	StringEscape string
	// BitStart/BitEnd, HexStart/HexEnd, ByteStart/ByteEnd port BIT_START/BIT_END,
	// HEX_START/HEX_END, BYTE_START/BYTE_END (dialects/dialect.py:292-295): the
	// bitstringSQL/hexstringSQL/bytestringSQL delimiters, derived upstream from the FIRST
	// entry of the tokenizer's BIT_STRINGS/HEX_STRINGS/BYTE_STRINGS class attribute (e.g.
	// mysql/postgres BIT_STRINGS=[("b'","'"),...] -> BIT_START="b'", BIT_END="'"). Unlike
	// upstream's `_FORMAT_STRINGS` dict (Python dict insertion order), this port's
	// TokenizerConfig.FormatStrings is a plain Go map with no reliable iteration order, so
	// these are explicit per-dialect fields (set alongside QuoteStart/IdentifierStart above)
	// rather than derived from FormatStrings at Dialect-construction time. Empty string
	// (base's zero value) means "dialect has no such string family", matching upstream's
	// None default.
	BitStart, BitEnd   string
	HexStart, HexEnd   string
	ByteStart, ByteEnd string
	// HexStringIsIntegerType ports HEX_STRING_IS_INTEGER_TYPE (dialects/dialect.py:676) and
	// ByteStringIsBytesType ports BYTE_STRING_IS_BYTES_TYPE (dialects/dialect.py:724): neither
	// base, mysql, nor postgres overrides either flag away from its False default, so both
	// stay at the Go zero value for all three dialects here - present for 1:1 fidelity with
	// hexstringSQL/bytestringSQL's upstream signature, not because any in-scope dialect sets
	// them true.
	HexStringIsIntegerType bool
	ByteStringIsBytesType  bool
	// ConcatCoalesce ports CONCAT_COALESCE (dialects/dialect.py:404, postgres.py:15): whether
	// the dialect's native CONCAT function already coalesces NULL args to empty strings.
	// Consumed by parsePrimary's adjacent-string-literal rewrite (`'a' 'b'` -> Concat) to set
	// the built node's own "coalesce" arg to match; postgres is the only override (true) among
	// base/mysql/postgres.
	ConcatCoalesce bool
	// LimitIsTop ports generator.py:382 LIMIT_IS_TOP: a Limit renders as SELECT TOP n
	// instead of a trailing LIMIT clause (TSQL, Teradata).
	LimitIsTop bool
	// LimitFetch ports generator.py:343 LIMIT_FETCH: "LIMIT" or "FETCH". "FETCH" rewrites a
	// Limit into a FETCH clause; "LIMIT" rewrites a Fetch into a LIMIT clause.
	LimitFetch string
	// EnsureBools ports generator.py ENSURE_BOOLS: render boolean literals as 1/0 in
	// predicate positions and (1 = 1)/(1 = 0) elsewhere (TSQL has no boolean type).
	EnsureBools bool
	// CteRecursiveKeywordRequired ports generator.py CTE_RECURSIVE_KEYWORD_REQUIRED
	// (default True); TSQL omits the RECURSIVE keyword.
	CteRecursiveKeywordRequired bool
	// ReturningEnd ports generator.py:385 RETURNING_END (default True): whether RETURNING/
	// OUTPUT renders after the statement body or immediately after the target.
	ReturningEnd bool
	// SetOpModifiers ports parser.py:1867 SET_OP_MODIFIERS: which query modifiers attach to
	// the UNION node rather than its right operand. TSQL keeps only OFFSET there.
	SetOpModifiers map[string]bool
	// OptionalAliasTokenCTE ports parser.py:1899 OPTIONAL_ALIAS_TOKEN_CTE: when false
	// (ClickHouse, parsers/clickhouse.py:246), a CTE requires the explicit AS keyword
	// (parser.py:4218).
	OptionalAliasTokenCTE bool
	// PreserveOriginalNames ports dialect.py:405 PRESERVE_ORIGINAL_NAMES: a function parsed
	// through a known builder records its original spelling in meta["name"], and the
	// function fallback renders that name instead of the class's canonical name
	// (parser.py:7283-7284, generator.py:4680-4681).
	PreserveOriginalNames bool
	// StructDelimiter ports generator.py:679 STRUCT_DELIMITER: the delimiters around a
	// nested DataType's parameter list (default `<...>`; ClickHouse uses `(...)`).
	StructDelimiter [2]string
	// AutoRefreshBareIntervals ports generator.py:340 AUTO_REFRESH_BARE_INTERVALS: a
	// REFRESH schedule interval renders without the INTERVAL keyword (ClickHouse).
	AutoRefreshBareIntervals bool
	// ExtraFuncTokens ports a dialect's FUNC_TOKENS additions (parsers/clickhouse.py:330-336
	// adds AND/FILE/OR/SET): a keyword token in this set starts a function call.
	ExtraFuncTokens map[tokens.TokenType]bool
	// RemovedFunctions names base FUNCTIONS entries a dialect drops
	// (parsers/clickhouse.py:250-254 removes TRANSFORM and APPROX_TOP_SUM).
	RemovedFunctions map[string]bool
	// EscapedSequences ports a dialect's ESCAPED_SEQUENCES table (dialect.py:66-90, derived
	// from UNESCAPED_SEQUENCES): nil keeps the default control-character set.
	EscapedSequences map[byte]string
	// ExtraIdVarTokens ports ID_VAR_TOKENS additions (parsers/clickhouse.py:340-343 adds LIKE).
	ExtraIdVarTokens map[tokens.TokenType]bool
	// WithPropertiesPrefix ports generator.py:558 WITH_PROPERTIES_PREFIX (default "WITH";
	// StarRocks uses "PROPERTIES").
	WithPropertiesPrefix string
	// InsertOverwrite ports generator.py:471 INSERT_OVERWRITE (default " OVERWRITE TABLE";
	// StarRocks uses " OVERWRITE").
	InsertOverwrite string
	// IsBoolAllowed ports generator.py:376 IS_BOOL_ALLOWED (StarRocks disables it).
	IsBoolAllowed bool
	// UpdateStatementSupportsFrom ports generator.py:625 UPDATE_STATEMENT_SUPPORTS_FROM.
	UpdateStatementSupportsFrom bool
	// OnConditionEmptyBeforeError ports dialect.py:654 ON_CONDITION_EMPTY_BEFORE_ERROR:
	// MySQL parses `X ON EMPTY Y ON ERROR`, Oracle the opposite (parser.py:8333-8341).
	OnConditionEmptyBeforeError bool
	// PivotAliasWithAs ports generator.py PIVOT_ALIAS_WITH_AS (Oracle disables it).
	PivotAliasWithAs bool
	// UnpivotAliasesAreIdentifiers ports generator.py UNPIVOT_ALIASES_ARE_IDENTIFIERS
	// (Oracle disables it, keeping string/number aliases as literals).
	UnpivotAliasesAreIdentifiers bool
	// ArrayConcatIsVarLen ports generator.py ARRAY_CONCAT_IS_VAR_LEN: whether ARRAY_CONCAT takes
	// any number of arguments (true) or nests binary calls (false, Redshift).
	ArrayConcatIsVarLen bool
	// ArrayFuncsPropagatesNulls ports ARRAY_FUNCS_PROPAGATES_NULLS (dialects/dialect.py): whether
	// the array functions return NULL when any input array is NULL (Redshift).
	ArrayFuncsPropagatesNulls bool
	// LogBaseFirst ports generator.py LOG_BASE_FIRST (base True; bigquery False): whether the
	// LOG base precedes the value in the canonical node and is swapped when rendering.
	LogBaseFirst bool
	// HexLowercase ports HEX_LOWERCASE (bigquery True): HEX renders lowercase hex digits.
	HexLowercase bool
	// HexFunc ports generator.py HEX_FUNC (base "HEX"; bigquery "TO_HEX").
	HexFunc string
	// JSONExtractScalarScalarOnly ports JSON_EXTRACT_SCALAR_SCALAR_ONLY (bigquery True): the
	// parser records scalar_only on the JSONExtractScalar node.
	JSONExtractScalarScalarOnly bool
	// JSONPathSingleDotIsWildcard ports JSON_PATH_SINGLE_DOT_IS_WILDCARD (bigquery True); the
	// JSONPath tokenizer is not ported yet, so the flag is recorded only.
	JSONPathSingleDotIsWildcard bool
	// LeastGreatestIgnoresNulls ports LEAST_GREATEST_IGNORES_NULLS (base True; bigquery False):
	// recorded on the parsed Least/Greatest nodes.
	LeastGreatestIgnoresNulls bool
	// DefaultNullType ports DEFAULT_NULL_TYPE (bigquery BIGINT): the type annotate_types gives a
	// bare NULL.
	DefaultNullType exp.DType
	// FormatMapping / InverseFormatMapping port FORMAT_MAPPING (bigquery) and its reverse,
	// the datetime-format tables used by the format_time inversions.
	FormatMapping        map[string]string
	InverseFormatMapping map[string]string
	// SetOpExplicitDistinct ports bigquery's SET_OP_DISTINCT_BY_DEFAULT = None for every set
	// operation: DISTINCT/ALL must always be rendered explicitly.
	SetOpExplicitDistinct bool
	// IgnoreNullsInFunc ports IGNORE_NULLS_IN_FUNC (base False; bigquery True): IGNORE/RESPECT
	// NULLS renders inside the aggregate call rather than after it.
	IgnoreNullsInFunc bool
	// IgnoreNullsBeforeOrder ports IGNORE_NULLS_BEFORE_ORDER (base True): when the aggregate has
	// ORDER BY/LIMIT modifiers, the inline IGNORE NULLS goes before them.
	IgnoreNullsBeforeOrder bool
	// UnnestWithOrdinality ports UNNEST_WITH_ORDINALITY (base True; bigquery False).
	UnnestWithOrdinality bool
	// InUnnestOpBare ports in_unnest_op (generator.py: in_unnest_op defaults to a wrapped
	// SELECT; bigquery renders the bare UNNEST expression).
	InUnnestOpBare bool
	// SupportsTableAliasColumns ports SUPPORTS_TABLE_ALIAS_COLUMNS (base True; bigquery False):
	// a table alias column list is dropped when false.
	SupportsTableAliasColumns bool
	// SupportsNamedCTEColumns ports SUPPORTS_NAMED_CTE_COLUMNS (generator.py:459, base True;
	// bigquery False): a CTE alias keeps its column list even when table-alias columns are not
	// supported.
	SupportsNamedCTEColumns bool
	// QualifyBeforeWindow ports bigquery's AFTER_HAVING_MODIFIER_TRANSFORMS order: QUALIFY is
	// rendered before the WINDOW clause (base renders WINDOW first).
	QualifyBeforeWindow bool
	// WildcardTableParts ports bigquery's _parse_table_parts override, which always forwards
	// wildcard=True (parsers/bigquery.py:451) so `x.*` / `x.y*` join the last part.
	WildcardTableParts bool
	// QualifiedTablesCaseSensitive ports BigQuery.normalize_identifier
	// (dialects/bigquery.py:132-159): under CASE_INSENSITIVE, UDF names and qualified table
	// names keep their case, so `FROM Coordinates, \`Coordinates.position\`` does not fold
	// the qualified reference.
	QualifiedTablesCaseSensitive bool
	// TimeFormat ports TIME_FORMAT (snowflake's `'YYYY-MM-DD HH24:MI:SS'`).
	TimeFormat string
	// UnescapedSequences ports UNESCAPED_SEQUENCES (snowflake maps \a and \v).
	UnescapedSequences map[string]string
	// ArrayAggIncludesNulls ports ARRAY_AGG_INCLUDES_NULLS.
	ArrayAggIncludesNulls *bool
	// StarIlikeBackslashEscape ports STAR_ILIKE_BACKSLASH_ESCAPE (snowflake True).
	StarIlikeBackslashEscape bool
	// JSONExtractRequiresJSONExpression ports JSON_EXTRACT_REQUIRES_JSON_EXPRESSION.
	JSONExtractRequiresJSONExpression bool
	// DefaultSamplingMethod ports DEFAULT_SAMPLING_METHOD.
	DefaultSamplingMethod string
	// IdentifyPivotStrings ports IDENTIFY_PIVOT_STRINGS (snowflake True).
	IdentifyPivotStrings bool
	// DirectedJoins ports DIRECTED_JOINS (snowflake True): `[kind] DIRECTED JOIN`.
	DirectedJoins bool
	// RlikeFullMatch ports snowflake RANGE_PARSERS[RLIKE] building RegexpLike(full_match=true).
	RlikeFullMatch bool
	// MapIntervalUnits maps an interval unit through DATE_PART_MAPPING (snowflake W -> WEEK).
	MapIntervalUnits bool
	// JSONKeyValuePairSep ports JSON_KEY_VALUE_PAIR_SEP (base ":"; bigquery/mysql/snowflake/...
	// ","): the separator jsonkeyvalue_sql emits between a JSON object key and value.
	JSONKeyValuePairSep string
	// MoveCTEsToTopLevel ports EXPRESSIONS_WITHOUT_NESTED_CTES  (generator.py:849); Hive
	// (generators/hive.py:246-251) lists Insert/Select/Subquery. A root of one of these kinds
	// hoists every nested WITH before generation (generator.py:986-995). nil disables it, which
	// matches the empty base set.
	MoveCTEsToTopLevel map[exp.Kind]bool
	// DeclareDefaultAssignment ports generator.py:620 DECLARE_DEFAULT_ASSIGNMENT, the token
	// between a DECLARE item's type and its default value (base "="; bigquery/trino "DEFAULT").
	DeclareDefaultAssignment string
	// HasDistinctArrayConstructors ports HAS_DISTINCT_ARRAY_CONSTRUCTORS: ARRAY[...] and
	// ARRAY(...) are different constructors, so the parser records which one was written.
	HasDistinctArrayConstructors bool
	// StarExcludeRequiresDerivedTable ports generator.py STAR_EXCLUDE_REQUIRES_DERIVED_TABLE
	// (true by default; Redshift renders a select-level EXCLUDE inline).
	StarExcludeRequiresDerivedTable bool
}

func Base() *Dialect {
	datePartMapping := baseDatePartMapping()
	return &Dialect{
		Name: "base",
		// dialects/dialect.py:754 SAFE_TO_ELIMINATE_DOUBLE_NEGATION = True.
		SafeToEliminateDoubleNegation:   true,
		ArrayConcatIsVarLen:             true,
		StarExcludeRequiresDerivedTable: true,
		// generator.py LOG_BASE_FIRST = True, HEX_FUNC = "HEX" (base).
		LogBaseFirst:              true,
		HexFunc:                   "HEX",
		IgnoreNullsBeforeOrder:    true,
		UnnestWithOrdinality:      true,
		SupportsTableAliasColumns: true,
		SupportsNamedCTEColumns:   true,
		// dialects/dialect.py LEAST_GREATEST_IGNORES_NULLS = True (base).
		LeastGreatestIgnoresNulls: true,
		// parser.py:1880 COLON_CHAIN_IS_SINGLE_EXTRACT = True (base).
		ColonChainIsSingleExtract: true,
		// generator.py:505-561 JSON_PATH_* / QUOTE_JSON_PATH defaults.
		QuoteJSONPath:                 true,
		JSONPathBracketedKeySupported: true,
		QuoteStart:                    "'",
		QuoteEnd:                      "'",
		IdentifierStart:               "\"",
		IdentifierEnd:                 "\"",
		TokenizerConfig:               tokens.BaseConfig(),
		NormalizationStrategy:         Lowercase,
		DPipeIsStringConcat:           true,
		NullOrdering:                  "nulls_are_small",
		DatePartMapping:               datePartMapping,
		ValidIntervalUnits:            validIntervalUnits(datePartMapping),
		SupportsUserDefinedTypes:      true,
		SupportsFixedSizeArrays:       false,
		// Accept a bare string as a table name/alias by default (upstream is unconditional here);
		// postgres/mysql override to false to match their real engines. See the field doc.
		StringTableIdentifiers: true,
		// dialect.py:664 SUPPORTS_LIMIT_ALL = False (base); postgres (dialects/postgres.py) and
		// presto (dialects/presto.py:25) both override to True.
		SupportsLimitAll: false,
		// dialect.py:670 SUPPORTS_VALUES_DEFAULT = True (base); presto overrides to False
		// (dialects/presto.py:26).
		SupportsValuesDefault: true,
		// dialect.py:757 NORMALIZE_NOT_NULL = True (base); postgres overrides to False.
		NormalizeNotNull: true,
		// generator.py:374 DUPLICATE_KEY_UPDATE_WITH_SET = True (base); MySQL overrides to
		// False (generators/mysql.py:137).
		DuplicateKeyUpdateWithSet: true,
		// generator.py:320 WRAP_DERIVED_VALUES = True (base); MySQL overrides to False
		// (generators/mysql.py:148).
		WrapDerivedValues: true,
		// generator.py:324 CREATE_FUNCTION_RETURN_AS = True (base); spark2 overrides to False
		// (generators/spark2.py:214).
		CreateFunctionReturnAs: true,
		// generator.py:397 VALUES_AS_TABLE = True (base); MySQL overrides to False
		// (generators/mysql.py:139), rewriting table-source VALUES into SELECT unions.
		ValuesAsTable:        true,
		IntervalSpans:        true,
		NormalizeFunctions:   "upper",
		AliasPostTablesample: false,
		AliasPostVersion:     true,
		UnnestColumnOnly:     false,
		Pseudocolumns:        map[string]bool{},
		IndexOffset:          0,
		// generator.py:344 RENAME_TABLE_WITH_DB = True (base); postgres overrides to False
		// (generators/postgres.py:234).
		RenameTableWithDB: true,
		// generator.py:475/478 SUPPORTS_MODIFY_COLUMN/SUPPORTS_CHANGE_COLUMN = False (base);
		// MySQL overrides both to True (generators/mysql.py:127-128).
		SupportsModifyColumn: false,
		SupportsChangeColumn: false,
		// generator.py:480 SUPPORTS_CREATE_TABLE_LIKE = True (base); Materialize overrides
		// to False (generators/materialize.py:15).
		SupportsCreateTableLike: true,
		// generator.py:608 SUPPORTS_BETWEEN_FLAGS = False (base); postgres overrides to True
		// (generators/postgres.py:280), Materialize/RisingWave back to False.
		SupportsBetweenFlags: false,
		// generator.py:684 EXPRESSION_PRECEDES_PROPERTIES_CREATABLES = set() (base); only
		// the risingwave SINK creatable overrides it (generators/risingwave.py:22).
		ExpressionPrecedesPropertiesCreatables: map[string]bool{},
		// parser.py:1896 WRAPPED_TRANSFORM_COLUMN_CONSTRAINT = True (base); only risingwave
		// overrides it to False (parsers/risingwave.py:11).
		WrappedTransformColumnConstraint: true,
		// generators/mysql.py:149 VARCHAR_REQUIRES_SIZE = True; not a base Generator
		// attribute at all (only referenced within the MySQL generator family), so base
		// (and postgres, which doesn't override it) is False.
		VarcharRequiresSize: false,
		// dialects/dialect.py:701 ALTER_TABLE_SUPPORTS_CASCADE = False (base); only the
		// Hive-family dialects (out of scope here) override to True.
		AlterTableSupportsCascade: false,
		// dialects/dialect.py:709 ALTER_TABLE_ADD_REQUIRED_FOR_EACH_COLUMN = True (base).
		AlterTableAddRequiredForEachColumn: true,
		// parser.py:1819 ALTER_RENAME_REQUIRES_COLUMN = True (base).
		AlterRenameRequiresColumn: true,
		// parser.py:1822 ALTER_TABLE_PARTITIONS = False (base); only Hive overrides to True.
		AlterTablePartitions: false,
		// parser.py:1860 TRIM_PATTERN_FIRST = False (base); spark2 overrides to True
		// (parsers/spark2.py:22).
		TrimPatternFirst: false,
		// parsers/hive.py:60 CHANGE_COLUMN_ALTER_SYNTAX = False (base); spark2 overrides to
		// True (parsers/spark2.py:23).
		ChangeColumnAlterSyntax: false,
		// parser.py:1846 PIVOT_COLUMN_NAMING = "agg_name_if_aliased" (base); spark2 overrides
		// to "agg_name_if_multiple" (parsers/spark2.py:24).
		PivotColumnNaming:    "agg_name_if_aliased",
		StrictJSONPathSyntax: true,
		UUIDIsStringType:     true,
		// generator.py:400 ALTER_TABLE_INCLUDE_COLUMN_KEYWORD = True (base).
		AlterTableIncludeColumnKeyword: true,
		// generator.py:412 COMPUTED_COLUMN_WITH_TYPE = True (base).
		ComputedColumnWithType: true,
		// generator.py:568 ALTER_SET_WRAPPED = False (base).
		AlterSetWrapped: false,
		// generator.py:582 ALTER_SET_TYPE = "SET DATA TYPE" (base).
		AlterSetType: "SET DATA TYPE",
		// generator.py:620 DECLARE_DEFAULT_ASSIGNMENT = "=" (base).
		DeclareDefaultAssignment: "=",
		// dialects/dialect.py:762 INITCAP_DEFAULT_DELIMITER_CHARS (base); hive and spark2
		// override it (dialects/hive.py:35, dialects/spark2.py:29).
		InitcapDefaultDelimiterChars: " \t\n\r\f\v!\"#$%&'()*+,\\-./:;<=>?@\\[\\]^_`{|}~",
		// generator.py:619 SUPPORTS_DROP_ALTER_ICEBERG_PROPERTY = True (base).
		SupportsDropAlterIcebergProperty: true,
		// generator.py:335 SINGLE_STRING_INTERVAL = False (base); postgres overrides to True
		// (generators/postgres.py:233).
		SingleStringInterval: false,
		// generator.py:335 INTERVAL_ALLOWS_PLURAL_FORM = True (base); mysql overrides to False
		// (generators/mysql.py:132).
		IntervalAllowsPluralForm: true,
		// generator.py:667 PARAMETER_TOKEN = "@" (base); postgres overrides to "$"
		// (generators/postgres.py:240).
		ParameterToken: "@",
		// generator.py:314 LOCKING_READS_SUPPORTED = False (base).
		LockingReadsSupported: false,
		// dialects/dialect.py:342 TABLESAMPLE_SIZE_IS_PERCENT = False (base); postgres
		// overrides to True (dialects/postgres.py:20).
		TablesampleSizeIsPercent: false,
		// generator.py:421 TABLESAMPLE_SIZE_IS_ROWS = True (base); postgres overrides to
		// False (generators/postgres.py:242).
		TablesampleSizeIsRows: true,
		// generator.py:430 TABLESAMPLE_SEED_KEYWORD = "SEED" (base); postgres overrides to
		// "REPEATABLE" (generators/postgres.py:243).
		TablesampleSeedKeyword: "SEED",
		// generator.py:418 TABLESAMPLE_REQUIRES_PARENS = True (base); no dialect override
		// within base/mysql/postgres.
		TablesampleRequiresParens: true,
		// generator.py:427 TABLESAMPLE_WITH_METHOD = True (base); no dialect override
		// within base/mysql/postgres.
		TablesampleWithMethod: true,
		// generator.py:424 TABLESAMPLE_KEYWORDS = "TABLESAMPLE" (base); no dialect override
		// within base/mysql/postgres.
		TablesampleKeywords: "TABLESAMPLE",
		// generator.py:526 COPY_HAS_INTO_KEYWORD = True (base); postgres overrides to False
		// (generators/postgres.py:251).
		CopyHasIntoKeyword: true,
		// generator.py:520 COPY_PARAMS_ARE_WRAPPED = True (base).
		CopyParamsAreWrapped: true,
		// dialects/dialect.py:363 COPY_PARAMS_ARE_CSV = True (base).
		CopyParamsAreCsv: true,
		// generator.py:523 COPY_PARAMS_EQ_REQUIRED = False (base).
		CopyParamsEqRequired: false,
		// parsers/postgres.py:191 JSON_ARROWS_REQUIRE_JSON_TYPE = False (base/mysql); postgres
		// overrides to True.
		JSONArrowsRequireJSONType: false,
		// generator.py:488 JSON_TYPE_REQUIRED_FOR_EXTRACTION = False (base); mysql
		// (generators/mysql.py:142) and postgres (generators/postgres.py:245) both override to
		// True.
		JSONTypeRequiredForExtraction: false,
		// parser.py:1755 STRICT_CAST = True (base); Hive overrides to False.
		StrictCast: true,
		// parser.py:1765 LOG_DEFAULTS_TO_LN = False (base); Hive overrides to True.
		LogDefaultsToLn: false,
		// parser.py:1824-1828 JOINS_HAVE_EQUAL_PRECEDENCE = False (base); Hive overrides
		// to True.
		JoinsHaveEqualPrecedence: false,
		// parser.py:1842-1844 ADD_JOIN_ON_TRUE = False (base); Hive overrides to True.
		AddJoinOnTrue: false,
		// dialect.py:679 REGEXP_EXTRACT_DEFAULT_GROUP = 0 (base); Hive overrides to 1.
		RegexpExtractDefaultGroup: 0,
		// dialect.py:682-683 REGEXP_EXTRACT_POSITION_OVERFLOW_RETURNS_NULL = True.
		RegexpExtractPositionOverflowReturnsNull: true,
		// parser.py:1801 VALUES_FOLLOWED_BY_PAREN = True (base); MySQL overrides to False
		// (parsers/mysql.py:303).
		ValuesFollowedByParen: true,
		// parser.py:1831 ZONE_AWARE_TIMESTAMP_CONSTRUCTOR = False (base); presto overrides to
		// True (parsers/presto.py:61).
		ZoneAwareTimestampConstructor: false,
		// generator.py:343 LIMIT_FETCH = "ALL" (base: no Limit/Fetch rewriting).
		LimitFetch: "ALL",
		// generator.py CTE_RECURSIVE_KEYWORD_REQUIRED defaults to True upstream.
		CteRecursiveKeywordRequired: true,
		// generator.py:385 RETURNING_END defaults to True upstream.
		ReturningEnd: true,
		// parser.py:1867 SET_OP_MODIFIERS defaults to order/limit/offset.
		SetOpModifiers: map[string]bool{"order": true, "limit": true, "offset": true},
		// parser.py:1899 OPTIONAL_ALIAS_TOKEN_CTE defaults to True.
		OptionalAliasTokenCTE: true,
		// generator.py:679 STRUCT_DELIMITER defaults to ("<", ">").
		StructDelimiter: [2]string{"<", ">"},
		// generator.py:558/471/376/625 generator flag defaults.
		WithPropertiesPrefix:         "WITH",
		InsertOverwrite:              " OVERWRITE TABLE",
		IsBoolAllowed:                true,
		UpdateStatementSupportsFrom:  true,
		OnConditionEmptyBeforeError:  true,
		PivotAliasWithAs:             true,
		UnpivotAliasesAreIdentifiers: true,
	}
}

var datePartMapping = map[string]string{
	"Y":                  "YEAR",
	"YY":                 "YEAR",
	"YYY":                "YEAR",
	"YYYY":               "YEAR",
	"YR":                 "YEAR",
	"YEARS":              "YEAR",
	"YRS":                "YEAR",
	"MM":                 "MONTH",
	"MON":                "MONTH",
	"MONS":               "MONTH",
	"MONTHS":             "MONTH",
	"D":                  "DAY",
	"DD":                 "DAY",
	"DAYS":               "DAY",
	"DAYOFMONTH":         "DAY",
	"DAY OF WEEK":        "DAYOFWEEK",
	"WEEKDAY":            "DAYOFWEEK",
	"DOW":                "DAYOFWEEK",
	"DW":                 "DAYOFWEEK",
	"WEEKDAY_ISO":        "DAYOFWEEKISO",
	"DOW_ISO":            "DAYOFWEEKISO",
	"DW_ISO":             "DAYOFWEEKISO",
	"DAYOFWEEK_ISO":      "DAYOFWEEKISO",
	"DAY OF YEAR":        "DAYOFYEAR",
	"DOY":                "DAYOFYEAR",
	"DY":                 "DAYOFYEAR",
	"W":                  "WEEK",
	"WK":                 "WEEK",
	"WEEKOFYEAR":         "WEEK",
	"WOY":                "WEEK",
	"WY":                 "WEEK",
	"WEEK_ISO":           "WEEKISO",
	"WEEKOFYEARISO":      "WEEKISO",
	"WEEKOFYEAR_ISO":     "WEEKISO",
	"Q":                  "QUARTER",
	"QTR":                "QUARTER",
	"QTRS":               "QUARTER",
	"QUARTERS":           "QUARTER",
	"H":                  "HOUR",
	"HH":                 "HOUR",
	"HR":                 "HOUR",
	"HOURS":              "HOUR",
	"HRS":                "HOUR",
	"M":                  "MINUTE",
	"MI":                 "MINUTE",
	"MIN":                "MINUTE",
	"MINUTES":            "MINUTE",
	"MINS":               "MINUTE",
	"S":                  "SECOND",
	"SEC":                "SECOND",
	"SECONDS":            "SECOND",
	"SECS":               "SECOND",
	"MS":                 "MILLISECOND",
	"MSEC":               "MILLISECOND",
	"MSECS":              "MILLISECOND",
	"MSECOND":            "MILLISECOND",
	"MSECONDS":           "MILLISECOND",
	"MILLISEC":           "MILLISECOND",
	"MILLISECS":          "MILLISECOND",
	"MILLISECON":         "MILLISECOND",
	"MILLISECONDS":       "MILLISECOND",
	"US":                 "MICROSECOND",
	"USEC":               "MICROSECOND",
	"USECS":              "MICROSECOND",
	"MICROSEC":           "MICROSECOND",
	"MICROSECS":          "MICROSECOND",
	"USECOND":            "MICROSECOND",
	"USECONDS":           "MICROSECOND",
	"MICROSECONDS":       "MICROSECOND",
	"NS":                 "NANOSECOND",
	"NSEC":               "NANOSECOND",
	"NANOSEC":            "NANOSECOND",
	"NSECOND":            "NANOSECOND",
	"NSECONDS":           "NANOSECOND",
	"NANOSECS":           "NANOSECOND",
	"EPOCH_SECOND":       "EPOCH",
	"EPOCH_SECONDS":      "EPOCH",
	"EPOCH_MILLISECONDS": "EPOCH_MILLISECOND",
	"EPOCH_MICROSECONDS": "EPOCH_MICROSECOND",
	"EPOCH_NANOSECONDS":  "EPOCH_NANOSECOND",
	"TZH":                "TIMEZONE_HOUR",
	"TZM":                "TIMEZONE_MINUTE",
	"DEC":                "DECADE",
	"DECS":               "DECADE",
	"DECADES":            "DECADE",
	"MIL":                "MILLENNIUM",
	"MILS":               "MILLENNIUM",
	"MILLENIA":           "MILLENNIUM", //nolint:misspell // upstream spells the SQL interval unit MILLENIA
	"C":                  "CENTURY",
	"CENT":               "CENTURY",
	"CENTS":              "CENTURY",
	"CENTURIES":          "CENTURY",
}

func baseDatePartMapping() map[string]string {
	out := make(map[string]string, len(datePartMapping))
	for key, value := range datePartMapping {
		out[key] = value
	}
	return out
}

func validIntervalUnits(mapping map[string]string) map[string]bool {
	out := map[string]bool{}
	for key, value := range mapping {
		out[key] = true
		out[value] = true
	}
	return out
}

// dialectTemplates caches fully constructed dialects. A Dialect is treated as immutable
// after construction (the parser and generator only read it), so callers receive a shallow
// copy of the template: field writes are isolated, and the expensive tokenizer config
// (keyword maps + trie) and date-part tables are shared read-only.
var dialectTemplates sync.Map

// dialectByName constructs a dialect from its bare name (no settings), returning an isolated
// copy so a caller can mutate fields without disturbing the cached template.
func dialectByName(name string) (*Dialect, error) {
	d, err := cachedDialect(strings.ToLower(strings.TrimSpace(name)))
	if err != nil {
		return nil, err
	}
	copy := *d
	return &copy, nil
}

// cachedDialect returns the shared cached template for a bare name, building and storing it on
// first use. The returned value must be treated as immutable.
func cachedDialect(key string) (*Dialect, error) {
	if cached, ok := dialectTemplates.Load(key); ok {
		return cached.(*Dialect), nil
	}
	d, err := buildDialectByName(key)
	if err != nil {
		return nil, err
	}
	// Store only the winner of a concurrent first resolution: both builders are equivalent,
	// but keeping one pointer keeps the cache canonical.
	actual, _ := dialectTemplates.LoadOrStore(key, d)
	return actual.(*Dialect), nil
}

func buildDialectByName(name string) (*Dialect, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "base":
		return Base(), nil
	case "bigquery":
		return BigQuery(), nil
	case "duckdb":
		return DuckDB(), nil
	case "snowflake":
		return Snowflake(), nil
	case "mysql":
		return MySQL(), nil
	case "mariadb":
		// Beyond upstream, which has no MariaDB dialect: an alias of MySQL, so the result is
		// the MySQL dialect itself (Name "mysql") with all of its behavior. MariaDB-only grammar
		// is not modeled yet. See DEVIATIONS "Opt-in behavioral extensions".
		return MySQL(), nil
	case "postgres":
		return Postgres(), nil
	case "presto":
		return Presto(), nil
	case "trino":
		return Trino(), nil
	case "hive":
		return Hive(), nil
	case "athena":
		return Athena(), nil
	case "tsql":
		return TSQL(), nil
	case "clickhouse":
		return Clickhouse(), nil
	case "starrocks":
		return StarRocks(), nil
	case "doris":
		return Doris(), nil
	case "oracle":
		return Oracle(), nil
	case "redshift":
		return Redshift(), nil
	case "materialize":
		return Materialize(), nil
	case "risingwave":
		return RisingWave(), nil
	case "spark2":
		return Spark2(), nil
	case "sqlite":
		return SQLite(), nil
	case "spark":
		return Spark(), nil
	case "databricks":
		return Databricks(), nil
	default:
		return nil, fmt.Errorf("unknown dialect %q", name)
	}
}

// parseNormalizationStrategy maps a canonical strategy name (case-insensitive; the
// upstream UPPER_SNAKE spellings, plus this port's two MySQL strategies) to the enum.
// parseSettingBool parses a boolean dialect-setting value (`true`/`false`/`1`/`0`, case-insensitive).
func parseSettingBool(key, v string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "on":
		return true, nil
	case "false", "0", "off":
		return false, nil
	default:
		return false, fmt.Errorf("dialect setting %q wants a boolean (true/false), got %q", key, v)
	}
}

func parseNormalizationStrategy(v string) (NormalizationStrategy, error) {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "LOWERCASE":
		return Lowercase, nil
	case "UPPERCASE":
		return Uppercase, nil
	case "CASE_SENSITIVE":
		return CaseSensitive, nil
	case "CASE_INSENSITIVE":
		return CaseInsensitive, nil
	case "CASE_INSENSITIVE_UPPERCASE":
		return CaseInsensitiveUppercase, nil
	case "MYSQL_CASE_INSENSITIVE":
		return MySQLCaseInsensitive, nil
	case "MYSQL_CASE_SENSITIVE_TABLE_NAMES":
		return MySQLCaseSensitiveTableNames, nil
	default:
		return 0, fmt.Errorf("unknown normalization_strategy %q", v)
	}
}

// normalizationStrategyToken is the reverse of parseNormalizationStrategy: it maps a
// strategy back to the settings-string token GetOrRaise accepts. Used by SettingsString so a
// typed *Dialect round-trips through the string-based settings resolver.
func normalizationStrategyToken(s NormalizationStrategy) (string, bool) {
	switch s {
	case Lowercase:
		return "lowercase", true
	case Uppercase:
		return "uppercase", true
	case CaseSensitive:
		return "case_sensitive", true
	case CaseInsensitive:
		return "case_insensitive", true
	case CaseInsensitiveUppercase:
		return "case_insensitive_uppercase", true
	case MySQLCaseInsensitive:
		return "mysql_case_insensitive", true
	case MySQLCaseSensitiveTableNames:
		return "mysql_case_sensitive_table_names", true
	default:
		return "", false
	}
}

// SettingsString serializes the dialect to the canonical "name, normalization_strategy=..."
// form that GetOrRaise round-trips. It captures the state that participates in identifier
// resolution — the dialect name (tokenizer/quoting) and normalization_strategy — which is
// the complete set of *Dialect state the qualify pipeline consults. Other tweaked fields are
// not serialized (they don't affect qualify), so a *Dialect handed to CanonicalString is
// honored for those two facets.
func (d *Dialect) SettingsString() string {
	if d == nil {
		return ""
	}
	out := d.Name
	if token, ok := normalizationStrategyToken(d.NormalizationStrategy); ok {
		out += ", normalization_strategy=" + token
	}
	if d.OpaqueFunctions {
		out += ", opaque_functions=true"
	}
	return out
}

// DialectType is the polymorphic dialect argument accepted throughout the public API,
// mirroring upstream sqlglot's DialectType (dialect.py: Union[str, Dialect, Type[Dialect], None]).
// A value is one of:
//
//   - nil       — the base, dialect-agnostic dialect
//   - string    — a dialect name, optionally followed by comma-separated settings, e.g.
//     "mysql" or "mysql, mysql_version=8.0.35, normalization_strategy=..."
//   - *Dialect  — an already-resolved dialect (returned as-is by GetOrRaise)
//
// It is a type alias for any, so it is fully interchangeable with any: adding it to a
// signature never breaks an existing string/*Dialect/nil caller. On a hot path prefer
// passing a *Dialect (resolve once via GetOrRaise) — each string value is re-resolved into
// a fresh *Dialect on every call, so reusing one avoids that repeated work.
//
// The underlying alias is declared in the root expressions package (as expressions.DialectType)
// so that package's builders and the parse/generate hooks can name the same type without
// importing dialects (which would form an import cycle, since dialects imports expressions).
// This is the canonical public name; the two are identical and interchangeable.
type DialectType = exp.DialectType

// CanonicalString reduces a DialectType-style value (nil | string | *Dialect) to a string the
// string-based resolvers understand, mirroring upstream sqlglot's polymorphic DialectType at
// the boundaries that still thread a dialect as a string (schema, identifier parsing).
func CanonicalString(dialect DialectType) (string, error) {
	switch v := dialect.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case *Dialect:
		return v.SettingsString(), nil
	case Dialect:
		return v.SettingsString(), nil
	default:
		return "", fmt.Errorf("invalid dialect %T (want string or *dialects.Dialect)", dialect)
	}
}

// GetOrRaise resolves a DialectType-style value to a *Dialect, mirroring upstream sqlglot's
// Dialect.get_or_raise(DialectType): it accepts nil (base), a string (bare name or the
// comma-separated settings form, e.g. "mysql, normalization_strategy=mysql_case_insensitive"),
// or an already-built *Dialect (returned as-is). The supported string settings are
// normalization_strategy and mysql_version (upstream also has "version", not modeled here).
//
// A name is served from a cached template as a shallow copy: field writes are isolated per
// call, so a per-call setting override cannot leak, but the tokenizer config and mapping
// tables are shared read-only. Callers must not mutate the maps reached through the returned
// Dialect; pass a *Dialect you built yourself for a fully private instance.
func GetOrRaise(dialect DialectType) (*Dialect, error) {
	switch v := dialect.(type) {
	case nil:
		return getOrRaiseString("")
	case string:
		return getOrRaiseString(v)
	case *Dialect:
		if v == nil {
			return getOrRaiseString("")
		}
		return v, nil
	case Dialect:
		return &v, nil
	default:
		return nil, fmt.Errorf("invalid dialect %T (want string or *dialects.Dialect)", dialect)
	}
}

// GetOrRaiseShared resolves a dialect for read-only use: a bare name returns the shared cached
// template instead of the defensive copy GetOrRaise makes, so it allocates nothing per call.
// The parse and generate entry points use it because they only read the dialect. The returned
// value must never be mutated; settings variants and callers that need to change a field must
// go through GetOrRaise.
func GetOrRaiseShared(dialect DialectType) (*Dialect, error) {
	switch v := dialect.(type) {
	case nil:
		return sharedDialect("")
	case string:
		return sharedDialect(v)
	case *Dialect:
		if v == nil {
			return sharedDialect("")
		}
		return v, nil
	case Dialect:
		return &v, nil
	default:
		return nil, fmt.Errorf("invalid dialect %T (want string or *dialects.Dialect)", dialect)
	}
}

func sharedDialect(name string) (*Dialect, error) {
	base, _, hasSettings := strings.Cut(name, ",")
	if hasSettings {
		// A settings variant mutates fields, so it needs the isolated copy path.
		return getOrRaiseString(name)
	}
	return cachedDialect(strings.ToLower(strings.TrimSpace(base)))
}

func getOrRaiseString(name string) (*Dialect, error) {
	base, rest, hasSettings := strings.Cut(name, ",")
	d, err := dialectByName(base)
	if err != nil {
		return nil, err
	}
	if !hasSettings {
		return d, nil
	}
	for _, part := range strings.Split(rest, ",") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		key, val, hasVal := strings.Cut(part, "=")
		key = strings.ToLower(strings.TrimSpace(key))
		switch key {
		case "normalization_strategy":
			if !hasVal {
				return nil, fmt.Errorf("dialect setting %q requires a value", key)
			}
			ns, err := parseNormalizationStrategy(val)
			if err != nil {
				return nil, err
			}
			d.NormalizationStrategy = ns
		case "mysql_version":
			if !hasVal {
				return nil, fmt.Errorf("dialect setting %q requires a value", key)
			}
			version, err := parseMySQLVersion(val)
			if err != nil {
				return nil, err
			}
			d.MySQLVersion = &version
		case "mysql_ansi_quotes":
			if !hasVal {
				return nil, fmt.Errorf("dialect setting %q requires a value", key)
			}
			on, err := parseSettingBool(key, val)
			if err != nil {
				return nil, err
			}
			d.MySQLAnsiQuotes = &on
		case "opaque_functions":
			if !hasVal {
				return nil, fmt.Errorf("dialect setting %q requires a value", key)
			}
			on, err := parseSettingBool(key, val)
			if err != nil {
				return nil, err
			}
			d.OpaqueFunctions = on
		default:
			return nil, fmt.Errorf("unsupported dialect setting %q (supported: normalization_strategy, mysql_version, mysql_ansi_quotes, opaque_functions)", key)
		}
	}
	// Apply the ANSI_QUOTES tokenizer change once, from the FINAL flag value — a duplicated setting
	// (`mysql_ansi_quotes=true, …=false`) must land on last-value-wins, not eagerly mutate on the first
	// `true` and leave stale identifier state behind.
	if d.MySQLAnsiQuotes != nil && *d.MySQLAnsiQuotes {
		applyMySQLAnsiQuotes(d)
	}
	return d, nil
}

func (d *Dialect) NewTokenizer() *tokens.Tokenizer {
	if d.TokenizerFactory != nil {
		return d.TokenizerFactory()
	}
	cfg := d.TokenizerConfig
	cfg.MySQLVersion = d.MySQLVersion
	return tokens.NewTokenizerWithConfig(cfg)
}

// asciiLower / asciiUpper case-fold ONLY the ASCII letters A-Z <-> a-z
// (bytes 0x41-0x5A / 0x61-0x7A), leaving every byte >= 0x80 unchanged. This
// models how real engines case-fold *unquoted* identifiers: PostgreSQL's
// downcase_identifier (src/backend/parser/scansup.c) folds ASCII-only on any
// multibyte (e.g. UTF-8) server_encoding, so `CAFÉ` -> `cafÉ` (C,A,F folded,
// É left alone), NOT `café`. UTF-8 lead/continuation bytes are always >= 0x80,
// so the plain byte scan never touches a multibyte sequence.
//
// DIVERGENCE FROM UPSTREAM (intentional): sqlglot's Dialect.normalize_identifier
// folds with Python str.lower()/str.upper() (full-Unicode) — dialects/dialect.py
// v30.12.0:1042-1050 — which over-folds non-ASCII case (É->é) and so diverges
// from the database it models. We fold ASCII-only: exact for UTF-8 and a safe
// under-fold otherwise (sqlglot has no DB connection, can't know the encoding/
// locale). Matches PostgreSQL downcase_identifier (scansup.c), which folds
// ASCII-only on multibyte encodings. Full detail + citations: DEVIATIONS.md §1.1.
func asciiLower(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 'A' && c <= 'Z' {
			if b == nil {
				b = []byte(s)
			}
			b[i] = c + ('a' - 'A')
		}
	}
	if b == nil {
		return s
	}
	return string(b)
}

func asciiUpper(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 'a' && c <= 'z' {
			if b == nil {
				b = []byte(s)
			}
			b[i] = c - ('a' - 'A')
		}
	}
	if b == nil {
		return s
	}
	return string(b)
}

// isRelationLevelIdentifier reports whether e names or references a table/relation (as opposed to a
// column). Under MySQL lower_case_table_names=0 (the MySQLCaseSensitiveTableNames strategy) these are
// case-SENSITIVE and must NOT fold, while column-level identifiers stay case-INSENSITIVE and fold. The
// relation-level positions are:
//   - a this/schema/catalog child of an exp.Table    — a physical table/schema/catalog name
//   - a table/schema/catalog child of an exp.Column  — a column QUALIFIER (it references a relation/alias)
//   - the this child of an exp.TableAlias        — a table alias OR a CTE name
//
// Everything else folds: exp.Column.this (the leaf column name), exp.TableAlias.columns (a CTE
// output-column list), exp.Alias.alias (a SELECT column alias), JOIN USING columns — all
// case-insensitive on every MySQL platform. This matches MySQL lctn=0 exactly: `SELECT users.rrn FROM
// Users` errors because the qualifier is case-sensitive, and `WITH Users AS (…) … FROM users` misses
// the CTE because CTE/alias names are case-sensitive, while column names fold. A nil parent
// (Copy()/parse-identifier paths) returns false so a detached identifier folds (the standalone-name
// default; the NormalizeIdentifiers tree walk always has parents).
func isRelationLevelIdentifier(e exp.Expression) bool {
	p := e.Parent()
	if p == nil {
		return false
	}
	switch p.Kind() {
	case exp.KindTable:
		switch e.ArgKey() {
		case "this", "schema", "catalog":
			return true
		}
	case exp.KindColumn:
		switch e.ArgKey() {
		case "table", "schema", "catalog":
			return true
		}
	case exp.KindTableAlias:
		return e.ArgKey() == "this"
	}
	return false
}

// isInformationSchemaRelationPart reports whether e is a relation-level identifier that names or
// qualifies the INFORMATION_SCHEMA virtual schema: its schema name, the table name under it, or a
// column's table-qualifier under it. MySQL matches INFORMATION_SCHEMA case-insensitively regardless of
// lower_case_table_names — uniquely among schemas; performance_schema/mysql/sys stay case-sensitive
// (live-verified on MySQL 8.0.46, lctn=0: `INFORMATION_SCHEMA.tables`, `information_schema.TABLES`, and
// mixed case all resolve, while `PERFORMANCE_SCHEMA`/`MySQL`/`SYS` error). So under the lctn=0 strategy
// these parts must fold even though sibling relation-level identifiers are preserved. It reads the
// sibling schema (never e's own kind), so a CTE/derived reference is never mistaken for information_schema.
// See DEVIATIONS.md §1.2.
func isInformationSchemaRelationPart(e exp.Expression) bool {
	p := e.Parent()
	if p == nil {
		return false
	}
	var schemaID exp.Expression
	switch p.Kind() {
	case exp.KindTable, exp.KindColumn:
		schemaID, _ = p.Arg("schema").(exp.Expression)
	default:
		return false
	}
	if schemaID == nil || schemaID.Kind() != exp.KindIdentifier {
		return false
	}
	// Match with MySQL's own fold (MySQLLower / utf8mb3_general_ci), not Go's Unicode fold: MySQL
	// recognizes the virtual schema case-insensitively via that collation (e.g. İ U+0130 → i) and NOT
	// via characters Go folds but MySQL keeps distinct (e.g. ſ U+017F). "information_schema" is already
	// MySQLLower-folded, so compare the folded name to it directly.
	if name, _ := schemaID.Arg("this").(string); MySQLLower(name) != "information_schema" {
		return false
	}
	switch e.ArgKey() {
	case "schema":
		return true // the schema name itself
	case "this":
		return p.Kind() == exp.KindTable // the table under information_schema
	case "table":
		return p.Kind() == exp.KindColumn // a column's information_schema-qualified table
	}
	return false
}

// FoldIdentifierName applies the dialect's normalization strategy to a bare identifier NAME
// (a plain string with no AST node/parent), returning the folded form. It is the string-level
// counterpart to NormalizeIdentifier — which derives an identifier's role from its parent — for
// callers that must fold a detached name (e.g. a catalog/schema key) where no AST context
// exists. isTable selects the role for the role-aware MySQLCaseSensitiveTableNames strategy
// (lctn=0): true = a relation-level name (table/schema/catalog/alias/CTE) that stays case-sensitive;
// false = a column-level name that folds. It is ignored by the other strategies.
//
// The input is treated as an UNQUOTED name (folding is unconditional): callers pass a name they
// have already decided is unquoted, matching how schema normalization builds identifiers with
// quoted=false. A quoted name must not be folded and should not be passed here.
func (d *Dialect) FoldIdentifierName(name string, isTable bool) string {
	switch d.NormalizationStrategy {
	case CaseSensitive:
		return name
	case MySQLCaseSensitiveTableNames:
		if isTable {
			return name
		}
		return MySQLLower(name)
	case MySQLCaseInsensitive:
		return MySQLLower(name)
	case Uppercase, CaseInsensitiveUppercase:
		return asciiUpper(name)
	default: // Lowercase, CaseInsensitive
		return d.lower(name)
	}
}

// lower is the dialect's lowercase fold: ASCII-only (DEVIATIONS 1.1) except Athena, whose Trino
// engine folds every identifier with Java toLowerCase (DEVIATIONS 1.19).
func (d *Dialect) lower(name string) string {
	if d.Name == "athena" {
		return strings.ToLower(name)
	}
	return asciiLower(name)
}

func (d *Dialect) NormalizeIdentifier(e exp.Expression) exp.Expression {
	s := d.NormalizationStrategy
	if e == nil || e.Kind() != exp.KindIdentifier || s == CaseSensitive {
		return e
	}

	// MySQL strategies (non-upstream): fold with MySQL's exact .tolower map, regardless of
	// quoting (MySQL identifier case-insensitivity ignores backticks). See DEVIATIONS.md §1.2.
	switch s {
	case MySQLCaseSensitiveTableNames:
		// Role-aware (models lctn=0): relation-level identifiers — table/schema/catalog names, column
		// QUALIFIERS, and table-alias/CTE names — stay case-sensitive; column-level identifiers (leaf
		// column names, CTE output-column lists, column aliases) fold with MySQLLower. EXCEPTION:
		// INFORMATION_SCHEMA is a virtual schema MySQL matches case-insensitively regardless of lctn
		// (DEVIATIONS.md §1.2), so its schema name and the table names under it fold even though they
		// are relation-level.
		if !isRelationLevelIdentifier(e) || isInformationSchemaRelationPart(e) {
			this, _ := e.Arg("this").(string)
			e.Set("this", MySQLLower(this))
		}
		return e
	case MySQLCaseInsensitive:
		this, _ := e.Arg("this").(string)
		e.Set("this", MySQLLower(this))
		return e
	}

	// dialects/bigquery.py:132-159: a UDF name or a qualified table keeps its case.
	if d.QualifiedTablesCaseSensitive && s == CaseInsensitive {
		parent := e.Parent()
		for parent != nil && parent.Kind() == exp.KindDot {
			parent = parent.Parent()
		}
		isTable := false
		if parent != nil && parent.Kind() == exp.KindUserDefinedFunction {
			isTable = true
		} else if parent != nil && parent.Kind() == exp.KindTable && parent.Arg("schema") != nil {
			quotedTable, _ := parent.MetaGet("quoted_table").(bool)
			maybeColumn, _ := parent.MetaGet("maybe_column").(bool)
			isTable = quotedTable || !maybeColumn
		}
		if isTable {
			return e
		}
	}
	// Upstream strategies: ASCII-only fold; quoted identifiers are protected unless the
	// strategy is case-insensitive (see DEVIATIONS.md §1.1).
	quoted, _ := e.Arg("quoted").(bool)
	if !quoted || s == CaseInsensitive || s == CaseInsensitiveUppercase {
		this, _ := e.Arg("this").(string)
		if s == Uppercase || s == CaseInsensitiveUppercase {
			e.Set("this", asciiUpper(this))
		} else {
			e.Set("this", d.lower(this))
		}
	}
	return e
}

// IsReservedKeyword reports whether word is a reserved keyword in this dialect — a word that
// cannot be an unquoted identifier. It is the case-safe accessor for ReservedKeywords (which is
// stored lowercase-keyed): a consumer folding keyword case must fold ONLY reserved words, since a
// non-reserved keyword can be a case-sensitive identifier (e.g. on MySQL lower_case_table_names=0).
// Only MySQL populates a reserved set today; other dialects return false.
func (d *Dialect) IsReservedKeyword(word string) bool {
	if d.ReservedKeywords == nil {
		return false
	}
	return d.ReservedKeywords[strings.ToLower(word)]
}

func (d *Dialect) CaseSensitive(text string) bool {
	// Strategies that fold every identifier unconditionally never need quoting to
	// preserve case: nothing is preserved.
	if d.NormalizationStrategy == CaseInsensitive || d.NormalizationStrategy == MySQLCaseInsensitive {
		return false
	}
	// ASCII-only, to stay consistent with the ASCII fold (asciiLower/asciiUpper): an
	// identifier is "case sensitive" (would be changed by folding, so must be quoted to
	// preserve it) only if it contains an ASCII letter of the case the fold would flip. A
	// non-ASCII letter like `É` is not considered here — for the ASCII-folding strategies it
	// is never folded, and for the MySQL strategies (MySQLCaseSensitiveTableNames) this stays
	// an ASCII approximation used only for output quoting (CanQuote), not the normalized
	// identity; being conservative here at worst under-quotes a non-ASCII name on output.
	// (Upstream uses full-Unicode str.isupper/str.islower — dialects/dialect.py
	// v30.12.0:1055-1064; we diverge to match the ASCII fold. See DEVIATIONS.md §1.1.)
	unsafe := func(r rune) bool { return r >= 'A' && r <= 'Z' } // lower-folding strategies
	if d.NormalizationStrategy == Uppercase {
		unsafe = func(r rune) bool { return r >= 'a' && r <= 'z' }
	}
	for _, r := range text {
		if unsafe(r) {
			return true
		}
	}
	return false
}

func (d *Dialect) CanQuote(identifier exp.Expression, identify any) bool {
	if identifier == nil || identifier.Kind() != exp.KindIdentifier {
		return false
	}
	quoted, _ := identifier.Arg("quoted").(bool)
	if quoted {
		return true
	}
	if identify == nil {
		return false
	}
	if b, ok := identify.(bool); ok && !b {
		return false
	}
	if parent := identifier.Parent(); parent != nil && parent.Is(exp.TraitFunc) {
		return false
	}
	if b, ok := identify.(bool); ok && b {
		return true
	}
	name := identifier.Name()
	isSafe := !d.CaseSensitive(name) && exp.IsSafeIdentifier(name)
	switch identify {
	case "safe":
		return isSafe
	case "unsafe":
		return !isSafe
	}
	panic(fmt.Sprintf("Unexpected argument for identify: '%v'", identify))
}

func (d *Dialect) QuoteIdentifier(identifier exp.Expression, identify bool) exp.Expression {
	if identifier != nil && identifier.Kind() == exp.KindIdentifier {
		mode := any(identify)
		if !identify {
			mode = "unsafe"
		}
		identifier.Set("quoted", d.CanQuote(identifier, mode))
	}
	return identifier
}

func (d *Dialect) GenerateValuesAliases(values exp.Expression) []exp.Expression {
	expressions := values.Expressions()
	if len(expressions) == 0 || expressions[0] == nil {
		return nil
	}
	columns := expressions[0].Expressions()
	aliases := make([]exp.Expression, 0, len(columns))
	for i := range columns {
		aliases = append(aliases, exp.ToIdentifier(fmt.Sprintf("_col_%d", i)))
	}
	return aliases
}

// castTargetsEquivalent ports the types_are_equivalent half of exp.cast
// (expressions/builders.py:494-499): whether the dialect's TYPE_MAPPING renders both target
// DTypes as the same type name, in which case a parse-time exp.cast call keeps the existing
// cast instead of nesting a second one. The mapping itself is declared on the Generator class
// upstream; see the castTypeMapping field comment for why the parse side carries its own copy.
func (d *Dialect) castTargetsEquivalent(from, to exp.DType) bool {
	return d.castTypeName(from) == d.castTypeName(to)
}

// castTypeName resolves a DType through the dialect's parse-side TYPE_MAPPING folds, falling
// back to the DType's own name - the .value fallback of expressions/builders.py:497-499.
func (d *Dialect) castTypeName(dt exp.DType) string {
	if name, ok := d.castTypeMapping[dt]; ok {
		return name
	}
	return string(dt)
}

// (helper kept in the dialects package for the Oracle builders)
