package generator

import (
	"strconv"
	"strings"

	"github.com/maltzsama/sqlglot-go/expressions"
)

func (g *Generator) prependCtes(e expressions.Expression, sql string) string {
	with := g.sqlKey(e, "with_")
	if with != "" {
		sql = with + g.sep() + sql
	}
	return sql
}

func (g *Generator) queryModifiers(e expressions.Expression, sqls ...string) string {
	// ksqlDB's query (SqlBase.g4:117-127) orders WINDOW before WHERE, puts PARTITION BY
	// between GROUP BY and HAVING, and renders EMIT just before the LIMIT clause —
	// none of which the base order (generator.py:3271-3306) can express.
	if g.isDialect("ksqldb") {
		return g.ksqlQueryModifiers(e, sqls...)
	}
	limit := asExpression(e.Arg("limit"))
	// generator.py:3275-3285 LIMIT_FETCH: rewrite between the LIMIT and FETCH spellings.
	if g.dialect.LimitFetch == "FETCH" && limit != nil && limit.Kind() == expressions.KindLimit {
		limit = expressions.Fetch(expressions.Args{"direction": "FIRST", "count": limit.Arg("expression")})
	} else if g.dialect.LimitFetch == "LIMIT" && limit != nil && limit.Kind() == expressions.KindFetch {
		// generator.py:3276-3281: a FETCH without a count means one row per the SQL standard;
		// emitting a bare LIMIT would be invalid SQL, so substitute the literal 1.
		count := limit.Arg("count")
		if count == nil {
			count = expressions.LiteralNumber("1")
		}
		limit = expressions.Limit(expressions.Args{"expression": count})
	}
	fetch := limit != nil && limit.Kind() == expressions.KindFetch
	parts := make([]string, 0, len(sqls)+12)
	parts = append(parts, sqls...)
	for _, join := range listFromValue(e.Arg("joins")) {
		parts = append(parts, g.gen(join))
	}
	parts = append(parts, g.sqlKey(e, "match"))
	for _, lateral := range listFromValue(e.Arg("laterals")) {
		parts = append(parts, g.gen(lateral))
	}
	parts = append(parts, g.sqlKey(e, "prewhere"), g.sqlKey(e, "where"), g.sqlKey(e, "connect"), g.sqlKey(e, "group"), g.sqlKey(e, "having"))
	parts = append(parts, g.sqlKey(e, "cluster"), g.sqlKey(e, "distribute"), g.sqlKey(e, "sort"))
	qualify := g.sqlKey(e, "qualify")
	if g.dialect.QualifyBeforeWindow {
		parts = append(parts, qualify)
	}
	if truthy(e.Arg("windows")) {
		parts = append(parts, g.seg("WINDOW ")+g.expressions(exprsOptions{expression: e, key: "windows", flat: true}))
	}
	if !g.dialect.QualifyBeforeWindow {
		parts = append(parts, qualify)
	}
	parts = append(parts, g.sqlKey(e, "order"))
	parts = append(parts, g.offsetLimitModifiers(e, fetch, limit)...)
	parts = append(parts, g.afterLimitModifiers(e)...)
	parts = append(parts, g.optionsModifier(e), g.sqlKey(e, "for_"))
	return joinNonEmpty("", parts...)
}

func (g *Generator) offsetLimitModifiers(e expressions.Expression, fetch bool, limit expressions.Expression) []string {
	// generators/presto.py:631-637: OFFSET precedes LIMIT in Presto.
	if fetch || g.isDialect("presto") {
		return []string{g.sqlKey(e, "offset"), g.gen(limit)}
	}
	return []string{g.gen(limit), g.sqlKey(e, "offset")}
}

func (g *Generator) afterLimitModifiers(e expressions.Expression) []string {
	locks := g.expressions(exprsOptions{expression: e, key: "locks", sep: " "})
	if locks != "" {
		locks = " " + locks
	}
	parts := []string{locks, g.sqlKey(e, "sample")}
	// after_limit_modifiers override (generators/clickhouse.py:564-576): SETTINGS and
	// FORMAT trail every other query modifier.
	if g.isDialect("clickhouse") {
		if truthy(e.Arg("settings")) {
			parts = append(parts, g.seg("SETTINGS ")+g.expressions(exprsOptions{expression: e, key: "settings", flat: true}))
		}
		if truthy(e.Arg("format")) {
			parts = append(parts, g.seg("FORMAT ")+g.sqlKey(e, "format"))
		}
	}
	return parts
}

