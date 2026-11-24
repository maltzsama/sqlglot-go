package generator

import "github.com/maltzsama/sqlglot-go/expressions"

func init() {
	dispatch[expressions.KindMaskingPolicyColumnConstraint] = (*Generator).maskingPolicySQL
	dispatch[expressions.KindProjectionPolicyColumnConstraint] = (*Generator).projectionPolicySQL
	dispatch[expressions.KindTags] = (*Generator).tagsSQL
	dispatch[expressions.KindRowAccessProperty] = (*Generator).rowAccessPolicySQL
}

func (g *Generator) maskingPolicySQL(e expressions.Expression) string {
	out := "MASKING POLICY " + g.sqlKey(e, "this")
	if values := g.expressions(exprsOptions{expression: e, flat: true}); values != "" {
		out += " USING (" + values + ")"
	}
	return out
}

func (g *Generator) projectionPolicySQL(e expressions.Expression) string {
	return "PROJECTION POLICY " + g.sqlKey(e, "this")
}

func (g *Generator) tagsSQL(e expressions.Expression) string {
	return "TAG (" + g.expressions(exprsOptions{expression: e, flat: true}) + ")"
}

func (g *Generator) rowAccessPolicySQL(e expressions.Expression) string {
	if !g.isDialect("snowflake") {
		return "ROW ACCESS"
	}
	out := "WITH ROW ACCESS POLICY " + g.sqlKey(e, "this")
	if values := g.expressions(exprsOptions{expression: e, flat: true}); values != "" {
		out += " ON (" + values + ")"
	}
	return out
}
