package optimizer

import exp "github.com/maltzsama/sqlglot-go/expressions"

// JournalEntry ports journal.JournalEntry (optimizer/journal.py:8): one recorded argument
// mutation — the node, the arg key, and the value before the mutation.
type JournalEntry struct {
	Node   exp.Expression
	ArgKey string
	Value  any
}

// Journal ports journal.Journal (optimizer/journal.py:9): the recorded mutations a rule made,
// so Revert can undo them.
type Journal []JournalEntry

// Record ports record (optimizer/journal.py:12-19): it saves the current value of
// node's argKey arg so Revert can restore it. It must be called before the arg is mutated.
// List values are copied shallowly, so the caller may freely mutate or replace the list
// afterwards.
func (j *Journal) Record(node exp.Expression, argKey string) {
	value := node.Arg(argKey)
	switch v := value.(type) {
	case []exp.Expression:
		value = append([]exp.Expression(nil), v...)
	case []any:
		value = append([]any(nil), v...)
	}
	*j = append(*j, JournalEntry{Node: node, ArgKey: argKey, Value: value})
}

// Revert ports revert (optimizer/journal.py:22-32): it restores every recorded arg, newest
// first, and empties the journal. Set reattaches the restored expressions (their parent, arg
// key and index), so rules that only detach or reorder nodes leave the tree exactly as it was
// before they ran. Rules that replace nodes with new ones can't be reverted this way.
func (j *Journal) Revert() {
	for i := len(*j) - 1; i >= 0; i-- {
		entry := (*j)[i]
		entry.Node.Set(entry.ArgKey, entry.Value)
	}
	*j = (*j)[:0]
}
