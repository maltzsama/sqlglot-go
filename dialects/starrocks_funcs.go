package dialects

import (
	exp "github.com/ridi-oss/sqlglot-go/expressions"
)

// starrocks_funcs.go ports the FUNCTIONS overlay in
// .reference/sqlglot-v30.17.0/sqlglot/parsers/starrocks.py:16-34 and the builders it uses.

// starRocksBuildDateDeltaWithInterval ports build_date_delta_with_interval
// (dialects/dialect.py:1658-1679).
func starRocksBuildDateDeltaWithInterval(kind exp.Kind, defaultUnit string) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		if len(args) < 2 {
			return nil
		}
		interval := args[1]
		if interval == nil || interval.Kind() != exp.KindInterval {
			if defaultUnit == "" {
				return nil
			}
			return exp.New(kind, exp.Args{
				"this":       args[0],
				"expression": interval,
				"unit":       exp.LiteralString(defaultUnit),
			})
		}
		return exp.New(kind, exp.Args{
			"this":       args[0],
			"expression": interval.This(),
			"unit":       starRocksUnitToStr(interval, "DAY"),
		})
	}
}

// starRocksUnitToStr ports unit_to_str (dialects/dialect.py:2059-2074) without WeekStart.
func starRocksUnitToStr(expression exp.Expression, defaultUnit string) exp.Expression {
	unit := expression.Arg("unit")
	if unit == nil {
		if defaultUnit == "" {
			return nil
		}
		return exp.LiteralString(defaultUnit)
	}
	unitExpr, ok := unit.(exp.Expression)
	if !ok {
		return nil
	}
	if unitExpr.Kind() == exp.KindPlaceholder {
		return unitExpr
	}
	if unitExpr.Kind() != exp.KindVar && unitExpr.Kind() != exp.KindLiteral {
		return unitExpr
	}
	return exp.LiteralString(unitExpr.Name())
}

// starRocksBuildTimestampTrunc ports build_timestamp_trunc (dialects/dialect.py:1903-1904).
func starRocksBuildTimestampTrunc(args []exp.Expression) exp.Expression {
	return exp.TimestampTrunc(exp.Args{
		"this": starRocksSeqGet(args, 1),
		"unit": starRocksSeqGet(args, 0),
	})
}

func starRocksSeqGet(args []exp.Expression, i int) exp.Expression {
	if i < 0 || i >= len(args) {
		return nil
	}
	return args[i]
}

// starRocksFunctions returns the StarRocks FUNCTIONS additions.
func starRocksFunctions() map[string]func([]exp.Expression) exp.Expression {
	return map[string]func([]exp.Expression) exp.Expression{
		"ADDDATE":    starRocksBuildDateDeltaWithInterval(exp.KindDateAdd, "DAY"),
		"DATE_ADD":   starRocksBuildDateDeltaWithInterval(exp.KindDateAdd, "DAY"),
		"DATE_SUB":   starRocksBuildDateDeltaWithInterval(exp.KindDateSub, "DAY"),
		"SUBDATE":    starRocksBuildDateDeltaWithInterval(exp.KindDateSub, "DAY"),
		"DATE_TRUNC": starRocksBuildTimestampTrunc,
		"DATEDIFF": func(args []exp.Expression) exp.Expression {
			return exp.DateDiff(exp.Args{
				"this":       starRocksSeqGet(args, 0),
				"expression": starRocksSeqGet(args, 1),
				"unit":       exp.LiteralString("DAY"),
			})
		},
		"DATE_DIFF": func(args []exp.Expression) exp.Expression {
			return exp.DateDiff(exp.Args{
				"this":       starRocksSeqGet(args, 1),
				"expression": starRocksSeqGet(args, 2),
				"unit":       starRocksSeqGet(args, 0),
			})
		},
		"ARRAY_FLATTEN": clickhouseFromArgList(exp.KindFlatten),
		"REGEXP":        clickhouseFromArgList(exp.KindRegexpLike),
		"MAP":           buildVarMap,
	}
}
