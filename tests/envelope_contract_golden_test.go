// envelope_contract_golden_test.go snapshots the JSON envelope the CLI actually
// puts on stdout, per pinned invocation: the envelope's own keys, the top-level
// keys of `data` AND THE JSON TYPE OF EACH, the `error` block's keys and `code`,
// the `stopped_at` value, and the PROCESS EXIT STATUS.
//
// WHY, given cmd/dossierx/machine_contract_test.go already exists. That file
// pins three PROPERTIES of the envelope — snake_case keys, an envelope on every
// invocation, dry-run/write-path agreement — and every one of them survives a
// rename. `data.lint_findings` becoming `data.lint_results` is still snake_case,
// still an envelope, still agrees with its write path; SKILL.md's `error.code →
// what you actually do about it` table says "findings are in **`data.lint_findings`**"
// and would simply be false, with nothing red. So would dropping a `stopped_at`
// value an agent branches on. The key SET was the hole.
//
// WHY TYPES AND NOT JUST NAMES. Because a key set is blind to the half of the
// wire contract that keeps its keys. `data.blocked` going from the JSON boolean
// `false` to the JSON string "false" is a one-tag change in internal/cliout, it
// breaks every client that branches on it, and with names alone it moved nothing
// in this repository but a package source hash — which moves for any edit to
// that package and so reads as "you touched it", never as "the contract moved".
// So every recorded key carries a bounded sketch of its value's shape, and a
// scalar that becomes a string, an object or a list is a diff on the line that
// names it.
//
// WHAT A TYPE SKETCH IS NOT. It is not a schema and it is not asserting values.
// Recording that `lint_findings` is `[{claim_id:string,...}]` says what a client
// can parse; it says nothing about which findings a fixture happens to produce.
// Where an array is empty in every pinned state, "[]" is what is recorded — the
// honest answer for that invocation, not a claim about the type.
//
// WHY IT EXECS THE BINARY rather than living in cmd/dossierx. The exit STATUS is
// half of what is being pinned and it is not observable in-process — see
// cmd/dossierx/cli_inprocess_test.go's own note on that — and surface.json's
// `envelope.exit_codes` is the STATIC half (code → status, straight out of
// cliout.ExitCode). What is asserted here is that a real process, taking a real
// path through the code, actually exits with it — CONFRONTED with that table,
// not merely snapshotted beside it. A snapshot alone cannot catch the two halves
// drifting apart: a path that exits without consulting cliout.ExitCode moves the
// number here and the table there stays put, and both files would be
// individually consistent while the documented `error.code → exit status`
// contract had stopped being true of the binary.
//
// WHAT IT DOES NOT REACH, SAID OUT LOUD. The pinned invocations produce eleven
// of the forty-four codes cliout can emit; the rest need states no fixture here
// builds. That is a real limit, so it is WRITTEN INTO the golden rather than
// left to be inferred from which blocks happen to exist — CLAUDE.md's rule is
// that coverage is never narrowed in silence. Recording it also makes both
// directions diffs: a code that loses its pinned invocation leaves the pinned
// list, and a code that arrives with none shows up in the unpinned one.
//
// WHAT THIS CAN AND CANNOT SAY ABOUT COMMAND → PAYLOAD.
// surface.json deliberately omits the command-path-to-payload association
// because it is not statically derivable, and warns that observing it by
// execution "reports the keys of whichever branch the fixture happened to take,
// not the contract, because most payload fields are `omitempty`". That warning
// is correct and it applies to this file too — so the association here is closed
// at the granularity of an INVOCATION, never of a command:
//
//   - every block below is labelled with the state its fixture was in, and the
//     keys recorded are the keys THAT invocation emitted. That is a contract: an
//     agent that runs `claim lock` on a claim with an open human thread gets
//     exactly these keys, this code and this exit status, and a change to any of
//     them is a change to what the skill told it to expect.
//   - the union of a command's blocks is a LOWER BOUND on its payload's key set,
//     never the set itself. surface.json's `envelope.payloads` is the upper
//     bound — the full marshalled key set of each payload TYPE. Neither
//     document claims a per-command mapping, and this one does not synthesise
//     one out of two partial views.
//
// Commands that take more than one shape are given one block per shape (lock
// succeeds and lock refused; check clean and check at lint) precisely so the
// omitempty subset each of them reports is attributable to a named state.
//
// The committed golden is regenerated the same way every other golden in this
// repository is:
//
//	go test ./tests -run TestEnvelopeContractGolden -regenerate-goldens
package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// envelopeGoldenFile is the committed snapshot, relative to the repository
// root. It sits beside the corpora under testdata/ rather than next to this
// file so that surfaces.yaml's `testdata/` out-of-scope entry already claims it
// — it is a fixture, not a client-facing surface.
const envelopeGoldenFile = "testdata/envelope-contract.golden.txt"

