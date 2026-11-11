package expressions

// Builders for the BigQuery node kinds (expressions/{aggregate,array,core,dml,functions,json,
// string,temporal}.py). They follow the same newNode convention as the other node files.

func ApproxTopK(args Args) Expression      { return newNode(KindApproxTopK, args) }
func AIForecast(args Args) Expression      { return newNode(KindAIForecast, args) }
func AIEmbed(args Args) Expression         { return newNode(KindAIEmbed, args) }
func AIGenerate(args Args) Expression      { return newNode(KindAIGenerate, args) }
func AISimilarity(args Args) Expression    { return newNode(KindAISimilarity, args) }
func ByteLength(args Args) Expression      { return newNode(KindByteLength, args) }
func Contains(args Args) Expression        { return newNode(KindContains, args) }
func Length(args Args) Expression          { return newNode(KindLength, args) }
func Lower(args Args) Expression           { return newNode(KindLower, args) }
func CurrentDatetime(args Args) Expression { return newNode(KindCurrentDatetime, args) }
func Datetime(args Args) Expression        { return newNode(KindDatetime, args) }
func ExplodingGenerateSeries(args Args) Expression {
	return newNode(KindExplodingGenerateSeries, args)
}
func Export(args Args) Expression            { return newNode(KindExport, args) }
func FeaturesAtTime(args Args) Expression    { return newNode(KindFeaturesAtTime, args) }
func ForIn(args Args) Expression             { return newNode(KindForIn, args) }
func GenerateBool(args Args) Expression      { return newNode(KindGenerateBool, args) }
func GenerateDouble(args Args) Expression    { return newNode(KindGenerateDouble, args) }
func GenerateEmbedding(args Args) Expression { return newNode(KindGenerateEmbedding, args) }
func GenerateInt(args Args) Expression       { return newNode(KindGenerateInt, args) }
func GenerateTable(args Args) Expression     { return newNode(KindGenerateTable, args) }
func GenerateText(args Args) Expression      { return newNode(KindGenerateText, args) }
func HavingMax(args Args) Expression         { return newNode(KindHavingMax, args) }
func Int64(args Args) Expression             { return newNode(KindInt64, args) }
func IntDiv(args Args) Expression            { return newNode(KindIntDiv, args) }
func JSONBool(args Args) Expression          { return newNode(KindJSONBool, args) }
func JSONExtractArray(args Args) Expression  { return newNode(KindJSONExtractArray, args) }
func JSONKeysAtDepth(args Args) Expression   { return newNode(KindJSONKeysAtDepth, args) }
func JSONStripNulls(args Args) Expression    { return newNode(KindJSONStripNulls, args) }
func JSONValueArray(args Args) Expression    { return newNode(KindJSONValueArray, args) }
func LowerHex(args Args) Expression          { return newNode(KindLowerHex, args) }
func MakeInterval(args Args) Expression      { return newNode(KindMakeInterval, args) }
func MLForecast(args Args) Expression        { return newNode(KindMLForecast, args) }
func MLTranslate(args Args) Expression       { return newNode(KindMLTranslate, args) }
func NetFunc(args Args) Expression           { return newNode(KindNetFunc, args) }
func Normalize(args Args) Expression         { return newNode(KindNormalize, args) }
func ParseTime(args Args) Expression         { return newNode(KindParseTime, args) }
func Predict(args Args) Expression           { return newNode(KindPredict, args) }
func SafeFunc(args Args) Expression          { return newNode(KindSafeFunc, args) }
func Time(args Args) Expression              { return newNode(KindTime, args) }
func Timestamp(args Args) Expression         { return newNode(KindTimestamp, args) }
func TsOrDsToDatetime(args Args) Expression  { return newNode(KindTsOrDsToDatetime, args) }
func TsOrDsToTime(args Args) Expression      { return newNode(KindTsOrDsToTime, args) }
func TsOrDsToTimestamp(args Args) Expression {
	return newNode(KindTsOrDsToTimestamp, args)
}
func UnixDate(args Args) Expression     { return newNode(KindUnixDate, args) }
func VectorSearch(args Args) Expression { return newNode(KindVectorSearch, args) }
func WeekStart(args Args) Expression    { return newNode(KindWeekStart, args) }
