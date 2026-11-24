package parser

import (
	"strings"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// propertyKwargs carries the modifier flags _parse_property_before matches ahead of the
// property key (parser.py:2859-2869) into the PROPERTY_PARSERS entry it dispatches to.
// `local` holds the "LOCAL"/"NOT LOCAL" string upstream stores (parser.py:2864-2865); the
// rest are booleans, with upstream's `default` field named isDefault because default is a
// Go keyword.
type propertyKwargs struct {
	no        bool
	dual      bool
	before    bool
	isDefault bool
	local     string
	after     bool
	minimum   bool
	maximum   bool
}

// truthy returns only the set kwargs, mirroring upstream's truthy filter before the
// dispatch call (parser.py:2874).
func (kw propertyKwargs) truthy() map[string]bool {
	set := map[string]bool{}
	if kw.no {
		set["no"] = true
	}
	if kw.dual {
		set["dual"] = true
	}
	if kw.before {
		set["before"] = true
	}
	if kw.isDefault {
		set["default"] = true
	}
	if kw.local != "" {
		set["local"] = true
	}
	if kw.after {
		set["after"] = true
	}
	if kw.minimum {
		set["minimum"] = true
	}
	if kw.maximum {
		set["maximum"] = true
	}
	return set
}

// propertyBeforeKwargAccepts maps each PROPERTY_PARSERS entry whose upstream lambda takes
// **kwargs (parser.py:1301-1413) to the _parse_property_before kwargs it can forward.
// Every other entry takes no kwargs, so a truthy kwarg there makes upstream's dispatch
// call raise TypeError, which _parse_property_before turns into ParseError
// (parser.py:2874-2876); the Go port records raiseError instead and returns nil so the
// enclosing CREATE fails closed through parseCreateStructured's tail guard. No dialect
// overlay in this port redefines any of these seven keys (parser/dialect_*.go), so the
// table is authoritative for every dialect.
var propertyBeforeKwargAccepts = map[string]map[string]bool{
	"CHARSET":         {"default": true},
	"COLLATE":         {"default": true},
	"DATABLOCKSIZE":   {"default": true, "minimum": true, "maximum": true},
	"FALLBACK":        {"no": true},
	"JOURNAL":         {"no": true, "dual": true, "before": true, "local": true, "after": true},
	"LOG":             {"no": true},
	"MERGEBLOCKRATIO": {"no": true, "default": true},
}

// propertyBeforeParsers holds the five **kwargs PROPERTY_PARSERS entries that build their
// node from the _parse_property_before kwargs (parser.py:2871-2876). The non-before path
// reaches the same entries through the base propertyParsers wrappers in parser_ddl.go,
// which pass at most the DEFAULT flag as a zero-or-default kwargs value.
var propertyBeforeParsers = map[string]func(*Parser, propertyKwargs) exp.Expression{
	"DATABLOCKSIZE":   (*Parser).parseDatablocksize,
	"FALLBACK":        (*Parser).parseFallback,
	"JOURNAL":         (*Parser).parseJournal,
	"LOG":             (*Parser).parseLog,
	"MERGEBLOCKRATIO": (*Parser).parseMergeBlockRatio,
}

// parsePropertyBefore ports _parse_property_before (parser.py:2855-2881), reached from
// CREATE's POST_NAME position (parser.py:2641-2642) for every dialect. Like upstream it
// never retreats: the comma and every matched modifier stay consumed even when no property
// key follows, so a miss falls through to CREATE's tail guard (parser.py:2719-2721)
// instead of rewinding into the schema.
func (p *Parser) parsePropertyBefore() exp.Expression {
	p.match(tokens.COMMA)

	kw := propertyKwargs{
		no:        p.matchTextSeq("NO"),
		dual:      p.matchTextSeq("DUAL"),
		before:    p.matchTextSeq("BEFORE"),
		isDefault: p.matchTextSeq("DEFAULT"),
	}
	switch {
	case p.matchTextSeq("LOCAL"):
		kw.local = "LOCAL"
	case p.matchTextSeq("NOT", "LOCAL"):
		kw.local = "NOT LOCAL"
	}
	kw.after = p.matchTextSeq("AFTER")
	kw.minimum = p.matchTexts(map[string]bool{"MIN": true, "MINIMUM": true})
	kw.maximum = p.matchTexts(map[string]bool{"MAX": true, "MAXIMUM": true})

	if p.matchTexts(p.propertyParserKeys()) {
		key := stringsUpper(p.prev.Text)
		truthy := kw.truthy()
		if accepts, ok := propertyBeforeKwargAccepts[key]; ok {
			for name := range truthy {
				if !accepts[name] {
					p.raiseError("Cannot parse property '" + p.prev.Text + "'")
					return nil
				}
			}
		} else if len(truthy) > 0 {
			p.raiseError("Cannot parse property '" + p.prev.Text + "'")
			return nil
		}
		if parse := propertyBeforeParsers[key]; parse != nil {
			return parse(p, kw)
		}
		if fn := p.propertyParser(key); fn != nil {
			return fn(p, kw.isDefault)
		}
		return nil
	}
	// "CHARACTER SET" is two tokens since v30.17 (parser.py:2878-2880).
	if p.matchTextSeq("CHARACTER", "SET") {
		return p.parseCharacterSet(kw.isDefault)
	}
	return nil
}

// parseFallback ports _parse_fallback (parser.py:2994-2997).
func (p *Parser) parseFallback(kw propertyKwargs) exp.Expression {
	args := exp.Args{"no": kw.no}
	if p.matchTextSeq("PROTECTION") {
		args["protection"] = true
	}
	return p.expression(exp.FallbackProperty(args), nil, nil)
}

// parseJournal ports _parse_journal (parser.py:3174-3175): upstream builds the node from
// the truthy _parse_property_before kwargs only (parser.py:2874), so unset modifiers stay
// absent from args.
func (p *Parser) parseJournal(kw propertyKwargs) exp.Expression {
	args := exp.Args{}
	if kw.no {
		args["no"] = true
	}
	if kw.dual {
		args["dual"] = true
	}
	if kw.before {
		args["before"] = true
	}
	if kw.local != "" {
		args["local"] = kw.local
	}
	if kw.after {
		args["after"] = true
	}
	return p.expression(exp.JournalProperty(args), nil, nil)
}

// parseLog ports _parse_log (parser.py:3177-3178); `no` is a required arg (properties.py
// LogProperty arg_types), so it is always set.
func (p *Parser) parseLog(kw propertyKwargs) exp.Expression {
	return p.expression(exp.LogProperty(exp.Args{"no": kw.no}), nil, nil)
}

// parseChecksum ports _parse_checksum (parser.py:3180-3190): ON/OFF record the `on` flag
// (False for the explicit OFF form, absent for the bare `CHECKSUM =` upstream leaves as
// None), and a trailing DEFAULT wins over both.
func (p *Parser) parseChecksum(_ propertyKwargs) exp.Expression {
	p.match(tokens.EQ)
	args := exp.Args{}
	if p.match(tokens.ON) {
		args["on"] = true
	} else if p.matchTextSeq("OFF") {
		args["on"] = false
	}
	if p.match(tokens.DEFAULT) {
		args["default"] = true
	}
	return p.expression(exp.ChecksumProperty(args), nil, nil)
}

// parseFreespace ports _parse_freespace (parser.py:3226-3231); `this` is required.
func (p *Parser) parseFreespace(_ propertyKwargs) exp.Expression {
	p.match(tokens.EQ)
	args := exp.Args{"this": p.parseNumber()}
	if p.match(tokens.PERCENT) {
		args["percent"] = true
	}
	return p.expression(exp.FreespaceProperty(args), nil, nil)
}

// parseDatablocksize ports _parse_datablocksize (parser.py:3234-3253); the size/units half
// is the `= <number> [BYTES|KBYTES|KILOBYTES]` form, the kwarg half is the leading
// DEFAULT/MINIMUM/MAXIMUM modifier.
func (p *Parser) parseDatablocksize(kw propertyKwargs) exp.Expression {
	p.match(tokens.EQ)
	args := exp.Args{}
	if size := p.parseNumber(); size != nil {
		args["size"] = size
	}
	if p.matchTexts(map[string]bool{"BYTES": true, "KBYTES": true, "KILOBYTES": true}) {
		args["units"] = p.prev.Text
	}
	if kw.isDefault {
		args["default"] = true
	}
	if kw.minimum {
		args["minimum"] = true
	}
	if kw.maximum {
		args["maximum"] = true
	}
	return p.expression(exp.DataBlocksizeProperty(args), nil, nil)
}

// parseBlockCompression ports _parse_blockcompression (parser.py:3255-3274).
func (p *Parser) parseBlockCompression(_ propertyKwargs) exp.Expression {
	p.match(tokens.EQ)
	args := exp.Args{}
	if p.matchTextSeq("ALWAYS") {
		args["always"] = true
	}
	if p.matchTextSeq("MANUAL") {
		args["manual"] = true
	}
	if p.matchTextSeq("NEVER") {
		args["never"] = true
	}
	if p.matchTextSeq("DEFAULT") {
		args["default"] = true
	}
	if p.matchTextSeq("AUTOTEMP") {
		// _parse_schema with no `this` (parser.py:7477-7489): the AUTOTEMP argument list
		// is parenthesized, or the arg stays unset.
		if schema := p.parseSchema(nil); schema != nil {
			args["autotemp"] = schema
		}
	}
	return p.expression(exp.BlockCompressionProperty(args), nil, nil)
}

// parseMergeBlockRatio ports _parse_mergeblockratio (parser.py:3214-3225): the `= <n>
// [PERCENT]` form carries the value, otherwise the NO/DEFAULT modifier lands on the node.
func (p *Parser) parseMergeBlockRatio(kw propertyKwargs) exp.Expression {
	args := exp.Args{}
	if p.match(tokens.EQ) {
		if this := p.parseNumber(); this != nil {
			args["this"] = this
		}
		if p.match(tokens.PERCENT) {
			args["percent"] = true
		}
		return p.expression(exp.MergeBlockRatioProperty(args), nil, nil)
	}
	if kw.no {
		args["no"] = true
	}
	if kw.isDefault {
		args["default"] = true
	}
	return p.expression(exp.MergeBlockRatioProperty(args), nil, nil)
}

// parseWithJournalTable ports _parse_withjournaltable (parser.py:3170-3173).
func (p *Parser) parseWithJournalTable() exp.Expression {
	p.match(tokens.TABLE)
	p.match(tokens.EQ)
	return p.expression(exp.WithJournalTableProperty(exp.Args{
		"this": p.parseTableParts(false, false, false, false),
	}), nil, nil)
}

// parseWithIsolatedLoading ports _parse_withisolatedloading (parser.py:3276-3290): the
// optional NO/CONCURRENT modifiers are only committed once ISOLATED LOADING matches, so a
// `WITH <other>` continuation retreats to the position it started from.
func (p *Parser) parseWithIsolatedLoading() exp.Expression {
	index := p.index
	no := p.matchTextSeq("NO")
	concurrent := p.matchTextSeq("CONCURRENT")
	if !p.matchTextSeq("ISOLATED", "LOADING") {
		p.retreat(index)
		return nil
	}
	args := exp.Args{}
	if no {
		args["no"] = true
	}
	if concurrent {
		args["concurrent"] = true
	}
	if target := p.parseVarFromOptions(isolatedLoadingOptions, false); target != nil {
		args["target"] = target
	}
	return p.expression(exp.IsolatedLoadingProperty(args), nil, nil)
}

// isolatedLoadingOptions mirrors ISOLATED_LOADING_OPTIONS (parser.py:1694): upstream's
// flat value tuple is equivalent to this port's one-word continuation sequences.
var isolatedLoadingOptions = optionsType{
	"FOR": {{"ALL"}, {"INSERT"}, {"NONE"}},
}

// parseVolatileProperty ports _parse_volatile_property (parser.py:3018-3027): only
// CREATE/REPLACE/UNIQUE directly before VOLATILE (PRE_VOLATILE_TOKENS, parser.py:1641)
// yield the Teradata VolatileProperty; anything else is the stability modifier. The token
// two positions back is the one before the matched VOLATILE (parser.py:3020-3022); a
// chunk that starts at VOLATILE has no predecessor, which cannot occur behind a CREATE
// keyword, so the guard takes the stability form instead of upstream's _tokens[-1] wrap.
func (p *Parser) parseVolatileProperty() exp.Expression {
	if p.index >= 2 {
		switch p.tokens[p.index-2].TokenType {
		case tokens.CREATE, tokens.REPLACE, tokens.UNIQUE:
			return p.expression(exp.VolatileProperty(nil), nil, nil)
		}
	}
	return p.expression(exp.StabilityProperty(exp.Args{"this": exp.LiteralString("VOLATILE")}), nil, nil)
}

// parseWrappedProperties ports _parse_wrapped_properties (parser.py:2792-2793), flattening
// temporary Properties wrappers used by list-returning registry entries.
func (p *Parser) parseWrappedProperties() []exp.Expression {
	parsed := p.parseWrappedCsv(p.parseProperty)
	properties := make([]exp.Expression, 0, len(parsed))
	for _, property := range parsed {
		if property != nil && property.Kind() == exp.KindProperties {
			properties = append(properties, property.Expressions()...)
		} else if property != nil {
			properties = append(properties, property)
		}
	}
	return properties
}

// parseProperty ports _parse_property (parser.py:2795-2815) for the registered property
// subset, followed by the generic key=value fallback. Unsupported registry entries are not
// registered, so they consume nothing and the enclosing CREATE degrades to Command.
func (p *Parser) parseProperty() exp.Expression {
	if p.dialectIs("snowflake") && p.matchTextSeq("FILE_FORMAT") {
		p.match(tokens.EQ)
		var options []exp.Expression
		if p.match(tokens.L_PAREN, false) {
			options = p.parseWrappedOptions()
		} else {
			options = []exp.Expression{p.parseFormatName()}
		}
		return p.expression(exp.FileFormatProperty(exp.Args{"expressions": options}), nil, nil)
	}
	// parsers/bigquery.py:543-548: NOT DETERMINISTIC becomes a VOLATILE stability property.
	if p.dialect.Name == "bigquery" && p.matchTextSeq("NOT", "DETERMINISTIC") {
		return p.expression(exp.StabilityProperty(exp.Args{"this": exp.LiteralString("VOLATILE")}), nil, nil)
	}
	if p.matchTexts(p.propertyParserKeys()) {
		return p.propertyParser(stringsUpper(p.prev.Text))(p, false)
	}
	// "CHARACTER SET" is two tokens since v30.17 (parser.py:2890-2898).
	if p.matchTextSeq("CHARACTER", "SET") {
		return p.parseCharacterSet(false)
	}
	if p.match(tokens.DEFAULT) {
		if p.matchTexts(p.propertyParserKeys()) {
			return p.propertyParser(stringsUpper(p.prev.Text))(p, true)
		}
		if p.matchTextSeq("CHARACTER", "SET") {
			return p.parseCharacterSet(true)
		}
	}
	// A MySQL compound-statement leader here is a routine BODY starting (grammar
	// extension, mysql_compound.go) — never a key=value property; without this stop the
	// key parse would route `IF 1 THEN ...` into the IF() function parser and error.
	// CASE has its own token type; a bare CASE statement body stops the same way.
	if p.dialectIs("mysql") &&
		((p.curr.TokenType == tokens.VAR && compoundWords[stringsUpper(p.curr.Text)]) ||
			p.curr.TokenType == tokens.CASE) {
		return nil
	}

	// _parse_property (parser.py:2907-2912): the sequence-properties grammar is attempted
	// before the generic key=value fallback; on a miss the position is restored so
	// parseKeyValueProperty starts from the same token.
	index := p.index
	if seqProps := p.parseSequenceProperties(); seqProps != nil {
		return seqProps
	}
	p.retreat(index)
	return p.parseKeyValueProperty(nil)
}

// createSequence mirrors the CREATE_SEQUENCE optionsType (parser.py:1670-1688): the
// value lists are the optional continuation keywords per leading option, and an empty
// list (dict.fromkeys(..., tuple())) means the option stands alone.
var createSequence = optionsType{
	"SCALE":      {{"EXTEND"}, {"NOEXTEND"}},
	"SHARD":      {{"EXTEND"}, {"NOEXTEND"}},
	"NO":         {{"CYCLE"}, {"CACHE"}, {"MAXVALUE"}, {"MINVALUE"}},
	"SESSION":    nil,
	"GLOBAL":     nil,
	"KEEP":       nil,
	"NOKEEP":     nil,
	"ORDER":      nil,
	"NOORDER":    nil,
	"NOCACHE":    nil,
	"CYCLE":      nil,
	"NOCYCLE":    nil,
	"NOMINVALUE": nil,
	"NOMAXVALUE": nil,
	"NOSCALE":    nil,
	"NOSHARD":    nil,
}

// parseSequenceProperties ports _parse_sequence_properties (parser.py:2733-2766): a
// comma-separated run of sequence options (INCREMENT [BY [=]] <term>, MINVALUE,
// MAXVALUE, START [WITH [=]] <term>, CACHE [<num>], OWNED BY <col|NONE>) plus any bare
// CREATE_SEQUENCE option captured as a Var. Like upstream, it returns nil when nothing
// net was consumed — which is what makes the attempt safe to embed in parseProperty.
// The comma match lives INSIDE the loop, so a consumed comma with no matching sequence
// key yields a non-nil (empty) SequenceProperties: upstream relies on that to split
// e.g. T-SQL COPY's `CREDENTIAL = (IDENTITY='x', SECRET='y')` properties, and the
// generator renders the empty node as "" (flat expressions() skips it).
func (p *Parser) parseSequenceProperties() exp.Expression {
	seq := exp.SequenceProperties(exp.Args{})

	var options []exp.Expression
	index := p.index

loop:
	for p.curr.IsValid() {
		p.match(tokens.COMMA)
		switch {
		case p.matchTextSeq("INCREMENT"):
			p.matchTextSeq("BY")
			p.matchTextSeq("=")
			seq.Set("increment", p.parseTerm())
		case p.matchTextSeq("MINVALUE"):
			seq.Set("minvalue", p.parseTerm())
		case p.matchTextSeq("MAXVALUE"):
			seq.Set("maxvalue", p.parseTerm())
		case p.matchTextSeq("START"):
			p.matchTextSeq("WITH")
			p.matchTextSeq("=")
			seq.Set("start", p.parseTerm())
		case p.matchTextSeq("CACHE"):
			// T-SQL allows an empty CACHE which is initialized dynamically.
			if num := p.parseNumber(); num != nil {
				seq.Set("cache", num)
			} else {
				seq.Set("cache", true)
			}
		case p.matchTextSeq("OWNED", "BY"):
			// "OWNED BY NONE" is the default.
			if p.matchTextSeq("NONE") {
				seq.Set("owned", nil)
			} else {
				seq.Set("owned", p.parseColumn())
			}
		default:
			opt := p.parseVarFromOptions(createSequence, false)
			if opt == nil {
				break loop
			}
			options = append(options, opt)
		}
	}

	// Upstream sets "options" to `options or None` — an empty list must stay nil so
	// sequenceproperties_sql renders no options clause.
	if len(options) > 0 {
		seq.Set("options", options)
	}

	if p.index == index {
		return nil
	}
	return seq
}

// parseKeyValueProperty ports _parse_key_value_property (parser.py:2817-2841), including
// Column-to-Dot/Var normalization for keys and Column-to-Var normalization for values.
func (p *Parser) parseKeyValueProperty(parseValue func() exp.Expression) exp.Expression {
	index := p.index
	key := p.parseColumn()
	if !p.match(tokens.EQ) {
		p.retreat(index)
		return nil
	}

	if key != nil && key.Kind() == exp.KindColumn {
		if len(key.Parts()) > 1 {
			key = exp.ColumnsToDot(key)
		} else {
			key = exp.Var(exp.Args{"this": key.Name()})
		}
	}

	var value exp.Expression
	if parseValue != nil {
		value = parseValue()
	} else {
		value = p.parseBitwise()
		if value == nil {
			value = p.parseVar(true, nil, false)
		}
	}
	if value != nil && value.Kind() == exp.KindColumn {
		value = exp.Var(exp.Args{"this": value.Name()})
	}

	return p.expression(exp.Property(exp.Args{"this": key, "value": value}), nil, nil)
}

// parseStored ports _parse_stored (parser.py:2843-2864).
func (p *Parser) parseStored() exp.Expression {
	if p.matchTextSeq("BY") {
		return p.expression(exp.StorageHandlerProperty(exp.Args{"this": p.parseVarOrString(false)}), nil, nil)
	}

	p.match(tokens.ALIAS)
	var inputFormat exp.Expression
	if p.matchTextSeq("INPUTFORMAT") {
		inputFormat = p.parseString()
	}
	var outputFormat exp.Expression
	if p.matchTextSeq("OUTPUTFORMAT") {
		outputFormat = p.parseString()
	}

	var this exp.Expression
	if inputFormat != nil || outputFormat != nil {
		this = p.expression(exp.InputOutputFormat(exp.Args{
			"input_format":  inputFormat,
			"output_format": outputFormat,
		}), nil, nil)
	} else {
		this = p.parseVarOrString(false)
		if this == nil {
			this = p.parseNumber()
		}
		if this == nil {
			this = p.parseIdVar(true, nil)
		}
	}

	return p.expression(exp.FileFormatProperty(exp.Args{"this": this, "hive_format": true}), nil, nil)
}

// parseClusteredBy ports _parse_clustered_by (parser.py:3108-3128).
func (p *Parser) parseClusteredBy() exp.Expression {
	p.matchTextSeq("BY")

	p.matchLParen(nil)
	expressions := p.parseCsv(p.parseColumn)
	p.matchRParen(nil)

	var sortedBy []exp.Expression
	if p.matchTextSeq("SORTED", "BY") {
		p.matchLParen(nil)
		sortedBy = p.parseCsv(func() exp.Expression { return p.parseOrdered(nil) })
		p.matchRParen(nil)
	}

	p.match(tokens.INTO)
	buckets := p.parseNumber()
	p.matchTextSeq("BUCKETS")

	return p.expression(exp.ClusteredByProperty(exp.Args{
		"expressions": expressions,
		"sorted_by":   sortedBy,
		"buckets":     buckets,
	}), nil, nil)
}

// parseDefiner ports _parse_definer (parser.py:3058-3069): DEFINER[=]user@host.
func (p *Parser) parseDefiner() exp.Expression {
	if p.dialect.Name == "clickhouse" {
		// parsers/clickhouse.py:925-929: DEFINER [=] {CURRENT_USER | <string>}.
		p.match(tokens.EQ)
		if p.match(tokens.CURRENT_USER) {
			return exp.DefinerProperty(exp.Args{"this": exp.Var(exp.Args{"this": stringsUpper(p.prev.Text)})})
		}
		return exp.DefinerProperty(exp.Args{"this": p.parseString()})
	}
	p.match(tokens.EQ)
	user := p.parseIdVar(true, nil)
	p.match(tokens.PARAMETER)
	host := p.parseIdVar(true, nil)

	var hostText string
	if host != nil {
		hostText = propertyIdentifierText(host)
	} else if p.match(tokens.MOD) {
		hostText = p.prev.Text
	}
	if user == nil || hostText == "" {
		return nil
	}
	return exp.DefinerProperty(exp.Args{"this": propertyIdentifierText(user) + "@" + hostText})
}

func propertyIdentifierText(identifier exp.Expression) string {
	name := identifier.Name()
	if quoted, _ := identifier.Arg("quoted").(bool); quoted {
		return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
	}
	return name
}

// parseWithProperty ports the worklist-relevant branches of _parse_with_property
// (parser.py:3002-3043): WITH (...), WITH DATA, and WITH NO DATA. The wrapped branch uses a
// temporary Properties node because propertyParserFunc returns one expression.
func (p *Parser) parseWithProperty() exp.Expression {
	// parser.py:3100-3104: WITH (SYSTEM_VERSIONING ...).
	if p.dialect.Name == "tsql" && p.matchTextSeq("(", "SYSTEM_VERSIONING") {
		prop := p.parseSystemVersioningProperty(true)
		p.matchRParen(prop)
		return prop
	}
	if p.match(tokens.L_PAREN, false) {
		return p.expression(exp.Properties(exp.Args{"expressions": p.parseWrappedProperties()}), nil, nil)
	}
	// parser.py:3112-3113: WITH JOURNAL TABLE=<name> (Teradata) sits ahead of the DATA
	// branches; the VIEW_ATTRIBUTES / SCHEMA / PROCEDURE_OPTIONS branches upstream has
	// between them (parser.py:3115-3133) are not part of this port's registry subset and
	// fall through to the isolated-loading attempt below exactly as they would return
	// nothing there.
	if p.matchTextSeq("JOURNAL") {
		return p.parseWithJournalTable()
	}
	if p.matchTextSeq("DATA") {
		return p.parseWithData(false)
	}
	if p.matchTextSeq("NO", "DATA") {
		return p.parseWithData(true)
	}
	if p.match(tokens.SERDE_PROPERTIES, false) {
		return p.parseSerdeProperties(true)
	}
	// parser.py:3135-3138: only a trailing WITH (no next token) stops before the
	// ISOLATED LOADING attempt; otherwise a non-matching tail retreats inside
	// parseWithIsolatedLoading and the caller sees nil.
	if !p.next.IsValid() {
		return nil
	}
	return p.parseWithIsolatedLoading()
}

// parseLocking ports _parse_locking (parser.py:3205-3250).
func (p *Parser) parseLocking() exp.Expression {
	var kind string
	switch {
	case p.match(tokens.TABLE):
		kind = "TABLE"
	case p.match(tokens.VIEW):
		kind = "VIEW"
	case p.match(tokens.ROW):
		kind = "ROW"
	case p.matchTextSeq("DATABASE"):
		kind = "DATABASE"
	}

	var this exp.Expression
	if kind == "DATABASE" || kind == "TABLE" || kind == "VIEW" {
		this = p.parseTableParts(false, false, false, false)
	}

	var forOrIn string
	if p.match(tokens.FOR) {
		forOrIn = "FOR"
	} else if p.match(tokens.IN) {
		forOrIn = "IN"
	}

	var lockType string
	switch {
	case p.matchTextSeq("ACCESS"):
		lockType = "ACCESS"
	case p.matchTexts(map[string]bool{"EXCL": true, "EXCLUSIVE": true}):
		lockType = "EXCLUSIVE"
	case p.matchTextSeq("SHARE"):
		lockType = "SHARE"
	case p.matchTextSeq("READ"):
		lockType = "READ"
	case p.matchTextSeq("WRITE"):
		lockType = "WRITE"
	case p.matchTextSeq("CHECKSUM"):
		lockType = "CHECKSUM"
	}

	return p.expression(exp.LockingProperty(exp.Args{
		"this": this, "kind": kind, "for_or_in": forOrIn,
		"lock_type": lockType, "override": p.matchTextSeq("OVERRIDE"),
	}), nil, nil)
}

// parsePartitionBoundSpec ports _parse_partition_bound_spec (parser.py:3257-3291).
func (p *Parser) parsePartitionBoundSpec() exp.Expression {
	parseBound := func() exp.Expression {
		if p.matchTextSeq("MINVALUE") {
			return exp.Var(exp.Args{"this": "MINVALUE"})
		}
		if p.matchTextSeq("MAXVALUE") {
			return exp.Var(exp.Args{"this": "MAXVALUE"})
		}
		return p.parseBitwise()
	}

	args := exp.Args{}
	switch {
	case p.match(tokens.IN):
		args["this"] = p.parseWrappedCsv(p.parseBitwise)
	case p.match(tokens.FROM):
		args["from_expressions"] = p.parseWrappedCsv(parseBound)
		p.matchTextSeq("TO")
		args["to_expressions"] = p.parseWrappedCsv(parseBound)
	case p.matchTextSeq("WITH", "(", "MODULUS"):
		args["this"] = p.parseNumber()
		p.matchTextSeq(",", "REMAINDER")
		args["expression"] = p.parseNumber()
		p.matchRParen(nil)
	default:
		p.raiseError("Failed to parse partition bound spec.")
	}
	return p.expression(exp.PartitionBoundSpec(args), nil, nil)
}

// parsePartitionedOf ports _parse_partitioned_of (parser.py:3293-3308).
func (p *Parser) parsePartitionedOf() exp.Expression {
	if !p.matchTextSeq("OF") {
		p.retreat(p.index - 1)
		return nil
	}
	this := p.parseTable(true, false, nil, false, false, false, false)

	var expression exp.Expression
	if p.match(tokens.DEFAULT) {
		expression = exp.Var(exp.Args{"this": "DEFAULT"})
	} else if p.matchTextSeq("FOR", "VALUES") {
		expression = p.parsePartitionBoundSpec()
	} else {
		p.raiseError("Expecting either DEFAULT or FOR VALUES clause.")
	}
	return p.expression(exp.PartitionedOfProperty(exp.Args{"this": this, "expression": expression}), nil, nil)
}

// parsePartitionedBy ports _parse_partitioned_by (parser.py:3310-3316).
func (p *Parser) parsePartitionedBy() exp.Expression {
	// parsers/starrocks.py:97-104: the key is a Schema of assignments.
	if p.dialectIs("starrocks") {
		return p.parseStarRocksPartitionedBy()
	}
	// parsers/clickhouse.py:989-992: a custom expression partition key.
	if p.dialect.Name == "clickhouse" {
		return p.expression(exp.PartitionedByProperty(exp.Args{"this": p.parseAssignment()}), nil, nil)
	}
	p.match(tokens.EQ)
	this := p.parseSchema(nil)
	if this == nil {
		this = p.parseBracket(p.parseField(false, nil, false))
	}
	return p.expression(exp.PartitionedByProperty(exp.Args{"this": this}), nil, nil)
}

// parseWithData ports _parse_withdata (parser.py:3318-3326).
func (p *Parser) parseWithData(no bool) exp.Expression {
	var statistics any
	if p.matchTextSeq("AND", "STATISTICS") {
		statistics = true
	} else if p.matchTextSeq("AND", "NO", "STATISTICS") {
		statistics = false
	}
	return p.expression(exp.WithDataProperty(exp.Args{"no": no, "statistics": statistics}), nil, nil)
}

func (p *Parser) parseContainsProperty() exp.Expression {
	if p.matchTextSeq("SQL") {
		return p.expression(exp.SqlReadWriteProperty(exp.Args{"this": "CONTAINS SQL"}), nil, nil)
	}
	return nil
}

func (p *Parser) parseModifiesProperty() exp.Expression {
	if p.matchTextSeq("SQL", "DATA") {
		return p.expression(exp.SqlReadWriteProperty(exp.Args{"this": "MODIFIES SQL DATA"}), nil, nil)
	}
	return nil
}

func (p *Parser) parseNoProperty() exp.Expression {
	if p.matchTextSeq("PRIMARY", "INDEX") {
		return exp.NoPrimaryIndexProperty(nil)
	}
	if p.matchTextSeq("SQL") {
		return p.expression(exp.SqlReadWriteProperty(exp.Args{"this": "NO SQL"}), nil, nil)
	}
	return nil
}

// parseOnProperty ports _parse_on_property (parser.py:3443-3448): ON COMMIT ... ROWS, else
// the generic OnProperty over `ON <id> [<schema>]` (e.g. T-SQL filetable `ON b (c)`).
func (p *Parser) parseOnProperty() exp.Expression {
	// parsers/clickhouse.py:809-817: `ON CLUSTER <name>`.
	if p.dialect.Name == "clickhouse" {
		index := p.index
		if p.matchTextSeq("CLUSTER") {
			this := p.parseString()
			if this == nil {
				this = p.parseIdVar(false, nil)
			}
			if this != nil {
				return p.expression(exp.OnCluster(exp.Args{"this": this}), nil, nil)
			}
			p.retreat(index)
		}
	}
	if p.matchTextSeq("COMMIT", "PRESERVE", "ROWS") {
		return exp.OnCommitProperty(nil)
	}
	if p.matchTextSeq("COMMIT", "DELETE", "ROWS") {
		return exp.OnCommitProperty(exp.Args{"delete": true})
	}
	return exp.OnProperty(exp.Args{"this": p.parseSchema(p.parseIdVar(false, nil))})
}

// parseSystemVersioningProperty ports _parse_system_versioning_property
// (parser.py:3036-3058). The SYSTEM_VERSIONING keyword is already consumed.
func (p *Parser) parseSystemVersioningProperty(with bool) exp.Expression {
	p.match(tokens.EQ)
	prop := exp.WithSystemVersioningProperty(exp.Args{"on": true, "with_": with})
	if p.matchTextSeq("OFF") {
		prop.Set("on", false)
		return p.expression(prop, nil, nil)
	}
	p.match(tokens.ON)
	if p.match(tokens.L_PAREN) {
		for p.curr.IsValid() && !p.match(tokens.R_PAREN) {
			switch {
			case p.matchTextSeq("HISTORY_TABLE", "="):
				prop.Set("this", p.parseTableParts(false, false, false, false))
			case p.matchTextSeq("DATA_CONSISTENCY_CHECK", "="):
				if tok := p.advanceAny(true); tok != nil {
					prop.Set("data_consistency", stringsUpper(tok.Text))
				}
			case p.matchTextSeq("HISTORY_RETENTION_PERIOD", "="):
				prop.Set("retention_period", p.parseRetentionPeriod())
			}
			p.match(tokens.COMMA)
		}
	}
	return p.expression(prop, nil, nil)
}

// parseRetentionPeriod ports _parse_retention_period (parser.py:3029-3034).
func (p *Parser) parseRetentionPeriod() exp.Expression {
	number := p.parseNumber()
	prefix := ""
	if number != nil {
		prefix = number.Name() + " "
	}
	unit := p.parseVar(true, nil, false)
	if unit == nil {
		return exp.Var(exp.Args{"this": prefix})
	}
	return exp.Var(exp.Args{"this": prefix + unit.Name()})
}

// parseDataDeletionProperty ports _parse_data_deletion_property (parser.py:3060-3074).
func (p *Parser) parseDataDeletionProperty() exp.Expression {
	p.match(tokens.EQ)
	on := p.matchTextSeq("ON") || !p.matchTextSeq("OFF")
	prop := exp.DataDeletionProperty(exp.Args{"on": on})
	if p.match(tokens.L_PAREN) {
		for p.curr.IsValid() && !p.match(tokens.R_PAREN) {
			switch {
			case p.matchTextSeq("FILTER_COLUMN", "="):
				prop.Set("filter_column", p.parseColumn())
			case p.matchTextSeq("RETENTION_PERIOD", "="):
				prop.Set("retention_period", p.parseRetentionPeriod())
			}
			p.match(tokens.COMMA)
		}
	}
	return p.expression(prop, nil, nil)
}

func (p *Parser) parseReadsProperty() exp.Expression {
	if p.matchTextSeq("SQL", "DATA") {
		return p.expression(exp.SqlReadWriteProperty(exp.Args{"this": "READS SQL DATA"}), nil, nil)
	}
	return nil
}

// parseCreateLike ports _parse_create_like (parser.py:3360-3375).
func (p *Parser) parseCreateLike() exp.Expression {
	table := p.parseTable(true, false, nil, false, false, false, false)
	options := []exp.Expression{}
	for p.matchTexts(map[string]bool{"INCLUDING": true, "EXCLUDING": true}) {
		this := stringsUpper(p.prev.Text)
		idVar := p.parseIdVar(true, nil)
		if idVar == nil {
			return nil
		}
		options = append(options, p.expression(exp.Property(exp.Args{
			"this":  this,
			"value": exp.Var(exp.Args{"this": stringsUpper(idVar.Name())}),
		}), nil, nil))
	}
	return p.expression(exp.LikeProperty(exp.Args{"this": table, "expressions": options}), nil, nil)
}
