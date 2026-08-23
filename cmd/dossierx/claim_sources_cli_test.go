// claim_sources_cli_test.go covers the claim leaves' half of the sources
// feature: what "claim show" reports about the evidence a claim rests on, what
// "claim list" reports about it, and the property that made the model type worth
// having at all — that a citation is only checkable if its ANCHOR travels with
// it.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSourcedClaimFixture writes one claim carrying both kinds of source, and
// one carrying none. The pair is the point: every assertion about the sourced
// claim has to be read against a claim in the same project that cites nothing,
// or "the field is populated" cannot be told apart from "the field is always
// populated".
func writeSourcedClaimFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	sourced := "id: widget.contract.retry-policy\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  requests retry three times with backoff [1], and the ceiling is fixed [2].\n" +
		"sources:\n" +
		"  - ref: 1\n    kind: external\n    title: Vendor retry guidance\n" +
		"    url: https://example.invalid/retries\n    accessed_on: 2026-08-01\n" +
		"    supports: three attempts with exponential backoff\n" +
		"    does_not_support: the specific ceiling value\n" +
		"  - ref: 2\n    kind: internal\n    title: Extraction ledger row\n" +
		"    path: research/ledger.jsonl\n    record_id: r-17\n" +
		"    sha256: 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n" +
		"governed_by:\n  type: none\n  reason: fixture claim\n"
	if err := os.WriteFile(filepath.Join(claimsDir, "retry.yaml"), []byte(sourced), 0o644); err != nil {
		t.Fatalf("write sourced claim: %v", err)
	}

	bare := "id: widget.contract.timeout-budget\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  the total time budget across retries.\n" +
		"governed_by:\n  type: none\n  reason: fixture claim\n"
	if err := os.WriteFile(filepath.Join(claimsDir, "timeout.yaml"), []byte(bare), 0o644); err != nil {
		t.Fatalf("write unsourced claim: %v", err)
	}
	return cfgPath
}

// TestClaimShowCarriesEverySourceWithItsAnchor is the assertion the feature
// exists for. Provenance that lives outside the claim is provenance nothing can
// check, and a citation reported WITHOUT its anchor — the date an external page
// was read, the hash an internal record was pinned at — is locatable but not
// falsifiable. Those are exactly the fields a summary would drop.
func TestClaimShowCarriesEverySourceWithItsAnchor(t *testing.T) {
	cfgPath := writeSourcedClaimFixture(t)

	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "show", "widget.contract.retry-policy")
	if err != nil {
		t.Fatalf("claim show: %v (%+v)", err, env)
	}
	var data claimShowData
	envData(t, env, &data)

	if len(data.Sources) != 2 {
		t.Fatalf("expected both sources, got %+v", data.Sources)
	}

	// Authored order, and the author-assigned Ref that the body's "[n]" markers
	// resolve against — not a position-derived index.
	external := data.Sources[0]
	if external.Ref != 1 || external.Kind != "external" {
		t.Fatalf("the first source is the external one at ref 1: %+v", external)
	}
	if external.URL == "" || external.AccessedOn != "2026-08-01" {
		t.Fatalf("an external source is anchored by URL PLUS the date it was read: %+v", external)
	}
	// The author's own statement of the citation's LIMIT. The common citation
	// defect is not a fabricated source but an overread one, and this is the
	// field that lets a later reader see the boundary without reconstructing the
	// original reasoning.
	if external.DoesNotSupport != "the specific ceiling value" {
		t.Fatalf("does_not_support must survive into the envelope: %+v", external)
	}
	if external.Supports == "" {
		t.Fatalf("supports must survive into the envelope: %+v", external)
	}

	internal := data.Sources[1]
	if internal.Ref != 2 || internal.Kind != "internal" {
		t.Fatalf("the second source is the internal one at ref 2: %+v", internal)
	}
	if internal.Path != "research/ledger.jsonl" || internal.RecordID != "r-17" {
		t.Fatalf("an internal source names the file and, when narrowed, the record: %+v", internal)
	}
	if len(internal.SHA256) != 64 {
		t.Fatalf("an internal source with no hash cannot be checked: %+v", internal)
	}

	// A claim that cites nothing reports an ARRAY, not null: a consumer must be
	// able to range over data.sources without first testing it.
	bare, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "show", "widget.contract.timeout-budget")
	if err != nil {
		t.Fatalf("claim show (unsourced): %v", err)
	}
	var bareData claimShowData
	envData(t, bare, &bareData)
	if bareData.Sources == nil {
		t.Fatalf("sources must be an empty array for a claim that cites nothing, not null")
	}
	if len(bareData.Sources) != 0 {
		t.Fatalf("expected no sources, got %+v", bareData.Sources)
	}
}

