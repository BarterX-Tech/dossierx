package briefs

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// MaxSummaryChars is the ceiling on a brief's summary, in Unicode code points.
// It is fixed rather than a config override: the summary is the one line
// `brief list` prints per brief, and 200 is the claim summary's default for the
// same reason — one line a reader takes in at a glance.
const MaxSummaryChars = 200

// frontmatter is what a brief's opening block yielded. A field the block did
// not carry, or carried in a shape parseFrontmatter refused, is its zero value;
// the refusal itself is reported separately.
type frontmatter struct {
	summary  string
	status   string
	restsOn  []string
	comments []model.Comment
}

// frontmatterFields is the whole key set, in the order a message names it.
// comments is engine-managed (NIT-205): the review threads on the brief, in the
// claim's own comment shape, written by the comment ops and never by hand. It
// is not signed by the brief's lock hash, as a claim's comments are not.
var frontmatterFields = []string{"summary", "status", "rests_on", "comments"}

// parseFrontmatter splits a brief into its frontmatter and its body and reads
// the frontmatter under a strict decode: four keys and no others, each in the
// one shape it may take.
//
// THE BLOCK. The file's first line is exactly "---", and the block runs to the
// next line that is exactly "---". Nothing else opens one — not "...", not a
// "---" further down, not a blank line first — because a rule with one spelling
// is a rule an author cannot get half right. The body is everything after the
// closing line.
//
// THE KEYS. summary is required: a single line of plain text, at most
// MaxSummaryChars code points, the line `brief list` prints. status is draft or
// locked and defaults to draft. rests_on is an optional list of claim ids, each
// a non-empty string, none repeated; whether each one IS a claim is
// brief-rests-on-unknown's question, asked once the claims are loaded.
//
// It decodes to a yaml.Node and checks each value's KIND rather than decoding
// into Go strings, because yaml.v3 coerces scalars: `summary: 5` would become
// "5" and `status: yes` a string, and a frontmatter that is not what it looks
// like should be refused, not reinterpreted. That is the discipline
// config.Conformance's decoder applies for the same reason.
func parseFrontmatter(content string) (fm frontmatter, body string, problems []string) {
	lines := strings.SplitAfter(content, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\n") != "---" {
		return fm, content, []string{"a brief opens with a --- frontmatter block carrying at least summary: (one line naming what the brief is for); this file does not start with ---"}
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\n") == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return fm, content, []string{"the frontmatter block opened by the first --- line is never closed; end it with a second line that is exactly ---"}
	}
	block := strings.Join(lines[1:end], "")
	body = strings.Join(lines[end+1:], "")

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(block), &doc); err != nil {
		return fm, body, []string{fmt.Sprintf("the frontmatter is not valid YAML: %v", err)}
	}
	var root *yaml.Node
	switch {
	case doc.Kind == 0:
		// An empty block: no keys at all, so the required one is missing.
	case doc.Kind == yaml.DocumentNode && len(doc.Content) == 1:
		root = doc.Content[0]
	}
	if root != nil && root.Kind != yaml.MappingNode {
		if root.Tag == "!!null" {
			root = nil
		} else {
			return fm, body, []string{"the frontmatter must be a mapping of summary, status, rests_on and comments"}
		}
	}

	seen := map[string]bool{}
	summaryRefused := false
	if root != nil {
		for i := 0; i+1 < len(root.Content); i += 2 {
			key, value := root.Content[i], root.Content[i+1]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
				problems = append(problems, fmt.Sprintf("frontmatter key on line %d is not a name", key.Line+1))
				continue
			}
			if !isFrontmatterField(key.Value) {
				problems = append(problems, fmt.Sprintf("frontmatter field %q is not a brief field; the fields are %s", key.Value, strings.Join(frontmatterFields, ", ")))
				continue
			}
			if seen[key.Value] {
				problems = append(problems, fmt.Sprintf("frontmatter field %q is set twice", key.Value))
				continue
			}
			seen[key.Value] = true
			switch key.Value {
			case "summary":
				s, ok := stringScalar(value)
				if !ok {
					summaryRefused = true
					problems = append(problems, "summary must be a single line of plain text, not "+nodeKind(value))
					continue
				}
				fm.summary = s
			case "status":
				s, ok := stringScalar(value)
				if !ok || (s != string(StatusDraft) && s != string(StatusLocked)) {
					problems = append(problems, fmt.Sprintf("status must be %q or %q (omit it for %q)", StatusDraft, StatusLocked, StatusDraft))
					continue
				}
				fm.status = s
			case "rests_on":
				ids, p := restsOnList(value)
				problems = append(problems, p...)
				fm.restsOn = ids
			case "comments":
				threads, p := commentThreads(value)
				problems = append(problems, p...)
				fm.comments = threads
			}
		}
	}

	switch {
	case summaryRefused:
		// Already reported, in its own words.
	case !seen["summary"]:
		problems = append(problems, "summary is required: one line of plain text naming what the brief is for, which brief list prints")
	case strings.TrimSpace(fm.summary) == "":
		problems = append(problems, "summary is blank; it is required, and it is the line brief list prints")
	case strings.ContainsAny(fm.summary, "\n\r"):
		problems = append(problems, "summary must be a single line, with no newline")
	case utf8.RuneCountInString(fm.summary) > MaxSummaryChars:
		problems = append(problems, fmt.Sprintf("summary is %d characters, over the %d-character ceiling; it is the one line brief list prints, so shorten it", utf8.RuneCountInString(fm.summary), MaxSummaryChars))
	}
	return fm, body, problems
}