// envelopeGoldenHeader is prose carried INSIDE the committed file, because the
// file is read by the release gate and by whoever is writing a CHANGELOG entry,
// neither of whom is reading this source.
const envelopeGoldenHeader = `# envelope-contract.golden.txt — what each pinned invocation of the dossierx CLI
# actually emitted: the envelope's keys, the keys of data WITH THE JSON TYPE OF
# EACH, the error block and its code, stopped_at, and the process exit status.
#
# HOW TO READ A TYPE. "blocked:bool" is a JSON boolean; "[string]" a list of
# strings; "{a:string,b:number}" an object; "[]" an array that was empty in this
# state. Nesting is described four levels deep, which reaches the fields of a
# comment reply — the deepest object the skills tell an agent to read by name.
# A key whose type changes here is a breaking change for every client that
# branches on it, even though its name never moved.
#
# AN EMPTY LIST PINS NOTHING. "[]" is honest about the invocation and silent
# about the type, so any payload whose contract lives inside a list is given a
# second block from a fixture state that fills it — an inbox with a thread in
# it, a thread with a reply on it. Where a block still shows "[]", the objects
# that list would carry are held by nothing here.
#
# Generated by tests/envelope_contract_golden_test.go. Regenerate with:
#
#     go test ./tests -run TestEnvelopeContractGolden -regenerate-goldens
#
# READ IT AS A CONTRACT PER INVOCATION, NOT PER COMMAND. Most payload fields are
# omitempty, so a block records the keys THAT run produced from THAT fixture
# state — which is why every block names the state. The union of a command's
# blocks is a lower bound on its payload's key set; surface.json's
# envelope.payloads holds the upper bound (the full key set of each payload
# type). No per-command mapping is stated anywhere, here or there, because none
# is derivable without guessing.
#
# THE LAST SECTION SAYS WHAT IS NOT PINNED. Only some of the codes cliout can
# emit are reachable from the fixture states above; the rest are listed as
# unpinned rather than left out, so the reach of this file is a fact you can read
# instead of one you have to infer.
#
# A diff in this file is a change to the machine contract skills/dossierx/SKILL.md
# documents. It is not automatically wrong — it is automatically something a
# client's agent has to be told about.
`

// envelopeCase is one pinned invocation: a fixture state, and the argv run
// against it.
type envelopeCase struct {
	// name is "<command path> / <the state the fixture was in>". It is the
	// block heading, and the reason a reader can tell an omitempty subset from
	// the payload type.
	name string
	// setup prepares the project directory and returns any substitutions the
	// argv needs — ids the engine minted, which cannot be written down here and
	// must not reach the golden. Nil means no substitutions.
	setup func(t *testing.T, dir string) map[string]string
	// args is the argv after the binary. Placeholder tokens ("{thread}") are
	// replaced by setup's substitutions before the run and left INTACT in the
	// golden, so a freshly minted id never makes the file differ from itself.
	args []string
}

// ---------------------------------------------------------------------
// fixture states
// ---------------------------------------------------------------------

// envNoProject leaves the directory empty: the state every agent meets first.
func envNoProject(_ *testing.T, _ string) map[string]string { return nil }

// envFresh is one draft claim, lint-clean apart from the warning-severity
// "orphan" every edgeless fixture trips.
func envFresh(t *testing.T, dir string) map[string]string {
	t.Helper()
	writeFixtureProject(t, dir, "widget")
	return nil
}

// envLocked is envFresh with the claim locked, which is the precondition for
// flag, link and reaudit.
func envLocked(t *testing.T, dir string) map[string]string {
	t.Helper()
	writeFixtureProject(t, dir, "widget")
	envMustRun(t, dir, "claim", "lock", "widget.contract.overview", "--reason", "fixture approval")
	return nil
}

// envAgentThread is envFresh with one open thread the AGENT opened, so
// "comment list" and "comment reply" have a real thread id to name.
func envAgentThread(t *testing.T, dir string) map[string]string {
	t.Helper()
	writeFixtureProject(t, dir, "widget")
	out := envMustRun(t, dir, "comment", "add", "widget.contract.overview", "--as", "agent", "--body", "does this still hold?")
	var env envelopeDoc
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("comment add did not emit one envelope: %v\n%s", err, out)
	}
	id, ok := env.Data["thread_id"].(string)
	if !ok || id == "" {
		t.Fatalf("comment add emitted no thread_id: %s", out)
	}
	return map[string]string{"{thread}": id}
}

// envOpenHumanThread is the state SKILL.md's `unresolved_comments` row is
// about: a thread the human opened, which the lock gate waits on and an agent
// may not resolve.
func envOpenHumanThread(t *testing.T, dir string) map[string]string {
	t.Helper()
	writeFixtureProject(t, dir, "widget")
	envMustRun(t, dir, "comment", "add", "widget.contract.overview", "--as", "human", "--body", "is this still true?")
	return nil
}