// TestClaimShowTextRendersSourcesReadably: the JSON is the contract and the
// prose is the courtesy, but the prose has to carry the same two things — the
// marker a reader matches against the body, and the anchor that makes the
// citation checkable.
func TestClaimShowTextRendersSourcesReadably(t *testing.T) {
	cfgPath := writeSourcedClaimFixture(t)

	out, _, err := execCLI(t, "--config", cfgPath, "claim", "show", "widget.contract.retry-policy")
	if err != nil {
		t.Fatalf("claim show --format text: %v (out %q)", err, out)
	}
	for _, want := range []string{
		"source [1]:",
		"Vendor retry guidance",
		"2026-08-01",
		"source [2]:",
		"research/ledger.jsonl#r-17",
		"does not support: the specific ceiling value",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("the text rendering must carry %q, got:\n%s", want, out)
		}
	}

	// Nothing at all for a claim with no citations: a "(none)" line on every card
	// in a corpus that has not adopted sources is noise on every card.
	bare, _, err := execCLI(t, "--config", cfgPath, "claim", "show", "widget.contract.timeout-budget")
	if err != nil {
		t.Fatalf("claim show --format text (unsourced): %v", err)
	}
	// Matched on the "source [n]:" LABEL, not the bare word: next_actions quotes
	// lint findings verbatim, and a source-* rule firing anywhere in the project
	// puts the word into every claim's advice block.
	if strings.Contains(bare, "source [") {
		t.Fatalf("a claim that cites nothing must print no source lines, got:\n%s", bare)
	}
}

// TestClaimListCountsSourcesPerClaim pins the minimal thing a LIST can honestly
// carry about evidence. The citations themselves are ten fields each and belong
// to "claim show"; the count answers the one question a list is asked — which
// claims have no evidence at all — that is otherwise a call per claim.
func TestClaimListCountsSourcesPerClaim(t *testing.T) {
	cfgPath := writeSourcedClaimFixture(t)

	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "list")
	if err != nil {
		t.Fatalf("claim list: %v (%+v)", err, env)
	}
	var data claimListData
	envData(t, env, &data)

	got := map[string]int{}
	for _, e := range data.Claims {
		got[e.ClaimID] = e.Sources
	}
	if got["widget.contract.retry-policy"] != 2 {
		t.Fatalf("expected the sourced claim to count 2, got %+v", got)
	}
	if got["widget.contract.timeout-budget"] != 0 {
		t.Fatalf("expected the unsourced claim to count 0, got %+v", got)
	}

	// The text form flags it only where there is something to flag, exactly like
	// open_threads: a column of zeroes is a column readers learn to skip.
	out, _, err := execCLI(t, "--config", cfgPath, "claim", "list")
	if err != nil {
		t.Fatalf("claim list --format text: %v", err)
	}
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.Contains(line, "widget.contract.retry-policy"):
			if !strings.Contains(line, "sources=2") {
				t.Fatalf("expected sources=2 on the sourced claim's line, got %q", line)
			}
		case strings.Contains(line, "widget.contract.timeout-budget"):
			if strings.Contains(line, "sources=") {
				t.Fatalf("expected no sources token on the unsourced claim's line, got %q", line)
			}
		}
	}
}

// TestClaimShowReportsTrackMembershipWithTheRoleResolved is the inverse lookup
// of "dossierx track show", and it is here because claim show is where an agent
// orients itself on one card: a claim that OWNS a track — whose body is a
// feature's own prose rather than one module's contract — would otherwise read
// exactly like any other claim.
//
// Role is the EFFECTIVE role. A membership that omits it means cites, and
// echoing the empty string would make every consumer re-implement that default
// in the one place it could be re-implemented differently.
func TestClaimShowReportsTrackMembershipWithTheRoleResolved(t *testing.T) {
	cfgPath, _ := writeTrackFixture(t)

	owner, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "show", "checkout.contract.guest-flow")
	if err != nil {
		t.Fatalf("claim show (owner): %v", err)
	}
	var ownerData claimShowData
	envData(t, owner, &ownerData)
	if len(ownerData.Tracks) != 1 || ownerData.Tracks[0].TrackID != "guest-checkout" {
		t.Fatalf("expected the owned track membership, got %+v", ownerData.Tracks)
	}
	if ownerData.Tracks[0].Role != "owns" {
		t.Fatalf("expected role owns, got %+v", ownerData.Tracks[0])
	}

	implicit, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "show", "payments.contract.card-capture")
	if err != nil {
		t.Fatalf("claim show (implicit role): %v", err)
	}
	var implicitData claimShowData
	envData(t, implicit, &implicitData)
	if len(implicitData.Tracks) != 1 || implicitData.Tracks[0].Role != "cites" {
		t.Fatalf("an omitted role must be resolved to cites before it reaches the envelope, got %+v", implicitData.Tracks)
	}

	none, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "show", "payments.contract.settlement")
	if err != nil {
		t.Fatalf("claim show (no tracks): %v", err)
	}
	var noneData claimShowData
	envData(t, none, &noneData)
	if noneData.Tracks == nil {
		t.Fatalf("tracks must be an empty array for a claim in no track, not null")
	}
	if len(noneData.Tracks) != 0 {
		t.Fatalf("expected no memberships, got %+v", noneData.Tracks)
	}
}
