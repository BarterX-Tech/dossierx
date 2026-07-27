// lockedhash.go implements LockedClaimHash — the hash the LOCK LEDGER signs a
// locked claim's approved content with. It is deliberately a SECOND, separate
// hash from ContentHash, and the distinction is the single most important
// decision in this file:
//
//   - ContentHash answers "would a DEPENDENT of this claim need to re-review?"
//     It hashes a small, hand-picked allowlist of ten fields, and it must stay
//     byte-identical forever: it is the baseline recorded in Store.Hashes for
//     every locked claim's dependencies, so widening it would make every
//     already-recorded baseline mismatch and flip every locked claim in every
//     existing project to review_pending on the day they upgrade.
//
//   - LockedClaimHash answers "is THIS locked claim still the bytes a human
//     approved?" That question has no allowlist: every field a claim persists
//     is part of what was approved. model.Claim persists twenty-two yaml-tagged
//     fields; ContentHash covers ten. The nine ContentHash cannot see —
//     raw_html, raw_html_reviewed, build_role, kind, section, order, emphasis,
//     migrated_from, audit_notes — include the only path in this entire
//     codebase that renders author bytes UNESCAPED (render/components: a locked
//     claim with raw_html_reviewed true, in an allowlisted module, has its
//     raw_html returned as trusted HTML). A ledger built on ContentHash would
//     certify a hand-swapped raw-HTML payload as unchanged — it would sign the
//     one edit that most needs a signature. So LockedClaimHash exists.
//
// LockedClaimHash is therefore built as a DENY-LIST over reflection rather than
// an allowlist over named fields: it hashes every field of model.Claim that
// yaml.v3 puts on disk (see PersistedYAMLName, which decides that question and
// decides it fail-closed) EXCEPT the three engine-managed ones below. A field
// added to model.Claim tomorrow is covered by default — the failure mode of a deny-list
// is "we hashed something we did not need to" (a spurious drift report, loud
// and recoverable), while the failure mode of an allowlist is "we silently
// stopped signing a field" (a blessed tamper, silent and permanent). The
// reflection test in lockedhash_test.go additionally refuses to compile a new
// field into the schema without an explicit, written include/exclude decision.
package lock

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// lockedClaimHashVersion is the version of the HASH ALGORITHM below, mixed into
// the hash as a domain separator. Bumping it changes every claim's
// LockedClaimHash, which would make every existing ledger record look like
// content drift — so a bump is a re-adoption event and must be accompanied by a
// ledgerSchemaVersion bump, which is precisely what re-triggers grandfathering
// (see AdoptProject). Stated the other way round: the grandfathering machinery
// IS the hash-migration machinery, so there is exactly one upgrade path to
// maintain rather than two.
const lockedClaimHashVersion = 1

// lockedClaimHashExcluded is THE DENY-LIST: the yaml tags of the only
// model.Claim fields LockedClaimHash does not sign. Every other persisted field
// is signed, including any field added later. Each exclusion is here because
// the engine itself rewrites the field as ordinary bookkeeping, so signing it
// would make routine, already-gated operations look like tampering:
//
//   - status: the ledger's whole job is to notice a status flip, and it does so
//     by the PRESENCE or ABSENCE of a record (lock-ledger-missing /
//     lock-ledger-orphan), never by hashing the field. Hashing it too would be
//     circular: Lock records the hash of the claim it is about to lock, so the
//     recorded hash would have to encode a status the on-disk claim does not
//     have yet, and Unlock's legitimate flip to draft would read as drift.
//
//   - review_pending: set automatically by three independent triggers (a
//     dependency's content drifting, a "claim flag", a comment thread opening)
//     with no human in the loop by design. Signing it would report drift every
//     time the engine's own reconcile pass did its job.
//
//   - comments: written by internal/comments on every comment op — including
//     from "dossierx serve", which has no write authority over the lock store.
//     Comment integrity is covered instead by its OWN store (internal/digest),
//     for exactly that reason; see that package's doc comment.
var lockedClaimHashExcluded = map[string]bool{
	"status":         true,
	"review_pending": true,
	"comments":       true,
}

