package generator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/maltzsama/sqlglot-go/dialects"
	"github.com/maltzsama/sqlglot-go/expressions"
)

var safeIdentifierRE = regexp.MustCompile(`^[_a-zA-Z]\w*$`)

var typeMapping = map[expressions.DType]string{
	expressions.DTypeDatetime2:     "TIMESTAMP",
	expressions.DTypeNChar:         "CHAR",
	expressions.DTypeNVarchar:      "VARCHAR",
	expressions.DTypeMediumText:    "TEXT",
	expressions.DTypeLongText:      "TEXT",
	expressions.DTypeTinyText:      "TEXT",
	expressions.DTypeBlob:          "VARBINARY",
	expressions.DTypeMediumBlob:    "BLOB",
	expressions.DTypeLongBlob:      "BLOB",
	expressions.DTypeTinyBlob:      "BLOB",
	expressions.DTypeInet:          "INET",
	expressions.DTypeRowVersion:    "VARBINARY",
	expressions.DTypeSmallDatetime: "TIMESTAMP",
}

// mysqlTypeMapping is a FULL REPLACEMENT of typeMapping for the mysql dialect (generators/
// mysql.py:255-273): unlike base, it does NOT remap LONGTEXT/MEDIUMTEXT/TINYTEXT/*BLOB to
// TEXT/BLOB/VARBINARY - those stay as their own MySQL-native type names. It also folds in
// mysql's TIMESTAMP_TYPE_MAPPING (mysql.py:246-253), used by plain (non-CAST) DataType
// rendering, e.g. a column typed TIMESTAMP renders as DATETIME.
var mysqlTypeMapping = map[expressions.DType]string{
	expressions.DTypeNChar:         "CHAR",
	expressions.DTypeNVarchar:      "VARCHAR",
	expressions.DTypeInet:          "INET",
	expressions.DTypeRowVersion:    "VARBINARY",
	expressions.DTypeUBigInt:       "BIGINT",
	expressions.DTypeUInt:          "INT",
	expressions.DTypeUMediumInt:    "MEDIUMINT",
	expressions.DTypeUSmallInt:     "SMALLINT",
	expressions.DTypeUTinyInt:      "TINYINT",
	expressions.DTypeUDecimal:      "DECIMAL",
	expressions.DTypeUDouble:       "DOUBLE",
	expressions.DTypeDatetime2:     "DATETIME",
	expressions.DTypeSmallDatetime: "DATETIME",
	expressions.DTypeTimestamp:     "DATETIME",
	expressions.DTypeTimestampNtz:  "DATETIME",
	expressions.DTypeTimestampTz:   "TIMESTAMP",
	expressions.DTypeTimestampLtz:  "TIMESTAMP",
}

// postgresTypeMapping is typeMapping (base) plus the postgres TYPE_MAPPING delta
// (generators/postgres.py:271-284).
var postgresTypeMapping = mergeTypeMappings(typeMapping, map[expressions.DType]string{
	expressions.DTypeTinyInt:      "SMALLINT",
	expressions.DTypeFloat:        "REAL",
	expressions.DTypeDouble:       "DOUBLE PRECISION",
	expressions.DTypeBinary:       "BYTEA",
	expressions.DTypeVarBinary:    "BYTEA",
	expressions.DTypeRowVersion:   "BYTEA",
	expressions.DTypeDatetime:     "TIMESTAMP",
	expressions.DTypeTimestampNtz: "TIMESTAMP",
	expressions.DTypeBlob:         "BYTEA",
})

func mergeTypeMappings(base map[expressions.DType]string, delta map[expressions.DType]string) map[expressions.DType]string {
	out := make(map[expressions.DType]string, len(base)+len(delta))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range delta {
		out[k] = v
	}
	return out
}

// mysqlCastMapping ports MySQL's narrow CAST_MAPPING (generators/mysql.py:315-331): MySQL's
// CAST only supports a handful of target types, so text/blob-ish types collapse to CHAR and
// signed-integer-ish types (plus BOOLEAN) collapse to SIGNED; UBIGINT is the one type that
// maps to UNSIGNED instead. Values are plain override strings (not further looked up in
// mysqlTypeMapping), matching upstream's rename-then-render-raw semantics.
var mysqlCastMapping = map[expressions.DType]string{
	expressions.DTypeLongText:   "CHAR",
	expressions.DTypeLongBlob:   "CHAR",
	expressions.DTypeMediumBlob: "CHAR",
	expressions.DTypeMediumText: "CHAR",
	expressions.DTypeText:       "CHAR",
	expressions.DTypeTinyBlob:   "CHAR",
	expressions.DTypeTinyText:   "CHAR",
	expressions.DTypeVarchar:    "CHAR",
	expressions.DTypeBigInt:     "SIGNED",
	expressions.DTypeBoolean:    "SIGNED",
	expressions.DTypeInt:        "SIGNED",
	expressions.DTypeSmallInt:   "SIGNED",
	expressions.DTypeTinyInt:    "SIGNED",
	expressions.DTypeMediumInt:  "SIGNED",
	expressions.DTypeUBigInt:    "UNSIGNED",
}

// mysqlTimestampFuncTypes ports MySQL's TIMESTAMP_FUNC_TYPES (generators/mysql.py:333-336):
// casting to either of these renders as a TIMESTAMP(...) function call rather than CAST(...
// AS ...), since MySQL's CAST doesn't accept a timezone-aware timestamp target type.
var mysqlTimestampFuncTypes = map[expressions.DType]bool{
	expressions.DTypeTimestampTz:  true,
	expressions.DTypeTimestampLtz: true,
}

// timePartSingulars ports TIME_PART_SINGULARS (generator.py:644-654): used by intervalSQL
// when the dialect doesn't allow plural interval unit forms (e.g. mysql).
var timePartSingulars = map[string]string{
	"MICROSECONDS": "MICROSECOND",
	"SECONDS":      "SECOND",
	"MINUTES":      "MINUTE",
	"HOURS":        "HOUR",
	"DAYS":         "DAY",
	"WEEKS":        "WEEK",
	"MONTHS":       "MONTH",
	"QUARTERS":     "QUARTER",
	"YEARS":        "YEAR",
}

func (g *Generator) expressionSQL(e expressions.Expression) string { return g.sqlKey(e, "this") }

func (g *Generator) nullSQL(e expressions.Expression) string { return "NULL" }

// boolTypes ports TSQLGenerator.BIT_TYPES (generators/tsql.py:31): parents whose boolean
// operands render as 1/0 rather than a comparison.
var boolTypes = map[expressions.Kind]bool{
	expressions.KindEQ:     true,
	expressions.KindNEQ:    true,
	expressions.KindIs:     true,
	expressions.KindIn:     true,
	expressions.KindSelect: true,
	expressions.KindAlias:  true,
}

func (g *Generator) booleanSQL(e expressions.Expression) string {
	truth := boolValue(e.Arg("this"))
	if g.dialect.EnsureBools {
		// generators/tsql.py:378-384: a boolean literal in a predicate position (or inside
		// VALUES) is 1/0; elsewhere TSQL has no boolean type, so it emits a comparison.
		parent := e.Parent()
		inValues := e.FindAncestor(expressions.KindValues, expressions.KindSelect) != nil &&
			e.FindAncestor(expressions.KindValues, expressions.KindSelect).Kind() == expressions.KindValues
		if (parent != nil && boolTypes[parent.Kind()]) || inValues {
			if truth {
				return "1"
			}
			return "0"
		}
		if truth {
			return "(1 = 1)"
		}
		return "(1 = 0)"
	}
	if truth {
		return "TRUE"
	}
	return "FALSE"
}

func (g *Generator) varSQL(e expressions.Expression) string { return g.sqlKey(e, "this") }

func (g *Generator) starSQL(e expressions.Expression) string {
	except := g.expressions(exprsOptions{expression: e, key: "except_", flat: true})
	if except != "" {
		except = g.seg("EXCEPT") + " (" + except + ")"
	}
	replace := g.expressions(exprsOptions{expression: e, key: "replace", flat: true})
	if replace != "" {
		replace = g.seg("REPLACE") + " (" + replace + ")"
	}
	rename := g.expressions(exprsOptions{expression: e, key: "rename", flat: true})
	if rename != "" {
		rename = g.seg("RENAME") + " (" + rename + ")"
	}
	ilike := g.sqlKey(e, "ilike")
	if ilike != "" {
		ilike = g.seg("ILIKE") + " " + ilike
	}
	return "*" + ilike + except + replace + rename
}

func (g *Generator) parenSQL(e expressions.Expression) string {
	sql := g.seg(g.indent(g.sqlKey(e, "this"), 0, nil, false, false), "")
	return "(" + sql + g.seg(")", "")
}

func (g *Generator) negSQL(e expressions.Expression) string {
	thisSQL := g.sqlKey(e, "this")
	sep := ""
	if strings.HasPrefix(thisSQL, "-") {
		sep = " "
	}
	return "-" + sep + thisSQL
}

func (g *Generator) notSQL(e expressions.Expression) string { return "NOT " + g.sqlKey(e, "this") }

func (g *Generator) dotSQL(e expressions.Expression) string {
	return g.sqlKey(e, "this") + "." + g.sqlKey(e, "expression")
}

func (g *Generator) columnParts(e expressions.Expression) string {
	// generator.py:1153-1156: a shadowed qualifier renders as the bare name (BigQuery's
	// GROUP BY/HAVING/QUALIFY would otherwise resolve it to the projection alias).
	if truthy(e.Arg("shadow")) && g.dialect.ProjectionAliasesShadowSourceNames {
		return g.sqlKey(e, "this")
	}
	// generator.py column_parts override (bigquery): a `dataset.table`.name reference keeps its
	// quoted table path.
	if truthy(e.MetaGet("quoted_column")) {
		parts := e.Parts()
		if len(parts) > 1 {
			names := make([]string, 0, len(parts)-1)
			for _, part := range parts[:len(parts)-1] {
				names = append(names, part.Name())
			}
			tablePath := g.gen(expressions.Identifier(expressions.Args{"this": strings.Join(names, "."), "quoted": true}))
			return tablePath + "." + g.sqlKey(e, "this")
		}
	}
	parts := []string{}
	for _, key := range []string{"catalog", "schema", "table", "this"} {
		if v := e.Arg(key); truthy(v) {
			parts = append(parts, g.gen(v))
		}
	}
	return strings.Join(parts, ".")
}

func (g *Generator) columnSQL(e expressions.Expression) string {
	joinMark := ""
	if truthy(e.Arg("join_mark")) {
		joinMark = " (+)"
	}
	if joinMark != "" && !g.dialect.SupportsColumnJoinMarks {
		joinMark = ""
		g.unsupported("Outer join syntax using the (+) operator is not supported.")
	}
	return g.columnParts(e) + joinMark
}

func (g *Generator) identifierSQL(e expressions.Expression) string {
	text := e.Name()
	lower := strings.ToLower(text)
	quoted := boolValue(e.Arg("quoted"))
	if g.normalize && !quoted {
		text = lower
	}
	text = strings.ReplaceAll(text, g.identifierEnd, g.escapedIdentifierEnd)
	if quoted || g.canQuoteIdentifier(e) || g.dialect.ReservedKeywords[lower] ||
		(!g.dialect.TokenizerConfig.IdentifiersCanStartWithDigit && startsWithDigit(text)) {
		text = g.identifierStart + g.replaceLineBreaks(text) + g.identifierEnd
	}
	return text
}

func (g *Generator) canQuoteIdentifier(e expressions.Expression) bool {
	identify := g.identify
	if identify == nil {
		return false
	}
	if b, ok := identify.(bool); ok && !b {
		return false
	}
	if parent := e.Parent(); parent != nil && parent.Is(expressions.TraitFunc) {
		return false
	}
	if b, ok := identify.(bool); ok && b {
		return true
	}
	// The case-sensitivity and SAFE_IDENTIFIER_RE checks run against the ORIGINAL
	// identifier text (identifier.this), not the normalized/escaped output text
	// (dialect.py:1089-1091).
	original := e.Name()
	caseSensitive := false
	for _, r := range original {
		if r >= 'A' && r <= 'Z' {
			caseSensitive = true
			break
		}
	}
	isSafe := !caseSensitive && safeIdentifierRE.MatchString(original)
	switch identify {
	case "safe":
		return isSafe
	case "unsafe":
		return !isSafe
	}
	return false
}

func startsWithDigit(text string) bool {
	if text == "" {
		return false
	}
	r := []rune(text)[0]
	return r >= '0' && r <= '9'
}

func (g *Generator) literalSQL(e expressions.Expression) string {
	text := e.Text("this")
	if e.IsString() {
		text = g.quoteStart + g.escapeStr(text) + g.quoteEnd
	}
	return text
}

// escapeStr is the common-case entry point, equivalent to upstream escape_str's defaults
// (escape_backslash=True, delimiter=None, escaped_delimiter=None, is_byte_string=False,
// generator.py:2983-2989). By far the hottest caller (every string literal), so it stays a
// plain single-arg function rather than requiring every call site to spell out defaults.
func (g *Generator) escapeStr(text string) string {
	return g.escapeStrOpts(text, escapeStrOptions{escapeBackslash: true})
}

// escapeStrOptions parameterizes escapeStrOpts, mirroring escape_str's non-default kwargs.
// Go's bool zero value is false, NOT upstream's escape_backslash=True default - callers
// that want that default must set EscapeBackslash explicitly (see escapeStr above); the
// only current caller of a non-default value is bytestringSQL (escape_backslash=False).
type escapeStrOptions struct {
	escapeBackslash  bool
	delimiter        string
	escapedDelimiter string
	isByteString     bool
}

// escapeStrOpts ports escape_str (generator.py:2983-3005) in full, parameterized form.
func (g *Generator) escapeStrOpts(text string, opts escapeStrOptions) string {
	supportsEscapeSequences := g.stringsSupportEscapedSequences
	if opts.isByteString {
		supportsEscapeSequences = g.byteStringsSupportEscapedSequences
	}
	if supportsEscapeSequences {
		// Mirrors the default ESCAPED_SEQUENCES dialect table (dialects/dialect.py:66-90,
		// 302-312): the inverse of UNESCAPED_SEQUENCES, filtered down to non-printable
		// control characters plus backslash itself (printable characters round-trip as-is).
		// No target dialect here overrides UNESCAPED_SEQUENCES (only clickhouse/snowflake
		// do, out of scope), so the table is hardcoded rather than threading a new
		// per-dialect field just for this.
		table := escapedSequences
		if len(g.dialect.EscapedSequences) > 0 {
			table = make(map[byte]string, len(escapedSequences)+len(g.dialect.EscapedSequences))
			for k, v := range escapedSequences {
				table[k] = v
			}
			for k, v := range g.dialect.EscapedSequences {
				table[k] = v
			}
		}
		var b strings.Builder
		for i := 0; i < len(text); i++ {
			c := text[i]
			if !opts.escapeBackslash && c == '\\' {
				b.WriteByte(c)
				continue
			}
			if esc, ok := table[c]; ok {
				b.WriteString(esc)
			} else {
				b.WriteByte(c)
			}
		}
		text = b.String()
	}
	delimiter := opts.delimiter
	if delimiter == "" {
		delimiter = g.quoteEnd
	}
	escapedDelimiter := opts.escapedDelimiter
	if escapedDelimiter == "" {
		escapedDelimiter = g.escapedQuoteEnd
	}
	return strings.ReplaceAll(g.replaceLineBreaks(text), delimiter, escapedDelimiter)
}

// escapedSequences is the default ESCAPED_SEQUENCES table (dialects/dialect.py:66-90,
// 302-312 UNESCAPED_SEQUENCES inverted, filtered to `not v.isprintable() or v == "\\"`).
// See escapeStr for why this is hardcoded rather than a per-dialect field.
var escapedSequences = map[byte]string{
	'\a': `\a`,
	'\b': `\b`,
	'\f': `\f`,
	'\n': `\n`,
	'\r': `\r`,
	'\t': `\t`,
	'\v': `\v`,
	'\\': `\\`,
}

// placeholderSQL ports placeholder_sql (generator.py:3417-3418) and postgres's override
// (generators/postgres.py:541-546). Postgres's `?` placeholder parses with jdbc=True
// (parsers/postgres.py:96) and always renders "?"; everything else there is its pyformat/
// psycopg placeholder: bare "%s", or named "%(name)s" (parseQueryParameter).
func (g *Generator) placeholderSQL(e expressions.Expression) string {
	if g.isDialect("postgres") {
		if truthy(e.Arg("jdbc")) {
			return "?"
		}
		this := ""
		if truthy(e.Arg("this")) {
			this = "(" + e.Name() + ")"
		}
		return "%" + this + "s"
	}
	if truthy(e.Arg("this")) {
		// A named placeholder (:name); base/mysql keep :name (NAMED_PLACEHOLDER_TOKEN=":").
		return ":" + e.Name()
	}
	// A bare `?` placeholder.
	return "?"
}

// parameterSQL ports parameter_sql (generator.py:3406-3408). PARAMETER_TOKEN is "@" for
// base/mysql, "$" for postgres (generators/postgres.py:240, e.g. `CAST($1 AS TEXT)`).
func (g *Generator) parameterSQL(e expressions.Expression) string {
	return g.parameterToken + g.sqlKey(e, "this")
}

