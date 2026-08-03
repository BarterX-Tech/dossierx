// Package loader reads claim YAML files off disk (from a project's
// configured claims_dir) into model.Claim values, and writes individual
// claims back to their source file. It is the one place in the engine that
// touches the filesystem for claim content; every other package works
// purely in memory against []model.Claim.
package loader

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// LoadClaims recursively reads every *.yaml/*.yml file under dir, strictly
// decoding each into a model.Claim (unknown fields are a hard error, same
// discipline as internal/config). Each claim's SourcePath is set to the
// file it was loaded from. The result is sorted by SourcePath so callers
// get deterministic ordering regardless of directory-walk order.
//
// A dir that does not exist is a hard error: claims_dir is required
// project configuration, not an optional feature.
func LoadClaims(dir string) ([]model.Claim, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("loader: claims_dir %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("loader: claims_dir %q is not a directory", dir)
	}

	var claims []model.Claim
	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		raw, err := readFileWithRetry(path)
		if err != nil {
			return fmt.Errorf("loader: read %s: %w", path, err)
		}

		var c model.Claim
		dec := yaml.NewDecoder(strings.NewReader(string(raw)))
		dec.KnownFields(true)
		if err := dec.Decode(&c); err != nil {
			return fmt.Errorf("loader: parse %s: %w", path, err)
		}
		// One claim per file is required, not merely recommended. A second
		// YAML document (--- separated) in the same file must be a hard
		// error rather than silently dropped: SaveClaim rewrites a claim's
		// file as a single document, so a later lock/reaudit would clobber
		// any file-siblings stacked behind the first. A clean single-
		// document file leaves the decoder at io.EOF on the next Decode;
		// anything else (another document, even an empty or malformed one)
		// means more than one document is present.
		var extra yaml.Node
		if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
			return fmt.Errorf("loader: %s contains more than one YAML document; exactly one claim per file is required", path)
		}
		c.SourcePath = path
		claims = append(claims, c)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	sort.Slice(claims, func(i, j int) bool { return claims[i].SourcePath < claims[j].SourcePath })
	return claims, nil
}

// readFileWithRetry is os.ReadFile with a short, bounded retry loop on
// Windows only. atomicWriteFile's rename-over-path is atomic on POSIX (a
// concurrent reader always sees either the old or new complete file, never
// an error), but Windows's mandatory file locking can make the rename
// itself transiently collide with a concurrent open-for-read on the same
// path (ERROR_SHARING_VIOLATION) — a real gap surfaced by
// TestConcurrentLocksDoNotLoseStoreUpdates running many "dossierx lock"
// processes against the same claims_dir simultaneously. The window is a
// single rename syscall, not a slow operation, so a handful of short
// retries resolves it without meaningfully slowing down the common,
// uncontended case (which never retries at all).
func readFileWithRetry(path string) ([]byte, error) {
	if runtime.GOOS != "windows" {
		return os.ReadFile(path)
	}
	const attempts = 5
	var raw []byte
	var err error
	for i := 0; i < attempts; i++ {
		raw, err = os.ReadFile(path)
		if err == nil {
			return raw, nil
		}
		if i < attempts-1 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	return raw, err
}

// SaveClaim writes c back to its SourcePath as YAML. It is used by the
// lock/unlock/reaudit-apply flows, which are the only paths that mutate a
// claim's on-disk representation.
//
// The write is atomic (temp file in the same directory, then rename) rather
// than a direct os.WriteFile. os.WriteFile truncates the destination before
// writing its new bytes, leaving a window where the file is empty or
// partially written; a concurrent LoadClaims (every "dossierx lock" invocation
// starts by loading the *entire* claims_dir, including files other
// in-flight processes are saving) can land its read inside that window and
// see a truncated file, failing YAML decode with a bare EOF. Writing to a
// sibling temp file and renaming it into place means any concurrent reader
// only ever observes the old complete file or the new complete file, never
// a partial one — os.Rename is atomic within a single filesystem/directory.
//
// SaveClaim serves TWO modes, and the difference is not incidental: it is
// also the file-CREATE path (dossierx claim new, which refuses to run if the
// file already exists), so it cannot assume a document to mutate. An absent
// SourcePath means CREATE and emits a fresh document from the struct; an
// existing one means MUTATE and goes through renderClaim, which rewrites only
// the top-level keys whose value actually changed. Both modes end in the same
// verifyRoundTrip + atomicWriteFile.
func SaveClaim(c model.Claim) error {
	if strings.TrimSpace(c.SourcePath) == "" {
		return fmt.Errorf("loader: claim %q has no source path to save to", c.ID)
	}

	existing, err := readFileWithRetry(c.SourcePath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("loader: read %s before save: %w", c.SourcePath, err)
		}
		existing = nil // CREATE: dossierx claim new writes a file that is not there yet.
	}

	data, err := renderClaim(c, existing)
	if err != nil {
		return fmt.Errorf("loader: marshal claim %q: %w", c.ID, err)
	}
	if err := verifyRoundTrip(c, data); err != nil {
		return err
	}
	if err := atomicWriteFile(c.SourcePath, data, 0o644); err != nil {
		return fmt.Errorf("loader: write %s: %w", c.SourcePath, err)
	}
	return nil
}

