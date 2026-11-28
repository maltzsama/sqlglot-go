package expressions

// Builders for the Teradata conversion-attribute nodes (expressions/core.py:2351-2382).

// TeradataConversion is a postfix type conversion with a Teradata format (core.py:2351).
func TeradataConversion(args Args) Expression { return newNode(KindTeradataConversion, args) }

// TeradataTitle is a TITLE display heading, which does not rename the expression (core.py:2365).
func TeradataTitle(args Args) Expression { return newNode(KindTeradataTitle, args) }

// TeradataNamed is a NAMED alias on a subexpression (core.py:2379).
func TeradataNamed(args Args) Expression { return newNode(KindTeradataNamed, args) }
