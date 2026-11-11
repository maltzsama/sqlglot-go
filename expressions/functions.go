package expressions

func FromArgList(kind Kind, args []Expression) Expression {
	specs := argTypesFor(kind)
	m := Args{}
	if len(specs) == 0 {
		return newNode(kind, m)
	}
	if varLenArgs[kind] {
		nNon := len(specs) - 1
		for i := 0; i < nNon && i < len(args); i++ {
			m[specs[i].Key] = args[i]
		}
		rest := []Expression{}
		if nNon <= len(args) {
			rest = args[nNon:]
		}
		m[specs[nNon].Key] = rest
	} else {
		for i, spec := range specs {
			if i < len(args) {
				m[spec.Key] = args[i]
			}
		}
	}
	return newNode(kind, m)
}

var FunctionByName = map[string]func([]Expression) Expression{
	"ARRAY": func(args []Expression) Expression {
		return newNode(KindArray, Args{"expressions": args})
	},
	// exp.Struct (array.py:355): var-len arguments land in "expressions". Every dialect's
	// parser maps STRUCT, so `STRUCT(1, 2)` is a Struct node rather than Anonymous.
	"STRUCT": func(args []Expression) Expression {
		return newNode(KindStruct, Args{"expressions": args})
	},
	"ARRAY_SUM":       genericFunction(KindArraySum),
	"ARRAY_TO_STRING": genericFunction(KindArrayToString),
	"DAYOFWEEK_ISO":   genericFunction(KindDayOfWeekIso),
	"PARSE_JSON":      genericFunction(KindParseJSON),
	"JSON_FORMAT":     genericFunction(KindJSONFormat),
	// EXPLODE/EXPLODE_OUTER are UDTF Func subclasses (array.py:238-249); the optimizer's
	// merge_subqueries refuses to fold a projection that contains one.
	"EXPLODE":       genericFunction(KindExplode),
	"EXPLODE_OUTER": genericFunction(KindExplodeOuter),
	// "LIST" registers exp.List the same way upstream does: FUNCTIONS spreads
	// exp.FUNCTION_BY_NAME (parser.py:374), which maps every Func subclass's sql_name to
	// from_arg_list (expressions/__init__.py:50) — so `LIST(1, 2, 3)` parses as exp.List,
	// not Anonymous, in every dialect.
	"LIST": func(args []Expression) Expression {
		return newNode(KindList, Args{"expressions": args})
	},
	"ABS":             genericFunction(KindAbs),
	"AVG":             genericFunction(KindAvg),
	"SUM":             genericFunction(KindSum),
	"SQRT":            genericFunction(KindSqrt),
	"LN":              genericFunction(KindLn),
	"EXP":             genericFunction(KindExp),
	"MIN":             genericFunction(KindMin),
	"MAX":             genericFunction(KindMax),
	"ROUND":           genericFunction(KindRound),
	"LOG":             genericFunction(KindLog),
	"POW":             genericFunction(KindPow),
	"POWER":           genericFunction(KindPow),
	"SUBSTR":          genericFunction(KindSubstring),
	"CEILING":         genericFunction(KindCeil),
	"PERCENTILE_DISC": genericFunction(KindPercentileDisc),
	"GROUP_CONCAT":    genericFunction(KindGroupConcat),
	// LAG/LEAD(this[, offset[, default]]) window functions (aggregate.py:150-163); FromArgList
	// maps args positionally onto {this, offset, default} per KindLag/KindLead's arg_types.
	"LAG":  genericFunction(KindLag),
	"LEAD": genericFunction(KindLead),
	// LISTAGG is deliberately NOT registered here: upstream only maps it in Oracle/Snowflake/
	// etc.'s FUNCTIONS (never base's), so base LISTAGG(x) must fall through to Anonymous and
	// round-trip verbatim (parity_gaps.txt:10,11 - WITHIN GROUP attaches generically via the
	// same postfix parseWindow path PERCENTILE_CONT already uses).
	"STDDEV":                genericFunction(KindStddev),
	"STDEV":                 genericFunction(KindStddev),
	"STDDEV_POP":            genericFunction(KindStddevPop),
	"STDDEV_SAMP":           genericFunction(KindStddevSamp),
	"VARIANCE":              genericFunction(KindVariance),
	"VARIANCE_SAMP":         genericFunction(KindVariance),
	"VAR_SAMP":              genericFunction(KindVariance),
	"VARIANCE_POP":          genericFunction(KindVariancePop),
	"VAR_POP":               genericFunction(KindVariancePop),
	"DAY":                   genericFunction(KindDay),
	"MONTH":                 genericFunction(KindMonth),
	"YEAR":                  genericFunction(KindYear),
	"QUARTER":               genericFunction(KindQuarter),
	"APPROX_DISTINCT":       genericFunction(KindApproxDistinct),
	"APPROX_COUNT_DISTINCT": genericFunction(KindApproxDistinct),
	"HLL":                   genericFunction(KindHll),
	"COUNT_IF":              genericFunction(KindCountIf),
	"COUNTIF":               genericFunction(KindCountIf),
	"QUANTILE":              genericFunction(KindQuantile),
	"ARRAY_AGG":             genericFunction(KindArrayAgg),
	"ARRAY_SIZE":            genericFunction(KindArraySize),
	"ARRAY_ALL":             genericFunction(KindArrayAll),
	"ARRAY_ANY":             genericFunction(KindArrayAny),
	"ARRAY_LENGTH":          genericFunction(KindArraySize),
	"ARRAY_CONTAINS":        genericFunction(KindArrayContains),
	"ARRAY_HAS":             genericFunction(KindArrayContains),
	"INITCAP":               genericFunction(KindInitcap),
	"SPLIT":                 genericFunction(KindSplit),
	"REGEXP_LIKE":           genericFunction(KindRegexpLike),
	"RLIKE":                 genericFunction(KindRegexpLike),
	"REGEXP_SPLIT":          genericFunction(KindRegexpSplit),
	"STRUCT_EXTRACT":        genericFunction(KindStructExtract),
	"STANDARD_HASH":         genericFunction(KindStandardHash),
	"HEX":                   genericFunction(KindHex),
	"UNHEX":                 genericFunction(KindUnhex),
	// FROM_BASE(this, expression) (string.py:297-299): upstream registers every Func
	// subclass's sql_name in FUNCTION_BY_NAME (parser.py:374), so FROM_BASE parses
	// structurally in every dialect — Redshift renames it to STRTOL on write
	// (generator/dialect_redshift.go). Without this entry it degrades to Anonymous.
	"FROM_BASE":           genericFunction(KindFromBase),
	"MD5":                 genericFunction(KindMD5),
	"ST_POINT":            genericFunction(KindStPoint),
	"ST_MAKEPOINT":        genericFunction(KindStPoint),
	"ST_DISTANCE":         genericFunction(KindStDistance),
	"GENERATE_SERIES":     genericFunction(KindGenerateSeries),
	"DATE":                genericFunction(KindDate),
	"ADD_MONTHS":          genericFunction(KindAddMonths),
	"DATE_ADD":            genericFunction(KindDateAdd),
	"DATEADD":             genericFunction(KindDateAdd),
	"DATEDIFF":            genericFunction(KindDateDiff),
	"DATE_DIFF":           genericFunction(KindDateDiff),
	"JSON_EXTRACT":        jsonExtractFunction(KindJSONExtract),
	"JSON_EXTRACT_SCALAR": jsonExtractFunction(KindJSONExtractScalar),
	// REPLACE(this, expression[, replacement]) (string.py:113); base has no replace_sql, so
	// it renders via functionFallbackSQL like any other unadorned Func (see KindInitcap).
	"REPLACE": genericFunction(KindReplace),
	// LENGTH/LEN/CHAR_LENGTH/CHARACTER_LENGTH are deliberately NOT registered as a single Kind:
	// upstream keeps them distinct per dialect (MySQL LENGTH=binary/byte-length vs
	// CHAR_LENGTH=char-length, parsers/mysql.py:127 + generators/mysql.py:179), which needs
	// per-dialect FUNCTIONS parse-side canonicalization (ROADMAP 5b, deferred). One global Kind
	// would render MySQL CHAR_LENGTH as LENGTH - a semantic (byte vs char) regression - so these
	// fall through to Anonymous and round-trip verbatim until 5b lands.
	// LogicalOr._sql_names (aggregate.py:172): all three spellings parse to the same Kind;
	// see generator/aggregate.go logicalOrSQL for the postgres/mysql renames.
	"LOGICAL_OR": genericFunction(KindLogicalOr),
	"BOOL_OR":    genericFunction(KindLogicalOr),
	"BOOLOR_AGG": genericFunction(KindLogicalOr),
	"COUNT": func(args []Expression) Expression {
		m := Args{"big_int": true}
		if len(args) > 0 {
			m["this"] = args[0]
		}
		if len(args) > 1 {
			m["expressions"] = args[1:]
		}
		return newNode(KindCount, m)
	},
	// Base dialect maps COALESCE/IFNULL/NVL to the same builder with is_nvl unset
	// (parser.py:329); only Oracle-family dialects set is_nvl=True (deferred).
	"COALESCE":            coalesceFunction(),
	"IFNULL":              coalesceFunction(),
	"NVL":                 coalesceFunction(),
	"GENERATE_DATE_ARRAY": FromArgListFunc(KindGenerateDateArray),
	"GREATEST": func(args []Expression) Expression {
		m := Args{"ignore_nulls": true}
		if len(args) > 0 {
			m["this"] = args[0]
		}
		if len(args) > 1 {
			m["expressions"] = args[1:]
		}
		return newNode(KindGreatest, m)
	},
	"LEAST": func(args []Expression) Expression {
		m := Args{"ignore_nulls": true}
		if len(args) > 0 {
			m["this"] = args[0]
		}
		if len(args) > 1 {
			m["expressions"] = args[1:]
		}
		return newNode(KindLeast, m)
	},
	// STR_TO_*/TIME_STR_TO_* (temporal.py:452-473): all six register generically here (no
	// per-dialect FUNCTIONS override upstream), parsing for free via FromArgList/genericFunction
	// since each Kind's argTypes row (kinds.go) already mirrors its arg_types dict key order.
	"STR_TO_DATE":      genericFunction(KindStrToDate),
	"STR_TO_TIME":      genericFunction(KindStrToTime),
	"STR_TO_UNIX":      genericFunction(KindStrToUnix),
	"TIME_STR_TO_DATE": genericFunction(KindTimeStrToDate),
	"TIME_STR_TO_TIME": genericFunction(KindTimeStrToTime),
	"TIME_STR_TO_UNIX": genericFunction(KindTimeStrToUnix),
	// TimeToStr (temporal.py:486-487) declares no _sql_names, so upstream auto-registers it
	// under its camel-case name only (core.py:1676-1683 sql_names -> FUNCTION_BY_NAME via
	// parser.py:374). Base has no TimeToStr generator transform, so it renders back verbatim
	// through functionFallbackSQL (`TIME_TO_STR(...)`); only dialects carrying a TIME_MAPPING
	// override generation (generators/postgres.py:399 -> TO_CHAR).
	"TIME_TO_STR": genericFunction(KindTimeToStr),
	// dialect-funcs cluster singleton base entries (parity_gaps.txt gaps 114/160; verified
	// against parser.py:121-129,394 - both are truly base-scope upstream, not per-dialect,
	// unlike DATABASE()/SCHEMA() below which parsers/mysql.py:134-135 register mysql-only
	// (see dialects/mysql.go's d.Functions map)).
	"MOD": buildMod,
	// Trunc._sql_names = ["TRUNC", "TRUNCATE"] (math.py:188-190); TRUNCATE(n, d) is
	// disambiguated from the TRUNCATE TABLE statement in parser/stmt_comment_truncate.go's
	// parseTruncateTable (ports _parse_truncate_table's "not to be confused with
	// TRUNCATE(number, decimals)" L_PAREN retreat, parser.py:9469-9472) before ever reaching
	// this map, so registering it here is safe.
	"TRUNC":    genericFunction(KindTrunc),
	"TRUNCATE": genericFunction(KindTrunc),
	// Base-scope temporal/string spellings that upstream auto-registers in FUNCTION_BY_NAME
	// via each class's _sql_names: DayOfMonth/DayOfWeek/DayOfYear/WeekOfYear each register BOTH
	// the underscore and no-underscore forms (temporal.py:209-270), and LCASE/UCASE ->
	// Lower/Upper (string.py:85-86,254-255). MySQL renders these with its own spelling
	// overrides (generator/dialect_funcs.go, gated on g.dialect.Name=="mysql"); base/postgres
	// keep the canonical DAY_OF_MONTH/.../LOWER/UPPER via functionFallbackSQL. (CURDATE/CURTIME/
	// DATABASE/SCHEMA stay mysql-only in dialects/mysql.go - base does not register those,
	// verified against the pinned oracle.)
	"DAY_OF_MONTH": genericFunction(KindDayOfMonth),
	"DAYOFMONTH":   genericFunction(KindDayOfMonth),
	"DAY_OF_WEEK":  genericFunction(KindDayOfWeek),
	"DAYOFWEEK":    genericFunction(KindDayOfWeek),
	"DAY_OF_YEAR":  genericFunction(KindDayOfYear),
	"DAYOFYEAR":    genericFunction(KindDayOfYear),
	"WEEK_OF_YEAR": genericFunction(KindWeekOfYear),
	"WEEKOFYEAR":   genericFunction(KindWeekOfYear),
	"LCASE":        genericFunction(KindLower),
	"UCASE":        genericFunction(KindUpper),
	// CurrentDate is a base NO_PAREN_FUNCTIONS keyword (bare CURRENT_DATE, handled in the
	// parser's noParenFunctions) but is ALSO a base FUNCTION_BY_NAME entry, so the
	// parenthesized CURRENT_DATE() / CURRENT_DATE(<zone>) forms build a CurrentDate node too
	// (currentdate_sql renders CURRENT_DATE without the empty parens an Anonymous call emits).
	"CURRENT_DATE": genericFunction(KindCurrentDate),
	"FIRST":        genericFunction(KindFirst),
	"SHA2":         genericFunction(KindSHA2),
	"LAST":         genericFunction(KindLast),
	// Spark/Databricks cluster: the base FUNCTION_BY_NAME entries for the new Func
	// classes, keyed by each class's sql_names() (expressions/__init__.py:50), so base
	// parses them typed exactly as upstream does. Dialect-specific spellings (AGGREGATE,
	// TRY_ADD, BIT_AND, ...) are overlaid in dialects/spark.go / dialects/databricks.go.
	"REDUCE":              genericFunction(KindReduce),
	"FROM_TIME_ZONE":      genericFunction(KindFromTimeZone),
	"ARRAY_INSERT":        genericFunction(KindArrayInsert),
	"BITWISE_AND_AGG":     genericFunction(KindBitwiseAndAgg),
	"BITWISE_OR_AGG":      genericFunction(KindBitwiseOrAgg),
	"BITWISE_XOR_AGG":     genericFunction(KindBitwiseXorAgg),
	"BITWISE_COUNT":       genericFunction(KindBitwiseCount),
	"TIMESTAMPDIFF":       genericFunction(KindTimestampDiff),
	"TIMESTAMP_DIFF":      genericFunction(KindTimestampDiff),
	"SAFE_ADD":            genericFunction(KindSafeAdd),
	"SAFE_DIVIDE":         genericFunction(KindSafeDivide),
	"SAFE_MULTIPLY":       genericFunction(KindSafeMultiply),
	"SAFE_SUBTRACT":       genericFunction(KindSafeSubtract),
	"JSON_KEYS":           genericFunction(KindJSONKeys),
	"UNIFORM":             genericFunction(KindUniform),
	"REGR_AVGX":           genericFunction(KindRegrAvgx),
	"REGR_AVGY":           genericFunction(KindRegrAvgy),
	"REGR_SXX":            genericFunction(KindRegrSxx),
	"REGR_SXY":            genericFunction(KindRegrSxy),
	"REGR_SYY":            genericFunction(KindRegrSyy),
	"REGR_COUNT":          genericFunction(KindRegrCount),
	"GETBIT":              genericFunction(KindGetbit),
	"GET_BIT":             genericFunction(KindGetbit),
	"DATE_FROM_UNIX_DATE": genericFunction(KindDateFromUnixDate),
	"READ_PARQUET":        genericFunction(KindReadParquet),
	"FORMAT":              genericFunction(KindFormat),
}