// rawStringSQL ports rawstring_sql (generator.py:1653-1659): a raw/heredoc string is
// re-emitted as an ordinary quoted string literal (e.g. postgres `$$doc$$` -> `'doc'`).
// rawStringSQL ports rawstring_sql (generator.py:1684-1690).
func (g *Generator) rawStringSQL(e expressions.Expression) string {
	text := e.Text("this")
	if g.dialect.TokenizerConfig.StringEscapes['\\'] {
		text = strings.ReplaceAll(text, "\\", "\\\\")
	}
	text = g.escapeStrOpts(text, escapeStrOptions{escapeBackslash: false})
	return g.quoteStart + text + g.quoteEnd
}

// fileFormatPropertySQL renders the FileFormatProperty node as `FORMAT=<fmt>`, matching
// upstream's generic property_sql path (generator.py:2083-2092, PROPERTY_TO_NAME lookup).
func (g *Generator) fileFormatPropertySQL(e expressions.Expression) string {
	return "FORMAT=" + g.sqlKey(e, "this")
}

// encodePropertySQL ports encodeproperty_sql (generator.py:5937-5945): RisingWave's
// `ENCODE|KEY ENCODE <fmt> [(k = v, ...)]` CREATE SOURCE/SINK option.
func (g *Generator) encodePropertySQL(e expressions.Expression) string {
	encode := "ENCODE"
	if boolValue(e.Arg("key")) {
		encode = "KEY ENCODE"
	}
	encode += " " + g.sqlKey(e, "this")
	if props := asExpression(e.Arg("properties")); props != nil {
		encode += " " + g.renderProperties(props, "", ", ", "", true)
	}
	return encode
}

// includePropertySQL ports includeproperty_sql (generator.py:5947-5957): RisingWave's
// `INCLUDE <name> [<coldef> [AS <alias>]]` CREATE SOURCE option.
func (g *Generator) includePropertySQL(e expressions.Expression) string {
	include := "INCLUDE " + g.sqlKey(e, "this")
	if columnDef := g.sqlKey(e, "column_def"); columnDef != "" {
		include += " " + columnDef
	}
	if alias := g.sqlKey(e, "alias"); alias != "" {
		include += " AS " + alias
	}
	return include
}

func (g *Generator) aliasSQL(e expressions.Expression) string {
	alias := g.sqlKey(e, "alias")
	if alias != "" {
		alias = " AS " + alias
	}
	return g.sqlKey(e, "this") + alias
}

func (g *Generator) aliasesSQL(e expressions.Expression) string {
	return g.sqlKey(e, "this") + " AS (" + g.expressions(exprsOptions{expression: e, flat: true}) + ")"
}

func (g *Generator) pivotAliasSQL(e expressions.Expression) string {
	alias := asExpression(e.Arg("alias"))
	parent := e.Parent()
	var pivot expressions.Expression
	if parent != nil {
		pivot = parent.Parent()
	}
	if pivot != nil && pivot.Kind() == expressions.KindPivot && boolValue(pivot.Arg("unpivot")) && alias != nil {
		if g.dialect.UnpivotAliasesAreIdentifiers {
			if alias.Kind() == expressions.KindLiteral {
				alias.Replace(expressions.ToIdentifier(alias.OutputName()))
			}
		} else if alias.Kind() == expressions.KindIdentifier {
			alias.Replace(expressions.LiteralString(alias.OutputName()))
		}
	}
	return g.aliasSQL(e)
}

func (g *Generator) binary(e expressions.Expression, op string) string {
	sqls := []string{}
	stack := []any{e}
	binaryKind := e.Kind()
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if expr, ok := node.(expressions.Expression); ok && !isNilExpression(expr) && expr.Kind() == binaryKind {
			if opFunc := expr.Arg("operator"); truthy(opFunc) {
				op = "OPERATOR(" + g.gen(opFunc) + ")"
			}
			stack = append(stack, expr.Arg("expression"))
			stack = append(stack, " "+g.maybeComment(op, nil, expr.Comments(), false)+" ")
			stack = append(stack, expr.Arg("this"))
		} else {
			sqls = append(sqls, g.gen(node))
		}
	}
	return strings.Join(sqls, "")
}

func (g *Generator) addSQL(e expressions.Expression) string { return g.binary(e, "+") }
func (g *Generator) subSQL(e expressions.Expression) string { return g.binary(e, "-") }
func (g *Generator) mulSQL(e expressions.Expression) string { return g.binary(e, "*") }
func (g *Generator) modSQL(e expressions.Expression) string { return g.binary(e, "%") }

// dtypeSetArgs expands a DType set into the variadic form Node.IsType accepts, mirroring
// upstream's `*exp.DataType.REAL_TYPES` argument unpacking in div_sql.
func dtypeSetArgs(set map[expressions.DType]bool) []any {
	out := make([]any, 0, len(set))
	for dt := range set {
		out = append(out, dt)
	}
	return out
}

var (
	divRealTypes    = dtypeSetArgs(expressions.RealTypes)
	divIntegerTypes = dtypeSetArgs(expressions.IntegerTypes)
)

// divSQL ports div_sql (generator.py:4451-4468): under a non-SAFE_DIVISION dialect a
// parsed `safe` divisor is wrapped in NULLIF(…, 0); a TYPED_DIVISION dialect casts an
// unannotated non-real dividend to DOUBLE (both sides annotated as integer under an
// untyped dialect cast the whole division back to BIGINT). Mutations go through
// Node.Replace on the tree Generate already copied (generator.go:193), exactly like
// upstream's generate(copy=True).
func (g *Generator) divSQL(e expressions.Expression) string {
	l := e.This()
	r := e.Expr()

	if !g.dialect.SafeDivision && truthy(e.Arg("safe")) {
		r.Replace(expressions.Nullif(expressions.Args{"this": r.Copy(), "expression": expressions.LiteralNumber(0)}))
	}

	if g.dialect.TypedDivision && !truthy(e.Arg("typed")) {
		if !divOperandIsType(l, divRealTypes...) && !divOperandIsType(r, divRealTypes...) {
			l.Replace(expressions.Cast(expressions.Args{"this": l.Copy(), "to": expressions.DTypeDouble.IntoExpr(nil)}))
		}
	} else if !g.dialect.TypedDivision && truthy(e.Arg("typed")) {
		// The operands read the locals captured above, not e.Expr(): after the NULLIF
		// replacement above they are stale on purpose — upstream builds `l / r` from the
		// same pre-replacement locals (generator.py:4465), so a typed integer division
		// keeps the raw divisor inside the BIGINT cast (oracle: sqlite
		// CAST(a AS INT) / CAST(b AS INT) -> base
		// CAST(CAST(a AS INT) / CAST(b AS INT) AS BIGINT), no NULLIF).
		if divOperandIsType(l, divIntegerTypes...) && divOperandIsType(r, divIntegerTypes...) {
			return g.gen(expressions.Cast(expressions.Args{
				"this": expressions.New(expressions.KindDiv, expressions.Args{"this": l, "expression": r}),
				"to":   expressions.DTypeBigInt.IntoExpr(nil),
			}))
		}
	}

	return g.binary(e, "/")
}

// divOperandIsType ports the operand type checks in div_sql (generator.py:4458, 4462):
// a cast operand checks its target type — Cast.is_type (functions.py:57-70) — while any
// other operand checks the type annotate_types recorded (Expression.is_type,
// core.py:984-986). Node.IsType deliberately excludes a cast's target (its annotation is
// nil at parse time), so the cast case goes through DataTypeIsType, which reads the `to`
// arg exactly like upstream's override.
func divOperandIsType(e expressions.Expression, dtypes ...any) bool {
	if e == nil {
		return false
	}
	switch e.Kind() {
	case expressions.KindCast, expressions.KindTryCast, expressions.KindJSONCast:
		return expressions.DataTypeIsType(e, false, dtypes...)
	}
	return e.IsType(dtypes...)
}

func (g *Generator) eqSQL(e expressions.Expression) string         { return g.binary(e, "=") }
func (g *Generator) neqSQL(e expressions.Expression) string        { return g.binary(e, "<>") }
func (g *Generator) gtSQL(e expressions.Expression) string         { return g.binary(e, ">") }
func (g *Generator) gteSQL(e expressions.Expression) string        { return g.binary(e, ">=") }
func (g *Generator) ltSQL(e expressions.Expression) string         { return g.binary(e, "<") }
func (g *Generator) lteSQL(e expressions.Expression) string        { return g.binary(e, "<=") }
func (g *Generator) bitwiseAndSQL(e expressions.Expression) string { return g.binary(e, "&") }
func (g *Generator) bitwiseOrSQL(e expressions.Expression) string  { return g.binary(e, "|") }
func (g *Generator) bitwiseXorSQL(e expressions.Expression) string { return g.binary(e, "^") }
func (g *Generator) dpipeSQL(e expressions.Expression) string      { return g.binary(e, "||") }
func (g *Generator) nullSafeEQSQL(e expressions.Expression) string {
	return g.binary(e, "IS NOT DISTINCT FROM")
}
func (g *Generator) nullSafeNEQSQL(e expressions.Expression) string {
	return g.binary(e, "IS DISTINCT FROM")
}
func (g *Generator) isSQL(e expressions.Expression) string {
	// is_sql (generator.py:4514-4519, v30.17.0): Is(negate=True) renders `IS NOT`
	// (postgres NOTNULL keeps this shape; NORMALIZE_NOT_NULL=False). When IS_BOOL_ALLOWED
	// is false (StarRocks), `x IS TRUE` / `x IS NOT FALSE` become the bare operand or its NOT.
	negate := boolValue(e.Arg("negate"))
	if !g.dialect.IsBoolAllowed {
		if boolArg := asExpression(e.Arg("expression")); boolArg != nil && boolArg.Kind() == expressions.KindBoolean {
			positive := boolValue(boolArg.Arg("this")) != negate
			if positive {
				return g.sqlKey(e, "this")
			}
			return "NOT " + g.sqlKey(e, "this")
		}
	}
	if negate {
		return g.binary(e, "IS NOT")
	}
	return g.binary(e, "IS")
}
func (g *Generator) likeSQL(e expressions.Expression) string  { return g.likeSQLWithOp(e, "LIKE") }
func (g *Generator) ilikeSQL(e expressions.Expression) string { return g.likeSQLWithOp(e, "ILIKE") }

func (g *Generator) likeSQLWithOp(e expressions.Expression, op string) string {
	if g.isDialect("duckdb") {
		if sql, ok := g.likeQuantifierSQL(e, nil, e.Parent()); ok {
			return sql
		}
	}
	if boolValue(e.Arg("negate")) {
		op = "NOT " + op
	}
	return g.binary(e, op)
}

// likeQuantifierSQL ports generator.py:4539-4566 _like_sql's quantifier expansion
// (SUPPORTS_LIKE_QUANTIFIERS=false, i.e. DuckDB): LIKE ANY/ALL over a tuple becomes a
// disjunction/conjunction of individual LIKE predicates.
func (g *Generator) likeQuantifierSQL(e expressions.Expression, escape expressions.Expression, parent expressions.Expression) (string, bool) {
	rhs := asExpression(e.Arg("expression"))
	if rhs == nil || (rhs.Kind() != expressions.KindAny && rhs.Kind() != expressions.KindAll) {
		return "", false
	}
	exprs := unnestLikeQuantifier(asExpression(rhs.This()))
	if len(exprs) == 0 {
		return "", false
	}
	negate := boolValue(e.Arg("negate"))
	kind := e.Kind()
	makeLike := func(value expressions.Expression) expressions.Expression {
		like := expressions.New(kind, expressions.Args{
			"this": e.This(), "expression": value, "negate": negate,
		})
		if escape != nil {
			like = expressions.Escape(expressions.Args{
				"this": like, "expression": escape.Arg("expression"),
			})
		}
		return like
	}
	result := makeLike(exprs[0])
	connective := expressions.KindOr
	if rhs.Kind() == expressions.KindAll {
		connective = expressions.KindAnd
	}
	for _, value := range exprs[1:] {
		prev := result
		if prev.Kind() == connective {
			prev = expressions.Paren(expressions.Args{"this": prev})
		}
		result = expressions.New(connective, expressions.Args{
			"this": prev, "expression": makeLike(value),
		})
	}
	sql := g.gen(result)
	// generator.py:4560-4565: group the expansion unless the parent is the same connective or
	// already a Paren. Every other condition parent (WHERE, HAVING, JOIN ON, ...) needs the
	// parentheses so SQL precedence cannot re-associate the expansion.
	if len(exprs) > 1 && parent != nil && parent.Kind() != connective && parent.Kind() != expressions.KindParen && isConditionParent(parent) {
		sql = "(" + sql + ")"
	}
	return sql, true
}

// isConditionParent reports whether a node is an upstream exp.Condition, the gate for wrapping
// a LIKE quantifier expansion (generator.py:4562). Connectors/functions carry TraitCondition;
// the clause nodes that hold a predicate are listed explicitly (our traits table leaves them
// as plain Expressions).
func isConditionParent(e expressions.Expression) bool {
	switch e.Kind() {
	case expressions.KindWhere, expressions.KindHaving, expressions.KindJoin, expressions.KindIf,
		expressions.KindFilter, expressions.KindWhen, expressions.KindCase:
		return true
	}
	return e.Is(expressions.TraitCondition)
}

// unnestLikeQuantifier unwraps nested Paren/Tuple nodes into the operand list.
func unnestLikeQuantifier(e expressions.Expression) []expressions.Expression {
	if e == nil {
		return nil
	}
	switch e.Kind() {
	case expressions.KindTuple:
		return e.Expressions()
	case expressions.KindParen:
		return unnestLikeQuantifier(asExpression(e.This()))
	}
	return []expressions.Expression{e}
}

// similarToSQL ports generator.py:4496 similarto_sql. Unlike Like/ILike, SimilarTo has no
// "negate" arg (expressions/core.py:2226): `NOT x SIMILAR TO y` is represented as
// Not(SimilarTo(...)) by parseRange, so notSQL renders the NOT prefix instead.
func (g *Generator) similarToSQL(e expressions.Expression) string { return g.binary(e, "SIMILAR TO") }

// varMapSQL ports varmap_sql: MAP(keys, values).
func (g *Generator) varMapSQL(e expressions.Expression) string {
	return g.funcCall("MAP", []any{e.Arg("keys"), e.Arg("values")}, "(", ")", true)
}

// decodeCaseSQL is generator.py:6143 decodecase_sql with SUPPORTS_DECODE_CASE=true.
func (g *Generator) decodeCaseSQL(e expressions.Expression) string {
	var args []any
	for _, expr := range e.Expressions() {
		args = append(args, expr)
	}
	return g.funcCall("DECODE", args, "(", ")", true)
}

// safeDivideSQL ports generator.py:4472-4476 safedivide_sql.
func (g *Generator) safeDivideSQL(e expressions.Expression) string {
	n := asExpression(e.This())
	d := asExpression(e.Arg("expression"))
	if n != nil && n.Is(expressions.TraitBinary) {
		n = expressions.Paren(expressions.Args{"this": n})
	}
	if d != nil && d.Is(expressions.TraitBinary) {
		d = expressions.Paren(expressions.Args{"this": d})
	}
	return g.gen(expressions.If(expressions.Args{
		"this":  expressions.NEQ(expressions.Args{"this": d, "expression": expressions.LiteralNumber("0")}),
		"true":  expressions.Div(expressions.Args{"this": n, "expression": d}),
		"false": expressions.Null(),
	}))
}

// escapeSQL ports generator.py:4411 escape_sql, including the LIKE-ANY/ALL quantifier
// expansion for dialects without quantifier support (DuckDB).
func (g *Generator) escapeSQL(e expressions.Expression) string {
	if g.isDialect("duckdb") {
		if like := asExpression(e.This()); like != nil &&
			(like.Kind() == expressions.KindLike || like.Kind() == expressions.KindILike) &&
			func() bool {
				rhs := asExpression(like.Arg("expression"))
				return rhs != nil && (rhs.Kind() == expressions.KindAny || rhs.Kind() == expressions.KindAll)
			}() {
			if sql, ok := g.likeQuantifierSQL(like, e, e.Parent()); ok {
				return sql
			}
		}
	}
	return g.binary(e, "ESCAPE")
}

func (g *Generator) andSQL(e expressions.Expression) string { return g.connectorSQL(e, "AND", nil) }
func (g *Generator) orSQL(e expressions.Expression) string  { return g.connectorSQL(e, "OR", nil) }

// connectorOp maps a TraitConnector node to its SQL operator token, so the
// flatten walk in connectorSQL renders each nested connector with its own
// operator (mirroring upstream's getattr(self, f"{node.key}_sql") dispatch,
// generator.py:4046) instead of assuming AND/OR. Only And/Or/Xor carry
// TraitConnector; the default covers Or.
func connectorOp(k expressions.Kind) string {
	switch k {
	case expressions.KindAnd:
		return "AND"
	case expressions.KindXor:
		return "XOR"
	default:
		return "OR"
	}
}