func (g *Generator) optionsModifier(e expressions.Expression) string {
	options := g.expressions(exprsOptions{expression: e, key: "options"})
	if options != "" {
		// generators/tsql.py:602-604: TSQL spells the modifier `OPTION (...)`.
		if g.isDialect("tsql") {
			return " OPTION" + g.wrap(options)
		}
		return " " + options
	}
	return ""
}

func (g *Generator) selectSQL(e expressions.Expression) string {
	// transforms.py:616-631: dialects that lack native SEMI/ANTI joins rewrite them into
	// correlated [NOT] EXISTS predicates before rendering the query.
	if g.dialect.EliminateDistinctOn {
		e = starRocksEliminateDistinctOn(g, e)
	}
	if g.dialect.EliminateSemiAntiJoins {
		e = eliminateSemiAndAntiJoins(e)
	}
	hint := g.sqlKey(e, "hint")
	distinct := g.sqlKey(e, "distinct")
	if distinct != "" {
		distinct = " " + distinct
	}
	kind := g.sqlKey(e, "kind")
	// generators/tsql.py:3347-3351 / generator.py:3347-3351: with LIMIT_IS_TOP a Limit is
	// rendered as SELECT TOP and removed from the trailing modifiers.
	top := ""
	if g.dialect.LimitIsTop {
		if limit := asExpression(e.Arg("limit")); limit != nil && limit.Kind() == expressions.KindLimit &&
			asExpression(e.Arg("offset")) == nil {
			top = " " + g.limitTopSQL(limit)
			e.Set("limit", nil)
		}
	}
	expressionsSQL := g.expressions(exprsOptions{expression: e})
	if kind != "" {
		if kind == "STRUCT" || kind == "VALUE" {
			kind = " AS " + kind
		} else {
			kind = ""
		}
	}
	operationModifiers := g.expressions(exprsOptions{expression: e, key: "operation_modifiers", sep: " "})
	if operationModifiers != "" {
		operationModifiers = g.sep() + operationModifiers
	}
	// generator.py:3381-3385: a select-level EXCLUDE renders after the projections when the
	// dialect doesn't need the derived-table rewrite (Redshift).
	if exclude := listFromValue(e.Arg("exclude")); len(exclude) > 0 && !g.dialect.StarExcludeRequiresDerivedTable {
		expressionsSQL += g.seg("EXCLUDE") + " (" + g.expressions(exprsOptions{sqls: exclude, flat: true}) + ")"
	}
	if expressionsSQL != "" {
		expressionsSQL = g.sep() + expressionsSQL
	}
	// SELECT ... INTO (generator.py:3302-3304, 3374-3381): postgres (SUPPORTS_SELECT_INTO)
	// keeps the INTO inline; every other dialect drops it here and instead wraps the finished
	// SELECT in `CREATE TABLE <into.this> AS ...` below. The keyed sql(expr,"into",...) call
	// still recurses with comments on, so we use sqlKey when we do render it inline.
	into := asExpression(e.Arg("into"))
	// MySQL `INTO {OUTFILE|DUMPFILE} '/path'` is a server-side file write, not a
	// table materialization: it renders inline at the trailing position (its canonical
	// spot) via intoSQL, never as the `CREATE TABLE ... AS` rewrite the plain SELECT
	// INTO takes on non-SUPPORTS_SELECT_INTO dialects. Grammar extension beyond upstream.
	intoIsFileWrite := into != nil && isOutfileKeyword(into.Text("kind"))
	intoInline := ""
	if into != nil && g.dialect.SupportsSelectInto && !intoIsFileWrite {
		intoInline = g.sqlKey(e, "into")
	}
	// generator.py:3388-3390: LIMIT_IS_TOP dialects place DISTINCT/hint before TOP.
	prefix := "SELECT" + top + hint + distinct
	if g.dialect.LimitIsTop {
		prefix = "SELECT" + distinct + hint + top
	}
	sql := g.queryModifiers(e,
		prefix+operationModifiers+kind+expressionsSQL,
		intoInline,
		g.sqlKey(e, "from_"),
	)
	if truthy(e.Arg("with_")) {
		sql = g.maybeComment(sql, e, nil, false)
		e.PopComments()
	}
	sql = g.prependCtes(e, sql)
	if intoIsFileWrite {
		sql += g.sqlKey(e, "into")
	}
	if into != nil && !intoIsFileWrite && !g.dialect.SupportsSelectInto {
		// generator.py:3374-3381: temporary -> " TEMPORARY", else "" (UNLOGGED is only kept
		// under SUPPORTS_UNLOGGED_TABLES, which no base/mysql dialect sets, so it is dropped).
		tableKind := ""
		if boolValue(into.Arg("temporary")) {
			tableKind = " TEMPORARY"
		}
		sql = "CREATE" + tableKind + " TABLE " + g.sqlKey(into, "this") + " AS " + sql
	}
	// Not ported: with STAR_EXCLUDE_REQUIRES_DERIVED_TABLE (the base default), upstream
	// (generator.py:3405-3409) rewrites a select-level `exclude` arg into a derived table
	// `SELECT * EXCLUDE (...) FROM (<subquery>)`. Only Redshift's parser produces that arg, and
	// Redshift renders it inline (above), so the rewrite is unreachable for same-dialect output.
	return sql
}

