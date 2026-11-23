package expressions

// This file holds the node builders for constructs that exist only in the ksqlDB dialect
// (a beyond-upstream dialect — see DEVIATIONS.md "Beyond-upstream dialects"). The Kind
// itself, its arg table and its class name live next to every other Kind in kinds.go.

// KsqlWindow builds the ksqlDB WINDOW clause: KindKsqlWindow carries the window kind
// (TUMBLING, HOPPING or SESSION), its SIZE duration, the HOPPING ADVANCE BY, and the
// optional RETENTION / GRACE PERIOD clauses. Every duration is a bare `1 HOUR` Interval.
func KsqlWindow(args Args) Expression { return newNode(KindKsqlWindow, args) }