func (g *Generator) connectorSQL(e expressions.Expression, op string, stack *[]any) string {
	if stack != nil {
		if exprs := g.expressions(exprsOptions{expression: e, sep: " " + op + " "}); exprs != "" && truthy(e.Arg("expressions")) {
			*stack = append(*stack, exprs)
		} else {
			*stack = append(*stack, e.Right())
			if len(e.Comments()) > 0 && g.comments {
				op = g.maybeComment(op, nil, e.Comments(), false)
			}
			*stack = append(*stack, op, e.Left())
		}
		return op
	}
	work := []any{e}
	sqls := []string{}
	ops := map[string]bool{}
	for len(work) > 0 {
		node := work[len(work)-1]
		work = work[:len(work)-1]
		if expr, ok := node.(expressions.Expression); ok && !isNilExpression(expr) && expr.Is(expressions.TraitConnector) {
			emitted := g.connectorSQL(expr, connectorOp(expr.Kind()), &work)
			ops[emitted] = true
		} else {
			sql := g.gen(node)
			// TSQL's ENSURE_BOOLS (generators/tsql.py:137) is an AST preprocess upstream
			// (transforms.ensure_bools -> canonicalize.ensure_bools:149-163, scoped to
			// Connector operands, Not/If, and Where/Having). This port renders without
			// rewriting the tree, so connector leaves get the same ensure-bool wrap here —
			// exactly the nodes upstream's walk replaces — while tsqlAndSQL/tsqlOrSQL
			// delegate to this renderer for the flatten + pretty too_wide breaks.
			if g.dialect.Name == "tsql" {
				if expr, ok := node.(expressions.Expression); ok && !isNilExpression(expr) {
					sql = g.ensureBoolOperand(expr)
				}
			}
			if len(sqls) > 0 && ops[sqls[len(sqls)-1]] {
				sqls[len(sqls)-1] += " " + sql
			} else {
				sqls = append(sqls, sql)
			}
		}
	}
	sep := " "
	if g.pretty && g.tooWide(sqls) {
		sep = "\n"
	}
	return strings.Join(sqls, sep)
}

func (g *Generator) subquerySQL(e expressions.Expression) string { return g.subquerySQLSep(e, " AS ") }

// subquerySQLSep is subquerySQL with an explicit alias separator (generators/oracle.py:92).
func (g *Generator) subquerySQLSep(e expressions.Expression, sep string) string {
	alias := g.sqlKey(e, "alias")
	if alias != "" {
		alias = sep + alias
	}
	sample := g.sqlKey(e, "sample")
	if g.dialect.AliasPostTablesample && sample != "" {
		alias = sample + alias
		e.Set("sample", nil)
	}
	pivots := g.expressions(exprsOptions{expression: e, key: "pivots", emptySep: true, flat: true})
	sql := g.queryModifiers(e, g.wrap(e), alias, pivots)
	return g.prependCtes(e, sql)
}

func (g *Generator) betweenSQL(e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	low := g.sqlKey(e, "low")
	high := g.sqlKey(e, "high")
	symmetric, hasSymmetric := e.Arg("symmetric").(bool)
	if symmetric && !g.dialect.SupportsBetweenFlags {
		return "(" + this + " BETWEEN " + low + " AND " + high + " OR " + this + " BETWEEN " + high + " AND " + low + ")"
	}
	flag := ""
	if symmetric {
		flag = " SYMMETRIC"
	} else if hasSymmetric && g.dialect.SupportsBetweenFlags {
		// ASYMMETRIC is the default semantics but an explicit spelling is preserved where the
		// dialect supports the flags; elsewhere it is silently dropped (identical semantics).
		flag = " ASYMMETRIC"
	}
	return this + " BETWEEN" + flag + " " + low + " AND " + high
}

func (g *Generator) inSQL(e expressions.Expression) string {
	query := e.Arg("query")
	unnest := asExpression(e.Arg("unnest"))
	field := e.Arg("field")
	isGlobal := ""
	if boolValue(e.Arg("is_global")) {
		isGlobal = " GLOBAL"
	}
	var inSQL string
	if truthy(query) {
		inSQL = g.gen(query)
	} else if unnest != nil {
		inSQL = g.inUnnestOp(unnest)
	} else if truthy(field) {
		inSQL = g.gen(field)
	} else {
		inSQL = "(" + g.expressions(exprsOptions{expression: e, dynamic: true, newLine: true, skipFirst: true, skipLast: true}) + ")"
	}
	return g.sqlKey(e, "this") + isGlobal + " IN " + inSQL
}

func (g *Generator) inUnnestOp(unnest expressions.Expression) string {
	if g.dialect.InUnnestOpBare {
		return g.gen(unnest)
	}
	return "(SELECT " + g.gen(unnest) + ")"
}

func (g *Generator) existsSQL(e expressions.Expression) string { return "EXISTS" + g.wrap(e) }

func (g *Generator) anySQL(e expressions.Expression) string {
	thisExpr := asExpression(e.Arg("this"))
	this := g.sqlKey(e, "this")
	if thisExpr != nil && (isUnwrappedQuery(thisExpr.Kind()) || thisExpr.Kind() == expressions.KindParen) {
		if isUnwrappedQuery(thisExpr.Kind()) {
			this = g.wrap(this)
		}
		return "ANY" + this
	}
	return "ANY " + this
}

func (g *Generator) allSQL(e expressions.Expression) string {
	thisExpr := asExpression(e.Arg("this"))
	this := g.sqlKey(e, "this")
	if thisExpr == nil || (thisExpr.Kind() != expressions.KindTuple && thisExpr.Kind() != expressions.KindParen) {
		this = g.wrap(this)
	}
	return "ALL " + this
}

func (g *Generator) caseSQL(e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	statements := []string{}
	if this != "" {
		statements = append(statements, "CASE "+this)
	} else {
		statements = append(statements, "CASE")
	}
	for _, item := range listFromValue(e.Arg("ifs")) {
		ifExpr := asExpression(item)
		statements = append(statements, "WHEN "+g.sqlKey(ifExpr, "this"), "THEN "+g.sqlKey(ifExpr, "true"))
	}
	defaultSQL := g.sqlKey(e, "default")
	if defaultSQL != "" {
		statements = append(statements, "ELSE "+defaultSQL)
	}
	statements = append(statements, "END")
	if g.pretty && g.tooWide(statements) {
		return g.indent(strings.Join(statements, "\n"), 0, nil, true, true)
	}
	return strings.Join(statements, " ")
}

// convertSQL ports convert_sql (generator.py:5308-5353): T-SQL's CONVERT(type, value, style)
// becomes a formatted parse when the style maps to a known pattern, else a CAST/TRY_CAST.
func (g *Generator) convertSQL(e expressions.Expression) string {
	to := asExpression(e.Arg("this"))
	value := asExpression(e.Arg("expression"))
	style := asExpression(e.Arg("style"))
	safe := truthy(e.Arg("safe"))
	if to == nil || value == nil {
		return ""
	}
	// A NULL style makes SQL Server return NULL.
	if style != nil && style.Kind() == expressions.KindNull {
		return "NULL"
	}
	dtypeForStyle, _ := to.Arg("this").(expressions.DType)
	if len(to.Expressions()) == 0 {
		switch dtypeForStyle {
		case expressions.DTypeVarchar, expressions.DTypeNVarchar, expressions.DTypeChar, expressions.DTypeNChar:
			to = expressions.DataType(expressions.Args{
				"this":        dtypeForStyle,
				"expressions": []expressions.Expression{expressions.LiteralNumber("30")},
			})
		}
	}
	var transformed expressions.Expression
	if style != nil && style.IsInt() {
		if converted, ok := dialects.TSQLConvertFormatMapping[style.Name()]; ok {
			fmt := expressions.LiteralString(converted)
			dtype, _ := to.Arg("this").(expressions.DType)
			switch dtype {
			case expressions.DTypeDate:
				transformed = expressions.StrToDate(expressions.Args{"this": value, "format": fmt, "safe": safe})
			case expressions.DTypeDatetime, expressions.DTypeDatetime2:
				transformed = expressions.StrToTime(expressions.Args{"this": value, "format": fmt, "safe": safe})
			case expressions.DTypeVarchar, expressions.DTypeNVarchar, expressions.DTypeChar, expressions.DTypeNChar:
				transformed = expressions.Cast(expressions.Args{
					"this": expressions.New(expressions.KindTimeToStr, expressions.Args{"this": value, "format": fmt}),
					"to":   to, "safe": safe,
				})
			case expressions.DTypeText:
				transformed = expressions.New(expressions.KindTimeToStr, expressions.Args{"this": value, "format": fmt})
			}
		} else {
			g.unsupported("Unsupported T-SQL 'style' value: " + style.Name())
		}
	}
	if transformed == nil {
		kind := expressions.KindCast
		if safe {
			kind = expressions.KindTryCast
		}
		transformed = expressions.New(kind, expressions.Args{"this": value, "to": to})
	}
	return g.gen(transformed)
}

// forClauseSQL ports forclause_sql (generator.py:3310-3321): a FOR clause with no renderable
// query option is dropped.
func (g *Generator) forClauseSQL(e expressions.Expression) string {
	kind := e.Text("kind")
	if kind == "BROWSE" {
		return g.sep() + "FOR BROWSE"
	}
	options := g.expressions(exprsOptions{expression: e, key: "expressions"})
	if options == "" {
		return ""
	}
	return g.sep() + "FOR " + kind + g.seg(options)
}

// queryOptionSQL ports queryoption_sql (generator.py:3323-3325): base dialects cannot render
// a T-SQL query option.
func (g *Generator) queryOptionSQL(expressions.Expression) string {
	g.unsupported("Unsupported query option.")
	return ""
}

// fromTimeZoneSQL ports fromtimezone_sql (generator.py:4056-4059).
func (g *Generator) fromTimeZoneSQL(e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	zone := g.sqlKey(e, "zone")
	return this + " AT TIME ZONE " + zone + " AT TIME ZONE 'UTC'"
}

func (g *Generator) ifSQL(e expressions.Expression) string {
	return g.caseSQL(expressions.Case(expressions.Args{"ifs": []expressions.Expression{e}, "default": e.Arg("false")}))
}

func (g *Generator) windowSQL(e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	partition := g.partitionBySQL(e)
	orderExpr := asExpression(e.Arg("order"))
	order := ""
	if orderExpr != nil {
		order = g.orderSQLFlat(orderExpr, true)
	}
	spec := g.sqlKey(e, "spec")
	alias := g.sqlKey(e, "alias")
	over := g.sqlKey(e, "over")
	if over == "" {
		over = "OVER"
	}
	if e.ArgKey() == "windows" {
		this += " AS"
	} else {
		this += " " + over
	}
	first := ""
	if v, ok := e.Arg("first").(bool); ok {
		if v {
			first = "FIRST"
		} else {
			first = "LAST"
		}
	}
	if partition == "" && order == "" && spec == "" && alias != "" {
		return this + " " + alias
	}
	args := []any{}
	for _, arg := range []string{alias, first, partition, order, spec} {
		if arg != "" {
			args = append(args, arg)
		}
	}
	return this + " (" + g.formatArgs(args, " ") + ")"
}

func (g *Generator) partitionBySQL(e expressions.Expression) string {
	partition := g.expressions(exprsOptions{expression: e, key: "partition_by", flat: true})
	if partition != "" {
		return "PARTITION BY " + partition
	}
	return ""
}

func (g *Generator) windowSpecSQL(e expressions.Expression) string {
	kind := g.sqlKey(e, "kind")
	start := joinNonEmpty(" ", g.sqlKey(e, "start"), g.sqlKey(e, "start_side"))
	end := joinNonEmpty(" ", g.sqlKey(e, "end"), g.sqlKey(e, "end_side"))
	if end == "" {
		end = "CURRENT ROW"
	}
	windowSpec := kind + " BETWEEN " + start + " AND " + end
	exclude := g.sqlKey(e, "exclude")
	if exclude != "" {
		// SUPPORTS_WINDOW_EXCLUDE (generator.py:512): base defaults to unsupported (dropping
		// EXCLUDE with a warning); dialects that support it render it.
		if g.dialect.SupportsWindowExclude {
			windowSpec += " EXCLUDE " + exclude
		} else {
			g.unsupported("EXCLUDE clause is not supported in the WINDOW clause")
		}
	}
	return windowSpec
}

func (g *Generator) filterSQL(e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	where := strings.TrimSpace(g.sqlKey(e, "expression"))
	return this + " FILTER(" + where + ")"
}

func (g *Generator) withinGroupSQL(e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	// order (the expression) has a leading separator (space when flat, "\n" when
	// pretty); strip the first char unconditionally to mirror Python's [1:].
	expressionSQL := g.sqlKey(e, "expression")
	if expressionSQL != "" {
		expressionSQL = expressionSQL[1:]
	}
	return this + " WITHIN GROUP (" + expressionSQL + ")"
}

func (g *Generator) distinctSQL(e expressions.Expression) string {
	this := g.expressions(exprsOptions{expression: e, flat: true})
	if !g.dialect.MultiArgDistinct && len(e.Expressions()) > 1 {
		// generators/generator.py:4411-4424: COUNT(DISTINCT a, b) becomes
		// COUNT(DISTINCT CASE WHEN a IS NULL THEN NULL WHEN b IS NULL THEN NULL ELSE (a, b) END).
		whens := make([]expressions.Expression, 0, len(e.Expressions()))
		for _, arg := range e.Expressions() {
			isNull := expressions.Is(expressions.Args{"this": arg, "expression": expressions.Null()})
			whens = append(whens, expressions.If(expressions.Args{
				"this": isNull, "true": expressions.Null(),
			}))
		}
		caseExpr := expressions.Case(expressions.Args{
			"ifs":     whens,
			"default": expressions.Tuple(expressions.Args{"expressions": e.Expressions()}),
		})
		this = g.gen(caseExpr)
	}
	if this != "" {
		this = " " + this
	}
	on := g.sqlKey(e, "on")
	if on != "" {
		on = " ON " + on
	}
	return "DISTINCT" + this + on
}

func (g *Generator) ignoreNullsSQL(e expressions.Expression) string {
	if g.isDialect("postgres") {
		// generators/postgres.py:545-548
		// https://www.postgresql.org/docs/current/functions-window.html
		g.unsupported("PostgreSQL does not support IGNORE NULLS.")
		return g.sqlKey(e, "this")
	}
	return g.embedIgnoreNulls(e, "IGNORE NULLS")
}
func (g *Generator) respectNullsSQL(e expressions.Expression) string {
	if g.isDialect("postgres") {
		// generators/postgres.py:550-553
		g.unsupported("PostgreSQL does not support RESPECT NULLS.")
		return g.sqlKey(e, "this")
	}
	return g.embedIgnoreNulls(e, "RESPECT NULLS")
}

func (g *Generator) embedIgnoreNulls(e expressions.Expression, text string) string {
	// generator.py:5396-5423: with IGNORE_NULLS_IN_FUNC the modifier moves inside the aggregate
	// call, before any HAVING MAX/ORDER BY/LIMIT modifiers when IGNORE_NULLS_BEFORE_ORDER.
	if g.dialect.IgnoreNullsInFunc && !truthy(e.Meta()["inline"]) {
		if g.dialect.IgnoreNullsBeforeOrder {
			var mod expressions.Expression
			for _, kind := range []expressions.Kind{expressions.KindHavingMax, expressions.KindOrder, expressions.KindLimit} {
				if found := e.Find(kind); found != nil {
					mod = found
					break
				}
			}
			if mod != nil && mod.This() != nil {
				inner := expressions.New(e.Kind(), expressions.Args{"this": mod.This().Copy(), "inline": true})
				mod.This().Replace(inner)
				return g.sqlKey(e, "this")
			}
		}
		if agg := e.Find(expressions.TraitAggFunc); agg != nil {
			aggSQL := g.genNoComment(agg)
			aggSQL = strings.TrimSuffix(aggSQL, ")") + " " + text + ")"
			return g.maybeComment(aggSQL, nil, agg.Comments(), false)
		}
	}
	return g.sqlKey(e, "this") + " " + text
}

func (g *Generator) insertSQL(e expressions.Expression) string {
	keyword := "INSERT"
	if g.isDialect("mysql") && boolValue(e.Arg("replace")) {
		keyword = "REPLACE"
	}
	hint := g.sqlKey(e, "hint")
	thisKeyword := " INTO"
	if boolValue(e.Arg("overwrite")) {
		// generator.py:2294: the dialect's INSERT_OVERWRITE keyword; a Directory target
		// (Hive/Spark INSERT OVERWRITE [LOCAL] DIRECTORY ...) renders " OVERWRITE" without
		// the TABLE keyword.
		if this := asExpression(e.Arg("this")); this != nil && this.Kind() == expressions.KindDirectory {
			thisKeyword = " OVERWRITE"
		} else {
			thisKeyword = g.dialect.InsertOverwrite
		}
	}
	stored := g.sqlKey(e, "stored")
	if stored != "" {
		stored = " " + stored
	}
	alternative := e.Text("alternative")
	if alternative != "" {
		alternative = " OR " + alternative
	}
	ignore := ""
	if boolValue(e.Arg("ignore")) {
		ignore = " IGNORE"
	}
	if boolValue(e.Arg("is_function")) {
		thisKeyword += " FUNCTION"
	}
	this := thisKeyword + " " + g.sqlKey(e, "this")
	exists := ""
	if boolValue(e.Arg("exists")) {
		exists = " IF EXISTS"
	}
	where := g.sqlKey(e, "where")
	if where != "" {
		where = g.sep() + "REPLACE WHERE " + where
	}
	// v30.17.0 (generator.py:2309-2310): INSERT ... REPLACE USING (cols).
	using := g.expressions(exprsOptions{expression: e, key: "using", flat: true})
	if using != "" {
		using = g.sep() + "REPLACE USING (" + using + ")"
	}
	expressionSQL := g.sep() + g.sqlKey(e, "expression")
	onConflict := g.sqlKey(e, "conflict")
	if onConflict != "" {
		onConflict = " " + onConflict
	}
	byName := ""
	if boolValue(e.Arg("by_name")) {
		byName = " BY NAME"
	}
	defaultValues := ""
	if boolValue(e.Arg("default")) {
		defaultValues = "DEFAULT VALUES"
	}
	returning := g.sqlKey(e, "returning")
	if g.dialect.ReturningEnd {
		expressionSQL = expressionSQL + onConflict + defaultValues + returning
	} else {
		// generator.py:2318-2321: OUTPUT/RETURNING goes before the source.
		expressionSQL = returning + expressionSQL + onConflict
	}
	partitionBy := g.sqlKey(e, "partition")
	if partitionBy != "" {
		partitionBy = " " + partitionBy
	}
	settings := g.sqlKey(e, "settings")
	if settings != "" {
		settings = " " + settings
	}
	source := g.sqlKey(e, "source")
	if source != "" {
		source = "TABLE " + source
	}
	// ksqlDB `INSERT INTO sourceName (WITH tableProperties)? query`
	// (SqlBase.g4:86): the WITH clause sits between the target and the source.
	withProperties := ""
	if g.isDialect("ksqldb") {
		if props := g.withPropertiesSQL(asExpression(e.Arg("properties"))); props != "" {
			withProperties = " " + props
		}
	}
	sql := keyword + hint + alternative + ignore + this + stored + byName + exists + withProperties + partitionBy + settings + where + using + expressionSQL + source
	return g.prependCtes(e, sql)
}

