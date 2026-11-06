package expressions

// Annotated types, ported from Expression.type / is_type (expressions/core.py:968-986): the
// DataType an expression evaluates to, set by the optimizer's annotate_types pass.

// isDataTypeKind ports is_data_type: DataType and its subclasses PseudoType, ObjectIdentifier
// and IntervalSpan (expressions/datatypes.py:190,451-461).
func isDataTypeKind(k Kind) bool {
	switch k {
	case KindDataType, KindPseudoType, KindObjectIdentifier, KindIntervalSpan:
		return true
	}
	return false
}

// isCastKind ports is_cast: Cast and its subclasses TryCast and JSONCast
// (expressions/functions.py:35,72,81).
func isCastKind(k Kind) bool {
	return k == KindCast || k == KindTryCast || k == KindJSONCast
}

// Type ports the Expression.type getter: a DataType is its own type, a cast defaults to its
// target type, and any other expression has the type annotate_types set (nil when none).
func (n *Node) Type() Expression {
	if isDataTypeKind(n.kind) {
		return n
	}
	if n.typ != nil {
		return n.typ
	}
	if isCastKind(n.kind) {
		return asExpression(n.args["to"])
	}
	return nil
}

// SetType ports the Expression.type setter: a DType, a type string or an Expression is built
// into a DataType (a DataType is used as-is); nil clears the type. A value that can't be built
// clears it too.
func (n *Node) SetType(dtype any) {
	switch v := dtype.(type) {
	case nil:
		n.typ = nil
		return
	case Expression:
		if v == nil {
			n.typ = nil
			return
		}
		if isDataTypeKind(v.Kind()) {
			n.typ = v
			return
		}
	}
	built, err := DataTypeBuild(dtype, "", false, false, nil)
	if err != nil {
		n.typ = nil
		return
	}
	n.typ = built
}

// IsType ports is_type. On a DataType it is DataType.is_type (expressions/datatypes.py:419-447);
// on any other expression it checks the annotated type (expressions/core.py:984-986), which a
// cast's target type does not count as.
func (n *Node) IsType(dtypes ...any) bool {
	if isDataTypeKind(n.kind) {
		return DataTypeIsType(n, false, dtypes...)
	}
	return n.typ != nil && DataTypeIsType(n.typ, false, dtypes...)
}
