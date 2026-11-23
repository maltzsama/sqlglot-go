package parser

import (
	"strings"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// ksqlDB parser overlays — the beyond-upstream ksqlDB dialect (Confluent ksqldb-parser
// SqlBase.g4, ported 1:1 where structured). Every statement root that SqlBase.g4:47-99
// declares gets a StatementParsers entry; the query's WINDOW / PARTITION BY / EMIT clauses
// (SqlBase.g4:117-127) are QueryModifiers; the join's WITHIN window (SqlBase.g4:251-262)
// is a hook in parseJoinBase. PRINT / ASSERT / RUN SCRIPT degrade to a verbatim Command
// (their printClause / timeout clauses are not modeled), as do any statements whose trailing
// tokens the structured parse cannot consume.
func init() {
	registerDialectParserOverrides("ksqldb", dialectParserOverrideSet{
		StatementParsers: map[tokens.TokenType]parserOverrideFunc{
			tokens.CREATE:    (*Parser).parseKsqlCreate,
			tokens.INSERT:    (*Parser).parseKsqlInsert,
			tokens.DROP:      (*Parser).parseKsqlDrop,
			tokens.SHOW:      (*Parser).parseKsqlShow,
			tokens.LIST:      (*Parser).parseKsqlShow,
			tokens.DESCRIBE:  (*Parser).parseKsqlDescribe,
			tokens.EXPLAIN:   (*Parser).parseKsqlExplain,
			tokens.ALTER:     (*Parser).parseKsqlAlter,
			tokens.TERMINATE: (*Parser).parseKsqlKill,
			tokens.PAUSE:     (*Parser).parseKsqlKill,
			tokens.RESUME:    (*Parser).parseKsqlKill,
			tokens.UNSET:     (*Parser).parseKsqlUnset,
			tokens.DEFINE:    (*Parser).parseKsqlDefine,
			tokens.UNDEFINE:  (*Parser).parseKsqlUndefine,
			// printClause / assert timeout / RUN SCRIPT are not modeled: raw text.
			tokens.PRINT:  (*Parser).parseKsqlCommandStatement,
			tokens.ASSERT: (*Parser).parseKsqlCommandStatement,
			tokens.RUN:    (*Parser).parseKsqlCommandStatement,
		},
		QueryModifiers: map[tokens.TokenType]func(*Parser) (string, any){
			tokens.WINDOW:       func(p *Parser) (string, any) { return "windows", p.parseKsqlWindowClause() },
			tokens.EMIT:         (*Parser).parseKsqlEmit,
			tokens.PARTITION_BY: (*Parser).parseKsqlPartitionBy,
		},
	})
}

// ksqlStructured runs a ksqlDB statement body under error isolation and degrades to a
// verbatim Command when the body rejects or leaves over any token. ksqlDB's statements are
// chunked like every other dialect (parseBatchStatements errors on an unconsumed top-level
// token), so a partial structured parse must roll back and hand the whole chunk to
// parseAsCommand — the same contract base parseCreate uses (parser/parser_ddl.go:10-34).
func (p *Parser) ksqlStructured(start tokens.Token, structured func() exp.Expression) exp.Expression {
	if stmt := p.tryParse(func() exp.Expression {
		stmt := structured()
		if stmt == nil || p.index < p.tokensSize {
			return nil
		}
		return stmt
	}, false); stmt != nil {
		return stmt
	}
	return p.parseAsCommand(start)
}

// parseKsqlCommandStatement degrades PRINT / ASSERT / RUN SCRIPT to raw text: the leading
// token was already consumed by parseStatement's advance (start = p.prev).
func (p *Parser) parseKsqlCommandStatement() exp.Expression {
	return p.parseAsCommand(p.prev)
}

// ---------------------------------------------------------------------------
// CREATE (SqlBase.g4:74-93)
// ---------------------------------------------------------------------------

// parseKsqlCreate ports CREATE STREAM/TABLE/SOURCE STREAM/CONNECTOR/TYPE.
func (p *Parser) parseKsqlCreate() exp.Expression {
	start := p.prev
	return p.ksqlStructured(start, p.parseKsqlCreateStructured)
}

func (p *Parser) parseKsqlCreateStructured() exp.Expression {
	replace := p.matchTextSeq("OR", "REPLACE")

	// CREATE (SINK | SOURCE) CONNECTOR (IF NOT EXISTS)? identifier WITH tableProperties
	if p.matchTextSeq("SOURCE", "CONNECTOR") || p.matchTextSeq("SINK", "CONNECTOR") {
		verb := stringsUpper(p.tokens[p.index-2].Text)
		args := exp.Args{"kind": verb + " CONNECTOR"}
		if exists := p.parseExists(true); exists == true {
			args["exists"] = true
		}
		this := p.parseIdVar(true, nil)
		if this == nil || !p.matchTextSeq("WITH") {
			return nil
		}
		properties := p.parseKsqlTableProperties()
		if properties == nil {
			return nil
		}
		args["this"] = this
		args["properties"] = properties
		if replace {
			args["replace"] = true
		}
		return p.expression(exp.Create(args), nil, nil)
	}

	// CREATE TYPE (IF NOT EXISTS)? identifier AS type
	if p.matchTextSeq("TYPE") {
		args := exp.Args{"kind": "TYPE"}
		if exists := p.parseExists(true); exists == true {
			args["exists"] = true
		}
		this := p.parseIdVar(true, nil)
		if this == nil || !p.matchTextSeq("AS") {
			return nil
		}
		dtype := p.parseTypes(false, true, true, false)
		if dtype == nil {
			return nil
		}
		args["this"] = this
		args["expression"] = dtype
		if replace {
			args["replace"] = true
		}
		return p.expression(exp.Create(args), nil, nil)
	}

	// CREATE (OR REPLACE)? (SOURCE)? {STREAM|TABLE} (IF NOT EXISTS)? sourceName
	//     (tableElements)? (WITH tableProperties)? [AS query]
	source := p.matchTextSeq("SOURCE")
	if !p.matchTexts(map[string]bool{"STREAM": true, "TABLE": true}) {
		return nil
	}
	kind := stringsUpper(p.prev.Text)
	if source {
		kind = "SOURCE " + kind
	}
	args := exp.Args{"kind": kind}
	if exists := p.parseExists(true); exists == true {
		args["exists"] = true
	}
	// schema=true skips base parseTablePart's function-call fast path so `name (...)` is
	// read as the tableElements list, not a call `name(...)` (parser/parser.go:1720-1727).
	this := p.parseTableParts(true, false, false, false)
	if this == nil {
		return nil
	}
	// (tableElements)? — the parenthesized column list, absent on the AS-query forms.
	schema := p.parseKsqlSchema(this)
	if schema == nil {
		return nil
	}
	args["this"] = schema
	if p.matchTextSeq("WITH") {
		properties := p.parseKsqlTableProperties()
		if properties == nil {
			return nil
		}
		args["properties"] = properties
	}
	hasElements := schema.Kind() == exp.KindSchema
	if p.match(tokens.ALIAS) {
		query := p.parseDDLSelect()
		// SqlBase.g4:74-83: the tableElements form and the AS-query form are separate
		// alternatives — a CREATE carrying both is not ksqlDB, so fail closed and let the
		// statement round-trip as a raw Command instead of admitting SQL the engine rejects.
		if query == nil || hasElements {
			return nil
		}
		args["expression"] = query
	}
	if replace {
		args["replace"] = true
	}
	return p.expression(exp.Create(args), nil, nil)
}

// parseKsqlSchema parses the optional parenthesized tableElements list of a CREATE
// (SqlBase.g4:142-148); the bare table passes through unchanged.
func (p *Parser) parseKsqlSchema(this exp.Expression) exp.Expression {
	if !p.match(tokens.L_PAREN) {
		return this
	}
	elements := p.parseCsv(p.parseKsqlColumnDef)
	if !p.match(tokens.R_PAREN) {
		return nil
	}
	if len(elements) == 0 {
		return nil
	}
	return p.expression(exp.Schema(exp.Args{"this": this, "expressions": elements}), nil, nil)
}

// parseKsqlColumnDef ports `identifier type columnConstraints?` (SqlBase.g4:146-154):
// KEY / PRIMARY KEY, HEADERS and HEADER ('x') are kept as raw column constraints so an
// unquoted `KEY` round-trips verbatim instead of canonicalizing to PRIMARY KEY.
func (p *Parser) parseKsqlColumnDef() exp.Expression {
	name := p.parseIdVar(true, nil)
	if name == nil {
		return nil
	}
	args := exp.Args{"this": name}
	if dtype := p.parseTypes(false, true, true, false); dtype != nil {
		args["kind"] = dtype
	}
	var constraints []exp.Expression
constraintsLoop:
	for {
		switch {
		case p.matchTextSeq("PRIMARY", "KEY"):
			constraints = append(constraints, p.expression(exp.PrimaryKeyColumnConstraint(exp.Args{}), nil, nil))
		case p.matchTextSeq("KEY"):
			constraints = append(constraints, p.expression(exp.ColumnConstraint(exp.Args{
				"kind": exp.Var(exp.Args{"this": "KEY"}),
			}), nil, nil))
		case p.matchTextSeq("HEADERS"):
			constraints = append(constraints, p.expression(exp.ColumnConstraint(exp.Args{
				"kind": exp.Var(exp.Args{"this": "HEADERS"}),
			}), nil, nil))
		case p.matchTextSeq("HEADER"):
			start := p.tokens[p.index-1]
			if !p.match(tokens.L_PAREN) {
				return nil
			}
			if p.parseString() == nil || !p.match(tokens.R_PAREN) {
				return nil
			}
			raw := p.findSQL(start, p.prev)
			constraints = append(constraints, p.expression(exp.ColumnConstraint(exp.Args{
				"kind": exp.Var(exp.Args{"this": raw}),
			}), nil, nil))
		default:
			break constraintsLoop
		}
	}
	if len(constraints) > 0 {
		args["constraints"] = constraints
	}
	return p.expression(exp.ColumnDef(args), nil, nil)
}

// parseKsqlTableProperties parses tableProperties (SqlBase.g4:156-162): a parenthesized
// comma list of `(identifier | STRING) EQ literal`. A STRING key keeps its quotes inside a
// Var so `WITH ('k' = 'v')` renders the same way it was written.
func (p *Parser) parseKsqlTableProperties() exp.Expression {
	if !p.match(tokens.L_PAREN) {
		return nil
	}
	var properties []exp.Expression
	for p.curr.IsValid() && p.curr.TokenType != tokens.R_PAREN {
		var key exp.Expression
		if p.curr.TokenType == tokens.STRING {
			tok := p.curr
			p.advance()
			key = exp.Var(exp.Args{"this": p.findSQL(tok, tok)})
		} else {
			key = p.parseIdVar(true, nil)
		}
		if key == nil || !p.match(tokens.EQ) {
			return nil
		}
		value := p.parsePrimary()
		if value == nil {
			return nil
		}
		properties = append(properties, p.expression(exp.Property(exp.Args{
			"this": key, "value": value,
		}), nil, nil))
		if !p.match(tokens.COMMA) {
			break
		}
	}
	if len(properties) == 0 || !p.match(tokens.R_PAREN) {
		return nil
	}
	return p.expression(exp.Properties(exp.Args{"expressions": properties}), nil, nil)
}

// ---------------------------------------------------------------------------
// INSERT (SqlBase.g4:86-87)
// ---------------------------------------------------------------------------

// parseKsqlInsert ports INSERT INTO sourceName (WITH tableProperties)? query and
// INSERT INTO sourceName (columns)? VALUES values. The column list and VALUES go through
// the base insert-table path (parseInsertTable -> parseSchema), so only the ksqlDB WITH
// clause between the target and the query needs a dedicated parse.
func (p *Parser) parseKsqlInsert() exp.Expression {
	start := p.prev
	return p.ksqlStructured(start, func() exp.Expression {
		p.match(tokens.INTO)
		this := p.parseInsertTable()
		if this == nil {
			return nil
		}
		args := exp.Args{"this": this}
		hasColumns := this.Kind() == exp.KindSchema
		if p.matchTextSeq("WITH") {
			properties := p.parseKsqlTableProperties()
			if properties == nil {
				return nil
			}
			args["properties"] = properties
		}
		hasProperties := args["properties"] != nil
		expression := firstExpression(p.parseDerivedTableValues(), p.parseDDLSelect())
		if expression == nil {
			return nil
		}
		// SqlBase.g4:86-87: the two INSERT forms are disjoint — the column list pairs only
		// with VALUES, and the WITH clause only with the query form. Fail closed on a mix so
		// the statement degrades to a raw Command instead of admitting SQL ksqlDB rejects.
		if hasColumns && expression.Kind() != exp.KindValues {
			return nil
		}
		if hasProperties && expression.Kind() == exp.KindValues {
			return nil
		}
		args["expression"] = expression
		return p.expression(exp.Insert(args), nil, nil)
	})
}

// ---------------------------------------------------------------------------
// DROP (SqlBase.g4:88-90, 93)
// ---------------------------------------------------------------------------

func (p *Parser) parseKsqlDrop() exp.Expression {
	start := p.prev
	return p.ksqlStructured(start, func() exp.Expression {
		if !p.matchTexts(map[string]bool{"STREAM": true, "TABLE": true, "CONNECTOR": true, "TYPE": true}) {
			return nil
		}
		kind := stringsUpper(p.prev.Text)
		args := exp.Args{"kind": kind}
		if exists := p.parseExists(false); exists == true {
			args["exists"] = true
		}
		var this exp.Expression
		if kind == "STREAM" || kind == "TABLE" {
			this = p.parseTableParts(true, false, false, false)
		} else {
			this = p.parseIdVar(true, nil)
		}
		if this == nil {
			return nil
		}
		args["this"] = this
		if p.matchTextSeq("DELETE", "TOPIC") {
			args["delete_topic"] = true
		}
		return p.expression(exp.Drop(args), nil, nil)
	})
}

// ---------------------------------------------------------------------------
// SHOW / LIST (SqlBase.g4:49-57, 63)
// ---------------------------------------------------------------------------

// parseKsqlShow ports the (LIST | SHOW) family. The verb and its phrase are kept together
// in Show.this (`SHOW ALL TOPICS`, `LIST SOURCE CONNECTORS`, ...) so LIST and SHOW
// round-trip distinctly; EXTENDED lands in Show.full.
func (p *Parser) parseKsqlShow() exp.Expression {
	start := p.prev
	verb := stringsUpper(p.prev.Text)
	return p.ksqlStructured(start, func() exp.Expression {
		parts := []string{verb}
		if p.match(tokens.ALL) {
			parts = append(parts, "ALL")
		}
		if p.matchTextSeq("SOURCE") || p.matchTextSeq("SINK") {
			parts = append(parts, stringsUpper(p.prev.Text))
		}
		switch {
		case p.matchTextSeq("CONNECTOR", "PLUGINS"):
			parts = append(parts, "CONNECTOR PLUGINS")
		case p.matchTexts(map[string]bool{
			"PROPERTIES": true, "TOPICS": true, "STREAMS": true, "TABLES": true,
			"FUNCTIONS": true, "CONNECTORS": true, "TYPES": true, "VARIABLES": true,
			"QUERIES": true,
		}):
			parts = append(parts, stringsUpper(p.prev.Text))
		default:
			return nil
		}
		args := exp.Args{"this": exp.Var(exp.Args{"this": strings.Join(parts, " ")})}
		if p.matchTexts(map[string]bool{"EXTENDED": true}) {
			args["full"] = true
		}
		return p.expression(exp.Show(args), nil, nil)
	})
}

// ---------------------------------------------------------------------------
// DESCRIBE / EXPLAIN (SqlBase.g4:58-61, 91)
// ---------------------------------------------------------------------------

// parseKsqlDescribe ports DESCRIBE sourceName EXTENDED?, DESCRIBE STREAMS EXTENDED?,
// DESCRIBE FUNCTION identifier and DESCRIBE CONNECTOR identifier. EXTENDED is a suffix in
// ksqlDB, so it lands in Describe.style (rendered after the target).
func (p *Parser) parseKsqlDescribe() exp.Expression {
	start := p.prev
	return p.ksqlStructured(start, func() exp.Expression {
		args := exp.Args{}
		switch {
		case p.matchTextSeq("FUNCTION"):
			args["kind"] = "FUNCTION"
		case p.matchTextSeq("CONNECTOR"):
			args["kind"] = "CONNECTOR"
		}
		this := p.parseIdVar(true, nil)
		if this == nil {
			return nil
		}
		args["this"] = this
		if args["kind"] == nil && p.matchTexts(map[string]bool{"EXTENDED": true}) {
			args["style"] = "EXTENDED"
		}
		return p.expression(exp.Describe(args), nil, nil)
	})
}

// parseKsqlExplain ports EXPLAIN (statement | identifier) (SqlBase.g4:91): a lone trailing
// token explains an identifier, anything else explains a nested statement.
func (p *Parser) parseKsqlExplain() exp.Expression {
	start := p.prev
	return p.ksqlStructured(start, func() exp.Expression {
		var inner exp.Expression
		if p.tokensSize-p.index == 1 {
			inner = p.parseIdVar(true, nil)
		} else {
			inner = p.parseStatement()
		}
		if inner == nil {
			return nil
		}
		return p.expression(exp.Describe(exp.Args{"kind": "EXPLAIN", "this": inner}), nil, nil)
	})
}

// ---------------------------------------------------------------------------
// ALTER (SqlBase.g4:94)
// ---------------------------------------------------------------------------

// parseKsqlAlter ports `ALTER (STREAM | TABLE) sourceName alterOption (',' alterOption)*`,
// where alterOption is only `ADD (COLUMN)? identifier type`. The target goes through
// parseTableParts (not parseTable): ADD is an unmapped reserved word (a VAR token), so the
// base alias pass would swallow it as the table's alias.
func (p *Parser) parseKsqlAlter() exp.Expression {
	start := p.prev
	return p.ksqlStructured(start, func() exp.Expression {
		if !p.matchTexts(map[string]bool{"STREAM": true, "TABLE": true}) {
			return nil
		}
		kind := stringsUpper(p.prev.Text)
		this := p.parseTableParts(true, false, false, false)
		if this == nil {
			return nil
		}
		var actions []exp.Expression
		for p.matchTextSeq("ADD") {
			p.match(tokens.COLUMN)
			name := p.parseIdVar(true, nil)
			dtype := p.parseTypes(false, true, true, false)
			if name == nil || dtype == nil {
				return nil
			}
			actions = append(actions, p.expression(exp.ColumnDef(exp.Args{
				"this": name, "kind": dtype,
			}), nil, nil))
			if !p.match(tokens.COMMA) {
				break
			}
		}
		if len(actions) == 0 {
			return nil
		}
		return p.expression(exp.Alter(exp.Args{
			"kind": kind, "this": this, "actions": actions,
		}), nil, nil)
	})
}

// ---------------------------------------------------------------------------
// TERMINATE / PAUSE / RESUME (SqlBase.g4:64-69)
// ---------------------------------------------------------------------------

// parseKsqlKill ports `TERMINATE|PAUSE|RESUME (identifier | ALL)` onto Kill{kind: verb}.
func (p *Parser) parseKsqlKill() exp.Expression {
	start := p.prev
	verb := stringsUpper(p.prev.Text)
	return p.ksqlStructured(start, func() exp.Expression {
		var this exp.Expression
		if p.match(tokens.ALL) {
			this = exp.Var(exp.Args{"this": "ALL"})
		} else {
			this = p.parseIdVar(true, nil)
		}
		if this == nil {
			return nil
		}
		return p.expression(exp.Kill(exp.Args{"kind": verb, "this": this}), nil, nil)
	})
}

// ---------------------------------------------------------------------------
// UNSET / DEFINE / UNDEFINE (SqlBase.g4:72-73)
// ---------------------------------------------------------------------------

// parseKsqlUnset ports `UNSET STRING` onto Set{unset}: base setSQL already renders the
// UNSET verb (generator/stmt_set.go:114-126), so no generator override is needed.
func (p *Parser) parseKsqlUnset() exp.Expression {
	start := p.prev
	return p.ksqlStructured(start, func() exp.Expression {
		key := p.parseString()
		if key == nil {
			return nil
		}
		return p.expression(exp.Set(exp.Args{"unset": true, "expressions": []exp.Expression{key}}), nil, nil)
	})
}

// parseKsqlDefine ports `DEFINE variableName EQ variableValue` onto Set{tag} with a single
// SetItem assignment; tag is the ksqlDB DEFINE/UNDEFINE discriminator (see ksqlSetSQL).
func (p *Parser) parseKsqlDefine() exp.Expression {
	start := p.prev
	return p.ksqlStructured(start, func() exp.Expression {
		name := p.parseIdVar(true, nil)
		if name == nil || !p.match(tokens.EQ) {
			return nil
		}
		value := p.parseString()
		if value == nil {
			return nil
		}
		item := p.expression(exp.SetItem(exp.Args{
			"this": p.expression(exp.EQ(exp.Args{"this": name, "expression": value}), nil, nil),
		}), nil, nil)
		return p.expression(exp.Set(exp.Args{"tag": true, "expressions": []exp.Expression{item}}), nil, nil)
	})
}

// parseKsqlUndefine ports `UNDEFINE variableName` onto Set{tag, unset} with the bare name.
func (p *Parser) parseKsqlUndefine() exp.Expression {
	start := p.prev
	return p.ksqlStructured(start, func() exp.Expression {
		name := p.parseIdVar(true, nil)
		if name == nil {
			return nil
		}
		return p.expression(exp.Set(exp.Args{
			"tag": true, "unset": true, "expressions": []exp.Expression{name},
		}), nil, nil)
	})
}

// ---------------------------------------------------------------------------
// Query modifiers: WINDOW / PARTITION BY / EMIT (SqlBase.g4:117-127)
// ---------------------------------------------------------------------------

// parseKsqlEmit ports `EMIT (CHANGES | FINAL)` into Select.emit as a raw Var. The clause is
// peeked before it is consumed: a malformed EMIT must leave its tokens in place so the
// statement fails closed rather than silently dropping the clause.
func (p *Parser) parseKsqlEmit() (string, any) {
	if p.curr.TokenType != tokens.EMIT {
		return "", nil
	}
	if !p.next.IsValid() || textMatchExcludedTokens[p.next.TokenType] || !ksqlEmitResults[stringsUpper(p.next.Text)] {
		return "", nil
	}
	p.advance() // EMIT
	p.advance() // CHANGES | FINAL
	return "emit", exp.Var(exp.Args{"this": stringsUpper(p.prev.Text)})
}

var ksqlEmitResults = map[string]bool{"CHANGES": true, "FINAL": true}

// parseKsqlPartitionBy ports `PARTITION BY valueExpression [, ...]` into Select.partition_by.
// A parenthesized list parses as one parenthesized expression, so it round-trips verbatim.
// PARTITION BY is consumed only once at least one expression parses, so a dangling clause
// retreats and the statement fails closed.
func (p *Parser) parseKsqlPartitionBy() (string, any) {
	if p.curr.TokenType != tokens.PARTITION_BY {
		return "", nil
	}
	index := p.index
	p.advance()
	exprs := p.parseCsv(p.parseValueExpression)
	if len(exprs) == 0 {
		p.retreat(index)
		return "", nil
	}
	return "partition_by", exprs
}

// parseKsqlWindowClause ports windowExpression (SqlBase.g4:184-199): an optional window
// name followed by TUMBLING / HOPPING / SESSION with SIZE, ADVANCE BY, RETENTION and
// GRACE PERIOD clauses, stored as one KindKsqlWindow in Select.windows. The RETENTION /
// GRACE PERIOD clauses (and HOPPING's ADVANCE BY) are comma-separated inside the parens.
// Any malformed body retreats to WINDOW so the statement fails closed instead of losing it.
func (p *Parser) parseKsqlWindowClause() exp.Expression {
	if p.curr.TokenType != tokens.WINDOW {
		return nil
	}
	index := p.index
	p.advance()
	window := p.parseKsqlWindowBody()
	if window == nil {
		p.retreat(index)
	}
	return window
}

func (p *Parser) parseKsqlWindowBody() exp.Expression {
	args := exp.Args{}
	// Optional window name: an identifier directly in front of the window kind.
	if (p.curr.TokenType == tokens.VAR || p.curr.TokenType == tokens.IDENTIFIER || idVarTokens[p.curr.TokenType]) &&
		p.next.IsValid() && ksqlWindowKinds[stringsUpper(p.next.Text)] {
		args["alias"] = p.parseIdVar(true, nil)
	}
	var kind string
	switch {
	case p.matchTexts(map[string]bool{"TUMBLING": true}):
		kind = "TUMBLING"
	case p.matchTexts(map[string]bool{"HOPPING": true}):
		kind = "HOPPING"
	case p.matchTexts(map[string]bool{"SESSION": true}):
		kind = "SESSION"
	default:
		p.raiseError("Expected TUMBLING, HOPPING or SESSION after WINDOW")
		return nil
	}
	args["kind"] = kind
	if !p.match(tokens.L_PAREN) {
		return nil
	}
	// TUMBLING/HOPPING spell SIZE explicitly; SESSION takes the bare duration.
	if kind != "SESSION" {
		if !p.matchTextSeq("SIZE") {
			return nil
		}
	}
	size := p.parseKsqlDuration()
	if size == nil {
		return nil
	}
	args["size"] = size
	if kind == "HOPPING" {
		if !p.match(tokens.COMMA) || !p.matchTextSeq("ADVANCE", "BY") {
			return nil
		}
		advance := p.parseKsqlDuration()
		if advance == nil {
			return nil
		}
		args["advance"] = advance
	}
	// (',' retentionClause)? (',' gracePeriodClause)? — either may lead when the other
	// is absent (SqlBase.g4:186-197), so each comma introduces the next optional clause.
	if p.match(tokens.COMMA) {
		switch {
		case p.matchTextSeq("RETENTION"):
			retention := p.parseKsqlDuration()
			if retention == nil {
				return nil
			}
			args["retention"] = retention
		case p.matchTextSeq("GRACE", "PERIOD"):
			grace := p.parseKsqlDuration()
			if grace == nil {
				return nil
			}
			args["grace"] = grace
		default:
			return nil
		}
	}
	if p.match(tokens.COMMA) {
		if !p.matchTextSeq("GRACE", "PERIOD") {
			return nil
		}
		grace := p.parseKsqlDuration()
		if grace == nil {
			return nil
		}
		args["grace"] = grace
	}
	if !p.match(tokens.R_PAREN) {
		return nil
	}
	return p.expression(exp.KsqlWindow(args), nil, nil)
}

var ksqlWindowKinds = map[string]bool{"TUMBLING": true, "HOPPING": true, "SESSION": true}

// parseKsqlDuration parses `number windowUnit` (SqlBase.g4:201-208) as a bare Interval —
// `1 HOUR` — which the ksqlDB generator renders without the INTERVAL keyword.
func (p *Parser) parseKsqlDuration() exp.Expression {
	number := p.parseNumber()
	if number == nil {
		return nil
	}
	unit := p.parseIdVar(true, nil)
	if unit == nil {
		return nil
	}
	return p.expression(exp.Interval(exp.Args{"this": number, "unit": unit}), nil, nil)
}

// ---------------------------------------------------------------------------
// Join window (SqlBase.g4:251-262)
// ---------------------------------------------------------------------------

// parseKsqlJoinWindow consumes the joinWindow clause between the joined relation and its
// ON criteria: `WITHIN joinWindowSize [gracePeriodClause]` or
// `WITHIN (before, after) [gracePeriodClause]`. Called from parseJoinBase for ksqlDB
// only; a missing/invalid duration raises so the statement fails instead of leaving the
// WITHIN tokens unconsumed.
func (p *Parser) parseKsqlJoinWindow(kwargs exp.Args) {
	if !p.match(tokens.WITHIN) {
		return
	}
	var within exp.Expression
	if p.match(tokens.L_PAREN) {
		before := p.parseKsqlDuration()
		if before == nil || !p.match(tokens.COMMA) {
			p.raiseError("Expected two join window sizes in WITHIN")
			return
		}
		after := p.parseKsqlDuration()
		if after == nil || !p.match(tokens.R_PAREN) {
			p.raiseError("Expected two join window sizes in WITHIN")
			return
		}
		within = p.expression(exp.Tuple(exp.Args{"expressions": []exp.Expression{before, after}}), nil, nil)
	} else {
		within = p.parseKsqlDuration()
		if within == nil {
			p.raiseError("Expected a join window size after WITHIN")
			return
		}
	}
	kwargs["within"] = within
	if p.matchTextSeq("GRACE", "PERIOD") {
		grace := p.parseKsqlDuration()
		if grace == nil {
			p.raiseError("Expected a duration after GRACE PERIOD")
			return
		}
		kwargs["grace"] = grace
	}
}