// LockedClaimHash returns a deterministic hash over every persisted field of c
// except the three in lockedClaimHashExcluded. It is what a lock-ledger record
// stores, and what the ledger gate re-computes to decide whether a locked claim
// still holds the content a human approved.
//
// Two properties callers rely on:
//
//   - It is INDEPENDENT of c.Status, so Lock may hash the claim either before
//     or after flipping it to locked and get the same answer. That removes a
//     whole class of ordering bug from every write hook.
//
//   - It is stable across a YAML save/load round-trip, because it hashes the
//     decoded struct rather than the file bytes: loader.SaveClaim rewrites a
//     claim's whole file (key order, quoting, indentation), so a byte hash of
//     the file would report drift on every legitimate write.
func LockedClaimHash(c model.Claim) string {
	h := sha256.New()
	// Domain separation: the version prefix means a future algorithm change
	// cannot accidentally collide with a hash produced by this one.
	fmt.Fprintf(h, "dossierx-locked-claim/v%d\n", lockedClaimHashVersion)
	hashStructFields(h, reflect.ValueOf(c), lockedClaimHashExcluded)
	return hex.EncodeToString(h.Sum(nil))
}

// hashStructFields writes every exported, yaml-persisted field of the struct v
// into h as "name=<encoded value>\n", skipping any name in exclude.
//
// Fields are emitted in on-disk-name alphabetical order, NOT struct declaration
// order, on purpose: reordering the fields of model.Claim is a pure source
// refactor that changes nothing on disk, and it must not invalidate every
// ledger record in every project.
//
// WHICH FIELDS ARE PERSISTED IS DECIDED BY persistedYAMLName, NOT BY "does it
// have a yaml tag". This used to skip every field whose tag was "" or "-", and
// the "" half was a hole in the deny-list: yaml.v3 persists an exported field
// with NO yaml tag under its lower-cased Go name, so such a field would be
// written to the claim file, read back from it, and yet excluded from the very
// signature that is supposed to cover "every field a claim persists". A
// deny-list that fails open on the shape nobody remembered to tag is not a
// deny-list. Only `yaml:"-"` (model.Claim.SourcePath, which the loader fills in
// from the filesystem) is genuinely not on disk, and it is the only thing
// skipped here.
//
// exclude applies only at this level; nested structs (model.Governed) hash all
// of their own fields, since the deny-list names top-level claim fields.
func hashStructFields(h io.Writer, v reflect.Value, exclude map[string]bool) {
	t := v.Type()
	type taggedField struct {
		tag string
		val reflect.Value
	}
	fields := make([]taggedField, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		tag, persisted := PersistedYAMLName(sf)
		if !persisted {
			continue
		}
		if exclude[tag] {
			continue
		}
		fields = append(fields, taggedField{tag: tag, val: v.Field(i)})
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].tag < fields[j].tag })

	for _, f := range fields {
		fmt.Fprintf(h, "%s=", f.tag)
		hashValue(h, f.val)
		fmt.Fprint(h, "\n")
	}
}

// hashValue writes a canonical, unambiguous encoding of v into h.
//
// Every variable-length encoding is LENGTH-PREFIXED, so no two distinct claims
// can produce the same byte stream by concatenation — without that, a body of
// "a\nrests_on=b" and a real rests_on edge could hash identically, which is
// exactly the kind of forgery a content signature must not permit.
func hashValue(h io.Writer, v reflect.Value) {
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		// model.Row's values are `any`; a nil interface and the string "nil"
		// must not collide, hence the distinct marker.
		if v.IsNil() {
			fmt.Fprint(h, "n:")
			return
		}
		hashValue(h, v.Elem())

	case reflect.String:
		s := v.String()
		fmt.Fprintf(h, "s%d:%s", len(s), s)

	case reflect.Bool:
		fmt.Fprintf(h, "b:%t", v.Bool())

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fmt.Fprintf(h, "i:%d", v.Int())

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		fmt.Fprintf(h, "u:%d", v.Uint())

	case reflect.Float32, reflect.Float64:
		// %v on a float is round-trippable (shortest representation that
		// parses back exactly), so two decodes of the same YAML scalar agree.
		fmt.Fprintf(h, "f:%v", v.Float())

	case reflect.Slice, reflect.Array:
		// A nil slice and an empty slice hash identically. That is correct
		// rather than sloppy: `omitempty` means both serialize to the same
		// absent key, so on disk they ARE the same claim.
		fmt.Fprintf(h, "l%d:[", v.Len())
		for i := 0; i < v.Len(); i++ {
			hashValue(h, v.Index(i))
			fmt.Fprint(h, ",")
		}
		fmt.Fprint(h, "]")

	case reflect.Map:
		// Sorted by the key's canonical string form so Go's randomized map
		// iteration order can never change a claim's hash between two runs.
		// model.Row is the only map in the schema, and it carries its authored
		// column order inside itself (under model's reserved NUL-prefixed key),
		// so hashing every entry — that reserved one included — is what makes a
		// hand-reordered table's columns a detectable change. Column order IS
		// persisted (Row.MarshalYAML writes it back), so it IS part of what was
		// approved.
		entries := make([]mapEntry, 0, v.Len())
		for _, k := range v.MapKeys() {
			entries = append(entries, mapEntry{name: canonicalMapKey(k), key: k})
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
		fmt.Fprintf(h, "m%d:{", len(entries))
		for _, e := range entries {
			fmt.Fprintf(h, "k%d:%s=", len(e.name), e.name)
			hashValue(h, v.MapIndex(e.key))
			fmt.Fprint(h, ",")
		}
		fmt.Fprint(h, "}")

	case reflect.Struct:
		fmt.Fprint(h, "{")
		hashStructFields(h, v, nil)
		fmt.Fprint(h, "}")

	default:
		// No schema field has ever reached here, and the reflection test in
		// lockedhash_test.go forces a written decision before one can. The
		// fallback still hashes the VALUE (via Go's syntax representation)
		// rather than silently hashing nothing, so even an unanticipated kind
		// is signed rather than becoming a hole in the signature.
		fmt.Fprintf(h, "x:%#v", v.Interface())
	}
}

