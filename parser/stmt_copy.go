package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

func init() {
	statementParsers[tokens.COPY] = (*Parser).parseCopy
}

// parseCopy ports _parse_copy (parser.py:9616-9649): `COPY [INTO] <table-or-select>
// FROM|TO <files> [<credentials>] [WITH] [(<params>)]`. Any input this doesn't
// structurally recognize (e.g. dialect-specific option shapes this slice doesn't model)
// degrades to a raw Command via parseAsCommand, mirroring upstream's own fallback.
func (p *Parser) parseCopy() exp.Expression {
	start := p.prev

	p.match(tokens.INTO)

	var this exp.Expression
	if p.match(tokens.L_PAREN, false) {
		this = p.parseSelect(true, false, false)
	} else {
		this = p.parseTable(true, false, nil, false, false, false, false)
	}

	// kind = self._match(TokenType.FROM) or not self._match_text_seq("TO")
	// (parser.py:9631): only probe for TO when FROM didn't match, preserving Python's
	// short-circuit "or".
	kind := p.match(tokens.FROM)
	if !kind {
		kind = !p.matchTextSeq("TO")
	}

	files := p.parseCsv(func() exp.Expression { return p.parseField(false, nil, false) })
	if p.match(tokens.EQ, false) {
		// Backtrack one token since we've consumed the lhs of a parameter assignment here
		// (parser.py:9636-9640): this can happen for Snowflake dialect. Instead, we'd like
		// to parse the parameter list via parseCopyParameters below.
		p.retreat(p.index - 1)
		files = nil
	}

	credentials := p.parseCredentials()

	// The CREDENTIALS/ENCRYPTION/STORAGE_INTEGRATION/IAM_ROLE/REGION clauses are
	// Snowflake/Redshift-only and render faithfully only via upstream's unported
	// _parse_property grammar. None of base/mysql/postgres produce them, so when one is
	// present degrade to a raw Command (exact round-trip via parseAsCommand's source
	// recapture) rather than emit a lossy structural rewrite. This keeps every credential
	// shape consistent — cf. the multi-option case that already degrades via trailing tokens.
	if credentialsPresent(credentials) {
		return p.parseAsCommand(start)
	}

	p.matchTextSeq("WITH")

	// _parse_wrapped(self._parse_copy_parameters, optional=True) (parser.py:9646;
	// _parse_wrapped at parser.py:8635-8641): _parse_wrapped ALWAYS invokes the parse method,
	// matching the closing ")" only when an opening "(" was present. With optional=True a
	// missing "(" is not an error, so unwrapped parameter lists parse too — postgres
	// `COPY tbl FROM 'file' WITH FORMAT csv` -> `COPY tbl FROM 'file' WITH (FORMAT csv)`.
	// Go has no list-returning parseWrapped, so the wrapper is inlined here.
	wrapped := p.match(tokens.L_PAREN)
	params := p.parseCopyParameters()
	if wrapped {
		p.matchRParen(nil)
	}

	// Fallback case.
	if p.curr.IsValid() {
		return p.parseAsCommand(start)
	}

	return p.expression(exp.Copy(exp.Args{
		"this":        this,
		"kind":        kind,
		"credentials": credentials,
		"files":       files,
		"params":      params,
	}), nil, nil)
}

// parseCopyParameters ports _parse_copy_parameters (parser.py:9844-9878): the generic
// branch, the COPY_INTO_VARLEN_OPTIONS `= (...)` branch, the T-SQL FILE_FORMAT branch,
// and the `FORMAT AS AVRO|JSON` branch (parser.py:9867-9872) — the latter is NOT
// dialect-gated, so postgres reaches it too.
func (p *Parser) parseCopyParameters() []exp.Expression {
	sep := p.dialect.CopyParamsAreCsv

	var options []exp.Expression
	for p.curr.IsValid() && !p.match(tokens.R_PAREN, false) {
		startIndex := p.index
		option := p.parseVar(true, nil, false)
		prev := stringsUpper(p.prev.Text)

		// Different dialects might separate options and values by white space, "=" and "AS".
		p.match(tokens.EQ)
		matchedAlias := p.match(tokens.ALIAS)

		param := p.expression(exp.CopyParameter(exp.Args{"this": option}), nil, nil)

		if copyIntoVarlenOptions[prev] && p.match(tokens.L_PAREN, false) {
			// Snowflake FILE_FORMAT case, Databricks COPY & FORMAT options
			// (parser.py:9861-9862): the parenthesized option list is a property list.
			param.Set("expressions", p.parseWrappedOptions())
		} else if prev == "FILE_FORMAT" {
			// T-SQL's external file format case (parser.py:9863-9864).
			param.Set("expression", p.parseField(false, nil, false))
		} else if prev == "FORMAT" && matchedAlias && p.matchTexts(map[string]bool{"AVRO": true, "JSON": true}) {
			// parser.py:9867-9872: `FORMAT AS AVRO|JSON`. Fold the keyword into `this` as
			// the var "FORMAT AS <fmt>" and take the following field (if any) as the value,
			// so it round-trips as `FORMAT AS JSON` / `FORMAT AS JSON 'x'` rather than the
			// lossy `FORMAT JSON` / `FORMAT JSON, x` the generic branch would produce.
			param.Set("this", exp.Var(exp.Args{"this": "FORMAT AS " + stringsUpper(p.prev.Text)}))
			param.Set("expression", p.parseField(false, nil, false))
		} else {
			expression := p.parseUnquotedField()
			if expression == nil {
				expression = p.parseBracket(nil)
			}
			param.Set("expression", expression)
		}

		// No-progress guard: upstream's _parse_var(any_token=True) consumes any token, but
		// this port's parseVar returns nil for reserved tokens (operators, stray separators).
		// If an iteration consumed nothing, stop instead of spinning — the leftover tokens
		// reach parseCopy's trailing-token Command fallback (degrade-to-Command). This now
		// also runs on unwrapped parameter lists, so guarding termination here matters.
		if p.index == startIndex {
			break
		}

		options = append(options, param)

		if sep {
			p.match(tokens.COMMA)
		}
	}

	return options
}

