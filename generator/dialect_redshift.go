package generator

import (
	"fmt"

	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// Redshift generator overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/generators/redshift.py. RedshiftGenerator extends
// PostgresGenerator, so everything not listed here falls back to the postgres behavior.
func init() {
	registerDialectDispatch("redshift", redshiftDispatch())
	// generators/redshift.py:47-56 TYPE_MAPPING delta.
	registerDialectTypeMapping("redshift", map[exp.DType]string{
		exp.DTypeBinary:      "VARBYTE",
		exp.DTypeBlob:        "VARBYTE",
		exp.DTypeInt:         "INTEGER",
		exp.DTypeTimeTz:      "TIME",
		exp.DTypeTimestampTz: "TIMESTAMP",
		exp.DTypeVarBinary:   "VARBYTE",
		exp.DTypeRowVersion:  "VARBYTE",
	})
}

func redshiftDispatch() map[exp.Kind]func(*Generator, exp.Expression) string {
	return map[exp.Kind]func(*Generator, exp.Expression) string{
		// generators/redshift.py:58-120 TRANSFORMS.
		exp.KindArrayConcat: arrayConcatSQL("ARRAY_CONCAT"),
		exp.KindConcat:      concatToDpipeSQL,
		exp.KindConcatWs:    concatWsToDpipeSQL,
		exp.KindApproxDistinct: func(g *Generator, e exp.Expression) string {
			return "APPROXIMATE COUNT(DISTINCT " + g.sqlKey(e, "this") + ")"
		},
		exp.KindCurrentTimestamp: func(g *Generator, e exp.Expression) string {
			if truthy(e.Arg("sysdate")) {
				return "SYSDATE"
			}
			return "GETDATE()"
		},
		exp.KindCurrentUserId: func(*Generator, exp.Expression) string { return "CURRENT_USER_ID" },
		exp.KindDateAdd:       dateDeltaSQL("DATEADD", false),
		exp.KindDateDiff:      dateDeltaSQL("DATEDIFF", false),
		exp.KindDistKeyProperty: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "DISTKEY", e.This())
		},
		exp.KindDistStyleProperty: func(g *Generator, e exp.Expression) string {
			return "DISTSTYLE " + g.sqlKey(e, "this")
		},
		exp.KindExplode:         redshiftExplode,
		exp.KindFarmFingerprint: clickhouseRename("FARMFINGERPRINT64"),
		exp.KindFromBase:        clickhouseRename("STRTOL"),
		exp.KindGeneratedAsIdentityColumnConstraint: func(g *Generator, e exp.Expression) string {
			// generatedasidentitycolumnconstraint_sql (dialects/dialect.py:1968-1973).
			start := g.sqlKey(e, "start")
			if start == "" {
				start = "1"
			}
			increment := g.sqlKey(e, "increment")
			if increment == "" {
				increment = "1"
			}
			return "IDENTITY(" + start + ", " + increment + ")"
		},
		exp.KindGroupConcat: clickhouseRename("LISTAGG"),
		exp.KindHex: func(g *Generator, e exp.Expression) string {
			return g.funcCall("UPPER", []any{g.funcCall("TO_HEX", []any{g.sqlKey(e, "this")}, "(", ")", true)}, "(", ")", true)
		},
		exp.KindRegexpExtract: clickhouseRename("REGEXP_SUBSTR"),
		exp.KindSelect:        redshiftSelect,
		exp.KindSortKeyProperty: func(g *Generator, e exp.Expression) string {
			compound := ""
			if truthy(e.Arg("compound")) {
				compound = "COMPOUND "
			}
			return compound + "SORTKEY(" + g.expressions(exprsOptions{expression: e, key: "this", flat: true}) + ")"
		},
		exp.KindStartsWith: func(g *Generator, e exp.Expression) string {
			return g.gen(e.This()) + " LIKE " + g.sqlKey(e, "expression") + " || '%'"
		},
		// generators/redshift.py:56-70: Redshift drops Postgres' exp.Getbit transform, so
		// the node falls through to the base functionFallbackSQL (GETBIT(...)) instead of
		// Postgres' getbit_sql (GET_BIT(...)).
		exp.KindGetbit:        (*Generator).functionFallbackSQL,
		exp.KindStringToArray: clickhouseRename("SPLIT_TO_ARRAY"),
		exp.KindTableSample: func(g *Generator, e exp.Expression) string {
			// no_tablesample_sql (dialects/dialect.py:1291-1293).
			g.unsupported("TABLESAMPLE unsupported")
			return g.sqlKey(e, "this")
		},
		exp.KindTsOrDsAdd:  dateDeltaSQL("DATEADD", false),
		exp.KindTsOrDsDiff: dateDeltaSQL("DATEDIFF", false),
		exp.KindUnixToTime: redshiftUnixToTime,
		exp.KindSHA2Digest: func(g *Generator, e exp.Expression) string {
			length := asExpression(e.Arg("length"))
			if length == nil {
				length = exp.LiteralNumber("256")
			}
			return clickhouseCall(g, "SHA2", e.This(), length)
		},

		// generators/redshift.py:271-372 methods.
		exp.KindStPoint: func(g *Generator, e exp.Expression) string {
			// ST_POINT only accepts 2 args in Redshift; ST_MAKEPOINT takes 3 or 4.
			if e.Arg("z") != nil || e.Arg("m") != nil {
				return clickhouseCall(g, "ST_MAKEPOINT", e.This(), asExpression(e.Arg("expression")), asExpression(e.Arg("z")), asExpression(e.Arg("m")))
			}
			return clickhouseCall(g, "ST_POINT", e.This(), asExpression(e.Arg("expression")))
		},
		exp.KindArrayContains: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "ARRAY_CONTAINS", e.This(), asExpression(e.Arg("expression")), asExpression(e.Arg("check_null")))
		},
		exp.KindObjectTransform: func(g *Generator, e exp.Expression) string {
			keep := g.expressions(exprsOptions{expression: e, key: "keep", flat: true})
			set := g.expressions(exprsOptions{expression: e, key: "set_", flat: true})
			sql := "OBJECT_TRANSFORM(" + g.sqlKey(e, "this")
			if keep != "" {
				sql += " KEEP " + keep
			}
			if set != "" {
				sql += " SET " + set
			}
			return sql + ")"
		},
		exp.KindApproxQuantile: func(g *Generator, e exp.Expression) string {
			return "APPROXIMATE " + g.gen(exp.WithinGroup(exp.Args{
				"this":       exp.PercentileDisc(exp.Args{"this": e.Arg("quantile")}),
				"expression": exp.Order(exp.Args{"expressions": []exp.Expression{exp.Ordered(exp.Args{"this": e.This()})}}),
			}))
		},
		exp.KindUnnest:   redshiftUnnest,
		exp.KindDataType: redshiftDataType,
		exp.KindCast:     redshiftCast,
		exp.KindAlterSet: redshiftAlterSet,
		exp.KindArray: func(g *Generator, e exp.Expression) string {
			if truthy(e.Arg("bracket_notation")) {
				return g.arraySQL(e)
			}
			return clickhouseRename("ARRAY")(g, e)
		},
		// Redshift restores the base IGNORE/RESPECT NULLS rendering that Postgres drops.
		exp.KindIgnoreNulls:  func(g *Generator, e exp.Expression) string { return g.embedIgnoreNulls(e, "IGNORE NULLS") },
		exp.KindRespectNulls: func(g *Generator, e exp.Expression) string { return g.embedIgnoreNulls(e, "RESPECT NULLS") },
	}
}

