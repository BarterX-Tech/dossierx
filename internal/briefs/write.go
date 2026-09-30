package briefs

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// write.go rewrites a brief's frontmatter in the two places the engine owns
// (NIT-205): the status line (`brief lock` / `brief unlock`) and the comments
// block (the comment ops). Every other byte of the file — the summary, rests_on,
// any YAML comment the author wrote, the body, the line endings — is kept as
// written, and every rewrite is re-parsed before it is returned: a rewrite that
// would change what the brief says, or that does not read back as intended, is
// an error and nothing is written.

// ErrFrontmatterNotRewritable is returned when the engine cannot set a field
// without rewriting bytes the author owns. The recovery is in the message.
var ErrFrontmatterNotRewritable = errors.New("briefs: the frontmatter cannot be rewritten in place")

// frontmatterLines splits raw into its lines (each keeping its line ending) and
// returns the index of the closing "---" line. The block is lines[1:end].
func frontmatterLines(raw []byte) (lines []string, end int, eol string, err error) {
	lines = strings.SplitAfter(string(raw), "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r\n") != "---" {
		return nil, 0, "", fmt.Errorf("%w: the file does not open with a --- frontmatter block", ErrFrontmatterNotRewritable)
	}
	eol = "\n"
	if strings.HasSuffix(lines[0], "\r\n") {
		eol = "\r\n"
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r\n") == "---" {
			return lines, i, eol, nil
		}
	}
	return nil, 0, "", fmt.Errorf("%w: the frontmatter block is never closed", ErrFrontmatterNotRewritable)
}

// frontmatterRoot parses the block and returns its top-level mapping, or nil for
// an empty block.
func frontmatterRoot(lines []string, end int) (*yaml.Node, error) {
	block := strings.ReplaceAll(strings.Join(lines[1:end], ""), "\r\n", "\n")
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(block), &doc); err != nil {
		return nil, fmt.Errorf("%w: the frontmatter is not valid YAML: %v", ErrFrontmatterNotRewritable, err)
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return nil, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: the frontmatter is not a mapping", ErrFrontmatterNotRewritable)
	}
	return root, nil
}

// SetStatus returns raw with the frontmatter's status set to status. An existing
// status line has its value token replaced in place; a brief with no status line
// gains one just before the closing ---. The result is re-parsed and must read
// back with the new status and an unchanged LockHash.
func SetStatus(raw []byte, status Status) ([]byte, error) {
	lines, end, eol, err := frontmatterLines(raw)
	if err != nil {
		return nil, err
	}
	root, err := frontmatterRoot(lines, end)
	if err != nil {
		return nil, err
	}
	var out []string
	replaced := false
	if root != nil {
		for i := 0; i+1 < len(root.Content); i += 2 {
			key, value := root.Content[i], root.Content[i+1]
			if key.Value != "status" {
				continue
			}
			// key.Line is 1-based within the block; the block starts at lines[1].
			li := key.Line
			if value.Line != key.Line || value.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("%w: status is not a one-line value; write a plain \"status: %s\" line and retry", ErrFrontmatterNotRewritable, status)
			}
			line := lines[li]
			quote := ""
			switch value.Style {
			case 0:
			case yaml.DoubleQuotedStyle:
				quote = `"`
			case yaml.SingleQuotedStyle:
				quote = "'"
			default:
				return nil, fmt.Errorf("%w: status is written in a style the engine does not rewrite; write a plain \"status: %s\" line and retry", ErrFrontmatterNotRewritable, status)
			}
			token := quote + value.Value + quote
			at := byteColumn(line, value.Column)
			if at < 0 || !strings.HasPrefix(line[at:], token) {
				return nil, fmt.Errorf("%w: the status value could not be located; write a plain \"status: %s\" line and retry", ErrFrontmatterNotRewritable, status)
			}
			out = append(append([]string{}, lines[:li]...), line[:at]+quote+string(status)+quote+line[at+len(token):])
			out = append(out, lines[li+1:]...)
			replaced = true
			break
		}
	}
	if !replaced {
		out = append(append([]string{}, lines[:end]...), "status: "+string(status)+eol)
		out = append(out, lines[end:]...)
	}
	result := []byte(strings.Join(out, ""))
	if err := verifyRewrite(raw, result, func(fm frontmatter) bool {
		return fm.status == string(status) || (fm.status == "" && status == StatusDraft)
	}); err != nil {
		return nil, err
	}
	return result, nil
}

