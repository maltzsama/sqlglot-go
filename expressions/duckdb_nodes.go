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
func RegexpFullMatch(args Args) Expression  { return newNode(KindRegexpFullMatch, args) }
func Summarize(args Args) Expression        { return newNode(KindSummarize, args) }
func UnpivotColumns(args Args) Expression   { return newNode(KindUnpivotColumns, args) }

// ConnectTo is DuckDB's `CONNECT 'uri'` statement (expressions/ddl.py:134).
func ConnectTo(args Args) Expression { return newNode(KindConnectTo, args) }

// Disconnect is DuckDB's `DISCONNECT` statement (expressions/ddl.py:138).
func Disconnect(args Args) Expression { return newNode(KindDisconnect, args) }

// Nearest is the `NEAREST n BY DISTANCE|SIMILARITY expr` join modifier
// (expressions/query.py:825).
func Nearest(args Args) Expression { return newNode(KindNearest, args) }
