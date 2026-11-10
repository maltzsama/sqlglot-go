package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// implicitUnnestsToExplicit ports _implicit_unnests_to_explicit (parser.py:4305-4330). In a
// dialect with SUPPORTS_IMPLICIT_UNNEST, a join whose left side is a multi-part table whose
// first part names an already-introduced relation is a column-unnesting join, not a real table
// join: `FROM x AS a, a.b AS c` means "unnest the array in a.b as c". It is rewritten into an
// explicit Unnest so other dialects render UNNEST(...) / LATERAL VIEW EXPLODE instead of a bare
// column reference.
//
// Upstream imports normalize_identifiers lazily inside the function (parser.py:4306) to dodge a
// module-level cycle; Go has no lazy imports, so the shared walker hangs off the Dialect instead
// (Dialect.NormalizeIdentifiers) and the parser needs no new package edge.
func (p *Parser) implicitUnnestsToExplicit(this exp.Expression) exp.Expression {
	from := asExpressionArg(this, "from_")
	if from == nil || from.This() == nil {
		return this
	}

	// The relations already in scope start with the FROM table; every join below contributes
	// its normalized alias or name, converted or not.
	refs := map[string]bool{
		p.dialect.NormalizeIdentifiers(from.This().Copy()).AliasOrName(): true,
	}

	joins, _ := this.Arg("joins").([]exp.Expression)
	for _, join := range joins {
		table := join.This()
		if table == nil {
			continue
		}
		normalizedTable := table.Copy()
		// parser.py:4312: the flag marks the copy as "a table that might be a column"; this port
		// never produces it downstream (only BigQuery's generator reads it, parsers/bigquery.py
		// is not ported yet), but the meta rides along on the throwaway copy as upstream does.
		normalizedTable.Meta()["maybe_column"] = true
		normalizedTable = p.dialect.NormalizeIdentifiers(normalizedTable)

		// upstream: if isinstance(table, exp.Table) and not join.args.get("on")
		if table.Kind() == exp.KindTable && join.Arg("on") == nil {
			if parts := normalizedTable.Parts(); len(parts) > 1 && refs[parts[0].Name()] {
				tableAsColumn := table.ToColumn()
				unnest := exp.Unnest(exp.Args{"expressions": []exp.Expression{tableAsColumn}})

				// Table.to_column creates a parent Alias node that we want to convert to
				// a TableAlias and attach to the Unnest, so it matches the parser's output
				// (parser.py:4317-4322).
				if alias := asExpressionArg(table, "alias"); alias != nil && alias.Kind() == exp.KindTableAlias {
					tableAsColumn.Replace(tableAsColumn.This())
					// exp.alias_(unnest, None, table=[table.args["alias"].this], copy=False):
					// a TableAlias with no `this` carrying the table alias as its column list.
					var columns []exp.Expression
					if ident := alias.This(); ident != nil {
						columns = append(columns, ident.Copy())
					}
					unnest.Set("alias", exp.TableAlias(exp.Args{"columns": columns}))
				}
				table.Replace(unnest)
			}
		}
		refs[normalizedTable.AliasOrName()] = true
	}
	return this
}
