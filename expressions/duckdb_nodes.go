package expressions

// Builders for the DuckDB node kinds (expressions/{array,ddl,aggregate,temporal,query}.py).

func ArrayAppend(args Args) Expression { return newNode(KindArrayAppend, args) }
func ArrayConstructCompact(args Args) Expression {
	return newNode(KindArrayConstructCompact, args)
}
func ArrayIntersect(args Args) Expression   { return newNode(KindArrayIntersect, args) }
func ArrayPrepend(args Args) Expression     { return newNode(KindArrayPrepend, args) }
func Attach(args Args) Expression           { return newNode(KindAttach, args) }
func AttachOption(args Args) Expression     { return newNode(KindAttachOption, args) }
func DateBin(args Args) Expression          { return newNode(KindDateBin, args) }
func Install(args Args) Expression          { return newNode(KindInstall, args) }
func PercentileCont(args Args) Expression   { return newNode(KindPercentileCont, args) }
func PositionalColumn(args Args) Expression { return newNode(KindPositionalColumn, args) }
func SortArray(args Args) Expression        { return newNode(KindSortArray, args) }
func Length(args Args) Expression           { return newNode(KindLength, args) }
func RegexpFullMatch(args Args) Expression  { return newNode(KindRegexpFullMatch, args) }
func Summarize(args Args) Expression        { return newNode(KindSummarize, args) }
func UnpivotColumns(args Args) Expression   { return newNode(KindUnpivotColumns, args) }
