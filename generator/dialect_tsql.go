package generator

import (
	"strconv"
	"strings"

	"github.com/ridi-oss/sqlglot-go/dialects"
	exp "github.com/ridi-oss/sqlglot-go/expressions"
)

// TSQL generator overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/generators/tsql.py:121-655.
func init() {
	registerDialectDispatch("tsql", map[exp.Kind]func(*Generator, exp.Expression) string{
		exp.KindAnyValue: func(g *Generator, e exp.Expression) string { return g.funcCall("MAX", []any{e.This()}, "(", ")", true) },
		exp.KindAtan2: func(g *Generator, e exp.Expression) string {
			return g.funcCall("ATN2", g.fallbackArgs(e), "(", ")", true)
		},
		exp.KindAutoIncrementColumnConstraint: func(g *Generator, e exp.Expression) string { return "IDENTITY" },
		exp.KindCeil:                          rename("CEILING"),
		exp.KindChr:                           rename("CHAR"),
		exp.KindDateAdd:                       tsqlDateDelta("DATEADD", false),
		exp.KindCurrentDate:                   func(g *Generator, e exp.Expression) string { return "GETDATE()" },
		exp.KindCurrentTimestamp:              func(g *Generator, e exp.Expression) string { return "GETDATE()" },
		exp.KindCurrentTimestampLTZ:           func(g *Generator, e exp.Expression) string { return "SYSDATETIMEOFFSET()" },
		exp.KindDateStrToDate: func(g *Generator, e exp.Expression) string {
			return g.gen(exp.Cast(exp.Args{"this": e.This(), "to": exp.DTypeDate.IntoExpr(nil)}))
		},
		exp.KindDay:   tsqlRemoveTsOrDsToDate,
		exp.KindMonth: tsqlRemoveTsOrDsToDate,
		exp.KindYear:  tsqlRemoveTsOrDsToDate,
		exp.KindGeneratedAsIdentityColumnConstraint: func(g *Generator, e exp.Expression) string {
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
		exp.KindGroupConcat: tsqlStringAgg,
		exp.KindIf: func(g *Generator, e exp.Expression) string {
			return g.funcCall("IIF", []any{e.This(), e.Arg("true"), e.Arg("false")}, "(", ")", true)
		},
		exp.KindJSONExtract:       tsqlJSONExtract,
		exp.KindJSONExtractScalar: tsqlJSONExtract,
		exp.KindLastDay: func(g *Generator, e exp.Expression) string {
			return g.funcCall("EOMONTH", []any{e.This()}, "(", ")", true)
		},
		exp.KindLn:  rename("LOG"),
		exp.KindMax: rename("MAX"),
		exp.KindMin: rename("MIN"),
		exp.KindMD5: func(g *Generator, e exp.Expression) string {
			return g.funcCall("HASHBYTES", []any{exp.LiteralString("MD5"), e.This()}, "(", ")", true)
		},
		exp.KindNumberToStr: tsqlFormatSQL,
		exp.KindRepeat: func(g *Generator, e exp.Expression) string {
			return g.funcCall("REPLICATE", g.fallbackArgs(e), "(", ")", true)
		},
		exp.KindCurrentSchema: rename("SCHEMA_NAME"),
		exp.KindStddev:        rename("STDEV"),
		exp.KindStrPosition: func(g *Generator, e exp.Expression) string {
			return g.funcCall("CHARINDEX", []any{e.Arg("substr"), e.This(), e.Arg("position")}, "(", ")", true)
		},
		exp.KindSHA: func(g *Generator, e exp.Expression) string {
			return g.funcCall("HASHBYTES", []any{exp.LiteralString("SHA1"), e.This()}, "(", ")", true)
		},
		exp.KindSHA1Digest: func(g *Generator, e exp.Expression) string {
			return g.funcCall("HASHBYTES", []any{exp.LiteralString("SHA1"), e.This()}, "(", ")", true)
		},
		exp.KindSHA2: func(g *Generator, e exp.Expression) string {
			length := "256"
			if l := g.sqlKey(e, "length"); l != "" {
				length = l
			}
			return g.funcCall("HASHBYTES", []any{exp.LiteralString("SHA2_" + length), e.This()}, "(", ")", true)
		},
		exp.KindTemporaryProperty: func(g *Generator, e exp.Expression) string { return "" },
		exp.KindTimeStrToTime:     tsqlTimeStrToTime,
		exp.KindTimeToStr:         tsqlFormatSQL,
		exp.KindTimestampAdd:      tsqlDateDelta("DATEADD", false),
		exp.KindTsOrDsAdd:         tsqlDateDelta("DATEADD", true),
		exp.KindTsOrDsDiff:        tsqlDateDelta("DATEDIFF", false),
		exp.KindTimestampTrunc: func(g *Generator, e exp.Expression) string {
			unit := asExpression(e.Arg("unit"))
			if unit != nil {
				unit = exp.TimeUnitVar(unit)
			}
			return g.funcCall("DATETRUNC", []any{unit, e.This()}, "(", ")", true)
		},
		exp.KindTrunc: func(g *Generator, e exp.Expression) string {
			decimals := e.Arg("decimals")
			if decimals == nil {
				decimals = exp.LiteralNumber(0)
			}
			return g.funcCall("ROUND", []any{e.This(), decimals, exp.LiteralNumber(1)}, "(", ")", true)
		},
		exp.KindUuid:          func(g *Generator, e exp.Expression) string { return "NEWID()" },
		exp.KindDateFromParts: rename("DATEFROMPARTS"),
		exp.KindTrim:          (*Generator).tsqlTrimSQL,
		exp.KindTsOrDsToDate:  (*Generator).tsqlTsOrDsToDateSQL,
		exp.KindCreate:        (*Generator).tsqlCreateSQL,
		exp.KindInsert:        (*Generator).tsqlInsertSQL,
		exp.KindCTE:           (*Generator).tsqlCTESQL,
		exp.KindSubquery:      (*Generator).tsqlSubquerySQL,
		exp.KindAnd:           (*Generator).tsqlAndSQL,
		exp.KindOr:            (*Generator).tsqlOrSQL,
		exp.KindNot:           (*Generator).tsqlNotSQL,
		exp.KindWhere:         (*Generator).tsqlWhereSQL,
		exp.KindHaving:        (*Generator).tsqlHavingSQL,
		exp.KindJoin:          (*Generator).tsqlJoinSQL,

		// Dedicated methods (generators/tsql.py:264-655).
		exp.KindConvert:                      (*Generator).tsqlConvertSQL,
		exp.KindQueryOption:                  (*Generator).tsqlQueryOptionSQL,
		exp.KindLateral:                      (*Generator).tsqlLateralSQL,
		exp.KindForClause:                    (*Generator).tsqlForClauseSQL,
		exp.KindXMLKeyValueOption:            (*Generator).tsqlXMLKeyValueOptionSQL,
		exp.KindNextValueFor:                 (*Generator).tsqlNextValueForSQL,
		exp.KindScopeResolution:              (*Generator).tsqlScopeResolutionSQL,
		exp.KindAlterSet:                     (*Generator).tsqlAlterSetSQL,
		exp.KindClusteredColumnConstraint:    (*Generator).tsqlClusteredConstraintSQL,
		exp.KindNonClusteredColumnConstraint: (*Generator).tsqlNonClusteredConstraintSQL,
		exp.KindSplitPart:                    (*Generator).tsqlSplitPartSQL,
		exp.KindExtract:                      (*Generator).tsqlExtractSQL,
		exp.KindTimeFromParts:                (*Generator).tsqlTimeFromPartsSQL,
		exp.KindTimestampFromParts:           (*Generator).tsqlTimestampFromPartsSQL,
		exp.KindSetItem:                      (*Generator).tsqlSetItemSQL,
		exp.KindIs:                           (*Generator).tsqlIsSQL,
		exp.KindInto:                         (*Generator).tsqlIntoSQL,
		exp.KindCount:                        (*Generator).tsqlCountSQL,
		exp.KindDateDiff:                     (*Generator).tsqlDateDiffSQL,
		exp.KindOffset:                       (*Generator).tsqlOffsetSQL,
		exp.KindVersion:                      (*Generator).tsqlVersionSQL,
		exp.KindReturnsProperty:              (*Generator).tsqlReturnsPropertySQL,
		exp.KindReturning:                    (*Generator).tsqlReturningSQL,
		exp.KindTransaction:                  (*Generator).tsqlTransactionSQL,
		exp.KindCommit:                       (*Generator).tsqlCommitSQL,
		exp.KindRollback:                     (*Generator).tsqlRollbackSQL,
		exp.KindIdentifier:                   (*Generator).tsqlIdentifierSQL,
		exp.KindConstraint:                   (*Generator).tsqlConstraintSQL,
		exp.KindLength:                       func(g *Generator, e exp.Expression) string { return g.tsqlUncastText(e, "LEN") },
		exp.KindRight:                        func(g *Generator, e exp.Expression) string { return g.tsqlUncastText(e, "RIGHT") },
		exp.KindLeft:                         func(g *Generator, e exp.Expression) string { return g.tsqlUncastText(e, "LEFT") },
		exp.KindPartition:                    (*Generator).tsqlPartitionSQL,
		exp.KindAlter:                        (*Generator).tsqlAlterSQL,
		exp.KindDrop:                         (*Generator).tsqlDropSQL,
		exp.KindDPipe:                        (*Generator).tsqlDPipeSQL,
		exp.KindIsAscii:                      (*Generator).tsqlIsAsciiSQL,
		exp.KindColumnDef:                    (*Generator).tsqlColumnDefSQL,
		exp.KindCoalesce:                     (*Generator).tsqlCoalesceSQL,
		exp.KindStoredProcedure:              (*Generator).tsqlStoredProcedureSQL,
		exp.KindIfBlock:                      (*Generator).tsqlIfBlockSQL,
		exp.KindWhileBlock:                   (*Generator).tsqlWhileBlockSQL,
		exp.KindExecute:                      (*Generator).tsqlExecuteSQL,
		exp.KindExecuteSql:                   (*Generator).tsqlExecuteSQL,
		exp.KindDeclare:                      (*Generator).tsqlDeclareSQL,
		exp.KindDeclareItem:                  (*Generator).tsqlDeclareItemSQL,
	})

	registerDialectTypeMapping("tsql", map[exp.DType]string{
		exp.DTypeBoolean:       "BIT",
		exp.DTypeDatetime2:     "DATETIME2",
		exp.DTypeDecimal:       "NUMERIC",
		exp.DTypeDouble:        "FLOAT",
		exp.DTypeInt:           "INTEGER",
		exp.DTypeRowVersion:    "ROWVERSION",
		exp.DTypeText:          "VARCHAR(MAX)",
		exp.DTypeTimestamp:     "DATETIME2",
		exp.DTypeTimestampNtz:  "DATETIME2",
		exp.DTypeTimestampTz:   "DATETIMEOFFSET",
		exp.DTypeSmallDatetime: "SMALLDATETIME",
		exp.DTypeUTinyInt:      "TINYINT",
		exp.DTypeVariant:       "SQL_VARIANT",
		exp.DTypeUUID:          "UNIQUEIDENTIFIER",
	})
	registerDialectRemoveTypeMappings("tsql", exp.DTypeNChar, exp.DTypeNVarchar)

	registerDialectPropertyLocations("tsql", map[exp.Kind]propertyLocation{
		exp.KindVolatileProperty: propertyLocationUnsupported,
	})
}

// rename ports rename_func(name): the function of the same node renders with another name.
func rename(name string) func(*Generator, exp.Expression) string {
	return func(g *Generator, e exp.Expression) string {
		return g.funcCall(name, []any{e.This()}, "(", ")", true)
	}
}

// tsqlDateDelta ports date_delta_sql("NAME") (dialects/dialect.py:2014-2026).
func tsqlDateDelta(name string, cast bool) func(*Generator, exp.Expression) string {
	return func(g *Generator, e exp.Expression) string {
		unit := asExpression(e.Arg("unit"))
		if unit != nil {
			unit = exp.TimeUnitVar(unit)
		}
		return g.funcCall(name, []any{unit, e.Arg("expression"), e.This()}, "(", ")", true)
	}
}

// tsqlRemoveTsOrDsToDate ports remove_ts_or_ds_to_date() (dialects/dialect.py:1997-2009).
func tsqlRemoveTsOrDsToDate(g *Generator, e exp.Expression) string {
	this := e.This()
	if tsOrDs, ok := this.(*exp.Node); ok && tsOrDs.Kind() == exp.KindTsOrDsToDate && tsOrDs.Arg("format") == nil {
		e.Set("this", tsOrDs.This())
	}
	return g.functionFallbackSQL(e)
}

func tsqlJSONExtract(g *Generator, e exp.Expression) string {
	jsonQuery := g.funcCall("JSON_QUERY", []any{e.This(), e.Arg("expression")}, "(", ")", true)
	jsonValue := g.funcCall("JSON_VALUE", []any{e.This(), e.Arg("expression")}, "(", ")", true)
	return g.funcCall("ISNULL", []any{jsonQuery, jsonValue}, "(", ")", true)
}

func tsqlStringAgg(g *Generator, e exp.Expression) string {
	this := e.This()
	separator := e.Arg("separator")
	if separator == nil {
		separator = exp.LiteralString(",")
	}
	order := ""
	if this != nil && this.Kind() == exp.KindOrder {
		orderExpr := this
		if inner := this.This(); inner != nil {
			this = inner
			orderExpr.Set("this", nil)
		}
		order = " WITHIN GROUP (" + strings.TrimPrefix(g.gen(orderExpr), " ") + ")"
	}
	return "STRING_AGG(" + g.formatArgs([]any{this, separator}, ", ") + ")" + order
}

// tsqlFormatSQL ports _format_sql (generators/tsql.py:34-48).
func tsqlFormatSQL(g *Generator, e exp.Expression) string {
	format := e.Arg("format")
	formatSQL := g.gen(format)
	if e.Kind() != exp.KindNumberToStr && format != nil {
		if fe, ok := format.(*exp.Node); ok && fe.IsString() {
			formatSQL = g.gen(exp.LiteralString(dialects.TSQLFormatTime(fe.Name())))
		}
	}
	return g.funcCall("FORMAT", []any{e.This(), formatSQL, e.Arg("culture")}, "(", ")", true)
}

// tsqlTimeStrToTime ports _timestrtotime_sql (generators/tsql.py:111-118).
func tsqlTimeStrToTime(g *Generator, e exp.Expression) string {
	// Port of exp.cast's re-cast guard (expressions/builders.py:457-490): casting an
	// existing cast to the type it already maps to must not double-cast.
	if this := e.This(); this != nil && this.Kind() == exp.KindCast &&
		exp.DataTypeIsType(this, false, exp.DTypeTimestamp, exp.DTypeTimestampNtz, exp.DTypeDatetime2) {
		return g.gen(this)
	}
	dt := exp.DTypeTimestamp
	if e.Arg("zone") != nil {
		dt = exp.DTypeTimestampTz
	}
	sql := g.gen(exp.Cast(exp.Args{"this": e.This(), "to": dt.IntoExpr(nil)}))
	if e.Arg("zone") != nil {
		sql = g.gen(exp.AtTimeZone(exp.Args{"this": sql, "zone": exp.LiteralString("UTC")}))
	}
	return sql
}

func (g *Generator) tsqlConvertSQL(e exp.Expression) string {
	name := "CONVERT"
	if boolValue(e.Arg("safe")) {
		name = "TRY_CONVERT"
	}
	return g.funcCall(name, []any{e.This(), e.Arg("expression"), e.Arg("style")}, "(", ")", true)
}

func (g *Generator) tsqlQueryOptionSQL(e exp.Expression) string {
	option := g.sqlKey(e, "this")
	value := g.sqlKey(e, "expression")
	if value == "" {
		return option
	}
	optionalEqual := ""
	switch strings.ToUpper(option) {
	case "MAX_GRANT_PERCENT", "MIN_GRANT_PERCENT", "LABEL":
		optionalEqual = "= "
	}
	return option + " " + optionalEqual + value
}

// tsqlLateralSQL mirrors lateralSQL (generator.py:2911-2930) but renders the T-SQL
// CROSS APPLY / OUTER APPLY operator.
func (g *Generator) tsqlLateralSQL(e exp.Expression) string {
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
	return g.tsqlLateralOp(e) + " " + this + alias + ordinality
}

// tsqlForClauseSQL ports forclause_sql (generator.py:3310-3319).
// tsqlTsOrDsToDateSQL ports tsordstodate_sql (generator.py:5202-5216) for the
// format-less case TSQL produces.
func (g *Generator) tsqlTsOrDsToDateSQL(e exp.Expression) string {
	this := e.This()
	if this != nil && (this.Kind() == exp.KindTsOrDsToDate || exp.DataTypeIsType(this, false, exp.DTypeDate)) {
		return g.gen(this)
	}
	if this != nil && this.Kind() == exp.KindCast && exp.DataTypeIsType(this, false, exp.DTypeDate) {
		return g.gen(this)
	}
	return g.gen(exp.Cast(exp.Args{"this": this, "to": exp.DTypeDate.IntoExpr(nil)}))
}

// tsqlTrimSQL ports trim_sql (dialects/dialect.py:1822-1837): T-SQL keeps the
// TRIM([BOTH|LEADING|TRAILING] chars FROM target) form.
func (g *Generator) tsqlTrimSQL(e exp.Expression) string {
	removeChars := g.sqlKey(e, "expression")
	if removeChars == "" {
		return g.trimSQL(e)
	}
	target := g.sqlKey(e, "this")
	trimType := g.sqlKey(e, "position")
	collation := g.sqlKey(e, "collation")
	trimType = strings.TrimSpace(trimType)
	if trimType != "" {
		trimType += " "
	}
	removeChars += " "
	fromPart := "FROM "
	collationSQL := ""
	if collation != "" {
		collationSQL = " COLLATE " + collation
	}
	return "TRIM(" + trimType + removeChars + fromPart + target + collationSQL + ")"
}

// tsqlCreateSQL ports the CREATE OR ALTER rewrite (generators/tsql.py:475-478).
func (g *Generator) tsqlCreateSQL(e exp.Expression) string {
	// generators/tsql.py:420-421: CREATE VIEW drops the catalog qualifier.
	if strings.EqualFold(e.Text("kind"), "VIEW") {
		if table := e.Find(exp.KindTable); table != nil {
			table.Set("catalog", nil)
		}
	}
	// generators/tsql.py:438-456: CREATE TABLE ... AS SELECT becomes SELECT ... INTO.
	if strings.EqualFold(e.Text("kind"), "TABLE") {
		if ctas := asExpression(e.Arg("expression")); ctas != nil {
			if sql, ok := g.tsqlCTAS(e, ctas); ok {
				return sql
			}
		}
	}
	sql := g.createSQL(e)

	if truthy(e.Arg("replace")) {
		sql = strings.Replace(sql, "CREATE OR REPLACE ", "CREATE OR ALTER ", 1)
	}
	return sql
}

// ensureBoolOperand ports transforms.ensure_bools' _ensure_bool (transforms.py:711-729) for
// the TSQL generator: a numeric/unknown-typed operand of a condition is wrapped in <> 0.
func (g *Generator) ensureBoolOperand(e exp.Expression) string {
	if e == nil {
		return ""
	}
	if boolExpressionKinds[e.Kind()] {
		return g.gen(e)
	}
	return g.gen(exp.NEQ(exp.Args{"this": e, "expression": exp.LiteralNumber(0)}))
}

var boolExpressionKinds = map[exp.Kind]bool{
	exp.KindAnd: true, exp.KindOr: true, exp.KindNot: true,
	exp.KindEQ: true, exp.KindNEQ: true, exp.KindGT: true, exp.KindGTE: true,
	exp.KindLT: true, exp.KindLTE: true, exp.KindIs: true, exp.KindLike: true,
	exp.KindILike: true, exp.KindIn: true, exp.KindBetween: true, exp.KindExists: true,
	exp.KindBoolean: true, exp.KindNullSafeEQ: true, exp.KindSimilarTo: true,
	// ensure_bools leaves an already-parenthesized condition alone in practice.
	exp.KindParen: true,
}

func (g *Generator) tsqlAndSQL(e exp.Expression) string {
	// generators/tsql.py:137 sets ENSURE_BOOLS; upstream applies it as an AST preprocess
	// (transforms.ensure_bools) and then renders through the base and_sql -> connector_sql.
	// This port renders without the rewrite, so the ensure-bool wrap lives at the
	// connectorSQL leaves (see there) and this delegates to connector_sql for the same
	// flatten + pretty too_wide line breaking (generator.py:4073-4099).
	return g.connectorSQL(e, "AND", nil)
}

func (g *Generator) tsqlOrSQL(e exp.Expression) string {
	// See tsqlAndSQL: OR is the same connector path with its own operator.
	return g.connectorSQL(e, "OR", nil)
}

func (g *Generator) tsqlNotSQL(e exp.Expression) string {
	return "NOT " + g.ensureBoolOperand(e.This())
}

func (g *Generator) tsqlWhereSQL(e exp.Expression) string {
	// Base where_sql applies self.indent(...) to the condition (generator.py:3508-3510);
	// keep that around the ensure-bool rewrite so pretty output stays aligned.
	this := g.indent(g.ensureBoolOperand(e.This()), 0, nil, false, false)
	return g.seg("WHERE") + g.sep() + this
}

func (g *Generator) tsqlHavingSQL(e exp.Expression) string {
	// Same indent as base having_sql (generator.py:2826-2828).
	this := g.indent(g.ensureBoolOperand(e.This()), 0, nil, false, false)
	return g.seg("HAVING") + g.sep() + this
}

func (g *Generator) tsqlJoinSQL(e exp.Expression) string {
	// Only the ON condition needs boolean coercion; reuse the base renderer afterwards by
	// temporarily wrapping it.
	if on := asExpression(e.Arg("on")); on != nil && !boolExpressionKinds[on.Kind()] {
		e.Set("on", exp.NEQ(exp.Args{"this": on, "expression": exp.LiteralNumber(0)}))
	}
	return g.joinSQL(e)
}

// tsqlInsertSQL ports the CTE hoist for INSERT ... WITH (generators/tsql.py's
// EXPRESSIONS_WITHOUT_NESTED_CTES + move_ctes_to_top_level).
func (g *Generator) tsqlInsertSQL(e exp.Expression) string {
	if child := asExpression(e.Arg("expression")); child != nil {
		if with := asExpression(child.Arg("with_")); with != nil {
			e.Set("with_", with.Pop())
			child.Set("with_", nil)
		}
	}
	return g.insertSQL(e)
}

// tsqlCTAS ports the CTAS -> SELECT INTO rewrite (generators/tsql.py:438-456).
func (g *Generator) tsqlCTAS(e exp.Expression, ctas exp.Expression) (string, bool) {
	table := e.Find(exp.KindTable)
	if table == nil {
		return "", false
	}
	if isUnwrappedQuery(ctas.Kind()) {
		ctas = exp.Subquery(exp.Args{"this": ctas})
	}
	isTemp := false
	if props, ok := e.Arg("properties").([]exp.Expression); ok {
		_ = props
	}
	if props := asExpression(e.Arg("properties")); props != nil {
		for _, p := range props.Expressions() {
			if p.Kind() == exp.KindTemporaryProperty {
				isTemp = true
			}
		}
	}
	source := ctas
	if isUnwrappedQuery(ctas.Kind()) {
		source = exp.Subquery(exp.Args{"this": ctas})
	}
	source.Set("alias", exp.TableAlias(exp.Args{"this": exp.Identifier(exp.Args{"this": "temp"})}))
	sel := exp.Select(exp.Args{
		"expressions": []exp.Expression{exp.Star(nil)},
		"from_":       exp.From(exp.Args{"this": source}),
		"into":        exp.Into(exp.Args{"this": table, "temporary": isTemp}),
	})
	return g.gen(sel), true
}

// tsqlAlterSetSQL ports the T-SQL ALTER TABLE ... SET property style: `KEY=value`
// without spaces (property nodes render `KEY = value` in the base generator).
func (g *Generator) tsqlAlterSetSQL(e exp.Expression) string {
	exprs := g.expressions(exprsOptions{expression: e, flat: true})
	if boolValue(e.Meta()["tsql_no_space"]) {
		exprs = strings.ReplaceAll(exprs, " = ", "=")
		// FILESTREAM_ON is an ordinary EQ upstream, not a named property: keep its spaces.
		exprs = strings.Replace(exprs, "FILESTREAM_ON=", "FILESTREAM_ON = ", 1)
	}
	if g.dialect.AlterSetWrapped {
		exprs = "(" + exprs + ")"
	}
	return "SET " + exprs
}

// tsqlScopeResolutionSQL ports scoperesolution_sql (generator.py:5536-5546): a CLR
// function on the RHS keeps its case (T-SQL CLR names are case sensitive).
func (g *Generator) tsqlScopeResolutionSQL(e exp.Expression) string {
	this := g.sqlKey(e, "this")
	expr := asExpression(e.Arg("expression"))
	if expr != nil && expr.Is(exp.TraitFunc) {
		name := expr.Text("this")
		if name == "" {
			name = g.sqlName(expr.Kind())
		}
		return this + "::" + name + "(" + g.formatArgs(listFromValue(expr.Arg("expressions")), ", ") + ")"
	}
	return this + "::" + g.sqlKey(e, "expression")
}

func (g *Generator) tsqlNextValueForSQL(e exp.Expression) string {
	sql := "NEXT VALUE FOR " + g.sqlKey(e, "this")
	if order := g.sqlKey(e, "order"); order != "" {
		sql += " OVER (" + order + ")"
	}
	return sql
}

func (g *Generator) tsqlClusteredConstraintSQL(e exp.Expression) string {
	return "CLUSTERED (" + g.expressions(exprsOptions{expression: e, key: "this", flat: true}) + ")"
}

func (g *Generator) tsqlNonClusteredConstraintSQL(e exp.Expression) string {
	return "NONCLUSTERED (" + g.expressions(exprsOptions{expression: e, key: "this", flat: true}) + ")"
}

func (g *Generator) tsqlXMLKeyValueOptionSQL(e exp.Expression) string {
	this := g.sqlKey(e, "this")
	if expression := g.sqlKey(e, "expression"); expression != "" {
		return this + "(" + expression + ")"
	}
	return this
}

func (g *Generator) tsqlForClauseSQL(e exp.Expression) string {
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

func tsqlSeqGetExpr(args []exp.Expression, i int) exp.Expression {
	if i < 0 || i >= len(args) {
		return nil
	}
	return args[i]
}

// tsqlQualifyDerivedOutputs ports qualify_derived_table_outputs
// (generators/tsql.py:70-100): an unaliased projection of a CTE/derived table gets an alias.
func tsqlQualifyDerivedOutputs(e exp.Expression) {
	alias := asExpression(e.Arg("alias"))
	if alias == nil || alias.Kind() != exp.KindTableAlias {
		return
	}
	if columns := alias.Arg("columns"); columns != nil {
		if list, ok := columns.([]exp.Expression); ok && len(list) > 0 {
			return
		}
	}
	query := asExpression(e.Arg("this"))
	if query == nil {
		return
	}
	selects := query.Arg("expressions")
	list, ok := selects.([]exp.Expression)
	if !ok {
		return
	}
	for i, projection := range list {
		if projection == nil || projection.Kind() == exp.KindAlias {
			continue
		}
		if projection.Kind() != exp.KindColumn && projection.Kind() != exp.KindLiteral {
			continue
		}
		name := projection.Name()
		if name == "" {
			continue
		}
		quoted := false
		if id := asExpression(projection.This()); id != nil && id.Kind() == exp.KindIdentifier {
			quoted = boolValue(id.Arg("quoted"))
		}
		list[i] = exp.AliasExpr(projection, exp.Identifier(exp.Args{"this": name, "quoted": quoted}), false)
	}
	query.Set("expressions", list)
}

func (g *Generator) tsqlCTESQL(e exp.Expression) string {
	tsqlQualifyDerivedOutputs(e)
	return g.cteSQL(e)
}

func (g *Generator) tsqlSubquerySQL(e exp.Expression) string {
	tsqlQualifyDerivedOutputs(e)
	return g.subquerySQL(e)
}

// tsqlVersionSQL ports version_sql (generators/tsql.py:501-514).
func (g *Generator) tsqlVersionSQL(e exp.Expression) string {
	name := e.Name()
	if name == "TIMESTAMP" {
		name = "SYSTEM_TIME"
	}
	kind := e.Text("kind")
	expr := asExpression(e.Arg("expression"))
	exprSQL := ""
	if kind == "FROM" || kind == "BETWEEN" {
		var args []exp.Expression
		if expr != nil {
			if list, ok := expr.Arg("expressions").([]exp.Expression); ok {
				args = list
			}
		}
		sep := "TO"
		if kind == "BETWEEN" {
			sep = "AND"
		}
		exprSQL = g.gen(tsqlSeqGetExpr(args, 0)) + " " + sep + " " + g.gen(tsqlSeqGetExpr(args, 1))
	} else if expr != nil {
		exprSQL = g.gen(expr)
	}
	if exprSQL != "" {
		exprSQL = " " + exprSQL
	}
	return "FOR " + name + " " + kind + exprSQL
}

// tsqlIfBlockSQL / tsqlWhileBlockSQL port generators/tsql.py:632-644. The body Block
// already carries its own terminating END statement.
func (g *Generator) tsqlIfBlockSQL(e exp.Expression) string {
	this := g.sqlKey(e, "this")
	trueSQL := g.sqlKey(e, "true")
	if trueSQL != "" {
		trueSQL = " " + trueSQL
	} else {
		trueSQL = " "
	}
	falseSQL := g.sqlKey(e, "false")
	if falseSQL != "" {
		falseSQL = "; ELSE BEGIN " + falseSQL
	}
	return "IF " + this + " BEGIN" + trueSQL + falseSQL
}

func (g *Generator) tsqlWhileBlockSQL(e exp.Expression) string {
	this := g.sqlKey(e, "this")
	body := g.sqlKey(e, "body")
	if body != "" {
		body = " " + body
	} else {
		body = " "
	}
	return "WHILE " + this + " BEGIN" + body
}

func (g *Generator) tsqlLateralOp(e exp.Expression) string {
	switch v := e.Arg("cross_apply").(type) {
	case bool:
		if v {
			return "CROSS APPLY"
		}
		return "OUTER APPLY"
	}
	return "LATERAL"
}

func (g *Generator) tsqlSplitPartSQL(e exp.Expression) string {
	this := e.This()
	delimiter := asExpression(e.Arg("delimiter"))
	partIndex := asExpression(e.Arg("part_index"))
	if this == nil || this.Kind() != exp.KindLiteral || delimiter == nil ||
		delimiter.Kind() != exp.KindLiteral || delimiter.Name() != "." ||
		partIndex == nil || partIndex.Kind() != exp.KindLiteral {
		return ""
	}
	splitCount := len(strings.Split(this.Name(), "."))
	index, err := strconv.Atoi(partIndex.Name())
	if err != nil {
		return ""
	}
	return g.funcCall("PARSENAME", []any{this, exp.LiteralNumber(splitCount + 1 - index)}, "(", ")", true)
}

func (g *Generator) tsqlExtractSQL(e exp.Expression) string {
	part := e.This()
	name := part
	if part != nil {
		switch strings.ToUpper(part.Name()) {
		case "WEEKISO":
			name = exp.Var(exp.Args{"this": "ISO_WEEK"})
		case "DAYOFWEEK":
			name = exp.Var(exp.Args{"this": "WEEKDAY"})
		case "TIMEZONE_MINUTE":
			name = exp.Var(exp.Args{"this": "TZOFFSET"})
		}
	}
	return g.funcCall("DATEPART", []any{name, e.Arg("expression")}, "(", ")", true)
}

func (g *Generator) tsqlTimeFromPartsSQL(e exp.Expression) string {
	if e.Arg("nano") != nil {
		e.Set("nano", nil)
	}
	if e.Arg("fractions") == nil {
		e.Set("fractions", exp.LiteralNumber(0))
	}
	if e.Arg("precision") == nil {
		e.Set("precision", exp.LiteralNumber(0))
	}
	return g.funcCall("TIMEFROMPARTS", []any{
		e.Arg("hour"), e.Arg("min"), e.Arg("sec"), e.Arg("fractions"), e.Arg("precision"),
	}, "(", ")", true)
}

func (g *Generator) tsqlTimestampFromPartsSQL(e exp.Expression) string {
	if e.Arg("zone") != nil {
		e.Set("zone", nil)
	}
	if e.Arg("nano") != nil {
		e.Set("nano", nil)
	}
	if e.Arg("milli") == nil {
		e.Set("milli", exp.LiteralNumber(0))
	}
	return g.funcCall("DATETIMEFROMPARTS", []any{
		e.Arg("year"), e.Arg("month"), e.Arg("day"), e.Arg("hour"),
		e.Arg("min"), e.Arg("sec"), e.Arg("milli"),
	}, "(", ")", true)
}

func (g *Generator) tsqlSetItemSQL(e exp.Expression) string {
	this := e.This()
	if this != nil && this.Kind() == exp.KindEQ {
		left := asExpression(this.Arg("this"))
		if left == nil || left.Kind() != exp.KindParameter {
			return g.gen(left) + " " + g.gen(this.Arg("expression"))
		}
	}
	return g.setItemSQL(e)
}

func (g *Generator) tsqlIsSQL(e exp.Expression) string {
	if expr := asExpression(e.Arg("expression")); expr != nil && expr.Kind() == exp.KindBoolean {
		if boolValue(e.Arg("negate")) {
			return g.binary(e, "<>")
		}
		return g.binary(e, "=")
	}
	if boolValue(e.Arg("negate")) {
		return g.binary(e, "IS NOT")
	}
	return g.binary(e, "IS")
}

func (g *Generator) tsqlIntoSQL(e exp.Expression) string {
	if boolValue(e.Arg("temporary")) {
		if table := e.Find(exp.KindTable); table != nil {
			if id := table.This(); id != nil && id.Kind() == exp.KindIdentifier {
				id.Set("temporary", true)
			}
		}
	}
	return g.seg("INTO") + " " + g.sqlKey(e, "this")
}

func (g *Generator) tsqlCountSQL(e exp.Expression) string {
	name := "COUNT"
	if boolValue(e.Arg("big_int")) {
		name = "COUNT_BIG"
	}
	return g.funcCall(name, g.fallbackArgs(e), "(", ")", true)
}

func (g *Generator) tsqlDateDiffSQL(e exp.Expression) string {
	name := "DATEDIFF"
	if boolValue(e.Arg("big_int")) {
		name = "DATEDIFF_BIG"
	}
	unit := asExpression(e.Arg("unit"))
	if unit != nil {
		unit = exp.TimeUnitVar(unit)
	}
	return g.funcCall(name, []any{unit, e.Arg("expression"), e.This()}, "(", ")", true)
}

func (g *Generator) tsqlOffsetSQL(e exp.Expression) string {
	return g.offsetSQL(e) + " ROWS"
}

func (g *Generator) tsqlReturnsPropertySQL(e exp.Expression) string {
	table := g.sqlKey(e, "table")
	if table != "" {
		table += " "
	}
	return "RETURNS " + table + g.sqlKey(e, "this")
}

func (g *Generator) tsqlReturningSQL(e exp.Expression) string {
	into := g.sqlKey(e, "into")
	intoSql := ""
	if into != "" {
		intoSql = g.seg("INTO " + into)
	}
	return g.seg("OUTPUT") + " " + g.expressions(exprsOptions{expression: e, flat: true}) + intoSql
}

func (g *Generator) tsqlTransactionSQL(e exp.Expression) string {
	this := g.sqlKey(e, "this")
	if this != "" {
		this = " " + this
	}
	mark := g.sqlKey(e, "mark")
	if mark != "" {
		mark = " WITH MARK " + mark
	}
	return "BEGIN TRANSACTION" + this + mark
}

func (g *Generator) tsqlCommitSQL(e exp.Expression) string {
	this := g.sqlKey(e, "this")
	if this != "" {
		this = " " + this
	}
	durability := ""
	if d, ok := e.Arg("durability").(bool); ok {
		on := "OFF"
		if d {
			on = "ON"
		}
		durability = " WITH (DELAYED_DURABILITY = " + on + ")"
	}
	return "COMMIT TRANSACTION" + this + durability
}

func (g *Generator) tsqlRollbackSQL(e exp.Expression) string {
	this := g.sqlKey(e, "this")
	if this != "" {
		this = " " + this
	}
	return "ROLLBACK TRANSACTION" + this
}

func (g *Generator) tsqlIdentifierSQL(e exp.Expression) string {
	identifier := g.identifierSQL(e)
	prefix := ""
	if boolValue(e.Arg("global_")) {
		prefix = "##"
	} else if boolValue(e.Arg("temporary")) {
		prefix = "#"
	} else {
		return identifier
	}
	start := g.identifierStart
	if boolValue(e.Arg("quoted")) && strings.HasPrefix(identifier, start) {
		return start + prefix + identifier[len(start):]
	}
	return prefix + identifier
}

func (g *Generator) tsqlConstraintSQL(e exp.Expression) string {
	this := g.sqlKey(e, "this")
	expressionsSQL := g.expressions(exprsOptions{expression: e, flat: true, sep: " "})
	return "CONSTRAINT " + this + " " + expressionsSQL
}

func (g *Generator) tsqlUncastText(e exp.Expression, name string) string {
	this := e.This()
	thisSQL := ""
	if this != nil && this.Kind() == exp.KindCast && exp.DataTypeIsType(this, false, exp.DTypeText) {
		thisSQL = g.sqlKey(this, "this")
	} else {
		thisSQL = g.gen(this)
	}
	expressionSQL := g.sqlKey(e, "expression")
	args := []any{thisSQL}
	if expressionSQL != "" {
		args = append(args, expressionSQL)
	}
	return g.funcCall(name, args, "(", ")", true)
}

func (g *Generator) tsqlPartitionSQL(e exp.Expression) string {
	return "WITH (PARTITIONS(" + g.expressions(exprsOptions{expression: e, flat: true}) + "))"
}

func (g *Generator) tsqlAlterSQL(e exp.Expression) string {
	actions, _ := e.Arg("actions").([]exp.Expression)
	if len(actions) > 0 && actions[0] != nil && actions[0].Kind() == exp.KindAlterRename {
		return "EXEC sp_rename '" + g.sqlKey(e, "this") + "', '" + actions[0].Name() + "'"
	}
	return g.alterSQL(e)
}

func (g *Generator) tsqlDropSQL(e exp.Expression) string {
	if stringValue(e.Arg("kind")) == "VIEW" {
		if table := e.Find(exp.KindTable); table != nil {
			table.Set("catalog", nil)
		}
	}
	return g.dropSQL(e)
}

func (g *Generator) tsqlDPipeSQL(e exp.Expression) string {
	parts := flattenDPipe(e)
	if len(parts) < 2 {
		return g.dpipeSQL(e)
	}
	var rendered []string
	for _, part := range parts {
		rendered = append(rendered, g.gen(part))
	}
	return strings.Join(rendered, " + ")
}

// flattenDPipe ports exp.DPipe.flatten: the left-deep DPipe chain as a flat operand list.
func flattenDPipe(e exp.Expression) []exp.Expression {
	if e == nil || e.Kind() != exp.KindDPipe {
		return nil
	}
	left := flattenDPipe(e.This())
	if left == nil {
		left = []exp.Expression{e.This()}
	}
	return append(left, asExpression(e.Arg("expression")))
}

func (g *Generator) tsqlIsAsciiSQL(e exp.Expression) string {
	return "(PATINDEX(CONVERT(VARCHAR(MAX), 0x255b5e002d7f5d25) COLLATE Latin1_General_BIN, " +
		g.gen(e.This()) + ") = 0)"
}

func (g *Generator) tsqlColumnDefSQL(e exp.Expression) string {
	this := g.columnDefSQL(e)
	if dflt := g.sqlKey(e, "default"); dflt != "" {
		this += " = " + dflt
	}
	if output := g.sqlKey(e, "output"); output != "" {
		this += " " + output
	}
	return this
}

func (g *Generator) tsqlCoalesceSQL(e exp.Expression) string {
	name := "COALESCE"
	if boolValue(e.Arg("is_null")) {
		name = "ISNULL"
	}
	return g.funcCall(name, g.fallbackArgs(e), "(", ")", true)
}

func (g *Generator) tsqlStoredProcedureSQL(e exp.Expression) string {
	this := g.sqlKey(e, "this")
	expressionsSQL := g.expressions(exprsOptions{expression: e})
	if boolValue(e.Arg("wrapped")) {
		expressionsSQL = g.wrap(expressionsSQL)
	} else if expressionsSQL != "" {
		expressionsSQL = " " + expressionsSQL
	}
	if strings.TrimSpace(expressionsSQL) == "" {
		return this
	}
	return this + expressionsSQL
}

func (g *Generator) tsqlExecuteSQL(e exp.Expression) string {
	this := g.sqlKey(e, "this")
	expressionsSQL := g.expressions(exprsOptions{expression: e})
	if expressionsSQL != "" {
		expressionsSQL = " " + expressionsSQL
	}
	returnStatus := g.sqlKey(e, "return_status")
	if returnStatus != "" {
		returnStatus += " = "
	}
	return "EXECUTE " + returnStatus + this + expressionsSQL
}

func (g *Generator) tsqlDeclareSQL(e exp.Expression) string {
	replace := ""
	if truthy(e.Arg("replace")) {
		replace = "OR REPLACE "
	}
	return "DECLARE " + replace + g.expressions(exprsOptions{expression: e, flat: true})
}

func (g *Generator) tsqlDeclareItemSQL(e exp.Expression) string {
	sql := g.expressions(exprsOptions{expression: e, key: "this", flat: true})
	if kind := e.Arg("kind"); kind != nil {
		if k, ok := kind.(*exp.Node); ok && k.Kind() == exp.KindSchema {
			sql += " TABLE"
		}
		sql += " " + g.gen(kind)
	}
	if dflt := g.sqlKey(e, "default"); dflt != "" {
		sql += " = " + dflt
	}
	return sql
}