// buildMod ports build_mod (parser.py:121-129): MOD(x, y) -> exp.Mod, parenthesizing either
// operand if it is itself a Binary node (e.g. MOD(a + 1, 7) -> (a + 1) % 7) so the operator
// precedence of the rendered `%` form round-trips correctly.
func buildMod(args []Expression) Expression {
	var this, expression Expression
	if len(args) > 0 {
		this = parenWrapBinary(args[0])
	}
	if len(args) > 1 {
		expression = parenWrapBinary(args[1])
	}
	return newNode(KindMod, Args{"this": this, "expression": expression})
}

func parenWrapBinary(e Expression) Expression {
	if e != nil && e.Is(TraitBinary) {
		return Paren(Args{"this": e})
	}
	return e
}

func genericFunction(kind Kind) func([]Expression) Expression {
	return func(args []Expression) Expression { return FromArgList(kind, args) }
}

// FromArgListFunc is the exported form of genericFunction, for building
// dialects.Dialect.Functions overlay entries (per-dialect FUNCTIONS overrides, e.g.
// mysql's CURDATE/DAY_OF_MONTH/LCASE/... cluster in dialects/mysql.go) from outside this
// package: a plain positional this/expression/... mapping via FromArgList, same as every
// exp.FunctionByName entry that has no custom builder.
func FromArgListFunc(kind Kind) func([]Expression) Expression {
	return genericFunction(kind)
}