// updateFromJoinsSQL ports _update_from_joins_sql (generator.py:2658-2688): dialects whose
// UPDATE has no FROM clause (UPDATE_STATEMENT_SUPPORTS_FROM = False, e.g. mysql) rewrite the
// parsed FROM into `JOIN ... ON TRUE` clauses placed after the target table, qualify
// unqualified SET columns with the target's alias, and drop the FROM from the output.
func (g *Generator) updateFromJoinsSQL(e expressions.Expression) (string, string) {
	fromExpr := asExpression(e.Arg("from_"))
	if g.dialect.UpdateStatementSupportsFrom || fromExpr == nil {
		return "", g.sqlKey(e, "from_")
	}

	// Qualify unqualified columns in the SET clause with the target table: MySQL requires
	// qualified column names in a multi-table UPDATE to avoid ambiguity.
	if targetTable := asExpression(e.Arg("this")); targetTable != nil && targetTable.Kind() == expressions.KindTable {
		targetName := expressions.ToIdentifier(targetTable.AliasOrName())
		for _, eq := range e.Expressions() {
			col := asExpression(eq.Arg("this"))
			if col != nil && col.Kind() == expressions.KindColumn && !truthy(col.Arg("table")) {
				col.Set("table", targetName)
			}
		}
	}

	table := asExpression(fromExpr.Arg("this"))
	var nestedJoins []expressions.Expression
	if table != nil {
		if joins, ok := table.Arg("joins").([]expressions.Expression); ok && len(joins) > 0 {
			nestedJoins = joins
			table.Set("joins", nil)
		}
	}

	joinSQL := g.gen(expressions.Join(expressions.Args{"this": table, "on": expressions.True_()}))
	for _, nested := range nestedJoins {
		if !truthy(nested.Arg("on")) && !truthy(nested.Arg("using")) {
			nested.Set("on", expressions.True_())
		}
		joinSQL += g.gen(nested)
	}
	return joinSQL, ""
}

func (g *Generator) updateSQL(e expressions.Expression) string {
	hint := g.sqlKey(e, "hint")
	this := g.sqlKey(e, "this")
	joinSQL, fromSQL := g.updateFromJoinsSQL(e)
	setSQL := g.expressions(exprsOptions{expression: e, flat: true})
	whereSQL := g.sqlKey(e, "where")
	returning := g.sqlKey(e, "returning")
	order := g.sqlKey(e, "order")
	limit := g.sqlKey(e, "limit")
	expressionSQL := fromSQL + whereSQL + returning
	if !g.dialect.ReturningEnd {
		expressionSQL = returning + fromSQL + whereSQL
	}
	options := g.expressions(exprsOptions{expression: e, key: "options"})
	if options != "" {
		options = " OPTION(" + options + ")"
	}
	sql := "UPDATE" + hint + " " + this + joinSQL + " SET " + setSQL + expressionSQL + order + limit + options
	return g.prependCtes(e, sql)
}

func (g *Generator) deleteSQL(e expressions.Expression) string {
	hint := g.sqlKey(e, "hint")
	this := g.sqlKey(e, "this")
	if this != "" {
		this = " FROM " + this
	}
	using := g.expressions(exprsOptions{expression: e, key: "using"})
	if using != "" {
		using = " USING " + using
	}
	cluster := g.sqlKey(e, "cluster")
	if cluster != "" {
		cluster = " " + cluster
	}
	where := g.sqlKey(e, "where")
	returning := g.sqlKey(e, "returning")
	order := g.sqlKey(e, "order")
	limit := g.sqlKey(e, "limit")
	tables := g.expressions(exprsOptions{expression: e, key: "tables"})
	if tables != "" {
		tables = " " + tables
	}
	expressionSQL := this + using + cluster + where + returning + order + limit
	if !g.dialect.ReturningEnd {
		expressionSQL = returning + this + using + cluster + where + order + limit
	}
	return g.prependCtes(e, "DELETE"+hint+tables+expressionSQL)
}

// mergeWithoutTarget ports merge_without_target_sql (dialects/dialect.py:2057-2090), postgres's
// exp.Merge generator transform (generators/postgres.py:337). It removes the target table (and
// its alias) qualifier from column refs on the LHS of each WHEN ... UPDATE SET assignment and
// from the INSERT column list, while leaving them intact in conditions and RHS values. It works
// on a Copy so the caller's AST is not mutated (upstream mutates in place via .replace()).
func (g *Generator) mergeWithoutTarget(e expressions.Expression) expressions.Expression {
	e = e.Copy()
	table := asExpression(e.Arg("this"))
	if table == nil {
		return e
	}
	// normalize returns the dialect-normalized name of an identifier for comparison only; it
	// copies first because NormalizeIdentifier mutates its argument in place.
	normalize := func(id expressions.Expression) string {
		if id == nil {
			return ""
		}
		return g.dialect.NormalizeIdentifier(id.Copy()).Name()
	}
	stripColumn := func(col expressions.Expression) expressions.Expression {
		return expressions.Column(expressions.Args{"this": col.Arg("this")})
	}
	isTarget := func(col expressions.Expression) bool {
		return col != nil && col.Kind() == expressions.KindColumn
	}

	targets := map[string]bool{normalize(asExpression(table.Arg("this"))): true}
	if alias := asExpression(table.Arg("alias")); alias != nil {
		targets[normalize(asExpression(alias.Arg("this")))] = true
	}

	whens := asExpression(e.Arg("whens"))
	if whens == nil {
		return e
	}
	for _, when := range whens.Expressions() {
		then := asExpression(when.Arg("then"))
		if then == nil {
			continue
		}
		switch then.Kind() {
		case expressions.KindUpdate:
			for _, eq := range then.FindAll(expressions.KindEQ) {
				lhs := asExpression(eq.Arg("this"))
				if isTarget(lhs) && targets[normalize(asExpression(lhs.Arg("table")))] {
					eq.Set("this", stripColumn(lhs))
				}
			}
		case expressions.KindInsert:
			columnList := asExpression(then.Arg("this"))
			if columnList == nil || columnList.Kind() != expressions.KindTuple {
				continue
			}
			cols := columnList.Expressions()
			newCols := make([]expressions.Expression, len(cols))
			changed := false
			for i, col := range cols {
				if isTarget(col) && targets[normalize(asExpression(col.Arg("table")))] {
					newCols[i] = stripColumn(col)
					changed = true
				} else {
					newCols[i] = col
				}
			}
			if changed {
				columnList.Set("expressions", newCols)
			}
		}
	}
	return e
}

func (g *Generator) mergeSQL(e expressions.Expression) string {
	// Postgres removes the target table qualifier from WHEN clause columns (see
	// mergeWithoutTarget); other dialects render the assignments verbatim.
	if g.isDialect("postgres") {
		e = g.mergeWithoutTarget(e)
	}
	table := asExpression(e.Arg("this"))
	tableAlias := ""
	// generator.py:4857-4866: T-SQL MERGE places the table hint before the alias.
	if table != nil {
		if hints, ok := table.Arg("hints").([]expressions.Expression); ok && len(hints) > 0 &&
			hints[0].Kind() == expressions.KindWithTableHint {
			if alias := asExpression(table.Arg("alias")); alias != nil {
				tableAlias = " AS " + g.gen(alias)
				table.Set("alias", nil)
			}
		}
	}
	this := g.gen(table)
	using := "USING " + g.sqlKey(e, "using")
	whens := g.sqlKey(e, "whens")
	on := g.sqlKey(e, "on")
	if on != "" {
		on = "ON " + on
	} else {
		on = g.expressions(exprsOptions{expression: e, key: "using_cond"})
		if on != "" {
			on = "USING (" + on + ")"
		}
	}
	returning := g.sqlKey(e, "returning")
	if returning != "" {
		whens += returning
	}
	sep := g.sep()
	return g.prependCtes(e, "MERGE INTO "+this+tableAlias+sep+using+sep+on+sep+whens)
}

func (g *Generator) whenSQL(e expressions.Expression) string {
	matched := "NOT MATCHED"
	if boolValue(e.Arg("matched")) {
		matched = "MATCHED"
	}
	source := ""
	if boolValue(e.Arg("source")) {
		source = " BY SOURCE"
	}
	condition := g.sqlKey(e, "condition")
	if condition != "" {
		condition = " AND " + condition
	}
	thenExpression := asExpression(e.Arg("then"))
	then := ""
	if thenExpression != nil && thenExpression.Kind() == expressions.KindInsert {
		this := g.sqlKey(thenExpression, "this")
		if this != "" {
			this = "INSERT " + this
		} else {
			this = "INSERT"
		}
		then = g.sqlKey(thenExpression, "expression")
		if then != "" {
			then = this + " VALUES " + then
		} else {
			then = this
		}
	} else if thenExpression != nil && thenExpression.Kind() == expressions.KindUpdate {
		// `WHEN MATCHED THEN UPDATE *` (a bare Star, not a SET list) renders `UPDATE *`
		// (generator.py:4755-4756), not `UPDATE SET *`.
		if star := asExpression(thenExpression.Arg("expressions")); star != nil && star.Kind() == expressions.KindStar {
			then = "UPDATE " + g.sqlKey(thenExpression, "expressions")
		} else {
			expressionsSQL := g.expressions(exprsOptions{expression: thenExpression})
			if expressionsSQL != "" {
				then = "UPDATE SET" + g.sep() + expressionsSQL
			} else {
				then = "UPDATE"
			}
		}
	} else {
		then = g.gen(thenExpression)
	}
	if thenExpression != nil && (thenExpression.Kind() == expressions.KindInsert || thenExpression.Kind() == expressions.KindUpdate) {
		then += g.sqlKey(thenExpression, "where")
	}
	return "WHEN " + matched + source + condition + " THEN " + then
}

func (g *Generator) whensSQL(e expressions.Expression) string {
	return g.expressions(exprsOptions{expression: e, sep: " ", noIndent: true})
}

func (g *Generator) onConflictSQL(e expressions.Expression) string {
	conflict := "ON CONFLICT"
	if boolValue(e.Arg("duplicate")) {
		conflict = "ON DUPLICATE KEY"
	}
	constraint := g.sqlKey(e, "constraint")
	if constraint != "" {
		constraint = " ON CONSTRAINT " + constraint
	}
	conflictKeys := g.expressions(exprsOptions{expression: e, key: "conflict_keys", flat: true})
	if conflictKeys != "" {
		conflictKeys = "(" + conflictKeys + ")"
	}
	indexPredicate := g.sqlKey(e, "index_predicate")
	conflictKeys = conflictKeys + indexPredicate + " "
	action := g.sqlKey(e, "action")
	expressionsSQL := g.expressions(exprsOptions{expression: e, flat: true})
	if expressionsSQL != "" {
		// generator.py:2339 DUPLICATE_KEY_UPDATE_WITH_SET (True by default; MySQL's own
		// generator omits SET, since real MySQL syntax is `ON DUPLICATE KEY UPDATE col = ...`).
		setKeyword := ""
		if g.dialect.DuplicateKeyUpdateWithSet {
			setKeyword = "SET "
		}
		expressionsSQL = " " + setKeyword + expressionsSQL
	}
	where := g.sqlKey(e, "where")
	return conflict + constraint + conflictKeys + action + expressionsSQL + where
}

func (g *Generator) returningSQL(e expressions.Expression) string {
	return g.seg("RETURNING") + " " + g.expressions(exprsOptions{expression: e, flat: true})
}

func (g *Generator) createSQL(e expressions.Expression) string {
	// MySQL CREATE {USER|ROLE} grammar extension: the body is not modeled; `this` holds a Command child
	// with the verbatim statement text, emitted as-is. Keying on the Command child (not the kind string)
	// keeps a future structured CREATE ROLE/USER, whose `this` is a real object, from being hijacked.
	// See parser/stmt_account_object.go.
	if body := asExpression(e.Arg("this")); body != nil && body.Kind() == expressions.KindCommand {
		return g.gen(body)
	}
	kind := g.sqlKey(e, "kind")
	properties := asExpression(e.Arg("properties"))

	// generator.py:1306-1313: a CONSTRAINT TRIGGER stores the modifier on the first
	// TriggerProperties node and restores it in front of the creatable kind here.
	if kind == "TRIGGER" && properties != nil {
		if propertyExpressions := properties.Expressions(); len(propertyExpressions) > 0 {
			first := propertyExpressions[0]
			if first.Kind() == expressions.KindTriggerProperties && boolValue(first.Arg("constraint")) {
				kind = "CONSTRAINT " + kind
			}
		}
	}

	propertyLocations := map[propertyLocation][]expressions.Expression{}
	if properties != nil {
		propertyLocations = g.locateProperties(properties)
	}
	// CREATE TABLE a (LIKE b): with no CREATE TABLE LIKE support the zero-row CTAS must live
	// outside the column list, so lift the property out of the schema before rendering it.
	schemaLikeSQL := ""
	if !g.dialect.SupportsCreateTableLike {
		if schema := asExpression(e.Arg("this")); schema != nil && schema.Kind() == expressions.KindSchema {
			entries := schema.Expressions()
			kept := make([]expressions.Expression, 0, len(entries))
			for _, entry := range entries {
				if entry != nil && entry.Kind() == expressions.KindLikeProperty {
					schemaLikeSQL = g.likePropertySQL(entry)
					continue
				}
				kept = append(kept, entry)
			}
			if schemaLikeSQL != "" {
				schema.Set("expressions", kept)
			}
		}
	}
	this := g.sqlKey(e, "this")
	// createable_sql override (generators/clickhouse.py:607-623): ClickHouse places
	// ON CLUSTER / UUID / REFRESH right after the object name, before the schema columns.
	if g.isDialect("clickhouse") && clickhouseOnClusterTargets[kind] {
		if postName := propertyLocations[propertyLocationPostName]; len(postName) > 0 {
			this = g.clickhouseCreateable(e, postName)
		}
	}
	// createable_sql override (generators/teradata.py:148-160): a TABLE's POST_NAME
	// properties sit between the name and its column list. The base createable_sql drops
	// POST_NAME silently (generator.py:1315-1316), which is why only Teradata renders them.
	if g.isDialect("teradata") && strings.ToUpper(kind) == "TABLE" {
		if postName := propertyLocations[propertyLocationPostName]; len(postName) > 0 {
			this = g.teradataCreateable(e, postName)
		}
	}

	propertiesSQL := ""
	postSchema := propertyLocations[propertyLocationPostSchema]
	postWith := propertyLocations[propertyLocationPostWith]
	if len(postSchema) > 0 || len(postWith) > 0 {
		combined := make([]expressions.Expression, 0, len(postSchema)+len(postWith))
		combined = append(combined, postSchema...)
		combined = append(combined, postWith...)
		propertiesSQL = g.gen(g.propertiesExpression(combined, e))
		if len(postSchema) > 0 {
			propertiesSQL = g.sep() + propertiesSQL
		} else if !g.pretty {
			propertiesSQL = " " + propertiesSQL
		}
	}

	begin := ""
	atomicBody := false
	if boolValue(e.Arg("begin")) {
		begin = " BEGIN"
	} else if str, ok := e.Arg("begin").(string); ok && str != "" {
		begin = " BEGIN " + str // PG `BEGIN ATOMIC` bodies keep the modifier
		atomicBody = true
	}
	expressionSQL := g.sqlKey(e, "expression")
	if expressionSQL != "" {
		if g.isDialect("duckdb") && properties != nil {
			for _, property := range properties.Expressions() {
				if property.Kind() == expressions.KindReturnsProperty && boolValue(property.Arg("is_table")) {
					expressionSQL = "TABLE " + expressionSQL
					break
				}
			}
		}
		expressionSQL = begin + g.sep() + expressionSQL
		// PG rejects `AS BEGIN ATOMIC`: the SQL-standard body replaces the AS clause.
		// generator.py:1362 additionally skips the AS when the body is a MacroOverloads
		// list: DuckDB's CREATE MACRO renders every signature, params and all, inside the
		// overload list itself (macrooverload_sql), so a leading AS would double it.
		body := asExpression(e.Arg("expression"))
		overloads := body != nil && body.Kind() == expressions.KindMacroOverloads
		returnAS := g.dialect.CreateFunctionReturnAs || body == nil || body.Kind() != expressions.KindReturn
		if !atomicBody && !overloads && returnAS {
			postAliasSQL := g.renderProperties(
				g.propertiesExpression(propertyLocations[propertyLocationPostAlias], e),
				"", ", ", "", false,
			)
			if postAliasSQL != "" {
				postAliasSQL = " " + postAliasSQL
			}
			expressionSQL = " AS" + postAliasSQL + expressionSQL
		}
	}

	postIndexSQL := g.renderProperties(
		g.propertiesExpression(propertyLocations[propertyLocationPostIndex], e),
		" ", ", ", "", false,
	)
	indexes := g.expressions(exprsOptions{expression: e, key: "indexes", noIndent: true, sep: " "})
	if indexes != "" {
		indexes = " " + indexes
	}
	indexSQL := indexes + postIndexSQL

	clustered := ""
	if clusteredArg := e.Arg("clustered"); clusteredArg != nil {
		if boolValue(clusteredArg) {
			clustered = " CLUSTERED COLUMNSTORE"
		} else {
			clustered = " NONCLUSTERED COLUMNSTORE"
		}
	}
	replace := ""
	if boolValue(e.Arg("replace")) {
		replace = " OR REPLACE"
	}
	refresh := ""
	if boolValue(e.Arg("refresh")) {
		refresh = " OR REFRESH"
	}
	unique := ""
	if boolValue(e.Arg("unique")) {
		unique = " UNIQUE"
	}
	postCreateSQL := g.renderProperties(
		g.propertiesExpression(propertyLocations[propertyLocationPostCreate], e),
		" ", " ", "", false,
	)
	modifiers := clustered + replace + refresh + unique + postCreateSQL

	postExpressionSQL := g.renderProperties(
		g.propertiesExpression(propertyLocations[propertyLocationPostExpression], e),
		" ", " ", "", false,
	)
	concurrently := ""
	if boolValue(e.Arg("concurrently")) {
		concurrently = " CONCURRENTLY"
	}
	exists := ""
	if boolValue(e.Arg("exists")) {
		exists = " IF NOT EXISTS"
	}
	noSchemaBinding := ""
	if boolValue(e.Arg("no_schema_binding")) {
		noSchemaBinding = " WITH NO SCHEMA BINDING"
	}
	clone := g.sqlKey(e, "clone")
	if clone != "" {
		clone = " " + clone
	}

	// generator.py:1431-1435 EXPRESSION_PRECEDES_PROPERTIES_CREATABLES: RisingWave's
	// `CREATE SINK ... AS <select> WITH (...)` renders the body before the WITH properties
	// (generators/risingwave.py:26).
	bodyAndProperties := propertiesSQL + expressionSQL
	if g.dialect.ExpressionPrecedesPropertiesCreatables[kind] {
		bodyAndProperties = expressionSQL + propertiesSQL
	}
	if schemaLikeSQL != "" {
		bodyAndProperties = " " + schemaLikeSQL + bodyAndProperties
	}
	sql := "CREATE" + modifiers + " " + kind + concurrently + exists + " " + this + bodyAndProperties + postExpressionSQL + indexSQL + noSchemaBinding + clone
	return g.prependCtes(e, sql)
}