// redshiftSelect applies the generators/redshift.py:93-101 Select preprocess steps
// (eliminate_distinct_on and unnest_generate_date_array_using_recursive_cte; the window clause,
// semi/anti join and UNNEST qualification steps are not ported yet) before the base rendering.
func redshiftSelect(g *Generator, e exp.Expression) string {
	e = starRocksEliminateDistinctOn(g, e)
	starRocksUnnestGenerateDateArray(g, e)
	return g.selectSQL(e)
}

// redshiftExplode ports explode_sql (generators/redshift.py:368-370).
func redshiftExplode(g *Generator, _ exp.Expression) string {
	g.unsupported("Unsupported EXPLODE() function")
	return ""
}

// redshiftUnixToTime ports _unix_to_time_sql (generators/redshift.py:372-379).
func redshiftUnixToTime(g *Generator, e exp.Expression) string {
	this := g.gen(e.This())
	if scale := asExpression(e.Arg("scale")); scale != nil && scale.IsInt() && scale.Name() != "0" {
		this = "(" + this + " / POWER(10, " + scale.Name() + "))"
	}
	return "(TIMESTAMP 'epoch' + " + this + " * INTERVAL '1 SECOND')"
}

// redshiftUnnest ports unnest_sql (generators/redshift.py:305-322): Redshift's UNNEST is a
// single navigated SUPER expression in FROM/JOIN, aliased with its column names.
func redshiftUnnest(g *Generator, e exp.Expression) string {
	args := e.Expressions()
	if len(args) != 1 {
		g.unsupported(fmt.Sprintf("Unsupported number of arguments in UNNEST: %d", len(args)))
		return ""
	}
	if ancestor := e.FindAncestor(exp.KindFrom, exp.KindJoin, exp.KindSelect); ancestor != nil && ancestor.Kind() == exp.KindSelect {
		g.unsupported("Unsupported UNNEST when not used in FROM/JOIN clauses")
		return ""
	}
	arg := g.gen(args[0])
	alias := ""
	if a := asExpression(e.Arg("alias")); a != nil {
		alias = g.expressions(exprsOptions{expression: a, key: "columns", flat: true})
	}
	if alias != "" {
		return arg + " AS " + alias
	}
	return arg
}

