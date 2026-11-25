package expressions

import "testing"

// TestKindExtensionBase guards the generated extension block's invariant: it starts at
// kindExtensionBase, so every base Kind must be strictly below it. Appending a base Kind
// without moving kindExtensionBase would alias it onto KindAIAgg (the first extension Kind)
// and silently repoint that node's dispatch, arg types, and class name.
func TestKindExtensionBase(t *testing.T) {
	if KindIntroducer >= kindExtensionBase {
		t.Fatalf("KindIntroducer (%d) must be below kindExtensionBase (%d)", KindIntroducer, kindExtensionBase)
	}
	if KindAIAgg < kindExtensionBase {
		t.Fatalf("KindAIAgg (%d) must be at or above kindExtensionBase (%d)", KindAIAgg, kindExtensionBase)
	}
	if KindIntroducer == KindAIAgg {
		t.Fatal("KindIntroducer collides with KindAIAgg")
	}
	if got := ClassName(KindIntroducer); got != "Introducer" {
		t.Fatalf("ClassName(KindIntroducer) = %q, want Introducer", got)
	}
	if got := ClassName(KindAIAgg); got != "AIAgg" {
		t.Fatalf("ClassName(KindAIAgg) = %q, want AIAgg", got)
	}
}