func (g *Generator) schemaSQL(e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	sql := g.schemaColumnsSQL(e)
	if this != "" && sql != "" {
		return this + " " + sql
	}
	return this + sql
}

func (g *Generator) schemaColumnsSQL(e expressions.Expression) string {
	if len(listFromValue(e.Arg("expressions"))) > 0 {
		return "(" + g.sep("") + g.expressions(exprsOptions{expression: e}) + g.seg(")", "")
	}
	return ""
}

func (g *Generator) columnDefSQL(e expressions.Expression) string {
	column := g.sqlKey(e, "this")
	kind := g.sqlKey(e, "kind")

	// Postgres places a function parameter mode (IN/OUT/INOUT/VARIADIC) BEFORE the
	// parameter name and omits it from the trailing constraints
	// (generators/postgres.py:409-419); MySQL's IN/OUT/INOUT modes render the same way
	// (§15.1.17). Unlike upstream, which pop()s the constraint, we render around it so
	// Generate() stays side-effect free.
	modePrefix := ""
	if g.isDialect("postgres") || g.isDialect("mysql") {
		for _, item := range listFromValue(e.Arg("constraints")) {
			if c, ok := item.(expressions.Expression); ok && c != nil && c.Kind() == expressions.KindInOutColumnConstraint {
				modePrefix = g.gen(c) + " "
				break
			}
		}
	}

	var constraints string
	if modePrefix != "" {
		parts := make([]string, 0)
		for _, item := range listFromValue(e.Arg("constraints")) {
			c, ok := item.(expressions.Expression)
			if !ok || c == nil || c.Kind() == expressions.KindInOutColumnConstraint {
				continue
			}
			parts = append(parts, g.gen(c))
		}
		constraints = strings.Join(parts, " ")
	} else {
		constraints = g.expressions(exprsOptions{expression: e, key: "constraints", sep: " ", flat: true})
	}
	exists := ""
	if boolValue(e.Arg("exists")) {
		exists = "IF NOT EXISTS "
	}
	if kind != "" {
		kind = " " + kind
	}
	if constraints != "" {
		constraints = " " + constraints
	}
	position := g.sqlKey(e, "position")
	if position != "" {
		position = " " + position
	}

	if e.Find(expressions.KindComputedColumnConstraint) != nil && !g.dialect.ComputedColumnWithType {
		kind = ""
	}

	return modePrefix + exists + column + kind + constraints + position
}

func (g *Generator) commandSQL(e expressions.Expression) string {
	return strings.TrimSpace(g.sqlKey(e, "this") + " " + strings.TrimSpace(e.Text("expression")))
}

// mysqlUnsignedTypeMapping ports MySQL's UNSIGNED_TYPE_MAPPING (generators/mysql.py:236-244):
// each unsigned integer/decimal DType renders as its signed base name followed by " UNSIGNED"
// (e.g. UINT -> "INT UNSIGNED", UBIGINT(20) -> "BIGINT(20) UNSIGNED"). This is the self-contained
// subset of MySQL's TYPE_MAPPING needed for the DDL slice; the broader per-dialect TYPE_MAPPING
// table (ROADMAP 5b) is still out of scope, but these seven fixed entries don't require it.
var mysqlUnsignedTypeMapping = map[expressions.DType]string{
	expressions.DTypeUBigInt:    "BIGINT",
	expressions.DTypeUInt:       "INT",
	expressions.DTypeUMediumInt: "MEDIUMINT",
	expressions.DTypeUSmallInt:  "SMALLINT",
	expressions.DTypeUTinyInt:   "TINYINT",
	expressions.DTypeUDecimal:   "DECIMAL",
	expressions.DTypeUDouble:    "DOUBLE",
}

// typeMappingTable selects the TYPE_MAPPING table datatype_sql's generic lookup consults for
// the current dialect: mysqlTypeMapping is a full replacement (generators/mysql.py:255-273),
// postgresTypeMapping is base typeMapping plus postgres's delta (generators/postgres.py:271-284),
// and every other dialect (including base) uses the base typeMapping as-is.
func (g *Generator) typeMappingTable() map[expressions.DType]string {
	switch g.switchDialect() {
	case "mysql":
		return mysqlTypeMapping
	case "postgres":
		return postgresTypeMapping
	default:
		return typeMapping
	}
}

// dataTypeSQL ports datatype_sql (generator.py:1706-...) plus dialect-specific overrides:
// MySQL's VARCHAR_REQUIRES_SIZE -> TEXT rewrite (generators/mysql.py:670-677), MySQL's
// UNSIGNED_TYPE_MAPPING suffix (generators/mysql.py:678-684, via mysqlUnsignedTypeMapping
// above), Postgres's ARRAY/ENUM/FLOAT-with-precision rendering (generators/postgres.py:
// 477-489), and the dialect-aware TYPE_MAPPING table lookup (typeMappingTable above).
func (g *Generator) dataTypeSQL(e expressions.Expression) string {
	if g.isDialect("mysql") {
		if g.dialect.VarcharRequiresSize && expressions.IsType(e, expressions.DTypeVarchar) && len(listFromValue(e.Arg("expressions"))) == 0 {
			// VARCHAR must always have a size - if it doesn't, we always generate TEXT.
			return "TEXT"
		}
	} else if g.isDialect("postgres") {
		if expressions.IsType(e, expressions.DTypeArray) {
			if len(listFromValue(e.Arg("expressions"))) > 0 {
				valuesSQL := g.expressions(exprsOptions{expression: e, key: "values", flat: true})
				return g.expressions(exprsOptions{expression: e, flat: true}) + "[" + valuesSQL + "]"
			}
			return "ARRAY"
		}
		if expressions.IsType(e, expressions.DTypeEnum) {
			return "ENUM (" + g.expressions(exprsOptions{expression: e, flat: true}) + ")"
		}
		if (expressions.IsType(e, expressions.DTypeDouble) || expressions.IsType(e, expressions.DTypeFloat)) && len(listFromValue(e.Arg("expressions"))) > 0 {
			// Postgres doesn't support precision for REAL and DOUBLE PRECISION types.
			return "FLOAT(" + g.expressions(exprsOptions{expression: e, flat: true}) + ")"
		}
	}

	nested := ""
	values := ""
	exprNested := truthy(e.Arg("nested"))
	typeValue := e.Arg("this")
	// MySQL renders unsigned numeric types as "<signed base> UNSIGNED" (see mysqlUnsignedTypeMapping).
	mysqlUnsignedBase := ""
	if g.isDialect("mysql") {
		if dt, ok := typeValue.(expressions.DType); ok {
			mysqlUnsignedBase = mysqlUnsignedTypeMapping[dt]
		}
	}
	var interior string
	if exprNested && g.pretty {
		interior = g.expressions(exprsOptions{expression: e, dynamic: true, newLine: true, skipFirst: true, skipLast: true})
	} else {
		interior = g.expressions(exprsOptions{expression: e, flat: true})
	}
	var typeSQL string
	switch tv := typeValue.(type) {
	case expressions.DType:
		if mysqlUnsignedBase != "" {
			typeSQL = mysqlUnsignedBase
		} else if tv == expressions.DTypeUserDefined && truthy(e.Arg("kind")) {
			typeSQL = g.sqlKey(e, "kind")
		} else if tv == expressions.DTypeCharacterSet {
			return "CHAR CHARACTER SET " + g.sqlKey(e, "kind")
		} else if mapped, ok := g.lookupTypeMapping(tv); ok {
			typeSQL = mapped
		} else if mapped, ok := g.typeMappingTable()[tv]; ok {
			typeSQL = mapped
		} else {
			typeSQL = string(tv)
		}
	case expressions.Expression:
		typeSQL = g.gen(tv)
	case string:
		typeSQL = tv
	default:
		typeSQL = fmt.Sprint(tv)
	}
	if interior != "" {
		if exprNested {
			delims := g.dialect.StructDelimiter
			if delims[0] == "" {
				delims = [2]string{"<", ">"}
			}
			nested = delims[0] + interior + delims[1]
			if e.Arg("values") != nil {
				delims := [2]string{"(", ")"}
				if typeValue == expressions.DTypeArray {
					delims = [2]string{"[", "]"}
				}
				valuesSQL := g.expressions(exprsOptions{expression: e, key: "values", flat: true})
				values = delims[0] + valuesSQL + delims[1]
			}
		} else if typeValue == expressions.DTypeInterval {
			nested = " " + interior
		} else {
			nested = "(" + interior + ")"
		}
	}
	typeSQL = typeSQL + nested + values
	if mysqlUnsignedBase != "" {
		// Append the UNSIGNED suffix after the base name + any size/params (mysql.py:681-683
		// does `f"{super().datatype_sql(...)} UNSIGNED"`), e.g. "BIGINT(20) UNSIGNED".
		typeSQL += " UNSIGNED"
	}
	collate := g.sqlKey(e, "collate")
	if collate != "" {
		typeSQL += " COLLATE " + collate
	}
	return typeSQL
}

func (g *Generator) dataTypeParamSQL(e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	if suffix := g.sqlKey(e, "expression"); suffix != "" {
		return this + " " + suffix
	}
	return this
}

// pseudoTypeSQL/objectIdentifierSQL port generator.py:2316-2320 pseudotype_sql/
// objectidentifier_sql: both just re-emit the uppercased token text stored in "this"
// (see parser_types.go parseTypes' PSEUDO_TYPE/OBJECT_IDENTIFIER branches).
func (g *Generator) pseudoTypeSQL(e expressions.Expression) string { return e.Name() }

func (g *Generator) objectIdentifierSQL(e expressions.Expression) string { return e.Name() }

// atTimeZoneSQL ports generator.py:3995-3998 attimezone_sql, plus mysql's override
// (generators/mysql.py:796-798): MySQL has no AT TIME ZONE syntax, so it drops the zone
// entirely and flags the query unsupported, keeping just the wrapped expression.
func (g *Generator) atTimeZoneSQL(e expressions.Expression) string {
	if g.isDialect("mysql") {
		g.unsupported("AT TIME ZONE is not supported by MySQL")
		return g.sqlKey(e, "this")
	}
	this := g.sqlKey(e, "this")
	zone := g.sqlKey(e, "zone")
	return this + " AT TIME ZONE " + zone
}

func (g *Generator) castSQL(e expressions.Expression) string { return g.castSQLWithPrefix(e, "") }

func (g *Generator) castSQLWithPrefix(e expressions.Expression, safePrefix string) string {
	formatSQL := g.sqlKey(e, "format")
	if formatSQL != "" {
		formatSQL = " FORMAT " + formatSQL
	}

	toExpr := asExpression(e.Arg("to"))

	// MySQL's cast_sql override (generators/mysql.py:689-697): a CAST to a timezone-aware
	// timestamp type renders as a TIMESTAMP(...) function call (bypassing CAST entirely, and
	// any FORMAT/DEFAULT/action clauses along with it, matching upstream's early return), and
	// a CAST to a type outside MySQL's narrow CAST-target vocabulary is rewritten to
	// CHAR/SIGNED/UNSIGNED (mysqlCastMapping) before the ordinary CAST(... AS ...) below. The
	// rename is computed on a copy of the "to" DataType so the caller's expression tree (which
	// Generate already Copy()'d once at the top) isn't mutated a second time here.
	if g.isDialect("mysql") && toExpr != nil {
		if dt, ok := toExpr.Arg("this").(expressions.DType); ok {
			if g.timestampFuncTypes()[dt] {
				return g.funcCall("TIMESTAMP", []any{e.Arg("this")}, "(", ")", true)
			}
			if renamed, ok := g.castMapping()[dt]; ok {
				toCopy := toExpr.Copy()
				toCopy.Set("this", renamed)
				toExpr = toCopy
			}
		}
	}

	toSQL := ""
	if toExpr != nil {
		toSQL = " " + g.gen(toExpr)
	}
	action := g.sqlKey(e, "action")
	if action != "" {
		action = " " + action
	}
	defaultSQL := g.sqlKey(e, "default")
	if defaultSQL != "" {
		defaultSQL = " DEFAULT " + defaultSQL + " ON CONVERSION ERROR"
	}
	return safePrefix + "CAST(" + g.sqlKey(e, "this") + " AS" + toSQL + defaultSQL + formatSQL + action + ")"
}

func (g *Generator) tryCastSQL(e expressions.Expression) string {
	// TRY_SUPPORTED (generator.py:543, base True; mysql/postgres/dremio disable it): a dialect
	// without TRY_CAST routes exp.TryCast through no_trycast_sql (dialects/dialect.py:1261),
	// which renders a plain CAST - so a MySQL cast-target rename (e.g. TEXT -> CHAR) applies
	// without a stray TRY_ prefix.
	if !g.dialect.TrySupported {
		return g.castSQLWithPrefix(e, "")
	}
	return g.castSQLWithPrefix(e, "TRY_")
}
func (g *Generator) jsonCastSQL(e expressions.Expression) string { return g.castSQLWithPrefix(e, "") }

// formatJSONSQL ports generator.py:3778 formatjson_sql.
func (g *Generator) formatJSONSQL(e expressions.Expression) string {
	return g.sqlKey(e, "this") + " FORMAT JSON"
}

// jsonColumnDefSQL ports generator.py:3838 jsoncolumndef_sql.
// jsonArrayAggSQL ports jsonarrayagg_sql (generator.py:3870-3882).
func (g *Generator) jsonArrayAggSQL(e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	order := g.sqlKey(e, "order")
	nullHandling := stringValue(e.Arg("null_handling"))
	if nullHandling != "" {
		nullHandling = " " + nullHandling
	}
	returnType := g.sqlKey(e, "return_type")
	if returnType != "" {
		returnType = " RETURNING " + returnType
	}
	strict := ""
	if truthy(e.Arg("strict")) {
		strict = " STRICT"
	}
	return "JSON_ARRAYAGG(" + this + order + nullHandling + returnType + strict + ")"
}

