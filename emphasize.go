package vascula

import (
	"html"
	"strings"
	"unicode"
)

// maxEmphasisWords bounds the words one emphasize call looks for.
const maxEmphasisWords = 32

// emphasize escapes text and wraps the listed words in <em>. Words are given
// as one string (separated by spaces or commas) or as an array. A word matches
// as a whole word, ignoring case, in any script; in scripts written without
// spaces between words (Chinese, Japanese, Thai, Lao, Khmer, Myanmar, Tibetan)
// it matches wherever it appears. Emphasised words with only spaces between
// them share one <em>. The printed text is the input text: only the <em>
// elements are added.
func emphasize(in interface{}, a FilterArgs) (interface{}, error) {
	text := []rune(toString(in))
	marked := make([]bool, len(text))
	for _, word := range emphasisWords(a.Arg(0)) {
		n := len(word)
		joined := unspacedScript(word)
		for i := 0; i+n <= len(text); i++ {
			if !runesFold(text[i:i+n], word) {
				continue
			}
			if !joined && ((i > 0 && wordRune(text[i-1])) || (i+n < len(text) && wordRune(text[i+n]))) {
				continue
			}
			for j := i; j < i+n; j++ {
				marked[j] = true
			}
		}
	}
	// Spaces between two emphasised words join them into one <em>.
	for i := 1; i < len(text); i++ {
		if marked[i] || !marked[i-1] || !unicode.IsSpace(text[i]) {
			continue
		}
		j := i
		for j < len(text) && unicode.IsSpace(text[j]) {
			j++
		}
		if j < len(text) && marked[j] {
			for k := i; k < j; k++ {
				marked[k] = true
			}
		}
		i = j
	}
	var b strings.Builder
	open := false
	for i, r := range text {
		if marked[i] != open {
			if open {
				b.WriteString("</em>")
			} else {
				b.WriteString("<em>")
			}
			open = marked[i]
		}
		b.WriteString(html.EscapeString(string(r)))
	}
	if open {
		b.WriteString("</em>")
	}
	return safeString(b.String()), nil
}

func emphasisWords(arg interface{}) [][]rune {
	var raw []string
	if list, ok := arg.([]interface{}); ok {
		for _, item := range list {
			raw = append(raw, toString(item))
		}
	} else if arg != nil {
		raw = []string{toString(arg)}
	}
	var words [][]rune
	for _, s := range raw {
		for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || unicode.IsSpace(r) }) {
			word := []rune(strings.TrimFunc(part, func(r rune) bool { return !wordRune(r) }))
			if len(word) == 0 {
				continue
			}
			if len(words) == maxEmphasisWords {
				return words
			}
			words = append(words, word)
		}
	}
	return words
}

func wordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r)
}

func unspacedScript(word []rune) bool {
	for _, r := range word {
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Thai, unicode.Lao, unicode.Khmer, unicode.Myanmar, unicode.Tibetan) {
			return true
		}
	}
	return false
}

func runesFold(a, b []rune) bool {
	for i := range a {
		if a[i] != b[i] && unicode.ToLower(a[i]) != unicode.ToLower(b[i]) && unicode.ToUpper(a[i]) != unicode.ToUpper(b[i]) {
			return false
		}
	}
	return true
}
