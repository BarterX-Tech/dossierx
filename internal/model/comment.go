// comment.go defines the review-discussion types attached to a claim: the
// engine-managed "comments on claims" feature. A Comment is one threaded
// review discussion (Google-Docs style) stored inline in the claim YAML; a
// Reply is one follow-up message under a thread. These are plain,
// yaml-tagged structs with NO custom (Un)MarshalYAML — unlike Row, a comment
// carries no authored-key-order to preserve, so gopkg.in/yaml.v3's default
// struct handling round-trips them exactly.
//
// The raw "does this claim have unresolved review threads?" predicates
// (OpenThreadIDs/HasOpenThreads) live in THIS package, on model.Claim, on
// purpose: internal/lint's comments-unresolved rule and internal/buildorder's
// completeness gate need them, and internal/lock (which the lock gate lives
// in) already imports internal/lint — so hosting the predicate anywhere lint
// or buildorder would have to import lock/comments to reach it would create
// an import cycle. model is the one package every consumer already depends
// on, so the predicate is cycle-free here.
package model

// CommentRole is the role — not the identity — that authored, resolved, or
// reopened a comment thread. It is the single enum used by every comment op
// and the CLI's `--as` flag. Advisory only: a human-opened thread is
// resolvable/reopenable/editable/deletable only by a human, an agent-opened
// one by either, but this is a coordination convention for a local
// single-user tool, not an authenticated authorization boundary.
type CommentRole string

const (
	// CommentRoleHuman is a human reviewer.
	CommentRoleHuman CommentRole = "human"
	// CommentRoleAgent is an AI/automation agent.
	CommentRoleAgent CommentRole = "agent"
)

// Comment status values. A thread is Open until resolved and can be
// reopened back to Open. Stored verbatim as the YAML `status` scalar.
const (
	CommentStatusOpen     = "open"
	CommentStatusResolved = "resolved"
)

// Reply is one follow-up message under a Comment thread. Its Id is
// engine-generated (see internal/comments' id generator: prefix "r-" + 6
// lowercase hex, unique within the claim file).
type Reply struct {
	ID      string      `yaml:"id"`
	Author  CommentRole `yaml:"author"`
	Created string      `yaml:"created"` // RFC 3339 UTC
	Body    string      `yaml:"body"`
	Edited  bool        `yaml:"edited"`
}

// Comment is one threaded review discussion attached to a claim. Its Id is
// engine-generated (prefix "c-" + 6 lowercase hex, unique within the claim
// file). Status is one of CommentStatusOpen/CommentStatusResolved. The
// resolved_by/at and reopened_by/at fields record the role and RFC 3339 UTC
// timestamp of the most recent resolve and reopen respectively, so a thread
// carries its full open/resolve/reopen history and the next cycle's advisory
// rights can be checked against the opening Author.
//
// The lifecycle metadata fields are omitempty so a fresh, never-resolved
// thread stays minimal on disk (id/status/author/created/body/edited only);
// Edited is written even when false, mirroring the schema block in FORMAT.md.
type Comment struct {
	ID         string      `yaml:"id"`
	Status     string      `yaml:"status"`
	Author     CommentRole `yaml:"author"`
	Created    string      `yaml:"created"` // RFC 3339 UTC
	Body       string      `yaml:"body"`
	Edited     bool        `yaml:"edited"`
	Replies    []Reply     `yaml:"replies,omitempty"`
	ResolvedBy CommentRole `yaml:"resolved_by,omitempty"`
	ResolvedAt string      `yaml:"resolved_at,omitempty"`
	ReopenedBy CommentRole `yaml:"reopened_by,omitempty"`
	ReopenedAt string      `yaml:"reopened_at,omitempty"`
}

// OpenThreadIDs returns the ids of every comment thread on c whose status is
// still open, in declaration order. It returns nil (not an empty slice) when
// c has no open threads, so `len(c.OpenThreadIDs()) == 0` is the canonical
// "nothing unresolved" test. This is the raw predicate internal/lint's
// comments-unresolved rule, internal/lock's lock gate, and
// internal/buildorder's completeness gate all read — see this file's package
// doc for why it lives on model rather than in internal/comments.
func (c Claim) OpenThreadIDs() []string {
	var ids []string
	for _, cm := range c.Comments {
		if cm.Status == CommentStatusOpen {
			ids = append(ids, cm.ID)
		}
	}
	return ids
}

// HasOpenThreads reports whether c has at least one unresolved comment
// thread. It short-circuits on the first open thread, so it is the cheap
// boolean form of OpenThreadIDs for callers that don't need the ids.
func (c Claim) HasOpenThreads() bool {
	for _, cm := range c.Comments {
		if cm.Status == CommentStatusOpen {
			return true
		}
	}
	return false
}
