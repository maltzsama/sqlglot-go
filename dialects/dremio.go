package dialects

import (
	"maps"
	"strconv"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// Dremio ports the Dremio dialect from
// .reference/sqlglot-v30.17.0/sqlglot/dialects/dremio.py. It extends the base dialect; the
// parser overrides live in parser/dialect_dremio.go and the generator overrides in
// generator/dialect_dremio.go.
func Dremio() *Dialect {
	d := Base()
	d.Name = "dremio"

	// dialects/dremio.py:9-13 dialect flags.
	d.SupportsUserDefinedTypes = false
	d.ConcatCoalesce = true
	d.ConcatWsCoalesce = true
	d.TypedDivision = true
	d.NullOrdering = "nulls_are_last"
	d.SupportsValuesDefault = false
	// parsers/dremio.py:87 LOG_DEFAULTS_TO_LN = True.
	d.LogDefaultsToLn = true

	// generators/dremio.py:32-46 generator flags.
	d.TrySupported = false
	d.LimitOnlyLiterals = true
	d.MultiArgDistinct = false
	d.SupportsConvertTimezone = true
	// generators/dremio.py:40 NVL2_SUPPORTED = False.
	d.Nvl2Supported = false
	d.IntervalAllowsPluralForm = false
	d.SupportsBetweenFlags = true

	// dialects/dremio.py:15-69 TIME_MAPPING, with its declaration order preserved for the
	// inverse mapping generation (duplicate values resolve to the last key, e.g. %Y -> yyyy).
	d.TimeMapping = dremioTimeMapping
	d.InverseTimeMapping = inverseTimeMapping(dremioTimeMapping, dremioTimeOrder)

	// dialects/dremio.py:71-73 Tokenizer COMMENTS adds `//`.
	cfg := d.TokenizerConfig
	cfg.Comments = maps.Clone(cfg.Comments)
	cfg.Comments["//"] = ""
	d.TokenizerConfig = tokens.CompileConfig(cfg)

	// parsers/dremio.py:98-109 FUNCTIONS overlay.
	merged := make(map[string]func([]exp.Expression) exp.Expression, len(d.Functions)+12)
	for name, builder := range d.Functions {
		merged[name] = builder
	}
	for name, builder := range dremioFunctions(d) {
		merged[name] = builder
	}
	d.Functions = merged

	return d
}

// dremioTimeMapping ports TIME_MAPPING (dialects/dremio.py:15-69).
var dremioTimeMapping = map[string]string{
	"YYYY": "%Y", "yyyy": "%Y",
	"YY": "%y", "yy": "%y",
	"MM": "%m", "mm": "%m",
	"MON": "%b", "mon": "%b",
	"MONTH": "%B", "month": "%B",
	"DDD": "%j", "ddd": "%j",
	"DD": "%d", "dd": "%d",
	"DY": "%a", "dy": "%a",
	"DAY": "%A", "day": "%A",
	"HH24": "%H", "hh24": "%H",
	"HH12": "%I", "hh12": "%I",
	"HH": "%I", "hh": "%I",
	"MI": "%M", "mi": "%M",
	"SS": "%S", "ss": "%S",
	"FFF": "%f", "fff": "%f",
	"AMPM": "%p", "ampm": "%p",
	"WW": "%W", "ww": "%W",
	"D": "%w", "d": "%w",
	"CC": "%C", "cc": "%C",
	"TZD": "%Z", "tzd": "%Z",
	"TZO": "%z", "tzo": "%z",
}

// dremioTimeOrder is the declaration order of dremioTimeMapping (Python dict order), which
// inverseTimeMapping needs because duplicate values resolve to the last key.
var dremioTimeOrder = []string{
	"YYYY", "yyyy", "YY", "yy",
	"MM", "mm", "MON", "mon", "MONTH", "month",
	"DDD", "ddd", "DD", "dd", "DY", "dy", "DAY", "day",
	"HH24", "hh24", "HH12", "hh12", "HH", "hh",
	"MI", "mi", "SS", "ss", "FFF", "fff", "AMPM", "ampm",
	"WW", "ww", "D", "d", "CC", "cc",
	"TZD", "tzd", "TZO", "tzo",
}

// dremioFunctions ports the DremioParser.FUNCTIONS overlay (parsers/dremio.py:98-109). The
// time-format builders close over the dialect so they can convert through its TIME_MAPPING.
func dremioFunctions(d *Dialect) map[string]func([]exp.Expression) exp.Expression {
	return map[string]func([]exp.Expression) exp.Expression{
		"ARRAY_GENERATE_RANGE": func(args []exp.Expression) exp.Expression {
			return exp.FromArgList(exp.KindGenerateSeries, args)
		},
		"BIT_AND": func(args []exp.Expression) exp.Expression {
			return exp.FromArgList(exp.KindBitwiseAndAgg, args)
		},
		"BIT_OR": func(args []exp.Expression) exp.Expression {
			return exp.FromArgList(exp.KindBitwiseOrAgg, args)
		},
		"DATE_ADD": dremioBuildDateDelta(exp.KindDateAdd),
		"DATE_SUB": dremioBuildDateDelta(exp.KindDateSub),
		// build_formatted_time(exp.TimeToStr) with no default.
		"DATE_FORMAT": dremioBuildFormattedTime(d, exp.KindTimeToStr),
		"TO_DATE":     dremioBuildFormattedTime(d, exp.KindTsOrDsToDate),
		"REGEXP_MATCHES": func(args []exp.Expression) exp.Expression {
			return exp.FromArgList(exp.KindRegexpLike, args)
		},
		"REPEATSTR": func(args []exp.Expression) exp.Expression {
			return exp.FromArgList(exp.KindRepeat, args)
		},
		"TO_CHAR": dremioBuildToChar(d),
		"DATE_PART": func(args []exp.Expression) exp.Expression {
			return exp.FromArgList(exp.KindExtract, args)
		},
		"DATETYPE": dremioBuildDateType,
	}
}

// dremioBuildFormattedTime ports build_formatted_time (dialects/dialect.py:1592-1618) without
// a default: a string-literal format converts through the dialect's TIME_MAPPING.
func dremioBuildFormattedTime(d *Dialect, kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		this := dremioSeqGet(args, 0)
		format := dremioSeqGet(args, 1)
		built := exp.Args{"this": this}
		if format != nil {
			if format.IsString() {
				converted, ok := FormatTime(format.Name(), d.TimeMapping)
				if !ok {
					converted = "None"
				}
				format = exp.LiteralString(converted)
			}
			built["format"] = format
		}
		return exp.New(kind, built)
	}
}

