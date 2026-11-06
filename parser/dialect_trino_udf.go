package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// Inline SQL UDFs, ported from parsers/trino.py:74-268 and the shared routine node shapes in
// expressions/query.py:2126-2160. A `WITH FUNCTION ...` entry parses into a
// FunctionSpecification, collected by parseWith into With.udfs.

var (
	routineEndTerminators    = map[string]bool{"END": true}
	routineIfTerminators     = map[string]bool{"ELSEIF": true, "ELSE": true, "END": true}
	routineCaseTerminators   = map[string]bool{"WHEN": true, "ELSE": true, "END": true}
	routineRepeatTerminators = map[string]bool{"UNTIL": true}
)

// parseFunctionSpecification ports _parse_function_specification (parsers/trino.py:81-110).
func (p *Parser) parseFunctionSpecification() exp.Expression {
	this := p.parseUserDefinedFunction()

	// Collected separately (rather than one parseProperties call) so the generator can place
	// `WITH (...)` in its own bracketed clause at the end, instead of it blending into the bare
	// characteristics list.
	var characteristics, properties []exp.Expression
	for {
		if p.match(tokens.WITH) {
			properties = append(properties, p.parseWrappedCsv(func() exp.Expression {
				return p.parseKeyValueProperty(nil)
			})...)
			continue
		}

		// TrinoParser._parse_property (parsers/trino.py:70-75): NOT DETERMINISTIC is a
		// VOLATILE stability property, rendered back as NOT DETERMINISTIC.
		if p.matchTextSeq("NOT", "DETERMINISTIC") {
			characteristics = append(characteristics, p.parseStabilityProperty("VOLATILE"))
			continue
		}

		characteristic := p.parseProperty()
		if characteristic == nil {
			break
		}
		if characteristic.Kind() == exp.KindProperties {
			characteristics = append(characteristics, characteristic.Expressions()...)
		} else {
			characteristics = append(characteristics, characteristic)
		}
	}

	args := exp.Args{"this": this, "expression": p.parseRoutineStatement()}
	if len(characteristics) > 0 {
		args["characteristics"] = p.expression(exp.Properties(exp.Args{"expressions": characteristics}), nil, nil)
	}
	if len(properties) > 0 {
		args["properties"] = p.expression(exp.Properties(exp.Args{"expressions": properties}), nil, nil)
	}
	return p.expression(exp.FunctionSpecification(args), nil, nil)
}

// parseRoutineStatements ports _parse_routine_statements (parsers/trino.py:112-137). Unlike
// parseBatchStatements it stops on any terminator text even when tokens follow, because Trino's
// END is always followed by the enclosing query.
func (p *Parser) parseRoutineStatements(terminators map[string]bool) []exp.Expression {
	statements := []exp.Expression{}
	for !p.matchTexts(terminators) {
		if !p.curr.IsValid() {
			if p.chunkIndex >= len(p.chunks) {
				p.raiseError("Unexpected end of routine body")
				break
			}
			p.advanceChunk()
		} else if !p.match(tokens.SEMICOLON) {
			statement := p.parseRoutineStatement()
			if statement == nil {
				break
			}
			statements = append(statements, statement)
		}
	}
	return statements
}

// parseRoutineBlock ports _parse_routine_block (parsers/trino.py:139-144).
func (p *Parser) parseRoutineBlock() exp.Expression {
	p.match(tokens.BEGIN)
	statements := p.parseRoutineStatements(routineEndTerminators)
	statements = append(statements, exp.EndStatement(nil))
	return p.expression(exp.Block(exp.Args{"expressions": statements, "begin": true}), nil, nil)
}

// parseRoutineIf ports _parse_routine_if (parsers/trino.py:146-173). ELSEIF chains nest into
// the false slot, reusing the binary IfBlock instead of a new N-way node.
func (p *Parser) parseRoutineIf() exp.Expression {
	parseBranch := func() exp.Expression {
		condition := p.parseDisjunction()
		p.matchTextSeq("THEN")
		trueBlock := p.expression(exp.Block(exp.Args{
			"expressions": p.parseRoutineStatements(routineIfTerminators),
		}), nil, nil)
		return p.expression(exp.IfBlock(exp.Args{"this": condition, "true": trueBlock}), nil, nil)
	}

	this := parseBranch()
	tail := this
	for stringsUpper(p.prev.Text) == "ELSEIF" {
		node := parseBranch()
		tail.Set("false", node)
		tail = node
	}

	if stringsUpper(p.prev.Text) == "ELSE" {
		tail.Set("false", p.expression(exp.Block(exp.Args{
			"expressions": p.parseRoutineStatements(routineEndTerminators),
		}), nil, nil))
	}

	p.matchTextSeq("IF")
	return this
}