// credentialsPresent reports whether parseCredentials matched any of its clauses (i.e.
// set any of Credentials' args). Used to decide whether a COPY must degrade to Command.
func credentialsPresent(e exp.Expression) bool {
	if e == nil {
		return false
	}
	for _, key := range []string{"storage", "credentials", "encryption", "iam_role", "region"} {
		if e.Arg(key) != nil {
			return true
		}
	}
	return false
}

// parseCredentials ports _parse_credentials (parser.py:9590-9611): `[STORAGE_INTEGRATION
// = <field>] [CREDENTIALS [= (<opts>)|<field>]] [ENCRYPTION (<opts>)] [IAM_ROLE
// DEFAULT|<field>] [REGION <field>]`. This port's target dialects (base/mysql/postgres)
// never exercise any of these guards (they're Snowflake/Redshift-specific), so this
// always returns an all-nil Credentials node in practice; it's still ported structurally
// so a stray CREDENTIALS/ENCRYPTION/... clause is consumed rather than falling through to
// the Command fallback.
func (p *Parser) parseCredentials() exp.Expression {
	expr := p.expression(exp.Credentials(exp.Args{}), nil, nil)

	if p.matchTextSeq("STORAGE_INTEGRATION", "=") {
		expr.Set("storage", p.parseField(false, nil, false))
	}
	if p.matchTextSeq("CREDENTIALS") {
		// Snowflake case: CREDENTIALS = (...), Redshift case: CREDENTIALS <string>.
		var credentials any
		if p.match(tokens.EQ) {
			credentials = p.parseWrappedOptions()
		} else {
			credentials = p.parseField(false, nil, false)
		}
		expr.Set("credentials", credentials)
	}
	if p.matchTextSeq("ENCRYPTION") {
		expr.Set("encryption", p.parseWrappedOptions())
	}
	if p.matchTextSeq("IAM_ROLE") {
		if p.match(tokens.DEFAULT) {
			expr.Set("iam_role", exp.Var(exp.Args{"this": p.prev.Text}))
		} else {
			expr.Set("iam_role", p.parseField(false, nil, false))
		}
	}
	if p.matchTextSeq("REGION") {
		expr.Set("region", p.parseField(false, nil, false))
	}

	return expr
}

// copyIntoVarlenOptions mirrors COPY_INTO_VARLEN_OPTIONS (parser.py:1781-1785): the
// COPY parameter keys whose value can be a parenthesized option list.
var copyIntoVarlenOptions = map[string]bool{
	"FILE_FORMAT":    true,
	"COPY_OPTIONS":   true,
	"FORMAT_OPTIONS": true,
	"CREDENTIAL":     true,
}

// parseWrappedOptions ports _parse_wrapped_options (parser.py:9823-9842): an optional
// `=` / `(` opener, then one property per iteration until the closing `)` — with
// FORMAT_NAME special-cased to _parse_format_name. It feeds the COPY_INTO_VARLEN_OPTIONS
// branch of parseCopyParameters above and Credentials' CREDENTIALS=(...) / ENCRYPTION(...)
// sub-clauses; the upstream-faithful `Unable to parse option` error on an unparseable
// option replaces the earlier scoped-down var-or-pair capture this port used before
// parseProperty existed.
func (p *Parser) parseWrappedOptions() []exp.Expression {
	p.match(tokens.EQ)
	p.match(tokens.L_PAREN)

	var opts []exp.Expression
	for p.curr.IsValid() && !p.match(tokens.R_PAREN) {
		var option exp.Expression
		if p.matchTextSeq("FORMAT_NAME", "=") {
			// The FORMAT_NAME can be set to an identifier for Snowflake and T-SQL.
			option = p.parseFormatName()
		} else {
			option = p.parseProperty()
		}
		if option == nil {
			p.raiseError("Unable to parse option")
			break
		}
		opts = append(opts, option)
	}

	return opts
}

// parseFormatName ports _parse_format_name (parser.py:10100-10107): although not
// specified in the docs, Snowflake/T-SQL accept a string or identifier for
// FORMAT_NAME = <format_name>.
func (p *Parser) parseFormatName() exp.Expression {
	value := p.parseString()
	if value == nil {
		value = p.parseTableParts(false, false, false, false)
	}
	return p.expression(exp.Property(exp.Args{
		"this":  exp.Var(exp.Args{"this": "FORMAT_NAME"}),
		"value": value,
	}), nil, nil)
}