// claimYAMLIndent is the block indent every claim file this package emits
// uses. It is 2 to match the authored corpus (see
// testdata/fixture-basic/claims/widget-overview.yaml's "body: |" block), not
// yaml.v3's default of 4: renderClaim re-emits the WHOLE document even when it
// only replaces one key, so an emitter that disagreed with the corpus would
// re-indent every block scalar on the first write and give back exactly the
// whole-file diff this indirection exists to remove.
const claimYAMLIndent = 2

// encodeYAML is the single YAML emitter for this package: every byte this
// package writes, and every byte its round-trip probes measure, comes out of
// here at claimYAMLIndent. It takes any so both a model.Claim (CREATE,
// CommentBodyRoundTrips) and a mutated *yaml.Node document (MUTATE) share one
// encoder configuration — the set of comment bodies the save-time guard
// refuses is INDENT-SENSITIVE (two of bodyRoundTripCases()' twenty flip
// between indent 4 and indent 2), so a second emitter with its own settings
// would silently break the by-construction match CommentBodyRoundTrips'
// doc comment below promises.
func encodeYAML(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(claimYAMLIndent)
	if err := enc.Encode(v); err != nil {
		_ = enc.Close() // the Encode error is the interesting one
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// encodeClaim marshals a whole claim as a fresh document, in model.Claim field
// order. It is the CREATE path's emitter and the probe CommentBodyRoundTrips
// measures; MUTATE uses it too, as the source of the replacement value for
// each key it rewrites.
func encodeClaim(c model.Claim) ([]byte, error) {
	return encodeYAML(c)
}

// renderClaim produces the bytes to write for c. existing is the file's
// current on-disk content, or nil when there is no file yet (CREATE).
//
// With an existing document it re-emits that document with only the changed
// top-level keys replaced, so an unchanged key keeps its AUTHORED style —
// quoting, block-scalar form (literal vs folded), key order and YAML comments —
// instead of being re-serialised from the struct. That is the whole point:
// adding one comment used to land as a 117-line diff in which exactly one key
// was new.
//
// One thing is NOT preserved, by design: block-scalar INDENT WIDTH. The merged
// tree is re-emitted through encodeYAML at claimYAMLIndent, so an authored
// `body: |` whose content sits at 4 spaces comes back at 2 even on a no-op
// save. Normalising it is what makes the merged bytes safe to hand to
// verifyRoundTrip in place of the fresh whole-document emission.
//
// Any reason the merge cannot be done faithfully (the file is not a single
// YAML mapping document, or the fresh emission does not itself re-parse)
// falls back to the fresh whole-document bytes, which is the pre-existing
// behaviour — and, for a non-round-tripping claim, is exactly the input
// verifyRoundTrip must see in order to refuse the write.
func renderClaim(c model.Claim, existing []byte) ([]byte, error) {
	fresh, err := encodeClaim(c)
	if err != nil {
		return nil, err
	}
	if len(existing) == 0 {
		return fresh, nil
	}
	if merged, ok := mergeTopLevelKeys(existing, fresh); ok {
		return merged, nil
	}
	return fresh, nil
}

// mergeTopLevelKeys applies fresh's top-level keys onto existing's node tree
// and re-emits it, reporting false if either side is not a single YAML mapping
// document (in which case the caller re-emits fresh wholesale). The key
// contract, in full:
//
//   - a key in both, whose two values decode to the same Go value, keeps the
//     EXISTING node untouched — that is what preserves authored style (except
//     block-scalar indent width; see renderClaim);
//   - a key in both whose decoded values differ takes the fresh node;
//   - a key only in fresh is APPENDED at the end, and because fresh is emitted
//     in model.Claim field order, several appends land in field order too;
//   - a key only in existing is REMOVED. "Absent from fresh" is exactly "zero
//     and omitempty", which is how the engine expresses a clear: reaudit sets
//     raw_html_reviewed back to false, and resolving the last thread empties
//     comments. Both are pinned by loader_test.go, and keeping such a key would
//     also make any file the struct cannot re-emit permanently unwritable,
//     because verifyRoundTrip's strict decode would reject the leftover.
//
// The result is therefore VALUE-identical to the fresh whole-document emission
// and differs from it only in style — which is the property that makes it safe
// to hand to verifyRoundTrip in place of those bytes.
func mergeTopLevelKeys(existing, fresh []byte) ([]byte, bool) {
	haveDoc, ok := singleMappingDocument(existing)
	if !ok {
		return nil, false
	}
	wantDoc, ok := singleMappingDocument(fresh)
	if !ok {
		return nil, false
	}
	have, want := haveDoc.Content[0], wantDoc.Content[0]

	kept := have.Content[:0]
	for i := 0; i+1 < len(have.Content); i += 2 {
		if indexOfKey(want, have.Content[i].Value) >= 0 {
			kept = append(kept, have.Content[i], have.Content[i+1])
		}
	}
	have.Content = kept

	for i := 0; i+1 < len(want.Content); i += 2 {
		key, value := want.Content[i], want.Content[i+1]
		at := indexOfKey(have, key.Value)
		if at < 0 {
			have.Content = append(have.Content, key, value)
			continue
		}
		same, err := sameDecodedValue(have.Content[at+1], value)
		if err != nil {
			return nil, false
		}
		if !same {
			have.Content[at+1] = value
		}
	}

	out, err := encodeYAML(haveDoc)
	if err != nil {
		return nil, false
	}
	return out, true
}

// singleMappingDocument decodes data as exactly one YAML mapping document,
// mirroring LoadClaims' one-claim-per-file discipline (a second document is a
// hard error there, so it is a "cannot merge this" here rather than something
// to silently drop).
func singleMappingDocument(data []byte) (*yaml.Node, bool) {
	var doc yaml.Node
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&doc); err != nil {
		return nil, false
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, false
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, false
	}
	return &doc, true
}

// indexOfKey returns the index of key's KEY node in a mapping node's flat
// key/value Content slice, or -1.
func indexOfKey(mapping *yaml.Node, key string) int {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return i
		}
	}
	return -1
}