// openJSONSQL ports openjson_sql (generator.py:3924-3934).
func (g *Generator) openJSONSQL(e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	path := g.sqlKey(e, "path")
	if path != "" {
		path = ", " + path
	}
	expressionsSQL := g.expressions(exprsOptions{expression: e})
	with := ""
	if expressionsSQL != "" {
		with = " WITH (" + g.seg(g.indent(expressionsSQL, 0, nil, false, false), "") + g.seg(")", "")
	}
	return "OPENJSON(" + this + path + ")" + with
}

// openJSONColumnDefSQL ports openjsoncolumndef_sql (generator.py:3916-3922).
func (g *Generator) openJSONColumnDefSQL(e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	kind := g.sqlKey(e, "kind")
	path := g.sqlKey(e, "path")
	if path != "" {
		path = " " + path
	}
	asJSON := ""
	if truthy(e.Arg("as_json")) {
		asJSON = " AS JSON"
	}
	return this + " " + kind + path + asJSON
}

// periodForSystemTimeConstraintSQL ports periodforsystemtimeconstraint_sql
// (generator.py:1269-1272).
func (g *Generator) periodForSystemTimeConstraintSQL(e expressions.Expression) string {
	return "PERIOD FOR SYSTEM_TIME (" + g.sqlKey(e, "this") + ", " + g.sqlKey(e, "expression") + ")"
}

// withSystemVersioningPropertySQL ports withsystemversioningproperty_sql
// (generator.py:2266-2288).
func (g *Generator) withSystemVersioningPropertySQL(e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	if this != "" {
		this = "HISTORY_TABLE=" + this
	}
	// Upstream passes None (not "") for absent optional args (generator.py:2276-2281), so
	// formatArgs drops them; an empty Go string would render as an empty slot and emit ", ,".
	var dataConsistency any
	if v := g.sqlKey(e, "data_consistency"); v != "" {
		dataConsistency = "DATA_CONSISTENCY_CHECK=" + v
	}
	var retention any
	if v := g.sqlKey(e, "retention_period"); v != "" {
		retention = "HISTORY_RETENTION_PERIOD=" + v
	}
	onSQL := "OFF"
	if truthy(e.Arg("on")) {
		onSQL = "ON"
	}
	if this != "" {
		onSQL = g.funcCall("ON", []any{this, dataConsistency, retention}, "(", ")", false)
	}
	sql := "SYSTEM_VERSIONING=" + onSQL
	if truthy(e.Arg("with_")) {
		return "WITH(" + sql + ")"
	}
	return sql
}

// dataDeletionPropertySQL ports datadeletionproperty_sql (generator.py:5508-5518).
func (g *Generator) dataDeletionPropertySQL(e expressions.Expression) string {
	onSQL := "OFF"
	if truthy(e.Arg("on")) {
		onSQL = "ON"
	}
	// Same None-not-"" rule as withSystemVersioningPropertySQL above: absent optional
	// args must be nil so formatArgs drops them (generator.py:5512-5515).
	var filterColumn any
	if v := g.sqlKey(e, "filter_column"); v != "" {
		filterColumn = "FILTER_COLUMN=" + v
	}
	var retention any
	if v := g.sqlKey(e, "retention_period"); v != "" {
		retention = "RETENTION_PERIOD=" + v
	}
	if filterColumn != nil || retention != nil {
		onSQL = g.funcCall("ON", []any{filterColumn, retention}, "(", ")", false)
	}
	return "DATA_DELETION=" + onSQL
}

func (g *Generator) jsonColumnDefSQL(e expressions.Expression) string {
	path := g.sqlKey(e, "path")
	if path != "" {
		path = " PATH " + path
	}
	nestedSchema := g.sqlKey(e, "nested_schema")
	if nestedSchema != "" {
		return "NESTED" + path + " " + nestedSchema
	}
	this := g.sqlKey(e, "this")
	kind := g.sqlKey(e, "kind")
	if kind != "" {
		kind = " " + kind
	}
	formatJSON := ""
	if boolValue(e.Arg("format_json")) {
		formatJSON = " FORMAT JSON"
	}
	ordinality := ""
	if boolValue(e.Arg("ordinality")) {
		ordinality = " FOR ORDINALITY"
	}
	return this + kind + formatJSON + path + ordinality
}

// jsonSchemaSQL ports generator.py:3854 jsonschema_sql.
func (g *Generator) jsonSchemaSQL(e expressions.Expression) string {
	return g.funcCall("COLUMNS", listFromValue(e.Arg("expressions")), "(", ")", true)
}

// jsonTableSQL ports generator.py:3857 jsontable_sql. error_handling/empty_handling are
// rendered via sqlKey (rather than upstream's raw f-string) so they work whether
// parseOnHandling produced a literal string ("ERROR ON ERROR") or a DEFAULT <expr> node.
func (g *Generator) jsonTableSQL(e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	path := g.sqlKey(e, "path")
	if path != "" {
		path = ", " + path
	}
	errorHandling := g.sqlKey(e, "error_handling")
	if errorHandling != "" {
		errorHandling = " " + errorHandling
	}
	emptyHandling := g.sqlKey(e, "empty_handling")
	if emptyHandling != "" {
		emptyHandling = " " + emptyHandling
	}
	schema := g.sqlKey(e, "schema")
	suffix := path + errorHandling + emptyHandling + " " + schema + ")"
	return g.funcCall("JSON_TABLE", []any{this}, "(", suffix, true)
}

func (g *Generator) extractSQL(e expressions.Expression) string {
	thisExpr := asExpression(e.Arg("this"))
	if g.dialect.NormalizeExtractDateParts && thisExpr != nil {
		if mapped := g.dialect.DatePartMapping[strings.ToUpper(thisExpr.Name())]; mapped != "" {
			thisExpr = expressions.Var(expressions.Args{"this": mapped})
		}
	}
	this := g.sqlKey(e, "this")
	if thisExpr != nil {
		this = g.gen(thisExpr)
	}
	expressionSQL := g.sqlKey(e, "expression")
	return "EXTRACT(" + this + " FROM " + expressionSQL + ")"
}

// trimSQL ports trim_sql (generator.py:3624-3633): the base LTRIM/RTRIM/TRIM(this, expr) form.
// mysql and postgres both override exp.Trim to trimSQLStandard instead (dialect.py:1782-1797,
// wired at generators/mysql.py:221 and generators/postgres.py:376).
func (g *Generator) trimSQL(e expressions.Expression) string {
	if g.dialect.OpaqueFunctions {
		return g.trimSQLStandard(e)
	}
	if g.isDialect("mysql") || g.isDialect("postgres") {
		return g.trimSQLStandard(e)
	}
	return g.trimSQLBase(e)
}

// padSQL ports pad_sql (generator.py): LPAD/RPAD(this, expression[, fill_pattern]), chosen by
// the node's is_left flag.
func (g *Generator) padSQL(e expressions.Expression) string {
	prefix := "L"
	if !boolValue(e.Arg("is_left")) {
		prefix = "R"
	}
	fill := e.Arg("fill_pattern")
	// generator.py:5575-5577: a two-argument LPAD/RPAD renders an explicit space fill for
	// dialects whose PAD_FILL_PATTERN_IS_REQUIRED is true.
	if !truthy(fill) && g.dialect.PadFillPatternIsRequired {
		fill = expressions.LiteralString(" ")
	}
	return g.funcCall(prefix+"PAD", []any{e.Arg("this"), e.Arg("expression"), fill}, "(", ")", true)
}

func (g *Generator) trimSQLBase(e expressions.Expression) string {
	trimType := g.sqlKey(e, "position")
	funcName := "TRIM"
	switch trimType {
	case "LEADING":
		funcName = "LTRIM"
	case "TRAILING":
		funcName = "RTRIM"
	}
	this := e.Arg("this")
	// Upstream's _parse_bitwise folds a trailing COLLATE into a Collate node wrapping the operand
	// it follows (verified against sqlglot v30.12.0), so upstream's trim_sql re-renders the
	// COLLATE for free out of `this` and never reads a separate collation arg. This port has no
	// Collate node (expressions has no KindCollate) and keeps "collation" as Trim's own arg
	// (parser/parser_functions.go:107-109), so splice it back onto the target here whenever it's
	// present - with OR without a remove-chars operand. Examples (base dialect):
	//   TRIM(LEADING ' XXX ' COLLATE "de_DE")          -> LTRIM(' XXX ' COLLATE "de_DE")
	//   TRIM(BOTH 'bla' FROM ' XXX ' COLLATE utf8_bin) -> TRIM(' XXX ' COLLATE utf8_bin, 'bla')
	if collation := g.sqlKey(e, "collation"); collation != "" {
		this = g.sqlKey(e, "this") + " COLLATE " + collation
	}
	return g.funcCall(funcName, []any{this, e.Arg("expression")}, "(", ")", true)
}

// trimSQLStandard ports trim_sql (dialect.py:1782-1797): the SQL-standard TRIM(<position>
// <remove_chars> FROM <target> COLLATE <collation>) form shared by mysql and postgres. When
// there's no "remove chars" expression, it falls back to the LTRIM/RTRIM/TRIM(this, expr) form
// (trimSQLBase) instead, since that's the more idiomatic rendering for a plain whitespace trim.
func (g *Generator) trimSQLStandard(e expressions.Expression) string {
	removeChars := g.sqlKey(e, "expression")
	if removeChars == "" {
		if g.dialect.OpaqueFunctions {
			// opaque_functions: a Trim node only exists for keyword forms — TRIM(BOTH FROM s) /
			// TRIM(LEADING FROM s) must keep their grammar, never the TRIM(s)/LTRIM(s) call form
			// (which would reparse opaque).
			trimType := g.sqlKey(e, "position")
			if trimType != "" {
				trimType += " "
			}
			collation := g.sqlKey(e, "collation")
			if collation != "" {
				collation = " COLLATE " + collation
			}
			return "TRIM(" + trimType + "FROM " + g.sqlKey(e, "this") + collation + ")"
		}
		return g.trimSQLBase(e)
	}

	target := g.sqlKey(e, "this")
	trimType := g.sqlKey(e, "position")
	collation := g.sqlKey(e, "collation")

	if trimType != "" {
		trimType += " "
	}
	removeChars += " "
	fromPart := ""
	if trimType != "" || removeChars != "" {
		fromPart = "FROM "
	}
	if collation != "" {
		collation = " COLLATE " + collation
	}
	return "TRIM(" + trimType + removeChars + fromPart + target + collation + ")"
}

func (g *Generator) ceilFloorSQL(e expressions.Expression) string {
	toClause := g.sqlKey(e, "to")
	if toClause != "" {
		return g.sqlName(e.Kind()) + "(" + g.sqlKey(e, "this") + " TO " + toClause + ")"
	}
	return g.functionFallbackSQL(e)
}

func (g *Generator) anonymousSQL(e expressions.Expression) string {
	parent := e.Parent()
	isQualified := parent != nil && parent.Kind() == expressions.KindDot && asExpression(parent.Arg("expression")) == e
	// opaque_functions: the stored name is the source spelling and renders verbatim.
	normalize := !isQualified && !g.dialect.OpaqueFunctions
	return g.funcCall(g.sqlKey(e, "this"), listFromValue(e.Arg("expressions")), "(", ")", normalize)
}

func (g *Generator) hexSQL(e expressions.Expression) string {
	name := "HEX"
	if g.dialect.HexFunc != "" {
		name = g.dialect.HexFunc
	}
	text := g.funcCall(name, []any{e.Arg("this")}, "(", ")", true)
	if g.dialect.HexLowercase {
		text = g.funcCall("LOWER", []any{text}, "(", ")", true)
	}
	return text
}

// lowerHexSQL ports lowerhex_sql (generator.py:2027-2032).
func (g *Generator) lowerHexSQL(e expressions.Expression) string {
	name := "HEX"
	if g.dialect.HexFunc != "" {
		name = g.dialect.HexFunc
	}
	text := g.funcCall(name, []any{e.Arg("this")}, "(", ")", true)
	if !g.dialect.HexLowercase {
		text = g.funcCall("LOWER", []any{text}, "(", ")", true)
	}
	return text
}

// arrayAggSQL ports arrayagg_sql (generator.py:5747-5753): when the source dialect excludes
// NULLs from ARRAY_AGG, the target re-adds them with a FILTER over the non-NULL input.
func (g *Generator) arrayAggSQL(e expressions.Expression) string {
	sql := g.functionFallbackSQL(e)
	includesNulls := true
	if g.dialect.ArrayAggIncludesNulls != nil {
		includesNulls = *g.dialect.ArrayAggIncludesNulls
	}
	if !includesNulls || !truthy(e.Arg("nulls_excluded")) {
		return sql
	}
	column := asExpression(e.This())
	if column != nil && column.Kind() == expressions.KindOrder {
		column = column.This()
	}
	// A DISTINCT input checks its underlying expression, never the Distinct node itself.
	nullCheck := column
	if nullCheck != nil && nullCheck.Kind() == expressions.KindDistinct {
		if distinctArgs := nullCheck.Expressions(); len(distinctArgs) == 1 {
			nullCheck = distinctArgs[0]
		}
	}
	if parent := e.Parent(); parent != nil && parent.Kind() == expressions.KindFilter && nullCheck != nil {
		if where := asExpression(parent.Arg("expression")); where != nil {
			inner := where
			if where.Kind() == expressions.KindWhere {
				inner = where.This()
			}
			if inner != nil {
				// Keep the original predicate grouped so a disjunction still evaluates as a
				// whole before the null check is AND-ed in.
				original := inner.Copy()
				if original.Is(expressions.TraitConnector) {
					original = expressions.Paren(expressions.Args{"this": original})
				}
				nullExpr := nullCheck.Copy()
				if nullExpr.Is(expressions.TraitConnector) || nullExpr.Is(expressions.TraitBinary) {
					nullExpr = expressions.Paren(expressions.Args{"this": nullExpr})
				}
				notNull := expressions.Not(expressions.Args{
					"this": expressions.Is(expressions.Args{"this": nullExpr, "expression": expressions.Null()}),
				})
				inner.Replace(expressions.And(expressions.Args{"this": original, "expression": notNull}))
			}
		}
		return sql
	}
	// generator.py:5741-5745: only a column input gets the filter, and DISTINCT is rendered
	// without propagating the keyword into FILTER.
	if column != nil && column.Find(expressions.KindColumn) != nil {
		columnSQL := g.gen(column)
		if column.Kind() == expressions.KindDistinct {
			columnSQL = g.expressions(exprsOptions{expression: column, flat: true})
		} else if column.Is(expressions.TraitBinary) || column.Is(expressions.TraitConnector) {
			// The null check must apply to the whole aggregate input, not its last operand.
			columnSQL = "(" + columnSQL + ")"
		}
		sql += " FILTER(WHERE " + columnSQL + " IS NOT NULL)"
	}
	return sql
}

// arraySizeSQL ports arraysize_sql (generator.py:5886-5897). Base's nil setting drops a `1`
// dimension arg and warns on other values; DuckDB's false setting preserves supplied dimensions;
// Postgres's true setting preserves them and defaults an omitted dimension to `1`.
func (g *Generator) arraySizeSQL(e expressions.Expression) string {
	dim := asExpression(e.Arg("expression"))
	if g.isDialect("postgres") {
		if dim == nil {
			dim = expressions.LiteralNumber(1)
		}
	} else if !g.isDialect("duckdb") && dim != nil {
		if !dim.IsInt() || dim.Name() != "1" {
			g.unsupported("Cannot transpile dimension argument for ARRAY_LENGTH")
		}
		dim = nil
	}
	// ARRAY_SIZE_NAME (generator.py:593) defaults to ARRAY_LENGTH; teradata overrides it
	// with CARDINALITY (generators/teradata.py:55) and snowflake with ARRAY_SIZE
	// (generators/snowflake.py:397). The other per-dialect names (hive SIZE, presto
	// CARDINALITY, ...) are dispatch-level renames upstream of this method.
	name := "ARRAY_LENGTH"
	if g.isDialect("teradata") {
		name = "CARDINALITY"
	} else if g.isDialect("snowflake") {
		name = "ARRAY_SIZE"
	}
	return g.funcCall(name, []any{e.Arg("this"), dim}, "(", ")", true)
}

// arrayAnySQL ports arrayany_sql (generator.py:5252-5265): a CAN_IMPLEMENT_ARRAY_ANY
// dialect rewrites ARRAY_ANY into an emptiness check on the array versus on its
// FILTERed copy; every other dialect warns (upstream skips exactly the base Dialect,
// generator.py:5260-5262) and falls back to the plain function call. The ArrayFilter
// children are copied because Go nodes track their parent, while upstream re-parents
// the same objects freely.
func (g *Generator) arrayAnySQL(e expressions.Expression) string {
	if g.dialect.CanImplementArrayAny {
		filtered := expressions.ArrayFilter(expressions.Args{
			"this":       e.This().Copy(),
			"expression": e.Expr().Copy(),
		})
		filteredNotEmpty := expressions.NEQ(expressions.Args{
			"this":       expressions.ArraySize(expressions.Args{"this": filtered}),
			"expression": expressions.LiteralNumber(0),
		})
		originalIsEmpty := expressions.EQ(expressions.Args{
			"this":       expressions.ArraySize(expressions.Args{"this": e.This().Copy()}),
			"expression": expressions.LiteralNumber(0),
		})
		return g.gen(expressions.Paren(expressions.Args{
			"this": expressions.Or(expressions.Args{"this": originalIsEmpty, "expression": filteredNotEmpty}),
		}))
	}

	if g.dialect.Name != "base" {
		g.unsupported("ARRAY_ANY is unsupported")
	}
	return g.functionFallbackSQL(e)
}

