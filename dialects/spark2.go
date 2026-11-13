package dialects

import (
	"maps"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// Spark2 ports dialects/spark2.py (class Spark2(Hive)): Hive plus the Spark 2.x specifics -
// the tokenizer's X'…' hex strings and TIMESTAMP keyword remap, the parser-side flags and
// FUNCTIONS overlay (parsers/spark2.py), and the Spark2Generator behavior, which is
// registered in the generator package (generator/dialect_spark2.go).
func Spark2() *Dialect {
	d := Hive()
	d.Name = "spark2"
	// generators/spark2.py:115 PARSE_JSON_NAME = None (bare value).
	d.ParseJSONName = stringPtr("")

	// dialects/spark2.py:14-15 ALTER_TABLE_SUPPORTS_CASCADE = False.
	d.AlterTableSupportsCascade = false

	// dialects/spark2.py:17-29 class attributes: the parser flags
	// (parsers/spark2.py:22-24), the lenient Spark 2.x initcap delimiters, and the
	// generator flags (generators/spark2.py:113, 213-214).
	d.TrimPatternFirst = true
	d.ChangeColumnAlterSyntax = true
	d.PivotColumnNaming = "agg_name_if_multiple"
	d.InitcapDefaultDelimiterChars = " "
	d.AlterSetType = "TYPE"
	d.WrapDerivedValues = false
	d.CreateFunctionReturnAs = false

	// dialects/spark2.py:30-34 Tokenizer: HEX_STRINGS = [("X'", "'"), ("x'", "'")] and
	// KEYWORDS overlays "TIMESTAMP" -> TokenType.TIMESTAMPTZ on top of Hive's.
	cfg := d.TokenizerConfig
	cfg.FormatStrings = maps.Clone(cfg.FormatStrings)
	cfg.Keywords = maps.Clone(cfg.Keywords)
	cfg.FormatStrings["X'"] = tokens.FormatString{End: "'", TokenType: tokens.HEX_STRING}
	cfg.FormatStrings["x'"] = tokens.FormatString{End: "'", TokenType: tokens.HEX_STRING}
	cfg.HasHexStrings = true
	cfg.Keywords["TIMESTAMP"] = tokens.TIMESTAMPTZ
	d.TokenizerConfig = tokens.CompileConfig(cfg)
	// dialects/dialect.py:323 HEX_START/HEX_END take the FIRST HEX_STRING format string.
	d.HexStart, d.HexEnd = "X'", "'"

	// dialects/spark2.py:26-28 and parsers/spark2.py:26: FUNC_TOKENS = HiveParser.FUNC_TOKENS
	// | {TokenType.AND, TokenType.OR}. Hive leaves ExtraFuncTokens nil, so this must be a fresh
	// map - mutating a shared one would leak AND/OR into Hive itself.
	d.ExtraFuncTokens = map[tokens.TokenType]bool{
		tokens.AND: true,
		tokens.OR:  true,
	}

	// parsers/spark2.py:30-79 FUNCTIONS = {**HiveParser.FUNCTIONS, ...}.
	d.Functions = spark2Functions(d, d.Functions)

	return d
}

// asExpression narrows a generic arg value to an Expression, mirroring the generator
// package's helper of the same name (generator/generator.go).
func asExpression(value any) exp.Expression {
	if expr, ok := value.(exp.Expression); ok && expr != nil {
		return expr
	}
	return nil
}

// spark2SeqGet mirrors sqlglot.helper.seq_get: args[i] or nil when out of range.
func spark2SeqGet(args []exp.Expression, i int) exp.Expression {
	if i >= 0 && i < len(args) {
		return args[i]
	}
	return nil
}

// spark2Functions builds the Spark2Parser.FUNCTIONS overlay (parsers/spark2.py:30-79) on top
// of the inherited Hive FUNCTIONS dict. The callbacks build the canonical expression Kinds
// directly, as this port keeps the base registry and dialect overlays separate.
func spark2Functions(d *Dialect, base map[string]func([]exp.Expression) exp.Expression) map[string]func([]exp.Expression) exp.Expression {
	out := make(map[string]func([]exp.Expression) exp.Expression, len(base)+30)
	maps.Copy(out, base)

	// parsers/spark2.py:17-18 build_as_cast: exp.Cast(this=args[0], to=DataType.from_str(t)).
	out["BOOLEAN"] = spark2BuildAsCast("boolean")
	out["DATE"] = spark2BuildAsCast("date")
	out["DOUBLE"] = spark2BuildAsCast("double")
	out["FLOAT"] = spark2BuildAsCast("float")
	out["INT"] = spark2BuildAsCast("int")
	out["STRING"] = spark2BuildAsCast("string")
	out["TIMESTAMP"] = spark2BuildAsCast("timestamp")

	// "AGGREGATE": exp.Reduce.from_arg_list.
	out["AGGREGATE"] = exp.FromArgListFunc(exp.KindReduce)
	// "DATE_TRUNC": TimestampTrunc(this=args[1], unit=exp.var(args[0])).
	out["DATE_TRUNC"] = spark2BuildDateTrunc
	// "DAYOFMONTH"/"DAYOFWEEK"/"DAYOFYEAR"/"WEEKOFYEAR": X(this=TsOrDsToDate(args[0])).
	out["DAYOFMONTH"] = spark2BuildTsOrDsDay(exp.KindDayOfMonth)
	out["DAYOFWEEK"] = spark2BuildTsOrDsDay(exp.KindDayOfWeek)
	out["DAYOFYEAR"] = spark2BuildTsOrDsDay(exp.KindDayOfYear)
	out["WEEKOFYEAR"] = spark2BuildTsOrDsDay(exp.KindWeekOfYear)
	// "ELEMENT_AT": Bracket(this=args[0], expressions=[args[1]], offset=1, safe=False).
	out["ELEMENT_AT"] = spark2BuildElementAt
	// "FORMAT_STRING": exp.Format.from_arg_list.
	out["FORMAT_STRING"] = exp.FromArgListFunc(exp.KindFormat)
	// "FROM_UTC_TIMESTAMP": AtTimeZone(this=cast(args[0], TIMESTAMP), zone=args[1]).
	out["FROM_UTC_TIMESTAMP"] = spark2BuildFromUtcTimestamp(d)
	// "LTRIM"/"RTRIM": build_trim(reverse_args=True / is_left=False).
	out["LTRIM"] = spark2BuildLTrim
	out["RTRIM"] = spark2BuildRTrim
	// "MAP_FROM_ARRAYS": exp.Map.from_arg_list.
	out["MAP_FROM_ARRAYS"] = exp.FromArgListFunc(exp.KindMap)
	// "RLIKE": exp.RegexpLike.from_arg_list.
	out["RLIKE"] = exp.FromArgListFunc(exp.KindRegexpLike)
	// "SHIFTLEFT"/"SHIFTRIGHT": binary_from_function(BitwiseLeftShift/RightShift).
	out["SHIFTLEFT"] = spark2BuildBitwiseShift(exp.KindBitwiseLeftShift)
	out["SHIFTRIGHT"] = spark2BuildBitwiseShift(exp.KindBitwiseRightShift)
	// "SLICE": exp.ArraySlice.from_arg_list.
	out["SLICE"] = exp.FromArgListFunc(exp.KindArraySlice)
	// "TO_TIMESTAMP": 1 arg -> cast, 2 args -> StrToTime with the mapped format.
	out["TO_TIMESTAMP"] = spark2BuildToTimestamp
	// "TO_UNIX_TIMESTAMP": exp.StrToUnix.from_arg_list.
	out["TO_UNIX_TIMESTAMP"] = exp.FromArgListFunc(exp.KindStrToUnix)
	// "TO_UTC_TIMESTAMP": FromTimeZone(this=cast(args[0], TIMESTAMP), zone=args[1]).
	out["TO_UTC_TIMESTAMP"] = spark2BuildToUtcTimestamp(d)
	// "TRUNC": DateTrunc(unit=args[1], this=args[0]).
	out["TRUNC"] = spark2BuildTrunc

	return out
}

// spark2BuildAsCast ports build_as_cast (parsers/spark2.py:17-18). The dtype string resolves
// through DataType.from_str with no dialect, i.e. the base type registry, so STRING is TEXT
// exactly as upstream's exp.DataType.from_str(to_type) is.
func spark2BuildAsCast(toType string) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		to, err := exp.DataTypeBuild(toType, "", false, false, nil)
		if err != nil {
			panic(err)
		}
		return exp.Cast(exp.Args{"this": spark2SeqGet(args, 0), "to": to})
	}
}

