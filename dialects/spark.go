package dialects

import (
	"maps"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// Spark ports dialects/spark.py (class Spark(Spark2)): Spark 3+'s tokenizer, parser FUNCTIONS
// overlay and feature flags. The SparkGenerator behavior is registered in
// generator/dialect_spark.go.
func Spark() *Dialect {
	d := Spark2()
	d.Name = "spark"
	// generators/redshift.py:33 / snowflake.py:394 / spark.py:70: native CONVERT_TIMEZONE.
	d.SupportsConvertTimezone = true

	// dialects/spark.py:16-19 class attributes. ARRAY_FUNCS_PROPAGATES_NULLS and the
	// ORDER BY / LIMIT ALL flags are the only ones this port models (the remaining
	// SUPPORTS_* flags gate base-generator helpers this port doesn't implement yet).
	d.SupportsOrderByAll = true
	d.SupportsLimitAll = true
	d.ArrayFuncsPropagatesNulls = true

	// dialects/spark.py:32-43 Tokenizer: RAW_STRINGS = [(prefix + q, q) for q in QUOTES for
	// prefix in ("r", "R")], STRING_ESCAPES_ALLOWED_IN_RAW_STRINGS = False and the DECLARE
	// keyword. The strict TIME_MAPPING needs no overlay: hive already carries the strict
	// %mstrict/%dstrict/... folds.
	cfg := d.TokenizerConfig
	cfg.FormatStrings = maps.Clone(cfg.FormatStrings)
	cfg.Keywords = maps.Clone(cfg.Keywords)
	for _, quote := range []string{"'", "\""} {
		for _, prefix := range []string{"r", "R"} {
			cfg.FormatStrings[prefix+quote] = tokens.FormatString{End: quote, TokenType: tokens.RAW_STRING}
		}
	}
	cfg.StringEscapesAllowedInRawStrings = false
	cfg.Keywords["DECLARE"] = tokens.DECLARE
	d.TokenizerConfig = tokens.CompileConfig(cfg)

	// parsed/spark.py:58-82 FUNCTIONS = {**Spark2Parser.FUNCTIONS, ...}.
	d.Functions = sparkFunctions(d.Functions)

	return d
}

// sparkFunctions builds the SparkParser.FUNCTIONS overlay (parsers/spark.py:58-82) on top of
// the inherited Spark 2 table.
func sparkFunctions(base map[string]func([]exp.Expression) exp.Expression) map[string]func([]exp.Expression) exp.Expression {
	out := make(map[string]func([]exp.Expression) exp.Expression, len(base)+30)
	maps.Copy(out, base)

	// "ANY_VALUE": build_with_ignore_nulls(exp.AnyValue).
	out["ANY_VALUE"] = hiveBuildWithIgnoreNulls(exp.KindAnyValue)
	// "ARRAY_INSERT": ArrayInsert(this, position, expression, offset=1).
	out["ARRAY_INSERT"] = sparkBuildArrayInsert
	// The BIT_* aggregates and BIT_GET.
	out["BIT_AND"] = exp.FromArgListFunc(exp.KindBitwiseAndAgg)
	out["BIT_OR"] = exp.FromArgListFunc(exp.KindBitwiseOrAgg)
	out["BIT_XOR"] = exp.FromArgListFunc(exp.KindBitwiseXorAgg)
	out["BIT_COUNT"] = exp.FromArgListFunc(exp.KindBitwiseCount)
	out["BIT_GET"] = exp.FromArgListFunc(exp.KindGetbit)
	// "CURDATE": exp.CurrentDate.from_arg_list.
	out["CURDATE"] = exp.FromArgListFunc(exp.KindCurrentDate)
	// DATE_ADD/DATEADD/TIMESTAMPADD share _build_dateadd.
	out["DATE_ADD"] = sparkBuildDateAdd
	out["DATEADD"] = sparkBuildDateAdd
	out["TIMESTAMPADD"] = sparkBuildDateAdd
	// TIMESTAMPDIFF: build_date_delta(exp.TimestampDiff).
	out["TIMESTAMPDIFF"] = sparkBuildDateDelta(exp.KindTimestampDiff)
	// MAKE_TIMESTAMP.
	out["MAKE_TIMESTAMP"] = exp.FromArgListFunc(exp.KindTimestampFromParts)
	// The TRY_* arithmetic helpers.
	out["TRY_ADD"] = exp.FromArgListFunc(exp.KindSafeAdd)
	out["TRY_DIVIDE"] = exp.FromArgListFunc(exp.KindSafeDivide)
	out["TRY_MULTIPLY"] = exp.FromArgListFunc(exp.KindSafeMultiply)
	out["TRY_SUBTRACT"] = exp.FromArgListFunc(exp.KindSafeSubtract)
	// DATEDIFF/DATE_DIFF share _build_datediff.
	out["DATEDIFF"] = sparkBuildDateDiff
	out["DATE_DIFF"] = sparkBuildDateDiff
	// JSON_OBJECT_KEYS and LISTAGG.
	out["JSON_OBJECT_KEYS"] = exp.FromArgListFunc(exp.KindJSONKeys)
	out["LISTAGG"] = exp.FromArgListFunc(exp.KindGroupConcat)
	// TIMESTAMP_LTZ/TIMESTAMP_NTZ cast builders.
	out["TIMESTAMP_LTZ"] = spark2BuildAsCast("TIMESTAMP_LTZ")
	out["TIMESTAMP_NTZ"] = spark2BuildAsCast("TIMESTAMP_NTZ")
	// "TRY_ELEMENT_AT": Bracket(this, [args[1]], offset=1, safe=True).
	out["TRY_ELEMENT_AT"] = sparkBuildTryElementAt
	// LIKE/ILIKE functions with an optional ESCAPE argument.
	out["LIKE"] = sparkBuildLike(exp.KindLike)
	out["ILIKE"] = sparkBuildLike(exp.KindILike)

	return out
}

// sparkBuildArrayInsert ports the ARRAY_INSERT lambda (parsers/spark.py:62-67).
func sparkBuildArrayInsert(args []exp.Expression) exp.Expression {
	return exp.ArrayInsert(exp.Args{
		"this":       spark2SeqGet(args, 0),
		"position":   spark2SeqGet(args, 1),
		"expression": spark2SeqGet(args, 2),
		"offset":     1,
	})
}

// sparkBuildTryElementAt ports the TRY_ELEMENT_AT lambda (parsers/spark.py:96-101): a safe,
// 1-based bracket.
func sparkBuildTryElementAt(args []exp.Expression) exp.Expression {
	expressions := []exp.Expression{}
	if index := spark2SeqGet(args, 1); index != nil {
		expressions = append(expressions, index)
	}
	return exp.Bracket(exp.Args{
		"this":        spark2SeqGet(args, 0),
		"expressions": expressions,
		"offset":      1,
		"safe":        true,
	})
}

// sparkBuildLike ports build_like (dialects/dialect.py:2444-2456): LIKE/ILIKE as a function
// call, with the optional third argument wrapped in ESCAPE.
func sparkBuildLike(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		like := exp.New(kind, exp.Args{
			"this":       spark2SeqGet(args, 0),
			"expression": spark2SeqGet(args, 1),
		})
		if escape := spark2SeqGet(args, 2); escape != nil {
			return exp.Escape(exp.Args{"this": like, "expression": escape})
		}
		return like
	}
}