func jsonExtractFunction(kind Kind) func([]Expression) Expression {
	return func(args []Expression) Expression {
		m := Args{}
		if len(args) > 0 {
			m["this"] = args[0]
		}
		if len(args) > 1 {
			m["expression"] = args[1]
		}
		// build_extract_json_with_path (parser.py:104-118) preserves the 3rd+ path arguments
		// only for JSONExtract, not JSONExtractScalar (scalar keeps just this + expression), so
		// e.g. JSON_EXTRACT_SCALAR(a, b, c) drops `c`.
		if len(args) > 2 && kind == KindJSONExtract {
			m["expressions"] = args[2:]
		}
		return newNode(kind, m)
	}
}

func coalesceFunction() func([]Expression) Expression {
	return func(args []Expression) Expression {
		m := Args{}
		if len(args) > 0 {
			m["this"] = args[0]
		}
		if len(args) > 1 {
			m["expressions"] = args[1:]
		}
		return newNode(KindCoalesce, m)
	}
}

func Abs(args Args) Expression            { return newNode(KindAbs, args) }
func Avg(args Args) Expression            { return newNode(KindAvg, args) }
func Sum(args Args) Expression            { return newNode(KindSum, args) }
func Sqrt(args Args) Expression           { return newNode(KindSqrt, args) }
func Ln(args Args) Expression             { return newNode(KindLn, args) }
func Exp(args Args) Expression            { return newNode(KindExp, args) }
func Min(args Args) Expression            { return newNode(KindMin, args) }
func Max(args Args) Expression            { return newNode(KindMax, args) }
func Round(args Args) Expression          { return newNode(KindRound, args) }
func Log(args Args) Expression            { return newNode(KindLog, args) }
func Pow(args Args) Expression            { return newNode(KindPow, args) }
func Stddev(args Args) Expression         { return newNode(KindStddev, args) }
func StddevPop(args Args) Expression      { return newNode(KindStddevPop, args) }
func StddevSamp(args Args) Expression     { return newNode(KindStddevSamp, args) }
func Variance(args Args) Expression       { return newNode(KindVariance, args) }
func VariancePop(args Args) Expression    { return newNode(KindVariancePop, args) }
func Day(args Args) Expression            { return newNode(KindDay, args) }
func Month(args Args) Expression          { return newNode(KindMonth, args) }
func Year(args Args) Expression           { return newNode(KindYear, args) }
func Quarter(args Args) Expression        { return newNode(KindQuarter, args) }
func ApproxDistinct(args Args) Expression { return newNode(KindApproxDistinct, args) }
func Hll(args Args) Expression            { return newNode(KindHll, args) }
func CountIf(args Args) Expression        { return newNode(KindCountIf, args) }
func Quantile(args Args) Expression       { return newNode(KindQuantile, args) }
func Count(args Args) Expression          { return newNode(KindCount, args) }
func Coalesce(args Args) Expression       { return newNode(KindCoalesce, args) }
func Greatest(args Args) Expression       { return newNode(KindGreatest, args) }
func Least(args Args) Expression          { return newNode(KindLeast, args) }
func Cast(args Args) Expression           { return newNode(KindCast, args) }
func TryCast(args Args) Expression        { return newNode(KindTryCast, args) }
func CastToStrType(args Args) Expression  { return newNode(KindCastToStrType, args) }
func Extract(args Args) Expression        { return newNode(KindExtract, args) }
func StrPosition(args Args) Expression    { return newNode(KindStrPosition, args) }
func Substring(args Args) Expression      { return newNode(KindSubstring, args) }
func Trim(args Args) Expression           { return newNode(KindTrim, args) }
func Ceil(args Args) Expression           { return newNode(KindCeil, args) }
func Floor(args Args) Expression          { return newNode(KindFloor, args) }
func GroupConcat(args Args) Expression    { return newNode(KindGroupConcat, args) }
func Replace(args Args) Expression        { return newNode(KindReplace, args) }
func LogicalOr(args Args) Expression      { return newNode(KindLogicalOr, args) }
func ArrayAgg(args Args) Expression       { return newNode(KindArrayAgg, args) }
func ArraySize(args Args) Expression      { return newNode(KindArraySize, args) }
func ArrayAll(args Args) Expression       { return newNode(KindArrayAll, args) }
func ArrayAny(args Args) Expression       { return newNode(KindArrayAny, args) }
func ArrayContains(args Args) Expression  { return newNode(KindArrayContains, args) }
func Initcap(args Args) Expression        { return newNode(KindInitcap, args) }
func Split(args Args) Expression          { return newNode(KindSplit, args) }
func RegexpLike(args Args) Expression     { return newNode(KindRegexpLike, args) }
func RegexpSplit(args Args) Expression    { return newNode(KindRegexpSplit, args) }
func StructExtract(args Args) Expression  { return newNode(KindStructExtract, args) }
func StandardHash(args Args) Expression   { return newNode(KindStandardHash, args) }
func Hex(args Args) Expression            { return newNode(KindHex, args) }
func MD5(args Args) Expression            { return newNode(KindMD5, args) }
func StPoint(args Args) Expression        { return newNode(KindStPoint, args) }
func StDistance(args Args) Expression     { return newNode(KindStDistance, args) }
func GenerateSeries(args Args) Expression { return newNode(KindGenerateSeries, args) }
func Date(args Args) Expression           { return newNode(KindDate, args) }
func AddMonths(args Args) Expression      { return newNode(KindAddMonths, args) }
func DateAdd(args Args) Expression        { return newNode(KindDateAdd, args) }
func DateDiff(args Args) Expression       { return newNode(KindDateDiff, args) }
func Overlay(args Args) Expression        { return newNode(KindOverlay, args) }
func Variadic(args Args) Expression       { return newNode(KindVariadic, args) }
func CurrentDate(args Args) Expression    { return newNode(KindCurrentDate, args) }
func CurrentTime(args Args) Expression    { return newNode(KindCurrentTime, args) }

