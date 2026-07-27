// Package implink tracks, per module, which real source files (and
// optionally which symbol within a file) implement which locked claim —
// the project-agnostic answer to "where in the real codebase does claim X
// actually live", filled in by an agent as it writes code or tests against
// a module's claims. Nothing in this package is reviewed or proposed: Set
// is the one and only write action, takes effect immediately, and is meant
// to be called autonomously by whatever agent just finished writing code
// against a claim — there is no confirm step the way internal/reaudit has
// one, because a link is a statement of fact about the codebase ("this
// file implements that claim"), not an edit to a claim's own content that a
// human needs to review before it lands.
//
// The only gate Set enforces is that the claim being linked must already
// be status: locked — you cannot ground a link to a claim that might still
// change out from under it. This applies identically regardless of the
// claim's build_role: a verification (test-checklist) claim links to the
// real test file(s)/function(s) that implement its checklist items via the
// exact same call shape as a schema/behavior/api claim linking to its own
// implementation.
//
// Like internal/buildorder, the on-disk artifact is one JSON file per
// module (ArtifactPath), generated and only ever rewritten by Set, never
// hand-edited; the sibling status.go recomputes drift against it on
// demand, mirroring buildorder.Status's read-only-recompute-on-load
// contract.
package implink

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// nowFunc is this package's clock, overridable in tests so LinkedAt can be
// asserted deterministically instead of racing real time — the same
// pattern internal/lock.nowFunc and internal/buildorder.nowFunc use.
var nowFunc = time.Now

// ErrNoArtifact is returned (wrapped) by LoadArtifact, and therefore by
// Status, whenever no implementation-link artifact exists yet at the given
// path — the common case for any module that has never called Set. Callers
// use errors.Is(err, ErrNoArtifact) to distinguish "nothing linked yet"
// from a genuine read/parse failure, the same way buildorder.ErrNotProposed
// lets callers special-case "no build-order artifact" instead of reporting
// a generic file error.
var ErrNoArtifact = errors.New("implink: no implementation-link artifact for this module yet")

// FileLink is one linked file entry: the project-relative path (never
// absolute — Set rejects an absolute path outright rather than silently
// rewriting it, unlike buildorder.go's displayPath, which converts an
// already-absolute internal path; here the path is agent-supplied input,
// so the contract is enforced at the door instead), an optional symbol
// name (e.g. a function or type Set was told this file's linked code lives
// in), and FileHash — a whole-file content hash snapshot taken at Set time,
// the drift baseline Status re-checks against.
type FileLink struct {
	File     string `json:"file"`
	Symbol   string `json:"symbol,omitempty"`
	FileHash string `json:"file_hash"`
}

// Link is one claim's full set of linked files. LinkedAt is refreshed by
// every Set call that touches this claim (whether it upserts an existing
// file entry or appends a new one) — it is a per-claim, not per-file,
// timestamp, and is what Status's drift reason string reports against
// ("file changed since linked at <LinkedAt>"). Note is reserved for a
// future free-form annotation on the link as a whole; nothing in this
// package's current Set signature populates it, so it is always empty
// today — see this package's implementation report for why.
type Link struct {
	ClaimID  string     `json:"claim_id"`
	Files    []FileLink `json:"files"`
	Note     string     `json:"note,omitempty"`
	LinkedAt string     `json:"linked_at"`
}

// Artifact is one module's implementation-link document: every claim in
// that module that has at least one linked file, each with its own Files
// list. A claim may have any number of linked files; a file may be linked
// from any number of different claims — both are unrestricted, since
// Artifact stores Links keyed by claim, not files keyed by some unique
// owner.
type Artifact struct {
	Module string `json:"module"`
	Links  []Link `json:"links"`
}

// linkIndex returns the index of a's Link entry for claimID, or -1 if none
// exists yet.
func (a *Artifact) linkIndex(claimID string) int {
	for i, l := range a.Links {
		if l.ClaimID == claimID {
			return i
		}
	}
	return -1
}

// ArtifactPath returns the on-disk path for module's implementation-link
// artifact, resolved against cfg's own directory (never the process cwd) —
// the same convention internal/buildorder.ArtifactPath and internal/lock's
// store path follow. The filename is scoped per-module, sibling to
// .catalog.json and .build-order.<module>.json.
func ArtifactPath(cfg *config.Config, module string) string {
	return filepath.Join(cfg.Dir(), fmt.Sprintf(".implementation.%s.json", module))
}

