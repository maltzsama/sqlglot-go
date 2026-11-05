package generator

import (
	"strconv"

	exp "github.com/ridi-oss/sqlglot-go/expressions"
)

// StarRocks generator overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/generators/starrocks.py.
func init() {
	registerDialectDispatch("starrocks", starRocksDispatch())
	registerDialectTypeMapping("starrocks", map[exp.DType]string{
		exp.DTypeInt128:      "LARGEINT",
		exp.DTypeText:        "STRING",
		exp.DTypeTimestamp:   "DATETIME",
		exp.DTypeTimestampTz: "DATETIME",
	})
	registerDialectPropertyLocations("starrocks", map[exp.Kind]propertyLocation{
		exp.KindPrimaryKey:            propertyLocationPostSchema,
		exp.KindUniqueKeyProperty:     propertyLocationPostSchema,
		exp.KindRollupProperty:        propertyLocationPostSchema,
		exp.KindPartitionedByProperty: propertyLocationPostSchema,
	})
}

func starRocksDispatch() map[exp.Kind]func(*Generator, exp.Expression) string {
	return map[exp.Kind]func(*Generator, exp.Expression) string{
		exp.KindArgMax:   clickhouseRename("MAX_BY"),
		exp.KindArgMin:   clickhouseRename("MIN_BY"),
		exp.KindArray:    clickhouseArray,
		exp.KindArrayAgg: clickhouseRename("ARRAY_AGG"),
		exp.KindArrayContainedBy: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "ARRAY_CONTAINS_ALL", asExpression(e.Arg("expression")), e.This())
		},
		exp.KindArrayContainsAll: clickhouseRename("ARRAY_CONTAINS_ALL"),
		exp.KindArrayFilter:      clickhouseRename("ARRAY_FILTER"),
		exp.KindArrayToString:    clickhouseRename("ARRAY_JOIN"),
		exp.KindApproxDistinct: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "APPROX_COUNT_DISTINCT", e.This())
		},
		exp.KindCurrentVersion: func(g *Generator, e exp.Expression) string { return "CURRENT_VERSION()" },
		exp.KindDateDiff: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "DATE_DIFF", starRocksUnitToStr(g, e), e.This(), asExpression(e.Arg("expression")))
		},
		exp.KindDelete:            starRocksDelete,
		exp.KindFlatten:           clickhouseRename("ARRAY_FLATTEN"),
		exp.KindJSONExtract:       starRocksArrowJSONExtract,
		exp.KindJSONExtractScalar: starRocksArrowJSONExtract,
		exp.KindMap:               starRocksVarMap,
		exp.KindVarMap:            starRocksVarMap,
		exp.KindProperty:          starRocksProperty,
		exp.KindRegexpLike:        clickhouseRename("REGEXP"),
		exp.KindSchemaCommentProperty: func(g *Generator, e exp.Expression) string {
			return "COMMENT " + g.sqlKey(e, "this")
		},
		exp.KindSqlSecurityProperty: func(g *Generator, e exp.Expression) string {
			return "SECURITY " + g.sqlKey(e, "this")
		},
		exp.KindStDistance: starRocksStDistance,
		exp.KindStrToUnix: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "UNIX_TIMESTAMP", e.This(), asExpression(e.Arg("format")))
		},
		exp.KindTimestampTrunc: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "DATE_TRUNC", starRocksUnitToStr(g, e), e.This())
		},
		exp.KindTimeStrToDate: clickhouseRename("TO_DATE"),
		exp.KindUnixToStr: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "FROM_UNIXTIME", e.This(), asExpression(e.Arg("format")))
		},
		exp.KindUnixToTime: clickhouseRename("FROM_UNIXTIME"),
		// Select preprocess (generators/starrocks.py:121-126) and DateSub.
		exp.KindSelect:  func(g *Generator, e exp.Expression) string { return starRocksSelect(g, e) },
		exp.KindDateSub: func(g *Generator, e exp.Expression) string { return g.mysqlDateAddSQL("SUB", e) },

		// StarRocks DDL nodes.
		exp.KindCreate:                          starRocksCreate,
		exp.KindPartitionedByProperty:           starRocksPartitionedBy,
		exp.KindClusterProperty:                 starRocksClusterProperty,
		exp.KindRefreshTriggerProperty:          starRocksRefreshTrigger,
		exp.KindUniqueKeyProperty:               starRocksUniqueKeyProperty,
		exp.KindRollupProperty:                  starRocksRollupProperty,
		exp.KindRollupIndex:                     starRocksRollupIndex,
		exp.KindPartitionByRangeProperty:        starRocksPartitionByRange,
		exp.KindPartitionByRangePropertyDynamic: starRocksPartitionByRangeDynamic,
		exp.KindDistributedByProperty:           starRocksDistributedBy,
	}
}

