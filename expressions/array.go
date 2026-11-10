package expressions

func Array(args Args) Expression   { return newNode(KindArray, args) }
func Struct(args Args) Expression  { return newNode(KindStruct, args) }
func Unnest(args Args) Expression  { return newNode(KindUnnest, args) }
func Bracket(args Args) Expression { return newNode(KindBracket, args) }

// List ports exp.List (array.py:35-37): the LIST constructor.
func List(args Args) Expression { return newNode(KindList, args) }

// ToMap ports exp.ToMap (array.py:340-341): materialize's MAP[...] / MAP(...) constructor.
func ToMap(args Args) Expression { return newNode(KindToMap, args) }