func (g *Generator) initcapSQL(e expressions.Expression) string {
	delimiters := asExpression(e.Arg("expression"))
	// generator.py:6246-6260: a delimiters argument equal to the dialect's default set is
	// dropped, so round-tripping `INITCAP(x)` never grows a redundant argument.
	if delimiters != nil && delimiters.IsString() && delimiters.Name() == g.dialect.InitcapDefaultDelimiterChars {
		delimiters = nil
	}
	return g.funcCall("INITCAP", []any{e.Arg("this"), delimiters}, "(", ")", true)
}

// dateAddSQL ports the base generator's generic dateadd_sql (generator.py:5158-5164) for
// exp.DateAdd, plus MySQL's and Postgres's per-dialect overrides
// (generators/mysql.py:74-83 date_add_sql("ADD"), generators/postgres.py:55-77
// _date_add_sql("+")). Divergence from those overrides: upstream's dialect-specific
// FUNCTIONS parser entry (build_date_delta_with_interval, parsers/mysql.py:114) flattens
// `DATE_ADD(x, INTERVAL y UNIT)` at parse time into DateAdd(this=x, expression=y,
// unit=Var(UNIT)); this port's DATE_ADD FUNCTIONS entry is still the generic positional
// builder (expressions/functions.go genericFunction), so "expression" stays the whole
// parsed Interval node and "unit" is never set on DateAdd directly. mysqlDateAddSQL/
// postgresDateAddSQL below detect and unwrap that unflattened shape so the rendered SQL
// still matches upstream byte-for-byte, without requiring the parser-side flattening
// (out of this part's scope: dialect FUNCTIONS registries belong to a sibling part).
func (g *Generator) dateAddSQL(e expressions.Expression) string {
	switch {
	case g.isDialect("mysql"):
		return g.mysqlDateAddSQL("ADD", e)
	case g.isDialect("postgres"):
		return g.postgresDateAddSQL("+", e)
	}
	return g.funcCall("DATE_ADD", []any{e.Arg("this"), e.Arg("expression"), unitToStr(e)}, "(", ")", true)
}

// randSQL ports rand_sql (generator.py:5554-5560): with both bounds present the base
// generator renders the uniform-range expansion `(upper - lower) * RAND(this) + lower`,
// otherwise a plain RAND(this).
func (g *Generator) randSQL(e expressions.Expression) string {
	lower := g.sqlKey(e, "lower")
	upper := g.sqlKey(e, "upper")
	rand := g.funcCall("RAND", []any{e.Arg("this")}, "(", ")", true)
	if lower != "" && upper != "" {
		return "(" + upper + " - " + lower + ") * " + rand + " + " + lower
	}
	return rand
}

// mysqlDateAddSQL ports date_add_sql (generators/mysql.py:74-83): reconstructs
// `DATE_<kind>(this, INTERVAL <value> <unit>)` from the DateAdd's flattened
// expression/unit. See dateAddSQL's comment for why this port's parse doesn't actually
// flatten - dateIntervalArg below recovers the equivalent (this, unit) pair either way.
func (g *Generator) mysqlDateAddSQL(kind string, e expressions.Expression) string {
	value, unit := dateIntervalArg(e)
	interval := expressions.Interval(expressions.Args{"this": value, "unit": unitToVarValue(unit)})
	return g.funcCall("DATE_"+kind, []any{e.Arg("this"), interval}, "(", ")", true)
}

// postgresDateAddSQL ports _date_add_sql (generators/postgres.py:55-77), restricted to the
// isinstance(e, exp.Interval) branch: the other branches simplify a non-Interval second
// argument (a bare numeric day offset) via the optimizer's simplify() pass, which is out
// of scope for the gap this part targets (no round-trip corpus case exercises it for
// base/mysql/postgres). If the second argument isn't already an Interval, fall back to the
// generic `DATE_ADD(...)` rendering rather than guessing at upstream's simplify output.
func (g *Generator) postgresDateAddSQL(kind string, e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	value, unit := dateIntervalArg(e)
	if value == nil {
		return g.funcCall("DATE_ADD", []any{e.Arg("this"), e.Arg("expression"), unitToStr(e)}, "(", ")", true)
	}
	interval := expressions.Interval(expressions.Args{"this": value, "unit": unit})
	return this + " " + kind + " " + g.gen(interval)
}

// dateIntervalArg recovers the (value, unit) pair a DateAdd's "expression"/"unit" args
// represent, regardless of whether they were flattened at parse time (upstream's shape:
// unit set directly on DateAdd) or left as a single nested Interval (this port's shape,
// see dateAddSQL's comment). Returns (nil, nil) if "expression" isn't an Interval and no
// top-level "unit" was set, i.e. this port's parse produced neither shape.
func dateIntervalArg(e expressions.Expression) (value any, unit expressions.Expression) {
	if u := asExpression(e.Arg("unit")); u != nil {
		return e.Arg("expression"), u
	}
	if iv := asExpression(e.Arg("expression")); iv != nil && iv.Kind() == expressions.KindInterval {
		return iv.Arg("this"), asExpression(iv.Arg("unit"))
	}
	return nil, nil
}

func unitToStr(e expressions.Expression) expressions.Expression {
	return unitToStrValue(asExpression(e.Arg("unit")))
}

func unitToStrValue(unit expressions.Expression) expressions.Expression {
	if unit == nil {
		return expressions.LiteralString("DAY")
	}
	if unit.Kind() == expressions.KindPlaceholder || (unit.Kind() != expressions.KindVar && unit.Kind() != expressions.KindLiteral) {
		return unit
	}
	return expressions.LiteralString(unit.Name())
}

// unitToVarValue ports unit_to_var (dialects/dialect.py:2017-2023): unlike unitToStrValue,
// a Var/Placeholder/Column unit is kept as-is (not stringified), and a nil unit defaults to
// a bare Var("DAY") rather than a string literal.
func unitToVarValue(unit expressions.Expression) expressions.Expression {
	if unit != nil {
		switch unit.Kind() {
		case expressions.KindVar, expressions.KindPlaceholder, expressions.KindColumn, expressions.KindWeekStart:
			return unit
		}
	}
	value := "DAY"
	if unit != nil {
		value = unit.Name()
	}
	if value == "" {
		return nil
	}
	return expressions.Var(expressions.Args{"this": value})
}

func (g *Generator) logSQL(e expressions.Expression) string {
	// generator.py:4624-4637: with LOG_BASE_FIRST=False (BigQuery) the base is rendered first,
	// so the canonical this/expression pair is swapped.
	this := e.Arg("this")
	expr := asExpression(e.Arg("expression"))
	if expr != nil && (g.overrideKey() == "clickhouse" || g.overrideKey() == "presto") {
		baseExpression := asExpression(this)
		base := ""
		if baseExpression != nil {
			base = baseExpression.Name()
		}
		if baseExpression != nil && baseExpression.Kind() == expressions.KindLiteral && baseExpression.IsNumber() && (base == "2" || base == "10") {
			return g.funcCall("LOG"+base, []any{expr}, "(", ")", true)
		}
		g.unsupported("Unsupported logarithm with base " + g.gen(baseExpression))
	}
	if !g.dialect.LogBaseFirst {
		this, expr = expr, asExpression(this)
	}
	return g.funcCall("LOG", []any{this, expr}, "(", ")", true)
}

// dateFromUnixDateSQL ports generator.py:6201-6208: base SQLGlot renders the
// number of days since the Unix epoch as a date addition. BigQuery and Spark
// override this with their native DATE_FROM_UNIX_DATE function.
func (g *Generator) dateFromUnixDateSQL(e expressions.Expression) string {
	return g.gen(expressions.DateAdd(expressions.Args{
		"this":       expressions.Cast(expressions.Args{"this": expressions.LiteralString("1970-01-01"), "to": expressions.DTypeDate.IntoExpr(nil)}),
		"expression": e.Arg("this"),
		"unit":       expressions.Var(expressions.Args{"this": "DAY"}),
	}))
}

func (g *Generator) pivotSQL(e expressions.Expression) string {
	expressionsSQL := g.expressions(exprsOptions{expression: e, flat: true})
	direction := "PIVOT"
	if boolValue(e.Arg("unpivot")) {
		direction = "UNPIVOT"
	}
	group := g.sqlKey(e, "group")
	if truthy(e.Arg("this")) {
		this := g.sqlKey(e, "this")
		var sql string
		if expressionsSQL == "" {
			sql = "UNPIVOT " + this
		} else {
			on := g.seg("ON") + " " + expressionsSQL
			into := g.sqlKey(e, "into")
			if into != "" {
				into = g.seg("INTO") + " " + into
			}
			using := g.expressions(exprsOptions{expression: e, key: "using", flat: true})
			if using != "" {
				using = g.seg("USING") + " " + using
			}
			sql = direction + " " + this + on + into + using + group
		}
		return g.prependCtes(e, sql)
	}
	alias := g.sqlKey(e, "alias")
	if alias != "" {
		if g.dialect.PivotAliasWithAs {
			alias = " AS " + alias
		} else {
			alias = " " + alias
		}
	}
	fields := g.expressions(exprsOptions{expression: e, key: "fields", sep: " ", dynamic: true, newLine: true, skipFirst: true, skipLast: true})
	nulls := ""
	if v, ok := e.Arg("include_nulls").(bool); ok {
		if v {
			nulls = " INCLUDE NULLS "
		} else {
			nulls = " EXCLUDE NULLS "
		}
	}
	defaultOnNull := g.sqlKey(e, "default_on_null")
	if defaultOnNull != "" {
		defaultOnNull = " DEFAULT ON NULL (" + defaultOnNull + ")"
	}
	sql := g.seg(direction) + nulls + "(" + expressionsSQL + " FOR " + fields + defaultOnNull + group + ")" + alias
	return g.prependCtes(e, sql)
}

func (g *Generator) lateralOp(e expressions.Expression) string {
	crossApply, ok := e.Arg("cross_apply").(bool)
	if ok {
		if crossApply {
			return "INNER JOIN LATERAL"
		}
		return "LEFT JOIN LATERAL"
	}
	return "LATERAL"
}

func (g *Generator) lateralSQL(e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	if boolValue(e.Arg("view")) {
		alias := asExpression(e.Arg("alias"))
		columns := g.expressions(exprsOptions{expression: alias, key: "columns", flat: true})
		table := ""
		if alias != nil && alias.Name() != "" {
			table = " " + alias.Name()
		}
		if columns != "" {
			columns = " AS " + columns
		}
		opSQL := g.seg("LATERAL VIEW")
		if boolValue(e.Arg("outer")) {
			opSQL = g.seg("LATERAL VIEW OUTER")
		}
		return opSQL + g.sep() + this + table + columns
	}
	alias := g.sqlKey(e, "alias")
	if alias != "" {
		alias = " AS " + alias
	}
	ordinality := e.Text("ordinality")
	if truthy(e.Arg("ordinality")) {
		ordinality = " WITH ORDINALITY" + alias
		alias = ""
	}
	return g.lateralOp(e) + " " + this + alias + ordinality
}

func (g *Generator) valuesSQL(e expressions.Expression) string {
	return g.valuesSQLAsTable(e, g.dialect.ValuesAsTable)
}

// valuesSQLAsTable is valuesSQL with an explicit VALUES_AS_TABLE decision, so a dialect
// whose choice depends on the node (ClickHouse, generators/clickhouse.py:701-712) can
// override per call.
func (g *Generator) valuesSQLAsTable(e expressions.Expression, valuesAsTable bool) string {
	// generator.py:2676-2718 values_sql. A VALUES node stays a table constructor when the
	// dialect sets VALUES_AS_TABLE, or when it is an INSERT source (not under FROM/JOIN);
	// otherwise (MySQL, generators/mysql.py:139) it is rewritten into SELECT unions.
	if valuesAsTable || e.FindAncestor(expressions.KindFrom, expressions.KindJoin) == nil {
		args := g.expressions(exprsOptions{expression: e})
		alias := g.sqlKey(e, "alias")
		values := "VALUES" + g.seg("") + args
		parent := e.Parent()
		// generator.py:2684-2689 wraps a derived VALUES only when WRAP_DERIVED_VALUES is set
		// (MySQL clears it, so `VALUES (1, 2) AS t` stays bare — generators/mysql.py:148).
		if g.dialect.WrapDerivedValues && (alias != "" || (parent != nil && (parent.Kind() == expressions.KindFrom || parent.Kind() == expressions.KindTable))) {
			values = "(" + values + ")"
		}
		values = g.queryModifiers(e, values)
		if alias != "" {
			return values + " AS " + alias
		}
		return values
	}

	// generator.py:2693-2718 — convert `VALUES (...), (...)` under FROM/JOIN into a series of
	// SELECT unions, aliasing the first row's values from the table alias's column list.
	aliasNode := asExpression(e.Arg("alias"))
	var columnNames []any
	if aliasNode != nil {
		columnNames = listFromValue(aliasNode.Arg("columns"))
	}

	tuples := listFromValue(e.Arg("expressions"))
	selects := make([]expressions.Expression, 0, len(tuples))
	for i, tupleValue := range tuples {
		rowValues := listFromValue(asExpression(tupleValue).Arg("expressions"))
		row := make([]expressions.Expression, len(rowValues))
		for j, value := range rowValues {
			row[j] = asExpression(value)
			if i == 0 && j < len(columnNames) {
				row[j] = expressions.AliasExpr(row[j], columnNames[j], false)
			}
		}
		selects = append(selects, expressions.Select(expressions.Args{"expressions": row}))
	}

	if g.pretty && len(selects) > 0 {
		// Fold into an exp.Union tree so the pretty-printer can format the branches
		// (generator.py:2709-2714).
		query := selects[0]
		for _, sel := range selects[1:] {
			query = expressions.Union(expressions.Args{"this": query, "expression": sel, "distinct": false})
		}
		sub := expressions.Args{"this": query}
		if aliasNode != nil {
			sub["alias"] = expressions.TableAlias(expressions.Args{"this": aliasNode.Arg("this")})
		}
		return g.subquerySQL(expressions.Subquery(sub))
	}

	// generator.py:2716-2718 non-pretty path: join the SELECT branches with UNION ALL.
	alias := ""
	if aliasNode != nil {
		alias = " AS " + g.sqlKey(aliasNode, "this")
	}
	unions := make([]string, len(selects))
	for i, sel := range selects {
		unions[i] = g.gen(sel)
	}
	return "(" + strings.Join(unions, " UNION ALL ") + ")" + alias
}

func (g *Generator) unnestSQL(e expressions.Expression) string {
	args := g.expressions(exprsOptions{expression: e, flat: true})
	aliasExpr := asExpression(e.Arg("alias"))
	offset := e.Arg("offset")

	// Base UNNEST_WITH_ORDINALITY = true: fold a WITH OFFSET AS <col> (offset is an
	// Expression) into the alias column list and clear the offset (generator.py:3475-3484).
	if g.dialect.UnnestWithOrdinality {
		if offsetExpr := asExpression(offset); aliasExpr != nil && offsetExpr != nil {
			aliasExpr.Append("columns", offsetExpr)
			// Clear only the ARG, not the local: upstream keeps `offset` truthy so WITH
			// ORDINALITY still emits after folding WITH OFFSET AS <col> into the alias
			// (generator.py:3475-3484 vs 3493-3494).
			e.Set("offset", nil)
		}
	}

	// Base UNNEST_COLUMN_ONLY = false, so the else branch is taken for base.
	var alias string
	if aliasExpr != nil && g.dialect.UnnestColumnOnly {
		if columns := listFromValue(aliasExpr.Arg("columns")); len(columns) > 0 {
			alias = g.gen(columns[0])
		}
	} else {
		alias = g.gen(aliasExpr)
	}
	if alias != "" {
		alias = " AS " + alias
	}

	// WITH ORDINALITY is emitted whenever offset is set, independent of the alias
	// (generator.py:3493-3494). Without UNNEST_WITH_ORDINALITY (BigQuery) a WITH OFFSET keeps
	// its own AS <col> after the alias (generator.py:3495-3501).
	suffix := alias
	if g.dialect.UnnestWithOrdinality {
		if truthy(offset) {
			suffix = " WITH ORDINALITY" + alias
		}
	} else if offsetExpr := asExpression(offset); offsetExpr != nil {
		suffix = alias + " WITH OFFSET AS " + g.gen(offsetExpr)
	} else if truthy(offset) {
		suffix = alias + " WITH OFFSET"
	}
	return "UNNEST(" + args + ")" + suffix
}

// bracketIndexValue reads an integer index, including the Neg literal form the parser and
// LiteralNumber use for a negative value.
func bracketIndexValue(index expressions.Expression) (int64, bool) {
	if index == nil {
		return 0, false
	}
	if index.Kind() == expressions.KindNeg {
		inner := index.This()
		if inner == nil || inner.Kind() != expressions.KindLiteral || !inner.IsInt() {
			return 0, false
		}
		if n, ok := inner.ToPy().(int64); ok {
			return -n, true
		}
		return 0, false
	}
	if index.Kind() != expressions.KindLiteral || !index.IsInt() {
		return 0, false
	}
	n, ok := index.ToPy().(int64)
	return n, ok
}

