package dialects

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// oracle_funcs.go ports the FUNCTIONS overlay in
// .reference/sqlglot-v30.17.0/sqlglot/parsers/oracle.py:38-56 and its builders
// (parsers/oracle.py:23-30, dialect.py:1592-1617, 1907-1943, 2586-2602).

func oracleSeqGet(args []exp.Expression, i int) exp.Expression {
	if i < 0 || i >= len(args) {
		return nil
	}
	return args[i]
}

// oracleFormatLiteral converts an Oracle format string literal argument to its strftime
// equivalent, leaving non-literals untouched (dialect.py:1617).
func oracleFormatLiteral(format exp.Expression) exp.Expression {
	if format == nil || !format.IsString() {
		return format
	}
	return exp.LiteralString(OracleFormatTime(format.Name()))
}

// oracleBuildTimeToStrOrToChar ports build_timetostr_or_tochar (dialect.py:2586-2602)
// without type annotation: two arguments become TimeToStr, everything else is TO_CHAR.
func oracleBuildTimeToStrOrToChar(args []exp.Expression) exp.Expression {
	if len(args) != 2 {
		return exp.Anonymous(exp.Args{"this": "TO_CHAR", "expressions": args})
	}
	return exp.TimeToStr(exp.Args{
		"this":   oracleSeqGet(args, 0),
		"format": oracleFormatLiteral(oracleSeqGet(args, 1)),
	})
}

// oracleBuildToTimestamp ports _build_to_timestamp (parsers/oracle.py:23-30).
func oracleBuildToTimestamp(args []exp.Expression) exp.Expression {
	if len(args) == 1 {
		return exp.Anonymous(exp.Args{"this": "TO_TIMESTAMP", "expressions": args})
	}
	return exp.StrToTime(exp.Args{
		"this":   oracleSeqGet(args, 0),
		"format": oracleFormatLiteral(oracleSeqGet(args, 1)),
	})
}

func oracleDTypeOf(v any) exp.DType {
	if e, ok := v.(exp.Expression); ok {
		if dt, ok := e.Arg("this").(exp.DType); ok {
			return dt
		}
	}
	return ""
}

func oracleTemporalDType(dt exp.DType) bool {
	switch dt {
	case exp.DTypeDate, exp.DTypeDatetime, exp.DTypeDatetime2, exp.DTypeDatetime64,
		exp.DTypeTimestamp, exp.DTypeTimestampTz, exp.DTypeTimestampNtz, exp.DTypeTimestampLtz,
		exp.DTypeSmallDatetime, exp.DTypeTime:
		return true
	}
	return false
}

// oracleIsTemporal reports whether e is recognizably a date/timestamp expression, replacing
// build_trunc's annotate_types check (dialect.py:1907-1943) with a kind check.
func oracleIsTemporal(e exp.Expression) bool {
	if e == nil {
		return false
	}
	switch e.Kind() {
	case exp.KindCurrentTimestamp, exp.KindCurrentDate, exp.KindSystimestamp,
		exp.KindDateStrToDate, exp.KindStrToDate, exp.KindStrToTime, exp.KindTimeStrToTime,
		exp.KindDateAdd, exp.KindDateSub, exp.KindTimestampAdd, exp.KindTimestampSub,
		exp.KindDateTrunc, exp.KindTimestampTrunc, exp.KindTsOrDsToDate:
		return true
	case exp.KindCast:
		return oracleTemporalDType(oracleDTypeOf(e.Arg("to")))
	}
	return false
}

// oracleBuildTrunc ports build_trunc (dialect.py:1907-1943) for Oracle's overloaded TRUNC.
// The date-vs-number distinction uses a kind check instead of annotate_types; the rendered
// SQL is identical for both overloads except for the default DD unit.
func oracleBuildTrunc(args []exp.Expression) exp.Expression {
	this := oracleSeqGet(args, 0)
	second := oracleSeqGet(args, 1)

	if len(args) == 1 && oracleIsTemporal(this) {
		return exp.DateTrunc(exp.Args{"this": this, "unit": exp.LiteralString("DD")})
	}
	if second != nil {
		if second.Kind() == exp.KindLiteral && second.IsString() {
			return exp.New(exp.KindDateTrunc, exp.Args{"this": this, "unit": second})
		}
		if second.Kind() == exp.KindLiteral {
			return exp.New(exp.KindTrunc, exp.Args{"this": this, "decimals": second})
		}
	}
	return exp.Anonymous(exp.Args{"this": "TRUNC", "expressions": args})
}

// oracleBuildNVL ports build_coalesce with is_nvl=True.
func oracleBuildNVL(args []exp.Expression) exp.Expression {
	return exp.Coalesce(exp.Args{
		"this":        oracleSeqGet(args, 0),
		"expressions": args[1:],
		"is_nvl":      true,
	})
}

// oracleFunctions returns the Oracle FUNCTIONS additions.
func oracleFunctions() map[string]func([]exp.Expression) exp.Expression {
	return map[string]func([]exp.Expression) exp.Expression{
		"CONVERT": func(args []exp.Expression) exp.Expression {
			return exp.ConvertToCharset(exp.Args{
				"this":   oracleSeqGet(args, 0),
				"dest":   oracleSeqGet(args, 1),
				"source": oracleSeqGet(args, 2),
			})
		},
		"L2_DISTANCE": clickhouseFromArgList(exp.KindEuclideanDistance),
		"NVL":         oracleBuildNVL,
		"SQUARE": func(args []exp.Expression) exp.Expression {
			return exp.Pow(exp.Args{
				"this":       oracleSeqGet(args, 0),
				"expression": exp.LiteralNumber("2"),
			})
		},
		"TO_CHAR":      oracleBuildTimeToStrOrToChar,
		"TO_TIMESTAMP": oracleBuildToTimestamp,
		"TO_DATE": func(args []exp.Expression) exp.Expression {
			return exp.StrToDate(exp.Args{
				"this":   oracleSeqGet(args, 0),
				"format": oracleFormatLiteral(oracleSeqGet(args, 1)),
			})
		},
		"TRUNC": oracleBuildTrunc,
	}
}
