// Package constitution loads, measures, hashes and judges the project-root
// constitution.yaml — the one lockable roof over every module (NIT-6).
//
// The constitution is NOT a module, not a claim and not a graph node. It
// lives beside project.config.yaml, outside claims_dir, and no claim ever
// cites it: it is the brief for the whole system, so every claim builds
// toward it by definition and there is no ref grammar for it (NIT-24). Its
// reach is the lock gate (NIT-26: no module work until it is locked), the
// viewer's Constitution pin (NIT-27), and the digest and full text that
// `manifest show` carries into an agent's context (NIT-10).
//
// This package imports only model. internal/lock imports it for the lock
// record, internal/check for the gate, and the manifest work (PR #103) for
// Digest and Text — so nothing here may reach back into any of those.
package constitution

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// WordCap is the hard word budget: check and `constitution lock` refuse a
// file over it with CONSTITUTION_OVER_CAP. Words, not bytes, are the meter
// (ratified 2026-09-24).
const WordCap = 800

// NearCap is where the warning band starts: at or over 720 words, up to the
// cap, check reports constitution-near-cap at warning severity.
const NearCap = 720

// The three sections, in the order the file and the viewer show them.
const (
	SectionInvariants = "invariants"
	SectionGlossary   = "glossary"
	SectionDecisions  = "decisions"
)

// File is one project's constitution.yaml.
type File struct {
	// Status is draft or locked. It is written by `constitution lock` and is
	// only half of the lock state: the other half is the LockRecord in the
	// lock store, whose hash must still match the file. See Evaluate.
	Status     model.Status `yaml:"status"`
	Invariants []Entry      `yaml:"invariants,omitempty"`
	Glossary   []Entry      `yaml:"glossary,omitempty"`
	Decisions  []Entry      `yaml:"decisions,omitempty"`
	SourcePath string       `yaml:"-"`
}

// Entry is one section item. Slug is required and unique within the file;
// Title is optional; Body is plain text (whether it becomes markdown is an
// open question on NIT-6 and the viewer escapes it as text until it closes).
type Entry struct {
	Slug  string `yaml:"slug"`
	Title string `yaml:"title,omitempty"`
	Body  string `yaml:"body"`
}

// Section is one named group of entries, for the show command and the
// manifest text.
type Section struct {
	Name    string  `json:"name"`
	Entries []Entry `json:"entries"`
}

// ErrNotFound means the configured path does not exist. The gate reads that
// as "not locked": a project with no constitution has no roof to lock.
var ErrNotFound = errors.New("constitution file not found")

// Load reads path. A missing file returns an error wrapping ErrNotFound.
func Load(path string) (*File, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("constitution: %s: %w", path, ErrNotFound)
		}
		return nil, fmt.Errorf("constitution: read %s: %w", path, err)
	}
	return Parse(raw, path)
}

// LoadOptional returns (nil, nil) when the file is absent.
func LoadOptional(path string) (*File, error) {
	f, err := Load(path)
	if err != nil && errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return f, err
}

// Parse decodes one constitution document under the same discipline the
// claim loader applies: unknown keys refused, one document per file.
func Parse(raw []byte, sourcePath string) (*File, error) {
	var f File
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("constitution: %s is empty", sourcePath)
		}
		return nil, fmt.Errorf("constitution: parse %s: %w", sourcePath, err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("constitution: %s contains more than one YAML document", sourcePath)
	}
	if f.Status == "" {
		f.Status = model.StatusDraft
	}
	if f.Status != model.StatusDraft && f.Status != model.StatusLocked {
		return nil, fmt.Errorf("constitution: %s: status must be draft or locked, got %q", sourcePath, f.Status)
	}
	f.SourcePath = sourcePath
	if err := validateEntries(f); err != nil {
		return nil, fmt.Errorf("constitution: %s: %w", sourcePath, err)
	}
	return &f, nil
}

// Marshal is the file's canonical on-disk form, used by `constitution lock`
// when it rewrites the status line.
func Marshal(f *File) ([]byte, error) {
	return yaml.Marshal(f)
}

func validateEntries(f File) error {
	seen := map[string]bool{}
	for _, s := range Sections(&f) {
		for i, e := range s.Entries {
			if strings.TrimSpace(e.Slug) == "" {
				return fmt.Errorf("%s[%d]: slug is required", s.Name, i)
			}
			key := s.Name + "." + e.Slug
			if seen[key] {
				return fmt.Errorf("duplicate entry %s", key)
			}
			seen[key] = true
		}
	}
	return nil
}

// Sections returns the three sections in file order. Every section is
// present, even when empty, so consumers can iterate without special cases.
func Sections(f *File) []Section {
	if f == nil {
		return nil
	}
	return []Section{
		{Name: SectionInvariants, Entries: f.Invariants},
		{Name: SectionGlossary, Entries: f.Glossary},
		{Name: SectionDecisions, Entries: f.Decisions},
	}
}

