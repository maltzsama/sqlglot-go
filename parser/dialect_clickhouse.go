package parser

import (
	"strings"

	exp "github.com/ridi-oss/sqlglot-go/expressions"
	"github.com/ridi-oss/sqlglot-go/tokens"
)

// ClickHouse parser overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/parsers/clickhouse.py.
func init() {
	registerDialectParserOverrides("clickhouse", dialectParserOverrideSet{
		FunctionParsers: map[string]parserOverrideFunc{
			"EXTRACT":     (*Parser).parseClickhouseExtract,
			"ARRAYJOIN":   (*Parser).parseClickhouseArrayJoin,
			"GROUPCONCAT": (*Parser).parseClickhouseGroupConcat,
			"QUANTILE":    (*Parser).parseClickhouseQuantile,
			"MEDIAN":      (*Parser).parseClickhouseQuantile,
			"COLUMNS":     (*Parser).parseClickhouseColumns,
			"TUPLE":       (*Parser).parseClickhouseTuple,
			"AND":         (*Parser).parseClickhouseAnd,
			"OR":          (*Parser).parseClickhouseOr,
			"XOR":         (*Parser).parseClickhouseXor,
		},
		// FUNCTION_PARSERS = {**base, ...} - {"MATCH"} (parsers/clickhouse.py:377-378).
		DisabledFunctionParsers: map[string]bool{"MATCH": true},
		StatementParsers: map[tokens.TokenType]parserOverrideFunc{
			tokens.DETACH: (*Parser).parseClickhouseDetach,
		},
		PropertyParsers: map[string]propertyParserFunc{
			"ENGINE":  (*Parser).parseClickhouseEngineProperty,
			"REFRESH": (*Parser).parseClickhouseAutoRefreshProperty,
			"UUID":    (*Parser).parseClickhouseUuidProperty,
		},
		// PROPERTY_PARSERS = {**base, ...} - {"DYNAMIC"} (parsers/clickhouse.py:390-395).
		DisabledPropertyParsers: map[string]bool{"DYNAMIC": true},
		TypeParser:              parseClickhouseTypes,
		QueryModifiers: map[tokens.TokenType]func(*Parser) (string, any){
			tokens.SETTINGS: func(p *Parser) (string, any) {
				return "settings", p.parseClickhouseSettings()
			},
			tokens.FORMAT: func(p *Parser) (string, any) {
				// parsers/clickhouse.py:450: `_advance() or _parse_id_var()`.
				p.match(tokens.FORMAT)
				return "format", p.parseIdVar(true, nil)
			},
		},
		// NO_PAREN_FUNCTION_PARSERS = {**base, ...} - {"ANY"} (parsers/clickhouse.py:397-399).
		DisabledNoParenFunctionParsers: map[string]bool{"ANY": true},
		// NO_PAREN_FUNCTIONS = {**base, ...} - {CURRENT_TIMESTAMP} (parsers/clickhouse.py:401-405).
		DisabledNoParenFunctions: map[tokens.TokenType]bool{tokens.CURRENT_TIMESTAMP: true},
		// TABLE_ALIAS_TOKENS = base - {ALL, ANY, ARRAY, ASOF, FINAL, FORMAT, SETTINGS}.
		RemovedTableAliasTokens: map[tokens.TokenType]bool{
			tokens.ALL: true, tokens.ANY: true, tokens.ARRAY: true, tokens.ASOF: true,
			tokens.FINAL: true, tokens.FORMAT: true, tokens.SETTINGS: true,
		},
		// ALIAS_TOKENS = base - {FORMAT, SETTINGS}.
		RemovedAliasTokens: map[tokens.TokenType]bool{tokens.FORMAT: true, tokens.SETTINGS: true},
	})
}

