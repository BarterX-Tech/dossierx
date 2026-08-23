// source_test.go covers the YAML round-trip of model.Claim's sources field
// and the per-kind field layout model.Source declares. It follows the
// round-trip / optional-and-omitted pair every other optional claim field in
// claim_test.go is covered by; the second half of each pair is the one that
// matters most here, because "a claim that carries no sources serializes
// exactly as it did before this field existed" is a compatibility promise the
// lock ledger depends on (see lock.lockedClaimHashOmitWhenEmpty).
package model

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const sourcesDoc = `id: widget.contract.a
facet: contract
status: draft
body: |
  The vendor documents no such property [1], and our own requirement
  record says the same [2].
governed_by:
  type: none
  reason: fixture
sources:
  - ref: 1
    kind: external
    title: SCShareableContent
    url: https://developer.apple.com/documentation/screencapturekit/scshareablecontent
    accessed_on: 2026-08-15
    supports: Represents the set of displays, apps and windows the calling app can capture.
    does_not_support: Provides no sharing-state field for other processes.
  - ref: 2
    kind: internal
    title: Product requirement PVR-010
    path: migration/synthesis/product-requirement-map.jsonl
    record_id: PVR-010
    sha256: 8afd3c9a0813b08551df24f85182172ad44f19d4392e0773981310228c6fcadf
`

func TestClaim_Sources_RoundTrip(t *testing.T) {
	c := decodeClaim(t, sourcesDoc)

	if len(c.Sources) != 2 {
		t.Fatalf("len(Sources) = %d, want 2", len(c.Sources))
	}

	ext := c.Sources[0]
	if ext.Ref != 1 {
		t.Errorf("Sources[0].Ref = %d, want 1", ext.Ref)
	}
	if !ext.IsExternal() || ext.IsInternal() {
		t.Errorf("Sources[0] kind %q: IsExternal=%v IsInternal=%v, want true/false", ext.Kind, ext.IsExternal(), ext.IsInternal())
	}
	if ext.URL == "" || ext.AccessedOn != "2026-08-15" {
		t.Errorf("Sources[0] external anchor did not decode: url=%q accessed_on=%q", ext.URL, ext.AccessedOn)
	}
	if ext.DoesNotSupport == "" {
		t.Error("Sources[0].DoesNotSupport did not decode; the field that records what a citation does NOT establish is the one that keeps an overread source honest")
	}

	in := c.Sources[1]
	if !in.IsInternal() || in.IsExternal() {
		t.Errorf("Sources[1] kind %q: IsInternal=%v IsExternal=%v, want true/false", in.Kind, in.IsInternal(), in.IsExternal())
	}
	if in.Path == "" || in.RecordID != "PVR-010" || in.SHA256 == "" {
		t.Errorf("Sources[1] internal anchor did not decode: path=%q record_id=%q sha256=%q", in.Path, in.RecordID, in.SHA256)
	}

	out, err := yaml.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	reloaded := decodeClaim(t, string(out))
	if len(reloaded.Sources) != len(c.Sources) {
		t.Fatalf("Sources did not round-trip: got %d entries, want %d\n%s", len(reloaded.Sources), len(c.Sources), out)
	}
	for i := range c.Sources {
		if reloaded.Sources[i] != c.Sources[i] {
			t.Errorf("Sources[%d] did not round-trip:\n got %+v\nwant %+v", i, reloaded.Sources[i], c.Sources[i])
		}
	}
}

// TestClaim_Sources_OptionalAndOmitted is the compatibility half of the pair.
// A claim that carries no sources must serialize with no `sources:` key at
// all — not an empty list — because every claim written before this field
// existed loads as a nil slice, and lock.LockedClaimHash's compatibility gate
// is keyed on that emptiness. An `omitempty` lost here would rewrite every
// claim file in every project on the next save and report content drift on
// every locked one.
func TestClaim_Sources_OptionalAndOmitted(t *testing.T) {
	c := decodeClaim(t, "id: widget.contract.a\nfacet: contract\nstatus: draft\ngoverned_by:\n  type: none\n  reason: fixture\n")
	if c.Sources != nil {
		t.Fatalf("Sources = %+v, want nil when omitted", c.Sources)
	}

	out, err := yaml.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "sources:") {
		t.Fatalf("expected omitempty to drop an unset sources field, got:\n%s", out)
	}
}

// TestSourceKindValues pins the two enum values as the strings they appear as
// on disk. They are a closed vocabulary the lint suite validates against, so a
// rename here is a schema break for every project that has authored a source.
func TestSourceKindValues(t *testing.T) {
	if SourceKindExternal != "external" {
		t.Errorf("SourceKindExternal = %q, want %q", SourceKindExternal, "external")
	}
	if SourceKindInternal != "internal" {
		t.Errorf("SourceKindInternal = %q, want %q", SourceKindInternal, "internal")
	}
}

// TestSourceKindPredicatesRejectAnUnknownKind checks the predicates fail
// CLOSED. An unrecognized kind must answer false to BOTH questions rather
// than defaulting into one of them: a source that silently counted as
// external would be checked for a URL it was never meant to have, and one
// that silently counted as internal would be hashed against a path it does
// not name. Neither is a check the author asked for, and the source-shape
// lint is what reports the real problem.
func TestSourceKindPredicatesRejectAnUnknownKind(t *testing.T) {
	s := Source{Ref: 1, Kind: SourceKind("archival"), Title: "Something"}
	if s.IsExternal() || s.IsInternal() {
		t.Errorf("Source with kind %q answered IsExternal=%v IsInternal=%v; an unknown kind must be neither", s.Kind, s.IsExternal(), s.IsInternal())
	}

	var unset Source
	if unset.IsExternal() || unset.IsInternal() {
		t.Error("a Source with no kind set answered true to one of the kind predicates; the zero value must be neither")
	}
}
