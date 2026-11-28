package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// isHexCharacterString ports TeradataParser._is_hex_character_string (parsers/teradata.py:118-124):
// a string literal directly followed by an XC or XCV word, as in '0A'XC.
func (p *Parser) isHexCharacterString() bool {
	return p.curr.TokenType == tokens.STRING &&
		p.next.TokenType == tokens.VAR &&
		(stringsUpper(p.next.Text) == "XC" || stringsUpper(p.next.Text) == "XCV") &&
		p.curr.End+1 == p.next.Start
}

// parseTeradataHexCharacter ports TeradataParser._parse_primary's character hex branch
// (parsers/teradata.py:136-147): the digits must be hexadecimal, and the literal is a
// HexString flagged is_character that consumes both tokens.
func (p *Parser) parseTeradataHexCharacter() exp.Expression {
	token := p.curr
	for _, char := range token.Text {
		if !isHexDigit(char) {
			p.raiseError("Invalid hexadecimal character literal", token)
			break
		}
	}
	p.advance()
	expression := p.expression(exp.HexString(exp.Args{"this": token.Text, "is_character": true}), &token, nil)
	p.advance()
	return expression
}

func isHexDigit(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
