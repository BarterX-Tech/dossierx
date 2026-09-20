// markwords.go fills in WHICH WORDS moved inside a passage that was replaced.
//
// Blocks says a passage changed. On its own that leaves the reader comparing
// an old passage against a new one to find the difference by eye, which is
// the work the diff was supposed to do for them. MarkWords does it: the same
// LCS, one level down, over words rather than passages.
//
// It compares words and carries whitespace, which is what makes the result
// blind to re-wrapping. The only difference between a word at the end of a
// line and the same word mid-line is the gap after it, and the gap is not
// part of the comparison.
package textdiff

// MarkWords fills Parts on every adjacent removal-then-addition pair, and
// leaves every other hunk untouched.
//
// Each side is marked against the other and keeps its own text: the removal
// hunk's Parts are its own words, flagged OpEqual or OpRemove, and the
// addition hunk's are its own, flagged OpEqual or OpAdd. Concatenating either
// hunk's Parts reproduces that hunk's Text exactly — no hunk ever carries the
// other side's bytes, so nothing downstream has to know which side it is
// looking at to render it.
func MarkWords(hunks []Hunk) []Hunk {
	out := make([]Hunk, len(hunks))
	copy(out, hunks)
	for i := 0; i+1 < len(out); i++ {
		if out[i].Op != OpRemove || out[i+1].Op != OpAdd {
			continue
		}
		before, after, ok := markPair(out[i].Text, out[i+1].Text)
		if !ok {
			continue
		}
		out[i].Parts = before
		out[i+1].Parts = after
	}
	return out
}

// token is one word plus the whitespace that follows it, so concatenating
// every token reproduces the input byte for byte. word is the same token with
// that whitespace stripped, and it is what comparisons use.
type token struct {
	text string
	word string
}

func tokenize(s string) []token {
	var out []token
	i := 0
	for i < len(s) {
		start := i
		for i < len(s) && !isSpace(s[i]) {
			i++
		}
		word := s[start:i]
		for i < len(s) && isSpace(s[i]) {
			i++
		}
		if word == "" {
			// Leading whitespace with no word in front of it: attach it to
			// the previous token rather than dropping it, so the
			// concatenation property holds for text that starts with a gap.
			if len(out) > 0 {
				out[len(out)-1].text += s[start:i]
			} else {
				out = append(out, token{text: s[start:i]})
			}
			continue
		}
		out = append(out, token{text: s[start:i], word: word})
	}
	return out
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// markPair word-diffs one passage against the passage that replaced it. It
// reports false when the pair must be left unmarked — either side past
// MaxMarkWords, or too little in common to read as one sentence.
func markPair(before, after string) (beforeParts, afterParts []Part, ok bool) {
	a, b := tokenize(before), tokenize(after)
	if len(a) == 0 || len(b) == 0 || len(a) > MaxMarkWords || len(b) > MaxMarkWords {
		return nil, nil, false
	}
	n, m := len(a), len(b)
	lcs := make([]int16, (n+1)*(m+1))
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i].word == b[j].word {
				lcs[i*(m+1)+j] = lcs[(i+1)*(m+1)+j+1] + 1
				continue
			}
			down, right := lcs[(i+1)*(m+1)+j], lcs[i*(m+1)+j+1]
			if down >= right {
				lcs[i*(m+1)+j] = down
			} else {
				lcs[i*(m+1)+j] = right
			}
		}
	}
	larger := n
	if m > larger {
		larger = m
	}
	if float64(lcs[0]) < MinMarkSimilarity*float64(larger) {
		return nil, nil, false
	}

	push := func(parts *[]Part, op Op, text string) {
		if text == "" {
			return
		}
		if k := len(*parts) - 1; k >= 0 && (*parts)[k].Op == op {
			(*parts)[k].Text += text
			return
		}
		*parts = append(*parts, Part{Op: op, Text: text})
	}
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i].word == b[j].word:
			push(&beforeParts, OpEqual, a[i].text)
			push(&afterParts, OpEqual, b[j].text)
			i++
			j++
		case lcs[(i+1)*(m+1)+j] >= lcs[i*(m+1)+j+1]:
			push(&beforeParts, OpRemove, a[i].text)
			i++
		default:
			push(&afterParts, OpAdd, b[j].text)
			j++
		}
	}
	for ; i < n; i++ {
		push(&beforeParts, OpRemove, a[i].text)
	}
	for ; j < m; j++ {
		push(&afterParts, OpAdd, b[j].text)
	}
	return beforeParts, afterParts, true
}
