// comment_races_test.go covers the comment-specific cross-process races that
// Phase 0 deferred until "dossierx comment add" existed. Each spawns two real
// binary invocations concurrently on the SAME claim; the project-wide claims
// sentinel must serialize their whole-file load->mutate->SaveClaim so the
// outcome is always ONE of the two serialized orders — never torn state, never
// a lost comment, never a lock-gate bypass. (-race cannot see a cross-PROCESS
// race, so these assert on the resulting file/CLI state, per the plan.)
package tests

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
)

// countThreads returns the number of comment threads "dossierx comment list
// --json" reports for id — which also proves the claim file still parses (a
// torn write would fail the load or the JSON decode).
func countThreads(t *testing.T, root, cfgPath, id string) int {
	t.Helper()
	out, stderr, code := run(t, root, "--config", cfgPath, "comment", "list", id, "--json")
	if code != 0 {
		t.Fatalf("comment list --json for %s exited %d (stderr: %s)", id, code, stderr)
	}
	var threads []map[string]any
	if err := json.Unmarshal([]byte(out), &threads); err != nil {
		t.Fatalf("comment list --json for %s is not valid JSON: %v\nout: %s", id, err, out)
	}
	return len(threads)
}

// runPair runs two binary invocations concurrently in root and waits for both.
// run() only t.Fatal's if the binary can't be exec'd at all (never on a
// non-zero exit), so calling it from goroutines is safe here.
func runPair(t *testing.T, root string, argsA, argsB []string) {
	t.Helper()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); run(t, root, argsA...) }()
	go func() { defer wg.Done(); run(t, root, argsB...) }()
	wg.Wait()
}

const raceIterations = 6

// comment add vs lock on the same claim: whichever order wins, the added
// comment is never lost, and the lock gate is never bypassed — a claim that
// ends up locked WITH an open thread must be review_pending (that can only be
// the lock-then-add order), never locked-with-an-open-thread-and-no-flag.
func TestCommentRace_AddVsLock(t *testing.T) {
	for i := 0; i < raceIterations; i++ {
		root := t.TempDir()
		cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
		claimPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.main", facet: "contract", module: "widget", status: "draft", body: "raced claim."})

		runPair(t, root,
			[]string{"--config", cfgPath, "comment", "add", "widget.contract.main", "--as", "human", "--body", "race body"},
			[]string{"--config", cfgPath, "lock", "widget.contract.main"},
		)

		if n := countThreads(t, root, cfgPath, "widget.contract.main"); n != 1 {
			t.Fatalf("iter %d: expected the added comment to survive (exactly 1 thread), got %d", i, n)
		}
		final := llReadFile(t, claimPath)
		if strings.Contains(final, "status: locked") && !strings.Contains(final, "review_pending: true") {
			t.Fatalf("iter %d: lock-gate bypass — claim locked with an open thread but NOT review_pending:\n%s", i, final)
		}
	}
}

// comment add vs unlock: both orders end draft with the comment intact (unlock
// preserves Comments; add on a draft never sets review_pending).
func TestCommentRace_AddVsUnlock(t *testing.T) {
	for i := 0; i < raceIterations; i++ {
		root := t.TempDir()
		cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
		claimPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.main", facet: "contract", module: "widget", status: "draft", body: "raced claim."})
		if _, stderr, code := run(t, root, "--config", cfgPath, "lock", "widget.contract.main"); code != 0 {
			t.Fatalf("iter %d: lock setup: %s", i, stderr)
		}

		runPair(t, root,
			[]string{"--config", cfgPath, "comment", "add", "widget.contract.main", "--as", "human", "--body", "race body"},
			[]string{"--config", cfgPath, "unlock", "widget.contract.main"},
		)

		if n := countThreads(t, root, cfgPath, "widget.contract.main"); n != 1 {
			t.Fatalf("iter %d: expected the comment to survive add-vs-unlock (exactly 1 thread), got %d", i, n)
		}
		if final := llReadFile(t, claimPath); !strings.Contains(final, "status: draft") {
			t.Fatalf("iter %d: expected the claim draft after add-vs-unlock (both orders end draft), got:\n%s", i, final)
		}
	}
}

// comment add vs flag: both are review_pending triggers on a locked claim; both
// orders end locked + review_pending with the comment intact and the flag
// recorded.
func TestCommentRace_AddVsFlag(t *testing.T) {
	for i := 0; i < raceIterations; i++ {
		root := t.TempDir()
		cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
		claimPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.main", facet: "contract", module: "widget", status: "draft", body: "raced claim."})
		if _, stderr, code := run(t, root, "--config", cfgPath, "lock", "widget.contract.main"); code != 0 {
			t.Fatalf("iter %d: lock setup: %s", i, stderr)
		}

		runPair(t, root,
			[]string{"--config", cfgPath, "comment", "add", "widget.contract.main", "--as", "human", "--body", "race body"},
			[]string{"--config", cfgPath, "flag", "widget.contract.main", "--claim-says", "x", "--now-does", "y", "--reason", "raced"},
		)

		if n := countThreads(t, root, cfgPath, "widget.contract.main"); n != 1 {
			t.Fatalf("iter %d: expected the comment to survive add-vs-flag (exactly 1 thread), got %d", i, n)
		}
		final := llReadFile(t, claimPath)
		if !strings.Contains(final, "status: locked") || !strings.Contains(final, "review_pending: true") {
			t.Fatalf("iter %d: expected locked + review_pending after add-vs-flag, got:\n%s", i, final)
		}
	}
}

// comment add vs a review_pending-flipping check: check reconciles main to
// review_pending (its dependency drifted) while a comment is added; the
// comments block must survive whichever order wins.
func TestCommentRace_AddVsCheck(t *testing.T) {
	for i := 0; i < raceIterations; i++ {
		root := t.TempDir()
		cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
		depPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.dep", facet: "contract", module: "widget", status: "draft", body: "dep, v1."})
		mainPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.main", facet: "contract", module: "widget", status: "draft", body: "main on dep.", restsOn: []string{"widget.contract.dep"}})
		for _, id := range []string{"widget.contract.dep", "widget.contract.main"} {
			if _, stderr, code := run(t, root, "--config", cfgPath, "lock", id); code != 0 {
				t.Fatalf("iter %d: lock %s: %s", i, id, stderr)
			}
		}
		// Drift the dependency so a concurrent check WANTS to flip main.
		drifted := strings.Replace(llReadFile(t, depPath), "dep, v1.", "dep, v2.", 1)
		if err := os.WriteFile(depPath, []byte(drifted), 0o644); err != nil {
			t.Fatalf("iter %d: rewrite dep: %v", i, err)
		}

		runPair(t, root,
			[]string{"--config", cfgPath, "comment", "add", "widget.contract.main", "--as", "human", "--body", "race body"},
			[]string{"--config", cfgPath, "check"},
		)

		if n := countThreads(t, root, cfgPath, "widget.contract.main"); n != 1 {
			t.Fatalf("iter %d: expected the comment to survive add-vs-check (exactly 1 thread), got %d", i, n)
		}
		if final := llReadFile(t, mainPath); !strings.Contains(final, "status: locked") {
			t.Fatalf("iter %d: expected main still locked after add-vs-check, got:\n%s", i, final)
		}
	}
}