// clickhouseAggFunctions mirrors _AGG_FUNCTIONS (parsers/clickhouse.py:90-208).
var clickhouseAggFunctions = map[string]bool{
	"count": true, "min": true, "max": true, "sum": true, "avg": true, "any": true,
	"stddevPop": true, "stddevSamp": true, "varPop": true, "varSamp": true, "corr": true,
	"covarPop": true, "covarSamp": true, "entropy": true, "exponentialMovingAverage": true,
	"intervalLengthSum": true, "kolmogorovSmirnovTest": true, "mannWhitneyUTest": true,
	"median": true, "rankCorr": true, "sumKahan": true, "studentTTest": true, "welchTTest": true,
	"anyHeavy": true, "anyLast": true, "boundingRatio": true, "first_value": true,
	"last_value": true, "argMin": true, "argMax": true, "avgWeighted": true, "topK": true,
	"approx_top_sum": true, "topKWeighted": true, "deltaSum": true, "deltaSumTimestamp": true,
	"groupArray": true, "groupArrayLast": true, "groupConcat": true, "groupUniqArray": true,
	"groupArrayInsertAt": true, "groupArrayMovingAvg": true, "groupArrayMovingSum": true,
	"groupArraySample": true, "groupBitAnd": true, "groupBitOr": true, "groupBitXor": true,
	"groupBitmap": true, "groupBitmapAnd": true, "groupBitmapOr": true, "groupBitmapXor": true,
	"sumWithOverflow": true, "sumMap": true, "minMap": true, "maxMap": true, "skewSamp": true,
	"skewPop": true, "kurtSamp": true, "kurtPop": true, "uniq": true, "uniqExact": true,
	"uniqCombined": true, "uniqCombined64": true, "uniqHLL12": true, "uniqTheta": true,
	"quantile": true, "quantiles": true, "quantileExact": true, "quantileExactInclusive": true,
	"quantilesExact": true, "quantilesExactExclusive": true, "quantileExactLow": true,
	"quantilesExactLow": true, "quantileExactHigh": true, "quantilesExactHigh": true,
	"quantileExactWeighted": true, "quantilesExactWeighted": true, "quantileTiming": true,
	"quantilesTiming": true, "quantileTimingWeighted": true, "quantilesTimingWeighted": true,
	"quantileDeterministic": true, "quantilesDeterministic": true, "quantileTDigest": true,
	"quantilesTDigest": true, "quantileTDigestWeighted": true, "quantilesTDigestWeighted": true,
	"quantileBFloat16": true, "quantilesBFloat16": true, "quantileBFloat16Weighted": true,
	"quantilesBFloat16Weighted": true, "simpleLinearRegression": true,
	"stochasticLinearRegression": true, "stochasticLogisticRegression": true,
	"categoricalInformationValue": true, "contingency": true, "cramersV": true,
	"cramersVBiasCorrected": true, "theilsU": true, "maxIntersections": true,
	"maxIntersectionsPosition": true, "meanZTest": true, "quantileInterpolatedWeighted": true,
	"quantilesInterpolatedWeighted": true, "quantileGK": true, "quantilesGK": true,
	"sparkBar": true, "sumCount": true, "largestTriangleThreeBuckets": true, "histogram": true,
	"sequenceMatch": true, "sequenceCount": true, "windowFunnel": true, "retention": true,
	"uniqUpTo": true, "sequenceNextNode": true, "exponentialTimeDecayedAvg": true,
}

// clickhouseAggSuffixes mirrors _AGG_FUNCTIONS_SUFFIXES (parsers/clickhouse.py:212-232):
// sorted longest-first so compound suffixes match before their sub-suffixes. Python's
// stable sort preserves the declaration order among equal lengths.
var clickhouseAggSuffixes = []string{
	"SimpleState", "MergeState", "OrDefault", "Distinct", "Resample",
	"ArrayIf", "ForEach", "OrNull", "ArgMin", "ArgMax",
	"Array", "State", "Merge", "Map", "If",
}

// clickhouseAggMapping mirrors _AGG_FUNC_MAPPING (parsers/clickhouse.py:235-237): every
// 0- and 1-suffix aggregate function name -> (base name, suffix).
var clickhouseAggMapping = func() map[string][2]string {
	m := make(map[string][2]string, len(clickhouseAggFunctions)*(len(clickhouseAggSuffixes)+1))
	for f := range clickhouseAggFunctions {
		m[f] = [2]string{f, ""}
		for _, sfx := range clickhouseAggSuffixes {
			m[f+sfx] = [2]string{f, sfx}
		}
	}
	return m
}()

// resolveClickhouseAgg ports _resolve_clickhouse_agg (parsers/clickhouse.py:347-375).
func resolveClickhouseAgg(name string) (string, []string, bool) {
	if parts, ok := clickhouseAggMapping[name]; ok {
		if parts[1] != "" {
			return parts[0], []string{parts[1]}, true
		}
		return parts[0], nil, true
	}

	var accumulated []string
	for {
		if parts, ok := clickhouseAggMapping[name]; ok {
			if parts[1] != "" {
				accumulated = append([]string{parts[1]}, accumulated...)
			}
			return parts[0], accumulated, true
		}
		matched := false
		for _, suffix := range clickhouseAggSuffixes {
			if strings.HasSuffix(name, suffix) && len(name) != len(suffix) {
				accumulated = append([]string{suffix}, accumulated...)
				name = name[:len(name)-len(suffix)]
				matched = true
				break
			}
		}
		if !matched {
			return "", nil, false
		}
	}
}