// mapEntry pairs a map key with its canonical string form, so the entries can
// be sorted once by that form rather than re-rendering each key inside the
// comparison.
type mapEntry struct {
	name string
	key  reflect.Value
}

// canonicalMapKey renders a map key as a string for deterministic ordering.
// model.Row's keys are strings, which is the only map the claim schema has;
// anything else falls back to fmt's default rendering rather than being
// dropped.
func canonicalMapKey(k reflect.Value) string {
	if k.Kind() == reflect.String {
		return k.String()
	}
	return fmt.Sprint(k.Interface())
}

// YAMLTagName returns the NAME PART of struct field sf's `yaml:"..."` tag — what
// comes before the first comma — or "" when the field has no yaml tag at all, or
// has one that names nothing (`yaml:",omitempty"`, `yaml:",inline"`).
//
// It answers only "what does the tag say"; it deliberately does NOT answer "is
// this field on disk, and under what name". That second question is
// PersistedYAMLName's, and the two must not be confused: an empty answer here
// means the tag is silent, not that the field is absent from the file.
func YAMLTagName(sf reflect.StructField) string {
	tag, ok := sf.Tag.Lookup("yaml")
	if !ok {
		return ""
	}
	name, _, _ := strings.Cut(tag, ",")
	return name
}

// PersistedYAMLName returns the name yaml.v3 would write struct field sf under,
// and whether the field is persisted at all. It is the single authority on what
// LockedClaimHash signs, and it is written to FAIL CLOSED: a field is treated as
// on disk unless it is provably not.
//
// The three cases, in the order they are decided:
//
//   - UNEXPORTED (PkgPath != ""): never marshalled by yaml.v3, so never on disk.
//     Not persisted.
//
//   - `yaml:"-"`: the author said "do not persist this" and yaml.v3 obeys.
//     model.Claim.SourcePath is the only one in the schema — the loader fills it
//     in from wherever the file happens to live, so signing it would make moving
//     a claim file read as content drift. Not persisted.
//
//   - EVERYTHING ELSE, including a field with no yaml tag whatsoever: persisted.
//     When the tag names nothing — no tag, `yaml:",omitempty"`, `yaml:",inline"`
//     — yaml.v3 falls back to the LOWER-CASED Go field name, and so does this.
//
// That last case is the one this function exists for. The old rule ("no tag
// means not persisted") got it backwards: an exported field added to model.Claim
// without a yaml tag WOULD be written into every claim file under its lowercased
// name and read back out of it, while being invisible to the hash that certifies
// a locked claim's content. The tamper it enabled needs no ledger edit at all —
// change that field on a locked claim and the gate reports nothing. Signing a
// field yaml.v3 turns out not to write costs a spurious drift report, which is
// loud and recoverable; not signing one it does write is a blessed edit, which
// is silent and permanent. An `,inline` field is hashed as a nested struct under
// its lowercased name rather than as promoted keys: the LABEL differs from what
// is on disk, the CONTENT is still fully signed, and fail-closed is the property
// that matters. (model.Claim uses no inline fields today; the schema-decision
// test in lockedhash_test.go forces a written decision before it can.)
//
// It is exported for the same reason its predecessor was: the ledger's
// schema-decision test walks model.Claim with exactly the rules the hasher uses,
// and a second, hand-rolled copy in the test could disagree with this one and
// pass while the hasher silently skipped a field.
func PersistedYAMLName(sf reflect.StructField) (string, bool) {
	if sf.PkgPath != "" {
		return "", false
	}
	name := YAMLTagName(sf)
	if name == "-" {
		return "", false
	}
	if name == "" {
		return strings.ToLower(sf.Name), true
	}
	return name, true
}
