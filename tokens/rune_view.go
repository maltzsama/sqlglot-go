package tokens

import "unicode/utf8"

// runeView presents the tokenizer input as a rune sequence. ASCII input — the overwhelming
// common case — is indexed straight out of the string: no []rune conversion, and slicing
// returns a substring instead of copying. Non-ASCII input falls back to []rune so the scanner
// keeps rune-indexed arithmetic, and invalid UTF-8 keeps the []rune replacement behavior it
// had before.
type runeView struct {
	ascii string
	runes []rune
}

func newRuneView(sql string) runeView {
	for i := 0; i < len(sql); i++ {
		if sql[i] >= utf8.RuneSelf {
			return runeView{runes: []rune(sql)}
		}
	}
	return runeView{ascii: sql}
}

func (v runeView) len() int {
	if v.runes != nil {
		return len(v.runes)
	}
	return len(v.ascii)
}

func (v runeView) at(i int) rune {
	if v.runes != nil {
		return v.runes[i]
	}
	return rune(v.ascii[i])
}

// slice returns the [start, end) range as a string. Both representations are rune-indexed.
func (v runeView) slice(start, end int) string {
	if v.runes != nil {
		return string(v.runes[start:end])
	}
	return v.ascii[start:end]
}

func indexRuneFrom(haystack runeView, needle rune, start, end int) int {
	if start < 0 {
		start = 0
	}
	if end > haystack.len() {
		end = haystack.len()
	}
	for i := start; i < end; i++ {
		if haystack.at(i) == needle {
			return i
		}
	}
	return -1
}

func countRune(haystack runeView, needle rune, start, end int) int {
	count := 0
	for i := start; i < end; i++ {
		if haystack.at(i) == needle {
			count++
		}
	}
	return count
}

func lastIndexRune(haystack runeView, needle rune, start, end int) int {
	for i := end - 1; i >= start; i-- {
		if haystack.at(i) == needle {
			return i
		}
	}
	return -1
}
