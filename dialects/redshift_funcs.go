package dialects

import (
	"strings"

	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// redshift_funcs.go ports the FUNCTIONS overlay of RedshiftParser
// (parsers/redshift.py:33-68) and its _build_date_delta helper (parsers/redshift.py:18-30).

// mapDatePart ports map_date_part (dialects/dialect.py:2149-2158) for the base dialect's
// DATE_PART_MAPPING, the mapping a parser builder gets when it passes no dialect.
func mapDatePart(part exp.Expression) exp.Expression {
	if part == nil || (part.Kind() == exp.KindColumn && len(part.Parts()) != 1) {
		return part
	}
	mapped := Base().DatePartMapping[strings.ToUpper(part.Name())]
	if mapped == "" {
		return part
	}
	if part.IsString() {
		return exp.LiteralString(mapped)
	}
	return exp.Var(exp.Args{"this": mapped})
}

// redshiftBuildDateDelta ports _build_date_delta: DATEADD(unit, n, date) and friends, with the
// unit mapped through DATE_PART_MAPPING. TsOrDsAdd also returns a TIMESTAMP.
func redshiftBuildDateDelta(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		e := exp.New(kind, exp.Args{
			"this":       seqGet(args, 2),
			"expression": seqGet(args, 1),
			"unit":       mapDatePart(seqGet(args, 0)),
		})
		if kind == exp.KindTsOrDsAdd {
			e.Set("return_type", exp.DTypeTimestamp.IntoExpr(nil))
		}
		return e
	}
}

// buildConvertTimezone ports build_convert_timezone (parser.py:158-167): the two-argument
// form takes defaultSourceTZ as its source time zone.
func buildConvertTimezone(defaultSourceTZ string) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		if len(args) == 2 {
			var sourceTZ exp.Expression
			if defaultSourceTZ != "" {
				sourceTZ = exp.LiteralString(defaultSourceTZ)
			}
			return exp.ConvertTimezone(exp.Args{"source_tz": sourceTZ, "target_tz": args[0], "timestamp": args[1]})
		}
		return exp.FromArgList(exp.KindConvertTimezone, args)
	}
}

// redshiftFunctions returns the RedshiftParser FUNCTIONS additions.
func redshiftFunctions() map[string]func([]exp.Expression) exp.Expression {
	return map[string]func([]exp.Expression) exp.Expression{
		"ADD_MONTHS": func(args []exp.Expression) exp.Expression {
			return exp.TsOrDsAdd(exp.Args{
				"this":        seqGet(args, 0),
				"expression":  seqGet(args, 1),
				"unit":        exp.Var(exp.Args{"this": "month"}),
				"return_type": exp.DTypeTimestamp.IntoExpr(nil),
			})
		},
		"CONVERT_TIMEZONE": buildConvertTimezone("UTC"),
		"DATEADD":          redshiftBuildDateDelta(exp.KindTsOrDsAdd),
		"DATE_ADD":         redshiftBuildDateDelta(exp.KindTsOrDsAdd),
		"DATEDIFF":         redshiftBuildDateDelta(exp.KindTsOrDsDiff),
		"DATE_DIFF":        redshiftBuildDateDelta(exp.KindTsOrDsDiff),
		"GETDATE":          exp.FromArgListFunc(exp.KindCurrentTimestamp),
		"LISTAGG":          exp.FromArgListFunc(exp.KindGroupConcat),
		"REGEXP_SUBSTR": func(args []exp.Expression) exp.Expression {
			return exp.RegexpExtract(exp.Args{
				"this":       seqGet(args, 0),
				"expression": seqGet(args, 1),
				"position":   seqGet(args, 2),
				"occurrence": seqGet(args, 3),
				"parameters": seqGet(args, 4),
			})
		},
		"SPLIT_TO_ARRAY": func(args []exp.Expression) exp.Expression {
			sep := seqGet(args, 1)
			if sep == nil {
				sep = exp.LiteralString(",")
			}
			return exp.StringToArray(exp.Args{"this": seqGet(args, 0), "expression": sep})
		},
		"ARRAY_CONTAINS": func(args []exp.Expression) exp.Expression {
			return exp.ArrayContains(exp.Args{
				"this":       seqGet(args, 0),
				"expression": seqGet(args, 1),
				"check_null": seqGet(args, 2),
			})
		},
		"STRTOL":  exp.FromArgListFunc(exp.KindFromBase),
		"TEXTLEN": exp.FromArgListFunc(exp.KindLength),
		// Length._sql_names = ["LENGTH", "LEN", ...] (string.py:69-71) is base-scope upstream;
		// base deliberately leaves LENGTH/LEN unregistered (functions.go:108-113, MySQL's
		// byte/char split), but Redshift has no such split — LEN and LENGTH are both char
		// length — so registering LEN here makes `LEN(x)` build a Length node that renders as
		// `LENGTH(x)` (the class-name fallback), matching upstream.
		"LEN": exp.FromArgListFunc(exp.KindLength),
	}
}

// redshiftTimeMappingOrder / redshiftTimeMapping port TIME_MAPPING (dialects/redshift.py:
// 30-34): `{**Postgres.TIME_MAPPING, "MON": "%b", "MONTH": "%B"}` — the two Redshift keys
// come last, so the generated inverse resolves %b/%B to MON/MONTH (overwriting Postgres's
// TMMon/TMMonth entries).
var redshiftTimeMappingOrder = append(append([]string{}, postgresTimeMappingOrder...), "MON", "MONTH")

var redshiftTimeMapping = func() map[string]string {
	mapping := make(map[string]string, len(postgresTimeMapping)+2)
	for key, value := range postgresTimeMapping {
		mapping[key] = value
	}
	mapping["MON"] = "%b"
	mapping["MONTH"] = "%B"
	return mapping
}()