// envHumanThreadReplied is envOpenHumanThread after the agent has answered: the
// state the whole review loop is made of, and the ONLY state in which the
// comment payloads are fully populated.
//
// It exists because an empty list pins nothing. `comment inbox` over a project
// with no threads records `threads:[]`, which is a true fact about that
// invocation and says nothing whatever about `inboxThread` — the object
// skills/dossierx-comments/SKILL.md prints field by field as the contract, down
// to calling `agent_can_resolve` "the rights rule, as data". Renaming any of
// those fourteen fields moved nothing anywhere in this tree but a package
// source hash, which moves for any edit to cmd/dossierx. Same for a thread with
// no replies: `replies:[]` leaves commentReplyView unpinned one level further
// in. A human question and an agent answer is the smallest fixture that puts
// both objects on the wire at once.
func envHumanThreadReplied(t *testing.T, dir string) map[string]string {
	t.Helper()
	writeFixtureProject(t, dir, "widget")
	out := envMustRun(t, dir, "comment", "add", "widget.contract.overview", "--as", "human", "--body", "is three retries right?")
	var env envelopeDoc
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("comment add did not emit one envelope: %v\n%s", err, out)
	}
	id, ok := env.Data["thread_id"].(string)
	if !ok || id == "" {
		t.Fatalf("comment add emitted no thread_id: %s", out)
	}
	envMustRun(t, dir, "comment", "reply", "widget.contract.overview", id, "--as", "agent", "--body", "checked against the lock, it holds")
	return map[string]string{"{thread}": id}
}

// envInboxPolled is envHumanThreadReplied after one inbox call, carrying that
// call's cursor as "{since}".
//
// It is here for one key. `commentInboxData.Since` is omitempty, so it is on
// the wire only when the caller passed --since — and --since is not an optional
// nicety, it is the polling loop the comments skill instructs every agent to
// run ("echo it back verbatim as the next call's --since"). Pinning the inbox
// from a first poll alone would leave the key that makes the loop work recorded
// nowhere.
//
// The cursor is read from the engine rather than composed here, because a
// hand-built timestamp would be testing this file's idea of the format instead
// of the value the agent is actually told to echo.
func envInboxPolled(t *testing.T, dir string) map[string]string {
	t.Helper()
	subs := envHumanThreadReplied(t, dir)
	out := envMustRun(t, dir, "comment", "inbox")
	var env envelopeDoc
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("comment inbox did not emit one envelope: %v\n%s", err, out)
	}
	cursor, ok := env.Data["cursor"].(string)
	if !ok || cursor == "" {
		t.Fatalf("comment inbox emitted no cursor to poll with: %s", out)
	}
	subs["{since}"] = cursor
	return subs
}

// envDangling is a corpus with an error-severity lint finding — a rests_on edge
// pointing at an id no claim carries — modelled on
// testdata/fixture-coverage/lint/dangling.
func envDangling(t *testing.T, dir string) map[string]string {
	t.Helper()
	claimsDir := filepath.Join(dir, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}
	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"
	if err := os.WriteFile(filepath.Join(dir, "project.config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write project.config.yaml: %v", err)
	}
	claim := "id: widget.contract.overview\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  a claim resting on an id nothing declares.\n" +
		"governed_by:\n  type: none\n  reason: fixture claim, not backed by any real doctrine\n" +
		"rests_on:\n  - widget.contract.ghost\n"
	if err := os.WriteFile(filepath.Join(claimsDir, "overview.yaml"), []byte(claim), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}
	return nil
}

// envRoleLocked is a locked claim carrying a build_role, which is what
// build-order propose needs: the shared writeFixtureProject claim has none, and
// proposing over it is refused with build_order_refused rather than producing an
// order.
func envRoleLocked(t *testing.T, dir string) map[string]string {
	t.Helper()
	claimsDir := filepath.Join(dir, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}
	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"
	if err := os.WriteFile(filepath.Join(dir, "project.config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write project.config.yaml: %v", err)
	}
	claim := "id: widget.contract.overview\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"build_role: schema\nbody: |\n  fixture claim carrying a build role.\n" +
		"governed_by:\n  type: none\n  reason: fixture claim, not backed by any real doctrine\n"
	if err := os.WriteFile(filepath.Join(claimsDir, "overview.yaml"), []byte(claim), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}
	envMustRun(t, dir, "claim", "lock", "widget.contract.overview", "--reason", "fixture approval")
	return nil
}

// envProposedOrder is envRoleLocked with the build order proposed but not yet
// locked.
func envProposedOrder(t *testing.T, dir string) map[string]string {
	t.Helper()
	envRoleLocked(t, dir)
	envMustRun(t, dir, "build-order", "propose", "--module", "widget")
	return nil
}

