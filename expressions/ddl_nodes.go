package expressions

// Builders for the "ddl" cluster: CREATE FUNCTION/PROCEDURE/INDEX/TRIGGER structured
// parsing, plus the small PROPERTY_PARSERS subset needed for the DDL-tail parity gaps. See
// the Kind block comment in kinds.go for the upstream class/line references.

// Properties is the generic property-list container (properties.py:590-591,
// `arg_types = {"expressions": True}`).
func Properties(args Args) Expression { return newNode(KindProperties, args) }

// UserDefinedFunction backs a CREATE FUNCTION/PROCEDURE signature, e.g. `f(a INT, b INT)`
// (ddl.py:225-226).
func UserDefinedFunction(args Args) Expression { return newNode(KindUserDefinedFunction, args) }

// MacroOverloads/MacroOverload back DuckDB's multi-signature CREATE MACRO and one of its
// `(params) AS [TABLE] body` entries (ddl.py:229-234).
func MacroOverloads(args Args) Expression { return newNode(KindMacroOverloads, args) }
func MacroOverload(args Args) Expression  { return newNode(KindMacroOverload, args) }

// ReturnsProperty/LanguageProperty/SqlSecurityProperty/CalledOnNullInputProperty/
// StrictProperty/StabilityProperty/SetConfigProperty/CharacterSetProperty/RowFormatProperty
// back the CREATE FUNCTION/TABLE tail properties this slice supports (properties.py:74,222,
// 393,397,401,456ish,464,480 - see the kinds.go block comment for exact line references).
func ReturnsProperty(args Args) Expression     { return newNode(KindReturnsProperty, args) }
func LanguageProperty(args Args) Expression    { return newNode(KindLanguageProperty, args) }
func SqlSecurityProperty(args Args) Expression { return newNode(KindSqlSecurityProperty, args) }
func CalledOnNullInputProperty(args Args) Expression {
	return newNode(KindCalledOnNullInputProperty, args)
}
func StrictProperty(args Args) Expression       { return newNode(KindStrictProperty, args) }
func StabilityProperty(args Args) Expression    { return newNode(KindStabilityProperty, args) }
func SetConfigProperty(args Args) Expression    { return newNode(KindSetConfigProperty, args) }
func CharacterSetProperty(args Args) Expression { return newNode(KindCharacterSetProperty, args) }
func RowFormatProperty(args Args) Expression    { return newNode(KindRowFormatProperty, args) }

// HandlerProperty backs the CREATE FUNCTION `HANDLER <expr>` property (properties.py:160-161).
func HandlerProperty(args Args) Expression { return newNode(KindHandlerProperty, args) }

// DynamicProperty backs the bare DYNAMIC marker of CREATE DYNAMIC TABLE (properties.py:254-255).
func DynamicProperty(args Args) Expression { return newNode(KindDynamicProperty, args) }

// Return wraps a UDF/procedure body in `RETURN <expr>` (query.py:882-883).
func Return(args Args) Expression { return newNode(KindReturn, args) }

// Index/Opclass back CREATE INDEX (query.py:558-566, core.py:1817-1818): Index's "params"
// arg is the already-ported IndexParameters node (expressions/constraints.go).
func Index(args Args) Expression   { return newNode(KindIndex, args) }
func Opclass(args Args) Expression { return newNode(KindOpclass, args) }

// TriggerProperties/TriggerExecute/TriggerEvent/TriggerReferencing back CREATE TRIGGER
// (ddl.py:76-101).
func TriggerProperties(args Args) Expression { return newNode(KindTriggerProperties, args) }

// SequenceProperties ports exp.SequenceProperties (expressions/ddl.py:64-73).
func SequenceProperties(args Args) Expression { return newNode(KindSequenceProperties, args) }
func TriggerExecute(args Args) Expression     { return newNode(KindTriggerExecute, args) }
func TriggerEvent(args Args) Expression       { return newNode(KindTriggerEvent, args) }
func TriggerReferencing(args Args) Expression { return newNode(KindTriggerReferencing, args) }
