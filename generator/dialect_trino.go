package generator

import (
	"strings"

	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// generators/trino.py:22-53.
func init() {
	registerDialectDispatch("trino", map[exp.Kind]func(*Generator, exp.Expression) string{
		exp.KindCurrentVersion:   prestoRename("VERSION"),
		exp.KindGroupConcat:      (*Generator).trinoGroupConcatSQL,
		exp.KindTrim:             (*Generator).trimSQLStandard,
		exp.KindLocationProperty: func(g *Generator, e exp.Expression) string { return "LOCATION=" + g.sqlKey(e, "this") },
		exp.KindJSONExtract:      (*Generator).trinoJSONExtractSQL,
		exp.KindArrayUniqueAgg:   func(g *Generator, e exp.Expression) string { return "ARRAY_AGG(DISTINCT " + g.sqlKey(e, "this") + ")" },
		exp.KindDeclare: func(g *Generator, e exp.Expression) string {
			replace := ""
			if truthy(e.Arg("replace")) {
				replace = "OR REPLACE "
			}
			return "DECLARE " + replace + g.expressions(exprsOptions{expression: e, flat: true})
		},
		// generator.py:6078-6090 with Trino's DECLARE_DEFAULT_ASSIGNMENT = "DEFAULT".
		exp.KindDeclareItem: func(g *Generator, e exp.Expression) string {
			sql := g.expressions(exprsOptions{expression: e, key: "this", flat: true})
			if kind := asExpression(e.Arg("kind")); kind != nil {
				if kind.Kind() == exp.KindSchema {
					sql += " TABLE"
				}
				sql += " " + g.gen(kind)
			}
			if dflt := g.sqlKey(e, "default"); dflt != "" {
				sql += " DEFAULT " + dflt
			}
			return sql
		},
		exp.KindStabilityProperty: func(_ *Generator, e exp.Expression) string {
			if e.Name() == "IMMUTABLE" {
				return "DETERMINISTIC"
			}
			return "NOT DETERMINISTIC"
		},
		// Inline SQL UDFs (generators/trino.py:55-125).
		exp.KindFunctionSpecification: (*Generator).trinoFunctionSpecificationSQL,
		exp.KindBlock:                 (*Generator).trinoBlockSQL,
		exp.KindIfBlock:               (*Generator).trinoIfBlockSQL,
		exp.KindCaseStatement:         (*Generator).trinoCaseStatementSQL,
		exp.KindWhileBlock:            (*Generator).trinoWhileBlockSQL,
		exp.KindLoopBlock:             (*Generator).trinoLoopBlockSQL,
		exp.KindRepeatBlock:           (*Generator).trinoRepeatBlockSQL,
		exp.KindLeave:                 (*Generator).trinoLeaveSQL,
		exp.KindIterate:               (*Generator).trinoIterateSQL,
	})
	registerDialectPropertyLocations("trino", map[exp.Kind]propertyLocation{exp.KindLocationProperty: propertyLocationPostWith})
}

// dialects/dialect.py:2539-2587.
func (g *Generator) trinoGroupConcatSQL(e exp.Expression) string {
	e = e.Copy()
	this := e.This()
	separator := g.sqlKey(e, "separator")
	if separator == "" {
		separator = "','"
	}
	if overflow := g.sqlKey(e, "on_overflow"); overflow != "" {
		separator += " ON OVERFLOW " + overflow
	}
	var limit exp.Expression
	if this != nil && this.Kind() == exp.KindLimit && this.This() != nil {
		limit = this
		this = this.This().Pop()
	}
	var order exp.Expression
	if this != nil {
		order = this.Find(exp.KindOrder)
	}
	if order != nil && order.This() != nil {
		this = order.This().Pop()
	}
	args := g.formatArgs([]any{this, separator}, ", ")
	if limit != nil {
		args += g.gen(limit)
	}
	sql := g.funcCall("LISTAGG", []any{args}, "(", ")", true)
	if order != nil {
		sql += " WITHIN GROUP (" + strings.TrimSpace(g.gen(order)) + ")"
	}
	return sql
}

// generators/trino.py:135-156.
func (g *Generator) trinoJSONExtractSQL(e exp.Expression) string {
	if !boolValue(e.Arg("json_query")) {
		args := []any{e.Arg("this"), e.Arg("expression")}
		for _, x := range e.Expressions() {
			args = append(args, x)
		}
		return g.funcCall("JSON_EXTRACT", args, "(", ")", true)
	}
	path := g.sqlKey(e, "expression")
	for _, key := range []string{"option", "quote", "on_condition"} {
		if value := g.sqlKey(e, key); value != "" {
			path += " " + value
		}
	}
	return g.funcCall("JSON_QUERY", []any{e.Arg("this"), path}, "(", ")", true)
}

// trinoFunctionSpecificationSQL ports functionspecification_sql (generators/trino.py:55-65).
func (g *Generator) trinoFunctionSpecificationSQL(e exp.Expression) string {
	characteristics := asExpression(e.Arg("characteristics"))
	characteristicsSQL := ""
	if characteristics != nil {
		// generator.py:2075-2086 properties(): a blank prefix contributes no separator.
		characteristicsSQL = g.renderProperties(characteristics, " ", " ", "", false)
	}
	propertiesSQL := ""
	if properties := asExpression(e.Arg("properties")); properties != nil {
		propertiesSQL = " " + g.withPropertiesSQL(properties)
	}
	return "FUNCTION " + g.sqlKey(e, "this") + characteristicsSQL + propertiesSQL + " " + g.sqlKey(e, "expression")
}

// trinoBlockSQL ports block_sql (generator.py:6301-6304): `BEGIN ` when begin is set, with the
// statements joined by `; `. The trailing EndStatement child renders the closing END.
func (g *Generator) trinoBlockSQL(e exp.Expression) string {
	sql := g.expressions(exprsOptions{expression: e, sep: "; ", flat: true})
	if boolValue(e.Arg("begin")) {
		return "BEGIN " + sql
	}
	return sql
}

// trinoIfBlockSQL ports ifblock_sql (generators/trino.py:67-83). ELSEIF chains are nested into
// `false` at parse time, so this flattens them back out.
func (g *Generator) trinoIfBlockSQL(e exp.Expression) string {
	branches := []string{}
	node := e
	for node != nil && node.Kind() == exp.KindIfBlock {
		keyword := "IF"
		if len(branches) > 0 {
			keyword = "ELSEIF"
		}
		branches = append(branches, keyword+" "+g.sqlKey(node, "this")+" THEN "+g.sqlKey(node, "true")+";")
		node = asExpression(node.Arg("false"))
	}
	if node != nil {
		branches = append(branches, "ELSE "+g.gen(node)+";")
	}
	return strings.Join(branches, " ") + " END IF"
}

// trinoCaseStatementSQL ports casestatement_sql (generators/trino.py:85-99).
func (g *Generator) trinoCaseStatementSQL(e exp.Expression) string {
	branches := []string{"CASE"}
	if this := g.sqlKey(e, "this"); this != "" {
		branches[0] = "CASE " + this
	}
	for _, item := range listFromValue(e.Arg("ifs")) {
		node := asExpression(item)
		if node == nil {
			continue
		}
		branches = append(branches, "WHEN "+g.sqlKey(node, "this")+" THEN "+g.sqlKey(node, "true")+";")
	}
	if dflt := asExpression(e.Arg("default")); dflt != nil {
		branches = append(branches, "ELSE "+g.gen(dflt)+";")
	}
	branches = append(branches, "END CASE")
	return strings.Join(branches, " ")
}

// trinoLabelSQL renders the optional `label: ` prefix shared by the loop blocks.
func (g *Generator) trinoLabelSQL(e exp.Expression) string {
	if label := asExpression(e.Arg("label")); label != nil {
		return g.gen(label) + ": "
	}
	return ""
}

// trinoWhileBlockSQL ports whileblock_sql (generators/trino.py:101-106).
func (g *Generator) trinoWhileBlockSQL(e exp.Expression) string {
	return g.trinoLabelSQL(e) + "WHILE " + g.sqlKey(e, "this") + " DO " + g.sqlKey(e, "body") + "; END WHILE"
}

// trinoLoopBlockSQL ports loopblock_sql (generators/trino.py:108-112).
func (g *Generator) trinoLoopBlockSQL(e exp.Expression) string {
	return g.trinoLabelSQL(e) + "LOOP " + g.sqlKey(e, "body") + "; END LOOP"
}

// trinoRepeatBlockSQL ports repeatblock_sql (generators/trino.py:114-119).
func (g *Generator) trinoRepeatBlockSQL(e exp.Expression) string {
	return g.trinoLabelSQL(e) + "REPEAT " + g.sqlKey(e, "body") + "; UNTIL " + g.sqlKey(e, "until") + " END REPEAT"
}

// trinoLeaveSQL ports leave_sql (generators/trino.py:121-122).
func (g *Generator) trinoLeaveSQL(e exp.Expression) string {
	return "LEAVE " + g.sqlKey(e, "this")
}

// trinoIterateSQL ports iterate_sql (generators/trino.py:124-125).
func (g *Generator) trinoIterateSQL(e exp.Expression) string {
	return "ITERATE " + g.sqlKey(e, "this")
}
