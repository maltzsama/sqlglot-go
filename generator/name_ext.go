package generator

import "github.com/maltzsama/sqlglot-go/expressions"

// Function classes whose upstream SQL name diverges from the naive camelToSnake split.
func init() {
	sqlNameOverrides[expressions.KindAIAgg] = "AI_AGG"
	sqlNameOverrides[expressions.KindAIClassify] = "AI_CLASSIFY"
	sqlNameOverrides[expressions.KindAISummarizeAgg] = "AI_SUMMARIZE_AGG"
	sqlNameOverrides[expressions.KindXMLGet] = "XMLGET"
}
