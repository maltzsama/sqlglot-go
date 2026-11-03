package dialects

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	exp "github.com/ridi-oss/sqlglot-go/expressions"
)

// tsql_funcs.go ports the FUNCTIONS overlay in
// .reference/sqlglot-v30.17.0/sqlglot/parsers/tsql.py:348-404 (plus the builders at :22-318).

func tsqlSeqGet(args []exp.Expression, i int) exp.Expression {
	if i < 0 || i >= len(args) {
		return nil
	}
	return args[i]
}

// DATE_DELTA_INTERVAL (parsers/tsql.py:31-47).
var tsqlDateDeltaInterval = map[string]string{
	"year": "year", "yyyy": "year", "yy": "year",
	"quarter": "quarter", "qq": "quarter", "q": "quarter",
	"month": "month", "mm": "month", "m": "month",
	"week": "week", "ww": "week", "wk": "week",
	"day": "day", "dd": "day", "d": "day",
}

var tsqlDefaultStartDate = time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)

func tsqlBuildDateDelta(kind exp.Kind, bigInt bool) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		unit := tsqlSeqGet(args, 0)
		if unit != nil {
			mapped := tsqlDateDeltaInterval[strings.ToLower(unit.Name())]
			if mapped == "" {
				mapped = unit.Name()
			}
			unit = exp.Var(exp.Args{"this": mapped})
		}
		start := tsqlSeqGet(args, 1)
		if start != nil && start.IsNumber() {
			if start.IsInt() {
				days, err := strconv.ParseInt(start.Name(), 10, 64)
				if start.Kind() == exp.KindNeg {
					if inner := start.This(); inner != nil {
						if n, err2 := strconv.ParseInt(inner.Name(), 10, 64); err2 == nil {
							days, err = -n, nil
						}
					}
				}
				if err == nil {
					start = exp.LiteralString(tsqlDefaultStartDate.AddDate(0, 0, int(days)).Format("2006-01-02"))
				}
			} else {
				built := exp.Args{
					"this":       tsqlSeqGet(args, 2),
					"expression": start,
					"unit":       unit,
				}
				if bigInt {
					built["big_int"] = true
				}
				return exp.New(kind, built)
			}
		}
		built := exp.Args{
			"this":       exp.TimeStrToTime(exp.Args{"this": tsqlSeqGet(args, 2)}),
			"expression": exp.TimeStrToTime(exp.Args{"this": start}),
			"unit":       unit,
		}
		if bigInt {
			built["big_int"] = true
		}
		return exp.New(kind, built)
	}
}

// tsqlBuildDateAddGeneric ports build_date_delta (dialects/dialect.py:1636-1655), the
// builder TSQL uses for DATEADD: no TimeStrToTime wrapping or numeric-date conversion.
func tsqlBuildDateAddGeneric(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		unit := tsqlSeqGet(args, 0)
		if unit != nil {
			mapped := tsqlDateDeltaInterval[strings.ToLower(unit.Name())]
			if mapped == "" {
				mapped = unit.Name()
			}
			unit = exp.Var(exp.Args{"this": mapped})
		}
		return exp.New(kind, exp.Args{
			"this":       tsqlSeqGet(args, 2),
			"expression": tsqlSeqGet(args, 1),
			"unit":       unit,
		})
	}
}

// TSQL TIME_MAPPING / FORMAT_TIME_MAPPING (dialects/tsql.py:50-140).
var tsqlTimeMapping = map[string]string{
	"year": "%Y", "dayofyear": "%j", "day": "%d", "dy": "%d", "y": "%Y",
	"week": "%W", "ww": "%W", "wk": "%W", "isowk": "%V", "isoww": "%V", "iso_week": "%V",
	"hour": "%h", "hh": "%I", "minute": "%M", "mi": "%M", "n": "%M",
	"second": "%S", "ss": "%S", "s": "%-S", "millisecond": "%f", "ms": "%f",
	"weekday": "%w", "dw": "%w", "month": "%m", "mm": "%M", "m": "%-M",
	"Y": "%Y", "YYYY": "%Y", "YY": "%y", "MMMM": "%B", "MMM": "%b",
	"MM": "%m", "M": "%-m", "dddd": "%A", "dd": "%d", "d": "%-d",
	"HH": "%H", "H": "%-H", "h": "%-I", "ffffff": "%f", "yyyy": "%Y", "yy": "%y",
}

var tsqlFullFormatTimeMapping = map[string]string{
	"weekday": "%A", "dw": "%A", "w": "%A", "month": "%B", "mm": "%B", "m": "%B",
}