// spark2BuildDateTrunc ports the DATE_TRUNC lambda (parsers/spark2.py:32-34). Upstream's
// TimestampTrunc constructor folds the unit through TimeUnit's normalization
// (expressions/core.py:2055-2066), which exp.TimeUnitVar ports.
func spark2BuildDateTrunc(args []exp.Expression) exp.Expression {
	return exp.New(exp.KindTimestampTrunc, exp.Args{
		"this": spark2SeqGet(args, 1),
		"unit": exp.TimeUnitVar(spark2SeqGet(args, 0)),
	})
}

// spark2BuildTsOrDsDay ports the DAYOF*/WEEKOFYEAR lambdas (parsers/spark2.py:35-37, 79):
// X(this=TsOrDsToDate(args[0])).
func spark2BuildTsOrDsDay(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		return exp.New(kind, exp.Args{
			"this": exp.TsOrDsToDate(exp.Args{"this": spark2SeqGet(args, 0)}),
		})
	}
}

// spark2BuildElementAt ports the ELEMENT_AT lambda (parsers/spark2.py:40-45).
func spark2BuildElementAt(args []exp.Expression) exp.Expression {
	expressions := []exp.Expression{}
	// Upstream stores [args[1]] even when it is missing; a nil list element would crash every
	// walker here, so the degenerate one-argument call keeps an empty list instead.
	if index := spark2SeqGet(args, 1); index != nil {
		expressions = append(expressions, index)
	}
	return exp.Bracket(exp.Args{
		"this":        spark2SeqGet(args, 0),
		"expressions": expressions,
		"offset":      1,
		"safe":        false,
	})
}