// CountWords counts Unicode letter/number runs. It is the meter for every
// number this package reports and for the viewer's "N of 800 words".
func CountWords(text string) int {
	n := 0
	in := false
	for _, r := range text {
		word := unicode.IsLetter(r) || unicode.IsNumber(r)
		if word && !in {
			n++
		}
		in = word
	}
	return n
}

// WordCount counts every entry's title and body. Slugs are identifiers, not
// prose, and are deliberately not counted (ratified 2026-09-24).
func WordCount(f *File) int {
	if f == nil {
		return 0
	}
	var b strings.Builder
	for _, s := range Sections(f) {
		for _, e := range s.Entries {
			b.WriteString(e.Title)
			b.WriteByte(' ')
			b.WriteString(e.Body)
			b.WriteByte(' ')
		}
	}
	return CountWords(b.String())
}

// OverCap reports the hard refuse.
func OverCap(f *File) bool { return WordCount(f) > WordCap }

// IsNearCap reports the warning band: at or over NearCap, not over WordCap.
func IsNearCap(f *File) bool {
	n := WordCount(f)
	return n >= NearCap && n <= WordCap
}

// Hash is the lock-store digest of the file's CONTENT: every section's
// entries, in order, slug, title and body. Status is excluded on purpose —
// `constitution lock` flips it, and a hash that moved on the lock's own
// write could never match its own record. Length-prefixed so no two files
// collide by concatenation.
func Hash(f *File) string {
	if f == nil {
		return ""
	}
	h := sha256.New()
	fmt.Fprint(h, "dossierx-constitution/v1\n")
	for _, s := range Sections(f) {
		fmt.Fprintf(h, "section=%s n=%d\n", s.Name, len(s.Entries))
		for _, e := range s.Entries {
			fmt.Fprintf(h, "slug=%d:%s\ntitle=%d:%s\nbody=%d:%s\n", len(e.Slug), e.Slug, len(e.Title), e.Title, len(e.Body), e.Body)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Text renders the whole constitution as plain text — the words an agent
// drafts against, not the hash. It is what `constitution show --format text`
// prints and what `manifest show --isolation` embeds (NIT-10). Sections with
// no entries are omitted; an entry without a title is headed by its slug.
func Text(f *File) string {
	if f == nil {
		return ""
	}
	var b strings.Builder
	for _, s := range Sections(f) {
		if len(s.Entries) == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("# ")
		b.WriteString(sectionTitle(s.Name))
		b.WriteString("\n")
		for _, e := range s.Entries {
			b.WriteString("\n## ")
			if e.Title != "" {
				b.WriteString(e.Title)
				b.WriteString(" (")
				b.WriteString(e.Slug)
				b.WriteString(")")
			} else {
				b.WriteString(e.Slug)
			}
			b.WriteString("\n")
			body := strings.TrimRight(e.Body, "\n")
			if body != "" {
				b.WriteString(body)
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}

func sectionTitle(name string) string {
	switch name {
	case SectionInvariants:
		return "Invariants"
	case SectionGlossary:
		return "Glossary"
	case SectionDecisions:
		return "Decisions"
	}
	return name
}

// Digest is the machine summary: counts and a hash, enough to tell WHICH
// constitution a view was built against and how full it is, never enough to
// draft against (that is Text).
type Digest struct {
	Path       string `json:"path"`
	Present    bool   `json:"present"`
	Status     string `json:"status,omitempty"`
	Words      int    `json:"words"`
	WordCap    int    `json:"word_cap"`
	OverCap    bool   `json:"over_cap"`
	NearCap    bool   `json:"near_cap"`
	Hash       string `json:"hash,omitempty"`
	Invariants int    `json:"invariants"`
	Glossary   int    `json:"glossary"`
	Decisions  int    `json:"decisions"`
}

// NewDigest projects f (possibly nil) for the CLI, the viewer and PR #103's
// manifest views.
func NewDigest(path string, f *File) Digest {
	d := Digest{Path: path, WordCap: WordCap}
	if f == nil {
		return d
	}
	d.Present = true
	d.Status = string(f.Status)
	d.Words = WordCount(f)
	d.OverCap = OverCap(f)
	d.NearCap = IsNearCap(f)
	d.Hash = Hash(f)
	d.Invariants = len(f.Invariants)
	d.Glossary = len(f.Glossary)
	d.Decisions = len(f.Decisions)
	return d
}

// LockRecord is what `constitution lock` writes into the lock store: the
// content hash it approved, the human's own words, and when. It is the same
// integrity contract a claim gets — a hand edit after the lock leaves a
// file whose Hash no longer matches, and Evaluate reports it as edited.
type LockRecord struct {
	Hash     string `json:"hash"`
	Reason   string `json:"reason"`
	LockedAt string `json:"locked_at"`
}

// State is the gate's one-word answer.
type State string

const (
	// StateLocked: status locked, a record stands, and the hashes agree.
	StateLocked State = "locked"
	// StateMissing: no constitution.yaml at the configured path.
	StateMissing State = "missing"
	// StateUnreadable: the file exists but does not parse.
	StateUnreadable State = "unreadable"
	// StateDraft: the file says status: draft.
	StateDraft State = "draft"
	// StateUnrecorded: the file says locked but the store holds no record —
	// a status line flipped by hand, never approved.
	StateUnrecorded State = "unrecorded"
	// StateEdited: a record stands but the file's content hash has moved.
	// The edited file is what every module now reads; work stops until a
	// human runs `constitution lock` again.
	StateEdited State = "edited"
)

// Verdict is Evaluate's answer, shaped for an envelope's error.details.
type Verdict struct {
	State      State  `json:"state"`
	Path       string `json:"path"`
	Words      int    `json:"words"`
	WordCap    int    `json:"word_cap"`
	OverCap    bool   `json:"over_cap"`
	NearCap    bool   `json:"near_cap"`
	Hash       string `json:"hash,omitempty"`
	StoredHash string `json:"stored_hash,omitempty"`
	Reason     string `json:"reason,omitempty"`
	LockedAt   string `json:"locked_at,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Locked reports whether module work may proceed.
func (v Verdict) Locked() bool { return v.State == StateLocked }

// Evaluate judges a loaded file against the store's record. loadErr is
// Load's error (nil when f is present); rec is the store's record or nil.
func Evaluate(path string, f *File, loadErr error, rec *LockRecord) Verdict {
	v := Verdict{Path: path, WordCap: WordCap}
	if rec != nil {
		v.StoredHash = rec.Hash
		v.Reason = rec.Reason
		v.LockedAt = rec.LockedAt
	}
	switch {
	case loadErr != nil && errors.Is(loadErr, ErrNotFound):
		v.State = StateMissing
		return v
	case loadErr != nil:
		v.State = StateUnreadable
		v.Error = loadErr.Error()
		return v
	case f == nil:
		v.State = StateMissing
		return v
	}
	v.Words = WordCount(f)
	v.OverCap = OverCap(f)
	v.NearCap = IsNearCap(f)
	v.Hash = Hash(f)
	switch {
	case f.Status != model.StatusLocked:
		v.State = StateDraft
	case rec == nil:
		v.State = StateUnrecorded
	case rec.Hash != v.Hash:
		v.State = StateEdited
	default:
		v.State = StateLocked
	}
	return v
}

// EvaluateAt loads path and judges it. It is the read-only form the gate and
// the viewer use; the CLI's lock command loads the file itself so it can
// rewrite it.
func EvaluateAt(path string, rec *LockRecord) Verdict {
	f, err := Load(path)
	return Evaluate(path, f, err, rec)
}

// Detail is the one sentence a refusal or a finding carries.
func (v Verdict) Detail() string {
	switch v.State {
	case StateLocked:
		return fmt.Sprintf("constitution is locked (%d of %d words)", v.Words, v.WordCap)
	case StateMissing:
		return fmt.Sprintf("no constitution at %s", v.Path)
	case StateUnreadable:
		return fmt.Sprintf("constitution at %s does not load: %s", v.Path, v.Error)
	case StateDraft:
		return fmt.Sprintf("constitution at %s is status: draft", v.Path)
	case StateUnrecorded:
		return fmt.Sprintf("constitution at %s says status: locked but the lock store holds no record for it", v.Path)
	case StateEdited:
		return fmt.Sprintf("constitution at %s was edited after its lock (stored hash %s, file hash %s)", v.Path, short(v.StoredHash), short(v.Hash))
	}
	return string(v.State)
}

// Hint is the recovery an agent shows the human.
func (v Verdict) Hint() string {
	switch v.State {
	case StateMissing:
		return "write constitution.yaml beside project.config.yaml (sections invariants / glossary / decisions, each entry a slug, an optional title and a body), then `dossierx constitution lock --reason \"<the human's words>\"` — no module claim locks and no plain check passes until the roof is locked"
	case StateUnreadable:
		return "fix the constitution file the message names, then `dossierx constitution lock --reason \"<the human's words>\"`"
	case StateDraft, StateUnrecorded:
		return "`dossierx constitution lock --dry-run`, show the human, then `dossierx constitution lock --reason \"<their words>\"`"
	case StateEdited:
		return "the edited file is what every module now reads; nothing from the old version stays in force. Show the human the diff, then `dossierx constitution lock --reason \"<their words>\"` to re-lock it — or restore the file from git"
	}
	return ""
}

func short(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}
