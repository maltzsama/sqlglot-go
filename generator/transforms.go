package generator

import (
	"strings"

	"github.com/maltzsama/sqlglot-go/expressions"
)

// Shared AST transforms ported from .reference/sqlglot-v30.17.0/sqlglot/transforms.py, used by
// the Materialize and Spark generators' `preprocess` entries.

// removeUniqueConstraints ports remove_unique_constraints (transforms.py:758-764): pop every
// UNIQUE column constraint wrapper from the CREATE body.
func removeUniqueConstraints(e expressions.Expression) expressions.Expression {
	for _, constraint := range e.FindAll(expressions.KindUniqueColumnConstraint) {
		if parent := constraint.Parent(); parent != nil {
			parent.Pop()
		}
	}
	return e
}

// ctasWithTmpTablesToCreateTmpView ports ctas_with_tmp_tables_to_create_tmp_view
// (transforms.py:767-788): CREATE TABLE with a TEMPORARY property and a body becomes
// CREATE TEMPORARY VIEW. provideTemporaryStorage selects Spark's temporary_storage_provider
// (generators/spark2.py:103-107), which turns a bodiless temporary CREATE TABLE into the same
// statement plus a `FORMAT=parquet` property; Materialize passes false (the identity provider).
func ctasWithTmpTablesToCreateTmpView(e expressions.Expression, provideTemporaryStorage bool) expressions.Expression {
	if e.Text("kind") != "TABLE" {
		return e
	}

	properties := asExpression(e.Arg("properties"))
	temporary := false
	if properties != nil {
		for _, prop := range properties.Expressions() {
			if prop.Kind() == expressions.KindTemporaryProperty {
				temporary = true
				break
			}
		}
	}
	if !temporary {
		return e
	}

	// `if expression.expression:` — an Expression is always truthy, so only a missing body
	// (a plain CREATE TABLE) reaches the storage provider.
	if body := asExpression(e.Arg("expression")); body != nil {
		return expressions.Create(expressions.Args{
			"kind":       "TEMPORARY VIEW",
			"this":       e.Arg("this"),
			"expression": body,
		})
	}
	if provideTemporaryStorage && properties != nil {
		properties.Append("expressions", expressions.FileFormatProperty(expressions.Args{
			"this": expressions.LiteralString("parquet"),
		}))
	}
	return e
}

// moveSchemaColumnsToPartitionedBy ports move_schema_columns_to_partitioned_by
// (transforms.py:791-815): a bare-name PARTITIONED BY list is folded into the schema, moving
// the matching column definitions into the property.
func moveSchemaColumnsToPartitionedBy(e expressions.Expression) expressions.Expression {
	schema := asExpression(e.Arg("this"))
	kind := e.Text("kind")
	if schema == nil || schema.Kind() != expressions.KindSchema || (kind != "TABLE" && kind != "VIEW") {
		return e
	}
	prop := e.Find(expressions.KindPartitionedByProperty)
	if prop == nil {
		return e
	}
	propThis := asExpression(prop.Arg("this"))
	if propThis == nil || propThis.Kind() == expressions.KindSchema {
		return e
	}

	columns := map[string]bool{}
	for _, v := range propThis.Expressions() {
		columns[strings.ToUpper(v.Name())] = true
	}
	schemaExprs := schema.Expressions()
	partitions := make([]expressions.Expression, 0, len(schemaExprs))
	rest := make([]expressions.Expression, 0, len(schemaExprs))
	for _, col := range schemaExprs {
		if columns[strings.ToUpper(col.Name())] {
			partitions = append(partitions, col)
		} else {
			rest = append(rest, col)
		}
	}
	schema.Set("expressions", rest)
	prop.Replace(expressions.PartitionedByProperty(expressions.Args{
		"this": expressions.Schema(expressions.Args{"expressions": partitions}),
	}))
	e.Set("this", schema)
	return e
}

// movePartitionedByToSchemaColumns ports move_partitioned_by_to_schema_columns
// (transforms.py:818-838): Spark 3+ uses the datasource CREATE TABLE format, so a PARTITIONED
// BY schema's column definitions move into the table schema and the property keeps bare names.
func movePartitionedByToSchemaColumns(e expressions.Expression) expressions.Expression {
	prop := e.Find(expressions.KindPartitionedByProperty)
	if prop == nil {
		return e
	}
	propThis := asExpression(prop.Arg("this"))
	if propThis == nil || propThis.Kind() != expressions.KindSchema {
		return e
	}
	columns := propThis.Expressions()
	if len(columns) == 0 {
		return e
	}
	for _, col := range columns {
		if col.Kind() != expressions.KindColumnDef || !truthy(col.Arg("kind")) {
			return e
		}
	}

	names := make([]expressions.Expression, 0, len(columns))
	for _, col := range columns {
		names = append(names, expressions.ToIdentifier(col.This().Name()))
	}
	schema := asExpression(e.Arg("this"))
	if schema == nil {
		return e
	}
	for _, col := range columns {
		schema.Append("expressions", col)
	}
	prop.Set("this", expressions.Tuple(expressions.Args{"expressions": names}))
	return e
}

// moveCTEsToTopLevel ports move_ctes_to_top_level (transforms.py:672-708): hoist every nested
// WITH onto the root statement, merging into an existing top-level WITH when present. Dialects
// that only accept top-level CTEs (Hive, Spark before 3, T-SQL) enable it through
// Dialect.MoveCTEsToTopLevel.
func moveCTEsToTopLevel(e expressions.Expression) expressions.Expression {
	topLevel := asExpression(e.Arg("with_"))
	for _, inner := range e.FindAll(expressions.KindWith) {
		if inner.Parent() == e {
			continue
		}
		if topLevel == nil {
			topLevel = inner.Pop()
			e.Set("with_", topLevel)
			continue
		}
		if truthy(inner.Arg("recursive")) {
			topLevel.Set("recursive", true)
		}
		parentCTE := inner.FindAncestor(expressions.KindCTE)
		inner.Pop()
		innerExpressions := inner.Expressions()

		index := -1
		if parentCTE != nil {
			for i, cte := range topLevel.Expressions() {
				if cte == parentCTE {
					index = i
					break
				}
			}
		}
		if index >= 0 {
			// Splice the nested CTEs in right before the CTE that contained them.
			for i, cte := range innerExpressions {
				topLevel.SetAt("expressions", cte, index+i, false)
			}
			continue
		}
		for _, cte := range innerExpressions {
			topLevel.Append("expressions", cte)
		}
	}
	return e
}

// removeWithinGroupForPercentiles ports remove_within_group_for_percentiles
// (transforms.py:570-584): `PERCENTILE_*(q) WITHIN GROUP (ORDER BY x)` becomes
// APPROX_QUANTILE(x, q). Upstream's PERCENTILES tuple is (PercentileCont, PercentileDisc); this
// port models only PercentileDisc, so PERCENTILE_CONT stays Anonymous and is left untouched.
func removeWithinGroupForPercentiles(e expressions.Expression) expressions.Expression {
	if e.Kind() != expressions.KindWithinGroup {
		return e
	}
	this := asExpression(e.Arg("this"))
	order := asExpression(e.Arg("expression"))
	if this == nil || this.Kind() != expressions.KindPercentileDisc || order == nil || order.Kind() != expressions.KindOrder {
		return e
	}
	ordered := e.Find(expressions.KindOrdered)
	if ordered == nil {
		return e
	}
	return e.Replace(expressions.ApproxQuantile(expressions.Args{
		"this":     asExpression(ordered.Arg("this")),
		"quantile": asExpression(this.Arg("this")),
	}))
}
