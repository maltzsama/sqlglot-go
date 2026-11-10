package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// Materialize parser overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/parsers/materialize.py. MaterializeParser extends
// PostgresParser, so lookups fall back to the postgres overlay.
func init() {
	registerDialectParserOverrides("materialize", dialectParserOverrideSet{
		// parsers/materialize.py:11 TYPED_LAMBDA_ARGS = True.
		TypedLambdaArgs: true,
		// parsers/materialize.py:13-16 NO_PAREN_FUNCTION_PARSERS: `MAP[...]` / `MAP(...)`.
		NoParenFunctionParsers: map[string]parserOverrideFunc{
			"MAP": (*Parser).parseMaterializeMap,
		},
		// parsers/materialize.py:18-24 LAMBDAS: the FARROW entry keeps the raw head
		// (no exp.var wrap) and parses the value with _parse_assignment.
		Lambdas: map[tokens.TokenType]func(*Parser, []exp.Expression) exp.Expression{
			tokens.FARROW: func(p *Parser, params []exp.Expression) exp.Expression {
				var this exp.Expression
				if len(params) > 0 {
					this = params[0]
				}
				return p.expression(exp.Kwarg(exp.Args{"this": this, "expression": p.parseAssignment()}), nil, nil)
			},
		},
		// parsers/materialize.py:16-17 _parse_lambda_arg -> _parse_field.
		LambdaArgParser: func(p *Parser) exp.Expression {
			return p.parseField(false, nil, false)
		},
	})
}

// parseMaterializeMap ports _parse_map (parsers/materialize.py:26-46): `MAP(<select>)` or
// `MAP[k => v, ...]`, where each bracket entry is a lambda-headed pair rewrapped as a bare
// PropertyEQ (upstream's comprehension builds those nodes directly, without self.expression
// validation, so this port does the same).
func (p *Parser) parseMaterializeMap() exp.Expression {
	if p.match(tokens.L_PAREN) {
		toMap := p.expression(exp.ToMap(exp.Args{"this": p.parseSelect()}), nil, nil)
		p.matchRParen(nil)
		return toMap
	}

	if !p.match(tokens.L_BRACKET) {
		p.raiseError("Expecting [")
	}

	entries := make([]exp.Expression, 0)
	for _, e := range p.parseCsv(func() exp.Expression { return p.parseLambda(false) }) {
		entries = append(entries, exp.PropertyEQ(exp.Args{
			"this":       e.Arg("this"),
			"expression": e.Arg("expression"),
		}))
	}

	if !p.match(tokens.R_BRACKET) {
		p.raiseError("Expecting ]")
	}

	strct := p.expression(exp.Struct(exp.Args{"expressions": entries}), nil, nil)
	return p.expression(exp.ToMap(exp.Args{"this": strct}), nil, nil)
}
