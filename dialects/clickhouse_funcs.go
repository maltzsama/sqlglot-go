package dialects

import (
	exp "github.com/ridi-oss/sqlglot-go/expressions"
)

// clickhouse_funcs.go ports the FUNCTIONS overlay in
// .reference/sqlglot-v30.17.0/sqlglot/parsers/clickhouse.py:249-325, plus the builders in
// that file (parsers/clickhouse.py:23-72).

func clickhouseSeqGet(args []exp.Expression, i int) exp.Expression {
	if i < 0 || i >= len(args) {
		return nil
	}
	return args[i]
}

// clickhouseBuildDateDelta ports build_date_delta (dialects/dialect.py:1636-1655).
func clickhouseBuildDateDelta(kind exp.Kind, defaultUnit string, supportsTimezone bool) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		unitBased := len(args) >= 3
		hasTimezone := len(args) == 4
		this := clickhouseSeqGet(args, 2)
		if !unitBased {
			this = clickhouseSeqGet(args, 0)
		}
		var unit exp.Expression
		if unitBased {
			if u := clickhouseSeqGet(args, 0); u != nil {
				unit = exp.Var(exp.Args{"this": u.Name()})
			}
		} else if defaultUnit != "" {
			unit = exp.LiteralString(defaultUnit)
		}
		node := exp.New(kind, exp.Args{
			"this":       this,
			"expression": clickhouseSeqGet(args, 1),
			"unit":       unit,
		})
		if supportsTimezone && hasTimezone {
			node.Set("zone", args[len(args)-1])
		}
		return node
	}
}

// clickhouseBuildLike ports build_like (dialects/dialect.py:2444-2454).
func clickhouseBuildLike(kind exp.Kind, notLike bool) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		var like exp.Expression = exp.New(kind, exp.Args{
			"this":       clickhouseSeqGet(args, 0),
			"expression": clickhouseSeqGet(args, 1),
		})
		if escape := clickhouseSeqGet(args, 2); escape != nil {
			like = exp.Escape(exp.Args{"this": like, "expression": escape})
		}
		if notLike {
			like = exp.Not(exp.Args{"this": like})
		}
		return like
	}
}

// clickhouseBuildTimestampTrunc ports _build_timestamp_trunc
// (parsers/clickhouse.py:53-56).
func clickhouseBuildTimestampTrunc(unit string) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		return exp.TimestampTrunc(exp.Args{
			"this": clickhouseSeqGet(args, 0),
			"unit": exp.Var(exp.Args{"this": unit}),
			"zone": clickhouseSeqGet(args, 1),
		})
	}
}

// clickhouseBuildCountIf ports _build_count_if (parsers/clickhouse.py:38-42).
func clickhouseBuildCountIf(args []exp.Expression) exp.Expression {
	if len(args) == 1 {
		return exp.CountIf(exp.Args{"this": clickhouseSeqGet(args, 0)})
	}
	return exp.CombinedAggFunc(exp.Args{"this": "countIf", "expressions": args})
}

// buildVarMap ports parser.build_var_map (parser.py): alternating key/value args.
func buildVarMap(args []exp.Expression) exp.Expression {
	keys := make([]exp.Expression, 0, len(args)/2+1)
	values := make([]exp.Expression, 0, len(args)/2+1)
	for i := 0; i < len(args); i += 2 {
		keys = append(keys, args[i])
		if i+1 < len(args) {
			values = append(values, args[i+1])
		}
	}
	return exp.VarMap(exp.Args{
		"keys":   exp.Array(exp.Args{"expressions": keys}),
		"values": exp.Array(exp.Args{"expressions": values}),
	})
}

// clickhouseFromArgList adapts exp.FromArgList to the builder signature.
func clickhouseFromArgList(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression { return exp.FromArgList(kind, args) }
}

// clickhouseSplit ports _build_split (parsers/clickhouse.py:69-72).
func clickhouseSplit(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		return exp.New(kind, exp.Args{
			"this":       clickhouseSeqGet(args, 1),
			"expression": clickhouseSeqGet(args, 0),
			"limit":      clickhouseSeqGet(args, 2),
		})
	}
}