// envMustRun runs a setup command in the JSON format and fails the test if it
// did not succeed. Setup failures must never present as a golden diff: a block
// recording the envelope of a command whose PRECONDITION quietly stopped
// holding would look like a contract change and be a broken fixture.
func envMustRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	stdout, stderr, code := run(t, dir, append([]string{"--format", "json"}, args...)...)
	if code != 0 {
		t.Fatalf("fixture setup %v exited %d\nstdout: %s\nstderr: %s", args, code, stdout, stderr)
	}
	return stdout
}

// ---------------------------------------------------------------------
// the pinned invocations
// ---------------------------------------------------------------------

// envelopeCases is every invocation the golden records, in the order it records
// them. The order is declared rather than sorted so that a command's success
// block and its refusal block sit next to each other and read as the pair they
// are.
//
// TestEnvelopeContractGoldenCoversEveryLeaf asserts this list reaches every leaf
// but "serve", so a twentieth leaf cannot arrive with no envelope pinned.
func envelopeCases() []envelopeCase {
	return []envelopeCase{
		{"version / the verb", envNoProject, []string{"version"}},
		{"version / the root flag", envNoProject, []string{"--version"}},

		{"check / read-only, lint-clean", envFresh, []string{"check", "--validate"}},
		{"check / writing the catalog and the viewer", envFresh, []string{"check"}},
		// Both doors into a lint failure, because they are two code paths and
		// only one of them is the one an agent's pre-commit run takes: the
		// read-only door sets stopped_at inline, the writing door computes it
		// from how far the pipeline got. Pinning one of them leaves the other
		// free to report a step it never reached.
		{"check / refused at lint by a dangling edge, read-only", envDangling, []string{"check", "--validate"}},
		{"check / refused at lint by a dangling edge, writing", envDangling, []string{"check"}},
		{"check / no project anywhere up the tree", envNoProject, []string{"check", "--validate"}},

		{"claim show / a draft claim", envFresh, []string{"claim", "show", "widget.contract.overview"}},
		{"claim show / an id no claim carries", envFresh, []string{"claim", "show", "widget.contract.ghost"}},
		{"claim list / every claim", envFresh, []string{"claim", "list"}},
		{"claim new / a fresh draft", envFresh, []string{"claim", "new", "widget.contract.second", "--body", "another fact", "--governed-reason", "fixture"}},

		{"claim lock / approved", envFresh, []string{"claim", "lock", "widget.contract.overview", "--reason", "approved"}},
		{"claim lock / refused by an open human thread", envOpenHumanThread, []string{"claim", "lock", "widget.contract.overview", "--reason", "approved"}},
		{"claim lock / previewed", envFresh, []string{"claim", "lock", "widget.contract.overview", "--reason", "approved", "--dry-run"}},
		{"claim lock / already locked", envLocked, []string{"claim", "lock", "widget.contract.overview", "--reason", "again"}},

		{"claim unlock / a locked claim", envLocked, []string{"claim", "unlock", "widget.contract.overview", "--reason", "fixing it"}},
		{"claim flag / a locked claim", envLocked, []string{"claim", "flag", "widget.contract.overview", "--claim-says", "a", "--now-does", "b", "--reason", "c"}},
		{"claim flag / a draft claim", envFresh, []string{"claim", "flag", "widget.contract.overview", "--claim-says", "a", "--now-does", "b", "--reason", "c"}},
		{"claim reaudit / a claim that is not review_pending", envLocked, []string{"claim", "reaudit", "widget.contract.overview"}},
		{"claim link / previewed against a draft claim", envFresh, []string{"claim", "link", "--module", "widget", "--claim", "widget.contract.overview", "--file", "claims/overview.yaml", "--dry-run"}},
		{"claim link / a locked claim", envLocked, []string{"claim", "link", "--module", "widget", "--claim", "widget.contract.overview", "--file", "claims/overview.yaml"}},

		// Three inbox blocks, because the empty one pins only that it is
		// empty. The populated block is what holds `inboxThread`'s fourteen
		// documented fields, and the polled block is what holds `since` —
		// omitempty, and absent from every first call.
		{"comment inbox / nothing since the beginning", envFresh, []string{"comment", "inbox"}},
		{"comment inbox / a human thread the agent has answered", envHumanThreadReplied, []string{"comment", "inbox"}},
		{"comment inbox / polling again with the cursor it returned", envInboxPolled, []string{"comment", "inbox", "--since", "{since}"}},
		{"comment list / one open thread", envAgentThread, []string{"comment", "list", "widget.contract.overview"}},
		// The reply-bearing shape, for the same reason: a thread whose
		// `replies` is empty says nothing about what a reply looks like.
		{"comment list / a thread carrying a reply", envHumanThreadReplied, []string{"comment", "list", "widget.contract.overview"}},
		{"comment add / a new thread", envFresh, []string{"comment", "add", "widget.contract.overview", "--as", "agent", "--body", "a note"}},
		{"comment add / an actor that is neither role", envFresh, []string{"comment", "add", "widget.contract.overview", "--as", "robot", "--body", "a note"}},
		{"comment reply / on the agent's own thread", envAgentThread, []string{"comment", "reply", "widget.contract.overview", "{thread}", "--as", "agent", "--body", "checked, it holds"}},

		{"build-order propose / one locked claim", envRoleLocked, []string{"build-order", "propose", "--module", "widget"}},
		{"build-order status / nothing proposed yet", envLocked, []string{"build-order", "status", "--module", "widget"}},
		{"build-order lock / a fresh proposal", envProposedOrder, []string{"build-order", "lock", "--module", "widget", "--reason", "approved"}},
		{"build-order lock / nothing proposed yet", envLocked, []string{"build-order", "lock", "--module", "widget", "--reason", "approved"}},

		{"skills export / into an explicit directory", envFresh, []string{"skills", "export", "skills-out"}},

		{"usage / a noun with no leaf", envFresh, []string{"claim"}},
		{"usage / a format nothing renders", envFresh, []string{"--format", "yaml", "version"}},
	}
}

