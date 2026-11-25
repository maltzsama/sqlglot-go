package expressions

// ArgsOf returns a shallow copy of e's live args map (keys -> node|[]node|scalar),
// mirroring Python's Expression.args for callers that must iterate all set args.
func ArgsOf(e Expression) map[string]any {
	n, ok := e.(*Node)
	if !ok || n == nil {
		return nil
	}
	out := make(map[string]any, len(n.args))
	for i := range n.args {
		out[n.args[i].key] = n.args[i].value
	}
	return out
}
