package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// Redshift parser overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/parsers/redshift.py. RedshiftParser extends
// PostgresParser, so lookups fall back to the postgres overlay. Its _parse_table,
// _parse_convert and _parse_projections overrides are applied inline where the base parser
// runs them (parseTable, parseConvert and the SELECT projection list).
func init() {
	registerDialectParserOverrides("redshift", dialectParserOverrideSet{
		// parsers/redshift.py:70-73 NO_PAREN_FUNCTIONS.
		NoParenFunctions: map[tokens.TokenType]func(exp.Args) exp.Expression{
			tokens.CURRENT_USER_ID: exp.CurrentUserId,
		},
		// parsers/redshift.py:75-79 NO_PAREN_FUNCTION_PARSERS.
		NoParenFunctionParsers: map[string]parserOverrideFunc{
			"APPROXIMATE": (*Parser).parseRedshiftApproximateCount,
			"SYSDATE":     (*Parser).parseOracleSysdate,
		},
		// parsers/redshift.py:81-84 FUNCTION_PARSERS.
		FunctionParsers: map[string]parserOverrideFunc{
			"OBJECT_TRANSFORM": (*Parser).parseRedshiftObjectTransform,
		},
	})
}

// parseRedshiftObjectTransform ports _parse_object_transform (parsers/redshift.py:116-124).
func (p *Parser) parseRedshiftObjectTransform() exp.Expression {
	this := p.parseColumn()
	var keep, set []exp.Expression
	if p.match(tokens.KEEP) {
		keep = p.parseCsv(p.parsePrimary)
	}
	if p.match(tokens.SET) {
		set = p.parseCsv(p.parseExpression)
	}
	return p.expression(exp.ObjectTransform(exp.Args{"this": this, "keep": keep, "set_": set}), nil, nil)
}

// parseRedshiftApproximateCount ports _parse_approximate_count (parsers/redshift.py:126-142):
// APPROXIMATE COUNT(DISTINCT x) and APPROXIMATE PERCENTILE_DISC(q) WITHIN GROUP (ORDER BY x).
func (p *Parser) parseRedshiftApproximateCount() exp.Expression {
	index := p.index - 1
	fn := p.parseFunction(nil, false, true, false)
	if fn != nil && fn.Kind() == exp.KindCount {
		if distinct := fn.This(); distinct != nil && distinct.Kind() == exp.KindDistinct {
			var first exp.Expression
			if exprs := distinct.Expressions(); len(exprs) > 0 {
				first = exprs[0]
			}
			return p.expression(exp.New(exp.KindApproxDistinct, exp.Args{"this": first}), nil, nil)
		}
	}
	if fn != nil && fn.Kind() == exp.KindWithinGroup {
		if percentile := fn.This(); percentile != nil && percentile.Kind() == exp.KindPercentileDisc {
			var this exp.Expression
			if order := fn.Expr(); order != nil {
				if exprs := order.Expressions(); len(exprs) > 0 {
					this = exprs[0].This()
				}
			}
			return p.expression(exp.New(exp.KindApproxQuantile, exp.Args{"this": this, "quantile": percentile.This()}), nil, nil)
		}
	}
	p.retreat(index)
	return nil
}

// redshiftParseConvert ports _parse_convert (parsers/redshift.py:110-114): CONVERT(type, x).
func (p *Parser) redshiftParseConvert(safe any) exp.Expression {
	to := p.parseTypes(false, false, false, false)
	p.match(tokens.COMMA)
	this := p.parseBitwise()
	args := exp.Args{"this": this, "to": to}
	if safe != nil {
		args["safe"] = safe
	}
	return p.expression(exp.Cast(args), nil, nil)
}

// redshiftProjectionsExclude ports _parse_projections (parsers/redshift.py:144-163): the
// EXCLUDE clause always comes at the end of the projection list and applies to it as a whole.
func (p *Parser) redshiftProjectionsExclude(projections []exp.Expression) ([]exp.Expression, []exp.Expression) {
	if p.prev.IsValid() && stringsUpper(p.prev.Text) == "EXCLUDE" && p.curr.IsValid() {
		p.retreat(p.index - 1)
	}
	var exclude []exp.Expression
	if p.matchTextSeq("EXCLUDE") {
		exclude = p.parseWrappedCsv(p.parseExpression, true)
	}
	if len(exclude) > 0 && len(projections) > 0 {
		if last := projections[len(projections)-1]; last.Kind() == exp.KindAlias && stringsUpper(last.Alias()) == "EXCLUDE" {
			projections[len(projections)-1] = last.This().Pop()
		}
	}
	return projections, exclude
}