// Niladic system-value function builders wired into NO_PAREN_FUNCTIONS (parser.py:431-438
// and the postgres/mysql overrides). CurrentSchema previously had no dedicated builder (it
// was only reached via mysql's FromArgListFunc for DATABASE/SCHEMA); postgres's bare
// CURRENT_SCHEMA path needs one.
func CurrentUser(args Args) Expression    { return newNode(KindCurrentUser, args) }    // functions.py:309
func CurrentRole(args Args) Expression    { return newNode(KindCurrentRole, args) }    // functions.py:277
func SessionUser(args Args) Expression    { return newNode(KindSessionUser, args) }    // functions.py:325
func Localtime(args Args) Expression      { return newNode(KindLocaltime, args) }      // temporal.py:49
func Localtimestamp(args Args) Expression { return newNode(KindLocaltimestamp, args) } // temporal.py:53
func CurrentSchema(args Args) Expression  { return newNode(KindCurrentSchema, args) }  // functions.py:285

// Redshift function and property nodes (expressions/functions.py, expressions/properties.py).
func FromBase(args Args) Expression          { return newNode(KindFromBase, args) }
func CurrentUserId(args Args) Expression     { return newNode(KindCurrentUserId, args) }
func ObjectTransform(args Args) Expression   { return newNode(KindObjectTransform, args) }
func ConvertTimezone(args Args) Expression   { return newNode(KindConvertTimezone, args) }
func ConcatWs(args Args) Expression          { return newNode(KindConcatWs, args) }
func SortKeyProperty(args Args) Expression   { return newNode(KindSortKeyProperty, args) }
func DistKeyProperty(args Args) Expression   { return newNode(KindDistKeyProperty, args) }
func DistStyleProperty(args Args) Expression { return newNode(KindDistStyleProperty, args) }
func PercentileDisc(args Args) Expression    { return newNode(KindPercentileDisc, args) }
func AtIndex(args Args) Expression           { return newNode(KindAtIndex, args) }

