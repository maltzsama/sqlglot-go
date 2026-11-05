package expressions

// ClickHouse nodes, ported from .reference/sqlglot-v30.17.0/sqlglot/expressions/.
// Builders follow the same one-line newNode convention as the other dialect files.

func ArrayDistinct(args Args) Expression          { return newNode(KindArrayDistinct, args) }
func ArrayConcat(args Args) Expression            { return newNode(KindArrayConcat, args) }
func ArrayFilter(args Args) Expression            { return newNode(KindArrayFilter, args) }
func ArrayRemove(args Args) Expression            { return newNode(KindArrayRemove, args) }
func ArrayReverse(args Args) Expression           { return newNode(KindArrayReverse, args) }
func ArraySum(args Args) Expression               { return newNode(KindArraySum, args) }
func ArrayMax(args Args) Expression               { return newNode(KindArrayMax, args) }
func ArrayMin(args Args) Expression               { return newNode(KindArrayMin, args) }
func ArgMax(args Args) Expression                 { return newNode(KindArgMax, args) }
func ArgMin(args Args) Expression                 { return newNode(KindArgMin, args) }
func CityHash64(args Args) Expression             { return newNode(KindCityHash64, args) }
func CurrentDatabase(args Args) Expression        { return newNode(KindCurrentDatabase, args) }
func CurrentSchemas(args Args) Expression         { return newNode(KindCurrentSchemas, args) }
func CosineDistance(args Args) Expression         { return newNode(KindCosineDistance, args) }
func DateSub(args Args) Expression                { return newNode(KindDateSub, args) }
func Explode(args Args) Expression                { return newNode(KindExplode, args) }
func FarmFingerprint(args Args) Expression        { return newNode(KindFarmFingerprint, args) }
func IsNan(args Args) Expression                  { return newNode(KindIsNan, args) }
func JarowinklerSimilarity(args Args) Expression  { return newNode(KindJarowinklerSimilarity, args) }
func JSONPathKey(args Args) Expression            { return newNode(KindJSONPathKey, args) }
func JSONPathRoot(args Args) Expression           { return newNode(KindJSONPathRoot, args) }
func JSONPathSubscript(args Args) Expression      { return newNode(KindJSONPathSubscript, args) }
func Map(args Args) Expression                    { return newNode(KindMap, args) }
func Median(args Args) Expression                 { return newNode(KindMedian, args) }
func Nullif(args Args) Expression                 { return newNode(KindNullif, args) }
func Rand(args Args) Expression                   { return newNode(KindRand, args) }
func StartsWith(args Args) Expression             { return newNode(KindStartsWith, args) }
func EndsWith(args Args) Expression               { return newNode(KindEndsWith, args) }
func EuclideanDistance(args Args) Expression      { return newNode(KindEuclideanDistance, args) }
func TimestampSub(args Args) Expression           { return newNode(KindTimestampSub, args) }
func Typeof(args Args) Expression                 { return newNode(KindTypeof, args) }
func ParseDatetime(args Args) Expression          { return newNode(KindParseDatetime, args) }
func Detach(args Args) Expression                 { return newNode(KindDetach, args) }
func OnCluster(args Args) Expression              { return newNode(KindOnCluster, args) }
func PartitionId(args Args) Expression            { return newNode(KindPartitionId, args) }
func ReplacePartition(args Args) Expression       { return newNode(KindReplacePartition, args) }
func ProjectionDef(args Args) Expression          { return newNode(KindProjectionDef, args) }
func NestedJSONSelect(args Args) Expression       { return newNode(KindNestedJSONSelect, args) }
func AutoRefreshProperty(args Args) Expression    { return newNode(KindAutoRefreshProperty, args) }
func UuidProperty(args Args) Expression           { return newNode(KindUuidProperty, args) }
func Apply(args Args) Expression                  { return newNode(KindApply, args) }
func Columns(args Args) Expression                { return newNode(KindColumns, args) }
func AssumeColumnConstraint(args Args) Expression { return newNode(KindAssumeColumnConstraint, args) }
func AlterModifySqlSecurity(args Args) Expression { return newNode(KindAlterModifySqlSecurity, args) }
func CombinedAggFunc(args Args) Expression        { return newNode(KindCombinedAggFunc, args) }
func ParameterizedAgg(args Args) Expression       { return newNode(KindParameterizedAgg, args) }
func AnonymousAggFunc(args Args) Expression       { return newNode(KindAnonymousAggFunc, args) }
func CombinedParameterizedAgg(args Args) Expression {
	return newNode(KindCombinedParameterizedAgg, args)
}
func FunctionSpecification(args Args) Expression { return newNode(KindFunctionSpecification, args) }
func SettingsProperty(args Args) Expression      { return newNode(KindSettingsProperty, args) }

func MergeTreeTTL(args Args) Expression       { return newNode(KindMergeTreeTTL, args) }
func MergeTreeTTLAction(args Args) Expression { return newNode(KindMergeTreeTTLAction, args) }
func DictProperty(args Args) Expression       { return newNode(KindDictProperty, args) }
func DictSubProperty(args Args) Expression    { return newNode(KindDictSubProperty, args) }
func DictRange(args Args) Expression          { return newNode(KindDictRange, args) }
func SampleProperty(args Args) Expression     { return newNode(KindSampleProperty, args) }

func ArrayCompact(args Args) Expression   { return newNode(KindArrayCompact, args) }
func ArrayExcept(args Args) Expression    { return newNode(KindArrayExcept, args) }
func SubstringIndex(args Args) Expression { return newNode(KindSubstringIndex, args) }
func UtcTimestamp(args Args) Expression   { return newNode(KindUtcTimestamp, args) }

func WithFill(args Args) Expression       { return newNode(KindWithFill, args) }
func SkipJSONColumn(args Args) Expression { return newNode(KindSkipJSONColumn, args) }
