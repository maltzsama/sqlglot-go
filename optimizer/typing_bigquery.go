package optimizer

// BigQuery's EXPRESSION_METADATA overrides (sqlglot/typing/bigquery.py). The generated
// typing_metadata.go covers the dialects listed in scripts/gen_typing_metadata.py's
// GO_DIALECTS; bigquery is not in that list yet, so the entries the JSON-path work needs are
// registered here. Extending GO_DIALECTS and regenerating is the follow-up for the annotate
// gaps burndown, which will fold these rows into the generated table.
func init() {
	registerTypingMetadata("bigquery", map[string]typeSpec{
		// typing/bigquery.py:210-225: JSONExtract keeps its document argument's type, so
		// JSON_EXTRACT('{"a": 1}', '$.a') is VARCHAR rather than UNKNOWN now that the path is
		// a JSONPath node instead of a string literal.
		"JSONExtract": {annotator: annByArgs(byKeys("this"))},
		// typing/bigquery.py:325-340: JSONExtractScalar returns VARCHAR.
		"JSONExtractScalar": {returns: dtype("VARCHAR")},
		// typing/bigquery.py:344-356: JSONExtractArray annotates by its document argument and
		// wraps the result in an ARRAY.
		"JSONExtractArray": {annotator: annByArgs(annotateArgs{keys: []string{"this"}, array: true})},
	})
}