var tsqlFormatTimeMapping = map[string]string{
	"y": "%B %Y", "d": "%m/%d/%Y", "H": "%-H", "h": "%-I", "s": "%Y-%m-%d %H:%M:%S",
	"D": "%A,%B,%Y", "f": "%A,%B,%Y %-I:%M %p", "F": "%A,%B,%Y %-I:%M:%S %p",
	"g": "%m/%d/%Y %-I:%M %p", "G": "%m/%d/%Y %-I:%M:%S %p", "M": "%B %-d",
	"m": "%B %-d", "O": "%Y-%m-%dT%H:%M:%S", "u": "%Y-%M-%D %H:%M:%S%z",
	"U": "%A, %B %D, %Y %H:%M:%S%z", "T": "%-I:%M:%S %p", "t": "%-I:%M", "Y": "%a %Y",
}

// tsqlConvertFormat applies a format_time mapping with longest-match-first semantics.
func tsqlConvertFormat(format string, mapping map[string]string) string {
	var b strings.Builder
	for i := 0; i < len(format); {
		matched := false
		for length := 8; length >= 1; length-- {
			if i+length > len(format) {
				continue
			}
			if replacement, ok := mapping[format[i:i+length]]; ok {
				b.WriteString(replacement)
				i += length
				matched = true
				break
			}
		}
		if !matched {
			b.WriteByte(format[i])
			i++
		}
	}
	return b.String()
}

// tsqlTimeMappingOrder replays the upstream dict insertion order (dialects/tsql.py:50-140)
// so INVERSE_TIME_MAPPING resolves collisions deterministically (last key wins).
var tsqlTimeMappingOrder = []string{
	"year", "dayofyear", "day", "dy", "y", "week", "ww", "wk", "isowk", "isoww", "iso_week",
	"hour", "hh", "minute", "mi", "n", "second", "ss", "s", "millisecond", "ms", "weekday",
	"dw", "month", "mm", "m", "Y", "YYYY", "YY", "MMMM", "MMM", "MM", "M", "dddd", "dd", "d",
	"HH", "H", "h", "ffffff", "yyyy", "yy",
}

var tsqlInverseTimeMapping = func() map[string]string {
	inverse := map[string]string{}
	for _, key := range tsqlTimeMappingOrder {
		inverse[tsqlTimeMapping[key]] = key
	}
	return inverse
}()

// TSQLFormatTime ports format_time with TSQL's inverse TIME_MAPPING (used by _format_sql).
func TSQLFormatTime(format string) string {
	return tsqlConvertFormat(format, tsqlInverseTimeMapping)
}

func tsqlBuildFormattedTime(full bool) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		format := tsqlSeqGet(args, 0)
		if format != nil && format.Kind() != exp.KindNull {
			mapping := tsqlTimeMapping
			if full {
				mapping = map[string]string{}
				for k, v := range tsqlTimeMapping {
					mapping[k] = v
				}
				for k, v := range tsqlFullFormatTimeMapping {
					mapping[k] = v
				}
			}
			format = exp.LiteralString(tsqlConvertFormat(strings.ToLower(format.Name()), mapping))
		}
		this := tsqlSeqGet(args, 1)
		if this != nil {
			this = exp.Cast(exp.Args{"this": this, "to": exp.DTypeDatetime2.IntoExpr(nil)})
		}
		return exp.New(exp.KindTimeToStr, exp.Args{"this": this, "format": format})
	}
}

var tsqlDateFmtRE = regexp.MustCompile(`([dD]{1,2})|([mM]{1,2})|([yY]{1,4})|([hH]{1,2})|([sS]{1,2})`)

// tsqlBuildFormat ports _build_format (parsers/tsql.py:155-174).
func tsqlBuildFormat(args []exp.Expression) exp.Expression {
	this := tsqlSeqGet(args, 0)
	format := tsqlSeqGet(args, 1)
	culture := tsqlSeqGet(args, 2)

	numberFormat := format != nil
	if numberFormat {
		name := format.Name()
		if name != "N" && name != "C" && tsqlDateFmtRE.MatchString(name) {
			numberFormat = false
		}
	}
	if numberFormat {
		return exp.NumberToStr(exp.Args{"this": this, "format": format, "culture": culture})
	}
	if format != nil {
		name := format.Name()
		mapping := tsqlTimeMapping
		if len(name) == 1 {
			mapping = tsqlFormatTimeMapping
		}
		format = exp.LiteralString(tsqlConvertFormat(name, mapping))
	}
	return exp.TimeToStr(exp.Args{"this": this, "format": format, "culture": culture})
}

