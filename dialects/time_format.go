package dialects

import "strings"

// time_format.go ports the generic time-format conversion shared by dialect TIME_MAPPING
// handling: format_time (sqlglot/time.py:10-62) and the inverse-mapping wiring the Dialect
// metaclass performs (dialects/dialect.py:142-161 STRICT_TIME_FORMATS / _with_strict_time_
// inverse, :261-271 INVERSE_TIME_MAPPING generation). Hive, Presto and TSQL predate this
// file with their own verified copies of the same algorithm; the Postgres family is the
// first consumer of this shared one.

// timeTrieNode is one node of the trie over mapping keys (sqlglot.trie new_trie).
type timeTrieNode struct {
	children map[rune]*timeTrieNode
	exists   bool
}

func newTimeTrie(mapping map[string]string) *timeTrieNode {
	root := &timeTrieNode{children: map[rune]*timeTrieNode{}}
	for pattern := range mapping {
		current := root
		for _, char := range pattern {
			if current.children[char] == nil {
				current.children[char] = &timeTrieNode{children: map[rune]*timeTrieNode{}}
			}
			current = current.children[char]
		}
		current.exists = true
	}
	return root
}

// strictTimeFormats is STRICT_TIME_FORMATS (dialects/dialect.py:142-149): lax/strict pairs
// whose inverse entries must degrade to each other so a foreign lax %m never leaks a format
// the dialect would render differently.
var strictTimeFormats = map[string]string{
	"%mstrict": "%m",
	"%dstrict": "%d",
	"%Hstrict": "%H",
	"%Istrict": "%I",
	"%Mstrict": "%M",
	"%Sstrict": "%S",
}

// withStrictTimeInverse ports _with_strict_time_inverse (dialects/dialect.py:152-161):
// Python's dict.setdefault semantics (never overwrite an existing inverse entry).
func withStrictTimeInverse(inverse map[string]string) map[string]string {
	for strict, lax := range strictTimeFormats {
		if value, ok := inverse[strict]; ok {
			if _, exists := inverse[lax]; !exists {
				inverse[lax] = value
			}
		} else if value, ok := inverse[lax]; ok {
			inverse[strict] = value
		} else {
			inverse[strict] = lax
		}
	}
	return inverse
}

// inverseTimeMapping builds INVERSE_TIME_MAPPING as
// `{v: k for k, v in TIME_MAPPING.items()} | class_dict_overrides` (dialect.py:265-269):
// resolved over the dialect's insertion order so duplicate values resolve to the LAST key,
// then passed through the strict-inverse pass.
func inverseTimeMapping(mapping map[string]string, order []string) map[string]string {
	inverse := make(map[string]string, len(mapping))
	for _, key := range order {
		inverse[mapping[key]] = key
	}
	return withStrictTimeInverse(inverse)
}

// FormatTime converts a format string through mapping, porting format_time
// (sqlglot/time.py:10-62); ok=false for the empty string (time.py:27 returns None). The
// trie is rebuilt per call: conversion happens only on the rare format-carrying function
// paths, never in a hot loop.
func FormatTime(value string, mapping map[string]string) (string, bool) {
	if value == "" {
		return "", false
	}
	return convertTimeFormat(value, mapping, newTimeTrie(mapping))
}

// convertTimeFormat ports time.py:10-62. In particular, it remembers the longest completed
// trie match (sym) and reprocesses the first character after that match on failure, then
// replaces every completed chunk through mapping (unmatched chunks pass through verbatim).
func convertTimeFormat(value string, mapping map[string]string, trie *timeTrieNode) (string, bool) {
	if value == "" {
		return "", false
	}

	characters := []rune(value)
	start, end := 0, 1
	current := trie
	chunks := make([]string, 0, len(characters))
	matchedSymbol := ""

	for end <= len(characters) {
		chars := string(characters[start:end])
		next, found := current.children[characters[end-1]]
		failed := !found
		if failed {
			if matchedSymbol != "" {
				end--
				chars = matchedSymbol
				matchedSymbol = ""
			} else {
				chars = string(characters[start])
				end = start + 1
			}
			start += len([]rune(chars))
			chunks = append(chunks, chars)
			current = trie
		} else {
			current = next
			if current.exists {
				matchedSymbol = chars
			}
		}

		end++
		if !failed && end > len(characters) {
			chunks = append(chunks, chars)
		}
	}

	var converted strings.Builder
	for _, chunk := range chunks {
		if replacement, ok := mapping[chunk]; ok {
			converted.WriteString(replacement)
		} else {
			converted.WriteString(chunk)
		}
	}
	return converted.String(), true
}
