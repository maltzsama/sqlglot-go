package dialects

import (
	"maps"

	exp "github.com/ridi-oss/sqlglot-go/expressions"
	"github.com/ridi-oss/sqlglot-go/tokens"
)

// StarRocks ports the StarRocks dialect class from
// .reference/sqlglot-v30.17.0/sqlglot/dialects/starrocks.py:9-22. StarRocks extends MySQL, so
// this starts from MySQL() and applies the StarRocks deltas.
func StarRocks() *Dialect {
	d := MySQL()
	d.Name = "starrocks"

	// dialects/starrocks.py:10-11 STRICT_JSON_PATH_SYNTAX = False, INDEX_OFFSET = 1.
	d.IndexOffset = 1

	// dialects/starrocks.py:13-18 Tokenizer.KEYWORDS deltas.
	cfg := d.TokenizerConfig
	cfg.Keywords = maps.Clone(cfg.Keywords)
	cfg.Keywords["LARGEINT"] = tokens.INT128
	cfg.Keywords["REFRESH"] = tokens.REFRESH
	d.TokenizerConfig = tokens.CompileConfig(cfg)

	// generators/starrocks.py:57-68 generator flags.
	d.VarcharRequiresSize = false
	d.JSONTypeRequiredForExtraction = false
	d.WithPropertiesPrefix = "PROPERTIES"
	d.InsertOverwrite = " OVERWRITE"
	d.IsBoolAllowed = false
	d.RenameTableWithDB = false
	d.UpdateStatementSupportsFrom = true

	// parsers/starrocks.py:16-34 FUNCTIONS overlay on top of MySQL's.
	merged := make(map[string]func([]exp.Expression) exp.Expression, len(d.Functions)+9)
	for name, builder := range d.Functions {
		merged[name] = builder
	}
	for name, builder := range starRocksFunctions() {
		merged[name] = builder
	}
	d.Functions = merged

	// generators/starrocks.py:141-293 RESERVED_KEYWORDS.
	d.ReservedKeywords = starRocksReservedKeywords()

	return d
}

// starRocksReservedKeywords mirrors StarRocksGenerator.RESERVED_KEYWORDS
// (generators/starrocks.py:141-293).
func starRocksReservedKeywords() map[string]bool {
	words := []string{
		"add", "all", "alter", "analyze", "and", "array", "as", "asc", "between", "bigint",
		"bitmap", "both", "by", "case", "char", "character", "check", "collate", "column",
		"compaction", "convert", "create", "cross", "cube", "current_date", "current_role",
		"current_time", "current_timestamp", "current_user", "database", "databases", "decimal",
		"decimalv2", "decimal32", "decimal64", "decimal128", "default", "deferred", "delete",
		"dense_rank", "desc", "describe", "distinct", "double", "drop", "dual", "else", "except",
		"exists", "explain", "false", "first_value", "float", "for", "force", "from", "full",
		"function", "grant", "group", "grouping", "grouping_id", "groups", "having", "hll", "host",
		"if", "ignore", "immediate", "in", "index", "infile", "inner", "insert", "int", "integer",
		"intersect", "into", "is", "join", "json", "key", "keys", "kill", "lag", "largeint",
		"last_value", "lateral", "lead", "left", "like", "limit", "load", "localtime",
		"localtimestamp", "maxvalue", "minus", "mod", "not", "ntile", "null", "on", "or", "order",
		"outer", "outfile", "over", "partition", "percentile", "primary", "procedure", "qualify",
		"range", "rank", "read", "regexp", "release", "rename", "replace", "revoke", "right",
		"rlike", "row", "row_number", "rows", "schema", "schemas", "select", "set", "set_var",
		"show", "smallint", "system", "table", "terminated", "text", "then", "tinyint", "to",
		"true", "union", "unique", "unsigned", "update", "use", "using", "values", "varchar",
		"when", "where", "with",
	}
	out := make(map[string]bool, len(words))
	for _, w := range words {
		out[w] = true
	}
	return out
}
