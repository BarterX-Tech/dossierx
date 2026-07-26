// cliout_test.go pins the wire shape of the machine contract. These are not
// "does the struct have fields" tests: every assertion here is something a
// skill in the field would break on if it changed — the key names, whether an
// empty list serializes as [] or null, whether an error envelope can also carry
// the partial data the run produced, and which exit status each code maps to.
package cliout

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// encode is the shape under test: what a command actually prints.
func encode(t *testing.T, env Envelope) map[string]any {
	t.Helper()
	var buf strings.Builder
	if err := Write(&buf, env); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Fatalf("envelope must end with a newline so a line-oriented reader sees a complete record; got %q", buf.String())
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(buf.String()), &got); err != nil {
		t.Fatalf("envelope is not valid JSON (%v): %s", err, buf.String())
	}
	return got
}

func TestSuccessEnvelopeShape(t *testing.T) {
	got := encode(t, Success("claim lock", map[string]string{"claim_id": "widget.contract.one"}, nil))

	if got["ok"] != true {
		t.Fatalf("ok must be true on a success envelope, got %v", got["ok"])
	}
	if got["command"] != "claim lock" {
		t.Fatalf("command must be the binary-less command path, got %v", got["command"])
	}
	// error and stopped_at are omitempty: their ABSENCE is what a consumer
	// branches on, so their presence with a null/empty value would be a
	// contract break, not a cosmetic one.
	if _, present := got["error"]; present {
		t.Fatalf("a success envelope must not carry an error key: %v", got)
	}
	if _, present := got["stopped_at"]; present {
		t.Fatalf("a non-pipeline success envelope must not carry stopped_at: %v", got)
	}
}

func TestFailureEnvelopeCarriesPartialDataAndStoppedAt(t *testing.T) {
	env := Failure("check", &Error{Code: CodeLintFailed, Message: "check: lint: 1 error-level finding(s)", Hint: "run: dossierx check"})
	env.Data = map[string]any{"lint_error_count": 1}
	env.StoppedAt = "lint"

	got := encode(t, env)
	if got["ok"] != false {
		t.Fatalf("ok must be false on a failure envelope, got %v", got["ok"])
	}
	if got["stopped_at"] != "lint" {
		t.Fatalf("stopped_at must survive onto the failure envelope, got %v", got["stopped_at"])
	}
	// The whole point of allowing data on a failure: a fail-fast run that got
	// three steps in has produced something real, and discarding it would force
	// a second full run to find out what happened.
	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("a failed run's partial data must survive onto the envelope, got %v", got["data"])
	}
	if data["lint_error_count"] != float64(1) {
		t.Fatalf("partial data drifted: %v", data)
	}
	errObj, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("failure envelope must carry an error object, got %v", got["error"])
	}
	if errObj["code"] != string(CodeLintFailed) {
		t.Fatalf("error.code is the ONLY stable branch target; got %v", errObj["code"])
	}
	if _, present := errObj["details"]; present {
		t.Fatalf("details is omitempty and must be absent when unset: %v", errObj)
	}
	// Exit is invocation state, not document state, and must never reach the wire.
	if _, present := errObj["Exit"]; present {
		t.Fatalf("Error.Exit must not be serialized: %v", errObj)
	}
}

// TestExitCodesStayInTheDocumentedThreeFamilies is the additive-only guard. The
// README documents exactly three statuses and two integration tests pin their
// meanings; v0.3.0 introduces no fourth, because error.code now carries the
// detail a fourth status would have.
func TestExitCodesStayInTheDocumentedThreeFamilies(t *testing.T) {
	all := []Code{
		CodeBadRequest, CodeInvalidActor, CodeReadOnly, CodeClaimNotFound, CodeThreadNotFound,
		CodeReplyNotFound, CodeBannerClaim, CodeEmptyBody, CodeUnsafeBody, CodeClaimNotSerializable,
		CodeRightsDenied, CodeThreadResolved, CodeThreadOpen, CodeClaimFileChanged, CodeInternal,
		CodeConfigNotFound, CodeInvalidConfig, CodeInvalidClaim, CodeLintFailed, CodeNotLocked,
		CodeAlreadyLocked, CodeReviewPending, CodeNotReviewPending, CodeWrongState,
		CodeUnresolvedComments, CodeDependencyNotLocked, CodeStructuredLayout, CodeNotProposed,
		CodeBuildOrderStale, CodeBuildOrderRefused, CodeNoArtifact, CodeImplinkRefused,
		CodeUnknownModule, CodeMissingFlag, CodeUnsupportedFormat, CodeUsage, CodeWriteConflict,
		CodeWriteFailed,
	}
	for _, c := range all {
		if got := ExitCode(c); got != 1 && got != 2 {
			t.Fatalf("code %q maps to exit %d; only the documented 1 and 2 are allowed", c, got)
		}
	}
}

