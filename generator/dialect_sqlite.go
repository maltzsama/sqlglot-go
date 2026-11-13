package generator

import (
	"strings"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// SQLite generator overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/generators/sqlite.py.
func init() {
	registerDialectDispatch("sqlite", sqliteDispatch())
	// generators/sqlite.py:128-144 TYPE_MAPPING: the base table minus BLOB, overlaid with the
	// SQLite storage classes.
	registerDialectRemoveTypeMappings("sqlite", exp.DTypeBlob)
	registerDialectTypeMapping("sqlite", map[exp.DType]string{
		exp.DTypeBoolean:   "INTEGER",
		exp.DTypeTinyInt:   "INTEGER",
		exp.DTypeSmallInt:  "INTEGER",
		exp.DTypeInt:       "INTEGER",
		exp.DTypeBigInt:    "INTEGER",
		exp.DTypeFloat:     "REAL",
		exp.DTypeDouble:    "REAL",
		exp.DTypeDecimal:   "REAL",
		exp.DTypeChar:      "TEXT",
		exp.DTypeNChar:     "TEXT",
		exp.DTypeVarchar:   "TEXT",
		exp.DTypeNVarchar:  "TEXT",
		exp.DTypeBinary:    "BLOB",
		exp.DTypeVarBinary: "BLOB",
	})
	// generators/sqlite.py:192-205: every property is UNSUPPORTED except a handful SQLite
	// can express (or transpile).
	locations := map[exp.Kind]propertyLocation{}
	for kind := range propertyLocations {
		locations[kind] = propertyLocationUnsupported
	}
	locations[exp.KindLikeProperty] = propertyLocationPostSchema
	locations[exp.KindModuleProperty] = propertyLocationPostSchema
	locations[exp.KindTemporaryProperty] = propertyLocationPostCreate
	locations[exp.KindVirtualProperty] = propertyLocationPostCreate
	registerDialectPropertyLocations("sqlite", locations)
	// generators/sqlite.py:146-148 TOKEN_MAPPING: AUTO_INCREMENT renders AUTOINCREMENT.
	registerDialectTokenMapping("sqlite", map[tokens.TokenType]string{
		tokens.AUTO_INCREMENT: "AUTOINCREMENT",
	})
}

// sqliteRename ports rename_func (dialects/dialect.py:1214-1215): render the call under a
// different name with every argument value flattened in order.
func sqliteRename(name string) func(*Generator, exp.Expression) string {
	return func(g *Generator, e exp.Expression) string {
		return g.funcCall(name, g.fallbackArgs(e), "(", ")", true)
	}
}

// sqliteExprList converts an argument value into a slice of expressions, dropping non-nodes.
func sqliteExprList(value any) []exp.Expression {
	items := listFromValue(value)
	out := make([]exp.Expression, 0, len(items))
	for _, item := range items {
		if expr := asExpression(item); expr != nil {
			out = append(out, expr)
		}
	}
	return out
}

func sqliteDispatch() map[exp.Kind]func(*Generator, exp.Expression) string {
	return map[exp.Kind]func(*Generator, exp.Expression) string{
		exp.KindAnyValue: sqliteRename("MAX"),
		exp.KindChr:      sqliteRename("CHAR"),
		exp.KindConcat: func(g *Generator, e exp.Expression) string {
			return g.concatToDPipe(e)
		},
		exp.KindCountIf:                  sqliteCountIfSQL,
		exp.KindCreate:                   func(g *Generator, e exp.Expression) string { return g.createSQL(sqliteTransformCreate(e)) },
		exp.KindCurrentDate:              func(*Generator, exp.Expression) string { return "CURRENT_DATE" },
		exp.KindCurrentTime:              func(*Generator, exp.Expression) string { return "CURRENT_TIME" },
		exp.KindCurrentTimestamp:         func(*Generator, exp.Expression) string { return "CURRENT_TIMESTAMP" },
		exp.KindCurrentVersion:           func(*Generator, exp.Expression) string { return "SQLITE_VERSION()" },
		exp.KindColumnDef:                func(g *Generator, e exp.Expression) string { return g.columnDefSQL(sqliteGeneratedToAutoIncrement(e)) },
		exp.KindDateStrToDate:            func(g *Generator, e exp.Expression) string { return g.sqlKey(e, "this") },
		exp.KindIf:                       sqliteRename("IIF"),
		exp.KindILike:                    sqliteNoILikeSQL,
		exp.KindJSONArrayAgg:             sqliteRename("JSON_GROUP_ARRAY"),
		exp.KindJSONObjectAgg:            func(g *Generator, e exp.Expression) string { return g.jsonObjectSQLNamed(e, "JSON_GROUP_OBJECT") },
		exp.KindLevenshtein:              sqliteRename("EDITDIST3"),
		exp.KindLogicalOr:                sqliteRename("MAX"),
		exp.KindLogicalAnd:               sqliteRename("MIN"),
		exp.KindPivot:                    sqliteNoPivotSQL,
		exp.KindRand:                     sqliteRename("RANDOM"),
		exp.KindSelect:                   func(g *Generator, e exp.Expression) string { return g.selectSQL(sqliteOffsetToLimit(e)) },
		exp.KindStrPosition:              sqliteStrPositionSQL,
		exp.KindTableSample:              sqliteNoTableSampleSQL,
		exp.KindTimeStrToTime:            func(g *Generator, e exp.Expression) string { return g.sqlKey(e, "this") },
		exp.KindTimeToStr:                sqliteTimeToStrSQL,
		exp.KindTryCast:                  func(g *Generator, e exp.Expression) string { return g.castSQLWithPrefix(e, "") },
		exp.KindTsOrDsToTimestamp:        func(g *Generator, e exp.Expression) string { return g.sqlKey(e, "this") },
		exp.KindInsert:                   sqliteInsertSQL,
		exp.KindBitwiseAndAgg:            sqliteBitwiseAggSQL("BITWISE_AND"),
		exp.KindBitwiseOrAgg:             sqliteBitwiseAggSQL("BITWISE_OR"),
		exp.KindBitwiseXorAgg:            sqliteBitwiseAggSQL("BITWISE_XOR"),
		exp.KindJSONExtract:              sqliteJSONExtractSQL,
		exp.KindJSONExtractScalar:        sqliteJSONExtractScalarSQL,
		exp.KindDateAdd:                  sqliteDateAddSQL,
		exp.KindCast:                     sqliteCastSQL,
		exp.KindComputedColumnConstraint: sqliteComputedColumnConstraintSQL,
		exp.KindTrunc:                    sqliteTruncSQL,
		exp.KindGenerateSeries:           sqliteGenerateSeriesSQL,
		exp.KindDateDiff:                 sqliteDateDiffSQL,
		exp.KindGroupConcat:              sqliteGroupConcatSQL,
		exp.KindLeast:                    sqliteLeastSQL,
		exp.KindGreatest:                 sqliteGreatestSQL,
		exp.KindTransaction:              sqliteTransactionSQL,
		exp.KindIsAscii:                  sqliteIsAsciiSQL,
		exp.KindCurrentSchema:            sqliteCurrentSchemaSQL,
		exp.KindIgnoreNulls:              sqliteIgnoreNullsSQL,
		exp.KindRespectNulls:             sqliteRespectNullsSQL,
		exp.KindWindowSpec:               sqliteWindowSpecSQL,
	}
}

// sqliteTransformCreate ports _transform_create (generators/sqlite.py:23-63): move a
// single-column primary key into the column and enforce AUTOINCREMENT after the key.
func sqliteTransformCreate(e exp.Expression) exp.Expression {
	schema := e.This()
	if e.Kind() != exp.KindCreate || schema == nil || schema.Kind() != exp.KindSchema {
		return e
	}
	defs := map[string]exp.Expression{}
	var primaryKey exp.Expression
	for _, child := range schema.Expressions() {
		switch child.Kind() {
		case exp.KindColumnDef:
			defs[child.Name()] = child
		case exp.KindPrimaryKey:
			primaryKey = child
		}
	}
	if primaryKey != nil && len(primaryKey.Expressions()) == 1 {
		if column := defs[primaryKey.Expressions()[0].Name()]; column != nil {
			column.Append("constraints", exp.ColumnConstraint(exp.Args{
				"kind": exp.PrimaryKeyColumnConstraint(nil),
			}))
			remaining := make([]exp.Expression, 0, len(schema.Expressions())-1)
			for _, child := range schema.Expressions() {
				if child != primaryKey {
					remaining = append(remaining, child)
				}
			}
			schema.Set("expressions", remaining)
		}
	}
	for _, column := range defs {
		primaryKeyIndex := -1
		var autoIncrement exp.Expression
		autoIncrementIndex := -1
		constraints := sqliteExprList(column.Arg("constraints"))
		for i, constraint := range constraints {
			kind := asExpression(constraint.Arg("kind"))
			if kind == nil {
				continue
			}
			switch kind.Kind() {
			case exp.KindPrimaryKeyColumnConstraint:
				primaryKeyIndex = i
			case exp.KindAutoIncrementColumnConstraint:
				autoIncrement = constraint
				autoIncrementIndex = i
			}
		}
		if autoIncrement != nil && (primaryKeyIndex == -1 || autoIncrementIndex < primaryKeyIndex) {
			next := make([]exp.Expression, 0, len(constraints))
			for _, constraint := range constraints {
				if constraint != autoIncrement {
					next = append(next, constraint)
				}
			}
			if primaryKeyIndex != -1 {
				insertAt := primaryKeyIndex
				if insertAt > len(next) {
					insertAt = len(next)
				}
				next = append(next[:insertAt], append([]exp.Expression{autoIncrement}, next[insertAt:]...)...)
			}
			column.Set("constraints", next)
		}
	}
	return e
}

// sqliteGeneratedToAutoIncrement ports _generated_to_auto_increment
// (generators/sqlite.py:66-85): a true identity column becomes AUTOINCREMENT, dropping the
// generated and NOT NULL constraints. Expression-bearing forms are computed columns.
func sqliteGeneratedToAutoIncrement(e exp.Expression) exp.Expression {
	if e.Kind() != exp.KindColumnDef {
		return e
	}
	generated := e.Find(exp.KindGeneratedAsIdentityColumnConstraint)
	if generated == nil || generated.Expr() != nil {
		return e
	}
	drop := map[exp.Expression]bool{}
	if parent := generated.Parent(); parent != nil {
		drop[parent] = true
	}
	if notNull := e.Find(exp.KindNotNullColumnConstraint); notNull != nil {
		if parent := notNull.Parent(); parent != nil {
			drop[parent] = true
		}
	}
	next := make([]exp.Expression, 0, len(sqliteExprList(e.Arg("constraints"))))
	for _, constraint := range sqliteExprList(e.Arg("constraints")) {
		if !drop[constraint] {
			next = append(next, constraint)
		}
	}
	e.Set("constraints", next)
	e.Append("constraints", exp.ColumnConstraint(exp.Args{
		"kind": exp.AutoIncrementColumnConstraint(nil),
	}))
	return e
}

// sqliteOffsetToLimit ports _offset_to_limit (generators/sqlite.py:88-97): an OFFSET without
// a LIMIT gains `LIMIT -1`.
func sqliteOffsetToLimit(e exp.Expression) exp.Expression {
	if e.Kind() != exp.KindSelect {
		return e
	}
	if e.Arg("offset") != nil && e.Arg("limit") == nil {
		e.Set("limit", exp.Limit(exp.Args{"expression": exp.LiteralNumber(-1)}))
	}
	return e
}

// sqliteCountIfSQL ports count_if_to_sum (dialects/dialect.py:1812-1819).
func sqliteCountIfSQL(g *Generator, e exp.Expression) string {
	cond := e.This()
	if cond != nil && cond.Kind() == exp.KindDistinct {
		if exprs := cond.Expressions(); len(exprs) > 0 {
			cond = exprs[0]
		}
		g.unsupported("DISTINCT is not supported when converting COUNT_IF to SUM")
	}
	iff := exp.If(exp.Args{
		"this":  cond,
		"true":  exp.LiteralNumber(1),
		"false": exp.LiteralNumber(0),
	})
	return g.funcCall("SUM", []any{iff}, "(", ")", true)
}

// sqliteNoILikeSQL ports no_ilike_sql (dialects/dialect.py:1269-1276).
func sqliteNoILikeSQL(g *Generator, e exp.Expression) string {
	return g.likeSQL(exp.Like(exp.Args{
		"this":       exp.Lower(exp.Args{"this": e.This()}),
		"expression": exp.Lower(exp.Args{"this": e.Arg("expression")}),
		"negate":     e.Arg("negate"),
	}))
}

// sqliteNoPivotSQL ports no_pivot_sql (dialects/dialect.py:1296-1298).
func sqliteNoPivotSQL(g *Generator, _ exp.Expression) string {
	g.unsupported("PIVOT unsupported")
	return ""
}

// sqliteNoTableSampleSQL ports no_tablesample_sql (dialects/dialect.py:1291-1293).
func sqliteNoTableSampleSQL(g *Generator, e exp.Expression) string {
	g.unsupported("TABLESAMPLE unsupported")
	return g.sqlKey(e, "this")
}

// sqliteTimeToStrSQL ports the SQLite TimeToStr TRANSFORM
// (generators/sqlite.py:187): STRFTIME(<format>, <this>).
func sqliteTimeToStrSQL(g *Generator, e exp.Expression) string {
	return g.funcCall("STRFTIME", []any{e.Arg("format"), e.This()}, "(", ")", true)
}

// sqliteStrPositionSQL ports strposition_sql with func_name="INSTR"
// (dialects/dialect.py:1321-1367, generators/sqlite.py:184).
func sqliteStrPositionSQL(g *Generator, e exp.Expression) string {
	str := e.This()
	substr := asExpression(e.Arg("substr"))
	position := asExpression(e.Arg("position"))
	occurrence := asExpression(e.Arg("occurrence"))

	transpilePosition := position != nil
	if transpilePosition {
		str = exp.Substring(exp.Args{"this": str, "start": position})
	}
	args := []exp.Expression{str}
	if substr != nil {
		args = append(args, substr)
	}
	if occurrence != nil {
		g.unsupported("INSTR does not support the occurrence parameter.")
	}
	fn := exp.Anonymous(exp.Args{"this": "INSTR", "expressions": args})
	if transpilePosition {
		offset := exp.Sub(exp.Args{
			"this":       exp.Add(exp.Args{"this": fn, "expression": position}),
			"expression": exp.LiteralNumber(1),
		})
		wrapped := exp.If(exp.Args{
			"this":  exp.EQ(exp.Args{"this": fn, "expression": exp.LiteralNumber(0)}),
			"true":  exp.LiteralNumber(0),
			"false": offset,
		})
		return g.gen(wrapped)
	}
	return g.gen(fn)
}

// sqliteInsertSQL ports insert_sql (generators/sqlite.py:209-214): INSERT OR IGNORE.
func sqliteInsertSQL(g *Generator, e exp.Expression) string {
	if ignore, _ := e.Arg("ignore").(bool); ignore {
		e.Set("ignore", false)
		e.Set("alternative", "IGNORE")
	}
	return g.insertSQL(e)
}

// sqliteBitwiseAggSQL ports the unsupported BITWISE_* aggregation methods
// (generators/sqlite.py:216-226).
func sqliteBitwiseAggSQL(name string) func(*Generator, exp.Expression) string {
	return func(g *Generator, e exp.Expression) string {
		g.unsupported(name + " aggregation is not supported in SQLite")
		return g.functionFallbackSQL(e)
	}
}

// sqliteJSONExtractSQL ports jsonextract_sql (generators/sqlite.py:228-231).
func sqliteJSONExtractSQL(g *Generator, e exp.Expression) string {
	if len(e.Expressions()) > 0 {
		return g.functionFallbackSQL(e)
	}
	return g.arrowJSONExtractBinary(e, "->")
}

// sqliteJSONExtractScalarSQL ports jsonextractscalar_sql (generators/sqlite.py:233-238):
// json_extract() keeps the JSON subtype on object/array results, so the function form is
// preserved when that subtype matters; ->> strips it.
func sqliteJSONExtractScalarSQL(g *Generator, e exp.Expression) string {
	if jsonSubtype, _ := e.Arg("json_subtype").(bool); jsonSubtype {
		return g.funcCall("JSON_EXTRACT", []any{e.This(), e.Arg("expression")}, "(", ")", true)
	}
	return g.arrowJSONExtractScalarSQL(e)
}

// sqliteDateAddSQL ports dateadd_sql (generators/sqlite.py:240-250): the modifier is a
// quoted `<amount> <unit>` string.
func sqliteDateAddSQL(g *Generator, e exp.Expression) string {
	modifier := asExpression(e.Arg("expression"))
	unit := asExpression(e.Arg("unit"))
	if modifier != nil && modifier.Kind() == exp.KindInterval {
		if unit == nil {
			unit = asExpression(modifier.Arg("unit"))
		}
		modifier = modifier.This()
	}
	var modifierText string
	if modifier != nil && modifier.Kind() == exp.KindLiteral {
		modifierText = modifier.Name()
	} else {
		modifierText = g.gen(modifier)
	}
	if unit != nil {
		modifierText = modifierText + " " + unit.Name()
	}
	return g.funcCall("DATE", []any{e.This(), exp.LiteralString(modifierText)}, "(", ")", false)
}

// sqliteCastSQL ports cast_sql (generators/sqlite.py:252-256): a DATE cast renders as
// DATE(<this>).
func sqliteCastSQL(g *Generator, e exp.Expression) string {
	if to := asExpression(e.Arg("to")); to != nil && to.Kind() == exp.KindDataType && to.Arg("this") == exp.DTypeDate {
		return g.funcCall("DATE", []any{e.This()}, "(", ")", true)
	}
	return g.castSQLWithPrefix(e, "")
}

// sqliteComputedColumnConstraintSQL ports computedcolumnconstraint_sql
// (generators/sqlite.py:261-269).
func sqliteComputedColumnConstraintSQL(g *Generator, e exp.Expression) string {
	if e.Arg("data_type") != nil {
		g.unsupported("SQLite generated columns do not support a data type")
	}
	this := e.This()
	var thisSQL string
	if this != nil && this.Kind() == exp.KindParen {
		thisSQL = g.gen(this)
	} else {
		thisSQL = "(" + g.gen(this) + ")"
	}
	storage := ""
	if persisted, _ := e.Arg("persisted").(bool); persisted {
		storage = " STORED"
	}
	notNull := ""
	if v, _ := e.Arg("not_null").(bool); v {
		notNull = " NOT NULL"
	}
	return "AS " + thisSQL + storage + notNull
}

// sqliteTruncSQL ports trunc_sql (generators/sqlite.py:275-277): SQLite TRUNC takes one arg.
func sqliteTruncSQL(g *Generator, e exp.Expression) string {
	if e.Arg("decimals") != nil {
		g.unsupported("'decimals' is not supported")
	}
	return g.funcCall("TRUNC", []any{e.This()}, "(", ")", true)
}

// sqliteGenerateSeriesSQL ports generateseries_sql (generators/sqlite.py:279-292): a column
// alias on the series becomes a projected subquery alias.
func sqliteGenerateSeriesSQL(g *Generator, e exp.Expression) string {
	parent := e.Parent()
	var alias exp.Expression
	if parent != nil {
		alias = asExpression(parent.Arg("alias"))
	}
	if alias != nil && alias.Kind() == exp.KindTableAlias {
		if columns := sqliteExprList(alias.Arg("columns")); len(columns) > 0 {
			columnAlias := columns[0]
			alias.Set("columns", nil)
			inner := exp.Select(exp.Args{
				"expressions": []exp.Expression{exp.AliasNode(exp.Args{"this": exp.ToIdentifier("value"), "alias": columnAlias})},
				"from_":       exp.From(exp.Args{"this": e}),
			})
			return g.gen(exp.Subquery(exp.Args{"this": inner}))
		}
	}
	return g.functionFallbackSQL(e)
}

// sqliteDateDiffSQL ports datediff_sql (generators/sqlite.py:294-319).
func sqliteDateDiffSQL(g *Generator, e exp.Expression) string {
	unit := "DAY"
	if u := asExpression(e.Arg("unit")); u != nil {
		unit = strings.ToUpper(u.Name())
	}
	sql := "(JULIANDAY(" + g.sqlKey(e, "this") + ") - JULIANDAY(" + g.sqlKey(e, "expression") + "))"
	switch unit {
	case "MONTH":
		sql += " / 30.0"
	case "YEAR":
		sql += " / 365.0"
	case "HOUR":
		sql += " * 24.0"
	case "MINUTE":
		sql += " * 1440.0"
	case "SECOND":
		sql += " * 86400.0"
	case "MILLISECOND":
		sql += " * 86400000.0"
	case "MICROSECOND":
		sql += " * 86400000000.0"
	case "NANOSECOND":
		// DIVERGENCE (DEVIATIONS §2): one day is 86,400,000,000,000 ns; upstream uses a
		// value ten times too small, so this matches the real engine instead.
		sql += " * 86400000000000.0"
	default:
		g.unsupported("DATEDIFF unsupported for '" + unit + "'.")
	}
	return "CAST(" + sql + " AS INTEGER)"
}

// sqliteGroupConcatSQL ports groupconcat_sql (generators/sqlite.py:322-338).
func sqliteGroupConcatSQL(g *Generator, e exp.Expression) string {
	this := e.This()
	distinctSQL := ""
	if distinct := sqliteFindInScope(e, exp.KindDistinct); distinct != nil {
		if exprs := distinct.Expressions(); len(exprs) > 0 {
			this = exprs[0]
		}
		distinctSQL = "DISTINCT "
	}
	if this != nil && this.Kind() == exp.KindOrder {
		g.unsupported("SQLite GROUP_CONCAT doesn't support ORDER BY.")
		if inner := this.This(); inner != nil && distinctSQL == "" {
			this = inner
		}
	}
	args := []any{this}
	if separator := e.Arg("separator"); separator != nil {
		args = append(args, separator)
	}
	return "GROUP_CONCAT(" + distinctSQL + g.formatArgs(args, ", ") + ")"
}

// sqliteLeastSQL ports least_sql (generators/sqlite.py:340-344).
func sqliteLeastSQL(g *Generator, e exp.Expression) string {
	if len(e.Expressions()) > 0 {
		return sqliteRename("MIN")(g, e)
	}
	return g.sqlKey(e, "this")
}

// sqliteGreatestSQL ports greatest_sql (generators/sqlite.py:346-350).
func sqliteGreatestSQL(g *Generator, e exp.Expression) string {
	if len(e.Expressions()) > 0 {
		return sqliteRename("MAX")(g, e)
	}
	return g.sqlKey(e, "this")
}

// sqliteTransactionSQL ports transaction_sql (generators/sqlite.py:352-355).
func sqliteTransactionSQL(g *Generator, e exp.Expression) string {
	this := g.sqlKey(e, "this")
	if this != "" {
		this = " " + this
	}
	return "BEGIN" + this + " TRANSACTION"
}

// sqliteIsAsciiSQL ports isascii_sql (generators/sqlite.py:357-358).
func sqliteIsAsciiSQL(g *Generator, e exp.Expression) string {
	return "(NOT " + g.sqlKey(e, "this") + " GLOB CAST(x'2a5b5e012d7f5d2a' AS TEXT))"
}

// sqliteCurrentSchemaSQL ports currentschema_sql (generators/sqlite.py:360-362).
func sqliteCurrentSchemaSQL(g *Generator, _ exp.Expression) string {
	g.unsupported("CURRENT_SCHEMA is not supported in SQLite")
	return "'main'"
}

// sqliteIgnoreNullsSQL ports ignorenulls_sql (generators/sqlite.py:364-366).
func sqliteIgnoreNullsSQL(g *Generator, e exp.Expression) string {
	g.unsupported("SQLite does not support IGNORE NULLS.")
	return g.sqlKey(e, "this")
}

// sqliteRespectNullsSQL ports respectnulls_sql (generators/sqlite.py:368-369).
func sqliteRespectNullsSQL(g *Generator, e exp.Expression) string {
	return g.sqlKey(e, "this")
}

// sqliteWindowSpecSQL ports windowspec_sql (generators/sqlite.py:371-378).
func sqliteWindowSpecSQL(g *Generator, e exp.Expression) string {
	// DIVERGENCE (DEVIATIONS §1.30): upstream abbreviates any RANGE frame whose start is
	// CURRENT ROW, dropping an explicit end (and any EXCLUDE); only the bare shorthand form
	// is equivalent to RANGE CURRENT ROW.
	if strings.EqualFold(e.Text("kind"), "RANGE") && strings.EqualFold(e.Text("start"), "CURRENT ROW") &&
		e.Arg("end") == nil && e.Arg("exclude") == nil {
		return "RANGE CURRENT ROW"
	}
	return g.windowSpecSQL(e)
}

// sqliteFindInScope ports optimizer.scope.find_in_scope: a depth-first search for the first
// descendant of the given kind that stops at a nested query scope (a Select other than the
// starting node), so GROUP_CONCAT never picks up a DISTINCT from a subquery.
func sqliteFindInScope(root exp.Expression, kind exp.Kind) exp.Expression {
	var walk func(node exp.Expression) exp.Expression
	walk = func(node exp.Expression) exp.Expression {
		for _, child := range node.Expressions() {
			if child == nil {
				continue
			}
			if child.Kind() == kind {
				return child
			}
			if child.Kind() == exp.KindSelect {
				continue
			}
			if found := walk(child); found != nil {
				return found
			}
		}
		return nil
	}
	return walk(root)
}