func (g *Generator) fromSQL(e expressions.Expression) string {
	return g.seg("FROM") + " " + g.sqlKey(e, "this")
}

func (g *Generator) whereSQL(e expressions.Expression) string {
	this := g.indent(g.sqlKey(e, "this"), 0, nil, false, false)
	return g.seg("WHERE") + g.sep() + this
}

func (g *Generator) havingSQL(e expressions.Expression) string {
	this := g.indent(g.sqlKey(e, "this"), 0, nil, false, false)
	return g.seg("HAVING") + g.sep() + this
}

// connect_sql / prior_sql (generator.py:2830-2839).
func (g *Generator) connectSQL(e expressions.Expression) string {
	start := g.sqlKey(e, "start")
	if start != "" {
		start = g.seg("START WITH " + start)
	}
	nocycle := ""
	if boolValue(e.Arg("nocycle")) {
		nocycle = " NOCYCLE"
	}
	return start + g.seg("CONNECT BY"+nocycle+" "+g.sqlKey(e, "connect"))
}

func (g *Generator) priorSQL(e expressions.Expression) string {
	return "PRIOR " + g.sqlKey(e, "this")
}

func (g *Generator) qualifySQL(e expressions.Expression) string {
	this := g.indent(g.sqlKey(e, "this"), 0, nil, false, false)
	return g.seg("QUALIFY") + g.sep() + this
}

func (g *Generator) groupingSetsSQL(e expressions.Expression) string {
	groupingSets := g.expressions(exprsOptions{expression: e, noIndent: true})
	return "GROUPING SETS " + g.wrap(groupingSets)
}

func (g *Generator) rollupSQL(e expressions.Expression) string {
	expressionsSQL := g.expressions(exprsOptions{expression: e, noIndent: true})
	if expressionsSQL != "" {
		return "ROLLUP " + g.wrap(expressionsSQL)
	}
	return "WITH ROLLUP"
}

func (g *Generator) cubeSQL(e expressions.Expression) string {
	expressionsSQL := g.expressions(exprsOptions{expression: e, noIndent: true})
	if expressionsSQL != "" {
		return "CUBE " + g.wrap(expressionsSQL)
	}
	return "WITH CUBE"
}

