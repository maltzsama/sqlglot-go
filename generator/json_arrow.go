package generator

import (
	"strings"

	"github.com/maltzsama/sqlglot-go/expressions"
)

// jsonExtractArgs returns the positional argument list for the *function* rendering of an
// exp.JSONExtract/JSONExtractScalar node: `this`, `expression`, then the variadic `expressions`
// tail. The tail is populated only for JSONExtract parsed from the 3+-arg function form
// (jsonExtractFunction, expressions/functions.go) - e.g. JSON_EXTRACT(a, '$.b', '$.c') - because
// upstream build_extract_json_with_path (parser.py:104-118) drops the tail for JSONExtractScalar.
// Arrow-created nodes and scalar nodes only set this/expression, so the tail is empty and this
// collapses to the two-operand list the operator branches also use.
func jsonExtractArgs(e expressions.Expression) []any {
	return append([]any{e.Arg("this"), e.Arg("expression")}, listFromValue(e.Arg("expressions"))...)
}

// This file ports JSON arrow rendering for exp.JSONExtract/JSONExtractScalar/JSONBExtract/
// JSONBExtractScalar. The parser builds JSONPath nodes for literal operands, consumed by
// pgJSONArrowSQL; dynamic operands remain expressions.

// pgJSONExtractSQL ports _json_extract_sql (generators/postgres.py:183-197): a path that is
// neither a JSONPath nor a Variadic, with no variadic tail, renders infix - spelling such a
// single dynamic operand as JSON_EXTRACT_PATH[_TEXT] would be jsonb-unsafe. Literal operator
// operands are parser-built JSONPath nodes and render infix with unquoted indices when
// only_json_types is set; other paths render through JSON_EXTRACT_PATH[_TEXT].
func (g *Generator) pgJSONExtractSQL(e expressions.Expression, name string, op string) string {
	path := asExpression(e.Arg("expression"))
	pathLike := path != nil && (path.Kind() == expressions.KindJSONPath || path.Kind() == expressions.KindVariadic)
	if !pathLike && len(listFromValue(e.Arg("expressions"))) == 0 {
		return g.binary(e, op)
	}
	if boolValue(e.Arg("only_json_types")) {
		return g.jsonExtractSegments(e, name, false, op)
	}
	return g.jsonExtractSegments(e, name, true, "")
}

// jsonExtractSegments ports json_extract_segments (dialects/dialect.py:2250-2277). A JSONPath
// path renders one quoted segment per path part - parts that render empty (postgres's
// JSONPathRoot and unsupported parts are skipped, while empty keys are retained - joined by
// the infix operator when one
// is given, or passed as the trailing arguments of name(...). A path that is not a JSONPath
// falls back to rename_func (dialects/dialect.py:1214-1215): the flattened positional args
// under name, which preserves e.g. VARIADIC array operands and non-literal path tails.
func (g *Generator) jsonExtractSegments(e expressions.Expression, name string, quotedIndex bool, op string) string {
	path := asExpression(e.Arg("expression"))
	if path == nil || path.Kind() != expressions.KindJSONPath {
		return g.funcCall(name, g.fallbackArgs(e), "(", ")", true)
	}

	segments := make([]string, 0, len(path.Expressions()))
	for _, segment := range path.Expressions() {
		part := g.gen(segment)
		if part == "" && segment.Kind() != expressions.KindJSONPathKey {
			continue
		}
		// Upstream additionally requires isinstance(segment, exp.JSONPathPart) before quoting;
		// every segment this port builds is one (jsonpath.go parse, dialects.BuildJSONExtractPath,
		// parser.go postgresArrowJSONPath), so the condition reduces to the subscript exemption
		// (quotedIndex=false, the operator form, leaves array indices unquoted).
		if quotedIndex || segment.Kind() != expressions.KindJSONPathSubscript {
			if segment.Kind() == expressions.KindJSONPathKey {
				part = g.escapeStr(segment.Name())
			}
			part = g.quoteStart + part + g.quoteEnd
		}
		segments = append(segments, part)
	}

	if op != "" {
		operands := make([]string, 0, len(segments)+1)
		operands = append(operands, g.sqlKey(e, "this"))
		operands = append(operands, segments...)
		return strings.Join(operands, " "+op+" ")
	}
	args := make([]any, 0, len(segments)+1)
	args = append(args, e.Arg("this"))
	for _, segment := range segments {
		args = append(args, segment)
	}
	return g.funcCall(name, args, "(", ")", true)
}