// tsqlBuildEoMonth ports _build_eomonth (parsers/tsql.py:177-187).
func tsqlBuildEoMonth(args []exp.Expression) exp.Expression {
	date := exp.TsOrDsToDate(exp.Args{"this": tsqlSeqGet(args, 0)})
	monthLag := tsqlSeqGet(args, 1)
	this := date
	if monthLag != nil {
		this = exp.DateAdd(exp.Args{
			"this":       date,
			"expression": monthLag,
			"unit":       exp.Var(exp.Args{"this": tsqlDateDeltaInterval["month"]}),
		})
	}
	return exp.LastDay(exp.Args{"this": this})
}

// tsqlBuildHashBytes ports _build_hashbytes (parsers/tsql.py:190-205).
func tsqlBuildHashBytes(args []exp.Expression) exp.Expression {
	var kind, data exp.Expression
	if len(args) > 0 {
		kind = args[0]
	}
	if len(args) > 1 {
		data = args[1]
	}
	name := ""
	if kind != nil && kind.IsString() {
		name = strings.ToUpper(kind.Name())
	}
	switch name {
	case "MD5":
		return exp.MD5(exp.Args{"this": data})
	case "SHA", "SHA1":
		return exp.SHA(exp.Args{"this": data})
	case "SHA2_256":
		return exp.SHA2(exp.Args{"this": data, "length": exp.LiteralNumber(256)})
	case "SHA2_512":
		return exp.SHA2(exp.Args{"this": data, "length": exp.LiteralNumber(512)})
	}
	return exp.Anonymous(exp.Args{"this": "HASHBYTES", "expressions": args})
}

// tsqlBuildDateTimeFromParts ports _build_datetimefromparts (parsers/tsql.py:240-249).
func tsqlBuildDateTimeFromParts(args []exp.Expression) exp.Expression {
	return exp.TimestampFromParts(exp.Args{
		"year":  tsqlSeqGet(args, 0),
		"month": tsqlSeqGet(args, 1),
		"day":   tsqlSeqGet(args, 2),
		"hour":  tsqlSeqGet(args, 3),
		"min":   tsqlSeqGet(args, 4),
		"sec":   tsqlSeqGet(args, 5),
		"milli": tsqlSeqGet(args, 6),
	})
}

// tsqlBuildTimeFromParts ports _build_timefromparts (parsers/tsql.py:253-260).
func tsqlBuildTimeFromParts(args []exp.Expression) exp.Expression {
	return exp.TimeFromParts(exp.Args{
		"hour":      tsqlSeqGet(args, 0),
		"min":       tsqlSeqGet(args, 1),
		"sec":       tsqlSeqGet(args, 2),
		"fractions": tsqlSeqGet(args, 3),
		"precision": tsqlSeqGet(args, 4),
	})
}

// tsqlBuildWithArgAsText ports _build_with_arg_as_text (parsers/tsql.py:263-280).
func tsqlBuildWithArgAsText(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		this := tsqlSeqGet(args, 0)
		if this != nil && !this.IsString() {
			this = exp.Cast(exp.Args{"this": this, "to": exp.DTypeText.IntoExpr(nil)})
		}
		built := exp.Args{"this": this}
		if expression := tsqlSeqGet(args, 1); expression != nil {
			built["expression"] = expression
		}
		return exp.New(kind, built)
	}
}

// tsqlBuildParseName ports _build_parsename (parsers/tsql.py:284-299).
func tsqlBuildParseName(args []exp.Expression) exp.Expression {
	if len(args) == 2 && args[0].Kind() == exp.KindLiteral && args[1].Kind() == exp.KindLiteral {
		this := args[0]
		partIndex := args[1]
		splitCount := len(strings.Split(this.Name(), "."))
		if splitCount <= 4 {
			index, _ := strconv.Atoi(partIndex.Name())
			return exp.SplitPart(exp.Args{
				"this":       this,
				"delimiter":  exp.LiteralString("."),
				"part_index": exp.LiteralNumber(splitCount + 1 - index),
			})
		}
	}
	return exp.Anonymous(exp.Args{"this": "PARSENAME", "expressions": args})
}

// tsqlBuildJSONExtract approximates parser.build_extract_json_with_path for TSQL's
// JSON_QUERY / JSON_VALUE (parsers/tsql.py:302-308, 378).
func tsqlBuildJSONExtract(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		path := tsqlSeqGet(args, 1)
		if path == nil {
			path = exp.LiteralString("$")
		}
		return exp.New(kind, exp.Args{"this": tsqlSeqGet(args, 0), "expression": path})
	}
}