func (g *Generator) groupSQL(e expressions.Expression) string {
	modifier := ""
	if v, ok := e.Arg("all").(bool); ok {
		if v {
			modifier = " ALL"
		} else {
			modifier = " DISTINCT"
		}
	}
	groupBy := g.opExpressions("GROUP BY"+modifier, e)
	groupingSets := g.expressions(exprsOptions{expression: e, key: "grouping_sets"})
	cube := g.expressions(exprsOptions{expression: e, key: "cube"})
	rollup := g.expressions(exprsOptions{expression: e, key: "rollup"})
	groupings := joinNonEmpty(",",
		func() string {
			if groupingSets != "" {
				return g.seg(groupingSets)
			}
			return ""
		}(),
		func() string {
			if cube != "" {
				return g.seg(cube)
			}
			return ""
		}(),
		func() string {
			if rollup != "" {
				return g.seg(rollup)
			}
			return ""
		}(),
		func() string {
			if boolValue(e.Arg("totals")) {
				return g.seg("WITH TOTALS")
			}
			return ""
		}(),
	)
	if len(listFromValue(e.Arg("expressions"))) > 0 && groupings != "" {
		trimmed := strings.TrimSpace(groupings)
		if !strings.HasPrefix(trimmed, "WITH ") {
			groupBy += ","
		}
	}
	return groupBy + groupings
}

func (g *Generator) orderSQL(e expressions.Expression) string { return g.orderSQLFlat(e, false) }

func (g *Generator) orderSQLFlat(e expressions.Expression, flat bool) string {
	this := g.sqlKey(e, "this")
	if this != "" {
		this += " "
	}
	siblings := ""
	if boolValue(e.Arg("siblings")) {
		siblings = "SIBLINGS "
	}
	return g.opExpressions(this+"ORDER "+siblings+"BY", e, this != "" || flat)
}

func (g *Generator) orderedSQL(e expressions.Expression) string {
	desc, descSet := e.Arg("desc").(bool)
	asc := !desc
	nullsFirst := boolValue(e.Arg("nulls_first"))
	nullsLast := !nullsFirst
	nullsAreLarge := g.dialect.NullOrdering == "nulls_are_large"
	nullsAreSmall := g.dialect.NullOrdering == "nulls_are_small"
	nullsAreLast := g.dialect.NullOrdering == "nulls_are_last"
	this := g.sqlKey(e, "this")
	sortOrder := ""
	if desc {
		sortOrder = " DESC"
	} else if descSet {
		sortOrder = " ASC"
	}
	nullsSortChange := ""
	if nullsFirst && ((asc && nullsAreLarge) || (desc && nullsAreSmall) || nullsAreLast) {
		nullsSortChange = " NULLS FIRST"
	} else if nullsLast && ((asc && nullsAreSmall) || (desc && nullsAreLarge)) && !nullsAreLast {
		nullsSortChange = " NULLS LAST"
	}
	withFill := g.sqlKey(e, "with_fill")
	if withFill != "" {
		withFill = " " + withFill
	}
	return this + sortOrder + nullsSortChange + withFill
}

func (g *Generator) limitSQL(e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	args := []string{}
	for _, key := range []string{"offset", "expression"} {
		if v := e.Arg(key); truthy(v) {
			args = append(args, g.gen(g.limitArg(v)))
		}
	}
	argsSQL := strings.Join(args, ", ")
	exprs := g.expressions(exprsOptions{expression: e, flat: true})
	if exprs != "" {
		exprs = " BY " + exprs
	}
	limitOptions := g.sqlKey(e, "limit_options")
	return this + g.seg("LIMIT") + " " + argsSQL + limitOptions + exprs
}

// limitArg ports _simplify_unless_literal (generator.py:5380-5386): under
// LIMIT_ONLY_LITERALS a non-literal LIMIT/OFFSET argument is constant-folded.
func (g *Generator) limitArg(v any) any {
	if !g.dialect.LimitOnlyLiterals {
		return v
	}
	expr := asExpression(v)
	if expr == nil || expr.Kind() == expressions.KindLiteral {
		return v
	}
	if folded := foldNumericLiteral(expr); folded != nil {
		return folded
	}
	return v
}