// ---------------------------------------------------------------------
// running them
// ---------------------------------------------------------------------

// envelopeDoc is the envelope, decoded loosely on purpose: this file asserts
// the KEY SET, so decoding into cliout.Envelope — which would silently discard
// a key that is no longer part of it and invent the ones that are — is the one
// shape it cannot use. (package tests cannot import cmd/dossierx at all, and
// deliberately does not import internal/cliout either: the point is to read
// what a client reads.)
type envelopeDoc struct {
	Raw       map[string]json.RawMessage
	Data      map[string]any
	DataKeys  []string
	ErrKeys   []string
	ErrDetail []string
	Code      string
	Command   string
	OK        bool
	StoppedAt string
}

// decodeEnvelope reads stdout as exactly one JSON object and pulls out
// everything a block records. Anything other than one object is fatal: "one
// envelope per invocation, on stdout" is the promise underneath every other
// promise here.
func decodeEnvelope(t *testing.T, stdout string) envelopeDoc {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(stdout))
	var doc envelopeDoc
	if err := dec.Decode(&doc.Raw); err != nil {
		t.Fatalf("stdout is not one JSON envelope (%v):\n%s", err, stdout)
	}
	if dec.More() {
		t.Fatalf("stdout carries more than one JSON document:\n%s", stdout)
	}

	if raw, ok := doc.Raw["ok"]; ok {
		_ = json.Unmarshal(raw, &doc.OK) //nolint:errcheck // a non-bool ok shows up as false and as a key-set diff
	}
	if raw, ok := doc.Raw["command"]; ok {
		_ = json.Unmarshal(raw, &doc.Command) //nolint:errcheck // ditto
	}
	if raw, ok := doc.Raw["stopped_at"]; ok {
		_ = json.Unmarshal(raw, &doc.StoppedAt) //nolint:errcheck // ditto
	}
	if raw, ok := doc.Raw["data"]; ok {
		var val any
		switch err := json.Unmarshal(raw, &val); {
		case err != nil:
			doc.DataKeys = []string{"(unreadable)"}
		default:
			if obj, isObj := val.(map[string]any); isObj {
				doc.Data = obj
				doc.DataKeys = typedKeysOf(obj, sketchDepth)
			} else {
				// A data that is not an object is itself contract-visible, so
				// it is recorded rather than dropped.
				doc.DataKeys = []string{"(not an object):" + typeSketch(val, sketchDepth)}
			}
		}
	}
	if raw, ok := doc.Raw["error"]; ok {
		var errObj map[string]any
		if err := json.Unmarshal(raw, &errObj); err == nil {
			// Depth 0: `details` shows as "object" here and is expanded on its
			// own line below, rather than printed twice.
			doc.ErrKeys = typedKeysOf(errObj, 0)
			// A code that is not a string leaves Code empty and shows up as a
			// key-set change rather than being read as an absent error.
			if code, ok := errObj["code"].(string); ok {
				doc.Code = code
			}
			if details, ok := errObj["details"].(map[string]any); ok {
				doc.ErrDetail = typedKeysOf(details, sketchDepth-1)
			}
		}
	}
	return doc
}

func sortedKeysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// sketchDepth is how many levels of a `data` value this file describes. It is
// set by the DEEPEST thing the wire contract names by name, which is a comment
// reply: `data.threads` is an array (1) of thread objects (2) whose `replies`
// is an array (3) of reply objects (4). Stop at three and `replies` records as
// "[object]" — commentReplyView's five fields renamed without a diff, one level
// below the hole this file was written to close.
//
// Four is not a guess at "deep enough". Regenerating at four with the earlier
// three-deep corpus produced a byte-identical file: nothing else in the pinned
// set is nested that far, so the extra level costs nothing and buys exactly the
// reply. The reason not to keep going is unchanged — error.details is sketched
// at sketchDepth-1 and is a free-form bag, and past this the golden would start
// recording the shape of debugging detail as if it were contract.
const sketchDepth = 4

// typeSketch renders the SHAPE of a decoded JSON value — its type, and down to
// `depth` further levels the types of what it holds.
//
// This is what makes the golden a WIRE contract rather than a list of names. A
// key-name snapshot is blind to the whole class of change that keeps the name:
// `data.blocked` going from the JSON boolean `false` to the JSON string
// "false" is one `json:"blocked,string"` tag away, breaks every client that
// branches on it, and moved nothing anywhere in this repository except a
// package source hash in surface.json — which moves for any edit to
// internal/cliout and so reads as "you touched the package", never as "the wire
// contract moved". Measured, not assumed. The same hole covered a scalar
// becoming an object and a scalar becoming a list.
//
// Arrays render as the MERGED sketch of their elements, so the record does not
// depend on which element happened to be first and a heterogeneous array says so
// ("[number|string]") instead of hiding half of itself. An empty array is "[]":
// a true fact about that invocation, which becomes "[{...}]" the day the fixture
// stops being empty — a diff a reader can interpret rather than one that hides.
func typeSketch(v any, depth int) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		if depth <= 0 {
			return "array"
		}
		var elems []string
		seen := map[string]bool{}
		for _, e := range t {
			s := typeSketch(e, depth-1)
			if !seen[s] {
				seen[s] = true
				elems = append(elems, s)
			}
		}
		sort.Strings(elems)
		return "[" + strings.Join(elems, "|") + "]"
	case map[string]any:
		if depth <= 0 {
			return "object"
		}
		return "{" + strings.Join(typedKeysOf(t, depth-1), ",") + "}"
	}
	// json.Unmarshal into `any` produces nothing else; a fifth kind would mean
	// the decoder changed under this file, which is worth seeing in the diff.
	return "unknown"
}

// typedKeysOf renders a JSON object's members as sorted "key:sketch" pairs.
func typedKeysOf(m map[string]any, depth int) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+":"+typeSketch(v, depth))
	}
	sort.Strings(out)
	return out
}

// envelopeBlock renders one case's observed envelope. Absent parts are OMITTED
// rather than written as empty, so losing `stopped_at` shows up as a deleted
// line in the diff instead of a value that changed to "".
func envelopeBlock(c envelopeCase, argv []string, exitCode int, doc envelopeDoc) string {
	var b strings.Builder
	b.WriteString(c.name + "\n")

	const labelWidth = 14
	field := func(name, value string) {
		b.WriteString("    " + name + strings.Repeat(" ", max(1, labelWidth-len(name))) + value + "\n")
	}
	list := func(name string, values []string) {
		if len(values) == 0 {
			return
		}
		field(name, strings.Join(values, " "))
	}
	// column writes one entry per line, which is how a typed key set stays
	// readable: a sketch runs to a couple of hundred characters, and on one line
	// a single field's type change would be reported as "this whole line
	// changed" with the reader left to find where.
	column := func(name string, values []string) {
		for i, v := range values {
			if i == 0 {
				field(name, v)
				continue
			}
			b.WriteString(strings.Repeat(" ", 4+labelWidth) + v + "\n")
		}
	}

	field("argv", strings.Join(argv, " "))
	field("exit", itoa(exitCode))
	// `ok`, `command` and `stopped_at` are pinned by VALUE, which pins their
	// types for free: each is decoded into a typed Go variable above, and a
	// value that is no longer a bool or a string fails to decode and leaves the
	// zero value, which is a golden diff.
	field("ok", map[bool]string{true: "true", false: "false"}[doc.OK])
	list("envelope", sortedKeysOf(doc.Raw))
	field("command", doc.Command)
	column("data", doc.DataKeys)
	column("error", doc.ErrKeys)
	if doc.Code != "" {
		field("error.code", doc.Code)
	}
	column("error.details", doc.ErrDetail)
	if doc.StoppedAt != "" {
		field("stopped_at", doc.StoppedAt)
	}
	return b.String()
}

// ---------------------------------------------------------------------
// the static half, and the reach of the empirical one
// ---------------------------------------------------------------------

