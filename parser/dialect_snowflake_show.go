package parser

import (
	"strings"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

var snowflakeShowParsers = func() map[string]func(*Parser) exp.Expression {
	out := map[string]func(*Parser) exp.Expression{}
	for _, name := range []string{"DATABASES", "SCHEMAS", "OBJECTS", "TABLES", "VIEWS", "PRIMARY KEYS", "IMPORTED KEYS", "UNIQUE KEYS", "SEQUENCES", "STAGES", "COLUMNS", "USERS", "FILE FORMATS", "FUNCTIONS", "PROCEDURES", "WAREHOUSES"} {
		name := name
		out[name] = func(p *Parser) exp.Expression { return p.parseSnowflakeShow(name, false, false) }
	}
	for _, name := range []string{"DATABASES", "SCHEMAS", "OBJECTS", "TABLES", "VIEWS", "SEQUENCES", "USERS", "PRIMARY KEYS", "IMPORTED KEYS", "UNIQUE KEYS"} {
		name := name
		terse := !strings.HasSuffix(name, "KEYS")
		out["TERSE "+name] = func(p *Parser) exp.Expression { return p.parseSnowflakeShow(name, terse, false) }
	}
	out["ICEBERG TABLES"] = func(p *Parser) exp.Expression { return p.parseSnowflakeShow("TABLES", false, true) }
	out["TERSE ICEBERG TABLES"] = func(p *Parser) exp.Expression { return p.parseSnowflakeShow("TABLES", true, true) }
	return out
}()

var snowflakeShowTrie = func() wordTrie {
	keys := make([]string, 0, len(snowflakeShowParsers))
	for key := range snowflakeShowParsers {
		keys = append(keys, key)
	}
	return newTrie(keys)
}()

func (p *Parser) parseSnowflakeShow(name string, terse, iceberg bool) exp.Expression {
	args := exp.Args{"this": name, "terse": terse, "iceberg": iceberg, "history": p.matchTextSeq("HISTORY")}
	if p.match(tokens.LIKE) {
		args["like"] = p.parseString()
	}
	if p.match(tokens.IN) {
		scopeKind := ""
		switch {
		case p.matchTextSeq("ACCOUNT"):
			scopeKind = "ACCOUNT"
		case p.matchTextSeq("CLASS"):
			scopeKind = "CLASS"
		case p.matchTextSeq("APPLICATION"):
			scopeKind = "APPLICATION"
			if p.matchTextSeq("PACKAGE") {
				scopeKind += " PACKAGE"
			}
		case p.matchTextSeq("DATABASE"):
			scopeKind = "DATABASE"
		case p.matchTextSeq("SCHEMA"):
			scopeKind = "SCHEMA"
		case p.matchTextSeq("TABLE"):
			scopeKind = "TABLE"
		case p.matchTextSeq("VIEW"):
			scopeKind = "VIEW"
		default:
			scopeKind = "TABLE"
			switch name {
			case "OBJECTS", "TABLES", "VIEWS", "SEQUENCES", "UNIQUE KEYS", "IMPORTED KEYS":
				scopeKind = "SCHEMA"
			}
		}
		args["scope_kind"] = scopeKind
		if scopeKind != "ACCOUNT" && p.curr.IsValid() {
			args["scope"] = p.parseTableParts(false, false, false, false)
		}
	}
	if p.matchTextSeq("STARTS", "WITH") {
		args["starts_with"] = p.parseString()
	}
	args["limit"] = p.parseLimit(nil, false, false)
	if p.match(tokens.FROM) {
		args["from_"] = p.parseString()
	}
	if p.matchTextSeq("WITH", "PRIVILEGES") {
		args["privileges"] = p.parseCsv(func() exp.Expression { return p.parseVar(true, nil, true) })
	}
	return p.expression(exp.Show(args), nil, nil)
}