// SetComments returns raw with the frontmatter's comments block replaced by
// threads (removed when threads is empty). An existing block is cut out — from
// its key line to the line before the next top-level key, or to the closing
// --- — and the new block is written just before the closing ---. The result is
// re-parsed and must read back with exactly these threads and an unchanged
// LockHash and status.
func SetComments(raw []byte, threads []model.Comment) ([]byte, error) {
	lines, end, eol, err := frontmatterLines(raw)
	if err != nil {
		return nil, err
	}
	root, err := frontmatterRoot(lines, end)
	if err != nil {
		return nil, err
	}
	from, to := -1, -1 // the cut, as indexes into lines: [from, to)
	if root != nil {
		for i := 0; i+1 < len(root.Content); i += 2 {
			if root.Content[i].Value != "comments" {
				continue
			}
			from = root.Content[i].Line
			to = end
			if i+2 < len(root.Content) {
				to = root.Content[i+2].Line
			}
			break
		}
	}
	kept := append([]string{}, lines[:end]...)
	if from >= 0 {
		kept = append(append([]string{}, lines[:from]...), lines[to:end]...)
	}
	if n := len(kept); n > 1 && !strings.HasSuffix(kept[n-1], "\n") {
		kept[n-1] += eol
	}
	if len(threads) > 0 {
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(struct {
			Comments []model.Comment `yaml:"comments"`
		}{threads}); err != nil {
			return nil, fmt.Errorf("briefs: encode comments: %w", err)
		}
		if err := enc.Close(); err != nil {
			return nil, fmt.Errorf("briefs: encode comments: %w", err)
		}
		block := buf.String()
		if eol != "\n" {
			block = strings.ReplaceAll(block, "\n", eol)
		}
		kept = append(kept, block)
	}
	result := []byte(strings.Join(append(kept, lines[end:]...), ""))
	if err := verifyRewrite(raw, result, func(fm frontmatter) bool {
		return commentsEqual(fm.comments, threads)
	}); err != nil {
		return nil, err
	}
	return result, nil
}

// verifyRewrite re-parses before and after and refuses a rewrite that moved the
// signed content or the status (other than as asked), or that does not satisfy
// want.
func verifyRewrite(before, after []byte, want func(frontmatter) bool) error {
	norm := func(b []byte) string { return string(normalizeLineEndings(b)) }
	fmBefore, bodyBefore, _ := parseFrontmatter(norm(before))
	fmAfter, bodyAfter, problems := parseFrontmatter(norm(after))
	if len(problems) > 0 {
		return fmt.Errorf("%w: the rewritten frontmatter does not read back: %s", ErrFrontmatterNotRewritable, strings.Join(problems, "; "))
	}
	if LockHash(fmBefore.summary, fmBefore.restsOn, bodyBefore) != LockHash(fmAfter.summary, fmAfter.restsOn, bodyAfter) {
		return fmt.Errorf("%w: the rewrite would have changed the brief's signed content", ErrFrontmatterNotRewritable)
	}
	if !want(fmAfter) {
		return fmt.Errorf("%w: the rewritten frontmatter does not read back as intended", ErrFrontmatterNotRewritable)
	}
	return nil
}

// commentsEqual compares two thread lists as they serialize, so an empty
// replies slice and an absent one — which read back the same — are equal.
func commentsEqual(a, b []model.Comment) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	ra, errA := yaml.Marshal(a)
	rb, errB := yaml.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(ra, rb)
}

// byteColumn converts a yaml 1-based rune column on line into a byte offset,
// or -1 when the line is shorter.
func byteColumn(line string, column int) int {
	col := 1
	for i := range line {
		if col == column {
			return i
		}
		col++
	}
	return -1
}