// redshiftAlterSet ports alterset_sql (generators/redshift.py:354-362).
func redshiftAlterSet(g *Generator, e exp.Expression) string {
	sql := "SET"
	if exprs := g.expressions(exprsOptions{expression: e, flat: true}); exprs != "" {
		sql += " TABLE PROPERTIES (" + exprs + ")"
	}
	if location := g.sqlKey(e, "location"); location != "" {
		sql += " LOCATION " + location
	}
	if fileFormat := g.expressions(exprsOptions{expression: e, key: "file_format", flat: true, sep: " "}); fileFormat != "" {
		sql += " FILE FORMAT " + fileFormat
	}
	return sql
}

// redshiftDataType ports datatype_sql (generators/redshift.py:338-352) with
// TZ_TO_WITH_TIME_ZONE = True. Redshift turns TEXT into VARCHAR(255) by default, while TEXT
// usually means VARCHAR of max length, VARCHAR(MAX) in Redshift: TEXT without precision becomes
// VARCHAR(MAX), and TEXT with precision becomes VARCHAR.
func redshiftDataType(g *Generator, e exp.Expression) string {
	if exp.IsType(e, exp.DTypeText) {
		e.Set("this", exp.DTypeVarchar)
		if len(listFromValue(e.Arg("expressions"))) == 0 {
			e.Append("expressions", exp.Var(exp.Args{"this": "MAX"}))
		}
	}
	return tzToWithTimeZone(g, e)
}

// tzToWithTimeZone is datatype_sql with TZ_TO_WITH_TIME_ZONE = True (generator.py:1789-1793):
// TIMETZ and TIMESTAMPTZ render with a WITH TIME ZONE suffix.
func tzToWithTimeZone(g *Generator, e exp.Expression) string {
	sql := g.dataTypeSQL(e)
	if dt, ok := e.Arg("this").(exp.DType); ok && (dt == exp.DTypeTimestampTz || dt == exp.DTypeTimeTz) {
		return sql + " WITH TIME ZONE"
	}
	return sql
}

