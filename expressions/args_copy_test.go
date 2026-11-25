package expressions

import (
	"slices"
	"testing"
)

// TestNodeArgOrderAndCopyTree pins the ordered small-args representation introduced for
// issue #89 and the tree invariants Copy must preserve: declared-then-sorted-extra arg order,
// dynamic Set/Append ordering, parent and list-index wiring, and copy isolation.
func TestNodeArgOrderAndCopyTree(t *testing.T) {
	// Declared arg keys come first in their arg_types order; any non-declared key follows,
	// sorted. The caller's map iteration order is irrelevant.
	col := Column(Args{
		"table": Identifier(Args{"this": "t"}),
		"this":  Identifier(Args{"this": "a"}),
		"zzz":   "z",
		"aaa":   "a",
	})
	if got, want := col.(*Node).ArgOrder(), []string{"this", "table", "aaa", "zzz"}; !slices.Equal(got, want) {
		t.Fatalf("ArgOrder() = %v, want %v", got, want)
	}
	if got := col.Arg("aaa"); got != "a" {
		t.Fatalf("Arg(aaa) = %v, want a", got)
	}
	if got := col.Arg("absent"); got != nil {
		t.Fatalf("Arg(absent) = %v, want nil", got)
	}

	// Set appends a new key, keeps an overwritten key's position, and nil deletes it.
	col.Set("later", 1)
	if got, want := col.(*Node).ArgOrder(), []string{"this", "table", "aaa", "zzz", "later"}; !slices.Equal(got, want) {
		t.Fatalf("after Set(later): %v, want %v", got, want)
	}
	col.Set("this", Identifier(Args{"this": "b"}))
	if got, want := col.(*Node).ArgOrder(), []string{"this", "table", "aaa", "zzz", "later"}; !slices.Equal(got, want) {
		t.Fatalf("overwrite reordered keys: %v, want %v", got, want)
	}
	col.Set("later", nil)
	if got, want := col.(*Node).ArgOrder(), []string{"this", "table", "aaa", "zzz"}; !slices.Equal(got, want) {
		t.Fatalf("Set(nil) did not delete: %v, want %v", got, want)
	}

	// Construction wires parents and list indexes.
	first := Identifier(Args{"this": "a"})
	second := Identifier(Args{"this": "b"})
	selectNode := Select(Args{"expressions": []Expression{first, second}})
	if first.Parent() != selectNode || second.Parent() != selectNode {
		t.Fatal("constructor did not wire parent pointers")
	}
	if first.Index() != 0 || second.Index() != 1 {
		t.Fatalf("constructor indexes = %d/%d, want 0/1", first.Index(), second.Index())
	}
	if first.ArgKey() != "expressions" {
		t.Fatalf("ArgKey = %q, want expressions", first.ArgKey())
	}

	// Copy keeps the arg order, deep-copies the tree, re-points parents/indexes at the copy,
	// and never aliases the original's nodes.
	clone := selectNode.Copy()
	if got, want := clone.(*Node).ArgOrder(), selectNode.(*Node).ArgOrder(); !slices.Equal(got, want) {
		t.Fatalf("copy ArgOrder() = %v, want %v", got, want)
	}
	cloneExprs := clone.Expressions()
	if len(cloneExprs) != 2 {
		t.Fatalf("copy expressions = %d, want 2", len(cloneExprs))
	}
	originalExprs := selectNode.Expressions()
	for i, child := range cloneExprs {
		if child == originalExprs[i] {
			t.Fatalf("copy aliases original child %d", i)
		}
		if child.Parent() != clone {
			t.Fatalf("copy child %d parent = %v, want the copy", i, child.Parent())
		}
		if child.Index() != i {
			t.Fatalf("copy child %d index = %d, want %d", i, child.Index(), i)
		}
	}

	// Mutating the copy does not affect the original, even for scalar args and list children.
	cloneExprs[0].Set("this", Identifier(Args{"this": "mutated"}))
	if got := originalExprs[0].Name(); got != "a" {
		t.Fatalf("mutating the copy changed the original: %q", got)
	}
	clone.Append("expressions", Identifier(Args{"this": "c"}))
	if got := len(selectNode.Expressions()); got != 2 {
		t.Fatalf("copy append leaked into original: %d expressions", got)
	}
	if got := len(clone.Expressions()); got != 3 {
		t.Fatalf("copy append missing: %d expressions", got)
	}
}