// applyClickhouseAggResolution ports the aggregate-combinator tail of _parse_function
// (parsers/clickhouse.py:704-750).
func (p *Parser) applyClickhouseAggResolution(expr exp.Expression) exp.Expression {
	func_ := expr
	if expr != nil && expr.Kind() == exp.KindWindow {
		func_ = asExpr(expr.Arg("this"))
	}
	if func_ == nil || func_.Kind() != exp.KindAnonymous {
		return expr
	}
	name := func_.Arg("this")
	nameText, _ := name.(string)
	// The base name is not needed: upstream keeps the as-written anon_func.this.
	_, suffixes, ok := resolveClickhouseAgg(nameText)
	if !ok {
		return expr
	}

	params := p.parseClickhouseFuncParams(func_)
	anonArgs, _ := func_.Arg("expressions").([]exp.Expression)
	arg := exp.Args{"this": name, "expressions": anonArgs}

	var instance exp.Expression
	switch {
	case len(suffixes) > 0 && len(params) > 0:
		if params != nil {
			arg["params"] = params
		}
		instance = p.expression(exp.CombinedParameterizedAgg(arg), nil, nil)
	case len(suffixes) > 0:
		instance = p.expression(exp.CombinedAggFunc(arg), nil, nil)
	case len(params) > 0:
		arg["params"] = params
		instance = p.expression(exp.ParameterizedAgg(arg), nil, nil)
	default:
		instance = p.expression(exp.AnonymousAggFunc(arg), nil, nil)
	}

	if expr.Kind() == exp.KindWindow {
		expr.Set("this", instance)
		return expr
	}
	if params != nil {
		return p.parseWindow(instance, false)
	}
	return instance
}

// parseClickhouseFuncParams ports _parse_func_params (parsers/clickhouse.py:752-761).
func (p *Parser) parseClickhouseFuncParams(this exp.Expression) []exp.Expression {
	if p.matchPair(tokens.R_PAREN, tokens.L_PAREN, true) {
		return p.parseCsv(func() exp.Expression { return p.parseLambda(true) })
	}
	if p.match(tokens.L_PAREN) {
		params := p.parseCsv(func() exp.Expression { return p.parseLambda(true) })
		p.matchRParen(this)
		return params
	}
	return nil
}

// parseClickhouseArrayJoin ports FUNCTION_PARSERS["ARRAYJOIN"] (parsers/clickhouse.py:379).
func (p *Parser) parseClickhouseArrayJoin() exp.Expression {
	return p.expression(exp.Explode(exp.Args{"this": p.parseExpression()}), nil, nil)
}

// parseClickhouseTuple ports FUNCTION_PARSERS["TUPLE"] (parsers/clickhouse.py:384).
func (p *Parser) parseClickhouseTuple() exp.Expression {
	return exp.FromArgList(exp.KindStruct, p.parseFunctionArgs(true))
}

// parseClickhouseGroupConcat ports _parse_group_concat (parsers/clickhouse.py:763-777).
func (p *Parser) parseClickhouseGroupConcat() exp.Expression {
	args := p.parseCsv(func() exp.Expression { return p.parseLambda(true) })
	params := p.parseClickhouseFuncParams(nil)

	if params != nil {
		separator := seqGetExpr(args, 0)
		limit := seqGetExpr(args, 1)
		this := seqGetExpr(params, 0)
		if limit != nil {
			this = p.expression(exp.Limit(exp.Args{"this": this, "expression": limit}), nil, nil)
		}
		return p.expression(exp.GroupConcat(exp.Args{"this": this, "separator": separator}), nil, nil)
	}
	return p.expression(exp.GroupConcat(exp.Args{"this": seqGetExpr(args, 0)}), nil, nil)
}

