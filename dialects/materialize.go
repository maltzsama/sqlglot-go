package dialects

// Materialize ports the Materialize dialect class (dialects/materialize.py:8-13). It extends
// Postgres; the parser and generator override chains fall back to the postgres tables.
func Materialize() *Dialect {
	d := Postgres()
	d.Name = "materialize"

	// dialects/materialize.py:9 NORMALIZE_NOT_NULL = True (postgres.py:16 overrides False).
	d.NormalizeNotNull = true

	// generators/materialize.py:15-16.
	d.SupportsCreateTableLike = false
	// generators/postgres.py:280 SUPPORTS_BETWEEN_FLAGS = True; materialize re-overrides it
	// back to the base default (generators/materialize.py:16).
	d.SupportsBetweenFlags = false

	return d
}
