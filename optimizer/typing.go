package optimizer

import (
	"strings"
	"sync"

	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// Ports the expression-type metadata of sqlglot/typing (Dialect.EXPRESSION_METADATA): for each
// expression class, the type it returns or the annotator that infers it. The tables live in
// typing_metadata.go: the base table (sqlglot/typing/__init__.py) plus, per dialect, the entries
// its typing module and dialect class resolve differently (sqlglot/typing/<dialect>.py), with
// inherited entries spelled out for each dialect. Classes with no Kind in this port yet are
// skipped, so those expressions annotate as UNKNOWN until their node lands.

// typeSpec is one EXPRESSION_METADATA value: {"returns": type} or {"annotator": fn}.
type typeSpec struct {
	returns   any // an exp.DType or a DataType expression
	annotator func(*TypeAnnotator, exp.Expression)
}

var (
	typingRegistry    = map[string]map[string]typeSpec{}
	typingResolved    sync.Map // dialect name -> map[exp.Kind]typeSpec
	hiveCoercesToOnce sync.Once
	hiveCoercesTo     map[exp.DType]map[exp.DType]bool
)

func registerTypingMetadata(dialect string, specs map[string]typeSpec) {
	typingRegistry[dialect] = specs
}

// expressionMetadataFor resolves a dialect's EXPRESSION_METADATA: base overlaid with the
// dialect's own entries.
func expressionMetadataFor(dialect string) map[exp.Kind]typeSpec {
	dialect = strings.ToLower(dialect)
	if cached, ok := typingResolved.Load(dialect); ok {
		return cached.(map[exp.Kind]typeSpec)
	}
	out := map[exp.Kind]typeSpec{}
	for _, specs := range []map[string]typeSpec{typingRegistry[""], typingRegistry[dialect]} {
		for name, spec := range specs {
			if k, ok := exp.KindByClassName(name); ok {
				out[k] = spec
			}
		}
	}
	actual, _ := typingResolved.LoadOrStore(dialect, out)
	return actual.(map[exp.Kind]typeSpec)
}

// coercesToFor ports Dialect.COERCES_TO or TypeAnnotator.COERCES_TO (annotate_types.py:208):
// Hive (non-ANSI mode, dialects/hive.py:37-44) also lets numeric, temporal and interval types
// coerce into text.
func coercesToFor(dialect string) map[exp.DType]map[exp.DType]bool {
	if strings.ToLower(dialect) != "hive" {
		return defaultCoercesTo
	}
	hiveCoercesToOnce.Do(func() {
		hiveCoercesTo = map[exp.DType]map[exp.DType]bool{}
		for k, v := range defaultCoercesTo {
			copied := map[exp.DType]bool{}
			for t := range v {
				copied[t] = true
			}
			hiveCoercesTo[k] = copied
		}
		targets := map[exp.DType]bool{exp.DTypeInterval: true}
		for t := range exp.NumericTypes {
			targets[t] = true
		}
		for t := range exp.TemporalTypes {
			targets[t] = true
		}
		for target := range targets {
			if hiveCoercesTo[target] == nil {
				hiveCoercesTo[target] = map[exp.DType]bool{}
			}
			for t := range exp.TextTypes {
				hiveCoercesTo[target][t] = true
			}
		}
	})
	return hiveCoercesTo
}

// dtype resolves an upstream DType value name (e.g. "VARCHAR") to the Go DType.
func dtype(name string) exp.DType {
	if d, ok := exp.DTypeFromName(name); ok {
		return d
	}
	return exp.DType(name)
}

// dataTypeOf builds DataType(this=this, expressions=[...], nested=...) the way DataType.build
// parses a parameterized type: nested when it has DataType parameters.
func dataTypeOf(this exp.DType, params ...exp.Expression) exp.Expression {
	args := exp.Args{"this": this}
	if len(params) > 0 {
		args["expressions"] = params
		if params[0].Kind() == exp.KindDataType {
			args["nested"] = true
		}
	}
	return exp.DataType(args)
}

// Annotators referenced by typing_metadata.go, one per distinct upstream lambda.

func annBinary(a *TypeAnnotator, e exp.Expression)         { a.annotateBinary(e) }
func annUnary(a *TypeAnnotator, e exp.Expression)          { a.annotateUnary(e) }
func annBracket(a *TypeAnnotator, e exp.Expression)        { a.annotateBracket(e) }
func annDiv(a *TypeAnnotator, e exp.Expression)            { a.annotateDiv(e) }
func annDot(a *TypeAnnotator, e exp.Expression)            { a.annotateDot(e) }
func annExplode(a *TypeAnnotator, e exp.Expression)        { a.annotateExplode(e) }
func annExtract(a *TypeAnnotator, e exp.Expression)        { a.annotateExtract(e) }
func annLiteral(a *TypeAnnotator, e exp.Expression)        { a.annotateLiteral(e) }
func annStruct(a *TypeAnnotator, e exp.Expression)         { a.annotateStruct(e) }
func annToMap(a *TypeAnnotator, e exp.Expression)          { a.annotateToMap(e) }
func annUnnest(a *TypeAnnotator, e exp.Expression)         { a.annotateUnnest(e) }
func annWithinGroup(a *TypeAnnotator, e exp.Expression)    { a.annotateWithinGroup(e) }
func annSubquery(a *TypeAnnotator, e exp.Expression)       { a.annotateSubquery(e) }
func annByArrayElement(a *TypeAnnotator, e exp.Expression) { a.annotateByArrayElement(e) }
func annTimeunit(a *TypeAnnotator, e exp.Expression)       { a.annotateTimeunit(e) }
func annMap(a *TypeAnnotator, e exp.Expression)            { a.annotateMap(e) }
func annNoop(*TypeAnnotator, exp.Expression)               {}
func annCast(a *TypeAnnotator, e exp.Expression)           { a.setType(e, asExpression(e.Arg("to"))) }
func annTimestamp(a *TypeAnnotator, e exp.Expression) {
	a.setType(e, flagType(e, "with_tz", exp.DTypeTimestampTz, exp.DTypeTimestamp))
}
func annHexString(a *TypeAnnotator, e exp.Expression) {
	a.setType(e, flagType(e, "is_integer", exp.DTypeBigInt, exp.DTypeBinary))
}
func annMySQLReverse(a *TypeAnnotator, e exp.Expression)  { annotateMySQLReverse(a, e) }
func annMySQLTruncate(a *TypeAnnotator, e exp.Expression) { annotateMySQLTruncate(a, e) }
func annMySQLCompress(a *TypeAnnotator, e exp.Expression) { annotateMySQLCompress(a, e) }
func annMySQLBitFunc(a *TypeAnnotator, e exp.Expression)  { annotateMySQLBitFunc(a, e) }
func annMySQLRegexpReplace(a *TypeAnnotator, e exp.Expression) {
	annotateMySQLRegexpReplace(a, e)
}

// annUDF ports `self._set_type(e, self.schema.get_udf_type(e))` (typing/__init__.py:322).
func annUDF(a *TypeAnnotator, e exp.Expression) {
	udfType, err := a.schema.GetUDFType(e, nil, nil)
	if err != nil {
		udfType = nil
	}
	a.setType(e, udfType)
}

// annCase ports the Case annotator (typing/__init__.py:343-347): every THEN value, then the
// default.
func annCase(a *TypeAnnotator, e exp.Expression) {
	var exprs []exp.Expression
	for _, ifExpr := range expressionsFor(e, "ifs") {
		if t := asExpression(ifExpr.Arg("true")); t != nil {
			exprs = append(exprs, t)
		}
	}
	if def := asExpression(e.Arg("default")); def != nil {
		exprs = append(exprs, def)
	}
	a.annotateByArgs(e, annotateArgs{exprs: exprs})
}

// annBigIntFlag ports `BIGINT if e.args.get(flag) else INT` (Count, DateDiff).
func annBigIntFlag(flag string) func(*TypeAnnotator, exp.Expression) {
	return func(a *TypeAnnotator, e exp.Expression) {
		a.setType(e, flagType(e, flag, exp.DTypeBigInt, exp.DTypeInt))
	}
}

// annPrestoRand ports Presto's Rand annotator (typing/presto.py:30-34).
func annPrestoRand(a *TypeAnnotator, e exp.Expression) {
	if e.This() != nil {
		a.annotateByArgs(e, byKeys("this"))
		return
	}
	a.setType(e, exp.DTypeDouble)
}

func annByArgs(args annotateArgs) func(*TypeAnnotator, exp.Expression) {
	return func(a *TypeAnnotator, e exp.Expression) { a.annotateByArgs(e, args) }
}

// annBuild ports `self._set_type(e, exp.DataType.build(sql, dialect=dialect))` (and from_str):
// the type is parsed on first use, once the root package has installed the parser hooks.
func annBuild(sql, dialect string) func(*TypeAnnotator, exp.Expression) {
	var once sync.Once
	var built exp.Expression
	return func(a *TypeAnnotator, e exp.Expression) {
		once.Do(func() {
			if t, err := exp.DataTypeBuild(sql, dialect, false, false, nil); err == nil {
				built = t
			}
		})
		var typ any
		if built != nil {
			typ = built.Copy()
		}
		a.setType(e, typ)
	}
}

func flagType(e exp.Expression, flag string, ifSet, otherwise exp.DType) exp.DType {
	if truthyArg(e.Arg(flag)) {
		return ifSet
	}
	return otherwise
}

// MySQL annotators (typing/mysql.py:12-91).

var mysqlCompressLongblobTypes = []any{
	exp.DTypeText, exp.DTypeMediumText, exp.DTypeLongText, exp.DTypeBlob, exp.DTypeMediumBlob, exp.DTypeLongBlob, exp.DTypeJSON,
}

var mysqlCompressVarbinaryTypes = []any{
	exp.DTypeChar, exp.DTypeVarchar, exp.DTypeBinary, exp.DTypeVarBinary, exp.DTypeTinyBlob, exp.DTypeEnum,
	exp.DTypeInt, exp.DTypeBigInt, exp.DTypeDecimal, exp.DTypeDouble, exp.DTypeDate, exp.DTypeDatetime,
}

func dtypeSet(set map[exp.DType]bool) []any {
	out := make([]any, 0, len(set))
	for t := range set {
		out = append(out, t)
	}
	return out
}

func annotateMySQLReverse(a *TypeAnnotator, e exp.Expression) {
	if this := e.This(); this != nil && this.IsType(exp.DTypeBinary, exp.DTypeVarBinary, exp.DTypeUnknown) {
		a.annotateByArgs(e, byKeys("this"))
		return
	}
	a.setType(e, exp.DTypeVarchar)
}

func annotateMySQLTruncate(a *TypeAnnotator, e exp.Expression) {
	if this := e.This(); this != nil && this.IsType(dtypeSet(exp.TextTypes)...) {
		a.setType(e, exp.DTypeDouble)
		return
	}
	a.annotateByArgs(e, byKeys("this"))
}

func annotateMySQLRegexpReplace(a *TypeAnnotator, e exp.Expression) {
	hasBinary := false
	for _, arg := range []exp.Expression{e.This(), e.Expr(), asExpression(e.Arg("replacement"))} {
		if arg == nil {
			continue
		}
		if arg.IsType(exp.DTypeUnknown) {
			a.setType(e, exp.DTypeUnknown)
			return
		}
		hasBinary = hasBinary || arg.IsType(dtypeSet(exp.BinaryTypes)...)
	}
	if hasBinary {
		a.setType(e, exp.DTypeLongBlob)
	} else {
		a.setType(e, exp.DTypeLongText)
	}
}

func annotateMySQLCompress(a *TypeAnnotator, e exp.Expression) {
	this := e.This()
	switch {
	case this != nil && this.IsType(mysqlCompressVarbinaryTypes...):
		a.setType(e, exp.DTypeVarBinary)
	case this != nil && this.IsType(mysqlCompressLongblobTypes...):
		a.setType(e, exp.DTypeLongBlob)
	case this != nil && this.IsType(exp.DTypeTinyText):
		a.setType(e, exp.DTypeBlob)
	default:
		a.setType(e, exp.DTypeUnknown)
	}
}

func annotateMySQLBitFunc(a *TypeAnnotator, e exp.Expression) {
	this := e.This()
	switch {
	case this == nil || this.IsType(exp.DTypeUnknown):
		a.setType(e, exp.DTypeUnknown)
	case this.IsType(dtypeSet(exp.BinaryTypes)...):
		a.setType(e, exp.DTypeVarBinary)
	default:
		a.setType(e, exp.DTypeUBigInt)
	}
}
