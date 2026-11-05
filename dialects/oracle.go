package dialects

import (
	"maps"
	"sort"
	"strings"

	"github.com/ridi-oss/sqlglot-go/tokens"
)

// Oracle ports the Oracle dialect class from
// .reference/sqlglot-v30.17.0/sqlglot/dialects/oracle.py:15-92.
func Oracle() *Dialect {
	d := Base()
	d.Name = "oracle"

	// dialects/oracle.py:16-22 class flags.
	d.AliasPostTablesample = true
	d.LockingReadsSupported = true
	d.TablesampleSizeIsPercent = true
	d.NullOrdering = "nulls_are_large"
	d.OnConditionEmptyBeforeError = false
	d.AlterTableAddRequiredForEachColumn = false
	d.DisablesAliasRefExpansion = true
	// dialects/oracle.py:25 NORMALIZATION_STRATEGY = UPPERCASE.
	d.NormalizationStrategy = Uppercase
	// dialects/oracle.py:52-53 PSEUDOCOLUMNS.
	d.Pseudocolumns = map[string]bool{
		"ROWNUM": true, "ROWID": true, "OBJECT_ID": true, "OBJECT_VALUE": true, "LEVEL": true,
	}
	// generators/oracle.py:23-42 generator flags.
	d.LimitFetch = "FETCH"
	d.TablesampleKeywords = "SAMPLE"
	d.SupportsSelectInto = true
	d.AlterTableIncludeColumnKeyword = false
	// dialect.py:345 derives SUPPORTS_COLUMN_JOIN_MARKS from "(+)" in the tokenizer keywords.
	d.SupportsColumnJoinMarks = true
	// generators/oracle.py:41-42.
	d.PivotAliasWithAs = false
	d.UnpivotAliasesAreIdentifiers = false

	cfg := d.TokenizerConfig
	// dialects/oracle.py:56-57 VAR_SINGLE_TOKENS.
	cfg.VarSingleTokens = maps.Clone(cfg.VarSingleTokens)
	cfg.VarSingleTokens['@'] = true
	cfg.VarSingleTokens['$'] = true
	cfg.VarSingleTokens['#'] = true
	// dialects/oracle.py:59-60 NESTED_COMMENTS = False.
	cfg.NestedComments = false
	// dialects/oracle.py:59-64 UNICODE_STRINGS: U'...' / u'...'.
	cfg.FormatStrings = maps.Clone(cfg.FormatStrings)
	cfg.FormatStrings["U'"] = tokens.FormatString{End: "'", TokenType: tokens.UNICODE_STRING}
	cfg.FormatStrings["u'"] = tokens.FormatString{End: "'", TokenType: tokens.UNICODE_STRING}

	// dialects/oracle.py:66-80 KEYWORDS deltas.
	cfg.Keywords = maps.Clone(cfg.Keywords)
	for keyword, tokenType := range map[string]tokens.TokenType{
		"(+)":               tokens.JOIN_MARKER,
		"^=":                tokens.NEQ,
		"BINARY_DOUBLE":     tokens.DOUBLE,
		"BINARY_FLOAT":      tokens.FLOAT,
		"BULK COLLECT INTO": tokens.BULK_COLLECT_INTO,
		"COLUMNS":           tokens.COLUMN,
		"MATCH_RECOGNIZE":   tokens.MATCH_RECOGNIZE,
		"MINUS":             tokens.EXCEPT,
		"NVARCHAR2":         tokens.NVARCHAR,
		"ORDER SIBLINGS BY": tokens.ORDER_SIBLINGS_BY,
		"SAMPLE":            tokens.TABLE_SAMPLE,
		"START":             tokens.BEGIN,
		"TOP":               tokens.TOP,
		"VARCHAR2":          tokens.VARCHAR,
		"SYSTIMESTAMP":      tokens.SYSTIMESTAMP,
	} {
		cfg.Keywords[keyword] = tokenType
	}
	d.TokenizerConfig = tokens.CompileConfig(cfg)

	// parsers/oracle.py:38-56 FUNCTIONS overlay.
	d.Functions = oracleFunctions()

	return d
}

// oracleTimeMapping ports Oracle.TIME_MAPPING (dialects/oracle.py:30-50).
var oracleTimeMapping = map[string]string{
	"D": "%u", "DAY": "%A", "DD": "%d", "DDD": "%j", "DY": "%a",
	"HH": "%I", "HH12": "%I", "HH24": "%H", "IW": "%V", "MI": "%M", "MM": "%m",
	"MON": "%b", "MONTH": "%B", "SS": "%S", "WW": "%W", "YY": "%y",
	"YYYY": "%Y", "FF6": "%f",
}

// oracleMappingOrder is the declaration order of Oracle.TIME_MAPPING; the inverse mapping
// keeps the LAST declaration for a duplicate target, as the Python dict comprehension does.
var oracleMappingOrder = []string{
	"D", "DAY", "DD", "DDD", "DY", "HH", "HH12", "HH24", "IW", "MI", "MM",
	"MON", "MONTH", "SS", "WW", "YY", "YYYY", "FF6",
}

var oracleInverseTimeMapping = func() map[string]string {
	inverse := map[string]string{}
	for _, key := range oracleMappingOrder {
		inverse[oracleTimeMapping[key]] = key
	}
	// dialect.py:152-161 _with_strict_time_inverse.
	for _, pair := range [][2]string{
		{"%mstrict", "%m"}, {"%dstrict", "%d"}, {"%Hstrict", "%H"},
		{"%Istrict", "%I"}, {"%Mstrict", "%M"}, {"%Sstrict", "%S"},
	} {
		strict, lax := pair[0], pair[1]
		if v, ok := inverse[strict]; ok {
			if _, exists := inverse[lax]; !exists {
				inverse[lax] = v
			}
		} else if v, ok := inverse[lax]; ok {
			inverse[strict] = v
		} else {
			inverse[strict] = lax
		}
	}
	return inverse
}()

func longestMatch(text string, keys []string) (string, int, bool) {
	best := ""
	bestLen := 0
	for _, key := range keys {
		if len(key) > bestLen && strings.HasPrefix(text, key) {
			best, bestLen = key, len(key)
		}
	}
	if bestLen == 0 {
		return "", 0, false
	}
	return best, bestLen, true
}

var oracleForwardKeys = func() []string {
	keys := make([]string, 0, len(oracleTimeMapping))
	for key := range oracleTimeMapping {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	return keys
}()

var oracleInverseKeys = func() []string {
	keys := make([]string, 0, len(oracleInverseTimeMapping))
	for key := range oracleInverseTimeMapping {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	return keys
}()

// OracleFormatTime converts an Oracle format string to a strftime one
// (dialect.py:1014-1025, _format_time).
func OracleFormatTime(format string) string {
	var b strings.Builder
	for format != "" {
		if key, n, ok := longestMatch(format, oracleForwardKeys); ok {
			b.WriteString(oracleTimeMapping[key])
			format = format[n:]
			continue
		}
		b.WriteByte(format[0])
		format = format[1:]
	}
	return b.String()
}

// OracleTimeFormat converts a strftime format string back to Oracle's spelling
// (Generator.format_time with INVERSE_TIME_MAPPING).
func OracleTimeFormat(format string) string {
	var b strings.Builder
	for format != "" {
		if key, n, ok := longestMatch(format, oracleInverseKeys); ok {
			b.WriteString(oracleInverseTimeMapping[key])
			format = format[n:]
			continue
		}
		b.WriteByte(format[0])
		format = format[1:]
	}
	return b.String()
}