// dremioBuildDateDelta ports build_date_delta_with_cast_interval
// (parsers/dremio.py:41-62) with the shared build_date_delta fallback.
func dremioBuildDateDelta(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		if len(args) == 2 {
			dateArg, intervalArg := args[0], args[1]
			if intervalArg != nil && intervalArg.Kind() == exp.KindCast {
				to := dremioExpr(intervalArg.Arg("to"))
				if to != nil && to.Kind() == exp.KindDataType {
					inner := to.This()
					if inner != nil && inner.Kind() == exp.KindInterval {
						return exp.New(kind, exp.Args{
							"this":       dateArg,
							"expression": intervalArg.This(),
							"unit":       inner.Arg("unit"),
						})
					}
				}
			}
			return exp.New(kind, exp.Args{"this": dateArg, "expression": intervalArg})
		}
		// build_date_delta fallback (dialects/dialect.py:1642-1653): unit-based when there
		// are at least three arguments, defaulting to DAY otherwise.
		unitBased := len(args) >= 3
		this := dremioSeqGet(args, 0)
		if unitBased {
			this = dremioSeqGet(args, 2)
		}
		unit := exp.Expression(exp.LiteralString("DAY"))
		if unitBased {
			unit = exp.Var(exp.Args{"this": dremioSeqGet(args, 0).Name()})
		}
		return exp.New(kind, exp.Args{"this": this, "expression": dremioSeqGet(args, 1), "unit": unit})
	}
}

