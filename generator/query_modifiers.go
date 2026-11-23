package generator

import (
	"strings"

	"github.com/maltzsama/sqlglot-go/expressions"
)

func (g *Generator) joinSQL(e expressions.Expression) string {
	side := e.Text("side")
	kind := e.Text("kind")
	globalSQL := ""
	if global := e.Text("global_"); global != "" {
		globalSQL = strings.ToUpper(global)
	}
	opSQL := joinNonEmpty(" ", e.Text("method"), globalSQL, side, kind, e.Text("hint"))
	if truthy(e.Arg("directed")) && g.dialect.DirectedJoins {
		opSQL = joinNonEmpty(" ", opSQL, "DIRECTED")
	}
	matchCond := g.sqlKey(e, "match_condition")
	if matchCond != "" {
		matchCond = " MATCH_CONDITION (" + matchCond + ")"
	}
	onSQL := g.sqlKey(e, "on")
	using := listFromValue(e.Arg("using"))
	if onSQL == "" && len(using) > 0 {
		cols := make([]string, 0, len(using))
		for _, column := range using {
			cols = append(cols, g.gen(column))
		}
		onSQL = strings.Join(cols, ", ")
	}
	this := asExpression(e.Arg("this"))
	thisSQL := g.gen(this)
	exprs := g.expressions(exprsOptions{expression: e})
	if exprs != "" {
		thisSQL = thisSQL + "," + g.seg(exprs)
	}
	// ksqlDB `joinWindow: WITHIN withinExpression` (SqlBase.g4:251-252) renders between
	// the joined relation and its ON criteria: `WITHIN <size>` or `WITHIN (<before>,
	// <after>)`, each optionally followed by `GRACE PERIOD <size>`.
	withinSQL := ""
	if g.isDialect("ksqldb") {
		within := g.sqlKey(e, "within")
		if within != "" {
			withinSQL = " WITHIN " + within
		}
		if grace := g.sqlKey(e, "grace"); grace != "" {
			withinSQL += " GRACE PERIOD " + grace
		}
	}
	if onSQL != "" {
		onSQL = g.indent(onSQL, 0, nil, true, false)
		space := " "
		if g.pretty {
			space = g.seg(strings.Repeat(" ", g.pad))
		}
		if len(using) > 0 {
			onSQL = space + "USING (" + onSQL + ")"
		} else {
			onSQL = space + "ON " + onSQL
		}
	} else if opSQL == "" {
		// generator.py:2850: CROSS/OUTER APPLY joins carry the `cross_apply` flag on the
		// Lateral child (the join's `this`, set in parser_pivot.go parseLateral), NOT on the
		// Join node — so read it from `this`. Without this, APPLY joins emit a spurious ", ".
		if this != nil && this.Kind() == expressions.KindLateral && this.Arg("cross_apply") != nil {
			return " " + thisSQL
		}
		return ", " + thisSQL
	}
	if opSQL != "STRAIGHT_JOIN" {
		if opSQL != "" {
			opSQL += " JOIN"
		} else {
			opSQL = "JOIN"
		}
	}
	pivots := g.expressions(exprsOptions{expression: e, key: "pivots", emptySep: true, flat: true})
	return g.seg(opSQL) + " " + thisSQL + withinSQL + matchCond + onSQL + pivots
}

func (g *Generator) clusterSQL(e expressions.Expression) string {
	return g.opExpressions("CLUSTER BY", e)
}
func (g *Generator) distributeSQL(e expressions.Expression) string {
	return g.opExpressions("DISTRIBUTE BY", e)
}
func (g *Generator) sortSQL(e expressions.Expression) string     { return g.opExpressions("SORT BY", e) }
func (g *Generator) preWhereSQL(e expressions.Expression) string { return "" }

func (g *Generator) lockSQL(e expressions.Expression) string {
	if !g.dialect.LockingReadsSupported {
		g.unsupported("Locking reads using 'FOR UPDATE/SHARE' are not supported")
		return ""
	}

	update := boolValue(e.Arg("update"))
	key := boolValue(e.Arg("key"))
	var lockType string
	if update {
		if key {
			lockType = "FOR NO KEY UPDATE"
		} else {
			lockType = "FOR UPDATE"
		}
	} else {
		if key {
			lockType = "FOR KEY SHARE"
		} else {
			lockType = "FOR SHARE"
		}
	}
	exprs := g.expressions(exprsOptions{expression: e, flat: true})
	if exprs != "" {
		exprs = " OF " + exprs
	}

	wait := ""
	if w := e.Arg("wait"); w != nil {
		if we := asExpression(w); we != nil && we.Kind() == expressions.KindLiteral {
			wait = " WAIT " + g.gen(we)
		} else if boolValue(w) {
			wait = " NOWAIT"
		} else {
			wait = " SKIP LOCKED"
		}
	}

	return lockType + exprs + wait
}
