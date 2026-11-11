package dialects

import (
	"regexp"
	"strings"

	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// Builder helpers for the BigQueryParser.FUNCTIONS overlay (parsers/bigquery.py:20-165,
// 224-323).

// bigQueryBuildContainsSubstring ports _build_contains_substring (parsers/bigquery.py:20-23):
// both operands are lowercased and the optional third argument is the JSON scope.
func bigQueryBuildContainsSubstring(args []exp.Expression) exp.Expression {
	return exp.Contains(exp.Args{
		"this":       exp.Lower(exp.Args{"this": seqGet(args, 0)}),
		"expression": exp.Lower(exp.Args{"this": seqGet(args, 1)}),
		"json_scope": seqGet(args, 2),
	})
}

// bigQueryBuildDate ports _build_date (parsers/bigquery.py:26-28): three arguments build
// DATE_FROM_PARTS, anything else a plain Date.
func bigQueryBuildDate(args []exp.Expression) exp.Expression {
	if len(args) == 3 {
		return exp.FromArgList(exp.KindDateFromParts, args)
	}
	return exp.FromArgList(exp.KindDate, args)
}

// bigQueryNormalizeBareWeek ports _normalize_bare_week (parsers/bigquery.py:31-37): a bare WEEK
// date part is equivalent to WEEK(SUNDAY).
func bigQueryNormalizeBareWeek(e exp.Expression) exp.Expression {
	if e == nil {
		return e
	}
	unit := asExpression(e.Arg("unit"))
	if unit == nil {
		return e
	}
	if unit.Kind() != exp.KindLiteral && unit.Kind() != exp.KindVar {
		return e
	}
	if strings.ToUpper(unit.Name()) == "WEEK" {
		e.Set("unit", exp.WeekStart(exp.Args{"this": exp.Var(exp.Args{"this": "SUNDAY"})}))
	}
	return e
}

// bigQueryBuildDateDiff ports build_date_diff (parsers/bigquery.py:40-53).
func bigQueryBuildDateDiff(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		return bigQueryNormalizeBareWeek(exp.New(kind, exp.Args{
			"this":               seqGet(args, 0),
			"expression":         seqGet(args, 1),
			"unit":               exp.TimeUnitVar(seqGet(args, 2)),
			"date_part_boundary": true,
		}))
	}
}

// bigQueryBuildDateDeltaWithInterval ports build_date_delta_with_interval
// (dialects/dialect.py:1658-1690): (this, INTERVAL <value> <unit>) becomes
// XxxAdd(this, expression=<value>, unit=<unit>).
func bigQueryBuildDateDeltaWithInterval(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		if len(args) < 2 {
			return nil
		}
		interval := seqGet(args, 1)
		if interval != nil && interval.Kind() == exp.KindInterval {
			return exp.New(kind, exp.Args{
				"this":       seqGet(args, 0),
				"expression": asExpression(interval.Arg("this")),
				"unit":       exp.TimeUnitVar(asExpression(interval.Arg("unit"))),
			})
		}
		return exp.New(kind, exp.Args{
			"this":       seqGet(args, 0),
			"expression": interval,
			"unit":       exp.LiteralString("DAY"),
		})
	}
}

// bigQueryBuildDateTrunc ports the DATE_TRUNC lambda (parsers/bigquery.py:237-243).
func bigQueryBuildDateTrunc(args []exp.Expression) exp.Expression {
	return bigQueryNormalizeBareWeek(exp.DateTrunc(exp.Args{
		"unit": seqGet(args, 1),
		"this": seqGet(args, 0),
		"zone": seqGet(args, 2),
	}))
}

// bigQueryBuildDatetimeTrunc ports the DATETIME_TRUNC lambda (parsers/bigquery.py:248).
func bigQueryBuildDatetimeTrunc(args []exp.Expression) exp.Expression {
	return bigQueryNormalizeBareWeek(exp.New(exp.KindDatetimeTrunc, exp.Args{
		"this": exp.FromArgList(exp.KindDatetimeTrunc, args).Arg("this"),
		"unit": seqGet(args, 1),
	}))
}