// sameDecodedValue reports whether two nodes carry the same value, compared
// after decoding rather than by node identity or raw bytes: a scalar that
// changed only its quoting style, or a mapping that changed only its
// indentation, is NOT a change worth rewriting the file for.
func sameDecodedValue(a, b *yaml.Node) (bool, error) {
	var av, bv any
	if err := a.Decode(&av); err != nil {
		return false, err
	}
	if err := b.Decode(&bv); err != nil {
		return false, err
	}
	return reflect.DeepEqual(av, bv), nil
}

// verifyRoundTrip is the systemic store-bricking guard: it decodes c's freshly
// marshaled bytes exactly the way LoadClaims will (a strict, single-document
// decode) and refuses the write — returning ErrClaimNotRoundTrippable — if that
// decode fails or the comment/reply bodies (the user-authored free text yaml.v3
// mishandles) do not come back byte-exact. Every claim-file write (lock,
// unlock, reaudit, flag, comment ops) passes through here, so a claim whose
// YAML would not re-parse is never persisted and the next whole-dir LoadClaims
// can never fail on a file this engine wrote. It is a pure check on already
// marshaled bytes: valid claims (every claim the engine writes today) decode
// cleanly and are unaffected.
func verifyRoundTrip(c model.Claim, data []byte) error {
	var back model.Claim
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&back); err != nil {
		return fmt.Errorf("loader: claim %q: marshaled YAML does not re-parse (%w): %w", c.ID, err, ErrClaimNotRoundTrippable)
	}
	if err := commentBodiesRoundTrip(c.Comments, back.Comments); err != nil {
		return fmt.Errorf("loader: claim %q: %w: %w", c.ID, err, ErrClaimNotRoundTrippable)
	}
	if c.Body != back.Body {
		return fmt.Errorf("loader: claim %q: body did not round-trip byte-exact: %w", c.ID, ErrClaimNotRoundTrippable)
	}
	return nil
}

