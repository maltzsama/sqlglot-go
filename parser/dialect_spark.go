package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// parsers/spark.py supplies the SparkParser overlays on top of Spark2Parser: the SESSION_USER
// no-paren function (parsers/spark.py:56-59). The FUNCTIONS overlay lives in dialects/spark.go
// and the grammar overrides are wired as dialect-gated branches into the shared parser
// methods: SET VAR/VARIABLE in parseSetItem, the widget placeholder in parsePlaceholder, the
// non-aggregate PIVOT entry in parsePivotAggregation, and the computed-column GENERATED form in
// parseGeneratedAsIdentity.
func init() {
	registerDialectParserOverrides("spark", dialectParserOverrideSet{
		// parsers/spark.py:104-106 FUNCTION_PARSERS adds SUBSTR's FROM/FOR grammar.
		FunctionParsers: map[string]parserOverrideFunc{
			"SUBSTR": (*Parser).parseSubstring,
		},
		StatementParsers: map[tokens.TokenType]parserOverrideFunc{
			tokens.DECLARE: (*Parser).parseDeclareStatement,
			tokens.REFRESH: (*Parser).parseRefresh,
		},
		NoParenFunctions: map[tokens.TokenType]func(exp.Args) exp.Expression{
			tokens.SESSION_USER: exp.SessionUser,
		},
	})
}