// parseClickhouseQuantile ports _parse_quantile (parsers/clickhouse.py:779-784).
func (p *Parser) parseClickhouseQuantile() exp.Expression {
	this := p.parseLambda(true)
	params := p.parseClickhouseFuncParams(nil)
	if params != nil {
		return p.expression(exp.Quantile(exp.Args{"this": seqGetExpr(params, 0), "quantile": this}), nil, nil)
	}
	return p.expression(exp.Quantile(exp.Args{
		"this":     this,
		"quantile": exp.LiteralNumber("0.5"),
	}), nil, nil)
}

// parseClickhouseColumns ports _parse_columns (parsers/clickhouse.py:962-968).
func (p *Parser) parseClickhouseColumns() exp.Expression {
	this := p.expression(exp.Columns(exp.Args{"this": p.parseLambda(true)}), nil, nil)
	for p.next.IsValid() && p.matchTextSeq(")", "APPLY", "(") {
		p.match(tokens.R_PAREN)
		this = p.expression(exp.Apply(exp.Args{"this": this, "expression": p.parseVar(true, nil, false)}), nil, nil)
	}
	return this
}

// parseClickhouseConnectorFunction ports _parse_connector_function
// (parsers/clickhouse.py:385-386 / parser.py:7180-7186).
func (p *Parser) parseClickhouseConnectorFunction(kind exp.Kind) exp.Expression {
	args := p.parseFunctionArgs(false)
	if len(args) == 0 {
		p.raiseError("Expected at least one argument")
	}
	var connected exp.Expression
	for i, arg := range args {
		if i == 0 {
			connected = arg
			continue
		}
		connected = p.expression(exp.New(kind, exp.Args{"this": connected, "expression": arg}), nil, nil)
	}
	return p.expression(exp.Paren(exp.Args{"this": connected}), nil, nil)
}

func (p *Parser) parseClickhouseAnd() exp.Expression {
	return p.parseClickhouseConnectorFunction(exp.KindAnd)
}

func (p *Parser) parseClickhouseOr() exp.Expression {
	return p.parseClickhouseConnectorFunction(exp.KindOr)
}

// parseClickhouseXor ports FUNCTION_PARSERS["XOR"] (parsers/clickhouse.py:387): exp.xor
// folds left and wraps connector operands in Paren to preserve precedence.
func (p *Parser) parseClickhouseXor() exp.Expression {
	args := p.parseFunctionArgs(false)
	if len(args) == 0 {
		return nil
	}
	wrap := func(e exp.Expression) exp.Expression {
		if e != nil && e.Is(exp.TraitConnector) {
			return p.expression(exp.Paren(exp.Args{"this": e}), nil, nil)
		}
		return e
	}
	xor := args[0]
	if len(args) > 1 {
		xor = wrap(xor)
	}
	for _, arg := range args[1:] {
		xor = p.expression(exp.Xor(exp.Args{"this": xor, "expression": wrap(arg)}), nil, nil)
	}
	return xor
}

// parseClickhouseTypes ports _parse_types (parsers/clickhouse.py:506-527): every parsed
// type is marked non-nullable unless it already carries an explicit nullable marker.
func parseClickhouseTypes(p *Parser, checkFunc, schema, allowIdentifiers, withCollation bool) exp.Expression {
	dtype := p.parseTypesBase(checkFunc, schema, allowIdentifiers, withCollation)
	if dtype != nil && dtype.Kind() == exp.KindDataType {
		if nullable, ok := dtype.Arg("nullable").(bool); !ok || !nullable {
			dtype.Set("nullable", false)
		}
	}
	return dtype
}

// parseClickhouseSettings ports the SETTINGS query modifier (parsers/clickhouse.py:446-449).
func (p *Parser) parseClickhouseSettings() []exp.Expression {
	if !p.match(tokens.SETTINGS) {
		return nil
	}
	return p.parseCsv(p.parseAssignment)
}

// parseClickhouseQueryParameter ports _parse_query_parameter (parsers/clickhouse.py:560-582).
// The caller has already consumed the opening `{`.
func (p *Parser) parseClickhouseQueryParameter(index int) exp.Expression {
	this := p.parseIdVar(false, nil)
	p.match(tokens.COLON)
	kind := p.parseTypes(false, false, false, false)
	if kind == nil && p.matchTextSeq("IDENTIFIER") {
		kind = exp.Var(exp.Args{"this": "Identifier"})
	}
	if kind == nil {
		p.retreat(index)
		return nil
	}
	if !p.match(tokens.R_BRACE) {
		p.raiseError("Expecting }")
	}
	if this != nil && this.Kind() == exp.KindIdentifier {
		if quoted, _ := this.Arg("quoted").(bool); !quoted {
			this = exp.Var(exp.Args{"this": this.Name()})
		}
	}
	return p.expression(exp.Placeholder(exp.Args{"this": this, "kind": kind}), nil, nil)
}

