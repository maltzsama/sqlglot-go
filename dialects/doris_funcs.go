package dialects

import (
	exp "github.com/ridi-oss/sqlglot-go/expressions"
)

// doris_funcs.go ports the FUNCTIONS overlay in
// .reference/sqlglot-v30.17.0/sqlglot/parsers/doris.py:27-49 and its _build_date_trunc
// helper (parsers/doris.py:15-25).

// dorisBuildDateTrunc accepts both DATE_TRUNC(datetime, unit) and DATE_TRUNC(unit, datetime).
func dorisBuildDateTrunc(args []exp.Expression) exp.Expression {
	a0, a1 := dorisSeqGet(args, 0), dorisSeqGet(args, 1)
	isUnit := func(e exp.Expression) bool {
		if e == nil || e.Kind() != exp.KindLiteral || !e.IsString() {
			return false
		}
		for _, r := range e.Name() {
			if r >= '0' && r <= '9' {
				return false
			}
		}
		return true
	}
	unit, this := a0, a1
	if !isUnit(a0) {
		unit, this = a1, a0
	}
	return exp.TimestampTrunc(exp.Args{"this": this, "unit": unit})
}

func dorisSeqGet(args []exp.Expression, i int) exp.Expression {
	if i < 0 || i >= len(args) {
		return nil
	}
	return args[i]
}

// dorisFunctions returns the Doris FUNCTIONS additions.
func dorisFunctions() map[string]func([]exp.Expression) exp.Expression {
	return map[string]func([]exp.Expression) exp.Expression{
		"ADDDATE":     starRocksBuildDateDeltaWithInterval(exp.KindDateAdd, "DAY"),
		"COLLECT_SET": clickhouseFromArgList(exp.KindArrayUniqueAgg),
		"DATE_ADD":    starRocksBuildDateDeltaWithInterval(exp.KindDateAdd, "DAY"),
		"DATE_SUB":    starRocksBuildDateDeltaWithInterval(exp.KindDateSub, "DAY"),
		"DATE_TRUNC":  dorisBuildDateTrunc,
		"L2_DISTANCE": clickhouseFromArgList(exp.KindEuclideanDistance),
		"MONTHS_ADD":  clickhouseFromArgList(exp.KindAddMonths),
		"REGEXP":      clickhouseFromArgList(exp.KindRegexpLike),
		"SUBDATE":     starRocksBuildDateDeltaWithInterval(exp.KindDateSub, "DAY"),
		"TO_DATE":     clickhouseFromArgList(exp.KindTsOrDsToDate),
	}
}