// clickhouseSplitByChar ports _build_split_by_char (parsers/clickhouse.py:59-66).
func clickhouseSplitByChar(args []exp.Expression) exp.Expression {
	sep := clickhouseSeqGet(args, 0)
	if sep != nil && sep.Kind() == exp.KindLiteral && sep.IsString() {
		if len([]rune(sep.Name())) == 1 {
			return exp.Split(exp.Args{
				"this":       clickhouseSeqGet(args, 1),
				"expression": clickhouseSeqGet(args, 0),
				"limit":      clickhouseSeqGet(args, 2),
			})
		}
	}
	return exp.Anonymous(exp.Args{"this": "splitByChar", "expressions": args})
}

// clickhouseSHA2 ports the SHA256/SHA512 lambdas (parsers/clickhouse.py:314-315).
func clickhouseSHA2(length int) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		return exp.SHA2(exp.Args{
			"this":   clickhouseSeqGet(args, 0),
			"length": exp.LiteralNumber(length),
		})
	}
}

// clickhouseFunctions returns the dialect's FUNCTIONS additions/overrides. Entries that
// only re-type a call whose rendering is unchanged upstream (formatDateTime, splitByString,
// JSONExtractString, ...) are intentionally left to the Anonymous fallback, which keeps the
// parsed spelling and round-trips identically; the entries below change the parse shape or
// the rendered argument order.
func clickhouseFunctions() map[string]func([]exp.Expression) exp.Expression {
	fns := map[string]func([]exp.Expression) exp.Expression{
		"ANY":           clickhouseFromArgList(exp.KindAnyValue),
		"ARRAYCOMPACT":  clickhouseFromArgList(exp.KindArrayCompact),
		"ARRAYCONCAT":   clickhouseFromArgList(exp.KindArrayConcat),
		"ARRAYDISTINCT": clickhouseFromArgList(exp.KindArrayDistinct),
		"ARRAYEXCEPT":   clickhouseFromArgList(exp.KindArrayExcept),
		"ARRAYSUM":      clickhouseFromArgList(exp.KindArraySum),
		"ARRAYMAX":      clickhouseFromArgList(exp.KindArrayMax),
		"ARRAYMIN":      clickhouseFromArgList(exp.KindArrayMin),
		"ARRAYREVERSE":  clickhouseFromArgList(exp.KindArrayReverse),
		"ARRAYSLICE":    clickhouseFromArgList(exp.KindArraySlice),
		"ARRAYFILTER": func(args []exp.Expression) exp.Expression {
			return exp.ArrayFilter(exp.Args{"this": clickhouseSeqGet(args, 1), "expression": clickhouseSeqGet(args, 0)})
		},
		"ARRAYMAP": func(args []exp.Expression) exp.Expression {
			return exp.Transform(exp.Args{"this": clickhouseSeqGet(args, 1), "expression": clickhouseSeqGet(args, 0)})
		},
		"CURRENTDATABASE":       clickhouseFromArgList(exp.KindCurrentDatabase),
		"CURRENTSCHEMAS":        clickhouseFromArgList(exp.KindCurrentSchemas),
		"COUNTIF":               clickhouseBuildCountIf,
		"CITYHASH64":            clickhouseFromArgList(exp.KindCityHash64),
		"COSINEDISTANCE":        clickhouseFromArgList(exp.KindCosineDistance),
		"VERSION":               clickhouseFromArgList(exp.KindCurrentVersion),
		"DATE_ADD":              clickhouseBuildDateDelta(exp.KindDateAdd, "", false),
		"DATEADD":               clickhouseBuildDateDelta(exp.KindDateAdd, "", false),
		"DATE_DIFF":             clickhouseBuildDateDelta(exp.KindDateDiff, "", true),
		"DATEDIFF":              clickhouseBuildDateDelta(exp.KindDateDiff, "", true),
		"DATE_SUB":              clickhouseBuildDateDelta(exp.KindDateSub, "", false),
		"DATESUB":               clickhouseBuildDateDelta(exp.KindDateSub, "", false),
		"DATETRUNC":             clickhouseFromArgList(exp.KindDateTrunc),
		"HAS":                   clickhouseFromArgList(exp.KindArrayContains),
		"ILIKE":                 clickhouseBuildLike(exp.KindILike, false),
		"LIKE":                  clickhouseBuildLike(exp.KindLike, false),
		"NOTLIKE":               clickhouseBuildLike(exp.KindLike, true),
		"L2Distance":            clickhouseFromArgList(exp.KindEuclideanDistance),
		"MAP":                   buildVarMap,
		"MATCH":                 clickhouseFromArgList(exp.KindRegexpLike),
		"RANDCANONICAL":         clickhouseFromArgList(exp.KindRand),
		"SUBSTRINGINDEX":        clickhouseFromArgList(exp.KindSubstringIndex),
		"TIMESTAMP_SUB":         clickhouseBuildDateDelta(exp.KindTimestampSub, "", false),
		"TIMESTAMPSUB":          clickhouseBuildDateDelta(exp.KindTimestampSub, "", false),
		"TIMESTAMP_ADD":         clickhouseBuildDateDelta(exp.KindTimestampAdd, "", false),
		"TIMESTAMPADD":          clickhouseBuildDateDelta(exp.KindTimestampAdd, "", false),
		"TOMONDAY":              clickhouseBuildTimestampTrunc("WEEK"),
		"UNIQ":                  clickhouseFromArgList(exp.KindApproxDistinct),
		"MD5":                   clickhouseFromArgList(exp.KindMD5Digest),
		"SHA256":                clickhouseSHA2(256),
		"SHA512":                clickhouseSHA2(512),
		"SPLITBYCHAR":           clickhouseSplitByChar,
		"SPLITBYREGEXP":         clickhouseSplit(exp.KindRegexpSplit),
		"SPLITBYSTRING":         clickhouseSplit(exp.KindSplit),
		"TOTYPENAME":            clickhouseFromArgList(exp.KindTypeof),
		"ISNAN":                 clickhouseFromArgList(exp.KindIsNan),
		"IS_NAN":                clickhouseFromArgList(exp.KindIsNan),
		"STARTSWITH":            clickhouseFromArgList(exp.KindStartsWith),
		"STARTS_WITH":           clickhouseFromArgList(exp.KindStartsWith),
		"STR_TO_DATE":           clickhouseBuildStrToDate,
		"EDITDISTANCE":          clickhouseFromArgList(exp.KindLevenshtein),
		"LEVENSHTEINDISTANCE":   clickhouseFromArgList(exp.KindLevenshtein),
		"JAROWINKLERSIMILARITY": clickhouseFromArgList(exp.KindJarowinklerSimilarity),
		"UTCTIMESTAMP":          clickhouseFromArgList(exp.KindUtcTimestamp),
	}

	// TO<UNIT> -> TimestampTrunc(unit) for every TIMESTAMP_TRUNC_UNITS member
	// (parsers/clickhouse.py:77-87, 263).
	for _, unit := range []string{
		"MICROSECOND", "MILLISECOND", "SECOND", "MINUTE", "HOUR",
		"DAY", "MONTH", "QUARTER", "YEAR",
	} {
		fns["TOSTARTOF"+unit] = clickhouseBuildTimestampTrunc(unit)
	}

	return fns
}

// clickhouseBuildStrToDate ports _build_str_to_date (parsers/clickhouse.py:45-50).
func clickhouseBuildStrToDate(args []exp.Expression) exp.Expression {
	if len(args) == 3 {
		return exp.Anonymous(exp.Args{"this": "STR_TO_DATE", "expressions": args})
	}
	strtodate := exp.StrToDate(exp.Args{
		"this":   clickhouseSeqGet(args, 0),
		"format": clickhouseSeqGet(args, 1),
		"zone":   clickhouseSeqGet(args, 2),
		"safe":   false,
	})
	return exp.Cast(exp.Args{"this": strtodate, "to": exp.DTypeDatetime.IntoExpr(nil)})
}