// redshiftCast ports cast_sql (generators/redshift.py:331-336): Redshift has no JSON type, so
// a cast to it is a no-op.
func redshiftCast(g *Generator, e exp.Expression) string {
	if to := asExpression(e.Arg("to")); to != nil && exp.IsType(to, exp.DTypeJSON) {
		return g.sqlKey(e, "this")
	}
	return g.castSQL(e)
}

// concatToDpipeSQL ports concat_to_dpipe_sql (dialects/dialect.py:1840-1841): CONCAT(a, b, c)
// renders as a || b || c.
func concatToDpipeSQL(g *Generator, e exp.Expression) string {
	exprs := e.Expressions()
	if len(exprs) == 0 {
		return ""
	}
	out := exprs[0]
	for _, y := range exprs[1:] {
		out = exp.DPipe(exp.Args{"this": out, "expression": y})
	}
	return g.gen(out)
}

// concatWsToDpipeSQL ports concat_ws_to_dpipe_sql (dialects/dialect.py:1844-1851):
// CONCAT_WS(d, a, b) renders as a || d || b.
func concatWsToDpipeSQL(g *Generator, e exp.Expression) string {
	exprs := e.Expressions()
	if len(exprs) < 2 {
		return ""
	}
	delim, rest := exprs[0], exprs[1:]
	out := rest[0]
	for _, y := range rest[1:] {
		out = exp.DPipe(exp.Args{"this": out, "expression": exp.DPipe(exp.Args{"this": delim, "expression": y})})
	}
	return g.gen(out)
}

// arrayConcatSQL ports array_concat_sql (dialects/dialect.py:1439-1516): ARRAY_CONCAT between
// dialects with different NULL propagation semantics.
func arrayConcatSQL(name string) func(*Generator, exp.Expression) string {
	buildCall := func(g *Generator, args []exp.Expression) string {
		if g.dialect.ArrayConcatIsVarLen {
			return clickhouseCall(g, name, args...)
		}
		if len(args) == 1 {
			// A single argument gets an empty array to keep the semantics.
			return clickhouseCall(g, name, args[0], exp.Array(exp.Args{"expressions": []exp.Expression{}}))
		}
		// Binary nesting, built right-deep: NAME(a, NAME(b, c)).
		result := clickhouseCall(g, name, args[len(args)-2], args[len(args)-1])
		for i := len(args) - 3; i >= 0; i-- {
			result = name + "(" + g.gen(args[i]) + ", " + result + ")"
		}
		return result
	}
	return func(g *Generator, e exp.Expression) string {
		this := e.This()
		exprs := e.Expressions()
		all := append([]exp.Expression{this}, exprs...)

		source := truthy(e.Arg("null_propagation"))
		target := g.dialect.ArrayFuncsPropagatesNulls
		// No wrapper when the NULL semantics match, when the first argument is an array literal
		// (never NULL), or for a single argument (the added empty array keeps NULL semantics).
		if source == target || (this != nil && this.Kind() == exp.KindArray) || len(exprs) == 0 {
			return buildCall(g, all)
		}
		if source {
			// The source propagates NULLs and the target doesn't: return NULL explicitly when
			// any argument is NULL.
			var check exp.Expression
			for _, arg := range all {
				isNull := exp.Is(exp.Args{"this": arg.Copy(), "expression": exp.Null()})
				if check == nil {
					check = isNull
				} else {
					check = exp.Or(exp.Args{"this": check, "expression": isNull})
				}
			}
			return g.gen(exp.If(exp.Args{"this": check, "true": exp.Null(), "false": buildCall(g, all)}))
		}
		// The target propagates NULLs and the source doesn't: COALESCE every argument to an
		// empty array.
		wrapped := make([]exp.Expression, len(all))
		for i, arg := range all {
			wrapped[i] = exp.Coalesce(exp.Args{"expressions": []exp.Expression{arg.Copy(), exp.Array(exp.Args{"expressions": []exp.Expression{}})}})
		}
		return buildCall(g, wrapped)
	}
}
