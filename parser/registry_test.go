package parser

import "testing"

// TestParserRegistriesContainAllInitEntries pins the package-level parser registries after
// every file's init() has run. The base literals live in parser.go and extra entries are added
// by parser_functions.go; the two merge rather than overwrite each other, so this guards
// against a future edit that reintroduces an init-order dependency and silently drops entries.
func TestParserRegistriesContainAllInitEntries(t *testing.T) {
	functionKeys := []string{
		"OVERLAY", "MATCH", "SUBSTR", "CONCAT", // parser_functions.go
		"CAST", "TRY_CAST", "EXTRACT", "POSITION", "SUBSTRING", "TRIM", // parser.go
	}
	for _, key := range functionKeys {
		if functionParsers[key] == nil {
			t.Errorf("functionParsers[%q] is nil", key)
		}
	}
	if noParenFunctionParsers["VARIADIC"] == nil { // parser_functions.go
		t.Error(`noParenFunctionParsers["VARIADIC"] is nil`)
	}
	for _, key := range []string{"CONNECT_BY_ROOT", "ANY", "CASE", "PRIOR", "IF"} { // parser.go
		if noParenFunctionParsers[key] == nil {
			t.Errorf("noParenFunctionParsers[%q] is nil", key)
		}
	}
}