// spark2BuildBitwiseShift ports binary_from_function (dialects/dialect.py:1898-1899) for the
// SHIFTLEFT/SHIFTRIGHT entries (parsers/spark2.py:60-61).
func spark2BuildBitwiseShift(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		return exp.New(kind, exp.Args{
			"this":       spark2SeqGet(args, 0),
			"expression": spark2SeqGet(args, 1),
		})
	}
}

// spark2BuildFromUtcTimestamp ports the FROM_UTC_TIMESTAMP lambda (parsers/spark2.py:46-52).
func spark2BuildFromUtcTimestamp(d *Dialect) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		return exp.AtTimeZone(exp.Args{
			"this": spark2CastTimestamp(d, args),
			"zone": spark2SeqGet(args, 1),
		})
	}
}

// spark2BuildToUtcTimestamp ports the TO_UTC_TIMESTAMP lambda (parsers/spark2.py:66-72).
func spark2BuildToUtcTimestamp(d *Dialect) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		return exp.FromTimeZone(exp.Args{
			"this": spark2CastTimestamp(d, args),
			"zone": spark2SeqGet(args, 1),
		})
	}
}

// spark2CastTimestamp ports exp.cast(seq_get(args, 0) or exp.Var(this=""), DType.TIMESTAMP,
// dialect=...) (parsers/spark2.py:47-51, 67-71): the argument is wrapped in a TIMESTAMP cast,
// unless exp.cast's no-re-cast short-circuit (expressions/builders.py:487-503) applies.
func spark2CastTimestamp(d *Dialect, args []exp.Expression) exp.Expression {
	this := spark2SeqGet(args, 0)
	if this == nil {
		this = exp.Var(exp.Args{"this": ""})
	}
	if this.Kind() == exp.KindCast || this.Kind() == exp.KindTryCast {
		if to := asExpression(this.Arg("to")); to != nil && to.Kind() == exp.KindDataType {
			if dtype, ok := to.Arg("this").(exp.DType); ok && d.castTargetsEquivalent(dtype, exp.DTypeTimestamp) {
				return this
			}
		}
	}
	return exp.New(exp.KindCast, exp.Args{"this": this, "to": exp.DTypeTimestamp.IntoExpr(nil)})
}

// spark2BuildLTrim ports the LTRIM lambda (parsers/spark2.py:53): build_trim(args,
// reverse_args=True).
func spark2BuildLTrim(args []exp.Expression) exp.Expression {
	return spark2BuildTrim(args, true, true)
}

// spark2BuildRTrim ports the RTRIM lambda (parsers/spark2.py:61): build_trim(args,
// is_left=False, reverse_args=True).
func spark2BuildRTrim(args []exp.Expression) exp.Expression {
	return spark2BuildTrim(args, false, true)
}

// spark2BuildTrim ports build_trim (parser.py:170-176): this/expression come from the first
// two arguments, swapped when reverse_args is set, plus the LEADING/TRAILING position.
func spark2BuildTrim(args []exp.Expression, isLeft bool, reverseArgs bool) exp.Expression {
	this := spark2SeqGet(args, 0)
	expression := spark2SeqGet(args, 1)
	if expression != nil && reverseArgs {
		this, expression = expression, this
	}
	position := "LEADING"
	if !isLeft {
		position = "TRAILING"
	}
	return exp.Trim(exp.Args{
		"this":       this,
		"expression": expression,
		"position":   position,
	})
}

// spark2BuildToTimestamp ports the TO_TIMESTAMP lambda (parsers/spark2.py:63-68): one argument
// is a plain cast, two build a StrToTime whose format goes through the dialect's time mapping.
func spark2BuildToTimestamp(args []exp.Expression) exp.Expression {
	if len(args) < 2 {
		return spark2BuildAsCast("timestamp")(args)
	}
	return hiveBuildFormattedTime(exp.KindStrToTime, args[0], args[1], false)
}

// spark2BuildTrunc ports the TRUNC lambda (parsers/spark2.py:77): DateTrunc(unit=args[1],
// this=args[0]).
func spark2BuildTrunc(args []exp.Expression) exp.Expression {
	return exp.DateTrunc(exp.Args{
		"this": spark2SeqGet(args, 0),
		"unit": spark2SeqGet(args, 1),
	})
}
