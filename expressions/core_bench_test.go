package expressions

import (
	"fmt"
	"testing"
)

var benchmarkExpressionSink Expression

func benchmarkAST(width int, withMetadata bool) Expression {
	projections := make([]Expression, width)
	var predicate Expression
	for i := range width {
		name := fmt.Sprintf("column_%d", i)
		column := Column(Args{"this": Identifier(Args{"this": name})})
		projections[i] = column
		condition := EQ(Args{
			"this":       Column(Args{"this": Identifier(Args{"this": name})}),
			"expression": Literal(Args{"this": fmt.Sprint(i), "is_string": false}),
		})
		if predicate == nil {
			predicate = condition
		} else {
			predicate = And(Args{"this": predicate, "expression": condition})
		}
	}
	query := Select(Args{"expressions": projections})
	query.Set("where", Where(Args{"this": predicate}))
	if withMetadata {
		query.Meta()["benchmark"] = []string{"copy", "isolation"}
		query.AddComments([]string{" representative benchmark query "}, false)
		query.SetSpan(1, width*10)
	}
	return query
}

func BenchmarkNewNode(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchmarkExpressionSink = Column(Args{"this": Identifier(Args{"this": "column_name"})})
	}
}

func BenchmarkNodeCopy(b *testing.B) {
	for _, width := range []int{4, 32, 128} {
		b.Run(fmt.Sprintf("width=%d", width), func(b *testing.B) {
			expr := benchmarkAST(width, false)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkExpressionSink = expr.Copy()
			}
		})
	}
	b.Run("width=32_metadata_comments", func(b *testing.B) {
		expr := benchmarkAST(32, true)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchmarkExpressionSink = expr.Copy()
		}
	})
}
