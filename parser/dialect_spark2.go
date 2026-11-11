package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// parsers/spark2.py supplies the Spark2Parser overlays that are not plain FUNCTIONS entries:
// FUNCTION_PARSERS (parsers/spark2.py:84-97) and the _parse_drop_column replacement
// (parsers/spark2.py:99-104, applied in parseDropColumn). The FUNC_TOKENS additions live on the
// dialect (d.ExtraFuncTokens, dialect.go) and the FUNCTIONS overlay in dialects/spark2.go.
func init() {
	registerDialectParserOverrides("spark2", spark2ParserOverrideSet())
}

func spark2ParserOverrideSet() dialectParserOverrideSet {
	return dialectParserOverrideSet{
		FunctionParsers: map[string]parserOverrideFunc{
			// parsers/spark2.py:86,92: AND/OR calls are connector functions.
			"AND": func(p *Parser) exp.Expression {
				return p.parseClickhouseConnectorFunction(exp.KindAnd)
			},
			"OR": func(p *Parser) exp.Expression {
				return p.parseClickhouseConnectorFunction(exp.KindOr)
			},
			// parsers/spark2.py:87: APPROX_PERCENTILE shares hive's distinct-argument
			// quantile grammar (parser.py:10109-10122 _parse_distinct_arg_function).
			"APPROX_PERCENTILE": func(p *Parser) exp.Expression {
				return p.parseHiveQuantileFunction(exp.KindApproxQuantile)
			},
			// parsers/spark2.py:88-96: the Spark join hints.
			"BROADCAST":            spark2JoinHint("BROADCAST"),
			"BROADCASTJOIN":        spark2JoinHint("BROADCASTJOIN"),
			"MAPJOIN":              spark2JoinHint("MAPJOIN"),
			"MERGE":                spark2JoinHint("MERGE"),
			"MERGEJOIN":            spark2JoinHint("MERGEJOIN"),
			"SHUFFLEMERGE":         spark2JoinHint("SHUFFLEMERGE"),
			"SHUFFLE_HASH":         spark2JoinHint("SHUFFLE_HASH"),
			"SHUFFLE_REPLICATE_NL": spark2JoinHint("SHUFFLE_REPLICATE_NL"),
		},
	}
}

// spark2JoinHint ports the `_parse_join_hint(func_name)` lambdas (parser.py:8525-8527,
// parsers/spark2.py:88-96): JoinHint(this=NAME, expressions=<csv of tables>).
func spark2JoinHint(name string) parserOverrideFunc {
	return func(p *Parser) exp.Expression {
		return exp.JoinHint(exp.Args{
			"this": stringsUpper(name),
			"expressions": p.parseCsv(func() exp.Expression {
				return p.parseTable(false, false, nil, false, false, false, false)
			}),
		})
	}
}
