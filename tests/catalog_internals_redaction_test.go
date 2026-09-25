package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// catalog.json is the integration projection: it must never name an
// internals claim (FORMAT.md, "Visibility"), and it must not hide that a
// contract claim is blocked or under review because of one. Before the
// redaction, a contract claim resting on its own module's internals showed
// `edges: {}` while its readiness paths and dependency_id named the
// internals id the catalog never lists, and an internals flag reason was
// copied out verbatim.
func TestCatalog_ContractRestingOnInternalsNamesNoInternalsID(t *testing.T) {
	const (
		api          = "widget.contract.api"
		queue        = "widget.internals.queue"
		secretReason = "queue-secret-flag-reason"
		redacted     = "(internals)"
	)
	root := t.TempDir()
	cfgPath := filepath.Join(root, "project.config.yaml")
	writeProjectConfigFile(t, cfgPath, "schema_version: 1\nfacets:\n  - contract\n  - internals\nmodules:\n  - widget\nclaims_dir: claims\n")
	lockFixtureConstitution(t, root)
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"api.yaml": "id: " + api + "\nsummary: The widget API accepts queued work.\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  The widget API accepts queued work.\nrests_on:\n  - " + queue + "\n",
		"queue.yaml": "id: " + queue + "\nsummary: Work waits in an in-memory queue.\nfacet: internals\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  Work waits in an in-memory queue.\nrests_on:\n  none: true\n  reason: fixture leaf\n",
	} {
		if err := os.WriteFile(filepath.Join(claimsDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	type record struct {
		Kind         string   `json:"kind"`
		SourceKind   string   `json:"source_kind"`
		DependencyID string   `json:"dependency_id"`
		Path         []string `json:"path"`
		Detail       string   `json:"detail"`
	}
	type entry struct {
		ID    string `json:"id"`
		Edges struct {
			RestsOn                 []string `json:"rests_on"`
			RestsOnInternalsOmitted int      `json:"rests_on_internals_omitted"`
		} `json:"edges"`
		Readiness struct {
			LocalApproved   bool     `json:"local_approved"`
			DependencyReady bool     `json:"dependency_ready"`
			ReviewPending   bool     `json:"review_pending"`
			Conditions      []record `json:"conditions"`
			DepConditions   []record `json:"dependency_conditions"`
			Causes          []record `json:"causes"`
			ReviewCauses    []record `json:"review_causes"`
		} `json:"readiness"`
	}
	readAPI := func(step string) entry {
		t.Helper()
		if stdout, stderr, code := run(t, root, "--config", cfgPath, "check"); code != 0 {
			t.Fatalf("%s: check exited %d\nstdout: %s\nstderr: %s", step, code, stdout, stderr)
		}
		raw, err := os.ReadFile(filepath.Join(root, "build", "catalog", "catalog.json"))
		if err != nil {
			t.Fatal(err)
		}
		for _, leaked := range []string{queue, secretReason} {
			if strings.Contains(string(raw), leaked) {
				t.Fatalf("%s: catalog.json names %q, which only an internals claim owns:\n%s", step, leaked, raw)
			}
		}
		var doc struct {
			Claims []entry `json:"claims"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		if len(doc.Claims) != 1 || doc.Claims[0].ID != api {
			t.Fatalf("%s: catalog entries = %+v, want only %s", step, doc.Claims, api)
		}
		e := doc.Claims[0]
		if len(e.Edges.RestsOn) != 0 || e.Edges.RestsOnInternalsOmitted != 1 {
			t.Fatalf("%s: edges = %+v, want no ids and rests_on_internals_omitted 1", step, e.Edges)
		}
		return e
	}

	// A locked contract claim over a draft internals prerequisite: locally
	// approved, not dependency-ready, and the one condition stays visible.
	mustLock(t, root, cfgPath, api)
	e := readAPI("draft internals prerequisite")
	wantCondition := record{Kind: "dependency_unapproved", DependencyID: redacted, Path: []string{api, redacted}, Detail: "required dependency is not locally approved"}
	if !e.Readiness.LocalApproved || e.Readiness.DependencyReady ||
		!reflect.DeepEqual(e.Readiness.Conditions, []record{wantCondition}) ||
		!reflect.DeepEqual(e.Readiness.DepConditions, []record{wantCondition}) {
		t.Fatalf("blocked state must survive redaction: %+v", e.Readiness)
	}

	// A flag on the internals prerequisite reaches the contract claim as a
	// review cause; the flag's authored reason does not.
	mustLock(t, root, cfgPath, queue)
	if stdout, stderr, code := run(t, root, "--config", cfgPath, "claim", "flag", queue,
		"--claim-says", "work waits in memory", "--now-does", "work is persisted", "--reason", secretReason); code != 0 {
		t.Fatalf("flag %s: exit %d\n%s\n%s", queue, code, stdout, stderr)
	}
	e = readAPI("flagged internals prerequisite")
	wantCause := record{Kind: "upstream_dependency_review", SourceKind: "own_flag", Path: []string{api, redacted}, Detail: "withheld: authored on an internals claim"}
	if !e.Readiness.DependencyReady || !e.Readiness.ReviewPending ||
		!reflect.DeepEqual(e.Readiness.Causes, []record{wantCause}) ||
		!reflect.DeepEqual(e.Readiness.ReviewCauses, []record{wantCause}) {
		t.Fatalf("review cause must survive redaction: %+v", e.Readiness)
	}
}
