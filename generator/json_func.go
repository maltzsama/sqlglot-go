package generator

import "github.com/maltzsama/sqlglot-go/expressions"

func init() {
	dispatch[expressions.KindJSONObject] = (*Generator).jsonObjectSQL
	dispatch[expressions.KindJSONObjectAgg] = (*Generator).jsonObjectSQL
	dispatch[expressions.KindJSONKeyValue] = (*Generator).jsonKeyValueSQL
	dispatch[expressions.KindOnCondition] = (*Generator).onConditionSQL
	dispatch[expressions.KindJSONArray] = (*Generator).jsonArraySQL
	dispatch[expressions.KindJSONExists] = (*Generator).jsonExistsSQL
	dispatch[expressions.KindJSONValue] = (*Generator).jsonValueSQL
}

// jsonObjectSQL ports _jsonobject_sql (generator.py:3788-3812), shared by JSON_OBJECT and
// JSON_OBJECTAGG (registered separately in dispatch above, matching upstream's TRANSFORMS
// entries at generator.py:210-211 - both map to the same private method).
func (g *Generator) jsonObjectSQL(e expressions.Expression) string {
	return g.jsonObjectSQLNamed(e, "")
}

// jsonObjectSQLNamed ports the name parameter of _jsonobject_sql (generator.py:3834): a
// dialect override can rename the call, e.g. sqlite's JSON_GROUP_OBJECT.
func (g *Generator) jsonObjectSQLNamed(e expressions.Expression, name string) string {
	nullHandling := g.sqlKey(e, "null_handling")
	if nullHandling != "" {
		nullHandling = " " + nullHandling
	}

	var uniqueKeys string
	if v, ok := e.Arg("unique_keys").(bool); ok {
		if v {
			uniqueKeys = " WITH UNIQUE KEYS"
		} else {
			uniqueKeys = " WITHOUT UNIQUE KEYS"
		}
	}

	returnType := g.sqlKey(e, "return_type")
	if returnType != "" {
		returnType = " RETURNING " + returnType
	}
	encoding := g.sqlKey(e, "encoding")
	if encoding != "" {
		encoding = " ENCODING " + encoding
	}

	if name == "" {
		name = "JSON_OBJECT"
		if e.Kind() == expressions.KindJSONObjectAgg {
			name = "JSON_OBJECTAGG"
			// Postgres renders JSONObjectAgg as JSON_OBJECT_AGG (generators/postgres.py:389,
			// rename_func("JSON_OBJECT_AGG")); base/mysql keep the JSON_OBJECTAGG spelling.
			if g.isDialect("postgres") {
				name = "JSON_OBJECT_AGG"
			}
		}
	}

	suffix := nullHandling + uniqueKeys + returnType + encoding + ")"
	return g.funcCall(name, listFromValue(e.Arg("expressions")), "(", suffix, true)
}

// jsonKeyValueSQL ports jsonkeyvalue_sql (generator.py:3746-3747). JSON_KEY_VALUE_PAIR_SEP
// is ":" by upstream base default (generator.py:460); MySQL overrides it to "," (generators/
// mysql.py:144) - Postgres has no override, so it keeps the base colon.
func (g *Generator) jsonKeyValueSQL(e expressions.Expression) string {
	sep := g.dialect.JSONKeyValuePairSep
	if sep == "" {
		sep = ":"
	}
	return g.sqlKey(e, "this") + sep + " " + g.sqlKey(e, "expression")
}

// onConditionSQL ports oncondition_sql (generator.py:5577-5603): renders the trailing
// "<empty> <error> <null>" clause shared by JSON_VALUE/JSON_TABLE/JSON_EXISTS. Static
// options ("NULL ON ERROR") are stored as plain strings; "DEFAULT <expr> ON <on>" is stored
// as the raw <expr> Expression, so onConditionPart re-adds the DEFAULT/ON wrapper around it.
func (g *Generator) onConditionSQL(e expressions.Expression) string {
	empty := g.onConditionPart(e, "empty", "EMPTY")
	error_ := g.onConditionPart(e, "error", "ERROR")

	// ON_CONDITION_EMPTY_BEFORE_ERROR is true for base/mysql/postgres alike (dialect.py:654),
	// so when both are set, EMPTY always precedes ERROR here (unlike upstream's
	// dialect-conditional order).
	if error_ != "" && empty != "" {
		if g.dialect.OnConditionEmptyBeforeError {
			error_ = empty + " " + error_
			empty = ""
		} else {
			// scripts/oracle.py sets ON_CONDITION_EMPTY_BEFORE_ERROR = False.
			empty = error_ + " " + empty
			error_ = ""
		}
	}

	null := g.sqlKey(e, "null")

	return empty + error_ + null
}

func (g *Generator) onConditionPart(e expressions.Expression, key, label string) string {
	if expr, ok := e.Arg(key).(expressions.Expression); ok && !isNilExpression(expr) {
		return "DEFAULT " + g.gen(expr) + " ON " + label
	}
	return g.sqlKey(e, key)
}

// jsonValueSQL ports jsonvalue_sql (generator.py:5550-5558): JSON_VALUE(<this>, <path>
// [RETURNING <type>] [<on_condition>]).
func (g *Generator) jsonValueSQL(e expressions.Expression) string {
	path := g.sqlKey(e, "path")
	returning := g.sqlKey(e, "returning")
	if returning != "" {
		returning = " RETURNING " + returning
	}
	onCondition := g.sqlKey(e, "on_condition")
	if onCondition != "" {
		onCondition = " " + onCondition
	}
	return g.funcCall("JSON_VALUE", []any{e.Arg("this"), path + returning + onCondition}, "(", ")", true)
}

// jsonArraySQL ports jsonarray_sql (generator.py:3860-3872).
func (g *Generator) jsonArraySQL(e expressions.Expression) string {
	nullHandling := ""
	if v := g.sqlKey(e, "null_handling"); v != "" {
		nullHandling = " " + v
	}
	returnType := ""
	if v := g.sqlKey(e, "return_type"); v != "" {
		returnType = " RETURNING " + v
	}
	strict := ""
	if boolValue(e.Arg("strict")) {
		strict = " STRICT"
	}
	return g.funcCall("JSON_ARRAY", exprAnySlice(e.Expressions()), "(", nullHandling+returnType+strict+")", true)
}

// jsonExistsSQL ports jsonexists_sql (generator.py:5693-5703).
func (g *Generator) jsonExistsSQL(e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	path := g.sqlKey(e, "path")
	if passing := g.expressions(exprsOptions{expression: e, key: "passing"}); passing != "" {
		path += " PASSING " + passing
	}
	if onCondition := g.sqlKey(e, "on_condition"); onCondition != "" {
		path += " " + onCondition
	}
	return g.funcCall("JSON_EXISTS", []any{this, path}, "(", ")", true)
}