// parseClickhouseGlobalIn ports _parse_global_in (parsers/clickhouse.py:617-623).
func (p *Parser) parseClickhouseGlobalIn(this exp.Expression) exp.Expression {
	isNegated := p.match(tokens.NOT)
	var inExpr exp.Expression
	if p.match(tokens.IN) {
		inExpr = p.parseIn(this, false)
		if inExpr != nil {
			inExpr.Set("is_global", true)
		}
	}
	if isNegated {
		return p.expression(exp.Not(exp.Args{"this": inExpr}), nil, nil)
	}
	return inExpr
}

// parseClickhouseDetach ports _parse_detach (parsers/clickhouse.py:994-1008).
func (p *Parser) parseClickhouseDetach() exp.Expression {
	var kind any
	if p.matchSet(dbCreatables) {
		kind = stringsUpper(p.prev.Text)
	}
	exists := p.parseExists(false)
	this := p.parseTableParts(false, false, false, false)
	var cluster exp.Expression
	if p.match(tokens.ON) {
		cluster = p.parseOnProperty()
	}
	return p.expression(exp.Detach(exp.Args{
		"this":      this,
		"kind":      kind,
		"exists":    exists,
		"cluster":   cluster,
		"permanent": p.matchTextSeq("PERMANENTLY"),
		"sync":      p.matchTextSeq("SYNC"),
	}), nil, nil)
}

// parseClickhouseEngineProperty ports _parse_engine_property (parsers/clickhouse.py:496-500).
func (p *Parser) parseClickhouseEngineProperty(isDefault bool) exp.Expression {
	p.match(tokens.EQ)
	return p.expression(exp.EngineProperty(exp.Args{
		"this": p.parseField(true, nil, true),
	}), nil, nil)
}

// parseClickhouseUuidProperty ports PROPERTY_PARSERS["UUID"] (parsers/clickhouse.py:394).
func (p *Parser) parseClickhouseUuidProperty(isDefault bool) exp.Expression {
	return p.expression(exp.UuidProperty(exp.Args{"this": p.parseString()}), nil, nil)
}

// parseClickhouseAutoRefreshProperty ports _parse_auto_refresh_property
// (parsers/clickhouse.py:819-869).
func (p *Parser) parseClickhouseAutoRefreshProperty(isDefault bool) exp.Expression {
	index := p.index - 1
	var cadence any
	if p.matchTexts(map[string]bool{"EVERY": true, "AFTER": true}) {
		cadence = stringsUpper(p.prev.Text)
	}
	var interval exp.Expression
	if cadence != nil {
		interval = p.parseInterval(false)
	}
	if cadence != nil && interval == nil {
		p.retreat(index)
		return nil
	}

	var offset exp.Expression
	if p.matchTextSeq("OFFSET") {
		offset = p.parseInterval(false)
		if offset == nil {
			p.retreat(index)
			return nil
		}
	}

	var randomize exp.Expression
	if p.matchTextSeq("RANDOMIZE", "FOR") {
		randomize = p.parseInterval(false)
		if randomize == nil {
			p.retreat(index)
			return nil
		}
	}

	var dependencies []exp.Expression
	if p.matchTextSeq("DEPENDS", "ON") {
		dependencies = p.parseCsv(func() exp.Expression { return p.parseTableParts(true, false, false, false) })
		if len(dependencies) == 0 {
			p.retreat(index)
			return nil
		}
	}

	if cadence == nil && len(dependencies) == 0 {
		p.retreat(index)
		return nil
	}

	var settings exp.Expression
	if p.matchTextSeq("SETTINGS") {
		settings = p.expression(exp.SettingsProperty(exp.Args{"expressions": p.parseCsv(p.parseAssignment)}), nil, nil)
	}

	return p.expression(exp.AutoRefreshProperty(exp.Args{
		"this":        interval,
		"cadence":     cadence,
		"offset":      offset,
		"randomize":   randomize,
		"expressions": dependencies,
		"settings":    settings,
		"append":      p.matchTextSeq("APPEND"),
	}), nil, nil)
}

