package dialects

import (
	"maps"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// SQLite ports the SQLite dialect class from
// .reference/sqlglot-v30.17.0/sqlglot/dialects/sqlite.py:13-40. It extends the base dialect;
// the parser overrides live in parser/dialect_sqlite.go and the generator overrides in
// generator/dialect_sqlite.go.
func SQLite() *Dialect {
	d := Base()
	d.Name = "sqlite"

	// dialects/sqlite.py:14-18. SQLite identifier matching is case-insensitive for ASCII
	// (https://sqlite.org/forum/forumpost/5e575586ac5c711b?raw), division is typed and
	// NULL-safe, and double negation is not eliminated (SQLite has no boolean type, so
	// `NOT NOT x` can differ from `x` for non-0/1 values).
	d.NormalizationStrategy = CaseInsensitive
	d.TypedDivision = true
	d.SafeDivision = true
	d.SafeToEliminateDoubleNegation = false

	// parsers/sqlite.py:38-46 parser flags.
	d.StringAliases = true
	d.AlterRenameRequiresColumn = false
	d.JoinsHaveEqualPrecedence = true
	d.AddJoinOnTrue = true
	// generators/sqlite.py:114 SUPPORTS_TABLE_ALIAS_COLUMNS = False: named columns in a table
	// alias are dropped with a warning.
	d.SupportsTableAliasColumns = false
	// generators/sqlite.py:119 JSON_KEY_VALUE_PAIR_SEP = ",".
	d.JSONKeyValuePairSep = ","
	// generators/sqlite.py:116 SUPPORTS_WINDOW_EXCLUDE = True.
	d.SupportsWindowExclude = true

	cfg := d.TokenizerConfig
	// dialects/sqlite.py:21 IDENTIFIERS = ['"', ("[", "]"), "`"].
	cfg.Identifiers = map[rune]string{'"': `"`, '[': "]", '`': "`"}
	cfg.FormatStrings = maps.Clone(cfg.FormatStrings)
	// dialects/sqlite.py:22 HEX_STRINGS = [("x'", "'"), ("X'", "'"), ("0x", ""), ("0X", "")].
	cfg.FormatStrings["x'"] = tokens.FormatString{End: "'", TokenType: tokens.HEX_STRING}
	cfg.FormatStrings["X'"] = tokens.FormatString{End: "'", TokenType: tokens.HEX_STRING}
	cfg.HasHexStrings = true
	// DIVERGENCE (DEVIATIONS §1.28): 0x-prefixed literals are integers in SQLite, not BLOBs.
	cfg.HexIntegerLiterals = true
	d.HexStart, d.HexEnd = "x'", "'"
	// dialects/sqlite.py:24 NESTED_COMMENTS = False.
	cfg.NestedComments = false
	// dialects/sqlite.py:34 KEYWORDS.pop("/*+"): SQLite has no hint comments.
	cfg.HintStart = ""

	// dialects/sqlite.py:26-32 keyword additions.
	cfg.Keywords = maps.Clone(cfg.Keywords)
	cfg.Keywords["ATTACH"] = tokens.ATTACH
	cfg.Keywords["DETACH"] = tokens.DETACH
	cfg.Keywords["INDEXED BY"] = tokens.INDEXED_BY
	cfg.Keywords["MATCH"] = tokens.MATCH

	// dialects/sqlite.py:36 COMMANDS = {*COMMANDS, TokenType.REPLACE}: a REPLACE statement
	// parses as a Command (the base Command parser preserves its text).
	cfg.Commands = maps.Clone(cfg.Commands)
	cfg.Commands[tokens.REPLACE] = true

	d.TokenizerConfig = tokens.CompileConfig(cfg)

	// parsers/sqlite.py:69-79 FUNCTIONS overlay on top of the base table.
	merged := make(map[string]func([]exp.Expression) exp.Expression, len(d.Functions)+8)
	for name, builder := range d.Functions {
		merged[name] = builder
	}
	for name, builder := range sqliteFunctions() {
		merged[name] = builder
	}
	d.Functions = merged

	return d
}

// sqliteFunctions ports SQLiteParser.FUNCTIONS additions (parsers/sqlite.py:69-79). The
// JSON path is kept as the raw argument expression (the JSONPath tokenizer is not ported,
// see ROADMAP); same-dialect round-trip is unaffected because SQLite's generator renders the
// function forms verbatim.
func sqliteFunctions() map[string]func([]exp.Expression) exp.Expression {
	return map[string]func([]exp.Expression) exp.Expression{
		"DATETIME": func(args []exp.Expression) exp.Expression {
			return exp.Anonymous(exp.Args{"this": "DATETIME", "expressions": args})
		},
		"EDITDIST3": func(args []exp.Expression) exp.Expression {
			return exp.FromArgList(exp.KindLevenshtein, args)
		},
		// parsers/sqlite.py:26-34 _build_json_extract: the single-path form returns an SQL
		// value like ->> but keeps the JSON subtype on object/array results (json_subtype);
		// the multi-path form returns a JSON array.
		"JSON_EXTRACT": func(args []exp.Expression) exp.Expression {
			if len(args) == 2 {
				return exp.JSONExtractScalar(exp.Args{
					"this":         args[0],
					"expression":   args[1],
					"json_subtype": true,
				})
			}
			if len(args) < 2 {
				return exp.Anonymous(exp.Args{"this": "JSON_EXTRACT", "expressions": args})
			}
			// The 2nd argument is the path (kept verbatim: the JSONPath tokenizer is not
			// ported) and any further arguments are extra paths.
			return exp.JSONExtract(exp.Args{"this": args[0], "expression": args[1], "expressions": args[2:]})
		},
		"JSON_GROUP_ARRAY": func(args []exp.Expression) exp.Expression {
			return exp.FromArgList(exp.KindJSONArrayAgg, args)
		},
		"JSON_GROUP_OBJECT": func(args []exp.Expression) exp.Expression {
			return exp.JSONObjectAgg(exp.Args{"expressions": args})
		},
		// parsers/sqlite.py:10-15 _build_strftime: with one argument the timestamp defaults
		// to CURRENT_TIMESTAMP; two arguments build TimeToStr; more stay an Anonymous call.
		"STRFTIME": func(args []exp.Expression) exp.Expression {
			if len(args) == 1 {
				args = append(args, exp.CurrentTimestamp(nil))
			}
			if len(args) == 2 {
				return exp.TimeToStr(exp.Args{
					"this":   exp.TsOrDsToTimestamp(exp.Args{"this": args[1]}),
					"format": args[0],
				})
			}
			return exp.Anonymous(exp.Args{"this": "STRFTIME", "expressions": args})
		},
		"SQLITE_VERSION": func(args []exp.Expression) exp.Expression {
			return exp.FromArgList(exp.KindCurrentVersion, args)
		},
		"TIME": func(args []exp.Expression) exp.Expression {
			return exp.Anonymous(exp.Args{"this": "TIME", "expressions": args})
		},
	}
}
