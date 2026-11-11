package parser

import (
	"strings"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// Upstream keeps parser callbacks in per-dialect class tables: the base statement,
// no-parentheses-function, and function tables are in .reference/sqlglot-v30.12.0/sqlglot/parser.py:
// 1081-1111, 1470-1477, and 1488-1521; Presto removes TRIM at .reference/sqlglot-v30.12.0/sqlglot/
// parsers/presto.py:137; and Athena adds a statement parser at .reference/sqlglot-v30.12.0/sqlglot/
// parsers/athena.py:17-21. This registry applies the same dependency-inversion principle as
// expressions.ParseIntoFunc (expressions/builders.go:14-15, wired at sqlglot.go:108-114), but
// cannot copy that mechanism literally. ParseIntoFunc uses leaf-package types, while these
// callbacks must resume an in-flight *Parser. Their typed callback table therefore remains in the
// parser package and is selected through a plain-string parser override name (normally the dialect
// Name), avoiding a dialects -> parser import cycle.
type parserOverrideFunc = func(*Parser) exp.Expression
type typeParserOverrideFunc = func(*Parser, bool, bool, bool, bool) exp.Expression

type dialectParserOverrideSet struct {
	FunctionParsers         map[string]parserOverrideFunc
	DisabledFunctionParsers map[string]bool
	StatementParsers        map[tokens.TokenType]parserOverrideFunc
	NoParenFunctionParsers  map[string]parserOverrideFunc
	// DisabledNoParenFunctionParsers removes base NO_PAREN_FUNCTION_PARSERS entries
	// (parsers/clickhouse.py:397-399).
	DisabledNoParenFunctionParsers map[string]bool
	NoParenFunctions               map[tokens.TokenType]func(exp.Args) exp.Expression
	// DisabledNoParenFunctions removes base NO_PAREN_FUNCTIONS entries (parsers/hive.py:126-128).
	DisabledNoParenFunctions map[tokens.TokenType]bool
	// AliasTokens adds to TABLE_ALIAS_TOKENS (parsers/presto.py:69-72).
	AliasTokens map[tokens.TokenType]bool
	// RemovedTableAliasTokens subtracts from TABLE_ALIAS_TOKENS
	// (parsers/clickhouse.py:427-435).
	RemovedTableAliasTokens map[tokens.TokenType]bool
	// RemovedAliasTokens subtracts from ALIAS_TOKENS (parsers/clickhouse.py:437-440).
	RemovedAliasTokens map[tokens.TokenType]bool
	PropertyParsers    map[string]propertyParserFunc
	// DisabledPropertyParsers removes base PROPERTY_PARSERS entries
	// (parsers/clickhouse.py:390-395).
	DisabledPropertyParsers map[string]bool
	TypeParser              typeParserOverrideFunc
	// QueryModifiers overrides QUERY_MODIFIER_PARSERS entries (parsers/tsql.py:333-337:
	// OPTION and FOR).
	QueryModifiers map[tokens.TokenType]func(*Parser) (string, any)
	// Lambdas overrides base LAMBDAS entries (parsers/materialize.py:20-24: FARROW builds a
	// raw-head Kwarg with _parse_assignment instead of the base var(_parse_disjunction)).
	Lambdas map[tokens.TokenType]func(*Parser, []exp.Expression) exp.Expression
	// TypedLambdaArgs ports Parser.TYPED_LAMBDA_ARGS (parser.py:1076, base False;
	// parsers/materialize.py:11 True): a single-argument lambda head is probed even when
	// the token after the argument is not a lambda arrow, so `x INT -> ...` annotations parse.
	TypedLambdaArgs bool
	// LambdaArgParser ports a dialect's _parse_lambda_arg override
	// (parsers/materialize.py:16-17: _parse_field).
	LambdaArgParser func(*Parser) exp.Expression
	// DisableTableHints ports RisingWaveParser._parse_table_hints (parsers/risingwave.py:
	// 34-37): the dialect has no T-SQL/MySQL hint grammar, so `FROM a WITH (...)` must not
	// swallow the connector properties that follow the table.
	DisableTableHints bool
}

var dialectParserOverrides = map[string]dialectParserOverrideSet{}

// registerDialectParserOverrides configures an override set during package initialization. A nil
// callback is not a removal marker: function removals must use DisabledFunctionParsers explicitly.
func registerDialectParserOverrides(name string, overrides dialectParserOverrideSet) {
	name = strings.ToLower(name)
	if _, exists := dialectParserOverrides[name]; exists {
		panic("parser: duplicate parser overrides for dialect " + name)
	}

	for _, callback := range overrides.FunctionParsers {
		if callback == nil {
			panic("parser: nil function parser override for dialect " + name)
		}
	}
	for _, callback := range overrides.StatementParsers {
		if callback == nil {
			panic("parser: nil statement parser override for dialect " + name)
		}
	}
	for _, callback := range overrides.NoParenFunctionParsers {
		if callback == nil {
			panic("parser: nil no-parentheses function parser override for dialect " + name)
		}
	}
	for _, callback := range overrides.NoParenFunctions {
		if callback == nil {
			panic("parser: nil no-parentheses function override for dialect " + name)
		}
	}
	for _, callback := range overrides.PropertyParsers {
		if callback == nil {
			panic("parser: nil property parser override for dialect " + name)
		}
	}
	for _, callback := range overrides.Lambdas {
		if callback == nil {
			panic("parser: nil lambda override for dialect " + name)
		}
	}

	dialectParserOverrides[name] = overrides
}

// parserOverrideKey resolves the parser-class overlay independently from the concrete dialect.
// Athena uses a Trino dialect instance for flags and tokenizer behavior while selecting the
// AthenaTrinoParser callback tables.
func (p *Parser) parserOverrideKey() string {
	if p.parserOverrideName != "" {
		return strings.ToLower(p.parserOverrideName)
	}
	return strings.ToLower(p.dialect.Name)
}

// parserOverrideParents models parser-class inheritance: StarRocksParser extends
// MySQLParser (parsers/starrocks.py:11), so a StarRocks lookup falls back to the mysql overlay.
var parserOverrideParents = map[string]string{
	"starrocks":   "mysql",
	"doris":       "mysql",
	"redshift":    "postgres",
	"materialize": "postgres",
	"risingwave":  "postgres",
	// Spark chain: Spark2Parser extends HiveParser (parsers/spark2.py:21), SparkParser
	// extends Spark2Parser (parsers/spark.py:15) and DatabricksParser extends SparkParser
	// (parsers/databricks.py).
	"spark2":     "hive",
	"spark":      "spark2",
	"databricks": "spark",
}

// overrideChain lists the override keys from the parser's own key up through its parents.
func (p *Parser) overrideChain() []string {
	if p.overrideChainCache != nil {
		return p.overrideChainCache
	}
	// The chain is fixed for a Parser instance: parserOverrideName and dialect are set at
	// construction. Cache it so each lookup stops rebuilding (and re-lowercasing) it.
	keys := make([]string, 0, 2)
	for key := p.parserOverrideKey(); key != ""; key = parserOverrideParents[key] {
		keys = append(keys, key)
	}
	p.overrideChainCache = keys
	return keys
}

// dialectFamily reports whether the concrete dialect (or one of its parents) is name. Unlike
// dialectIs it ignores the parser override name, so a gate that must follow the real dialect
// (Athena's query parser is real Trino) is unaffected by a foreign overlay.
func (p *Parser) dialectFamily(name string) bool {
	for key := strings.ToLower(p.dialect.Name); key != ""; key = parserOverrideParents[key] {
		if key == name {
			return true
		}
	}
	return false
}

// dialectIs reports whether the dialect (or one of its parser parents) is name.
func (p *Parser) dialectIs(name string) bool {
	for _, key := range p.overrideChain() {
		if key == name {
			return true
		}
	}
	return false
}

func (p *Parser) functionParser(name string) parserOverrideFunc {
	for _, key := range p.overrideChain() {
		overrides := dialectParserOverrides[key]
		if parser := overrides.FunctionParsers[name]; parser != nil {
			return parser
		}
		if overrides.DisabledFunctionParsers[name] {
			return nil
		}
	}

	parser := functionParsers[name]
	if parser == nil {
		return nil
	}

	// Keep the current base-singleton compatibility gates on the fallback only. An overlay above
	// may intentionally add or override any of these names for another dialect. These gates use
	// the concrete dialect rather than the parser overlay key: Athena's query parser is real Trino.
	switch name {
	case "VALUES":
		if !p.dialect.ValuesIsFunction {
			return nil
		}
	case "SUBSTR", "JSON_VALUE":
		if !p.dialectFamily("mysql") {
			return nil
		}
	case "DATE_PART":
		if !p.dialectFamily("postgres") {
			return nil
		}
	}
	return parser
}

func (p *Parser) statementParser(tokenType tokens.TokenType) parserOverrideFunc {
	for _, key := range p.overrideChain() {
		if parser := dialectParserOverrides[key].StatementParsers[tokenType]; parser != nil {
			return parser
		}
	}
	return statementParsers[tokenType]
}

// propertyParser mirrors functionParser and statementParser: the selected dialect overlay wins,
// while the shared PROPERTY_PARSERS singleton remains the fallback for every dialect.
func (p *Parser) propertyParser(name string) propertyParserFunc {
	name = strings.ToUpper(name)
	for _, key := range p.overrideChain() {
		overrides := dialectParserOverrides[key]
		if parser := overrides.PropertyParsers[name]; parser != nil {
			return parser
		}
		if overrides.DisabledPropertyParsers[name] {
			return nil
		}
	}
	return propertyParsers[name]
}

// propertyParserKeys returns a fresh union because matchTexts needs every overlay key even when the
// same dialect continues to inherit the rest of the shared registry.
func (p *Parser) propertyParserKeys() map[string]bool {
	keys := make(map[string]bool, len(propertyParsers))
	for name := range propertyParsers {
		keys[name] = true
	}
	for _, key := range p.overrideChain() {
		overrides := dialectParserOverrides[key]
		for name := range overrides.PropertyParsers {
			keys[name] = true
		}
		for name := range overrides.DisabledPropertyParsers {
			delete(keys, name)
		}
	}
	return keys
}

func (p *Parser) typeParserOverride() typeParserOverrideFunc {
	for _, key := range p.overrideChain() {
		if parser := dialectParserOverrides[key].TypeParser; parser != nil {
			return parser
		}
	}
	return nil
}

// lambdaFor resolves a LAMBDAS entry: the dialect overlay wins over the shared table
// (parsers/materialize.py:20-24 overrides the base FARROW entry).
func (p *Parser) lambdaFor(tt tokens.TokenType) func(*Parser, []exp.Expression) exp.Expression {
	for _, key := range p.overrideChain() {
		if builder, ok := dialectParserOverrides[key].Lambdas[tt]; ok {
			return builder
		}
	}
	return lambdas[tt]
}

// typedLambdaArgs reports whether any parser class in the chain enables TYPED_LAMBDA_ARGS
// (base False; parsers/materialize.py:11 True).
func (p *Parser) typedLambdaArgs() bool {
	for _, key := range p.overrideChain() {
		if dialectParserOverrides[key].TypedLambdaArgs {
			return true
		}
	}
	return false
}

// disableTableHints reports whether any parser class in the chain disables table hints
// (parsers/risingwave.py:34-37: RisingWaveParser._parse_table_hints always returns None).
func (p *Parser) disableTableHints() bool {
	for _, key := range p.overrideChain() {
		if dialectParserOverrides[key].DisableTableHints {
			return true
		}
	}
	return false
}

// lambdaArgParser resolves a dialect's _parse_lambda_arg override, falling back to the
// base id-var parse (parser.py:7489-7490).
func (p *Parser) lambdaArgParser() func(*Parser) exp.Expression {
	for _, key := range p.overrideChain() {
		if parser := dialectParserOverrides[key].LambdaArgParser; parser != nil {
			return parser
		}
	}
	return nil
}

// isQueryModifierToken reports whether tt starts a query modifier in this dialect, including
// an overlay-only modifier such as TSQL's OPTION.
func (p *Parser) isQueryModifierToken(tt tokens.TokenType) bool {
	for _, key := range p.overrideChain() {
		if _, ok := dialectParserOverrides[key].QueryModifiers[tt]; ok {
			return true
		}
	}
	return queryModifierTokens[tt]
}

// queryModifierFor resolves a query modifier parser: the dialect overlay wins over the shared
// QUERY_MODIFIER_PARSERS table.
func (p *Parser) queryModifierFor(tt tokens.TokenType) func(*Parser) (string, any) {
	for _, key := range p.overrideChain() {
		if parse, ok := dialectParserOverrides[key].QueryModifiers[tt]; ok {
			return parse
		}
	}
	return queryModifierParsers[tt]
}

func (p *Parser) noParenFunctionParserFor(name string) parserOverrideFunc {
	for _, key := range p.overrideChain() {
		overrides := dialectParserOverrides[key]
		if parser := overrides.NoParenFunctionParsers[name]; parser != nil {
			return parser
		}
		if overrides.DisabledNoParenFunctionParsers[name] {
			return nil
		}
	}
	parser := noParenFunctionParsers[name]
	if parser == nil || (name == "VARIADIC" && !p.dialectFamily("postgres")) ||
		(name == "PRIOR" && p.connectPriorDepth == 0) {
		return nil
	}
	return parser
}

func (p *Parser) noParenFunctionFor(tokenType tokens.TokenType) func(exp.Args) exp.Expression {
	for _, key := range p.overrideChain() {
		overrides := dialectParserOverrides[key]
		if build := overrides.NoParenFunctions[tokenType]; build != nil {
			return build
		}
		if overrides.DisabledNoParenFunctions[tokenType] {
			return nil
		}
	}
	return noParenFunctions[tokenType]
}

// tableAliasTokensFor is TABLE_ALIAS_TOKENS plus the dialect overlay's additions.
func (p *Parser) tableAliasTokensFor() map[tokens.TokenType]bool {
	added := map[tokens.TokenType]bool{}
	removed := map[tokens.TokenType]bool{}
	changed := false
	for _, key := range p.overrideChain() {
		overrides := dialectParserOverrides[key]
		for tt := range overrides.AliasTokens {
			added[tt] = true
			changed = true
		}
		for tt := range overrides.RemovedTableAliasTokens {
			removed[tt] = true
			changed = true
		}
	}
	if !changed {
		return tableAliasTokens
	}
	return mergeTokenSet(tableAliasTokens, added, removed)
}

// aliasTokensFor is ALIAS_TOKENS with the dialect overlay's removals applied
// (parsers/clickhouse.py:437-440).
func (p *Parser) aliasTokensFor() map[tokens.TokenType]bool {
	var removed map[tokens.TokenType]bool
	for _, key := range p.overrideChain() {
		overrides := dialectParserOverrides[key]
		if len(overrides.RemovedAliasTokens) > 0 {
			if removed == nil {
				removed = map[tokens.TokenType]bool{}
			}
			for tt := range overrides.RemovedAliasTokens {
				removed[tt] = true
			}
		}
	}
	if len(removed) == 0 {
		return aliasTokens
	}
	return mergeTokenSet(aliasTokens, nil, removed)
}

func mergeTokenSet(base, added, removed map[tokens.TokenType]bool) map[tokens.TokenType]bool {
	merged := make(map[tokens.TokenType]bool, len(base)+len(added))
	for tt := range base {
		if !removed[tt] {
			merged[tt] = true
		}
	}
	for tt := range added {
		merged[tt] = true
	}
	return merged
}

// switchDialect is the dialect name a per-dialect switch keys on. The Postgres subclasses
// (Redshift, Materialize, RisingWave) inherit PostgresParser, so they fold into "postgres".
func (p *Parser) switchDialect() string {
	if p.dialectFamily("postgres") {
		return "postgres"
	}
	return p.dialect.Name
}