// parseClickhouseIndexConstraint ports _parse_index_constraint
// (parsers/clickhouse.py:871-884).
func (p *Parser) parseClickhouseIndexConstraint(kind string) exp.Expression {
	this := p.parseIdVar(false, nil)
	expression := p.parseAssignment()

	var indexType exp.Expression
	if p.matchTextSeq("TYPE") {
		indexType = p.parseFunction(nil, false, true, false)
		if indexType == nil {
			indexType = p.parseVar(true, nil, false)
		}
	}

	var granularity exp.Expression
	if p.matchTextSeq("GRANULARITY") {
		granularity = p.parseTerm()
	}

	return p.expression(exp.IndexColumnConstraint(exp.Args{
		"this":        this,
		"expression":  expression,
		"index_type":  indexType,
		"granularity": granularity,
	}), nil, nil)
}

// parseClickhouseAssumeConstraint ports _parse_assume_constraint
// (parsers/clickhouse.py:491-494).
func (p *Parser) parseClickhouseAssumeConstraint() exp.Expression {
	return p.expression(exp.AssumeColumnConstraint(exp.Args{"this": p.parseClickhouseWrappedSelectOrAssignment()}), nil, nil)
}

// parseClickhouseCheckConstraint ports _parse_check_constraint
// (parsers/clickhouse.py:486-489).
func (p *Parser) parseClickhouseCheckConstraint() exp.Expression {
	return p.expression(exp.CheckColumnConstraint(exp.Args{"this": p.parseClickhouseWrappedSelectOrAssignment()}), nil, nil)
}

// parseClickhouseWrappedSelectOrAssignment ports _parse_wrapped_select_or_assignment
// (parsers/clickhouse.py:481-484).
func (p *Parser) parseClickhouseWrappedSelectOrAssignment() exp.Expression {
	return p.parseWrapped(func() exp.Expression {
		if s := p.parseSelect(false, false, false); s != nil {
			return s
		}
		return p.parseAssignment()
	}, true)
}

// parseClickhousePartition ports _parse_partition (parsers/clickhouse.py:886-899).
func (p *Parser) parseClickhousePartition() exp.Expression {
	if !p.match(tokens.PARTITION) {
		return nil
	}
	var expressions []exp.Expression
	if p.matchTextSeq("ID") {
		expressions = []exp.Expression{
			p.expression(exp.PartitionId(exp.Args{"this": p.parseString()}), nil, nil),
		}
	} else {
		expressions = p.parseExpressions()
	}
	return p.expression(exp.Partition(exp.Args{"expressions": expressions}), nil, nil)
}

// parseClickhouseAlterTableReplace ports _parse_alter_table_replace
// (parsers/clickhouse.py:901-909).
func (p *Parser) parseClickhouseAlterTableReplace() exp.Expression {
	partition := p.parseClickhousePartition()
	if partition == nil || !p.match(tokens.FROM) {
		return nil
	}
	return p.expression(exp.ReplacePartition(exp.Args{
		"expression": partition,
		"source":     p.parseTableParts(false, false, false, false),
	}), nil, nil)
}

// parseClickhouseProjectionDef ports _parse_projection_def (parsers/clickhouse.py:931-939).
func (p *Parser) parseClickhouseProjectionDef() exp.Expression {
	if !p.match(tokens.PROJECTION) {
		return nil
	}
	return p.expression(exp.ProjectionDef(exp.Args{
		"this":       p.parseIdVar(false, nil),
		"expression": p.parseWrapped(p.parseStatement, false),
	}), nil, nil)
}

// clickhouseJoinKinds ports ClickHouseParser.JOIN_KINDS (parsers/clickhouse.py:419-425):
// the base set plus ALL/ANY/ASOF/ARRAY.
var clickhouseJoinKinds = map[tokens.TokenType]bool{
	tokens.ALL: true, tokens.ANY: true, tokens.ASOF: true, tokens.ARRAY: true,
}

// parseClickhouseJoinParts ports _parse_join_parts (parsers/clickhouse.py:671-680).
func (p *Parser) parseClickhouseJoinParts() (method, side, kind *tokens.Token) {
	var isGlobal *tokens.Token
	if p.match(tokens.GLOBAL) {
		tok := p.prev
		isGlobal = &tok
	}
	matchKind := func() *tokens.Token {
		if joinKinds[p.curr.TokenType] || clickhouseJoinKinds[p.curr.TokenType] {
			p.advance()
			tok := p.prev
			return &tok
		}
		return nil
	}
	kindPre := matchKind()
	var sideTok *tokens.Token
	if p.matchSet(joinSides) {
		tok := p.prev
		sideTok = &tok
	}
	kindTok := matchKind()

	method = isGlobal
	side = sideTok
	if side == nil {
		side = kindTok
	}
	kind = kindPre
	if kind == nil {
		kind = kindTok
	}
	return method, side, kind
}