// surfaceExitCodes reads `envelope.exit_codes` out of surface.json: every code
// cliout can emit, against the status cliout.ExitCode gives it. That is the
// static half of the exit contract, extracted from the source by
// cmd/dossierx/surface_test.go; what this file observes is the empirical half.
//
// Unreadable is FATAL, never tolerated. A cross-check that quietly passed
// because surface.json moved would leave the two halves free to disagree, which
// is the one thing pairing them is for.
func surfaceExitCodes(t *testing.T) map[string]int {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "surface.json"))
	if err != nil {
		t.Fatalf("read surface.json for the static exit-code table: %v", err)
	}
	var doc struct {
		Envelope struct {
			ExitCodes map[string]int `json:"exit_codes"`
		} `json:"envelope"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode surface.json: %v", err)
	}
	if len(doc.Envelope.ExitCodes) == 0 {
		t.Fatal("surface.json carries no envelope.exit_codes, so the static half of the exit contract cannot be read; a cross-check over an empty table is a pass over zero assertions")
	}
	return doc.Envelope.ExitCodes
}

// codeCoverageSection renders which codes the pinned invocations actually
// produced, and which ones none of them did.
//
// observed maps a code to the status the process exited with; static is
// surface.json's table. Both lists are written out in full: the pinned one so a
// code that stops being reachable is a deletion in the diff, and the unpinned
// one so a code that ARRIVES with no pinned invocation is an insertion. Neither
// is summarised down to a count, because a count is exactly the shape that lets
// one code quietly replace another.
func codeCoverageSection(observed, static map[string]int) string {
	var b strings.Builder
	const title = "ERROR-CODE COVERAGE"
	b.WriteString("\n" + title + "\n" + strings.Repeat("-", len(title)) + "\n")

	var pinned, unpinned []string
	for _, code := range sortedKeysOf(static) {
		if _, ok := observed[code]; ok {
			pinned = append(pinned, code)
		} else {
			unpinned = append(unpinned, code)
		}
	}

	b.WriteString("    pinned by an invocation above (" + itoa(len(pinned)) + " of " + itoa(len(static)) + ")\n")
	for _, code := range pinned {
		b.WriteString("        " + code + strings.Repeat(" ", max(1, 26-len(code))) + "exit " + itoa(observed[code]) + "\n")
	}
	b.WriteString("    reachable only from states no fixture here builds (" + itoa(len(unpinned)) + ")\n")
	b.WriteString("        their exit status is held by surface.json's static table alone; nothing\n" +
		"        below has been observed coming out of a real process.\n")
	for _, code := range unpinned {
		b.WriteString("        " + code + "\n")
	}
	return b.String()
}

// TestEnvelopeContractGolden runs every pinned invocation and compares the
// rendered snapshot against the committed one, byte for byte.
//
// It also confronts each observed exit status with surface.json's static table
// as it goes, rather than in a second test: the observation is one real process
// launch per pinned invocation, and both assertions are questions about the
// same one.
func TestEnvelopeContractGolden(t *testing.T) {
	cases := envelopeCases()
	if len(cases) == 0 {
		t.Fatal("no pinned invocations; this suite would pass over zero assertions")
	}

	static := surfaceExitCodes(t)
	observed := map[string]int{}

	var b strings.Builder
	b.WriteString(envelopeGoldenHeader)

	for _, c := range cases {
		dir := t.TempDir()
		subs := map[string]string{}
		if c.setup != nil {
			for k, v := range c.setup(t, dir) {
				subs[k] = v
			}
		}

		// The argv as WRITTEN goes in the golden; the argv as SUBSTITUTED is
		// what runs. A minted thread id in the file would make every
		// regeneration a diff.
		live := make([]string, len(c.args))
		for i, a := range c.args {
			live[i] = a
			if sub, ok := subs[a]; ok {
				live[i] = sub
			}
		}

		stdout, stderr, code := run(t, dir, append([]string{"--format", "json"}, live...)...)
		if strings.TrimSpace(stdout) == "" {
			t.Fatalf("%s: no envelope on stdout (exit %d)\nstderr: %s", c.name, code, stderr)
		}
		doc := decodeEnvelope(t, stdout)

		// The static table says what status this code is supposed to carry;
		// this process just told us what it actually carried. A disagreement is
		// an exit path that stopped going through cliout.ExitCode, and it is
		// reported on its own terms — regenerating the golden would record the
		// wrong number as if it were the contract.
		if doc.Code != "" {
			want, known := static[doc.Code]
			switch {
			case !known:
				t.Errorf("%s emitted error.code %q, which surface.json's envelope.exit_codes does not carry; either the code is new and surface.json is stale, or it is not a cliout.Code at all", c.name, doc.Code)
			case want != code:
				// cliout.Error.Exit lets a call site override the code's
				// default status, and surface.json's table — read out of
				// cliout.ExitCode alone — cannot express that. So a
				// disagreement is either a path that stopped consulting
				// ExitCode, or a deliberate override the static table has no
				// way to say. Both need a human: the first is a broken
				// contract, the second is a table that has stopped being the
				// whole truth.
				t.Errorf("%s emitted error.code %q and exited %d, but surface.json's envelope.exit_codes says %q exits %d.\n"+
					"The empirical and static halves of the exit contract disagree. If this is a deliberate\n"+
					"cliout.Error.Exit override, surface.json's table can no longer be read as `code -> status`\n"+
					"and the emitter has to say so; if it is not, an exit path has stopped going through\n"+
					"cliout.ExitCode.", c.name, doc.Code, code, doc.Code, want)
			default:
				observed[doc.Code] = code
			}
		}

		b.WriteString("\n")
		b.WriteString(envelopeBlock(c, c.args, code, doc))
	}
	b.WriteString(codeCoverageSection(observed, static))

	goldenPath := filepath.Join("..", filepath.FromSlash(envelopeGoldenFile))
	got := b.String()

	if *regenerateGoldens {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write %s: %v", envelopeGoldenFile, err)
		}
		t.Logf("regenerated %s (%d invocations)", envelopeGoldenFile, len(cases))
		return
	}

	wantBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read %s: %v\n\nRegenerate it with:\n  go test ./tests -run TestEnvelopeContractGolden -regenerate-goldens", envelopeGoldenFile, err)
	}
	want := string(wantBytes)
	if want == got {
		return
	}
	line, wantLine, gotLine := firstDifferingLine(want, got)
	t.Fatalf(`%s is out of date: the CLI's machine contract is not what the committed snapshot says.

first difference at line %d
  committed: %s
  observed:  %s

This is a change to the contract skills/dossierx/SKILL.md documents to every
client's agent. Regenerate the snapshot, read the diff, and make sure the skill
and the CHANGELOG say what it says:

  go test ./tests -run TestEnvelopeContractGolden -regenerate-goldens`,
		envelopeGoldenFile, line, truncateForDiff(wantLine), truncateForDiff(gotLine))
}

// TestEnvelopeContractGoldenCoversEveryLeaf keeps the pinned list honest against
// the surface it claims to snapshot. A snapshot of eighteen leaves out of
// nineteen is not a smaller snapshot, it is a leaf whose contract nothing holds
// — and the leaf that arrives NEXT is exactly the one nobody remembers to add.
//
// The leaf list is read out of skills/dossierx/SKILL.md's noun block, which is
// the document this file's snapshot exists to keep true, rather than restated
// here where it would be one more list that can fall behind.
func TestEnvelopeContractGoldenCoversEveryLeaf(t *testing.T) {
	pinned := map[string]bool{}
	for _, c := range envelopeCases() {
		// The block heading is "<command path> / <state>"; the command path is
		// what identifies the leaf.
		path := strings.TrimSpace(strings.SplitN(c.name, "/", 2)[0])
		pinned[path] = true
	}

	for _, leaf := range skillLeaves(t) {
		if leaf == "serve" {
			// serve blocks forever and is the one permanent text-only command;
			// cmd/dossierx/envelope_cli_test.go pins that exemption itself.
			continue
		}
		if !pinned[leaf] {
			t.Errorf("leaf %q has no pinned envelope in envelopeCases(); its data keys, exit status and error codes are held by nothing", leaf)
		}
	}
}

// skillLeaves reads the leaf set out of the fenced noun block in
// skills/dossierx/SKILL.md — the "seven nouns, nineteen leaves" listing. Lines
// look like "dossierx claim  show list new lock unlock flag reaudit link", so a
// leaf is "<noun> <verb>", or the bare noun where the noun IS the command
// (check, serve, version).
func skillLeaves(t *testing.T) []string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "skills", "dossierx", "SKILL.md"))
	if err != nil {
		t.Fatalf("read the router skill: %v", err)
	}
	text := string(src)
	start := strings.Index(text, "```\ndossierx check")
	if start < 0 {
		t.Fatal("the router skill no longer opens its noun block with `dossierx check`; this reader is broken, not the skill")
	}
	block := text[start+len("```\n"):]
	if end := strings.Index(block, "```"); end >= 0 {
		block = block[:end]
	}

	var leaves []string
	for _, line := range strings.Split(block, "\n") {
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "dossierx" {
			continue
		}
		noun := fields[1]
		verbs := fields[2:]
		// "dossierx skills export [dir]" carries an argument placeholder, not a
		// second verb.
		filtered := verbs[:0]
		for _, v := range verbs {
			if strings.HasPrefix(v, "[") || strings.HasPrefix(v, "<") {
				continue
			}
			filtered = append(filtered, v)
		}
		if len(filtered) == 0 {
			leaves = append(leaves, noun)
			continue
		}
		for _, v := range filtered {
			leaves = append(leaves, noun+" "+v)
		}
	}
	if len(leaves) < 19 {
		t.Fatalf("found only %d leaves in the router skill's noun block (%v); the reader is broken, not the skill", len(leaves), leaves)
	}
	return leaves
}
