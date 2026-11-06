package optimizer

import (
	"reflect"
	"testing"
)

func set(names ...string) map[string]bool {
	out := map[string]bool{}
	for _, name := range names {
		out[name] = true
	}
	return out
}

// TestTsort ports test_tsort (tests/test_helper.py:7-30).
func TestTsort(t *testing.T) {
	for _, tc := range []struct {
		dag  map[string]map[string]bool
		want []string
	}{
		{map[string]map[string]bool{"a": set()}, []string{"a"}},
		{map[string]map[string]bool{"a": set("b")}, []string{"b", "a"}},
		{map[string]map[string]bool{"a": set("c"), "b": set(), "c": set()}, []string{"b", "c", "a"}},
		{map[string]map[string]bool{"a": set("b", "c"), "b": set("c"), "c": set(), "d": set("a")}, []string{"c", "b", "a", "d"}},
	} {
		if got := tsort(tc.dag); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("tsort(%v) = %v, want %v", tc.dag, got, tc.want)
		}
	}

	defer func() {
		if recover() == nil {
			t.Error("tsort on a cycle did not panic")
		}
	}()
	tsort(map[string]map[string]bool{"a": set("b", "c"), "b": set("a"), "c": set()})
}