// TestExitCodeFamilyMembership pins the specific mappings the shipped tests and
// README depend on. A code moving between families is a breaking change for
// every script that branched on the status.
func TestExitCodeFamilyMembership(t *testing.T) {
	notFoundOrWrongState := []Code{
		CodeConfigNotFound, CodeClaimNotFound, CodeThreadNotFound, CodeReplyNotFound,
		CodeNotLocked, CodeNotReviewPending, CodeReviewPending, CodeWrongState,
	}
	for _, c := range notFoundOrWrongState {
		if ExitCode(c) != 2 {
			t.Fatalf("%q is a not-found / wrong-state code and must exit 2, got %d", c, ExitCode(c))
		}
	}
	// The one tests/check_exit_test.go asserts loudest: a check failure must
	// never be mistaken for a missing claim or config.
	generic := []Code{CodeLintFailed, CodeWriteFailed, CodeImplinkRefused, CodeRightsDenied, CodeInternal}
	for _, c := range generic {
		if ExitCode(c) != 1 {
			t.Fatalf("%q is a generic-failure code and must exit 1, got %d", c, ExitCode(c))
		}
	}
}

func TestErrorExitStatusPrefersExplicitOverride(t *testing.T) {
	e := &Error{Code: CodeClaimNotFound}
	if e.ExitStatus() != 2 {
		t.Fatalf("without an override the code's family decides; got %d", e.ExitStatus())
	}
	e.Exit = 1
	if e.ExitStatus() != 1 {
		t.Fatalf("an explicit Exit must win so a historical call site can keep its status; got %d", e.ExitStatus())
	}
}

// TestCodedErrorPreservesMessageAndSentinel is the property the whole text-mode
// byte-parity story rests on: attaching a code must change neither the string
// the terminal prints nor the sentinel errors.Is can see.
func TestCodedErrorPreservesMessageAndSentinel(t *testing.T) {
	sentinel := errors.New("claim not found")

	plain := fmt.Errorf("lock: claim %q not found: %w", "widget.contract.ghost", sentinel)
	coded := Errorf(CodeClaimNotFound, "lock: claim %q not found: %w", "widget.contract.ghost", sentinel)

	if coded.Error() != plain.Error() {
		t.Fatalf("coded message drifted from fmt.Errorf:\n got: %q\nwant: %q", coded.Error(), plain.Error())
	}
	if !errors.Is(coded, sentinel) {
		t.Fatal("errors.Is must still see the wrapped sentinel through a CodedError")
	}
	if got := As(coded); got == nil || got.Code != CodeClaimNotFound {
		t.Fatalf("As must recover the attached code, got %v", got)
	}
	// And a chain that has no code at all must report none, so callers can tell
	// "explicitly classified" from "fall back to sentinel inference".
	if got := As(plain); got != nil {
		t.Fatalf("As on an uncoded error must return nil, got %v", got)
	}
}

func TestWrapKeepsCauseMessage(t *testing.T) {
	cause := fmt.Errorf("disk on fire")
	coded := Wrap(cause, CodeWriteFailed).WithHint("run: dossierx check")
	if coded.Error() != "disk on fire" {
		t.Fatalf("Wrap must not reword the cause, got %q", coded.Error())
	}
	if coded.E.Hint != "run: dossierx check" {
		t.Fatalf("hint not attached: %+v", coded.E)
	}
	if Wrap(nil, CodeWriteFailed) != nil {
		t.Fatal("Wrap(nil) must be nil so it composes in an if err != nil ladder")
	}
}

// TestDryRunListsAreNeverNull: a consumer should be able to range over
// preconditions, missing, and side_effects unconditionally.
func TestDryRunListsAreNeverNull(t *testing.T) {
	var buf strings.Builder
	if err := json.NewEncoder(&buf).Encode(NewDryRun("lock claim x")); err != nil {
		t.Fatalf("encode: %v", err)
	}
	for _, key := range []string{`"preconditions":[]`, `"side_effects":[]`, `"missing":[]`} {
		if !strings.Contains(strings.ReplaceAll(buf.String(), " ", ""), key) {
			t.Fatalf("empty dry run must emit %s, got %s", key, buf.String())
		}
	}
}

// TestDryRunRequireBlocksAndRecordsMissing pins the invariant that a failed
// precondition cannot be reported without also blocking the run — the two are
// kept in sync inside Require precisely so a call site cannot forget.
func TestDryRunRequireBlocksAndRecordsMissing(t *testing.T) {
	dr := NewDryRun("lock claim x").
		Require("lint_clean", true, "0 findings").
		Require("no_open_comment_threads", false, "1 unresolved thread")

	if !dr.Blocked {
		t.Fatal("a failed precondition must block the dry run")
	}
	if len(dr.Missing) != 1 || dr.Missing[0] != "no_open_comment_threads" {
		t.Fatalf("failed precondition must land in missing, got %v", dr.Missing)
	}
	// Passing preconditions are kept: they are the evidence a human is being
	// asked to approve on.
	if len(dr.Preconditions) != 2 || !dr.Preconditions[0].OK {
		t.Fatalf("passing preconditions must be reported too, got %+v", dr.Preconditions)
	}

	dr.Lacking("--reason")
	if len(dr.Missing) != 2 || dr.Missing[1] != "--reason" {
		t.Fatalf("Lacking must append to missing, got %v", dr.Missing)
	}
}
