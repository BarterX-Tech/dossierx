package model

// SourceKind is the anchoring regime a Source is held to. It is a CLOSED
// enum of exactly two values, and the split is not cosmetic: the two kinds
// are falsifiable in different ways, so they carry different required
// fields and are checked by different lints.
//
//   - SourceKindExternal is something outside this repository — a vendor API
//     reference, a standard, a blog post. The engine cannot read it, and it
//     can change underneath the claim without anything local moving. What
//     makes such a citation checkable is not the URL alone but the URL
//     PLUS the date it was read: "this page said X on this day" is a claim a
//     later reader can go and refute. See the source-external-unanchored lint.
//
//   - SourceKindInternal is a file in this repository (or beside it) — a
//     requirements record, an extraction ledger, a research note. The engine
//     CAN read it, so the citation is anchored by a content hash rather than
//     by a date, and drift is detectable rather than merely possible. See
//     the source-internal-drift lint.
//
// A third kind has been deliberately declined. The filing issue proposed an
// open vocabulary (api-reference, internal, ...) mixing the MEDIUM of a
// source with its anchoring regime; those are different axes, and only the
// anchoring axis changes what the engine can check. Medium is a matter for
// the Title a human writes.
type SourceKind string

const (
	// SourceKindExternal is a source the engine cannot read: anchored by
	// URL + AccessedOn.
	SourceKindExternal SourceKind = "external"

	// SourceKindInternal is a source the engine can read: anchored by
	// Path (+ optional RecordID) and SHA256.
	SourceKindInternal SourceKind = "internal"
)

// Source is one piece of evidence a claim rests on, addressable from the
// claim's Body by a "[n]" citation marker matching its Ref.
//
// This type exists because provenance that lives outside the claim is
// provenance nothing can check. Before it, a claim could record WHICH
// sources it came from — in MigratedFrom, a single free-text string — but
// not WHAT they were: a reader had to know which external registry to open,
// and in which directory, before they could verify one sentence. Worse, a
// sidecar file beside the claim is invisible to the lock ledger, so the
// evidence behind a locked claim could be rewritten freely after approval
// while the claim itself stayed signed. The part of a locked claim that most
// needs pinning was the one part nothing pinned.
//
// Sources are therefore first-class claim content: signed by
// lock.LockedClaimHash like every other authored field (so editing a citation
// after lock is caught exactly like editing the body), and checked by the
// source-* lint family.
//
// Sources are deliberately NOT part of lock.ContentHash, which is the
// STALENESS baseline a dependent records for its dependencies. Adding a
// citation to a claim does not change what that claim promises, so it must
// not flip every dependent to review_pending. Provenance is not contract.
// See lock.ContentHash's doc comment for the two-hash split.
//
// Which fields are required depends on Kind — see SourceKind. Nothing here
// is validated by this type: the schema is enforced by the lint suite (the
// same place status, id and row shape are enforced), so a malformed source
// yields a reportable finding rather than a load failure that takes the whole
// project down.
type Source struct {
	// Ref is the citation number this source answers to: the body cites it
	// as "[1]", "[2]" and so on, the Perplexity/Wikipedia convention readers
	// already know how to read. It is author-assigned rather than derived
	// from position, so reordering the list does not silently renumber every
	// marker in the prose. Refs must be positive and unique within a claim
	// (source-shape), every marker must resolve (source-ref-undefined), and
	// every Ref should be cited by at least one marker (source-ref-unused).
	Ref int `yaml:"ref"`

	// Kind selects the anchoring regime. See SourceKind.
	Kind SourceKind `yaml:"kind"`

	// Title is how the source reads in the viewer's citation footer — the
	// page's name, the record's label. Required for every kind: a citation
	// a reader cannot identify without dereferencing it is a footnote that
	// only works online.
	Title string `yaml:"title"`

	// URL is where an external source lives. External only.
	URL string `yaml:"url,omitempty"`

	// AccessedOn is the ISO date (YYYY-MM-DD) the external source was read.
	// External only, and required there: it is the field that makes the
	// citation falsifiable rather than merely locatable. A page that no
	// longer says what the claim says it said is a finding; without a date,
	// it is an argument.
	AccessedOn string `yaml:"accessed_on,omitempty"`

	// Path is the internal source's file, relative to the project config's
	// own directory (the same anchor ClaimsDir and SourceDirs use — never
	// the process working directory). Internal only.
	Path string `yaml:"path,omitempty"`

	// RecordID optionally narrows an internal source to ONE record inside a
	// JSONL file, matched against that record's top-level "id". When set,
	// SHA256 pins that record's line; when unset, it pins the whole file.
	//
	// The distinction is about noise, not rigour. A shared registry file
	// holding hundreds of records changes constantly for reasons unrelated
	// to any one claim, and a whole-file hash would report drift on every
	// one of those edits — training readers to wave the finding through,
	// which is the failure mode a gate can least afford. Internal only.
	RecordID string `yaml:"record_id,omitempty"`

	// SHA256 is the hex content hash of what Path (and RecordID, when set)
	// named at the time the source was recorded. Internal only, and required
	// there: an internal source with no hash cannot be checked, and a check
	// that cannot execute is not a pass. source-internal-drift treats a
	// missing hash as the failure it is, not as an exemption.
	SHA256 string `yaml:"sha256,omitempty"`

	// Supports is what this source is cited FOR, in the author's words —
	// the sentence the claim leans on. Optional but strongly encouraged: it
	// is what lets a later reader re-validate the citation without
	// reconstructing the original author's reasoning from scratch.
	Supports string `yaml:"supports,omitempty"`

	// DoesNotSupport is the boundary of the citation: what this source
	// notably does NOT establish. Optional. It exists because the most
	// common citation defect is not a fabricated source but an overread one,
	// and the cheapest defence is for the author to state the limit while
	// they still remember it.
	DoesNotSupport string `yaml:"does_not_support,omitempty"`
}

// IsExternal reports whether s is anchored by URL + AccessedOn.
func (s Source) IsExternal() bool { return s.Kind == SourceKindExternal }

// IsInternal reports whether s is anchored by Path + SHA256.
func (s Source) IsInternal() bool { return s.Kind == SourceKindInternal }