// bigQueryBuildTimestampTrunc ports the TIMESTAMP_TRUNC lambda (parsers/bigquery.py:311-313).
func bigQueryBuildTimestampTrunc(args []exp.Expression) exp.Expression {
	return bigQueryNormalizeBareWeek(exp.New(exp.KindTimestampTrunc, exp.Args{
		"this": seqGet(args, 0),
		"unit": exp.TimeUnitVar(seqGet(args, 1)),
		"zone": seqGet(args, 2),
	}))
}

// bigQueryBuildLastDay ports the LAST_DAY lambda (parsers/bigquery.py:264).
func bigQueryBuildLastDay(args []exp.Expression) exp.Expression {
	return bigQueryNormalizeBareWeek(exp.FromArgList(exp.KindLastDay, args))
}

// bigQueryBuildDatetime ports _build_datetime (parsers/bigquery.py:56-61).
func bigQueryBuildDatetime(args []exp.Expression) exp.Expression {
	switch len(args) {
	case 1:
		return exp.New(exp.KindTsOrDsToDatetime, exp.Args{"this": seqGet(args, 0)})
	case 2:
		return exp.Datetime(exp.Args{"this": seqGet(args, 0), "expression": seqGet(args, 1)})
	}
	return exp.FromArgList(exp.KindTimestampFromParts, args)
}

// bigQueryBuildTime ports _build_time (parsers/bigquery.py:147-152).
func bigQueryBuildTime(args []exp.Expression) exp.Expression {
	switch len(args) {
	case 1:
		return exp.TsOrDsToTime(exp.Args{"this": seqGet(args, 0)})
	case 2:
		return exp.Time(exp.Args{"this": seqGet(args, 0), "zone": seqGet(args, 1)})
	}
	return exp.FromArgList(exp.KindTimeFromParts, args)
}

// bigQueryBuildTimestamp ports _build_timestamp (parsers/bigquery.py:155-158).
func bigQueryBuildTimestamp(args []exp.Expression) exp.Expression {
	built := exp.FromArgList(exp.KindTimestamp, args)
	built.Set("with_tz", true)
	return built
}

// bigQueryBuildExtractJSON ports _build_extract_json_with_default_path
// (parsers/bigquery.py:64-72) plus build_extract_json_with_path (parser.py:110-124). The
// JSONPath subsystem is not ported, so the path expression is kept verbatim.
func bigQueryBuildExtractJSON(d *Dialect, kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		if len(args) == 1 {
			args = append(args, exp.LiteralString("$"))
		}
		built := exp.New(kind, exp.Args{"this": seqGet(args, 0), "expression": seqGet(args, 1)})
		if len(args) > 2 && kind == exp.KindJSONExtract {
			built.Set("expressions", args[2:])
		}
		if kind == exp.KindJSONExtractScalar {
			built.Set("scalar_only", d.JSONExtractScalarScalarOnly)
		}
		return built
	}
}

// bigQueryBuildFormatTime ports _build_format_time (parsers/bigquery.py:75-83).
func bigQueryBuildFormatTime(d *Dialect, kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		inner := exp.New(kind, exp.Args{"this": seqGet(args, 1)})
		built := postgresBuildFormattedTimeClass(d, exp.KindTimeToStr)([]exp.Expression{inner, seqGet(args, 0)})
		built.Set("zone", seqGet(args, 2))
		return built
	}
}

// bigQueryBuildJSONStripNulls ports _build_json_strip_nulls (parsers/bigquery.py:86-93).
func bigQueryBuildJSONStripNulls(args []exp.Expression) exp.Expression {
	built := exp.JSONStripNulls(exp.Args{"this": seqGet(args, 0)})
	for _, arg := range args[1:] {
		if arg == nil {
			continue
		}
		if arg.Kind() == exp.KindKwarg {
			built.Set(strings.ToLower(arg.This().Name()), arg)
			continue
		}
		built.Set("expression", arg)
	}
	return built
}

// bigQueryBuildLevenshtein ports _build_levenshtein (parsers/bigquery.py:96-102).
func bigQueryBuildLevenshtein(args []exp.Expression) exp.Expression {
	var maxDist exp.Expression
	if arg := seqGet(args, 2); arg != nil {
		maxDist = asExpression(arg.Arg("expression"))
	}
	return exp.Levenshtein(exp.Args{
		"this":       seqGet(args, 0),
		"expression": seqGet(args, 1),
		"max_dist":   maxDist,
	})
}