// Spark/Databricks function nodes (expressions/{array,math,aggregate,json,temporal,
// functions,string}.py) and the Databricks TIMESERIES key constraint
// (expressions/constraints.py:224).
func Reduce(args Args) Expression           { return newNode(KindReduce, args) }
func FromTimeZone(args Args) Expression     { return newNode(KindFromTimeZone, args) }
func ArrayInsert(args Args) Expression      { return newNode(KindArrayInsert, args) }
func BitwiseAndAgg(args Args) Expression    { return newNode(KindBitwiseAndAgg, args) }
func BitwiseOrAgg(args Args) Expression     { return newNode(KindBitwiseOrAgg, args) }
func BitwiseXorAgg(args Args) Expression    { return newNode(KindBitwiseXorAgg, args) }
func BitwiseCount(args Args) Expression     { return newNode(KindBitwiseCount, args) }
func TimestampDiff(args Args) Expression    { return newNode(KindTimestampDiff, args) }
func SafeAdd(args Args) Expression          { return newNode(KindSafeAdd, args) }
func SafeDivide(args Args) Expression       { return newNode(KindSafeDivide, args) }
func SafeMultiply(args Args) Expression     { return newNode(KindSafeMultiply, args) }
func SafeSubtract(args Args) Expression     { return newNode(KindSafeSubtract, args) }
func JSONKeys(args Args) Expression         { return newNode(KindJSONKeys, args) }
func Uniform(args Args) Expression          { return newNode(KindUniform, args) }
func TimeseriesKey(args Args) Expression    { return newNode(KindTimeseriesKey, args) }
func RegrAvgx(args Args) Expression         { return newNode(KindRegrAvgx, args) }
func RegrAvgy(args Args) Expression         { return newNode(KindRegrAvgy, args) }
func RegrSxx(args Args) Expression          { return newNode(KindRegrSxx, args) }
func RegrSxy(args Args) Expression          { return newNode(KindRegrSxy, args) }
func RegrSyy(args Args) Expression          { return newNode(KindRegrSyy, args) }
func RegrCount(args Args) Expression        { return newNode(KindRegrCount, args) }
func Getbit(args Args) Expression           { return newNode(KindGetbit, args) }
func DateFromUnixDate(args Args) Expression { return newNode(KindDateFromUnixDate, args) }
func ReadParquet(args Args) Expression      { return newNode(KindReadParquet, args) }
func Format(args Args) Expression           { return newNode(KindFormat, args) }