// CommentBodyRoundTrips reports whether body can be stored as a comment (or
// reply) body and read back BYTE-EXACT through the very marshal + strict-decode
// the save-time guard (verifyRoundTrip) applies. It is the shared, round-trip-
// ACCURATE pre-check the comments input boundary (comments.validateBody) uses so
// it rejects EXACTLY the bodies the save-time guard would refuse — matching that
// guard BY CONSTRUCTION rather than by a hand-rolled leading-whitespace
// heuristic, which both MISSED store-bricking bodies (a first CONTENT line that
// itself begins with a tab or space indent, e.g. "\tcode\nmore" or
// "    code\n    more") and FALSE-REJECTED bodies that actually round-trip
// (" \n…", "\r\n…", a NBSP/NEL/VT/FF-led first line).
//
// yaml.v3 v3.0.1 emits certain leading-whitespace bodies as a block scalar it
// then cannot re-parse (a bare leading newline, a leading blank/whitespace-only
// line) or re-parses lossily (a space-indented first content line, whose block
// indent indicator is stripped on read); persisting one bricks the whole claims
// dir on the next LoadClaims. A minimal claim carrying body as its single
// comment body is marshaled and strict-decoded here; the reply-body nesting
// round-trips identically (verified empirically against v3.0.1), so probing the
// thread-body position alone is faithful. Empty/whitespace-only bodies are not
// this function's concern (comments.validateBody rejects those as ErrEmptyBody
// first); it is called only on bodies that carry real content.
func CommentBodyRoundTrips(body string) bool {
	probe := model.Claim{
		ID:       "probe",
		Comments: []model.Comment{{ID: "c", Body: body}},
	}
	data, err := encodeClaim(probe)
	if err != nil {
		return false
	}
	var back model.Claim
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&back); err != nil {
		return false
	}
	return len(back.Comments) == 1 && back.Comments[0].Body == body
}

// commentBodiesRoundTrip reports whether every thread and reply body in want is
// reproduced byte-exact in got (the marshal->decode image of want). A count or
// body mismatch is the silent-corruption sibling of an outright parse failure,
// and equally a reason to refuse the write.
func commentBodiesRoundTrip(want, got []model.Comment) error {
	if len(want) != len(got) {
		return fmt.Errorf("comment thread count changed on round-trip (%d -> %d)", len(want), len(got))
	}
	for i := range want {
		if want[i].Body != got[i].Body {
			return fmt.Errorf("comment %q body did not round-trip byte-exact", want[i].ID)
		}
		if len(want[i].Replies) != len(got[i].Replies) {
			return fmt.Errorf("comment %q reply count changed on round-trip (%d -> %d)", want[i].ID, len(want[i].Replies), len(got[i].Replies))
		}
		for j := range want[i].Replies {
			if want[i].Replies[j].Body != got[i].Replies[j].Body {
				return fmt.Errorf("comment %q reply %q body did not round-trip byte-exact", want[i].ID, want[i].Replies[j].ID)
			}
		}
	}
	return nil
}

// ErrClaimFileChanged is returned by SaveClaimIfUnchanged when the target
// claim file's on-disk content no longer matches the snapshot the caller
// captured (via CaptureClaimFileToken) before mutating it — i.e. some other
// writer changed the file underneath this load->mutate->save sequence. It is
// a distinct, matchable sentinel (errors.Is) so a caller such as a future
// HTTP server can map it to a 409 Conflict / "reload, this claim changed"
// response rather than a generic 500.
var ErrClaimFileChanged = errors.New("claim file changed on disk since it was loaded")

// ErrClaimNotRoundTrippable is returned by SaveClaim/SaveClaimIfUnchanged when
// the claim's marshaled YAML would not decode back into the same claim — i.e.
// writing it would leave a file the very next LoadClaims cannot parse, bricking
// the whole claims dir (every loader-backed command then fails to load
// anything). The canonical trigger is a comment/reply body whose leading
// whitespace drives yaml.v3 v3.0.1 to emit a block scalar it cannot re-parse (a
// leading newline, a leading blank line, or a leading whitespace-only line). The
// save path REFUSES such a write and returns this matchable sentinel instead of
// persisting the store-bricking file, so no writer — present or future — can
// brick the store, even if it skipped the higher-level body validation.
var ErrClaimNotRoundTrippable = errors.New("claim would not round-trip through YAML; refusing to write a store-bricking file")