// clickhouseTableToColumn ports Table.to_column (expressions/query.py:1004-1018).
func clickhouseTableToColumn(table exp.Expression) exp.Expression {
	parts := table.Parts()
	if len(parts) == 0 {
		return table
	}
	last := parts[len(parts)-1]
	var col exp.Expression
	if last.Kind() != exp.KindIdentifier {
		col = last
	} else {
		args := exp.Args{}
		upper := parts
		if len(upper) > 4 {
			args["fields"] = upper[4:]
			upper = upper[:4]
		}
		switch len(upper) {
		case 1:
			args["this"] = upper[0]
		case 2:
			args["this"] = upper[1]
			args["table"] = upper[0]
		case 3:
			args["this"] = upper[2]
			args["table"] = upper[1]
			args["schema"] = upper[0]
		case 4:
			args["this"] = upper[3]
			args["table"] = upper[2]
			args["schema"] = upper[1]
			args["catalog"] = upper[0]
		}
		col = exp.Column(args)
	}
	if alias := asExpr(table.Arg("alias")); alias != nil {
		if aliasThis := asExpr(alias.Arg("this")); aliasThis != nil {
			col = exp.AliasNode(exp.Args{"this": col, "alias": aliasThis})
		}
	}
	return col
}

// parseSettingsProperty ports _parse_settings_property (parser.py:3006-3009).
func (p *Parser) parseSettingsProperty() exp.Expression {
	return p.expression(exp.SettingsProperty(exp.Args{
		"expressions": p.parseCsv(p.parseAssignment),
	}), nil, nil)
}

// parseMergeTreeTTL ports _parse_ttl (parser.py:2317-2353).
func (p *Parser) parseMergeTreeTTL() exp.Expression {
	parseTTLAction := func() exp.Expression {
		this := p.parseBitwise()
		if p.matchTextSeq("DELETE") {
			return p.expression(exp.MergeTreeTTLAction(exp.Args{"this": this, "delete": true}), nil, nil)
		}
		if p.matchTextSeq("RECOMPRESS") {
			return p.expression(exp.MergeTreeTTLAction(exp.Args{"this": this, "recompress": p.parseBitwise()}), nil, nil)
		}
		if p.matchTextSeq("TO", "DISK") {
			return p.expression(exp.MergeTreeTTLAction(exp.Args{"this": this, "to_disk": p.parseString()}), nil, nil)
		}
		if p.matchTextSeq("TO", "VOLUME") {
			return p.expression(exp.MergeTreeTTLAction(exp.Args{"this": this, "to_volume": p.parseString()}), nil, nil)
		}
		return this
	}

	expressions := p.parseCsv(parseTTLAction)
	where := p.parseWhere(false)
	group := p.parseGroup(false)
	var aggregates []exp.Expression
	if group != nil && p.match(tokens.SET) {
		aggregates = p.parseCsv(p.parseSetItem)
	}
	return p.expression(exp.MergeTreeTTL(exp.Args{
		"expressions": expressions,
		"where":       where,
		"group":       group,
		"aggregates":  aggregates,
	}), nil, nil)
}

// parseDictProperty ports _parse_dict_property (parser.py:9607-9626).
func (p *Parser) parseDictProperty(this string) exp.Expression {
	var settings []exp.Expression
	p.match(tokens.L_PAREN)
	kind := p.parseIdVar(false, nil)
	if p.match(tokens.L_PAREN) {
		for {
			key := p.parseIdVar(false, nil)
			value := p.parseFunction(nil, false, true, false)
			if value == nil {
				value = p.parsePrimaryOrVar()
			}
			if key == nil && value == nil {
				break
			}
			settings = append(settings, p.expression(exp.DictSubProperty(exp.Args{"this": key, "value": value}), nil, nil))
		}
		p.match(tokens.R_PAREN)
	}
	p.match(tokens.R_PAREN)
	var kindName any
	if kind != nil {
		kindName = kind.Name()
	}
	return p.expression(exp.DictProperty(exp.Args{
		"this":     this,
		"kind":     kindName,
		"settings": settings,
	}), nil, nil)
}

