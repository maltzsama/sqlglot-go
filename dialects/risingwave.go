package dialects

import (
	"github.com/maltzsama/sqlglot-go/tokens"
)

// RisingWave ports the RisingWave dialect class (dialects/risingwave.py:9-24). It extends
// Postgres; the parser and generator override chains fall back to the postgres tables.
func RisingWave() *Dialect {
	d := Postgres()
	d.Name = "risingwave"

	// dialects/risingwave.py:10 NORMALIZE_NOT_NULL = True (postgres.py:16 overrides False).
	d.NormalizeNotNull = true
	// dialects/risingwave.py:12-13.
	d.RequiresParenthesizedStructAccess = true
	d.SupportsStructStarExpansion = true

	// generators/risingwave.py:9-10.
	d.LockingReadsSupported = false
	// generators/postgres.py:280 SUPPORTS_BETWEEN_FLAGS = True; risingwave re-overrides it
	// back to the base default (generators/risingwave.py:10).
	d.SupportsBetweenFlags = false
	// generators/risingwave.py:22: `CREATE SINK ... AS <select> WITH (...)` renders the
	// expression before the properties.
	d.ExpressionPrecedesPropertiesCreatables = map[string]bool{"SINK": true}
	// parsers/risingwave.py:11 WRAPPED_TRANSFORM_COLUMN_CONSTRAINT = False: `col INT AS
	// expr` parses without parentheses around the transform expression.
	d.WrappedTransformColumnConstraint = false

	// dialects/risingwave.py:15-20 Tokenizer.KEYWORDS: SINK/SOURCE become real keywords so
	// CREATE SINK / CREATE SOURCE tokenize as creatables rather than identifiers. Postgres()'s
	// config carries its own fresh Keywords map (tokens.BaseConfig builds one per call), so
	// mutating it here cannot leak into the cached template.
	cfg := d.TokenizerConfig
	cfg.Keywords["SINK"] = tokens.SINK
	cfg.Keywords["SOURCE"] = tokens.SOURCE
	d.TokenizerConfig = tokens.CompileConfig(cfg)

	return d
}