// starRocksUnitToStr ports weekstart_unit_to_str's unit_to_str half for the generator
// (dialects/dialect.py:2059-2083): a Var/Literal unit becomes a string literal.
func starRocksUnitToStr(g *Generator, e exp.Expression) exp.Expression {
	unit := asExpression(e.Arg("unit"))
	if unit == nil {
		return exp.LiteralString("DAY")
	}
	if unit.Kind() == exp.KindPlaceholder {
		return unit
	}
	if unit.Kind() != exp.KindVar && unit.Kind() != exp.KindLiteral {
		return unit
	}
	return exp.LiteralString(unit.Name())
}

// starRocksArrowJSONExtract ports arrow_json_extract_sql (dialects/dialect.py:1250-1255).
func starRocksArrowJSONExtract(g *Generator, e exp.Expression) string {
	op := "->"
	if e.Kind() == exp.KindJSONExtractScalar {
		op = "->>"
	}
	return g.sqlKey(e, "this") + " " + op + " " + g.sqlKey(e, "expression")
}

// starRocksVarMap ports var_map_sql with the MAP name (dialects/dialect.py:1521-1536).
func starRocksVarMap(g *Generator, e exp.Expression) string {
	keys := asExpression(e.Arg("keys"))
	values := asExpression(e.Arg("values"))
	if keys == nil || values == nil || keys.Kind() != exp.KindArray || values.Kind() != exp.KindArray {
		g.unsupported("Cannot convert array columns into map.")
		return clickhouseCall(g, "MAP", keys, values)
	}
	args := []any{}
	ks, vs := keys.Expressions(), values.Expressions()
	for i := 0; i < len(ks) && i < len(vs); i++ {
		args = append(args, ks[i], vs[i])
	}
	return g.funcCall("MAP", args, "(", ")", true)
}

// starRocksStDistance ports st_distance_sphere (generators/starrocks.py:44-53).
func starRocksStDistance(g *Generator, e exp.Expression) string {
	point1 := e.This()
	point2 := asExpression(e.Arg("expression"))
	return clickhouseCall(g, "ST_Distance_Sphere",
		exp.Anonymous(exp.Args{"this": "ST_X", "expressions": []exp.Expression{point1}}),
		exp.Anonymous(exp.Args{"this": "ST_Y", "expressions": []exp.Expression{point1}}),
		exp.Anonymous(exp.Args{"this": "ST_X", "expressions": []exp.Expression{point2}}),
		exp.Anonymous(exp.Args{"this": "ST_Y", "expressions": []exp.Expression{point2}}),
	)
}

// starRocksDelete ports the _eliminate_between_in_delete preprocessing
// (generators/starrocks.py:18-40).
func starRocksDelete(g *Generator, e exp.Expression) string {
	where := asExpression(e.Arg("where"))
	if where != nil {
		for _, between := range where.FindAll(exp.KindBetween) {
			low := asExpression(between.Arg("low"))
			high := asExpression(between.Arg("high"))
			this := asExpression(between.Arg("this"))
			if this == nil || low == nil || high == nil {
				continue
			}
			lowSQL := exp.GTE(exp.Args{"this": this.Copy(), "expression": low})
			highSQL := exp.LTE(exp.Args{"this": this.Copy(), "expression": high})
			between.Replace(exp.And(exp.Args{"this": lowSQL, "expression": highSQL}))
		}
	}
	return g.deleteSQL(e)
}