// dremioBuildToChar ports build_timetostr_or_tochar (dialects/dialect.py:2586-2599) plus the
// is_numeric marking (parsers/dremio.py:19-31): a statically temporal first argument builds a
// TimeToStr with the converted format, everything else a ToChar; a literal format containing
// '#' marks the ToChar as numeric.
func dremioBuildToChar(d *Dialect) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		if len(args) == 2 && dremioStaticTemporal(dremioSeqGet(args, 0)) {
			// build_formatted_time(exp.TimeToStr, default=True): a missing format uses the
			// dialect TIME_FORMAT, already in strftime form.
			if dremioSeqGet(args, 1) == nil {
				args = append(args, exp.LiteralString("%Y-%m-%d %H:%M:%S"))
			}
			return dremioBuildFormattedTime(d, exp.KindTimeToStr)(args)
		}
		built := exp.FromArgList(exp.KindToChar, args)
		if format := dremioSeqGet(args, 1); format != nil && format.IsString() {
			if containsHash(format.Name()) {
				built.Set("is_numeric", true)
			}
		}
		return built
	}
}

// dremioBuildDateType ports datetype_handler (parsers/dremio.py:65-84): three integer
// literals fold into a DATE literal, anything else casts a concatenation to DATE.
func dremioBuildDateType(args []exp.Expression) exp.Expression {
	year, month, day := dremioSeqGet(args, 0), dremioSeqGet(args, 1), dremioSeqGet(args, 2)
	if dremioIntLiteral(year) && dremioIntLiteral(month) && dremioIntLiteral(day) {
		y, _ := parseIntLiteral(year)
		m, _ := parseIntLiteral(month)
		d, _ := parseIntLiteral(day)
		return exp.Date(exp.Args{"this": exp.LiteralString(dremioDateString(y, m, d))})
	}
	concat := exp.Concat(exp.Args{
		"expressions": []exp.Expression{
			year,
			exp.LiteralString("-"),
			month,
			exp.LiteralString("-"),
			day,
		},
		"coalesce": true,
	})
	return exp.Cast(exp.Args{
		"this": concat,
		"to":   exp.DataType(exp.Args{"this": exp.DTypeDate}),
	})
}

// dremioStaticTemporal reports whether expression's type is statically known to be temporal
// (the type-annotation result build_timetostr_or_tochar relies on): a cast or typed literal to
// a DATE/TIME/TIMESTAMP family type.
func dremioStaticTemporal(expression exp.Expression) bool {
	if expression == nil || expression.Kind() != exp.KindCast {
		return false
	}
	to := dremioExpr(expression.Arg("to"))
	if to == nil || to.Kind() != exp.KindDataType {
		return false
	}
	dtype, _ := to.Arg("this").(exp.DType)
	switch dtype {
	case exp.DTypeDate, exp.DTypeDatetime, exp.DTypeTimestamp, exp.DTypeTimestampNtz,
		exp.DTypeTimestampTz, exp.DTypeTimestampLtz, exp.DTypeTime, exp.DTypeTimeTz:
		return true
	}
	return false
}

func dremioIntLiteral(expression exp.Expression) bool {
	value, ok := parseIntLiteral(expression)
	return ok && value >= 0
}

func parseIntLiteral(expression exp.Expression) (int, bool) {
	if expression == nil || expression.Kind() != exp.KindLiteral || expression.IsString() {
		return 0, false
	}
	value, err := strconv.Atoi(expression.Name())
	return value, err == nil
}

func dremioDateString(year, month, day int) string {
	pad := func(v int, width int) string {
		digits := strconv.Itoa(v)
		for len(digits) < width {
			digits = "0" + digits
		}
		return digits
	}
	return pad(year, 4) + "-" + pad(month, 2) + "-" + pad(day, 2)
}

func containsHash(value string) bool {
	for _, char := range value {
		if char == '#' {
			return true
		}
	}
	return false
}

// dremioSeqGet returns args[i] or nil.
func dremioSeqGet(args []exp.Expression, i int) exp.Expression {
	if i >= 0 && i < len(args) {
		return args[i]
	}
	return nil
}

// dremioExpr returns the argument value as an expression, or nil.
func dremioExpr(value any) exp.Expression {
	if expr, ok := value.(exp.Expression); ok && expr != nil {
		return expr
	}
	return nil
}