// LoadArtifact reads and decodes the implementation-link artifact at path.
// A missing file returns an error wrapping ErrNoArtifact rather than a bare
// os.IsNotExist-shaped error, so callers can use errors.Is(err,
// ErrNoArtifact) the same way they use errors.Is(err,
// buildorder.ErrNotProposed).
func LoadArtifact(path string) (*Artifact, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrNoArtifact, path)
		}
		return nil, fmt.Errorf("implink: read %s: %w", path, err)
	}
	var a Artifact
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, fmt.Errorf("implink: parse %s: %w", path, err)
	}
	return &a, nil
}

// WriteArtifact serializes a to path as indented JSON, creating path's
// parent directory if needed. It is always a full overwrite: like every
// other generated artifact in this engine, this file is never hand-edited
// or merged with a prior version outside of Set's own load-mutate-write
// cycle.
func WriteArtifact(a *Artifact, path string) error {
	if a == nil {
		return fmt.Errorf("implink: cannot write a nil artifact")
	}
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return fmt.Errorf("implink: marshal artifact: %w", err)
	}
	data = append(data, '\n')

	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("implink: create output dir %q: %w", dir, err)
		}
	}
	if err := atomicWriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("implink: write %s: %w", path, err)
	}
	return nil
}

// Set links file (optionally at/around symbol) to claimID inside module's
// implementation-link artifact, creating the artifact if this is the
// module's first ever link. It is an upsert keyed on (claimID, file): if
// claimID already has an entry for file, that entry's Symbol/FileHash are
// refreshed in place and the claim's Link.LinkedAt is bumped; if file is
// new for that claim, it is appended to the claim's Files list instead
// (claim.LinkedAt is still bumped, since a claim gaining a new linked file
// is exactly the kind of event LinkedAt exists to record). Nothing about a
// different claim's links, or a different file's entry under the same
// claim, is touched.
//
// Set validates, in order: claimID names a real claim in claims; that
// claim belongs to module; that claim is status: locked (you cannot ground
// a link to a claim that might still change); and file exists on disk,
// resolved relative to cfg.Dir() (never absolute — an absolute file
// argument is refused outright rather than silently accepted, since a
// project-relative path is this artifact's whole point: it must mean the
// same thing regardless of which machine's absolute directory layout
// happens to be running the engine). On success it computes file's whole-
// file content hash (mirroring internal/lock.ContentHash's approach,
// applied to file bytes instead of claim fields) as the new drift
// baseline, writes the updated artifact back to disk, and returns it.
//
// Set applies identically to a claim of any build_role, including
// verification: a test-checklist claim links to the real test file(s) that
// implement its checklist items via this exact same call.
func Set(claims []model.Claim, cfg *config.Config, module, claimID, file, symbol string) (*Artifact, error) {
	if cfg == nil {
		return nil, fmt.Errorf("implink: cfg must not be nil")
	}
	path := ArtifactPath(cfg, strings.TrimSpace(module))

	artifact, err := LoadArtifact(path)
	if err != nil {
		if !errors.Is(err, ErrNoArtifact) {
			return nil, err
		}
		artifact = &Artifact{Module: strings.TrimSpace(module)}
	}
	if err := applyLink(artifact, claims, cfg, module, claimID, file, symbol); err != nil {
		return nil, err
	}
	if err := WriteArtifact(artifact, path); err != nil {
		return nil, err
	}
	return artifact, nil
}