func isFrontmatterField(name string) bool {
	for _, f := range frontmatterFields {
		if f == name {
			return true
		}
	}
	return false
}

// stringScalar reads a node as a YAML string and nothing else: a number, a
// boolean, a null, a list or a mapping is refused rather than coerced.
func stringScalar(n *yaml.Node) (string, bool) {
	if n == nil || n.Kind != yaml.ScalarNode || n.Tag != "!!str" {
		return "", false
	}
	return n.Value, true
}

// restsOnList reads rests_on: a list of distinct, non-empty string ids. A null
// or empty list is no dependencies.
func restsOnList(n *yaml.Node) (ids, problems []string) {
	if n.Kind == yaml.ScalarNode && n.Tag == "!!null" {
		return nil, nil
	}
	if n.Kind != yaml.SequenceNode {
		return nil, []string{"rests_on must be a list of claim ids, not " + nodeKind(n)}
	}
	seen := map[string]bool{}
	for _, item := range n.Content {
		id, ok := stringScalar(item)
		if !ok || strings.TrimSpace(id) == "" {
			problems = append(problems, fmt.Sprintf("rests_on entry on line %d must be a claim id (a non-empty string)", item.Line+1))
			continue
		}
		if seen[id] {
			problems = append(problems, fmt.Sprintf("rests_on lists %q twice", id))
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, problems
}

func nodeKind(n *yaml.Node) string {
	switch n.Kind {
	case yaml.SequenceNode:
		return "a list"
	case yaml.MappingNode:
		return "a mapping"
	case yaml.ScalarNode:
		switch n.Tag {
		case "!!null":
			return "null"
		case "!!int", "!!float":
			return "a number"
		case "!!bool":
			return "a boolean"
		}
		return "a scalar"
	}
	return "an alias"
}

// commentThreads reads the engine-managed comments block: a list of threads in
// model.Comment's shape, decoded strictly (an unknown key is refused), each
// with a known status and author. A null or empty list is no threads.
func commentThreads(n *yaml.Node) (threads []model.Comment, problems []string) {
	if n.Kind == yaml.ScalarNode && n.Tag == "!!null" {
		return nil, nil
	}
	if n.Kind != yaml.SequenceNode {
		return nil, []string{"comments must be a list of comment threads, not " + nodeKind(n) + "; the comment ops write it, never a hand edit"}
	}
	raw, err := yaml.Marshal(n)
	if err != nil {
		return nil, []string{fmt.Sprintf("comments could not be read: %v", err)}
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&threads); err != nil {
		return nil, []string{fmt.Sprintf("comments is not a list of comment threads: %v", err)}
	}
	for i, t := range threads {
		if t.Status != model.CommentStatusOpen && t.Status != model.CommentStatusResolved {
			problems = append(problems, fmt.Sprintf("comment thread %d has status %q; a thread is %q or %q", i+1, t.Status, model.CommentStatusOpen, model.CommentStatusResolved))
		}
		if t.Author != model.CommentRoleHuman && t.Author != model.CommentRoleAgent {
			problems = append(problems, fmt.Sprintf("comment thread %d has author %q; an author is %q or %q", i+1, t.Author, model.CommentRoleHuman, model.CommentRoleAgent))
		}
	}
	return threads, problems
}
