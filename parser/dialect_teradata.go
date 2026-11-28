package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// Teradata parser overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/parsers/teradata.py.
func init() {
	registerDialectParserOverrides("teradata", dialectParserOverrideSet{
		// parsers/teradata.py:17-20 TABLE_ALIAS_TOKENS adds ANTI and SEMI.
		AliasTokens: map[tokens.TokenType]bool{tokens.ANTI: true, tokens.SEMI: true},
		// parsers/teradata.py:76-82 STATEMENT_PARSERS: DATABASE -> Use, REPLACE -> CREATE,
		// LOCK -> LOCKING statement.
		StatementParsers: map[tokens.TokenType]parserOverrideFunc{
			tokens.DATABASE: (*Parser).parseTeradataDatabase,
			tokens.REPLACE:  (*Parser).parseCreate,
			tokens.LOCK:     (*Parser).parseTeradataLocking,
			tokens.EXECUTE:  (*Parser).parseTeradataExecute,
		},
		// parsers/teradata.py:145-153 FUNCTION_PARSERS: TRYCAST aliases the base TRY_CAST,
		// RANGE_N and TRANSLATE have dedicated grammars.
		FunctionParsers: map[string]parserOverrideFunc{
			"TRYCAST":   func(p *Parser) exp.Expression { return p.parseCast(false, true) },
			"RANGE_N":   (*Parser).parseTeradataRangeN,
			"TRANSLATE": (*Parser).parseTeradataTranslate,
		},
	})
	// parsers/teradata.py:135-140 SET_PARSERS adds QUERY_BAND — the table itself is built
	// in stmt_set.go's init, after the base SET_PARSERS/SET_TRIE it extends (this file's
	// init runs first under lexical file ordering, so building it here would copy a nil
	// map and leave `SET SESSION …` dispatching to a Command).
}

var (
	teradataSetParsers map[string]func(*Parser) exp.Expression
	teradataSetTrie    wordTrie
)

// teradataCharsetTranslators ports CHARSET_TRANSLATORS (parsers/teradata.py:22-60).
var teradataCharsetTranslators = map[string]bool{
	"GRAPHIC_TO_KANJISJIS": true, "GRAPHIC_TO_LATIN": true, "GRAPHIC_TO_UNICODE": true,
	"GRAPHIC_TO_UNICODE_PadSpace": true, "KANJI1_KanjiEBCDIC_TO_UNICODE": true,
	"KANJI1_KanjiEUC_TO_UNICODE": true, "KANJI1_KANJISJIS_TO_UNICODE": true,
	"KANJI1_SBC_TO_UNICODE": true, "KANJISJIS_TO_GRAPHIC": true, "KANJISJIS_TO_LATIN": true,
	"KANJISJIS_TO_UNICODE": true, "LATIN_TO_GRAPHIC": true, "LATIN_TO_KANJISJIS": true,
	"LATIN_TO_UNICODE": true, "LOCALE_TO_UNICODE": true, "UNICODE_TO_GRAPHIC": true,
	"UNICODE_TO_GRAPHIC_PadGraphic": true, "UNICODE_TO_GRAPHIC_VarGraphic": true,
	"UNICODE_TO_KANJI1_KanjiEBCDIC": true, "UNICODE_TO_KANJI1_KanjiEUC": true,
	"UNICODE_TO_KANJI1_KANJISJIS": true, "UNICODE_TO_KANJI1_SBC": true,
	"UNICODE_TO_KANJISJIS": true, "UNICODE_TO_LATIN": true, "UNICODE_TO_LOCALE": true,
	"UNICODE_TO_UNICODE_FoldSpace": true, "UNICODE_TO_UNICODE_Fullwidth": true,
	"UNICODE_TO_UNICODE_Halfwidth": true, "UNICODE_TO_UNICODE_NFC": true,
	"UNICODE_TO_UNICODE_NFD": true, "UNICODE_TO_UNICODE_NFKC": true,
	"UNICODE_TO_UNICODE_NFKD": true,
}

// parseTeradataDatabase ports the DATABASE statement (parsers/teradata.py:78-80): `DATABASE db`
// builds Use(this=Table).
func (p *Parser) parseTeradataDatabase() exp.Expression {
	return p.expression(exp.Use(exp.Args{
		"this": p.parseTable(false, false, nil, false, false, false, false),
	}), nil, nil)
}

// parseTeradataLocking ports _parse_locking_statement (parsers/teradata.py:84-93).
func (p *Parser) parseTeradataLocking() exp.Expression {
	locking := p.parseLocking()
	query := p.parseSelect()
	if query == nil {
		p.raiseError("Expected SELECT statement after LOCKING clause")
	}
	return p.expression(exp.LockingStatement(exp.Args{
		"this":       locking,
		"expression": query,
	}), nil, nil)
}

// parseTeradataRangeN ports _parse_rangen (parsers/teradata.py:178-186).
func (p *Parser) parseTeradataRangeN() exp.Expression {
	this := p.parseIdVar(false, nil)
	p.match(tokens.BETWEEN)
	expressions := p.parseCsv(func() exp.Expression { return p.parseAssignment() })
	var each exp.Expression
	if p.matchTextSeq("EACH") {
		each = p.parseAssignment()
	}
	return p.expression(exp.RangeN(exp.Args{
		"this":        this,
		"expressions": expressions,
		"each":        each,
	}), nil, nil)
}

// parseTeradataTranslate ports _parse_translate (parsers/teradata.py:165-176).
func (p *Parser) parseTeradataTranslate() exp.Expression {
	this := p.parseAssignment()
	p.match(tokens.USING)
	p.matchTexts(teradataCharsetTranslators)
	charset := stringsUpper(p.prev.Text)
	withError := p.matchTextSeq("WITH", "ERROR")
	return p.expression(exp.TranslateCharacters(exp.Args{
		"this":       this,
		"expression": charset,
		"with_error": withError,
	}), nil, nil)
}

// parseTeradataQueryBand ports _parse_query_band (parsers/teradata.py:188-207).
func (p *Parser) parseTeradataQueryBand() exp.Expression {
	p.match(tokens.EQ)
	var this exp.Expression
	if p.matchTextSeq("NONE") {
		this = exp.Var(exp.Args{"this": "NONE"})
	} else {
		this = p.parseString()
	}
	update := p.matchTextSeq("UPDATE")
	p.matchTextSeq("FOR")
	var scope any
	if p.matchTextSeq("SESSION", "VOLATILE") {
		scope = "SESSION VOLATILE"
	} else if p.matchTexts(map[string]bool{"SESSION": true, "TRANSACTION": true}) {
		scope = stringsUpper(p.prev.Text)
	}
	return p.expression(exp.QueryBand(exp.Args{
		"this":   this,
		"scope":  scope,
		"update": update,
	}), nil, nil)
}