// parseDictRange ports _parse_dict_range (parser.py:9628-9639).
func (p *Parser) parseDictRange(this string) exp.Expression {
	p.match(tokens.L_PAREN)
	var minExpr, maxExpr exp.Expression
	if p.matchTextSeq("MIN") {
		minExpr = p.parseVar(false, nil, false)
		if minExpr == nil {
			minExpr = p.parsePrimary()
		}
		p.matchTextSeq("MAX")
		maxExpr = p.parseVar(false, nil, false)
		if maxExpr == nil {
			maxExpr = p.parsePrimary()
		}
	} else {
		maxExpr = p.parseVar(false, nil, false)
		if maxExpr == nil {
			maxExpr = p.parsePrimary()
		}
		minExpr = exp.LiteralNumber("0")
	}
	p.match(tokens.R_PAREN)
	return p.expression(exp.DictRange(exp.Args{"this": this, "min": minExpr, "max": maxExpr}), nil, nil)
}

// applyClickhouseApplyChain ports the `APPLY(func)` loop in _parse_expression
// (parsers/clickhouse.py:952-960).
func (p *Parser) applyClickhouseApplyChain(this exp.Expression) exp.Expression {
	for p.matchPair(tokens.APPLY, tokens.L_PAREN, true) {
		this = p.expression(exp.Apply(exp.Args{
			"this":       this,
			"expression": p.parseVar(true, nil, false),
		}), nil, nil)
		p.match(tokens.R_PAREN)
	}
	return this
}

// parseClickhouseBracket ports _parse_bracket (parsers/clickhouse.py:584-615).
func (p *Parser) parseClickhouseBracket(this exp.Expression) exp.Expression {
	if this != nil {
		var jsonType exp.Expression
		for p.matchPair(tokens.L_BRACKET, tokens.R_BRACKET, true) {
			inner := jsonType
			if inner == nil {
				inner = exp.DTypeJSON.IntoExpr(exp.Args{"nullable": false})
			}
			jsonType = exp.DataType(exp.Args{
				"this":        exp.DTypeArray,
				"expressions": []exp.Expression{inner},
				"nested":      true,
			})
		}
		if jsonType != nil {
			return p.expression(exp.JSONCast(exp.Args{"this": this, "to": jsonType}), nil, nil)
		}
	}

	lBrace := p.match(tokens.L_BRACE, false)
	bracket := p.parseBracketBase(this)
	if lBrace && bracket != nil && bracket.Kind() == exp.KindStruct {
		keys := exp.Array(nil)
		values := exp.Array(nil)
		for _, item := range bracket.Expressions() {
			if item.Kind() != exp.KindPropertyEQ {
				break
			}
			keys.Append("expressions", exp.LiteralString(item.Name()))
			values.Append("expressions", asExpr(item.Arg("expression")))
		}
		return p.expression(exp.VarMap(exp.Args{"keys": keys, "values": values}), nil, nil)
	}
	return bracket
}

// parseClickhouseExtract ports _parse_extract (parsers/clickhouse.py:529-544): EXTRACT
// without a FROM is the ClickHouse two-argument function form, kept Anonymous.
func (p *Parser) parseClickhouseExtract() exp.Expression {
	index := p.index
	this := p.parseBitwise()
	if p.match(tokens.FROM) {
		p.retreat(index)
		return p.parseExtractBase()
	}
	p.match(tokens.COMMA)
	return p.expression(exp.Anonymous(exp.Args{
		"this":        "extract",
		"expressions": []exp.Expression{this, p.parseBitwise()},
	}), nil, nil)
}

// parseDistributedProperty ports _parse_distributed_property (parser.py:3076-3090).
func (p *Parser) parseDistributedProperty() exp.Expression {
	kind := "HASH"
	var expressions []exp.Expression
	if p.matchTextSeq("BY", "HASH") {
		expressions = p.parseWrappedCsv(func() exp.Expression { return p.parseIdVar(false, nil) })
	} else if p.matchTextSeq("BY", "RANDOM") {
		kind = "RANDOM"
	}
	var buckets exp.Expression
	if p.matchTextSeq("BUCKETS") && !p.matchTextSeq("AUTO") {
		buckets = p.parseNumber()
	}
	return p.expression(exp.DistributedByProperty(exp.Args{
		"expressions": expressions,
		"kind":        kind,
		"buckets":     buckets,
		"order":       p.parseOrder(nil, false),
	}), nil, nil)
}