// sparkBuildDateAdd ports _build_dateadd (parsers/spark.py:38-54).
func sparkBuildDateAdd(args []exp.Expression) exp.Expression {
	expression := spark2SeqGet(args, 1)
	if len(args) == 2 {
		return exp.TsOrDsAdd(exp.Args{
			"this":       spark2SeqGet(args, 0),
			"expression": expression,
			"unit":       exp.TimeUnitVar(exp.LiteralString("DAY")),
		})
	}
	return exp.TimestampAdd(exp.Args{
		"this":       spark2SeqGet(args, 2),
		"expression": expression,
		"unit":       exp.TimeUnitVar(spark2SeqGet(args, 0)),
	})
}

// sparkBuildDateDiff ports _build_datediff (parsers/spark.py:16-36): the optional first
// argument is the unit.
func sparkBuildDateDiff(args []exp.Expression) exp.Expression {
	var unit exp.Expression
	this := spark2SeqGet(args, 0)
	expression := spark2SeqGet(args, 1)
	if len(args) == 3 {
		unit = exp.TimeUnitVar(this)
		this = args[2]
	}
	return exp.DateDiff(exp.Args{
		"this":       exp.TsOrDsToDate(exp.Args{"this": this}),
		"expression": exp.TsOrDsToDate(exp.Args{"this": expression}),
		"unit":       unit,
	})
}

// sparkBuildDateDelta ports build_date_delta (dialects/dialect.py:1636-1655) with the default
// DAY unit and no timezone argument, the form TIMESTAMPDIFF uses.
func sparkBuildDateDelta(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		unitBased := len(args) >= 3
		this := spark2SeqGet(args, 0)
		unit := exp.LiteralString("DAY")
		if unitBased {
			this = args[2]
			unit = args[0]
		}
		return exp.New(kind, exp.Args{
			"this":       this,
			"expression": spark2SeqGet(args, 1),
			"unit":       exp.TimeUnitVar(unit),
		})
	}
}
