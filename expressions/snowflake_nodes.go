package expressions

// Builders for the Snowflake node kinds.

func ArrayPosition(args Args) Expression     { return newNode(KindArrayPosition, args) }
func DotProduct(args Args) Expression        { return newNode(KindDotProduct, args) }
func LogicalAnd(args Args) Expression        { return newNode(KindLogicalAnd, args) }
func ManhattanDistance(args Args) Expression { return newNode(KindManhattanDistance, args) }
func Skewness(args Args) Expression          { return newNode(KindSkewness, args) }
func StrtokToArray(args Args) Expression     { return newNode(KindStrtokToArray, args) }
func Stuff(args Args) Expression             { return newNode(KindStuff, args) }
func ToArray(args Args) Expression           { return newNode(KindToArray, args) }
func Booland(args Args) Expression           { return newNode(KindBooland, args) }
func Boolor(args Args) Expression            { return newNode(KindBoolor, args) }
func YearOfWeek(args Args) Expression        { return newNode(KindYearOfWeek, args) }
func ToBinary(args Args) Expression          { return newNode(KindToBinary, args) }
func ToBoolean(args Args) Expression         { return newNode(KindToBoolean, args) }
func ToDouble(args Args) Expression          { return newNode(KindToDouble, args) }
func ToFile(args Args) Expression            { return newNode(KindToFile, args) }
func TimeSlice(args Args) Expression         { return newNode(KindTimeSlice, args) }

func Undrop(args Args) Expression { return newNode(KindUndrop, args) }

func TimeAdd(args Args) Expression  { return newNode(KindTimeAdd, args) }
func TimeDiff(args Args) Expression { return newNode(KindTimeDiff, args) }