// ClaimFileToken is an opaque snapshot of a claim file's on-disk content at
// load time, handed back to SaveClaimIfUnchanged so it can refuse to
// overwrite a file that changed underneath the caller. It records the file's
// byte length and a content hash; a content hash (rather than only mtime+size)
// is used deliberately, so the check is robust to same-size edits and to
// coarse filesystem mtime granularity — a hash mismatch is exactly "the bytes
// differ", with no timestamp-resolution guesswork.
type ClaimFileToken struct {
	size int64
	hash string
}

// tokenForBytes derives a ClaimFileToken from a file's raw bytes.
func tokenForBytes(data []byte) ClaimFileToken {
	sum := sha256.Sum256(data)
	return ClaimFileToken{size: int64(len(data)), hash: hex.EncodeToString(sum[:])}
}

// CaptureClaimFileToken snapshots path's current on-disk content for a later
// SaveClaimIfUnchanged optimistic-concurrency check. Call it right after
// loading the claim (inside the same claims-sentinel critical section) so the
// token reflects the bytes the caller is about to mutate. A file that cannot
// be read is an error, not an empty token: callers only ever snapshot a claim
// they just loaded, so an unreadable file there is a real failure, never the
// expected "fresh, absent store" case LoadStore/LoadFlagStore tolerate.
func CaptureClaimFileToken(path string) (ClaimFileToken, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ClaimFileToken{}, fmt.Errorf("loader: snapshot %s: %w", path, err)
	}
	return tokenForBytes(data), nil
}

// SaveClaimIfUnchanged is SaveClaim guarded by an optimistic-concurrency
// check: it writes c back to its SourcePath only if that file's current
// on-disk content still matches want (the token the caller captured at load
// time via CaptureClaimFileToken). If the file changed underneath, it writes
// nothing and returns ErrClaimFileChanged.
//
// This is a best-effort backstop layered UNDER the project-wide claims
// sentinel (see cmd/dossierx's claimsSentinelPath), not a replacement for it.
// The sentinel serializes every cooperating claim-file writer; this check
// additionally catches an out-of-band edit (a text editor, or a future writer
// that forgets the sentinel) — the one class of change the sentinel alone
// cannot see. The re-read/compare/write is deliberately not a single atomic
// transaction (a change slipped in after the compare but before the rename
// would be missed), which is precisely why it BACKS the sentinel rather than
// standing in for it.
func SaveClaimIfUnchanged(c model.Claim, want ClaimFileToken) error {
	if strings.TrimSpace(c.SourcePath) == "" {
		return fmt.Errorf("loader: claim %q has no source path to save to", c.ID)
	}

	current, err := os.ReadFile(c.SourcePath)
	if err != nil {
		return fmt.Errorf("loader: re-read %s before optimistic save: %w", c.SourcePath, err)
	}
	if tokenForBytes(current) != want {
		return fmt.Errorf("loader: %s: %w", c.SourcePath, ErrClaimFileChanged)
	}

	data, err := renderClaim(c, current)
	if err != nil {
		return fmt.Errorf("loader: marshal claim %q: %w", c.ID, err)
	}
	if err := verifyRoundTrip(c, data); err != nil {
		return err
	}
	if err := atomicWriteFile(c.SourcePath, data, 0o644); err != nil {
		return fmt.Errorf("loader: write %s: %w", c.SourcePath, err)
	}
	return nil
}

// atomicWriteFile writes data to path without ever leaving a reader able to
// observe a partially-written file: it writes to a temp file created in
// path's own directory (so the later rename stays on one filesystem, which
// is what makes it atomic) and then renames it over path.
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
	return renameWithRetry(tmpPath, path)
}

// renameWithRetry is os.Rename with the same short, bounded Windows-only
// retry as readFileWithRetry, for the symmetric direction of the same
// race: a rename-over-path can transiently collide with another process
// currently holding path open for read (ERROR_SHARING_VIOLATION).
func renameWithRetry(oldpath, newpath string) error {
	if runtime.GOOS != "windows" {
		return os.Rename(oldpath, newpath)
	}
	const attempts = 5
	var err error
	for i := 0; i < attempts; i++ {
		err = os.Rename(oldpath, newpath)
		if err == nil {
			return nil
		}
		if i < attempts-1 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	return err
}

// FindByID returns the claim with the given id, if present.
func FindByID(claims []model.Claim, id string) (model.Claim, bool) {
	for _, c := range claims {
		if c.ID == id {
			return c, true
		}
	}
	return model.Claim{}, false
}