// isMapTypedExpression reports whether an expression is a cast to a MAP/OBJECT type, the case
// apply_index_offset (expressions/builders.py:1117-1121) excludes from index shifting.
func isMapTypedExpression(e expressions.Expression) bool {
	if e == nil || (e.Kind() != expressions.KindCast && e.Kind() != expressions.KindTryCast) {
		return false
	}
	to := asExpression(e.Arg("to"))
	if to == nil {
		return false
	}
	dtype, _ := to.Arg("this").(expressions.DType)
	return dtype == expressions.DTypeMap || dtype == expressions.DTypeObject
}

func (g *Generator) bracketSQL(e expressions.Expression) string {
	exprs := listFromValue(e.Arg("expressions"))
	// generator.py:3583-3599 bracket_offset_expressions: re-apply the write dialect's index
	// offset on top of the parser's normalized zero-based index.
	if !truthy(e.Arg("json_access")) {
		offset := g.dialect.IndexOffset
		switch v := e.Arg("offset").(type) {
		case int:
			offset -= v
		case int64:
			offset -= int(v)
		case float64:
			offset -= int(v)
		}
		// apply_index_offset only shifts arrays and unknown-typed values; a map access is keyed
		// by the map's own key space and must keep its literal (builders.py:1117-1121).
		if isMapTypedExpression(asExpression(e.This())) {
			offset = 0
		}
		if offset != 0 && len(exprs) == 1 {
			if value, ok := bracketIndexValue(asExpression(exprs[0])); ok {
				exprs[0] = expressions.LiteralNumber(fmt.Sprintf("%d", value+int64(offset)))
			}
		}
	}
	sqls := make([]string, 0, len(exprs))
	for _, expr := range exprs {
		sqls = append(sqls, g.gen(expr))
	}
	return g.sqlKey(e, "this") + "[" + strings.Join(sqls, ", ") + "]"
}

// inlineArraySQL ports inline_array_sql (dialects/dialect.py:1218-1219): the postgres
// bracket-notation array body, e.g. `[1, 2, 3]`, with the same dynamic pretty line-wrap as
// any other comma list. Consumed by arraySQL (generator/residual_tail.go); the postgres
// ARRAY[...] literal now parses to a dedicated exp.Array node (parser _parse_bracket's
// ARRAY_CONSTRUCTORS swap), so bracketSQL itself no longer special-cases it.
func (g *Generator) inlineArraySQL(e expressions.Expression) string {
	return "[" + g.expressions(exprsOptions{expression: e, dynamic: true, newLine: true, skipFirst: true, skipLast: true}) + "]"
}

// intervalSQL ports interval_sql (generator.py:3910-3930). unit_expression carries the
// full unit sub-expression (e.g. an IntervalSpan for "DAY TO SECOND") so it's read before
// TIME_PART_SINGULARS/pluralization collapses it down to a bare unit string.
func (g *Generator) intervalSQL(e expressions.Expression) string {
	unitExpression := e.Arg("unit")
	unit := ""
	if truthy(unitExpression) {
		if inner := asExpression(unitExpression); inner != nil && g.dialect.MapIntervalUnits {
			switch inner.Kind() {
			case expressions.KindVar, expressions.KindLiteral, expressions.KindColumn:
				name := strings.ToUpper(inner.Name())
				if mapped := g.dialect.DatePartMapping[name]; mapped != "" {
					unit = mapped
				}
			}
		}
		if unit == "" {
			unit = g.gen(unitExpression)
		}
	}
	if !g.intervalAllowsPluralForm {
		if singular, ok := timePartSingulars[unit]; ok {
			unit = singular
		}
	}
	if unit != "" {
		unit = " " + unit
	}

	// generator.py:3956-3967: ClickHouse renders a REFRESH schedule interval without the
	// INTERVAL keyword and without quoting a string literal value.
	if g.dialect.AutoRefreshBareIntervals {
		if ancestor := e.FindAncestor(expressions.KindAutoRefreshProperty, expressions.KindSelect); ancestor != nil && ancestor.Kind() == expressions.KindAutoRefreshProperty {
			this := ""
			if thisExpr := asExpression(e.Arg("this")); thisExpr != nil {
				this = thisExpr.Name()
			}
			return strings.TrimSpace(this + unit)
		}
	}

	if g.singleStringInterval {
		this := ""
		if thisExpr := asExpression(e.Arg("this")); thisExpr != nil {
			this = thisExpr.Name()
		}
		if this != "" {
			if unitExprAsExpression := asExpression(unitExpression); unitExprAsExpression != nil && unitExprAsExpression.Kind() == expressions.KindIntervalSpan {
				return "INTERVAL '" + this + "'" + unit
			}
			return "INTERVAL '" + this + unit + "'"
		}
		return "INTERVAL" + unit
	}

	this := g.sqlKey(e, "this")
	if this != "" {
		thisExpr := asExpression(e.Arg("this"))
		unwrapped := thisExpr != nil && (thisExpr.Kind() == expressions.KindColumn || thisExpr.Kind() == expressions.KindLiteral || thisExpr.Kind() == expressions.KindNeg || thisExpr.Kind() == expressions.KindParen)
		if g.dialect.IntervalUnwrappedLiteralsOnly && thisExpr != nil {
			unwrapped = thisExpr.Kind() == expressions.KindLiteral || thisExpr.Kind() == expressions.KindParen
		}
		if unwrapped {
			this = " " + this
		} else {
			this = " (" + this + ")"
		}
	}
	return "INTERVAL" + this + unit
}

func (g *Generator) tableParts(e expressions.Expression) string {
	// generator.py table_parts override (bigquery): a dotted reference parsed as one quoted
	// path keeps the whole path quoted (`proj.dataset.table`).
	if truthy(e.MetaGet("quoted_table")) {
		parts := e.Parts()
		names := make([]string, 0, len(parts))
		for _, part := range parts {
			names = append(names, part.Name())
		}
		return g.gen(expressions.Identifier(expressions.Args{"this": strings.Join(names, "."), "quoted": true}))
	}
	parts := []string{}
	for _, key := range []string{"catalog", "schema", "this"} {
		if v := e.Arg(key); v != nil {
			parts = append(parts, g.gen(v))
		}
	}
	return strings.Join(parts, ".")
}

func (g *Generator) tableColumnSQL(e expressions.Expression) string { return g.sqlKey(e, "this") }

func (g *Generator) tableSQL(e expressions.Expression) string { return g.tableSQLSep(e, " AS ") }

// tableSQLSep is tableSQL with an explicit alias separator (generators/doris.py:614-619).
func (g *Generator) tableSQLSep(e expressions.Expression, sep string) string {
	table := g.tableParts(e)
	only := ""
	if boolValue(e.Arg("only")) {
		only = "ONLY "
	}
	partition := g.sqlKey(e, "partition")
	if partition != "" {
		partition = " " + partition
	}
	version := g.sqlKey(e, "version")
	if version != "" {
		version = " " + version
	}
	alias := g.sqlKey(e, "alias")
	if alias != "" {
		alias = sep + alias
	}
	sample := g.sqlKey(e, "sample")
	postAlias := ""
	preAlias := ""
	if g.dialect.AliasPostTablesample {
		preAlias = sample
	} else {
		postAlias = sample
	}
	if g.dialect.AliasPostVersion {
		preAlias += version
	} else {
		postAlias += version
	}
	hints := g.expressions(exprsOptions{expression: e, key: "hints", sep: " "})
	if hints != "" {
		hints = " " + hints
	}
	pivots := g.expressions(exprsOptions{expression: e, key: "pivots", emptySep: true, flat: true})
	joins := g.indent(g.expressions(exprsOptions{expression: e, key: "joins", emptySep: true, flat: true}), 0, nil, true, false)
	laterals := g.expressions(exprsOptions{expression: e, key: "laterals", emptySep: true})
	fileFormat := g.sqlKey(e, "format")
	pattern := g.sqlKey(e, "pattern")
	if fileFormat != "" {
		if pattern != "" {
			pattern = ", PATTERN => " + pattern
		}
		fileFormat = " (FILE_FORMAT => " + fileFormat + pattern + ")"
	} else if pattern != "" {
		fileFormat = " (PATTERN => " + pattern + ")"
	}
	ordinality := ""
	if truthy(e.Arg("ordinality")) {
		ordinality = " WITH ORDINALITY" + alias
		alias = ""
	}
	when := g.sqlKey(e, "when")
	if when != "" {
		// generators/duckdb.py:1587 HISTORICAL_DATA_POST_ALIAS = True: the AT/VERSION clause
		// follows the table alias.
		if g.isDialect("duckdb") {
			// A duckdb-source AT (VERSION => n) parses to PropertyEQ; Snowflake's form is a
			// HistoricalData node. Both move after the alias.
			if historical := asExpression(e.Arg("when")); historical != nil &&
				historical.Kind() == expressions.KindPropertyEQ {
				when = "AT (" + g.sqlKey(historical, "this") + " => " +
					g.sqlKey(historical, "expression") + ")"
			}
			postAlias += " " + when
		} else {
			table = table + " " + when
		}
	}
	changes := g.sqlKey(e, "changes")
	if changes != "" {
		changes = " " + changes
	}
	rowsFrom := g.expressions(exprsOptions{expression: e, key: "rows_from"})
	if rowsFrom != "" {
		table = "ROWS FROM " + g.wrap(rowsFrom)
	}
	indexed := ""
	if v := e.Arg("indexed"); v != nil {
		if b, ok := v.(bool); ok && !b {
			indexed = " NOT INDEXED"
		} else {
			indexed = " INDEXED BY " + g.gen(v)
		}
	}
	return only + table + changes + partition + fileFormat + preAlias + alias + indexed + hints + pivots + postAlias + joins + laterals + ordinality
}

func (g *Generator) tableAliasSQL(e expressions.Expression) string {
	alias := g.sqlKey(e, "this")
	columns := g.expressions(exprsOptions{expression: e, key: "columns", flat: true})
	if columns != "" {
		keepNamedCTEColumns := g.dialect.SupportsNamedCTEColumns && e.Parent() != nil && e.Parent().Kind() == expressions.KindCTE
		if !g.dialect.SupportsTableAliasColumns && !keepNamedCTEColumns {
			g.unsupported("Named columns are not supported in table alias.")
			columns = ""
		} else {
			columns = "(" + columns + ")"
		}
	}
	if alias == "" && !g.dialect.UnnestColumnOnly {
		alias = g.nextNameSQL()
	}
	return alias + columns
}

func (g *Generator) tupleSQL(e expressions.Expression) string {
	return "(" + g.expressions(exprsOptions{expression: e, dynamic: true, newLine: true, skipFirst: true, skipLast: true}) + ")"
}

func (g *Generator) blockSQL(e expressions.Expression) string {
	stmts := e.Expressions()
	// A routine-body block (trailing EndStatement) renders as MySQL's stmt_list, each
	// statement `;`-terminated; other blocks (a single-statement function body) join plainly.
	if n := len(stmts); n > 0 && stmts[n-1] != nil && stmts[n-1].Kind() == expressions.KindEndStatement {
		return g.compoundBodySQL(stmts)
	}
	return g.expressions(exprsOptions{expression: e, sep: "; ", flat: true})
}

// heredoc_sql (generator.py:1510-1512).
func (g *Generator) heredocSQL(e expressions.Expression) string {
	tag := g.sqlKey(e, "tag")
	return "$" + tag + "$" + g.sqlKey(e, "this") + "$" + tag + "$"
}

// semicolon_sql (generator.py:5421-5422): the node is only a comment carrier.
func (g *Generator) semicolonSQL(expressions.Expression) string {
	return ""
}

// exp.EndStatement: lambda *_: "END" (generator.py:183).
func (g *Generator) endStatementSQL(expressions.Expression) string {
	return "END"
}

// whileblock_sql (generator.py:6208-6210): base has no WHILE rendering (T-SQL overrides
// it). mysql renders the compound-grammar form `WHILE cond DO stmts; END WHILE [label]`.
func (g *Generator) whileBlockSQL(e expressions.Expression) string {
	if g.isDialect("mysql") {
		return g.labelWrap(e, "WHILE "+g.sqlKey(e, "this")+" DO "+g.bodyKeySQL(e, "body")+" END WHILE")
	}
	g.unsupported("Unsupported While block syntax")
	return ""
}

func (g *Generator) hintSQL(e expressions.Expression) string {
	// QUERY_HINT_SEP: mysql joins hint items with a space (generators/mysql.py:138),
	// base/postgres with ", " (generator.py:368).
	sep := ", "
	if g.isDialect("mysql") {
		sep = " "
	}
	return " /*+ " + strings.TrimSpace(g.expressions(exprsOptions{expression: e, sep: sep})) + " */"
}

// joinHintSQL ports joinhint_sql (generator.py:4816-4819): `<NAME>(<flat exprs>)`, e.g.
// MERGE(t1, t2) inside an optimizer hint.
func (g *Generator) joinHintSQL(e expressions.Expression) string {
	return g.sqlKey(e, "this") + "(" + g.expressions(exprsOptions{expression: e, flat: true}) + ")"
}

// queryTransformSQL ports querytransform_sql (generator.py:4959-4972): Hive's
// `TRANSFORM(<exprs>) USING '<script>' [AS schema]`, with the optional ROW FORMAT /
// RECORDWRITER / RECORDREADER fragments around it.
func (g *Generator) queryTransformSQL(e expressions.Expression) string {
	args := make([]any, 0, len(e.Expressions()))
	for _, x := range e.Expressions() {
		args = append(args, x)
	}
	var b strings.Builder
	b.WriteString(g.funcCall("TRANSFORM", args, "(", ")", true))
	for _, part := range []struct{ key, prefix string }{
		{"row_format_before", " "},
		{"record_writer", " RECORDWRITER "},
		{"command_script", " USING "},
		{"schema", " AS "},
		{"row_format_after", " "},
		{"record_reader", " RECORDREADER "},
	} {
		if value := g.sqlKey(e, part.key); value != "" {
			b.WriteString(part.prefix + value)
		}
	}
	return b.String()
}

// convertTimezoneSQL ports converttimezone_sql (generator.py:5600-5617): dialects that
// support CONVERT_TIMEZONE render the function, the rest rewrite it as AT TIME ZONE chains.
func (g *Generator) convertTimezoneSQL(e expressions.Expression) string {
	if g.dialect.SupportsConvertTimezone {
		return g.functionFallbackSQL(e)
	}
	sourceTZ := asExpression(e.Arg("source_tz"))
	targetTZ := asExpression(e.Arg("target_tz"))
	timestamp := asExpression(e.Arg("timestamp"))
	if sourceTZ != nil && timestamp != nil {
		timestamp = expressions.AtTimeZone(expressions.Args{
			"this": expressions.Cast(expressions.Args{
				"this": timestamp,
				"to":   expressions.DataType(expressions.Args{"this": expressions.DTypeTimestampNtz}),
			}),
			"zone": sourceTZ,
		})
	}
	return g.gen(expressions.AtTimeZone(expressions.Args{"this": timestamp, "zone": targetTZ}))
}

// toCharSQL ports the base tochar_sql (generator.py:4889-4890): CAST(this AS TEXT).
// Dialects that keep a TO_CHAR call override this through their dispatch.
func (g *Generator) toCharSQL(e expressions.Expression) string {
	return g.gen(expressions.Cast(expressions.Args{
		"this": e.This(),
		"to":   expressions.DataType(expressions.Args{"this": expressions.DTypeText}),
	}))
}

// nvl2SQL ports nvl2_sql (generator.py:5023-5036).
func (g *Generator) nvl2SQL(e expressions.Expression) string {
	if g.dialect.Nvl2Supported {
		return g.functionFallbackSQL(e)
	}
	condition := expressions.Not(expressions.Args{
		"this": expressions.Is(expressions.Args{"this": e.This(), "expression": expressions.Null()}),
	})
	args := expressions.Args{
		"ifs": []expressions.Expression{expressions.If(expressions.Args{
			"this": condition,
			"true": asExpression(e.Arg("true")),
		})},
	}
	if falseArg := asExpression(e.Arg("false")); falseArg != nil {
		args["default"] = falseArg
	}
	return g.gen(expressions.Case(args))
}

// queryBandSQL ports queryband_sql (generator.py:2975-2982).
func (g *Generator) queryBandSQL(e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	update := ""
	if boolValue(e.Arg("update")) {
		update = " UPDATE"
	}
	scope := ""
	if s := g.sqlKey(e, "scope"); s != "" {
		scope = " FOR " + s
	}
	return "QUERY_BAND = " + this + update + scope
}

// translateCharactersSQL ports translatecharacters_sql (generator.py:6137-6141).
func (g *Generator) translateCharactersSQL(e expressions.Expression) string {
	withError := ""
	if boolValue(e.Arg("with_error")) {
		withError = " WITH ERROR"
	}
	return "TRANSLATE(" + g.sqlKey(e, "this") + " USING " + g.sqlKey(e, "expression") + withError + ")"
}

// formatPhraseSQL ports formatphrase_sql (generator.py:3827-3832).
func (g *Generator) formatPhraseSQL(e expressions.Expression) string {
	return g.sqlKey(e, "this") + " (FORMAT " + g.sqlKey(e, "format") + ")"
}

// intDivSQL ports intdiv_sql (generator.py:4438-4444): CAST(this / expression AS INT).
func (g *Generator) intDivSQL(e expressions.Expression) string {
	return g.gen(expressions.Cast(expressions.Args{
		"this": expressions.Div(expressions.Args{"this": e.This(), "expression": e.Expr()}),
		"to":   expressions.DataType(expressions.Args{"this": expressions.DTypeInt}),
	}))
}