// starRocksCreate ports create_sql (generators/starrocks.py:295-314): the schema's
// PrimaryKey moves into the properties right after ENGINE before the base assembly.
func starRocksCreate(g *Generator, e exp.Expression) string {
	schema := asExpression(e.Arg("this"))
	if schema != nil && schema.Kind() == exp.KindSchema {
		if primaryKey := schema.Find(exp.KindPrimaryKey); primaryKey != nil {
			props := asExpression(e.Arg("properties"))
			if props == nil {
				props = g.propertiesExpression(nil, e)
				e.Set("properties", props)
			}
			exprs := props.Expressions()
			insertAt := 0
			for i, prop := range exprs {
				if prop.Kind() == exp.KindEngineProperty {
					insertAt = i + 1
					break
				}
			}
			popped := primaryKey.Pop()
			newProps := make([]exp.Expression, 0, len(exprs)+1)
			newProps = append(newProps, exprs[:insertAt]...)
			newProps = append(newProps, popped)
			newProps = append(newProps, exprs[insertAt:]...)
			props.Set("expressions", newProps)
		}
	}
	return g.createSQL(e)
}

// starRocksPartitionedBy ports partitionedbyproperty_sql (generators/starrocks.py:316-330).
func starRocksPartitionedBy(g *Generator, e exp.Expression) string {
	this := asExpression(e.Arg("this"))
	if this != nil && this.Kind() == exp.KindSchema {
		sql := g.expressions(exprsOptions{expression: this, flat: true})
		wrap := false
		if create := e.FindAncestor(exp.KindCreate); create != nil && create.Text("kind") == "VIEW" {
			wrap = true
		} else {
			wrap = true
			for _, item := range this.Expressions() {
				if item.Kind() != exp.KindColumn && item.Kind() != exp.KindIdentifier {
					wrap = false
					break
				}
			}
		}
		if wrap {
			sql = "(" + sql + ")"
		}
		return "PARTITION BY " + sql
	}
	return "PARTITION BY " + g.gen(this)
}

// starRocksClusterProperty ports clusterproperty_sql (generators/starrocks.py:332-338).
func starRocksClusterProperty(g *Generator, e exp.Expression) string {
	if e.This() != nil {
		g.unsupported("Unsupported CLUSTER BY " + g.sqlKey(e, "this"))
		return ""
	}
	return "ORDER BY (" + g.expressions(exprsOptions{expression: e, flat: true}) + ")"
}

// starRocksRefreshTrigger ports refreshtriggerproperty_sql
// (generators/starrocks.py:340-354).
func starRocksRefreshTrigger(g *Generator, e exp.Expression) string {
	method := g.sqlKey(e, "method")
	if method != "" {
		method = " " + method
	}
	kind := g.sqlKey(e, "kind")
	if kind != "" {
		kind = " " + kind
	}
	starts := g.sqlKey(e, "starts")
	if starts != "" {
		starts = " START (" + starts + ")"
	}
	every := ""
	if everySQL := g.sqlKey(e, "every"); everySQL != "" {
		if unit := g.sqlKey(e, "unit"); unit != "" {
			every = " EVERY (INTERVAL " + everySQL + " " + unit + ")"
		}
	}
	return "REFRESH" + method + kind + starts + every
}

// starRocksUniqueKeyProperty ports uniquekeyproperty_sql (generator.py:4925-4928).
func starRocksUniqueKeyProperty(g *Generator, e exp.Expression) string {
	return "UNIQUE KEY (" + g.expressions(exprsOptions{expression: e, flat: true}) + ")"
}

