package optimizer

import (
	"fmt"
	"strconv"
	"strings"

	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// BigQuery and Snowflake custom type annotators, ported from sqlglot/typing/bigquery.py and
// sqlglot/typing/snowflake.py (v30.17.0). The generated typing_metadata.go references the
// ann* wrappers below.

// snowflakeNumberType builds a Snowflake NUMBER type; it mirrors
// exp.DataType.from_str("NUMBER(...)", dialect="snowflake") in the upstream annotators.
func snowflakeNumberType(sql string) any {
	if t, err := exp.DataTypeBuild(sql, "snowflake", false, false, nil); err == nil {
		return t
	}
	return exp.DTypeUnknown
}

// dataTypeParamInt is `param.this.to_py()` for a DataType parameter: the integer value of the
// parameter's literal, or fallback when it is absent or not a literal.
func dataTypeParamInt(param exp.Expression, fallback int) int {
	if param == nil || param.Kind() != exp.KindDataTypeParam {
		return fallback
	}
	this := param.This()
	if this == nil || this.Kind() != exp.KindLiteral {
		return fallback
	}
	v, err := strconv.Atoi(this.Text("this"))
	if err != nil {
		return fallback
	}
	return v
}

// --- BigQuery ---

// annotateBQApproxTop ports _annotate_by_args_approx_top (typing/bigquery.py:102-113):
// ARRAY<STRUCT<value type, BIGINT>>.
func (a *TypeAnnotator) annotateBQApproxTop(e exp.Expression) exp.Expression {
	var valueType exp.Expression
	if this := e.This(); this != nil && this.Type() != nil {
		valueType = this.Type()
	}
	if valueType == nil {
		valueType = exp.DataType(exp.Args{"this": exp.DTypeUnknown})
	}
	structType := exp.DataType(exp.Args{
		"this":        exp.DTypeStruct,
		"expressions": []exp.Expression{valueType, exp.DataType(exp.Args{"this": exp.DTypeBigInt})},
		"nested":      true,
	})
	a.setType(e, exp.DataType(exp.Args{
		"this":        exp.DTypeArray,
		"expressions": []exp.Expression{structType},
		"nested":      true,
	}))
	return e
}

// isDataTypeKind ports the expressions-package is_data_type predicate: DataType and its
// subclasses (expressions/datatypes.py:190,451-461).
func isDataTypeKind(k exp.Kind) bool {
	switch k {
	case exp.KindDataType, exp.KindPseudoType, exp.KindObjectIdentifier, exp.KindIntervalSpan:
		return true
	}
	return false
}

// annotateBQMath ports _annotate_math_functions (typing/bigquery.py:50-65): an integer input
// returns DOUBLE; any other input keeps its own type.
func (a *TypeAnnotator) annotateBQMath(e exp.Expression) exp.Expression {
	this := e.This()
	if this != nil && exp.IntegerTypes[typeThis(this.Type())] {
		a.setType(e, exp.DTypeDouble)
	} else if this != nil {
		a.setType(e, this.Type())
	} else {
		a.setType(e, exp.DTypeUnknown)
	}
	return e
}

// annotateBQConcat ports _annotate_concat (typing/bigquery.py:116-124).
func (a *TypeAnnotator) annotateBQConcat(e exp.Expression) exp.Expression {
	a.annotateByArgs(e, annotateArgs{keys: []string{"expressions"}})
	// Args must be BYTES or string-castable; the result is BYTES or STRING.
	if !e.IsType(exp.DTypeBinary, exp.DTypeUnknown) {
		a.setType(e, exp.DTypeVarchar)
	}
	return e
}

// annotateBQDateFunc ports _annotate_date_func (typing/bigquery.py:33-47): a string-literal
// first argument is implicitly cast to the function's own temporal type.
func (a *TypeAnnotator) annotateBQDateFunc(e exp.Expression) exp.Expression {
	this := e.This()
	if this != nil && this.IsString() {
		var literalType exp.DType
		switch e.Kind() {
		case exp.KindDateAdd, exp.KindDateSub, exp.KindDateTrunc:
			literalType = exp.DTypeDate
		case exp.KindDatetimeTrunc:
			literalType = exp.DTypeDatetime
		case exp.KindTimestampTrunc:
			literalType = exp.DTypeTimestampTz
		default:
			literalType = exp.DTypeUnknown
		}
		a.setType(e, literalType)
		return e
	}
	return a.annotateByArgs(e, annotateArgs{keys: []string{"this"}})
}

// annotateBQByArgsWithCoerce ports _annotate_by_args_with_coerce (typing/bigquery.py:87-99).
func (a *TypeAnnotator) annotateBQByArgsWithCoerce(e exp.Expression) exp.Expression {
	left, right := e.This(), e.Expr()
	var leftType, rightType any
	if left != nil {
		leftType = left.Type()
	}
	if right != nil {
		rightType = right.Type()
	}
	a.setType(e, a.maybeCoerce(leftType, rightType))
	return e
}

// annotateBQSafeDivide ports _annotate_safe_divide (typing/bigquery.py:68-84).
func (a *TypeAnnotator) annotateBQSafeDivide(e exp.Expression) exp.Expression {
	left, right := e.This(), e.Expr()
	if left != nil && right != nil &&
		exp.IntegerTypes[typeThis(left.Type())] && exp.IntegerTypes[typeThis(right.Type())] {
		a.setType(e, exp.DTypeDouble)
		return e
	}
	return a.annotateBQByArgsWithCoerce(e)
}

// annotateBQJSONFormat ports the JSON_FORMAT lambda (typing/bigquery.py:372-376).
func (a *TypeAnnotator) annotateBQJSONFormat(e exp.Expression) exp.Expression {
	if truthyArg(e.Arg("to_json")) {
		a.setType(e, exp.DTypeJSON)
	} else {
		a.setType(e, exp.DTypeVarchar)
	}
	return e
}

// --- Snowflake ---

var snowflakeDateParts = map[string]bool{
	"DAY": true, "WEEK": true, "MONTH": true, "QUARTER": true, "YEAR": true,
}

const (
	snowflakeMaxPrecision = 38
	snowflakeMaxScale     = 37
)

// annotateSFReverse ports _annotate_reverse (typing/snowflake.py:19-25): REVERSE(NULL) is a
// VARCHAR in Snowflake.
func (a *TypeAnnotator) annotateSFReverse(e exp.Expression) exp.Expression {
	a.annotateByArgs(e, annotateArgs{keys: []string{"this"}})
	if e.IsType(exp.DTypeNull) {
		a.setType(e, exp.DTypeVarchar)
	}
	return e
}

// annotateSFTimestampFromParts ports _annotate_timestamp_from_parts
// (typing/snowflake.py:28-40): a time_zone argument yields TIMESTAMPTZ, otherwise TIMESTAMP.
func (a *TypeAnnotator) annotateSFTimestampFromParts(e exp.Expression) exp.Expression {
	if truthyArg(e.Arg("zone")) {
		a.setType(e, exp.DTypeTimestampTz)
	} else {
		a.setType(e, exp.DTypeTimestamp)
	}
	return e
}

// annotateSFDateOrTimeAdd ports _annotate_date_or_time_add (typing/snowflake.py:43-51).
func (a *TypeAnnotator) annotateSFDateOrTimeAdd(e exp.Expression) exp.Expression {
	if this := e.This(); this != nil && this.IsType(exp.DTypeDate) &&
		!snowflakeDateParts[strings.ToUpper(e.Text("unit"))] {
		a.setType(e, exp.DTypeTimestampNtz)
		return e
	}
	return a.annotateByArgs(e, annotateArgs{keys: []string{"this"}})
}

// annotateSFDecodeCase ports _annotate_decode_case (typing/snowflake.py:54-80): only the
// return values (indices 2, 4, ... plus a trailing default) determine the type.
func (a *TypeAnnotator) annotateSFDecodeCase(e exp.Expression) exp.Expression {
	exprs := e.Expressions()
	var returnTypes []any
	for i := 2; i < len(exprs); i += 2 {
		returnTypes = append(returnTypes, exprs[i].Type())
	}
	if len(exprs)%2 == 0 {
		returnTypes = append(returnTypes, exprs[len(exprs)-1].Type())
	}
	var last any
	for _, retType := range returnTypes {
		if isNilType(last) {
			last = retType
		}
		last = a.maybeCoerce(last, retType)
	}
	a.setType(e, last)
	return e
}

// annotateSFArgMaxMin ports _annotate_arg_max_min (typing/snowflake.py:83-88): the count form
// returns ARRAY, otherwise the value argument's type.
func (a *TypeAnnotator) annotateSFArgMaxMin(e exp.Expression) exp.Expression {
	if truthyArg(e.Arg("count")) {
		a.setType(e, exp.DTypeArray)
	} else if this := e.This(); this != nil {
		a.setType(e, this.Type())
	} else {
		a.setType(e, exp.DTypeUnknown)
	}
	return e
}

// annotateSFWithinGroup ports Snowflake's _annotate_within_group (typing/snowflake.py:91-108):
// PERCENTILE_DISC/CONT take the ordered expression's type.
func (a *TypeAnnotator) annotateSFWithinGroup(e exp.Expression) exp.Expression {
	this := e.This()
	orderExpr := e.Expr()
	if this != nil && (this.Kind() == exp.KindPercentileDisc || this.Kind() == exp.KindPercentileCont) &&
		orderExpr != nil && orderExpr.Kind() == exp.KindOrder && len(orderExpr.Expressions()) == 1 {
		if ordered := orderExpr.Expressions()[0]; ordered != nil && ordered.Kind() == exp.KindOrdered {
			if orderedThis := ordered.This(); orderedThis != nil {
				a.setType(e, orderedThis.Type())
				return e
			}
		}
	}
	if this != nil {
		a.setType(e, this.Type())
	} else {
		a.setType(e, exp.DTypeUnknown)
	}
	return e
}

// annotateSFMedian ports _annotate_median (typing/snowflake.py:111-146).
func (a *TypeAnnotator) annotateSFMedian(e exp.Expression) exp.Expression {
	a.annotateByArgs(e, annotateArgs{keys: []string{"this"}})
	this := e.This()
	if this == nil {
		return e
	}
	inputType := this.Type()
	if inputType != nil && inputType.IsType(exp.DTypeDouble) {
		a.setType(e, exp.DTypeDouble)
		return e
	}
	var precision, scale int
	if inputType != nil {
		exprs := inputType.Expressions()
		p, _ := seqGet(exprs, 0)
		precision = dataTypeParamInt(p, snowflakeMaxPrecision)
		s, _ := seqGet(exprs, 1)
		scale = dataTypeParamInt(s, 0)
	} else {
		precision = snowflakeMaxPrecision
	}
	newPrecision := min(precision+3, snowflakeMaxPrecision)
	newScale := min(scale+3, snowflakeMaxScale)
	a.setType(e, snowflakeNumberType(fmt.Sprintf("NUMBER(%d, %d)", newPrecision, newScale)))
	return e
}

// annotateSFVariance ports _annotate_variance (typing/snowflake.py:149-186).
func (a *TypeAnnotator) annotateSFVariance(e exp.Expression) exp.Expression {
	a.annotateByArgs(e, annotateArgs{keys: []string{"this"}})
	this := e.This()
	if this == nil {
		return e
	}
	inputType := this.Type()
	switch {
	case inputType != nil && inputType.IsType(exp.DTypeDecFloat):
		a.setType(e, snowflakeNumberType("DECFLOAT"))
	case inputType != nil && inputType.IsType(exp.DTypeFloat, exp.DTypeDouble):
		a.setType(e, exp.DTypeDouble)
	default:
		var scale int
		if inputType != nil {
			s, _ := seqGet(inputType.Expressions(), 1)
			scale = dataTypeParamInt(s, 0)
		}
		newScale := 6
		if scale != 0 {
			newScale = max(12, scale)
		}
		a.setType(e, snowflakeNumberType(fmt.Sprintf("NUMBER(%d, %d)", snowflakeMaxPrecision, newScale)))
	}
	return e
}

// annotateSFKurtosis ports _annotate_kurtosis (typing/snowflake.py:189-209).
func (a *TypeAnnotator) annotateSFKurtosis(e exp.Expression) exp.Expression {
	a.annotateByArgs(e, annotateArgs{keys: []string{"this"}})
	this := e.This()
	if this == nil {
		return e
	}
	inputType := this.Type()
	switch {
	case inputType != nil && inputType.IsType(exp.DTypeDecFloat):
		a.setType(e, snowflakeNumberType("DECFLOAT"))
	case inputType != nil && inputType.IsType(exp.DTypeFloat, exp.DTypeDouble):
		a.setType(e, exp.DTypeDouble)
	default:
		a.setType(e, snowflakeNumberType(fmt.Sprintf("NUMBER(%d, 12)", snowflakeMaxPrecision)))
	}
	return e
}

// annotateSFMathFloatDecfloat ports _annotate_math_with_float_decfloat
// (typing/snowflake.py:212-229): DECFLOAT is preserved, everything else returns DOUBLE.
func (a *TypeAnnotator) annotateSFMathFloatDecfloat(e exp.Expression) exp.Expression {
	a.annotateByArgs(e, annotateArgs{keys: []string{"this"}})
	if this := e.This(); this != nil && this.IsType(exp.DTypeDecFloat) {
		a.setType(e, this.Type())
	} else {
		a.setType(e, exp.DTypeDouble)
	}
	return e
}

// annotateSFStrToTime ports _annotate_str_to_time (typing/snowflake.py:232-239).
func (a *TypeAnnotator) annotateSFStrToTime(e exp.Expression) exp.Expression {
	if target, ok := e.Arg("target_type").(exp.Expression); ok && target != nil && isDataTypeKind(target.Kind()) {
		a.setType(e, target.Arg("this"))
	} else {
		a.setType(e, exp.DTypeTimestamp)
	}
	return e
}

// annotateSFConvertTimezone ports the ConvertTimezone lambda (typing/snowflake.py:545-550).
func (a *TypeAnnotator) annotateSFConvertTimezone(e exp.Expression) exp.Expression {
	if truthyArg(e.Arg("source_tz")) {
		a.setType(e, exp.DTypeTimestampNtz)
	} else {
		a.setType(e, exp.DTypeTimestampTz)
	}
	return e
}

// --- generated-table wrappers ---

func annBQApproxTop(a *TypeAnnotator, e exp.Expression)          { a.annotateBQApproxTop(e) }
func annBQMath(a *TypeAnnotator, e exp.Expression)               { a.annotateBQMath(e) }
func annBQConcat(a *TypeAnnotator, e exp.Expression)             { a.annotateBQConcat(e) }
func annBQDateFunc(a *TypeAnnotator, e exp.Expression)           { a.annotateBQDateFunc(e) }
func annBQByArgsWithCoerce(a *TypeAnnotator, e exp.Expression)   { a.annotateBQByArgsWithCoerce(e) }
func annBQSafeDivide(a *TypeAnnotator, e exp.Expression)         { a.annotateBQSafeDivide(e) }
func annBQJSONFormat(a *TypeAnnotator, e exp.Expression)         { a.annotateBQJSONFormat(e) }
func annSFReverse(a *TypeAnnotator, e exp.Expression)            { a.annotateSFReverse(e) }
func annSFTimestampFromParts(a *TypeAnnotator, e exp.Expression) { a.annotateSFTimestampFromParts(e) }
func annSFDateOrTimeAdd(a *TypeAnnotator, e exp.Expression)      { a.annotateSFDateOrTimeAdd(e) }
func annSFDecodeCase(a *TypeAnnotator, e exp.Expression)         { a.annotateSFDecodeCase(e) }
func annSFArgMaxMin(a *TypeAnnotator, e exp.Expression)          { a.annotateSFArgMaxMin(e) }
func annSFWithinGroup(a *TypeAnnotator, e exp.Expression)        { a.annotateSFWithinGroup(e) }
func annSFMedian(a *TypeAnnotator, e exp.Expression)             { a.annotateSFMedian(e) }
func annSFVariance(a *TypeAnnotator, e exp.Expression)           { a.annotateSFVariance(e) }
func annSFKurtosis(a *TypeAnnotator, e exp.Expression)           { a.annotateSFKurtosis(e) }
func annSFMathFloatDecfloat(a *TypeAnnotator, e exp.Expression) {
	a.annotateSFMathFloatDecfloat(e)
}
func annSFStrToTime(a *TypeAnnotator, e exp.Expression)       { a.annotateSFStrToTime(e) }
func annSFConvertTimezone(a *TypeAnnotator, e exp.Expression) { a.annotateSFConvertTimezone(e) }
