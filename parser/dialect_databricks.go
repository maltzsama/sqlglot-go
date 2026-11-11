package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// parsers/databricks.py supplies the DatabricksParser overlays on top of SparkParser: the
// REGR_* distinct-argument functions, the optional-parens CURDATE, the `?::` SAFE_CAST
// operator, the TIMESERIES primary-key part and the AUTO/NONE CLUSTER BY property.
func init() {
	registerDialectParserOverrides("databricks", dialectParserOverrideSet{
		FunctionParsers: map[string]parserOverrideFunc{
			// parsers/databricks.py:37-43: only AVGX/SXX/SYY wrap the second argument.
			"REGR_AVGX": func(p *Parser) exp.Expression { return p.parseDistinctArgFunction(exp.KindRegrAvgx, 1) },
			"REGR_AVGY": func(p *Parser) exp.Expression { return p.parseDistinctArgFunction(exp.KindRegrAvgy, 0) },
			"REGR_SXX":  func(p *Parser) exp.Expression { return p.parseDistinctArgFunction(exp.KindRegrSxx, 1) },
			"REGR_SXY":  func(p *Parser) exp.Expression { return p.parseDistinctArgFunction(exp.KindRegrSxy, 0) },
			"REGR_SYY":  func(p *Parser) exp.Expression { return p.parseDistinctArgFunction(exp.KindRegrSyy, 1) },
		},
		NoParenFunctionParsers: map[string]parserOverrideFunc{
			"CURDATE": (*Parser).parseDatabricksCurdate,
		},
	})
}

// parseDatabricksCurdate ports _parse_curdate (parsers/databricks.py:64-69): CURDATE with
// optional parentheses, an alias of CURRENT_DATE.
func (p *Parser) parseDatabricksCurdate() exp.Expression {
	if p.match(tokens.L_PAREN) {
		p.matchRParen(nil)
	}
	return p.expression(exp.CurrentDate(nil), nil, nil)
}

// parseClusterProperty ports _parse_cluster_property (parser.py:3199-3204), overridden by
// databricks (parsers/databricks.py:73-76) for the AUTO/NONE forms.
func (p *Parser) parseClusterProperty() exp.Expression {
	if p.dialectIs("databricks") && p.matchTexts(map[string]bool{"AUTO": true, "NONE": true}) {
		return p.expression(exp.ClusterProperty(exp.Args{"this": stringsUpper(p.prev.Text)}), nil, nil)
	}
	return p.expression(exp.ClusterProperty(exp.Args{
		"expressions": p.parseWrappedCsv(func() exp.Expression { return p.parseColumn() }),
	}), nil, nil)
}