// starRocksRollupProperty ports rollupproperty_sql (generator.py:2787-2788).
func starRocksRollupProperty(g *Generator, e exp.Expression) string {
	return "ROLLUP (" + g.expressions(exprsOptions{expression: e, flat: true}) + ")"
}

// starRocksRollupIndex ports rollupindex_sql (generator.py:2772-2785).
func starRocksRollupIndex(g *Generator, e exp.Expression) string {
	this := g.sqlKey(e, "this")
	columns := g.expressions(exprsOptions{expression: e, flat: true})
	fromSQL := g.sqlKey(e, "from_index")
	if fromSQL != "" {
		fromSQL = " FROM " + fromSQL
	}
	properties := ""
	if props := asExpression(e.Arg("properties")); props != nil {
		properties = " " + g.withPropertiesSQL(props)
	}
	return this + "(" + columns + ")" + fromSQL + properties
}

// starRocksPartitionByRange ports partitionbyrangeproperty_sql (generator.py:5972-5975).
func starRocksPartitionByRange(g *Generator, e exp.Expression) string {
	partitions := g.expressions(exprsOptions{expression: e, key: "partition_expressions", flat: true})
	create := g.expressions(exprsOptions{expression: e, key: "create_expressions", flat: true})
	return "PARTITION BY RANGE (" + partitions + ") (" + create + ")"
}

// starRocksPartitionByRangeDynamic ports partitionbyrangepropertydynamic_sql
// (generator.py:5977-5987).
func starRocksPartitionByRangeDynamic(g *Generator, e exp.Expression) string {
	start := g.sqlKey(e, "start")
	end := g.sqlKey(e, "end")
	every := asExpression(e.Arg("every"))
	if every != nil && every.Kind() == exp.KindInterval {
		if this := asExpression(every.Arg("this")); this != nil && this.IsString() {
			every.Set("this", exp.LiteralNumber(every.Name()))
		}
	}
	return "START (" + start + ") END (" + end + ") EVERY (" + g.gen(every) + ")"
}

// starRocksDistributedBy ports distributedbyproperty_sql (generator.py:4930-4939).
func starRocksDistributedBy(g *Generator, e exp.Expression) string {
	expressionsSQL := g.expressions(exprsOptions{expression: e, flat: true})
	if expressionsSQL != "" {
		expressionsSQL = " (" + expressionsSQL + ")"
	}
	buckets := g.sqlKey(e, "buckets")
	if buckets != "" {
		buckets = " BUCKETS " + buckets
	}
	return "DISTRIBUTED BY " + g.sqlKey(e, "kind") + expressionsSQL + buckets + g.sqlKey(e, "order")
}

// starRocksCastIfNeeded ports exp.cast's no-re-cast rule (expressions/builders.py:487-507).
func starRocksCastIfNeeded(g *Generator, expression exp.Expression, dtype exp.DType) exp.Expression {
	if expression != nil && expression.Kind() == exp.KindCast {
		if to := asExpression(expression.Arg("to")); to != nil {
			if existing, ok := to.Arg("this").(exp.DType); ok {
				mapped := func(dt exp.DType) string {
					if m, found := g.lookupTypeMapping(dt); found {
						return m
					}
					return string(dt)
				}
				if existing == dtype || mapped(existing) == mapped(dtype) {
					return expression
				}
			}
		}
	}
	return exp.Cast(exp.Args{"this": expression, "to": dtype.IntoExpr(nil)})
}

// starRocksProperty ports dialect.py's property_sql helper
// (dialects/dialect.py:1317-1318): the key is always rendered as a string literal.
func starRocksProperty(g *Generator, e exp.Expression) string {
	return g.propertyName(e, true) + "=" + g.sqlKey(e, "value")
}

// starRocksSelect applies the generators/starrocks.py:121-126 Select preprocess steps before
// the base SELECT rendering.
func starRocksSelect(g *Generator, e exp.Expression) string {
	e = starRocksEliminateDistinctOn(g, e)
	starRocksUnnestGenerateDateArray(g, e)
	return g.selectSQL(e)
}

