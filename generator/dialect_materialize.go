package generator

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// Materialize generator overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/generators/materialize.py. MaterializeGenerator
// subclasses PostgresGenerator and spreads its TRANSFORMS except exp.ToMap (generators/
// materialize.py:19 drops the base `MAP {this}` lambda so the tomap_sql method below wins),
// so the rest falls back to the postgres overlay via dialectParents.
func init() {
	registerDialectDispatch("materialize", materializeDispatch())
}

func materializeDispatch() map[exp.Kind]func(*Generator, exp.Expression) string {
	return map[exp.Kind]func(*Generator, exp.Expression) string{
		// generators/materialize.py:20/27/28/29: constructs Postgres renders but Materialize
		// does not support are emitted as nothing (empty results are filtered out by
		// g.expressions).
		exp.KindAutoIncrementColumnConstraint:       materializeUnsupported,
		exp.KindGeneratedAsIdentityColumnConstraint: materializeUnsupported,
		exp.KindOnConflict:                          materializeUnsupported,
		exp.KindPrimaryKeyColumnConstraint:          materializeUnsupported,
		// generators/materialize.py:21-26 exp.Create: preprocess([remove_unique_constraints,
		// ctas_with_tmp_tables_to_create_tmp_view]).
		exp.KindCreate: materializeCreate,
		// generators/materialize.py:32-33 propertyeq_sql: MAP entries render `k => v`
		// (the base entry renders the mysql `:=` assignment).
		exp.KindPropertyEQ: func(g *Generator, e exp.Expression) string { return g.binary(e, "=>") },
		// generators/materialize.py:35-45 datatype_sql.
		exp.KindDataType: materializeDataType,
		// generators/materialize.py:47-51 list_sql.
		exp.KindList: materializeList,
		// generators/materialize.py:53-57 tomap_sql.
		exp.KindToMap: materializeToMap,
	}
}

// materializeUnsupported stands in for the `lambda self, e: ""` transforms.
func materializeUnsupported(_ *Generator, _ exp.Expression) string { return "" }

// materializeCreate ports the exp.Create entry (generators/materialize.py:21-26): the two
// upstream transforms run first, then the result goes through the normal create_sql.
// Upstream's preprocess re-dispatches through the "<key>_sql" lookup after the transforms;
// this port calls createSQL directly to avoid re-entering gen.
func materializeCreate(g *Generator, e exp.Expression) string {
	return g.createSQL(ctasWithTmpTablesToCreateTmpView(removeUniqueConstraints(e), false))
}

// materializeDataType ports datatype_sql (generators/materialize.py:35-45): LIST renders
// `<params> LIST` (or bare LIST) and MAP renders `MAP[<key> => <value>]`.
func materializeDataType(g *Generator, e exp.Expression) string {
	if exp.IsType(e, exp.DTypeList) {
		if len(e.Expressions()) > 0 {
			return g.expressions(exprsOptions{expression: e, flat: true}) + " LIST"
		}
		return "LIST"
	}
	if exp.IsType(e, exp.DTypeMap) && len(e.Expressions()) == 2 {
		exprs := e.Expressions()
		return "MAP[" + g.gen(exprs[0]) + " => " + g.gen(exprs[1]) + "]"
	}
	return g.dataTypeSQL(e)
}

// materializeList ports list_sql (generators/materialize.py:47-51): a SELECT body renders
// `LIST(<select>)`, everything else `LIST[<flat exprs>]`.
func materializeList(g *Generator, e exp.Expression) string {
	exprs := e.Expressions()
	if len(exprs) > 0 && exprs[0] != nil && exprs[0].Kind() == exp.KindSelect {
		return g.funcCall("LIST", []any{exprs[0]}, "(", ")", true)
	}
	return g.normalizeFunc("LIST") + "[" + g.expressions(exprsOptions{expression: e, flat: true}) + "]"
}

// materializeToMap ports tomap_sql (generators/materialize.py:53-57): a SELECT body renders
// `MAP(<select>)`, everything else `MAP[<struct exprs>]`.
func materializeToMap(g *Generator, e exp.Expression) string {
	if this := e.This(); this != nil && this.Kind() == exp.KindSelect {
		return g.funcCall("MAP", []any{this}, "(", ")", true)
	}
	this := asExpression(e.Arg("this"))
	body := ""
	if this != nil {
		body = g.expressions(exprsOptions{expression: this})
	}
	return g.normalizeFunc("MAP") + "[" + body + "]"
}
