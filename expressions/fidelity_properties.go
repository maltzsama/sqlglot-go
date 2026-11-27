package expressions

// Builders for CREATE/ALTER property nodes from expressions/properties.py.
func Property(args Args) Expression               { return newNode(KindProperty, args) }
func AlgorithmProperty(args Args) Expression      { return newNode(KindAlgorithmProperty, args) }
func AutoIncrementProperty(args Args) Expression  { return newNode(KindAutoIncrementProperty, args) }
func CollateProperty(args Args) Expression        { return newNode(KindCollateProperty, args) }
func DefinerProperty(args Args) Expression        { return newNode(KindDefinerProperty, args) }
func EngineProperty(args Args) Expression         { return newNode(KindEngineProperty, args) }
func InheritsProperty(args Args) Expression       { return newNode(KindInheritsProperty, args) }
func LikeProperty(args Args) Expression           { return newNode(KindLikeProperty, args) }
func ModuleProperty(args Args) Expression         { return newNode(KindModuleProperty, args) }
func VirtualProperty(args Args) Expression        { return newNode(KindVirtualProperty, args) }
func LockProperty(args Args) Expression           { return newNode(KindLockProperty, args) }
func LockingProperty(args Args) Expression        { return newNode(KindLockingProperty, args) }
func MaterializedProperty(args Args) Expression   { return newNode(KindMaterializedProperty, args) }
func NoPrimaryIndexProperty(args Args) Expression { return newNode(KindNoPrimaryIndexProperty, args) }
func OnCommitProperty(args Args) Expression       { return newNode(KindOnCommitProperty, args) }
func OnProperty(args Args) Expression             { return newNode(KindOnProperty, args) }
func PartitionedByProperty(args Args) Expression  { return newNode(KindPartitionedByProperty, args) }
func PartitionByRangeProperty(args Args) Expression {
	return newNode(KindPartitionByRangeProperty, args)
}
func PartitionByListProperty(args Args) Expression { return newNode(KindPartitionByListProperty, args) }
func PartitionList(args Args) Expression           { return newNode(KindPartitionList, args) }
func PartitionBoundSpec(args Args) Expression      { return newNode(KindPartitionBoundSpec, args) }
func PartitionedOfProperty(args Args) Expression   { return newNode(KindPartitionedOfProperty, args) }
func SchemaCommentProperty(args Args) Expression   { return newNode(KindSchemaCommentProperty, args) }
func SqlReadWriteProperty(args Args) Expression    { return newNode(KindSqlReadWriteProperty, args) }
func TemporaryProperty(args Args) Expression       { return newNode(KindTemporaryProperty, args) }
func UnloggedProperty(args Args) Expression        { return newNode(KindUnloggedProperty, args) }
func WithDataProperty(args Args) Expression        { return newNode(KindWithDataProperty, args) }

// Teradata CREATE TABLE properties (properties.py: exp.SetProperty .. exp.VolatileProperty),
// dispatched from _parse_property_before / PROPERTY_PARSERS (parser.py:2855-2881, 3171-3240).
func SetProperty(args Args) Expression           { return newNode(KindSetProperty, args) }
func FallbackProperty(args Args) Expression      { return newNode(KindFallbackProperty, args) }
func JournalProperty(args Args) Expression       { return newNode(KindJournalProperty, args) }
func ChecksumProperty(args Args) Expression      { return newNode(KindChecksumProperty, args) }
func FreespaceProperty(args Args) Expression     { return newNode(KindFreespaceProperty, args) }
func DataBlocksizeProperty(args Args) Expression { return newNode(KindDataBlocksizeProperty, args) }
func BlockCompressionProperty(args Args) Expression {
	return newNode(KindBlockCompressionProperty, args)
}
func MergeBlockRatioProperty(args Args) Expression { return newNode(KindMergeBlockRatioProperty, args) }
func GlobalProperty(args Args) Expression          { return newNode(KindGlobalProperty, args) }
func IcebergProperty(args Args) Expression         { return newNode(KindIcebergProperty, args) }
func LogProperty(args Args) Expression             { return newNode(KindLogProperty, args) }
func IsolatedLoadingProperty(args Args) Expression { return newNode(KindIsolatedLoadingProperty, args) }
func WithJournalTableProperty(args Args) Expression {
	return newNode(KindWithJournalTableProperty, args)
}
