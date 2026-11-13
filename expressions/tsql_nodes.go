package expressions

// tsql_nodes.go: builders for the nodes the T-SQL dialect parses into. One-file-per-slice,
// mirroring presto_nodes.go / hive_nodes.go. Each Kind's arg_types/traits/className rows live
// in kinds.go; the builders here mirror the functions.go one-liner style.

// Execute ports exp.Execute (expressions/ddl.py:436): EXECUTE <procedure> <args>, optionally
// capturing a return-status parameter (`@rc = EXEC ...`).
func Execute(args Args) Expression { return newNode(KindExecute, args) }

// ExecuteSql ports exp.ExecuteSql (expressions/ddl.py:444): sp_executesql.
func ExecuteSql(args Args) Expression { return newNode(KindExecuteSql, args) }

// ScopeResolution ports exp.ScopeResolution (expressions/query.py:2081): T-SQL's `::`
// scope-resolution operator (`schema::function`).
func ScopeResolution(args Args) Expression { return newNode(KindScopeResolution, args) }

// QueryOption ports exp.QueryOption (expressions/query.py:914): one OPTION (...) / FOR XML|JSON
// option.
func QueryOption(args Args) Expression { return newNode(KindQueryOption, args) }

// XMLKeyValueOption ports exp.XMLKeyValueOption (expressions/query.py:2097): a FOR XML option
// with an optional parenthesized value.
func XMLKeyValueOption(args Args) Expression { return newNode(KindXMLKeyValueOption, args) }

// ForClause ports exp.ForClause (expressions/query.py:919): the FOR XML / FOR JSON / FOR BROWSE
// query modifier.
func ForClause(args Args) Expression { return newNode(KindForClause, args) }

// JSONArrayAgg ports exp.JSONArrayAgg (expressions/json.py:22).
func JSONArrayAgg(args Args) Expression { return newNode(KindJSONArrayAgg, args) }

// ConvertExpr ports exp.Convert (expressions/functions.py:89): CONVERT(type, expr[, style]).
// (Convert is already the value-coercion helper in builders.go.)
func ConvertExpr(args Args) Expression { return newNode(KindConvert, args) }

// TimeFromParts ports exp.TimeFromParts (expressions/temporal.py:367).
func TimeFromParts(args Args) Expression { return newNode(KindTimeFromParts, args) }

// TimestampFromParts ports exp.TimestampFromParts (expressions/temporal.py:384).
func TimestampFromParts(args Args) Expression { return newNode(KindTimestampFromParts, args) }

// DateFromParts ports exp.DateFromParts (expressions/temporal.py:293).
func DateFromParts(args Args) Expression { return newNode(KindDateFromParts, args) }

// SplitPart ports exp.SplitPart (expressions/string.py:174).
func SplitPart(args Args) Expression { return newNode(KindSplitPart, args) }

// StoredProcedure ports exp.StoredProcedure (expressions/query.py:2115).
func StoredProcedure(args Args) Expression { return newNode(KindStoredProcedure, args) }

// LastDay ports exp.LastDay (expressions/temporal.py:338).
func LastDay(args Args) Expression { return newNode(KindLastDay, args) }

// Atan2 ports exp.Atan2 (expressions/math.py:35).
func Atan2(args Args) Expression { return newNode(KindAtan2, args) }

// Uuid ports exp.Uuid (expressions/functions.py:534).
func Uuid(args Args) Expression { return newNode(KindUuid, args) }

// Repeat ports exp.Repeat (expressions/string.py:109).
func Repeat(args Args) Expression { return newNode(KindRepeat, args) }

// Left ports exp.Left (expressions/string.py:65).
func Left(args Args) Expression { return newNode(KindLeft, args) }

// Right ports exp.Right (expressions/string.py:121).
func Right(args Args) Expression { return newNode(KindRight, args) }

// SHA ports exp.SHA (expressions/string.py:569).
func SHA(args Args) Expression { return newNode(KindSHA, args) }

// SHA1Digest ports exp.SHA1Digest (expressions/string.py:573).
func SHA1Digest(args Args) Expression { return newNode(KindSHA1Digest, args) }

// CurrentTimestampLTZ ports exp.CurrentTimestampLTZ (expressions/temporal.py:41).
func CurrentTimestampLTZ(args Args) Expression { return newNode(KindCurrentTimestampLTZ, args) }

// DateStrToDate ports exp.DateStrToDate (expressions/temporal.py:420).
func DateStrToDate(args Args) Expression { return newNode(KindDateStrToDate, args) }

// TimestampAdd ports exp.TimestampAdd (expressions/temporal.py:132).
func TimestampAdd(args Args) Expression { return newNode(KindTimestampAdd, args) }

// TsOrDsDiff ports exp.TsOrDsDiff (expressions/temporal.py:154).
func TsOrDsDiff(args Args) Expression { return newNode(KindTsOrDsDiff, args) }

// NumberToStr ports exp.NumberToStr (expressions/string.py:97).
func NumberToStr(args Args) Expression { return newNode(KindNumberToStr, args) }

// IsAscii ports exp.IsAscii (expressions/string.py:61).
func IsAscii(args Args) Expression { return newNode(KindIsAscii, args) }

// NextValueFor ports exp.NextValueFor (expressions/ddl.py:432).
func NextValueFor(args Args) Expression { return newNode(KindNextValueFor, args) }
func PrevValueFor(args Args) Expression { return newNode(KindPrevValueFor, args) }

// ClusteredColumnConstraint ports exp.ClusteredColumnConstraint
// (expressions/constraints.py:64).
func ClusteredColumnConstraint(args Args) Expression {
	return newNode(KindClusteredColumnConstraint, args)
}

// NonClusteredColumnConstraint ports exp.NonClusteredColumnConstraint
// (expressions/constraints.py:139).
func NonClusteredColumnConstraint(args Args) Expression {
	return newNode(KindNonClusteredColumnConstraint, args)
}

// OpenJSON ports exp.OpenJSON (expressions/json.py:235).
func OpenJSON(args Args) Expression { return newNode(KindOpenJSON, args) }

// OpenJSONColumnDef ports exp.OpenJSONColumnDef (expressions/query.py:2070).
func OpenJSONColumnDef(args Args) Expression { return newNode(KindOpenJSONColumnDef, args) }

// PeriodForSystemTimeConstraint ports exp.PeriodForSystemTimeConstraint
// (expressions/constraints.py:44).
func PeriodForSystemTimeConstraint(args Args) Expression {
	return newNode(KindPeriodForSystemTimeConstraint, args)
}

// WithSystemVersioningProperty ports exp.WithSystemVersioningProperty
// (expressions/properties.py:572).
func WithSystemVersioningProperty(args Args) Expression {
	return newNode(KindWithSystemVersioningProperty, args)
}

// DataDeletionProperty ports exp.DataDeletionProperty (expressions/properties.py:544).
func DataDeletionProperty(args Args) Expression { return newNode(KindDataDeletionProperty, args) }

// VolatileProperty ports exp.VolatileProperty (expressions/properties.py:556).
func VolatileProperty(args Args) Expression { return newNode(KindVolatileProperty, args) }
