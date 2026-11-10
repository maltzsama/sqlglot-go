package dialects

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// Redshift ports the Redshift dialect class (dialects/redshift.py:11-63). It extends Postgres;
// the parser and generator override chains fall back to the postgres tables.
func Redshift() *Dialect {
	d := Postgres()
	d.Name = "redshift"

	// https://docs.aws.amazon.com/redshift/latest/dg/r_names.html
	d.NormalizationStrategy = CaseInsensitive
	d.NormalizeNotNull = true
	d.SupportsUserDefinedTypes = false
	d.IndexOffset = 0
	d.CoalesceComparisonNonStandard = true
	d.ArrayFuncsPropagatesNulls = true
	d.ArrayConcatIsVarLen = false
	d.HasDistinctArrayConstructors = true
	d.StarExcludeRequiresDerivedTable = false
	// dialect.py:345 derives SUPPORTS_COLUMN_JOIN_MARKS from "(+)" in the tokenizer keywords;
	// Redshift's KEYWORDS add it (dialects/redshift.py:45), so the `(+)` outer-join marker is
	// parsed and rendered like Oracle's.
	d.SupportsColumnJoinMarks = true

	// generators/redshift.py:21-44 generator flags that have a Go counterpart.
	d.LockingReadsSupported = false
	d.ValuesAsTable = false
	d.CopyParamsAreWrapped = false
	// dialects/redshift.py:20: COPY params are space-separated, not comma-separated.
	d.CopyParamsAreCsv = false
	d.AlterSetType = "TYPE"
	d.LimitFetch = "LIMIT"
	// Redshift has no WITH in its with_properties.
	d.WithPropertiesPrefix = " "

	cfg := d.TokenizerConfig
	// BIT_STRINGS = [] and HEX_STRINGS = [] (dialects/redshift.py:38-39).
	cfg.HasBitStrings = false
	cfg.HasHexStrings = false
	for _, prefix := range []string{"b'", "B'", "x'", "X'"} {
		delete(cfg.FormatStrings, prefix)
	}
	d.BitStart, d.BitEnd = "", ""
	d.HexStart, d.HexEnd = "", ""
	// STRING_ESCAPES = ["\\", "'"].
	cfg.StringEscapes = map[rune]bool{'\\': true, '\'': true}

	for keyword, tokenType := range map[string]tokens.TokenType{
		"(+)":             tokens.JOIN_MARKER,
		"BINARY VARYING":  tokens.VARBINARY,
		"CURRENT_USER_ID": tokens.CURRENT_USER_ID,
		"HLLSKETCH":       tokens.HLLSKETCH,
		"MINUS":           tokens.EXCEPT,
		"SUPER":           tokens.SUPER,
		"TOP":             tokens.TOP,
		"UNLOAD":          tokens.COMMAND,
		"USER":            tokens.CURRENT_USER,
		"VARBYTE":         tokens.VARBINARY,
	} {
		cfg.Keywords[keyword] = tokenType
	}
	delete(cfg.Keywords, "VALUES")
	// Redshift allows # to appear as a table identifier prefix.
	delete(cfg.SingleTokens, '#')
	d.TokenizerConfig = tokens.CompileConfig(cfg)
	// STRING_ESCAPES[0] is the backslash, so the generator escapes a quote as \'.
	d.StringEscape = "\\"

	// parsers/redshift.py:33-68 FUNCTIONS overlay on top of Postgres's (minus GET_BIT, which
	// the Go Postgres table doesn't register).
	merged := make(map[string]func([]exp.Expression) exp.Expression, len(d.Functions)+16)
	for name, builder := range d.Functions {
		merged[name] = builder
	}
	for name, builder := range redshiftFunctions() {
		merged[name] = builder
	}
	d.Functions = merged

	// dialects/redshift.py:30-34 TIME_MAPPING overrides Postgres's with MON/MONTH on top; the
	// inverse follows from it (dialect.py:261-271) and drives generators/postgres.py:399's
	// TO_CHAR generation inherited via PostgresGenerator.
	d.TimeMapping = redshiftTimeMapping
	d.InverseTimeMapping = inverseTimeMapping(redshiftTimeMapping, redshiftTimeMappingOrder)

	// generators/redshift.py:111-269 RESERVED_KEYWORDS.
	d.ReservedKeywords = redshiftReservedKeywords()
	return d
}

// redshiftReservedKeywords mirrors RedshiftGenerator.RESERVED_KEYWORDS
// (generators/redshift.py:111-269), including upstream's "encrypt" entry with its trailing
// spaces.
func redshiftReservedKeywords() map[string]bool {
	words := []string{
		"aes128",
		"aes256",
		"all",
		"allowoverwrite",
		"analyse",
		"analyze",
		"and",
		"any",
		"array",
		"as",
		"asc",
		"authorization",
		"az64",
		"backup",
		"between",
		"binary",
		"blanksasnull",
		"both",
		"bytedict",
		"bzip2",
		"case",
		"cast",
		"check",
		"collate",
		"column",
		"constraint",
		"create",
		"credentials",
		"cross",
		"current_date",
		"current_time",
		"current_timestamp",
		"current_user",
		"current_user_id",
		"default",
		"deferrable",
		"deflate",
		"defrag",
		"delta",
		"delta32k",
		"desc",
		"disable",
		"distinct",
		"do",
		"else",
		"emptyasnull",
		"enable",
		"encode",
		"encrypt     ",
		"encryption",
		"end",
		"except",
		"explicit",
		"false",
		"for",
		"foreign",
		"freeze",
		"from",
		"full",
		"globaldict256",
		"globaldict64k",
		"grant",
		"group",
		"gzip",
		"having",
		"identity",
		"ignore",
		"ilike",
		"in",
		"initially",
		"inner",
		"intersect",
		"interval",
		"into",
		"is",
		"isnull",
		"join",
		"leading",
		"left",
		"like",
		"limit",
		"localtime",
		"localtimestamp",
		"lun",
		"luns",
		"lzo",
		"lzop",
		"minus",
		"mostly16",
		"mostly32",
		"mostly8",
		"natural",
		"new",
		"not",
		"notnull",
		"null",
		"nulls",
		"off",
		"offline",
		"offset",
		"oid",
		"old",
		"on",
		"only",
		"open",
		"or",
		"order",
		"outer",
		"overlaps",
		"parallel",
		"partition",
		"percent",
		"permissions",
		"pivot",
		"placing",
		"primary",
		"raw",
		"readratio",
		"recover",
		"references",
		"rejectlog",
		"resort",
		"respect",
		"restore",
		"right",
		"select",
		"session_user",
		"similar",
		"snapshot",
		"some",
		"sysdate",
		"system",
		"table",
		"tag",
		"tdes",
		"text255",
		"text32k",
		"then",
		"timestamp",
		"to",
		"top",
		"trailing",
		"true",
		"truncatecolumns",
		"type",
		"union",
		"unique",
		"unnest",
		"unpivot",
		"user",
		"using",
		"verbose",
		"wallet",
		"when",
		"where",
		"with",
		"without",
	}
	out := make(map[string]bool, len(words))
	for _, w := range words {
		out[w] = true
	}
	return out
}