// tsqlBuildDateTrunc ports _build_datetrunc (parsers/tsql.py:311-318).
func tsqlBuildDateTrunc(args []exp.Expression) exp.Expression {
	unit := tsqlSeqGet(args, 0)
	this := tsqlSeqGet(args, 1)
	if this != nil && this.IsString() {
		this = exp.Cast(exp.Args{"this": this, "to": exp.DTypeDatetime2.IntoExpr(nil)})
	}
	return exp.TimestampTrunc(exp.Args{"this": this, "unit": unit})
}

func tsqlDayMonthYear(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		return exp.New(kind, exp.Args{"this": exp.TsOrDsToDate(exp.Args{
			"this":         tsqlSeqGet(args, 0),
			"default_date": exp.LiteralString("1900-01-01"),
		})})
	}
}

// tsqlFunctions ports the FUNCTIONS overlay (parsers/tsql.py:348-404).
func tsqlFunctions() map[string]func([]exp.Expression) exp.Expression {
	return map[string]func([]exp.Expression) exp.Expression{
		"ATN2": exp.FromArgListFunc(exp.KindAtan2),
		"CHARINDEX": func(args []exp.Expression) exp.Expression {
			return exp.New(exp.KindStrPosition, exp.Args{
				"this":     tsqlSeqGet(args, 1),
				"substr":   tsqlSeqGet(args, 0),
				"position": tsqlSeqGet(args, 2),
			})
		},
		"COUNT": func(args []exp.Expression) exp.Expression {
			return exp.New(exp.KindCount, exp.Args{"this": tsqlSeqGet(args, 0), "expressions": args[1:], "big_int": false})
		},
		"COUNT_BIG": func(args []exp.Expression) exp.Expression {
			return exp.New(exp.KindCount, exp.Args{"this": tsqlSeqGet(args, 0), "expressions": args[1:], "big_int": true})
		},
		"DATEADD":           tsqlBuildDateAddGeneric(exp.KindDateAdd),
		"DATEDIFF":          tsqlBuildDateDelta(exp.KindDateDiff, false),
		"DATEDIFF_BIG":      tsqlBuildDateDelta(exp.KindDateDiff, true),
		"DATENAME":          tsqlBuildFormattedTime(true),
		"DATETIMEFROMPARTS": tsqlBuildDateTimeFromParts,
		"DAY":               tsqlDayMonthYear(exp.KindDay),
		"EOMONTH":           tsqlBuildEoMonth,
		"FORMAT":            tsqlBuildFormat,
		"GETDATE":           exp.FromArgListFunc(exp.KindCurrentTimestamp),
		"HASHBYTES":         tsqlBuildHashBytes,
		"ISNULL": func(args []exp.Expression) exp.Expression {
			return exp.Coalesce(exp.Args{"this": tsqlSeqGet(args, 0), "expressions": args[1:], "is_null": true})
		},
		"JSON_QUERY":  tsqlBuildJSONExtract(exp.KindJSONExtract),
		"JSON_VALUE":  tsqlBuildJSONExtract(exp.KindJSONExtractScalar),
		"LEN":         tsqlBuildWithArgAsText(exp.KindLength),
		"LEFT":        tsqlBuildWithArgAsText(exp.KindLeft),
		"MONTH":       tsqlDayMonthYear(exp.KindMonth),
		"NEWID":       exp.FromArgListFunc(exp.KindUuid),
		"RIGHT":       tsqlBuildWithArgAsText(exp.KindRight),
		"PARSENAME":   tsqlBuildParseName,
		"REPLICATE":   exp.FromArgListFunc(exp.KindRepeat),
		"SCHEMA_NAME": exp.FromArgListFunc(exp.KindCurrentSchema),
		"SQUARE": func(args []exp.Expression) exp.Expression {
			return exp.Pow(exp.Args{"this": tsqlSeqGet(args, 0), "expression": exp.LiteralNumber(2)})
		},
		"SYSDATETIME":       exp.FromArgListFunc(exp.KindCurrentTimestamp),
		"SUSER_NAME":        exp.FromArgListFunc(exp.KindCurrentUser),
		"SUSER_SNAME":       exp.FromArgListFunc(exp.KindCurrentUser),
		"SYSDATETIMEOFFSET": exp.FromArgListFunc(exp.KindCurrentTimestampLTZ),
		"SYSTEM_USER":       exp.FromArgListFunc(exp.KindCurrentUser),
		"TIMEFROMPARTS":     tsqlBuildTimeFromParts,
		"DATETRUNC":         tsqlBuildDateTrunc,
		"YEAR":              tsqlDayMonthYear(exp.KindYear),
	}
}
