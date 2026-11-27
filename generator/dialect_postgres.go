package generator

import (
	"strconv"
	"strings"

	"github.com/maltzsama/sqlglot-go/dialects"
	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// Postgres generator overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/generators/postgres.py. The Postgres subclasses
// (Redshift, Materialize, RisingWave) inherit these via dialectParents, mirroring upstream's
// PostgresGenerator subclasses.
func init() {
	registerDialectDispatch("postgres", postgresDispatch())
	// generators/postgres.py:426 exp.VolatileProperty: UNSUPPORTED.
	registerDialectPropertyLocations("postgres", map[exp.Kind]propertyLocation{
		exp.KindVolatileProperty: propertyLocationUnsupported,
		// generators/postgres.py:425 TransientProperty is UNSUPPORTED.
		exp.KindTransientProperty: propertyLocationUnsupported,
	})
}

func postgresDispatch() map[exp.Kind]func(*Generator, exp.Expression) string {
	return map[exp.Kind]func(*Generator, exp.Expression) string{
		exp.KindStructExtract: structExtractSQL,
		exp.KindSelect: func(g *Generator, e exp.Expression) string {
			if asExpression(e.Arg("qualify")) != nil {
				e = eliminateQualify(e)
			}
			return g.selectSQL(e)
		},
		exp.KindJoin:        postgresJoinSQL,
		exp.KindArrayFilter: postgresArrayFilterSQL,
		exp.KindUnnest:      postgresUnnestSQL,
		exp.KindUnicode:     renameFunc("ASCII"),
		exp.KindSHA2Digest: func(g *Generator, e exp.Expression) string {
			length := e.Text("length")
			if length == "" {
				length = "256"
			}
			return g.funcCall("SHA"+length, []any{e.This()}, "(", ")", true)
		},
		exp.KindBracket: func(g *Generator, e exp.Expression) string {
			// generators/postgres.py:480-485: literals need parentheses before subscript.
			if base := e.This(); base != nil && base.Kind() == exp.KindArray {
				e.Set("this", exp.Paren(exp.Args{"this": base}))
			}
			return g.bracketSQL(e)
		},
		exp.KindJSONArrayAgg: func(g *Generator, e exp.Expression) string {
			return g.funcCall("JSON_AGG", []any{e.This()}, "(", g.sqlKey(e, "order")+")", true)
		},
		exp.KindDateDiff:      postgresDateDiffSQL,
		exp.KindTsOrDsDiff:    postgresDateDiffSQL,
		exp.KindTimestampDiff: postgresDateDiffSQL,
		exp.KindDatetimeDiff:  postgresDateDiffSQL,
		exp.KindCountIf: func(g *Generator, e exp.Expression) string {
			return "SUM(CASE WHEN " + g.sqlKey(e, "this") + " THEN 1 ELSE 0 END)"
		},
		exp.KindMedian: func(g *Generator, e exp.Expression) string {
			if g.dialect.Name == "redshift" {
				return g.functionFallbackSQL(e)
			}
			return "PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY " + g.sqlKey(e, "this") + ")"
		},
		exp.KindIsAscii: func(g *Generator, e exp.Expression) string {
			return "(" + g.sqlKey(e, "this") + " ~ '^[[:ascii:]]*$')"
		},
		exp.KindTimeToUnix: func(g *Generator, e exp.Expression) string {
			value := g.sqlKey(e, "this")
			dtype, ok := castResultDType(asExpression(e.This()))
			if ok && (dtype == exp.DTypeTimestampTz || dtype == exp.DTypeTimestampLtz || dtype == exp.DTypeTimestampNtz) {
				return "DATE_PART('epoch', " + value + ")"
			}
			if !ok || dtype != exp.DTypeTimestamp {
				value = "CAST(" + value + " AS TIMESTAMP)"
			}
			return "DATE_PART('epoch', " + value + ")"
		},
		exp.KindUuid: func(*Generator, exp.Expression) string { return "GEN_RANDOM_UUID()" },
		exp.KindArrayPrepend: func(g *Generator, e exp.Expression) string {
			return g.funcCall("ARRAY_PREPEND", []any{e.Arg("expression"), e.This()}, "(", ")", true)
		},
		exp.KindJSONBExtract: func(g *Generator, e exp.Expression) string {
			return g.sqlKey(e, "this") + " #> " + g.sqlKey(e, "expression")
		},
		exp.KindJSONBExtractScalar: func(g *Generator, e exp.Expression) string {
			return g.sqlKey(e, "this") + " #>> " + g.sqlKey(e, "expression")
		},
		exp.KindCurrentDate: func(g *Generator, e exp.Expression) string {
			if zone := asExpression(e.This()); zone != nil {
				return "CURRENT_DATE AT TIME ZONE " + g.sqlKey(e, "this")
			}
			return "CURRENT_DATE"
		},
		exp.KindLastDay: func(g *Generator, e exp.Expression) string {
			if g.dialect.Name == "redshift" {
				return g.functionFallbackSQL(e)
			}
			return postgresLastDaySQL(g, e)
		},
		exp.KindTimeFromParts: func(g *Generator, e exp.Expression) string {
			return g.funcCall("MAKE_TIME", g.fallbackArgs(e), "(", ")", true)
		},
		exp.KindBitwiseAndAgg: clickhouseRename("BIT_AND"),
		exp.KindBitwiseOrAgg:  clickhouseRename("BIT_OR"),
		exp.KindBitwiseXorAgg: clickhouseRename("BIT_XOR"),
		exp.KindRound:         postgresRoundSQL,
		exp.KindXor: func(g *Generator, e exp.Expression) string {
			group := func(v exp.Expression) string {
				sql := g.gen(v)
				if v != nil && (v.Is(exp.TraitConnector) || v.Is(exp.TraitBinary)) {
					return "(" + sql + ")"
				}
				return sql
			}
			a, b := group(e.This()), group(asExpression(e.Arg("expression")))
			return "(" + a + " AND (NOT " + b + ")) OR ((NOT " + a + ") AND " + b + ")"
		},
		exp.KindDateStrToDate: func(g *Generator, e exp.Expression) string {
			return "CAST(" + g.sqlKey(e, "this") + " AS DATE)"
		},
		exp.KindCurrentVersion: func(*Generator, exp.Expression) string { return "VERSION()" },
		exp.KindGenerateSeries: postgresGenerateSeriesSQL,
		exp.KindExplode:        renameFunc("UNNEST"),
		exp.KindDateAdd:        postgresIntervalAddSQL(true),
		exp.KindTsOrDsAdd:      postgresIntervalAddSQL(true),
		exp.KindDateSub:        postgresIntervalAddSQL(false),
		exp.KindRegexpReplace:  postgresRegexpReplaceSQL,
		exp.KindAnonymous: func(g *Generator, e exp.Expression) string {
			switch strings.ToUpper(e.Text("this")) {
			case "TO_NUMBER":
				if args := e.Expressions(); len(args) == 1 {
					return "CAST(" + g.gen(args[0]) + " AS DOUBLE PRECISION)"
				}
			case "FROM_UNIXTIME":
				if args := e.Expressions(); len(args) == 1 {
					return g.funcCall("TO_TIMESTAMP", []any{args[0]}, "(", ")", true)
				}
			case "JSONEXTRACTSTRING":
				args := []any{}
				for _, arg := range e.Expressions() {
					args = append(args, arg)
				}
				return g.funcCall("JSON_EXTRACT_PATH_TEXT", args, "(", ")", true)
			case "SPLIT_BY_STRING":
				return g.funcCall("STRING_TO_ARRAY", listToAny(e.Expressions()), "(", ")", true)
			case "JSONB_EXTRACT":
				if args := e.Expressions(); len(args) == 2 {
					return g.gen(args[0]) + " #> " + g.gen(args[1])
				}
			case "JSONB_EXTRACT_SCALAR":
				if args := e.Expressions(); len(args) == 2 {
					return g.gen(args[0]) + " #>> " + g.gen(args[1])
				}
			}
			return g.anonymousSQL(e)
		},
		// generators/postgres.py:524-527: a DECIMAL cast around DIV is superfluous.
		exp.KindCast: func(g *Generator, e exp.Expression) string {
			this := asExpression(e.This())
			to := asExpression(e.Arg("to"))
			if this != nil && this.Kind() == exp.KindIntDiv && to != nil && exp.IsType(to, exp.DTypeDecimal) && len(to.Expressions()) == 0 {
				return g.gen(this)
			}
			return g.castSQL(e)
		},
		exp.KindIntDiv: func(g *Generator, e exp.Expression) string {
			return g.funcCall("DIV", []any{e.This(), e.Arg("expression")}, "(", ")", true)
		},
		exp.KindArrayContains: func(g *Generator, e exp.Expression) string {
			value := asExpression(e.Arg("expression"))
			array := e.This()
			coalesce := exp.Coalesce(exp.Args{"this": exp.EQ(exp.Args{"this": value, "expression": exp.Any(exp.Args{"this": exp.Paren(exp.Args{"this": array})})}), "expressions": []exp.Expression{exp.Boolean(exp.Args{"this": false})}})
			ifs := []exp.Expression{
				exp.If(exp.Args{"this": exp.Is(exp.Args{"this": array, "expression": exp.Null()}), "true": exp.Null()}),
				exp.If(exp.Args{"this": exp.Is(exp.Args{"this": value, "expression": exp.Null()}), "true": exp.Null()}),
			}
			return g.gen(exp.Case(exp.Args{"ifs": ifs, "default": coalesce}))
		},
		exp.KindPercentileCont: percentileOrderedSQL("PERCENTILE_CONT"),
		exp.KindPercentileDisc: percentileOrderedSQL("PERCENTILE_DISC"),
		// generators/postgres.py:323 exp.BitwiseXor: binary(e, "#") — postgres renders a
		// bitwise xor with the `#` operator (parsed back via parsers/postgres.py:163-166).
		exp.KindBitwiseXor: func(g *Generator, e exp.Expression) string {
			return g.binary(e, "#")
		},
		// generators/postgres.py:317 exp.ArrayConcat: array_concat_sql("ARRAY_CAT"):
		// binary right-nested calls.
		exp.KindArrayConcat: arrayConcatSQL("ARRAY_CAT"),
		// generators/postgres.py:390 exp.StrPosition: strposition_sql(func_name="POSITION").
		exp.KindStrPosition: strPositionTranspileSQL("POSITION", false, false),
		// generators/postgres.py:397 exp.TimestampTrunc: timestamptrunc_sql(zone=True) —
		// DATE_TRUNC(unit, this[, zone]), with the unit rendered as a string literal via
		// unit_to_str (dialects/dialect.py:2059-2083).
		exp.KindTimestampTrunc: postgresTimestampTrunc,
		// generators/postgres.py:399 exp.TimeToStr: TO_CHAR(this, format_time(e)) — the format
		// arg's generated SQL converts back through the dialect's INVERSE_TIME_MAPPING with its
		// quotes intact (Generator.format_time, generator.py:4712-4721), emitted raw.
		exp.KindTimeToStr: postgresTimeToStr,
		// generators/postgres.py:392 exp.StrToTime: TO_TIMESTAMP(this, format_time(e)).
		exp.KindStrToTime: postgresStrToTime,
		exp.KindStrToDate: func(g *Generator, e exp.Expression) string {
			return postgresFormattedTime(g, e, "TO_DATE", e.This())
		},
		// generators/postgres.py:407/415 exp.UnixToTime — the TRANSFORMS dict lists the key
		// twice and Python keeps the last entry, so the scale-aware _unix_to_time_sql
		// (generators/postgres.py:201-212) is the effective transform, not the plain
		// TO_TIMESTAMP(this) lambda at line 407.
		exp.KindUnixToTime: postgresUnixToTime,
		// generators/postgres.py:325 exp.ColumnDef: preprocess([_auto_increment_to_serial,
		// _serial_to_generated]) — `id INT AUTO_INCREMENT` becomes `id SERIAL`, which renders
		// as `id INT GENERATED BY DEFAULT AS IDENTITY NOT NULL`.
		exp.KindColumnDef: postgresColumnDef,
		// generators/postgres.py:338 exp.Getbit: getbit_sql (dialects/dialect.py:2626-2644).
		exp.KindGetbit: getbitSQL,
		// generators/postgres.py:416 exp.Levenshtein: _levenshtein_sql.
		exp.KindLevenshtein: postgresLevenshteinSQL,
		// generators/postgres.py:369 exp.Rand: rename_func("RANDOM") — flatten the node's
		// args in declaration order (this, lower, upper), skipping absent ones the way
		// Generator.func skips a None arg, so e.g. a bounds-only Rand renders
		// RANDOM(lower, upper).
		exp.KindRand: func(g *Generator, e exp.Expression) string {
			return g.funcCall("RANDOM", g.fallbackArgs(e), "(", ")", true)
		},
		exp.KindToChar: func(g *Generator, e exp.Expression) string {
			return g.functionFallbackSQL(e)
		},
		// generators/postgres.py:354-356: JSONPath parts render name-only and the root
		// renders empty, so json_extract_segments' walk (generator/json_arrow.go) skips the
		// root and joins raw key names. Redshift, Materialize and RisingWave inherit these
		// entries through dialectParents, exactly like their TRANSFORMS chain. Note that
		// postgres.py:287-291 SUPPORTED_JSON_PATH_PARTS prunes every other JSONPathPart
		// kind from this dialect (dialect.py:296-305), which is why pgJSONPathSubscript
		// renders an unsupported part empty instead of dispatching it.
		exp.KindJSONPathRoot:      func(*Generator, exp.Expression) string { return "" },
		exp.KindJSONPathKey:       pgJSONPathKeyOnlyName,
		exp.KindJSONPathSubscript: pgJSONPathSubscript,
		exp.KindJSONPathRecursive: func(g *Generator, e exp.Expression) string {
			g.unsupported("Unsupported JSONPath part " + exp.ClassName(e.Kind()))
			return ""
		},
	}
}

func postgresLevenshteinSQL(g *Generator, e exp.Expression) string {
	name := "LEVENSHTEIN"
	if truthy(e.Arg("max_dist")) {
		name = "LEVENSHTEIN_LESS_EQUAL"
	}
	return g.funcCall(name, g.fallbackArgs(e), "(", ")", true)
}

// pgJSONPathKeyOnlyName ports json_path_key_only_name (dialects/dialect.py:2280-2284): the
// raw key name, with a wildcard key reported unsupported (it still renders empty, since a
// JSONPathWildcard node has no name).
func pgJSONPathKeyOnlyName(g *Generator, e exp.Expression) string {
	if this := asExpression(e.Arg("this")); this != nil && this.Kind() == exp.KindJSONPathWildcard {
		g.unsupported("Unsupported wildcard in JSONPathKey expression")
	}
	return e.Name()
}

// pgJSONPathSubscript ports generators/postgres.py:356
// (`lambda self, e: self.json_path_part(e.this)`): an integer or string index renders through
// json_path_part (generator/jsonpath.go jsonPathPartValue), a nested supported part
// re-dispatches, and any other part class - pruned from this dialect by
// SUPPORTED_JSON_PATH_PARTS (postgres.py:287-291, dialect.py:296-305) - is reported
// unsupported and renders empty, which makes json_extract_segments skip it.
func pgJSONPathSubscript(g *Generator, e exp.Expression) string {
	this := e.Arg("this")
	if node := asExpression(this); node != nil {
		switch node.Kind() {
		case exp.KindJSONPathRoot, exp.KindJSONPathKey, exp.KindJSONPathSubscript:
			return g.gen(node)
		default:
			g.unsupported("Unsupported JSONPathPart type " + exp.ClassName(node.Kind()))
			return ""
		}
	}
	return g.jsonPathPartValue(this)
}

// postgresColumnDef ports the exp.ColumnDef transform registered at
// generators/postgres.py:325: _auto_increment_to_serial (generators/postgres.py:136-153)
// followed by _serial_to_generated (generators/postgres.py:157-181). Upstream's preprocess
// helper re-dispatches the transformed node through the normal "<key>_sql" lookup after the
// transforms run, so this port calls columnDefSQL directly instead of re-entering gen.
// The mutations are safe: Generate copies the tree before generation (generator/generator.go:
// 193-195), so the caller's AST is untouched.
func postgresColumnDef(g *Generator, e exp.Expression) string {
	autoIncrementToSerial(e)
	serialToGenerated(e)
	return g.columnDefSQL(e)
}

// autoIncrementToSerial ports _auto_increment_to_serial (generators/postgres.py:136-153):
// drop the AUTO_INCREMENT constraint wrapper and promote the column's integer type to its
// SERIAL alias (INT -> SERIAL, SMALLINT -> SMALLSERIAL, BIGINT -> BIGSERIAL).
//
// Unlike upstream, a column with no type (`CREATE TABLE t (a AUTO_INCREMENT)`) is left
// untouched instead of raising AttributeError on kind.this: Python's crash there is an
// internal error on malformed input, not modeled behavior worth reproducing.
func autoIncrementToSerial(e exp.Expression) {
	auto := e.Find(exp.KindAutoIncrementColumnConstraint)
	if auto == nil {
		return
	}

	// upstream: expression.args["constraints"].remove(auto.parent) — list.remove drops the
	// first element equal to auto.parent (Expression.__eq__ is identity-or-HashKey).
	if wrapper := auto.Parent(); wrapper != nil {
		constraints := childList(e, "constraints")
		kept := make([]exp.Expression, 0, len(constraints))
		removed := false
		for _, c := range constraints {
			if !removed && c != nil && c.Equal(wrapper) {
				removed = true
				continue
			}
			kept = append(kept, c)
		}
		if removed {
			e.Set("constraints", kept)
		}
	}

	kind := asExpression(e.Arg("kind"))
	if kind == nil {
		return
	}
	var serial exp.DType
	switch {
	case exp.IsType(kind, exp.DTypeInt):
		serial = exp.DTypeSerial
	case exp.IsType(kind, exp.DTypeSmallInt):
		serial = exp.DTypeSmallSerial
	case exp.IsType(kind, exp.DTypeBigInt):
		serial = exp.DTypeBigSerial
	default:
		return
	}
	e.Set("kind", exp.DataType(exp.Args{"this": serial}))
}

// serialToGenerated ports _serial_to_generated (generators/postgres.py:157-181): rewrite a
// SERIAL column as its base integer type plus a GENERATED BY DEFAULT AS IDENTITY constraint
// and NOT NULL. Upstream inserts notnull at index 0 first and generated at index 0 second,
// so generated ends up ahead of notnull in the rendered constraints; the same order is kept
// here, with the same "insert only if absent" dedupe (list containment via Expression.__eq__).
func serialToGenerated(e exp.Expression) {
	kind := asExpression(e.Arg("kind"))
	if kind == nil {
		return
	}
	var base exp.DType
	switch {
	case exp.IsType(kind, exp.DTypeSerial):
		base = exp.DTypeInt
	case exp.IsType(kind, exp.DTypeSmallSerial):
		base = exp.DTypeSmallInt
	case exp.IsType(kind, exp.DTypeBigSerial):
		base = exp.DTypeBigInt
	default:
		return
	}
	e.Set("kind", exp.DataType(exp.Args{"this": base}))

	notnull := exp.ColumnConstraint(exp.Args{"kind": exp.NotNullColumnConstraint(exp.Args{})})
	generated := exp.ColumnConstraint(exp.Args{"kind": exp.GeneratedAsIdentityColumnConstraint(exp.Args{"this": false})})

	constraints := childList(e, "constraints")
	if !constraintPresent(constraints, notnull) {
		constraints = append([]exp.Expression{notnull}, constraints...)
	}
	if !constraintPresent(constraints, generated) {
		constraints = append([]exp.Expression{generated}, constraints...)
	}
	e.Set("constraints", constraints)
}

// constraintPresent mirrors Python's `x in list` for Expression elements: Expression.__eq__
// is identity-or-(type + HashKey), which Node.Equal implements.
func constraintPresent(constraints []exp.Expression, want exp.Expression) bool {
	for _, c := range constraints {
		if c != nil && c.Equal(want) {
			return true
		}
	}
	return false
}

// childList reads a child expression list (e.g. ColumnDef's "constraints") through the
// generic Expression interface — ExpressionsFor lives on *Node only.
func childList(e exp.Expression, key string) []exp.Expression {
	items := listFromValue(e.Arg(key))
	out := make([]exp.Expression, 0, len(items))
	for _, item := range items {
		if c, ok := item.(exp.Expression); ok && !isNilExpression(c) {
			out = append(out, c)
		}
	}
	return out
}

func postgresTimestampTrunc(g *Generator, e exp.Expression) string {
	args := []any{starRocksUnitToStr(g, e), e.This()}
	if zone := asExpression(e.Arg("zone")); zone != nil {
		args = append(args, zone)
	}
	return g.funcCall("DATE_TRUNC", args, "(", ")", true)
}

// postgresFormattedTime renders TO_CHAR/TO_TIMESTAMP as `NAME(this, format_time(e))`: the
// format arg's generated SQL converts back through the dialect's INVERSE_TIME_MAPPING with
// its quotes intact (Generator.format_time, generator.py:4712-4721) and is emitted raw; an
// absent or empty format stays nil, which funcCall skips like upstream's func() does with
// the None that format_time returns (time.py:27).
func postgresFormattedTime(g *Generator, e exp.Expression, name string, this any) string {
	var format any
	if f := asExpression(e.Arg("format")); f != nil {
		if sql := g.gen(f); sql != "" {
			converted, _ := dialects.FormatTime(sql, g.dialect.InverseTimeMapping)
			format = converted
		}
	}
	return g.funcCall(name, []any{this, format}, "(", ")", true)
}

func postgresTimeToStr(g *Generator, e exp.Expression) string {
	return postgresFormattedTime(g, e, "TO_CHAR", e.This())
}

// postgresStrToTime ports generators/postgres.py:392.
func postgresStrToTime(g *Generator, e exp.Expression) string {
	return postgresFormattedTime(g, e, "TO_TIMESTAMP", e.This())
}

// postgresUnixToTime ports _unix_to_time_sql (generators/postgres.py:201-212): the epoch
// stands alone for a SECONDS scale (or none), any other scale divides it through
// POW(10, scale); the optional format arg converts like TO_CHAR's.
func postgresUnixToTime(g *Generator, e exp.Expression) string {
	scale := asExpression(e.Arg("scale"))
	if scale == nil || (scale.IsInt() && scale.Name() == "0") {
		return postgresFormattedTime(g, e, "TO_TIMESTAMP", e.This())
	}
	this := exp.Div(exp.Args{
		"this":       e.This(),
		"expression": exp.Pow(exp.Args{"this": exp.LiteralNumber("10"), "expression": scale}),
	})
	return postgresFormattedTime(g, e, "TO_TIMESTAMP", this)
}

// getbitIntegerTypes is the union of SIGNED_INTEGER_TYPES and UNSIGNED_INTEGER_TYPES
// (expressions/datatypes.py), built once for getbitSQL's is_type check. IntegerTypes is
// NOT used here: it additionally carries BIT, which upstream's union excludes.
var getbitIntegerTypes = func() []any {
	out := make([]any, 0, len(exp.SignedIntegerTypes)+len(exp.UnsignedIntegerTypes))
	for t := range exp.SignedIntegerTypes {
		out = append(out, t)
	}
	for t := range exp.UnsignedIntegerTypes {
		out = append(out, t)
	}
	return out
}()

// getbitSQL ports getbit_sql (dialects/dialect.py:2626-2644): when the zero index is the
// least-significant bit and the input annotates as an integer, transpile to the bitwise
// form `(value >> position) & 1`; otherwise emit GET_BIT(value, position).
func getbitSQL(g *Generator, e exp.Expression) string {
	value := e.This()
	position := asExpression(e.Arg("expression"))
	if !truthy(e.Arg("zero_is_msb")) && e.IsType(getbitIntegerTypes...) {
		shifted := exp.BitwiseRightShift(exp.Args{"this": value, "expression": position})
		masked := exp.BitwiseAnd(exp.Args{"this": shifted, "expression": exp.LiteralNumber(1)})
		return g.gen(masked)
	}
	return g.funcCall("GET_BIT", []any{value, position}, "(", ")", true)
}

// eliminateQualify ports transforms.eliminate_qualify (transforms.py:201-260) for the common
// window-function case.
func eliminateQualify(selectExpr exp.Expression) exp.Expression {
	qualify := asExpression(selectExpr.Arg("qualify"))
	if qualify == nil {
		return selectExpr
	}
	selectExpr = selectExpr.Copy()
	selectExpr.Set("qualify", nil)
	taken := map[string]bool{}
	for _, name := range selectExpr.NamedSelects() {
		taken[name] = true
	}
	type outerProjection struct {
		inner   string
		visible string
		quoted  bool
		star    bool
	}
	plan := make([]outerProjection, 0)
	projections := selectExpr.Expressions()
	seen := map[string]bool{}
	for i, projection := range projections {
		if projection == nil {
			continue
		}
		if projection.Kind() == exp.KindStar {
			plan = append(plan, outerProjection{star: true})
			continue
		}
		visible := projection.OutputName()
		quoted := false
		if projection.Kind() == exp.KindAlias {
			if ident := asExpression(projection.Arg("alias")); ident != nil {
				quoted = boolValue(ident.Arg("quoted"))
			}
		} else if ident := asExpression(projection.Arg("this")); ident != nil && ident.Kind() == exp.KindIdentifier {
			quoted = boolValue(ident.Arg("quoted"))
		}
		inner := visible
		needsAlias := projection.Kind() != exp.KindAlias && projection.Kind() != exp.KindColumn
		if inner == "" || seen[inner] {
			needsAlias = true
		}
		if needsAlias {
			inner = findNewName(taken, "_c")
			taken[inner] = true
			projections[i] = exp.AliasExpr(projection, exp.ToIdentifier(inner, false), false)
		}
		seen[inner] = true
		plan = append(plan, outerProjection{inner: inner, visible: visible, quoted: quoted})
	}
	selectExpr.Set("expressions", projections)

	filter := qualify.This()
	if filter == nil {
		filter = qualify
	}
	innerNames := map[string]bool{}
	for _, projection := range selectExpr.Expressions() {
		if name := projection.OutputName(); name != "" {
			innerNames[name] = true
		}
	}
	visibleToInner := map[string]string{}
	for _, item := range plan {
		if !item.star && item.visible != "" {
			visibleToInner[item.visible] = item.inner
		}
	}
	for _, column := range filter.FindAll(exp.KindColumn) {
		column.Set("table", nil)
		column.Set("schema", nil)
		column.Set("catalog", nil)
		if inner, ok := visibleToInner[column.Name()]; ok && inner != column.Name() {
			column.Set("this", exp.ToIdentifier(inner, false))
			continue
		}
		if !innerNames[column.Name()] {
			selectExpr.Set("expressions", append(selectExpr.Expressions(), column.Copy()))
			innerNames[column.Name()] = true
		}
	}
	for _, window := range filter.FindAll(exp.KindWindow) {
		alias := findNewName(taken, "_w")
		taken[alias] = true
		selectExpr.Set("expressions", append(selectExpr.Expressions(), exp.AliasExpr(window.Copy(), exp.ToIdentifier(alias, false), false)))
		window.Replace(exp.Column(exp.Args{"this": exp.ToIdentifier(alias, false)}))
	}

	outer := make([]exp.Expression, 0, len(plan))
	for _, item := range plan {
		if item.star {
			outer = append(outer, exp.Star(exp.Args{}))
			continue
		}
		column := exp.Column(exp.Args{"this": exp.ToIdentifier(item.inner, item.quoted && item.inner == item.visible)})
		if item.visible != "" && item.visible != item.inner {
			outer = append(outer, exp.AliasExpr(column, exp.ToIdentifier(item.visible, item.quoted), false))
		} else {
			outer = append(outer, column)
		}
	}

	outerArgs := exp.Args{
		"expressions": outer,
		"from_": exp.From(exp.Args{"this": exp.Subquery(exp.Args{
			"this":  selectExpr,
			"alias": exp.TableAlias(exp.Args{"this": exp.ToIdentifier("_t", false)}),
		})}),
		"where": exp.Where(exp.Args{"this": filter}),
	}
	for _, key := range []string{"distinct", "order", "limit", "offset"} {
		if value := selectExpr.Arg(key); value != nil {
			outerArgs[key] = value
			selectExpr.Set(key, nil)
		}
	}
	return exp.Select(outerArgs)
}

// postgresJoinSQL appends ON TRUE to a LATERAL join, which Postgres requires.
func postgresJoinSQL(g *Generator, e exp.Expression) string {
	this := asExpression(e.This())
	kind := strings.ToUpper(e.Text("kind"))
	applied := this != nil && (this.Arg("cross_apply") != nil || this.Arg("outer_apply") != nil)
	sql := g.joinSQL(e)
	if this != nil && this.Kind() == exp.KindLateral && e.Arg("on") == nil && len(listFromValue(e.Arg("using"))) == 0 &&
		(applied || (kind != "" && kind != "CROSS")) {
		sql += " ON TRUE"
	}
	return sql
}

// postgresDateDiffSQL ports the Postgres date-difference formulas.
func postgresDateDiffSQL(g *Generator, e exp.Expression) string {
	unitExpr := asExpression(e.Arg("unit"))
	if unitExpr == nil {
		return "(CAST(" + g.sqlKey(e, "this") + " AS DATE) - CAST(" + g.sqlKey(e, "expression") + " AS DATE))"
	}
	unit := strings.ToUpper(unitExpr.Name())
	end := "CAST(" + g.sqlKey(e, "this") + " AS TIMESTAMP)"
	start := "CAST(" + g.sqlKey(e, "expression") + " AS TIMESTAMP)"
	epoch := "EXTRACT(epoch FROM " + end + " - " + start + ")"
	switch unit {
	case "DAY":
		return "CAST(" + epoch + " / 86400 AS BIGINT)"
	case "HOUR":
		return "CAST(" + epoch + " / 3600 AS BIGINT)"
	case "MINUTE":
		return "CAST(" + epoch + " / 60 AS BIGINT)"
	case "SECOND":
		return "CAST(" + epoch + " AS BIGINT)"
	case "MILLISECOND":
		return "CAST(" + epoch + " * 1000 AS BIGINT)"
	case "MICROSECOND":
		return "CAST(" + epoch + " * 1000000 AS BIGINT)"
	case "WEEK":
		return "CAST(EXTRACT(days FROM (" + end + " - " + start + ")) / 7 AS BIGINT)"
	case "MONTH":
		age := "AGE(" + end + ", " + start + ")"
		return "CAST(EXTRACT(year FROM " + age + ") * 12 + EXTRACT(month FROM " + age + ") AS BIGINT)"
	case "QUARTER":
		age := "AGE(" + end + ", " + start + ")"
		return "CAST(EXTRACT(year FROM " + age + ") * 4 + EXTRACT(month FROM " + age + ") / 3 AS BIGINT)"
	case "YEAR":
		return "CAST(EXTRACT(year FROM AGE(" + end + ", " + start + ")) AS BIGINT)"
	}
	return g.functionFallbackSQL(e)
}

// postgresDateSubSQL renders a date subtraction as a binary interval operation.

// postgresLastDaySQL renders LAST_DAY/EOMONTH through DATE_TRUNC arithmetic.
func postgresLastDaySQL(g *Generator, e exp.Expression) string {
	base := e.This()
	offset := ""
	if dateAdd := asExpression(base); dateAdd != nil && dateAdd.Kind() == exp.KindDateAdd {
		base = dateAdd.This()
		amount := asExpression(dateAdd.Arg("expression"))
		unit := asExpression(dateAdd.Arg("unit"))
		unitName := "MONTH"
		if unit != nil {
			unitName = strings.ToUpper(unit.Name())
		}
		if amount != nil {
			if value, ok := numericLiteralName(amount); ok {
				offset = " + INTERVAL '" + value + " " + unitName + "'"
			} else {
				// generators/postgres.py:72 builds Interval(1, unit) * amount, so the interval
				// is the left operand, as the Python oracle renders it.
				interval := exp.Interval(exp.Args{"this": exp.LiteralNumber("1"), "unit": exp.Var(exp.Args{"this": unitName})})
				offset = " + " + g.gen(exp.Mul(exp.Args{"this": interval, "expression": amount}))
			}
		}
	}
	return "CAST(DATE_TRUNC('MONTH', " + g.gen(base) + offset + ") + INTERVAL '1 MONTH' - INTERVAL '1 DAY' AS DATE)"
}

// postgresRoundSQL wraps a floating operand in DECIMAL before ROUND, matching Postgres types.
func postgresRoundSQL(g *Generator, e exp.Expression) string {
	this := e.This()
	if e.Arg("decimals") != nil && this != nil {
		typeExpr := this.Type()
		if this.Kind() == exp.KindCast || this.Kind() == exp.KindTryCast {
			typeExpr = asExpression(this.Arg("to"))
		}
		if typeExpr != nil && exp.IsType(typeExpr, exp.DTypeDouble) {
			inner := exp.Cast(exp.Args{"this": this, "to": exp.DTypeDecimal.IntoExpr(nil)})
			return g.funcCall("ROUND", []any{inner, e.Arg("decimals")}, "(", ")", true)
		}
	}
	return g.funcCall("ROUND", []any{this, e.Arg("decimals")}, "(", ")", true)
}

// postgresIntervalAddSQL renders DateAdd/TsOrDsAdd (or the negated DateSub) as an interval
// binary operation, converting QUARTER to three months.
func postgresIntervalAddSQL(add bool) func(*Generator, exp.Expression) string {
	return func(g *Generator, e exp.Expression) string {
		this := e.This()
		if e.Kind() == exp.KindTsOrDsAdd {
			if returnType := asExpression(e.Arg("return_type")); returnType != nil {
				// dialect.py:1984-1994: truncate through TIMESTAMP for DATE results.
				if exp.IsType(returnType, exp.DTypeDate) {
					this = exp.Cast(exp.Args{"this": this, "to": exp.DTypeTimestamp.IntoExpr(nil)})
				}
				this = exp.Cast(exp.Args{"this": this, "to": returnType})
			}
		}
		thisSQL := g.gen(this)
		count := asExpression(e.Arg("expression"))
		unit := asExpression(e.Arg("unit"))
		op := "+"
		if !add {
			op = "-"
		}
		if count != nil && count.Kind() == exp.KindInterval {
			nestedUnit := asExpression(count.Arg("unit"))
			if nestedUnit == nil || nestedUnit.Kind() == exp.KindIntervalSpan {
				return thisSQL + " " + op + " " + g.gen(count)
			}
			unit, count = nestedUnit, count.This()
		}
		unitName := "DAY"
		if unit != nil {
			unitName = strings.ToUpper(unit.Name())
		}
		quarter := unitName == "QUARTER"
		if quarter {
			unitName = "MONTH"
		}
		if count == nil {
			return thisSQL
		}
		value, isNumeric := numericLiteralName(count)
		if count.IsString() {
			value = count.Name()
			_, err := strconv.ParseFloat(value, 64)
			isNumeric = err == nil
			if !isNumeric && !quarter {
				interval := exp.Interval(exp.Args{"this": count, "unit": unit})
				return thisSQL + " " + op + " " + g.gen(interval)
			}
		}
		if isNumeric {
			if quarter {
				if parsed, err := strconv.ParseFloat(value, 64); err == nil {
					value = strconv.FormatFloat(parsed*3, 'f', -1, 64)
				}
			}
			return thisSQL + " " + op + " INTERVAL '" + value + " " + unitName + "'"
		}
		if count.Is(exp.TraitBinary) {
			count = exp.Paren(exp.Args{"this": count})
		}
		value = "1"
		if quarter {
			value = "3"
		}
		return thisSQL + " " + op + " INTERVAL '" + value + " " + unitName + "' * " + g.gen(count)
	}
}

func postgresRegexpReplaceSQL(g *Generator, e exp.Expression) string {
	args := []any{e.This(), e.Arg("expression"), e.Arg("replacement")}
	position := e.Arg("position")
	occurrence := e.Arg("occurrence")
	parameters := asExpression(e.Arg("modifiers"))
	if parameters == nil {
		parameters = asExpression(e.Arg("parameters"))
	}
	if position != nil {
		args = append(args, position)
	}
	if occurrence != nil {
		if position == nil {
			args = append(args, exp.LiteralNumber("1"))
		}
		args = append(args, occurrence)
	}
	occurrenceExpr := asExpression(occurrence)
	replaceAll := (!truthy(e.Arg("single_replace")) &&
		(occurrenceExpr == nil || occurrenceExpr.IsInt() && occurrenceExpr.Name() == "0"))
	if replaceAll && (parameters == nil || parameters.IsString()) {
		modifiers := ""
		if parameters != nil {
			modifiers = parameters.Name()
		}
		if !strings.Contains(modifiers, "g") {
			modifiers += "g"
		}
		parameters = exp.LiteralString(modifiers)
	}
	if parameters != nil {
		args = append(args, parameters)
	}
	return g.funcCall("REGEXP_REPLACE", args, "(", ")", true)
}

func postgresGenerateSeriesSQL(g *Generator, e exp.Expression) string {
	start := e.Arg("start")
	end := asExpression(e.Arg("end"))
	endArg := any(end)
	if truthy(e.Arg("is_end_exclusive")) {
		adjust := exp.Sub(exp.Args{"this": end, "expression": exp.LiteralNumber("1")})
		if step := asExpression(e.Arg("step")); step != nil {
			if value, ok := numericLiteralName(step); ok {
				if parsed, err := strconv.ParseFloat(value, 64); err == nil && parsed < 0 {
					adjust = exp.Add(exp.Args{"this": end, "expression": exp.LiteralNumber("1")})
				}
			}
		}
		endArg = adjust
	}
	args := []any{start, endArg}
	if step := asExpression(e.Arg("step")); step != nil {
		if step.IsString() {
			if interval := postgresIntervalExpr(step.Name()); interval != nil {
				args = append(args, interval)
			} else {
				args = append(args, step)
			}
		} else {
			args = append(args, step)
		}
	}
	return g.funcCall("GENERATE_SERIES", args, "(", ")", true)
}

// postgresArrayFilterSQL ports filter_array_using_unnest (dialects/dialect.py:2287-2305).
func postgresArrayFilterSQL(g *Generator, e exp.Expression) string {
	condition := e.Expr()
	alias := exp.ToIdentifier("_u")
	switch {
	case condition != nil && condition.Kind() == exp.KindLambda && len(condition.Expressions()) == 1:
		alias, condition = condition.Expressions()[0], condition.This()
	case condition == nil || !condition.Is(exp.TraitPredicate):
		g.unsupported("Unsupported filter condition")
		return ""
	}
	unnest := exp.Unnest(exp.Args{
		"expressions": []exp.Expression{e.This()},
		"alias":       exp.TableAlias(exp.Args{"columns": []exp.Expression{alias.Copy()}}),
	})
	filtered := exp.Select(exp.Args{
		"expressions": []exp.Expression{alias},
		"from_":       exp.From(exp.Args{"this": unnest}),
		"where":       exp.Where(exp.Args{"this": condition}),
	})
	return g.gen(exp.Array(exp.Args{"expressions": []exp.Expression{filtered}}))
}

// postgresUnnestSQL ports the date-series and typed JSON cases in generators/postgres.py:449-478.
func postgresUnnestSQL(g *Generator, e exp.Expression) string {
	args := e.Expressions()
	if len(args) != 1 {
		return g.unnestSQL(e)
	}
	arg := args[0]
	if arg.Kind() == exp.KindGenerateDateArray {
		series := exp.GenerateSeries(exp.Args{"start": arg.Arg("start"), "end": arg.Arg("end"), "step": arg.Arg("step")})
		if parent := e.Parent(); parent != nil && (parent.Kind() == exp.KindFrom || parent.Kind() == exp.KindJoin) {
			value := exp.ToIdentifier("value")
			table := exp.Table(exp.Args{
				"this": series,
				"alias": exp.TableAlias(exp.Args{
					"this": exp.ToIdentifier("_t"), "columns": []exp.Expression{value.Copy()},
				}),
			})
			selectExpr := exp.Select(exp.Args{
				"expressions": []exp.Expression{exp.Cast(exp.Args{"this": value, "to": exp.DTypeDate.IntoExpr(nil)})},
				"from_":       exp.From(exp.Args{"this": table}),
			})
			alias := asExpression(e.Arg("alias"))
			if alias == nil {
				alias = exp.TableAlias(exp.Args{"this": exp.ToIdentifier("_unnested_generate_series")})
			}
			return g.gen(exp.Subquery(exp.Args{"this": selectExpr, "alias": alias}))
		}
		return g.gen(series)
	}

	// The generator cannot import the optimizer's type annotator. Honor existing
	// annotations and explicit casts, both of which supply the relevant nested type.
	argType := arg.Type()
	if arg.Kind() == exp.KindCast || arg.Kind() == exp.KindTryCast {
		argType = asExpression(arg.Arg("to"))
	}
	if argType != nil && exp.IsType(argType, exp.DTypeArray) {
		elementTypes := argType.Expressions()
		if len(elementTypes) == 1 && exp.IsType(elementTypes[0], exp.DTypeJSON) {
			for arg.Kind() == exp.KindCast || arg.Kind() == exp.KindTryCast {
				arg = arg.This()
			}
			argAsJSON := exp.Cast(exp.Args{"this": arg, "to": exp.DTypeJSON.IntoExpr(nil)})
			sql := g.funcCall("JSON_ARRAY_ELEMENTS", []any{argAsJSON}, "(", ")", true)
			if alias := g.sqlKey(e, "alias"); alias != "" {
				sql += " AS " + alias
			}
			if truthy(e.Arg("offset")) {
				g.unsupported("Unsupported JSON_ARRAY_ELEMENTS with offset")
			}
			return sql
		}
	}
	return g.unnestSQL(e)
}

func postgresIntervalExpr(text string) exp.Expression {
	trimmed := strings.TrimSpace(text)
	digits := ""
	for len(trimmed) > 0 && (trimmed[0] == '-' || trimmed[0] == '+' || trimmed[0] == '.' || (trimmed[0] >= '0' && trimmed[0] <= '9')) {
		digits += string(trimmed[0])
		trimmed = trimmed[1:]
	}
	unit := strings.ToUpper(strings.TrimSpace(trimmed))
	unit = strings.TrimSuffix(unit, "S")
	if digits == "" || unit == "" {
		return nil
	}
	return exp.Interval(exp.Args{"this": exp.LiteralString(digits), "unit": exp.Var(exp.Args{"this": unit})})
}

// numericLiteralName returns the signed numeric text of a Literal or Neg(Literal).
func numericLiteralName(e exp.Expression) (string, bool) {
	if e == nil {
		return "", false
	}
	if e.Kind() == exp.KindLiteral {
		return e.Name(), true
	}
	if e.Kind() == exp.KindNeg {
		if inner := e.This(); inner != nil && inner.Kind() == exp.KindLiteral {
			return "-" + inner.Name(), true
		}
	}
	return "", false
}
