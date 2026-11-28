package dialects

import (
	"maps"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// Teradata ports the Teradata dialect from
// .reference/sqlglot-v30.17.0/sqlglot/dialects/teradata.py (parser overrides in
// parser/dialect_teradata.go, generator overrides in generator/dialect_teradata.go).
func Teradata() *Dialect {
	d := Base()
	d.Name = "teradata"

	// dialects/teradata.py:10 TYPED_DIVISION = True.
	d.TypedDivision = true
	// generators/teradata.py:53 CAN_IMPLEMENT_ARRAY_ANY = True.
	d.CanImplementArrayAny = true
	// parsers/teradata.py:99-101 EXPONENT[DSTAR] = Pow — Teradata's `^` is BitwiseXor, so
	// CARET is deliberately absent here.
	d.ExponentOperators = map[tokens.TokenType]bool{tokens.DSTAR: true}
	// parsers/teradata.py:14-15 TABLESAMPLE_CSV = True, VALUES_FOLLOWED_BY_PAREN = False.
	d.TablesampleCSV = true
	d.ValuesFollowedByParen = false
	// generators/teradata.py:40-41 SELECT_KINDS = (), TRY_SUPPORTED = False.
	d.TrySupported = false
	// generators/teradata.py:55-56 LIMIT_IS_TOP, TABLESAMPLE_KEYWORDS = "SAMPLE".
	d.LimitIsTop = true
	d.TablesampleKeywords = "SAMPLE"

	// dialects/teradata.py:12-40 TIME_MAPPING, with declaration order for the inverse.
	d.TimeMapping = teradataTimeMapping
	d.InverseTimeMapping = inverseTimeMapping(teradataTimeMapping, teradataTimeOrder)

	cfg := d.TokenizerConfig
	// dialects/teradata.py:43-45 HEX_STRINGS = [("X'", "'"), ("x'", "'"), ("0x", "")].
	cfg.FormatStrings = maps.Clone(cfg.FormatStrings)
	cfg.FormatStrings["X'"] = tokens.FormatString{End: "'", TokenType: tokens.HEX_STRING}
	cfg.FormatStrings["x'"] = tokens.FormatString{End: "'", TokenType: tokens.HEX_STRING}
	cfg.HasHexStrings = true
	d.HexStart, d.HexEnd = "X'", "'"
	// dialects/teradata.py:47-77 KEYWORDS.
	cfg.Keywords = maps.Clone(cfg.Keywords)
	for keyword, tokenType := range map[string]tokens.TokenType{
		"**":          tokens.DSTAR,
		"^=":          tokens.NEQ,
		"BYTEINT":     tokens.SMALLINT,
		"COLLECT":     tokens.COMMAND,
		"DEL":         tokens.DELETE,
		"EXEC":        tokens.EXECUTE,
		"EQ":          tokens.EQ,
		"GE":          tokens.GTE,
		"GT":          tokens.GT,
		"HELP":        tokens.COMMAND,
		"INS":         tokens.INSERT,
		"LE":          tokens.LTE,
		"LOCKING":     tokens.LOCK,
		"LT":          tokens.LT,
		"MINUS":       tokens.EXCEPT,
		"MOD":         tokens.MOD,
		"NE":          tokens.NEQ,
		"NOT=":        tokens.NEQ,
		"SAMPLE":      tokens.TABLE_SAMPLE,
		"SEL":         tokens.SELECT,
		"ST_GEOMETRY": tokens.GEOMETRY,
		"TOP":         tokens.TOP,
		"UPD":         tokens.UPDATE,
	} {
		cfg.Keywords[keyword] = tokenType
	}
	// parsers/teradata.py Tokenizer: COMMANDS = base COMMANDS - {EXECUTE}, so EXECUTE parses as a statement.
	delete(cfg.Commands, tokens.EXECUTE)
	// dialects/teradata.py:78 KEYWORDS.pop("/*+").
	cfg.HintStart = ""
	// dialects/teradata.py:81-83 SINGLE_TOKENS.pop("%"): Teradata has no % modulo operator.
	cfg.SingleTokens = maps.Clone(cfg.SingleTokens)
	delete(cfg.SingleTokens, '%')
	d.TokenizerConfig = tokens.CompileConfig(cfg)

	// parsers/teradata.py:157-161 FUNCTIONS additions.
	merged := make(map[string]func([]exp.Expression) exp.Expression, len(d.Functions)+2)
	for name, builder := range d.Functions {
		merged[name] = builder
	}
	merged["CARDINALITY"] = func(args []exp.Expression) exp.Expression {
		return exp.FromArgList(exp.KindArraySize, args)
	}
	merged["RANDOM"] = func(args []exp.Expression) exp.Expression {
		return exp.Rand(exp.Args{"lower": dremioSeqGet(args, 0), "upper": dremioSeqGet(args, 1)})
	}
	d.Functions = merged

	return d
}

// teradataTimeMapping ports TIME_MAPPING (dialects/teradata.py:12-40).
var teradataTimeMapping = map[string]string{
	"YY": "%y", "Y4": "%Y", "YYYY": "%Y",
	"M4": "%B", "M3": "%b", "M": "%-M", "MI": "%M", "MM": "%m", "MMM": "%b", "MMMM": "%B",
	"D": "%-d", "DD": "%d", "D3": "%j", "DDD": "%j",
	"H": "%-H", "HH": "%H", "HH24": "%H",
	"S": "%-S", "SS": "%S", "SSSSSS": "%f",
	"E": "%a", "EE": "%a", "E3": "%a", "E4": "%A", "EEE": "%a", "EEEE": "%A",
}

// teradataTimeOrder is the declaration order of teradataTimeMapping.
var teradataTimeOrder = []string{
	"YY", "Y4", "YYYY",
	"M4", "M3", "M", "MI", "MM", "MMM", "MMMM",
	"D", "DD", "D3", "DDD",
	"H", "HH", "HH24",
	"S", "SS", "SSSSSS",
	"E", "EE", "E3", "E4", "EEE", "EEEE",
}
