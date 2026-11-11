package dialects

import (
	"maps"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// Databricks ports dialects/databricks.py (class Databricks(Spark)): the parser flag and
// FUNCTIONS overlays, the tokenizer's STREAM/VOID keywords, the JSON path flags and the
// DatabricksGenerator behavior (generator/dialect_databricks.go).
func Databricks() *Dialect {
	d := Spark()
	d.Name = "databricks"

	// dialects/databricks.py:19-21 and parsers/databricks.py:18-21.
	d.SafeDivision = false
	d.CopyParamsAreCsv = false
	d.LogDefaultsToLn = true
	d.ColonIsVariantExtract = true
	d.ColonChainIsSingleExtract = false
	// parsers/databricks.py:12 STRICT_CAST = True (hive and spark2 leave it False).
	d.StrictCast = true

	// generators/databricks.py:14-21.
	d.TablesampleSeedKeyword = "REPEATABLE"
	d.CopyParamsAreWrapped = false
	d.CopyParamsEqRequired = true
	d.QuoteJSONPath = false
	d.JSONPathKeyQuotedForcesBrackets = true

	// dialects/databricks.py:35-41 Tokenizer: STREAM and VOID keywords.
	cfg := d.TokenizerConfig
	cfg.Keywords = maps.Clone(cfg.Keywords)
	cfg.Keywords["STREAM"] = tokens.STREAM
	cfg.Keywords["VOID"] = tokens.VOID
	d.TokenizerConfig = tokens.CompileConfig(cfg)

	// parsers/databricks.py:23-35 FUNCTIONS = {**SparkParser.FUNCTIONS, ...}.
	d.Functions = databricksFunctions(d.Functions)

	return d
}

// databricksFunctions builds the DatabricksParser.FUNCTIONS overlay
// (parsers/databricks.py:23-35) on top of the inherited Spark table.
func databricksFunctions(base map[string]func([]exp.Expression) exp.Expression) map[string]func([]exp.Expression) exp.Expression {
	out := make(map[string]func([]exp.Expression) exp.Expression, len(base)+8)
	maps.Copy(out, base)

	out["IFF"] = exp.FromArgListFunc(exp.KindIf)
	out["GETDATE"] = exp.FromArgListFunc(exp.KindCurrentTimestamp)
	out["NOW"] = exp.FromArgListFunc(exp.KindCurrentTimestamp)
	// DATEDIFF/DATE_DIFF use the standard date-delta grammar here, replacing Spark's builder.
	out["DATEDIFF"] = sparkBuildDateDelta(exp.KindDateDiff)
	out["DATE_DIFF"] = sparkBuildDateDelta(exp.KindDateDiff)
	// TO_DATE: build_formatted_time(exp.TsOrDsToDate).
	out["TO_DATE"] = func(args []exp.Expression) exp.Expression {
		return hiveBuildFormattedTime(exp.KindTsOrDsToDate, spark2SeqGet(args, 0), spark2SeqGet(args, 1), false)
	}
	// UNIFORM(min, max[, seed]).
	out["UNIFORM"] = func(args []exp.Expression) exp.Expression {
		return exp.Uniform(exp.Args{
			"this":       spark2SeqGet(args, 0),
			"expression": spark2SeqGet(args, 1),
			"seed":       spark2SeqGet(args, 2),
		})
	}

	return out
}
