package expressions

import "testing"

// TestAnnotatedType covers the Expression.type port: setting a DType builds a DataType, copies
// keep the annotation, a cast falls back to its target type, and IsType reads only the
// annotation on a non-DataType node, as upstream's Expression.is_type does.
func TestAnnotatedType(t *testing.T) {
	col := Column(Args{"this": ToIdentifier("a")})
	if col.Type() != nil || col.IsType(DTypeInt) {
		t.Fatal("unannotated column has a type")
	}
	col.SetType(DTypeInt)
	if typ := col.Type(); typ == nil || typ.Arg("this") != DTypeInt || !col.IsType(DTypeInt, DTypeText) || col.IsType(DTypeText) {
		t.Fatalf("SetType(INT): type=%v", typ)
	}
	if copied := col.Copy(); !copied.IsType(DTypeInt) || copied.Type() == col.Type() {
		t.Fatal("Copy did not deep-copy the annotated type")
	}
	col.SetType(nil)
	if col.Type() != nil {
		t.Fatal("SetType(nil) did not clear the type")
	}

	cast := Cast(Args{"this": col, "to": DataType(Args{"this": DTypeText})})
	if typ := cast.Type(); typ == nil || typ.Arg("this") != DTypeText {
		t.Fatalf("cast type = %v, want its target type", typ)
	}
	if cast.IsType(DTypeText) {
		t.Fatal("IsType on an unannotated cast must not read its target type")
	}

	dt := DataType(Args{"this": DTypeDouble})
	if dt.Type() != dt || !dt.IsType(DTypeDouble) {
		t.Fatal("a DataType is its own type")
	}
}