// starRocksEliminateDistinctOn ports transforms.eliminate_distinct_on
// (transforms.py:144-200).
func starRocksEliminateDistinctOn(g *Generator, expression exp.Expression) exp.Expression {
	if expression.Kind() != exp.KindSelect {
		return expression
	}
	distinct := asExpression(expression.Arg("distinct"))
	if distinct == nil {
		return expression
	}
	on := asExpression(distinct.Arg("on"))
	if on == nil || on.Kind() != exp.KindTuple {
		return expression
	}

	named := map[string]bool{}
	for _, projection := range expression.Expressions() {
		named[projection.Name()] = true
	}
	rowName := "_row_number"
	for i := 1; named[rowName]; i++ {
		rowName = "_row_number_" + strconv.Itoa(i)
	}

	distinctCols := on.Expressions()
	window := exp.Window(exp.Args{
		"this":         exp.RowNumber(nil),
		"partition_by": distinctCols,
	})
	if order := asExpression(expression.Arg("order")); order != nil {
		window.Set("order", order.Pop())
	} else {
		orderExprs := make([]exp.Expression, 0, len(distinctCols))
		for _, col := range distinctCols {
			orderExprs = append(orderExprs, col.Copy())
		}
		window.Set("order", exp.Order(exp.Args{"expressions": orderExprs}))
	}

	projections := expression.Expressions()
	newProjections := make([]exp.Expression, len(projections), len(projections)+1)
	copy(newProjections, projections)
	newProjections = append(newProjections, exp.AliasNode(exp.Args{
		"this":  window,
		"alias": exp.ToIdentifier(rowName),
	}))

	taken := map[string]bool{rowName: true}
	outerSelects := make([]exp.Expression, 0, len(projections))
	starBreak := false
	for i, projection := range projections {
		if projection.IsStar() {
			outerSelects = []exp.Expression{exp.Star(nil)}
			starBreak = true
			break
		}
		if projection.Kind() != exp.KindAlias {
			name := projection.Name()
			if name == "" {
				name = "_col"
			}
			for j := 1; taken[name]; j++ {
				name = "_col_" + strconv.Itoa(j)
			}
			aliasID := exp.ToIdentifier(name)
			newProjections[i] = exp.AliasNode(exp.Args{"this": projection, "alias": aliasID})
			taken[name] = true
			outerSelects = append(outerSelects, aliasID)
			continue
		}
		taken[projection.Name()] = true
		outerSelects = append(outerSelects, exp.ToIdentifier(projection.Name()))
	}
	_ = starBreak

	expression.Set("expressions", newProjections)
	expression.Set("distinct", nil)

	inner := exp.Subquery(exp.Args{
		"this":  expression,
		"alias": exp.TableAlias(exp.Args{"this": exp.ToIdentifier("_t")}),
	})
	outer := exp.Select(exp.Args{
		"expressions": outerSelects,
		"from_":       exp.From(exp.Args{"this": inner}),
		"where": exp.Where(exp.Args{"this": exp.EQ(exp.Args{
			"this":       exp.Column(exp.Args{"this": exp.ToIdentifier(rowName)}),
			"expression": exp.LiteralNumber("1"),
		})}),
	})
	return outer
}