// applyLink is Set's validation and in-memory mutation, without the load or the
// write. It exists so Scan can apply MANY links to one artifact under a single
// acquisition of that artifact's sentinel and persist the result once, instead
// of running a whole load-mutate-write cycle per tag.
//
// THE CALLER OWNS THE SENTINEL, and that is not a matter of taste here: the
// artifact is a whole-file overwrite, so two writers that each load, mutate and
// write independently lose one of the two updates outright. That was
// reproducible — `dossierx check &` then `dossierx claim link` a second later
// dropped the link in half of ten runs, and `claim link` exists precisely for
// the cases scanning cannot reach, so a re-scan never restores it. Both callers
// in this tree hold lock.AcquireFileLock(ArtifactPath(cfg, module)) across their
// load-mutate-write: cmd/dossierx's claim link takes it around Set, and Scan
// takes it once per module around the whole batch.
func applyLink(artifact *Artifact, claims []model.Claim, cfg *config.Config, module, claimID, file, symbol string) error {
	if cfg == nil {
		return fmt.Errorf("implink: cfg must not be nil")
	}
	module = strings.TrimSpace(module)
	claimID = strings.TrimSpace(claimID)
	file = strings.TrimSpace(file)
	if module == "" {
		return fmt.Errorf("implink: module must not be empty")
	}
	if claimID == "" {
		return fmt.Errorf("implink: claim id must not be empty")
	}
	if file == "" {
		return fmt.Errorf("implink: file must not be empty")
	}

	claim, ok := findByID(claims, claimID)
	if !ok {
		return fmt.Errorf("implink: claim %q not found", claimID)
	}
	if claim.Module != module {
		return fmt.Errorf("implink: claim %q belongs to module %q, not %q", claimID, claim.Module, module)
	}
	if claim.Status != model.StatusLocked {
		return fmt.Errorf("implink: claim %q is not locked (status %q); only a locked claim can be linked", claimID, claim.Status)
	}

	if filepath.IsAbs(file) {
		return fmt.Errorf("implink: file %q must be a project-relative path, not absolute", file)
	}
	absDir, err := filepath.Abs(cfg.Dir())
	if err != nil {
		return fmt.Errorf("implink: resolve project dir %q: %w", cfg.Dir(), err)
	}
	absFile, err := filepath.Abs(filepath.Join(absDir, file))
	if err != nil {
		return fmt.Errorf("implink: resolve file %q: %w", file, err)
	}
	rel, err := filepath.Rel(absDir, absFile)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("implink: file %q must resolve to a path inside the project directory, not escape it via \"..\"", file)
	}
	hash, err := hashFile(filepath.Join(cfg.Dir(), file))
	if err != nil {
		return fmt.Errorf("implink: file %q does not exist (looked relative to %s): %w", file, cfg.Dir(), err)
	}
	file = filepath.ToSlash(file)

	now := nowFunc().UTC().Format(time.RFC3339Nano)

	idx := artifact.linkIndex(claimID)
	if idx == -1 {
		artifact.Links = append(artifact.Links, Link{ClaimID: claimID})
		idx = len(artifact.Links) - 1
	}
	link := &artifact.Links[idx]
	link.LinkedAt = now

	fidx := -1
	for i, f := range link.Files {
		if f.File == file {
			fidx = i
			break
		}
	}
	if fidx == -1 {
		link.Files = append(link.Files, FileLink{File: file, Symbol: symbol, FileHash: hash})
	} else {
		link.Files[fidx].Symbol = symbol
		link.Files[fidx].FileHash = hash
	}

	// Sorted (Links by ClaimID, each Link's Files by File) so the artifact
	// is byte-deterministic regardless of the order Set calls arrived in —
	// the same "generated JSON should read the same way twice for the same
	// content" reasoning as catalog.Document's alphabetical-by-id claim
	// order, not a claim about authored/call order mattering semantically.
	sortArtifact(artifact)
	return nil
}

func sortArtifact(a *Artifact) {
	sort.Slice(a.Links, func(i, j int) bool { return a.Links[i].ClaimID < a.Links[j].ClaimID })
	for i := range a.Links {
		files := a.Links[i].Files
		sort.Slice(files, func(x, y int) bool { return files[x].File < files[y].File })
	}
}

// hashFile returns the hex-encoded sha256 of path's whole file content.
// This mirrors internal/lock.ContentHash's approach of hashing comparable
// content as a staleness baseline, applied here to raw file bytes instead
// of claim fields — this package cannot know *what* changed inside a file
// (it is language-agnostic and has no parser for any project's source
// language), only *that* something did.
func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// findByID is a small local claim lookup, duplicated here rather than
// imported from internal/loader, mirroring internal/buildorder's own
// precedent of keeping each generated-artifact package's dependency
// footprint limited to exactly what it needs.
func findByID(claims []model.Claim, id string) (model.Claim, bool) {
	for _, c := range claims {
		if c.ID == id {
			return c, true
		}
	}
	return model.Claim{}, false
}

// atomicWriteFile writes data to path without ever leaving a reader able to
// observe a partially-written file: it writes to a temp file created in
// path's own directory (so the later rename stays on one filesystem, which
// is what makes it atomic) and then renames it over path. Duplicated here
// rather than exported from internal/lock/internal/buildorder/internal/loader
// (which each already have their own copy), matching buildorder/store.go's
// own reasoning for why it duplicates rather than imports this same helper.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