// jsonExtractSQL ports the base/mysql default (functionFallbackSQL - neither dialect overrides
// exp.JSONExtract in TRANSFORMS), postgres's _json_extract_sql("JSON_EXTRACT_PATH", "->")
// (generators/postgres.py:348), and Trino's JSON_QUERY-specific override
// (generators/trino.py:50-68). The Postgres branch stays first so its existing extraction
// spelling remains authoritative even if a caller supplies a JSONExtract with json_query set.
// Redshift never reaches this method: its own TRANSFORMS entries (dialect_redshift.go) render
// JSON_EXTRACT_PATH_TEXT directly, mirroring generators/redshift.py:87.
func (g *Generator) jsonExtractSQL(e expressions.Expression) string {
	if g.isDialect("postgres") {
		return g.pgJSONExtractSQL(e, "JSON_EXTRACT_PATH", "->")
	}
	if g.isDialect("trino") && boolValue(e.Arg("json_query")) {
		path := g.sqlKey(e, "expression")

		option := g.sqlKey(e, "option")
		if option != "" {
			option = " " + option
		}

		quote := g.sqlKey(e, "quote")
		if quote != "" {
			quote = " " + quote
		}

		onCondition := g.sqlKey(e, "on_condition")
		if onCondition != "" {
			onCondition = " " + onCondition
		}

		return g.funcCall("JSON_QUERY", []any{e.Arg("this"), path + option + quote + onCondition}, "(", ")", true)
	}
	return g.funcCall("JSON_EXTRACT", jsonExtractArgs(e), "(", ")", true)
}

// jsonExtractScalarSQL ports base's default (functionFallbackSQL), mysql's
// exp.JSONExtractScalar: arrow_json_extract_sql (generators/mysql.py:178, dialect.py:1210-1215),
// and postgres's _json_extract_sql("JSON_EXTRACT_PATH_TEXT", "->>") (generators/postgres.py:349)
// - postgres's own TRANSFORMS entry takes precedence over the generic JSON_TYPE_REQUIRED_FOR_
// EXTRACTION-gated arrow_json_extract_sql helper mysql shares, exactly mirroring upstream's
// per-dialect TRANSFORMS dict precedence. Redshift's own entry (dialect_redshift.go) wins
// before this method for that dialect, mirroring generators/redshift.py:88.
func (g *Generator) jsonExtractScalarSQL(e expressions.Expression) string {
	if g.isDialect("postgres") {
		return g.pgJSONExtractSQL(e, "JSON_EXTRACT_PATH_TEXT", "->>")
	}
	if g.dialect.JSONTypeRequiredForExtraction {
		return g.arrowJSONExtractScalarSQL(e)
	}
	return g.funcCall("JSON_EXTRACT_SCALAR", jsonExtractArgs(e), "(", ")", true)
}

// arrowJSONExtractScalarSQL ports arrow_json_extract_sql (dialect.py:1210-1215): a string-
// literal `this` is wrapped in CAST(... AS JSON) before rendering the `->>` operator form.
func (g *Generator) arrowJSONExtractScalarSQL(e expressions.Expression) string {
	// dialect.py:1250-1255: only dialects that require JSON-typed operands cast a string
	// literal; sqlite (like the base default) must not, since CAST(x AS JSON) is numeric there.
	if this := asExpression(e.Arg("this")); g.dialect.JSONTypeRequiredForExtraction && this != nil && this.Kind() == expressions.KindLiteral && this.IsString() {
		cast := expressions.Cast(expressions.Args{
			"this": this,
			"to":   expressions.DataType(expressions.Args{"this": expressions.DTypeJSON}),
		})
		e.Set("this", cast)
	}
	return g.binary(e, "->>")
}

// jsonbExtractSQL ports base/mysql's default (functionFallbackSQL - JSONB_EXTRACT) and
// postgres's `lambda self, e: self.binary(e, "#>")` (generators/postgres.py:324).
func (g *Generator) jsonbExtractSQL(e expressions.Expression) string {
	if g.isDialect("postgres") {
		return g.binary(e, "#>")
	}
	return g.funcCall("JSONB_EXTRACT", []any{e.Arg("this"), e.Arg("expression")}, "(", ")", true)
}

// jsonbExtractScalarSQL ports base/mysql's default (functionFallbackSQL - JSONB_EXTRACT_SCALAR)
// and postgres's `lambda self, e: self.binary(e, "#>>")` (generators/postgres.py:325).
func (g *Generator) jsonbExtractScalarSQL(e expressions.Expression) string {
	if g.isDialect("postgres") {
		return g.binary(e, "#>>")
	}
	return g.funcCall("JSONB_EXTRACT_SCALAR", []any{e.Arg("this"), e.Arg("expression")}, "(", ")", true)
}

func init() {
	dispatch[expressions.KindJSONExtract] = (*Generator).jsonExtractSQL
	dispatch[expressions.KindJSONExtractScalar] = (*Generator).jsonExtractScalarSQL
	dispatch[expressions.KindJSONBExtract] = (*Generator).jsonbExtractSQL
	dispatch[expressions.KindJSONBExtractScalar] = (*Generator).jsonbExtractScalarSQL
}