// starRocksUnnestGenerateDateArray ports unnest_generate_date_array_using_recursive_cte
// (transforms.py:72-134).
func starRocksUnnestGenerateDateArray(g *Generator, expression exp.Expression) {
	if expression.Kind() != exp.KindSelect {
		return
	}
	count := 0
	var ctes []exp.Expression
	for _, unnest := range expression.FindAll(exp.KindUnnest) {
		parent := unnest.Parent()
		if parent == nil || (parent.Kind() != exp.KindFrom && parent.Kind() != exp.KindJoin) {
			continue
		}
		unnestExprs := unnest.Expressions()
		if len(unnestExprs) != 1 || unnestExprs[0].Kind() != exp.KindGenerateDateArray {
			continue
		}
		gda := unnestExprs[0]
		start := asExpression(gda.Arg("start"))
		end := asExpression(gda.Arg("end"))
		step := asExpression(gda.Arg("step"))
		if start == nil || end == nil || step == nil || step.Kind() != exp.KindInterval {
			continue
		}
		columnName := "date_value"
		if alias := asExpression(unnest.Arg("alias")); alias != nil {
			if cols := listFromValue(alias.Arg("columns")); len(cols) > 0 {
				columnName = asExpression(cols[0]).Name()
			}
		}

		startCast := starRocksCastIfNeeded(g, start, exp.DTypeDate)
		dateAdd := exp.DateAdd(exp.Args{
			"this":       exp.Column(exp.Args{"this": exp.ToIdentifier(columnName)}),
			"expression": exp.LiteralNumber(step.Name()),
			"unit":       step.Arg("unit"),
		})
		castDateAdd := exp.Cast(exp.Args{"this": dateAdd, "to": exp.DTypeDate.IntoExpr(nil)})
		castDateAddWhere := exp.Cast(exp.Args{
			"this": exp.DateAdd(exp.Args{
				"this":       exp.Column(exp.Args{"this": exp.ToIdentifier(columnName)}),
				"expression": exp.LiteralNumber(step.Name()),
				"unit":       step.Arg("unit"),
			}),
			"to": exp.DTypeDate.IntoExpr(nil),
		})

		cteName := "_generated_dates"
		if count > 0 {
			cteName += "_" + strconv.Itoa(count)
		}

		baseQuery := exp.Select(exp.Args{"expressions": []exp.Expression{
			exp.AliasNode(exp.Args{"this": startCast, "alias": exp.ToIdentifier(columnName)}),
		}})
		recursiveQuery := exp.Select(exp.Args{
			"expressions": []exp.Expression{castDateAdd},
			"from_": exp.From(exp.Args{"this": exp.Table(exp.Args{
				"this": exp.ToIdentifier(cteName),
			})}),
			"where": exp.Where(exp.Args{"this": exp.LTE(exp.Args{
				"this":       castDateAddWhere,
				"expression": starRocksCastIfNeeded(g, end.Copy(), exp.DTypeDate),
			})}),
		})
		cteQuery := exp.Union(exp.Args{"this": baseQuery, "expression": recursiveQuery, "distinct": false})

		generateDates := exp.Select(exp.Args{
			"expressions": []exp.Expression{exp.Column(exp.Args{"this": exp.ToIdentifier(columnName)})},
			"from_": exp.From(exp.Args{"this": exp.Table(exp.Args{
				"this": exp.ToIdentifier(cteName),
			})}),
		})
		subquery := exp.Subquery(exp.Args{
			"this":  generateDates,
			"alias": exp.TableAlias(exp.Args{"this": exp.ToIdentifier(cteName)}),
		})
		unnest.Replace(subquery)

		ctes = append(ctes, exp.CTE(exp.Args{
			"this": cteQuery,
			"alias": exp.TableAlias(exp.Args{
				"this":    exp.ToIdentifier(cteName),
				"columns": []exp.Expression{exp.ToIdentifier(columnName)},
			}),
		}))
		count++
	}

	if len(ctes) > 0 {
		with := asExpression(expression.Arg("with_"))
		if with == nil {
			with = exp.With(exp.Args{"expressions": []exp.Expression{}})
		}
		with.Set("recursive", true)
		var existing []exp.Expression
		if items, ok := with.Arg("expressions").([]exp.Expression); ok {
			existing = items
		}
		combined := make([]exp.Expression, 0, len(ctes)+len(existing))
		combined = append(combined, ctes...)
		combined = append(combined, existing...)
		with.Set("expressions", combined)
		expression.Set("with_", with)
	}
}