// foldNumericLiteral folds a constant arithmetic expression over numeric literals (e.g.
// `1 + 1` -> 2), the subset of the optimizer's simplify() pass LIMIT_ONLY_LITERALS needs for
// LIMIT/OFFSET arguments. Returns nil when the expression is not fully constant.
func foldNumericLiteral(e expressions.Expression) expressions.Expression {
	if e == nil {
		return nil
	}
	switch e.Kind() {
	case expressions.KindLiteral:
		if e.IsString() {
			return nil
		}
		return e
	case expressions.KindParen:
		return foldNumericLiteral(e.This())
	case expressions.KindNeg:
		inner := foldNumericLiteral(e.This())
		if inner == nil {
			return nil
		}
		value, ok := numericLiteralValue(inner)
		if !ok {
			return nil
		}
		return expressions.LiteralNumber(-value)
	case expressions.KindAdd, expressions.KindSub, expressions.KindMul, expressions.KindDiv, expressions.KindMod:
		left := foldNumericLiteral(e.This())
		right := foldNumericLiteral(asExpression(e.Arg("expression")))
		if left == nil || right == nil {
			return nil
		}
		a, okA := numericLiteralValue(left)
		b, okB := numericLiteralValue(right)
		if !okA || !okB {
			return nil
		}
		switch e.Kind() {
		case expressions.KindAdd:
			return expressions.LiteralNumber(a + b)
		case expressions.KindSub:
			return expressions.LiteralNumber(a - b)
		case expressions.KindMul:
			return expressions.LiteralNumber(a * b)
		case expressions.KindDiv:
			if b == 0 || a%b != 0 {
				return nil
			}
			return expressions.LiteralNumber(a / b)
		case expressions.KindMod:
			if b == 0 {
				return nil
			}
			return expressions.LiteralNumber(a % b)
		}
	}
	return nil
}

// numericLiteralValue returns the integer value of an integer Literal.
func numericLiteralValue(e expressions.Expression) (int64, bool) {
	if e == nil || e.Kind() != expressions.KindLiteral || e.IsString() {
		return 0, false
	}
	value, err := strconv.ParseInt(e.Name(), 10, 64)
	return value, err == nil
}

// limitTopSQL ports limit_sql(..., top=True) (generator.py:2932-2948): the SELECT TOP form
// used by LIMIT_IS_TOP dialects. A non-literal argument is parenthesized, e.g. `TOP (@n)`.
func (g *Generator) limitTopSQL(e expressions.Expression) string {
	var parts []string
	needsParens := false
	for _, key := range []string{"offset", "expression"} {
		v := e.Arg(key)
		if !truthy(v) {
			continue
		}
		parts = append(parts, g.gen(g.limitArg(v)))
		if ex, ok := v.(expressions.Expression); !ok || !ex.IsNumber() {
			needsParens = true
		}
	}
	argsSQL := strings.Join(parts, ", ")
	if needsParens {
		argsSQL = "(" + argsSQL + ")"
	}
	exprs := g.expressions(exprsOptions{expression: e, flat: true})
	if exprs != "" {
		exprs = " BY " + exprs
	}
	return "TOP " + argsSQL + g.sqlKey(e, "limit_options") + exprs
}

func (g *Generator) offsetSQL(e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	exprs := g.expressions(exprsOptions{expression: e, flat: true})
	if exprs != "" {
		exprs = " BY " + exprs
	}
	return this + g.seg("OFFSET") + " " + g.sqlKey(e, "expression") + exprs
}

func (g *Generator) fetchSQL(e expressions.Expression) string {
	direction := e.Text("direction")
	if direction != "" {
		direction = " " + direction
	}
	count := g.sqlKey(e, "count")
	if count != "" {
		count = " " + count
	}
	limitOptions := g.sqlKey(e, "limit_options")
	if limitOptions == "" {
		limitOptions = " ROWS ONLY"
	}
	return g.seg("FETCH") + direction + count + limitOptions
}

func (g *Generator) limitOptionsSQL(e expressions.Expression) string {
	percent := ""
	if boolValue(e.Arg("percent")) {
		percent = " PERCENT"
	}
	rows := ""
	if boolValue(e.Arg("rows")) {
		rows = " ROWS"
	}
	withTies := ""
	if boolValue(e.Arg("with_ties")) {
		withTies = " WITH TIES"
	}
	if withTies == "" && rows != "" {
		withTies = " ONLY"
	}
	return percent + rows + withTies
}
