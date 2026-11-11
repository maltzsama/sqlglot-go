package parser

import (
	"strings"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// parsers/bigquery.py supplies the BigQueryParser overlays: FUNCTIONS entries live in
// dialects/bigquery.go; these are the FUNCTION_PARSERS/STATEMENT_PARSERS callbacks and the
// grammar helpers they share.
func init() {
	registerDialectParserOverrides("bigquery", dialectParserOverrideSet{
		FunctionParsers: map[string]parserOverrideFunc{
			// parsers/bigquery.py:325-346 FUNCTION_PARSERS.
			"ARRAY": func(p *Parser) exp.Expression {
				return p.expression(exp.Array(exp.Args{
					"expressions":             []exp.Expression{p.parseStatement()},
					"struct_name_inheritance": true,
				}), nil, nil)
			},
			"JSON_ARRAY": func(p *Parser) exp.Expression {
				return p.expression(exp.JSONArray(exp.Args{
					"expressions": p.parseCsv(func() exp.Expression { return p.parseBitwise() }),
				}), nil, nil)
			},
			"MAKE_INTERVAL":    (*Parser).parseBigQueryMakeInterval,
			"PREDICT":          func(p *Parser) exp.Expression { return p.parseBigQueryML(exp.KindPredict, nil) },
			"TRANSLATE":        (*Parser).parseBigQueryTranslate,
			"FEATURES_AT_TIME": (*Parser).parseBigQueryFeaturesAtTime,
			"GENERATE_EMBEDDING": func(p *Parser) exp.Expression {
				return p.parseBigQueryML(exp.KindGenerateEmbedding, nil)
			},
			"GENERATE_TEXT_EMBEDDING": func(p *Parser) exp.Expression {
				return p.parseBigQueryML(exp.KindGenerateEmbedding, exp.Args{"is_text": true})
			},
			"GENERATE_TEXT":   func(p *Parser) exp.Expression { return p.parseBigQueryGenerate(exp.KindGenerateText, nil) },
			"GENERATE_TABLE":  func(p *Parser) exp.Expression { return p.parseBigQueryGenerate(exp.KindGenerateTable, nil) },
			"GENERATE_BOOL":   func(p *Parser) exp.Expression { return p.parseBigQueryGenerate(exp.KindGenerateBool, nil) },
			"GENERATE_INT":    func(p *Parser) exp.Expression { return p.parseBigQueryGenerate(exp.KindGenerateInt, nil) },
			"GENERATE_DOUBLE": func(p *Parser) exp.Expression { return p.parseBigQueryGenerate(exp.KindGenerateDouble, nil) },
			"VECTOR_SEARCH":   (*Parser).parseBigQueryVectorSearch,
			"FORECAST":        (*Parser).parseBigQueryForecast,
			// parsers/bigquery.py:558-577 _parse_json_object (array key/value pairs).
			"JSON_OBJECT": func(p *Parser) exp.Expression { return p.parseBigQueryJSONObject(false) },
		},
		// parsers/bigquery.py:348-351 NO_PAREN_FUNCTIONS adds CURRENT_DATETIME.
		NoParenFunctions: map[tokens.TokenType]func(exp.Args) exp.Expression{
			tokens.CURRENT_DATETIME: func(exp.Args) exp.Expression { return exp.CurrentDatetime(exp.Args{}) },
		},
		ColumnOps:     bigQueryColumnOps,
		Column:        bigQueryColumn,
		QualifiedFunc: bigQueryQualifiedFunc,
		PropertyParsers: map[string]propertyParserFunc{
			// parsers/bigquery.py:358-361 PROPERTY_PARSERS["OPTIONS"].
			"OPTIONS": func(p *Parser, _ bool) exp.Expression { return p.parseWithProperty() },
			// parsers/bigquery.py:539-544 _parse_cluster_property (CSV, no parens).
			"CLUSTER BY": func(p *Parser, _ bool) exp.Expression {
				return p.expression(exp.ClusterProperty(exp.Args{
					"expressions": p.parseCsv(func() exp.Expression { return p.parseColumn() }),
				}), nil, nil)
			},
		},
		TablePart:  bigQueryTablePart,
		TableParts: bigQueryTableParts,
		StatementParsers: map[tokens.TokenType]parserOverrideFunc{
			// parsers/bigquery.py:378-384 STATEMENT_PARSERS.
			tokens.ELSE:   func(p *Parser) exp.Expression { return p.parseAsCommand(p.prev) },
			tokens.END:    func(p *Parser) exp.Expression { return p.parseAsCommand(p.prev) },
			tokens.FOR:    (*Parser).parseBigQueryForIn,
			tokens.EXPORT: (*Parser).parseBigQueryExport,
			// The base DECLARE grammar is the same one Trino/Spark register.
			tokens.DECLARE: (*Parser).parseDeclareStatement,
		},
	})
}

// bigQueryMakeIntervalKwargs ports MAKE_INTERVAL_KWARGS (parsers/bigquery.py:186).
var bigQueryMakeIntervalKwargs = []string{"year", "month", "week", "day", "hour", "minute", "second"}

// parseBigQueryMakeInterval ports _parse_make_interval (parsers/bigquery.py:625-643): named
// arguments can appear in any order after the positional ones.
func (p *Parser) parseBigQueryMakeInterval() exp.Expression {
	args := exp.Args{}
	for _, argKey := range bigQueryMakeIntervalKwargs {
		value := p.parseLambda(false)
		if value == nil {
			break
		}
		if value.Kind() == exp.KindKwarg {
			argKey = value.This().Name()
		}
		args[argKey] = value
		p.match(tokens.COMMA)
	}
	return p.expression(exp.MakeInterval(args), nil, nil)
}

// parseBigQueryML ports _parse_ml (parsers/bigquery.py:645-663):
// MODEL <model> [, TABLE <table-or-query>] [, <params struct>].
func (p *Parser) parseBigQueryML(kind exp.Kind, kwargs exp.Args) exp.Expression {
	p.matchTextSeq("MODEL")
	this := p.parseTable(false, false, nil, false, false, false, false)
	p.match(tokens.COMMA)
	p.matchTextSeq("TABLE")
	var expression exp.Expression
	if !p.match(tokens.STRUCT, false) {
		expression = p.parseTable(false, false, nil, false, false, false, false)
	}
	p.match(tokens.COMMA)
	args := exp.Args{"this": this, "expression": expression, "params_struct": p.parseBitwise()}
	for k, v := range kwargs {
		args[k] = v
	}
	return p.expression(exp.New(kind, args), nil, nil)
}

// parseBigQueryGenerate ports _parse_generate (parsers/bigquery.py:665-682).
func (p *Parser) parseBigQueryGenerate(kind exp.Kind, kwargs exp.Args) exp.Expression {
	p.matchTextSeq("MODEL")
	this := p.parseTable(false, false, nil, false, false, false, false)
	p.match(tokens.COMMA)

	var expression exp.Expression
	switch {
	case p.matchTextSeq("TABLE"):
		expression = p.parseTable(false, false, nil, false, false, false, false)
	case p.match(tokens.L_PAREN, false):
		expression = p.parseTable(false, false, nil, false, false, false, false)
	default:
		expression = p.parseBitwise()
	}

	var paramsStruct exp.Expression
	if p.match(tokens.COMMA) {
		paramsStruct = p.parseBitwise()
	}
	args := exp.Args{"this": this, "expression": expression, "params_struct": paramsStruct}
	for k, v := range kwargs {
		args[k] = v
	}
	return p.expression(exp.New(kind, args), nil, nil)
}

// parseBigQueryTranslate ports _parse_translate (parsers/bigquery.py:684-690): ML.TRANSLATE
// uses the ML grammar, everything else the plain function arguments.
func (p *Parser) parseBigQueryTranslate() exp.Expression {
	if p.bigQueryMLPrefix() {
		return p.parseBigQueryML(exp.KindMLTranslate, nil)
	}
	return exp.FromArgList(exp.KindTranslate, p.parseFunctionArgs(false))
}

// parseBigQueryForecast ports _parse_forecast (parsers/bigquery.py:692-714).
func (p *Parser) parseBigQueryForecast() exp.Expression {
	if p.bigQueryMLPrefix() {
		return p.parseBigQueryML(exp.KindMLForecast, nil)
	}
	p.match(tokens.TABLE)
	this := p.parseTable(false, false, nil, false, false, false, false)
	if this == nil {
		p.raiseError("Expected table or query statement")
	}
	result := p.expression(exp.AIForecast(exp.Args{"this": this}), nil, nil)
	for p.match(tokens.COMMA) {
		arg := p.parseLambda(false)
		if arg != nil && arg.Kind() == exp.KindKwarg {
			result.Set(strings.ToLower(arg.This().Name()), arg)
			continue
		}
		p.raiseError("Expected key => value syntax for AI.FORECAST")
		break
	}
	return result
}

// parseBigQueryFeaturesAtTime ports _parse_features_at_time (parsers/bigquery.py:716-730).
func (p *Parser) parseBigQueryFeaturesAtTime() exp.Expression {
	p.match(tokens.TABLE)
	this := p.parseTable(false, false, nil, false, false, false, false)
	result := p.expression(exp.FeaturesAtTime(exp.Args{"this": this}), nil, nil)
	for p.match(tokens.COMMA) {
		arg := p.parseLambda(false)
		if arg != nil {
			result.Set(strings.ToLower(arg.This().Name()), arg)
		}
	}
	return result
}

// parseBigQueryVectorSearch ports _parse_vector_search (parsers/bigquery.py:732-760).
func (p *Parser) parseBigQueryVectorSearch() exp.Expression {
	p.match(tokens.TABLE)
	baseTable := p.parseTable(false, false, nil, false, false, false, false)
	p.match(tokens.COMMA)
	columnToSearch := p.parseBitwise()
	p.match(tokens.COMMA)
	p.match(tokens.TABLE)
	queryTable := p.parseTable(false, false, nil, false, false, false, false)
	result := p.expression(exp.VectorSearch(exp.Args{
		"this":             baseTable,
		"column_to_search": columnToSearch,
		"query_table":      queryTable,
	}), nil, nil)
	for p.match(tokens.COMMA) {
		if p.match(tokens.STRING, false) {
			result.Set("query_column_to_search", p.parseString())
			continue
		}
		if arg := p.parseLambda(false); arg != nil {
			result.Set(strings.ToLower(arg.This().Name()), arg)
		}
	}
	return result
}

// parseBigQueryExport ports _parse_export_data (parsers/bigquery.py:762-771).
func (p *Parser) parseBigQueryExport() exp.Expression {
	p.matchTextSeq("DATA")
	var connection exp.Expression
	if p.matchTextSeq("WITH", "CONNECTION") {
		connection = p.parseTableParts(false, false, false, false)
	}
	options := p.parseProperties()
	var this exp.Expression
	if p.matchTextSeq("AS") {
		this = p.parseSelect(true, false, true, false)
	}
	return p.expression(exp.Export(exp.Args{
		"this":       this,
		"connection": connection,
		"options":    options,
	}), nil, nil)
}

// parseBigQueryForIn ports _parse_for_in (parsers/bigquery.py:393-400).
func (p *Parser) parseBigQueryForIn() exp.Expression {
	index := p.index
	this := p.parseRange(nil)
	p.matchTextSeq("DO")
	if p.match(tokens.COMMAND) {
		p.retreat(index)
		return p.parseAsCommand(p.prev)
	}
	return p.expression(exp.ForIn(exp.Args{"this": this, "expression": p.parseStatement()}), nil, nil)
}

// bigQueryMLPrefix reports whether the function name is qualified with ML. — upstream inspects
// the token four positions back (parsers/bigquery.py:686-687).
func (p *Parser) bigQueryMLPrefix() bool {
	index := p.index - 4
	if index < 0 || index >= len(p.tokens) {
		return false
	}
	return stringsUpper(p.tokens[index].Text) == "ML"
}

// bigQueryColumnOps ports _parse_column_ops (parsers/bigquery.py:773-799).
func bigQueryColumnOps(p *Parser, this exp.Expression, funcIndex int) exp.Expression {
	if this == nil || this.Kind() != exp.KindDot {
		return this
	}
	inner, ok := this.Arg("expression").(exp.Expression)
	if !ok || inner == nil || !inner.Is(exp.TraitFunc) || !inner.Is(exp.TraitCondition) {
		return this
	}
	prefix := strings.ToUpper(this.This().Name())
	var kind exp.Kind
	switch prefix {
	case "NET":
		kind = exp.KindNetFunc
	case "SAFE":
		kind = exp.KindSafeFunc
	}
	if kind != 0 {
		p.retreat(funcIndex)
		return p.expression(exp.New(kind, exp.Args{"this": p.parseFunction(nil, false, true, true)}), nil, nil)
	}
	if prefix == "AI" || prefix == "ML" {
		p.retreat(funcIndex)
		if parsed := p.parseFunction(nil, false, true, true); parsed != nil {
			return p.expression(exp.Dot(exp.Args{"this": this.This(), "expression": parsed}), nil, nil)
		}
	}
	return this
}

// bigQueryDashedTablePartFollowTokens ports DASHED_TABLE_PART_FOLLOW_TOKENS
// (parsers/bigquery.py:372-376).
var bigQueryDashedTablePartFollowTokens = map[tokens.TokenType]bool{
	tokens.DOT:     true,
	tokens.L_PAREN: true,
	tokens.R_PAREN: true,
}

// bigQuerySplitNumWords ports split_num_words (helper.py:274-320).
func bigQuerySplitNumWords(value, sep string, minNumWords int) []string {
	words := strings.Split(value, sep)
	if padding := minNumWords - len(words); padding > 0 {
		words = append(make([]string, padding), words...)
	}
	return words
}

// bigQuerySplitQualifiedName ports _split_qualified_name (parsers/bigquery.py:169-183): a
// dashed project ID like `domain.com:project-id` keeps its domain dots.
func bigQuerySplitQualifiedName(name string, minNumWords int) []string {
	if colon := strings.Index(name, ":"); colon != -1 && strings.Contains(name[:colon], ".") {
		masked := strings.ReplaceAll(name[:colon], ".", "\x00") + name[colon:]
		parts := bigQuerySplitNumWords(masked, ".", minNumWords)
		for i, part := range parts {
			parts[i] = strings.ReplaceAll(part, "\x00", ".")
		}
		return parts
	}
	return bigQuerySplitNumWords(name, ".", minNumWords)
}

// bigQueryTablePart ports _parse_table_part (parsers/bigquery.py:402-425): a dashed table name
// stays one identifier and a numeric part becomes a quoted identifier.
func bigQueryTablePart(p *Parser, schema bool, this exp.Expression) exp.Expression {
	if this == nil {
		this = p.parseNumber()
	}
	if this == nil {
		return nil
	}
	switch this.Kind() {
	case exp.KindIdentifier:
		tableName := this.Name()
		for p.match(tokens.DASH, false) && p.next.IsValid() {
			startIndex := p.index
			for p.isConnected() && !p.matchSet(bigQueryDashedTablePartFollowTokens, false) {
				p.advance()
			}
			if p.index == startIndex {
				break
			}
			tableName += p.findSQL(p.tokens[startIndex], p.prev)
		}
		return p.expression(exp.Identifier(exp.Args{"this": tableName, "quoted": this.Arg("quoted")}), nil, nil)
	case exp.KindLiteral:
		tableName := this.Name()
		if p.isConnected() && p.parseVar(true, nil, false) != nil {
			tableName += p.prev.Text
		}
		return p.expression(exp.Identifier(exp.Args{"this": tableName, "quoted": true}), nil, nil)
	}
	return this
}

// bigQueryTableParts ports _parse_table_parts (parsers/bigquery.py:427-518).
func bigQueryTableParts(p *Parser, table exp.Expression) exp.Expression {
	if table == nil || table.Kind() != exp.KindTable {
		return table
	}
	// proj-1.db.tbl -- `1.` is tokenized as a float, so unravel it here.
	if table.Arg("catalog") == nil {
		if schemaPart := asExpr(table.Arg("schema")); schemaPart != nil {
			parts := strings.Split(schemaPart.Name(), ".")
			if len(parts) == 2 && !truthyExpr(schemaPart.Arg("quoted")) {
				table.Set("catalog", p.expression(exp.Identifier(exp.Args{"this": parts[0]}), nil, nil))
				table.Set("schema", p.expression(exp.Identifier(exp.Args{"this": parts[1]}), nil, nil))
			}
		} else if this := asExpr(table.Arg("this")); this != nil {
			parts := strings.Split(this.Name(), ".")
			if len(parts) == 2 && !truthyExpr(this.Arg("quoted")) {
				table.Set("schema", p.expression(exp.Identifier(exp.Args{"this": parts[0]}), nil, nil))
				table.Set("this", p.expression(exp.Identifier(exp.Args{"this": parts[1]}), nil, nil))
			}
		}
	}

	var alias exp.Expression
	this := asExpr(table.Arg("this"))
	hasDottedPart := false
	for _, part := range table.Parts() {
		if strings.Contains(part.Name(), ".") {
			hasDottedPart = true
			break
		}
	}
	if this != nil && this.Kind() == exp.KindIdentifier && hasDottedPart {
		alias = this
		names := make([]string, 0, len(table.Parts()))
		for _, part := range table.Parts() {
			names = append(names, part.Name())
		}
		split := bigQuerySplitQualifiedName(strings.Join(names, "."), 3)
		ids := make([]exp.Expression, 0, len(split))
		for _, name := range split {
			if name == "" {
				ids = append(ids, nil)
				continue
			}
			ids = append(ids, p.expression(exp.Identifier(exp.Args{"this": name, "quoted": true}), nil, nil))
		}
		catalog, schemaPart, thisID := ids[0], ids[1], ids[2]
		rest := ids[3:]
		inner := thisID
		for _, part := range rest {
			inner = exp.Dot(exp.Args{"this": inner, "expression": part})
		}
		rebuilt := p.expression(exp.Table(exp.Args{
			"this":    inner,
			"schema":  schemaPart,
			"catalog": catalog,
			"pivots":  table.Arg("pivots"),
		}), nil, nil)
		rebuilt.Meta()["quoted_table"] = true
		table = rebuilt
	}

	// The INFORMATION_SCHEMA views need a region or dataset qualifier; if the project is
	// omitted, represent `INFORMATION_SCHEMA.X` as one quoted identifier so the Table is not
	// misread as a 4-part reference (parsers/bigquery.py:484-514).
	parts := table.Parts()
	if len(parts) > 1 && strings.ToUpper(parts[len(parts)-2].Name()) == "INFORMATION_SCHEMA" {
		aliasTarget := alias
		if aliasTarget == nil {
			aliasTarget = parts[len(parts)-1]
		}
		table.Set("alias", exp.TableAlias(exp.Args{"this": aliasTarget}))
		view := parts[len(parts)-2].Name() + "." + parts[len(parts)-1].Name()
		table.Set("this", p.expression(exp.Identifier(exp.Args{"this": view, "quoted": true}), nil, nil))
		var schemaPart, catalog exp.Expression
		if len(parts) >= 3 {
			schemaPart = parts[len(parts)-3]
		}
		if len(parts) >= 4 {
			catalog = parts[len(parts)-4]
		}
		table.Set("schema", schemaPart)
		table.Set("catalog", catalog)
	}
	return table
}

// truthyExpr reports whether a node's bool arg is true.
func truthyExpr(value any) bool {
	b, ok := value.(bool)
	return ok && b
}

// parseBigQueryJSONObject ports the BigQueryParser._parse_json_object override
// (parsers/bigquery.py:558-577): the array form JSON_OBJECT([keys], [values]) is expanded into
// one JSONKeyValue per pair.
func (p *Parser) parseBigQueryJSONObject(agg bool) exp.Expression {
	jsonObject := p.parseJSONObject(agg)
	if jsonObject == nil {
		return nil
	}
	items := jsonObject.Expressions()
	if len(items) == 0 {
		return jsonObject
	}
	pair := items[0]
	key := asExpr(pair.Arg("this"))
	value := asExpr(pair.Arg("expression"))
	if key == nil || value == nil || key.Kind() != exp.KindArray || value.Kind() != exp.KindArray {
		return jsonObject
	}
	keys := key.Expressions()
	values := value.Expressions()
	pairs := make([]exp.Expression, 0, len(keys))
	for i, k := range keys {
		var v exp.Expression
		if i < len(values) {
			v = values[i]
		}
		pairs = append(pairs, p.expression(exp.JSONKeyValue(exp.Args{"this": k, "expression": v}), nil, nil))
	}
	jsonObject.Set("expressions", pairs)
	return jsonObject
}

// bigQueryColumn ports the BigQueryParser._parse_column override
// (parsers/bigquery.py:518-536): a quoted reference that spans dots is split into up to four
// quoted parts and marked quoted_column.
func bigQueryColumn(p *Parser, column exp.Expression) exp.Expression {
	if column == nil || column.Kind() != exp.KindColumn {
		return column
	}
	parts := column.Parts()
	hasDotted := false
	for _, part := range parts {
		if strings.Contains(part.Name(), ".") {
			hasDotted = true
			break
		}
	}
	if !hasDotted {
		return column
	}
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		names = append(names, part.Name())
	}
	split := bigQuerySplitQualifiedName(strings.Join(names, "."), 4)
	ids := make([]exp.Expression, 0, len(split))
	for _, name := range split {
		if name == "" {
			ids = append(ids, nil)
			continue
		}
		ids = append(ids, p.expression(exp.Identifier(exp.Args{"this": name, "quoted": true}), nil, nil))
	}
	catalog, schemaPart, tablePart, thisID := ids[0], ids[1], ids[2], ids[3]
	inner := thisID
	for _, part := range ids[4:] {
		inner = exp.Dot(exp.Args{"this": inner, "expression": part})
	}
	built := p.expression(exp.Column(exp.Args{
		"this":    inner,
		"table":   tablePart,
		"schema":  schemaPart,
		"catalog": catalog,
	}), nil, nil)
	built.Meta()["quoted_column"] = true
	return built
}

// bigQueryQualifiedFunc parses AI./ML. function calls with their custom BigQuery signatures
// (parsers/bigquery.py:790-799) before the anonymous fallback would mis-parse them.
func bigQueryQualifiedFunc(p *Parser, this exp.Expression) (exp.Expression, bool) {
	if this == nil || this.Kind() != exp.KindColumn || len(this.Parts()) != 1 || p.next.TokenType != tokens.L_PAREN {
		return nil, false
	}
	switch strings.ToUpper(this.Name()) {
	case "AI", "ML":
		return p.parseFunction(nil, false, true, true), true
	}
	return nil, false
}