// parseRoutineCase ports _parse_routine_case (parsers/trino.py:175-201).
func (p *Parser) parseRoutineCase() exp.Expression {
	this := p.parseDisjunction()

	parseBranch := func() exp.Expression {
		condition := p.parseDisjunction()
		p.matchTextSeq("THEN")
		trueBlock := p.expression(exp.Block(exp.Args{
			"expressions": p.parseRoutineStatements(routineCaseTerminators),
		}), nil, nil)
		return p.expression(exp.If(exp.Args{"this": condition, "true": trueBlock}), nil, nil)
	}

	ifs := []exp.Expression{}
	p.matchTextSeq("WHEN")
	for stringsUpper(p.prev.Text) == "WHEN" {
		ifs = append(ifs, parseBranch())
	}

	var defaultBlock exp.Expression
	if stringsUpper(p.prev.Text) == "ELSE" {
		defaultBlock = p.expression(exp.Block(exp.Args{
			"expressions": p.parseRoutineStatements(routineEndTerminators),
		}), nil, nil)
	}

	p.matchTextSeq("CASE")
	return p.expression(exp.CaseStatement(exp.Args{
		"this":    this,
		"ifs":     ifs,
		"default": defaultBlock,
	}), nil, nil)
}

// parseRoutineWhile ports _parse_routine_while (parsers/trino.py:203-209).
func (p *Parser) parseRoutineWhile(label exp.Expression) exp.Expression {
	condition := p.parseDisjunction()
	p.matchTextSeq("DO")
	body := p.expression(exp.Block(exp.Args{
		"expressions": p.parseRoutineStatements(routineEndTerminators),
	}), nil, nil)
	p.matchTextSeq("WHILE")
	return p.expression(exp.WhileBlock(exp.Args{"this": condition, "body": body, "label": label}), nil, nil)
}

// parseRoutineLoop ports _parse_routine_loop (parsers/trino.py:211-216).
func (p *Parser) parseRoutineLoop(label exp.Expression) exp.Expression {
	body := p.expression(exp.Block(exp.Args{
		"expressions": p.parseRoutineStatements(routineEndTerminators),
	}), nil, nil)
	p.matchTextSeq("LOOP")
	return p.expression(exp.LoopBlock(exp.Args{"body": body, "label": label}), nil, nil)
}

// parseRoutineRepeat ports _parse_routine_repeat (parsers/trino.py:218-225). Unlike WHILE/LOOP,
// the body runs at least once and UNTIL is tested after it.
func (p *Parser) parseRoutineRepeat(label exp.Expression) exp.Expression {
	body := p.expression(exp.Block(exp.Args{
		"expressions": p.parseRoutineStatements(routineRepeatTerminators),
	}), nil, nil)
	until := p.parseDisjunction()
	p.matchTextSeq("END", "REPEAT")
	return p.expression(exp.RepeatBlock(exp.Args{"body": body, "until": until, "label": label}), nil, nil)
}

// parseRoutineStatement ports _parse_routine_statement (parsers/trino.py:227-268). An optional
// `label :` can precede WHILE, LOOP or REPEAT. Any non-reserved keyword is a valid label name,
// so the colon lookahead runs before the statement keywords are matched.
func (p *Parser) parseRoutineStatement() exp.Expression {
	var label exp.Expression
	if p.next.TokenType == tokens.COLON {
		label = p.parseIdVar(false, nil)
		p.match(tokens.COLON)
	}

	if p.curr.TokenType == tokens.BEGIN {
		return p.parseRoutineBlock()
	}

	if p.matchTextSeq("RETURN") {
		return p.expression(exp.Return(exp.Args{"this": p.parseDisjunction()}), nil, nil)
	}

	if p.matchTextSeq("IF") {
		return p.parseRoutineIf()
	}

	if p.matchTextSeq("CASE") {
		return p.parseRoutineCase()
	}

	if p.match(tokens.DECLARE) {
		return p.parseDeclareStatement()
	}

	if p.match(tokens.SET) {
		return p.parseSet()
	}

	if p.matchTextSeq("ITERATE") {
		return p.expression(exp.Iterate(exp.Args{"this": p.parseIdVar(false, nil)}), nil, nil)
	}

	if p.matchTextSeq("LEAVE") {
		return p.expression(exp.Leave(exp.Args{"this": p.parseIdVar(false, nil)}), nil, nil)
	}

	if p.matchTextSeq("WHILE") {
		return p.parseRoutineWhile(label)
	}

	if p.matchTextSeq("LOOP") {
		return p.parseRoutineLoop(label)
	}

	if p.matchTextSeq("REPEAT") {
		return p.parseRoutineRepeat(label)
	}

	p.raiseError("Expected routine statement")
	return nil
}