// bigQueryBuildParseDate ports _build_parse_date (parsers/bigquery.py:105-108).
func bigQueryBuildParseDate(d *Dialect) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		built := postgresBuildFormattedTimeClass(d, exp.KindStrToDate)([]exp.Expression{seqGet(args, 1), seqGet(args, 0)})
		built.Set("default_year", exp.LiteralNumber("1970"))
		return built
	}
}

// bigQueryBuildParseTimestamp ports _build_parse_timestamp (parsers/bigquery.py:111-115).
func bigQueryBuildParseTimestamp(d *Dialect) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		built := postgresBuildFormattedTimeClass(d, exp.KindStrToTime)([]exp.Expression{seqGet(args, 1), seqGet(args, 0)})
		built.Set("zone", seqGet(args, 2))
		built.Set("default_year", exp.LiteralNumber("1970"))
		return built
	}
}

// bigQueryBuildParseDatetime ports _build_parse_datetime (parsers/bigquery.py:118-121).
func bigQueryBuildParseDatetime(d *Dialect) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		built := postgresBuildFormattedTimeClass(d, exp.KindParseDatetime)([]exp.Expression{seqGet(args, 1), seqGet(args, 0)})
		built.Set("default_year", exp.LiteralNumber("1970"))
		return built
	}
}

// bigQueryBuildRegexpExtract ports _build_regexp_extract (parsers/bigquery.py:124-144).
// bigQueryRegexpExtractOverflowNull ports the base
// REGEXP_EXTRACT_POSITION_OVERFLOW_RETURNS_NULL = True (dialects/dialect.py:719), which
// BigQuery inherits.
const bigQueryRegexpExtractOverflowNull = true

func bigQueryBuildRegexpExtract(kind exp.Kind, defaultGroup exp.Expression) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		hasOneGroup := false
		if pattern := seqGet(args, 1); pattern != nil {
			if compiled, err := regexp.Compile(pattern.Name()); err == nil {
				hasOneGroup = compiled.NumSubexp() == 1
			}
		}

		built := exp.New(kind, exp.Args{
			"this":       seqGet(args, 0),
			"expression": seqGet(args, 1),
			"position":   seqGet(args, 2),
			"occurrence": seqGet(args, 3),
		})
		if hasOneGroup {
			built.Set("group", exp.LiteralNumber("1"))
		} else if defaultGroup != nil {
			built.Set("group", defaultGroup)
		}
		if kind == exp.KindRegexpExtract {
			built.Set("null_if_pos_overflow", bigQueryRegexpExtractOverflowNull)
		}
		return built
	}
}

// bigQueryBuildToHex ports _build_to_hex (parsers/bigquery.py:161-163).
func bigQueryBuildToHex(args []exp.Expression) exp.Expression {
	arg := seqGet(args, 0)
	if arg != nil && arg.Kind() == exp.KindMD5Digest {
		return exp.MD5(exp.Args{"this": arg.This()})
	}
	return exp.LowerHex(exp.Args{"this": arg})
}

// bigQueryBinaryFromFunction ports binary_from_function (dialects/dialect.py:1898-1899).
func bigQueryBinaryFromFunction(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		return exp.New(kind, exp.Args{"this": seqGet(args, 0), "expression": seqGet(args, 1)})
	}
}

// bigQueryBuildLogarithm ports build_logarithm (parser.py:78-89) with BigQuery's
// LOG_BASE_FIRST=False and LOG_DEFAULTS_TO_LN=True: the single-argument form is Ln, and the
// two-argument form stores Log(this=value, expression=base).
func bigQueryBuildLogarithm(d *Dialect) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		this := seqGet(args, 0)
		expression := seqGet(args, 1)
		if expression != nil {
			if !d.LogBaseFirst {
				this, expression = expression, this
			}
			return exp.New(exp.KindLog, exp.Args{"this": this, "expression": expression})
		}
		if d.LogDefaultsToLn {
			return exp.FromArgList(exp.KindLn, args)
		}
		return exp.FromArgList(exp.KindLog, args)
	}
}
