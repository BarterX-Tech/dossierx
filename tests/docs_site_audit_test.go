// docs_site_audit_test.go is a documentation-consistency audit: it reads the
// shipped docs/site/harness text as source of truth files and asserts they do
// not misdescribe the actual binary behavior. These guard three confirmed
// audit defects from drifting back in:
//
//   - README's CLI table lumped `comment reply` into the rights-gated row,
//     falsely implying an agent cannot reply to a human-opened thread. Reply
//     is deliberately ungated (internal/comments/comments.go Reply never calls
//     canAct); only resolve/reopen/edit/delete are gated.
//   - The website's `reaudit` example depicted "reaudit: applied, ..." with no
//     claim id, but the binary prints "reaudit: <id> applied, ..."
//     (cmd/dossierx/main.go).
//   - viewer-tests/harness_test.go claimed CI runs the chromedp browser suite;
//     there is no CI job for it — it is maintainer-run via DOSSIERX_TEST_BROWSER.
package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// readRepoFile reads a file addressed relative to the repository root, located
// from this test source file (which lives at <root>/tests/) rather than the
// process CWD, so it is robust to how `go test` is invoked.
func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed to locate the test source")
	}
	root := filepath.Dir(filepath.Dir(thisFile)) // <root>/tests/<file> -> <root>
	path := filepath.Join(root, rel)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}

// normalizeGoComment strips "//" comment markers and collapses all runs of
// whitespace (including newlines) to single spaces, so a phrase check is
// insensitive to how a comment is line-wrapped.
func normalizeGoComment(s string) string {
	s = strings.ReplaceAll(s, "//", " ")
	return strings.Join(strings.Fields(s), " ")
}

// TestREADME_ReplyRightsNotOvergated asserts the README does not lump `comment
// reply` into the advisory-rights ("an agent may only act on its own messages")
// row. Reply is ungated — the core agent-replies-to-human workflow — while
// resolve/reopen/edit/delete are gated (internal/comments/comments.go canAct).
func TestREADME_ReplyRightsNotOvergated(t *testing.T) {
	readme := readRepoFile(t, "README.md")

	const advisory = "an agent may only act on its own messages"
	if !strings.Contains(readme, advisory) {
		t.Fatalf("README no longer documents the advisory-rights gate (%q); it should govern the comment resolve/reopen/edit/delete row", advisory)
	}

	// The line carrying the advisory-rights parenthetical must not mention
	// reply: reply is ungated, so lumping it in falsely implies an agent
	// cannot reply to a human-opened thread.
	for _, line := range strings.Split(readme, "\n") {
		if strings.Contains(line, advisory) && strings.Contains(strings.ToLower(line), "reply") {
			t.Fatalf("README lumps reply into the rights-gated row, implying an agent cannot reply to a human thread (false):\n%s", line)
		}
	}

	// Reply must be documented as ungated somewhere.
	ungatedReplyDocumented := false
	for _, line := range strings.Split(readme, "\n") {
		l := strings.ToLower(line)
		if strings.Contains(l, "reply") && strings.Contains(l, "ungated") {
			ungatedReplyDocumented = true
			break
		}
	}
	if !ungatedReplyDocumented {
		t.Fatal("README does not document reply as ungated (an agent may reply to a human-opened thread)")
	}
}

// TestSiteContent_ReauditExampleIncludesClaimID asserts the website's reaudit
// example depicts the success line exactly as the binary prints it:
// "reaudit: <id> applied, review_pending cleared" — carrying the claim id.
// Source of truth: cmd/dossierx/main.go's
//
//	fmt.Fprintf(out, "reaudit: %s applied, review_pending cleared\n", id)
func TestSiteContent_ReauditExampleIncludesClaimID(t *testing.T) {
	content := readRepoFile(t, filepath.Join("site", "src", "content.ts"))

	const bare = "reaudit: applied, review_pending cleared"
	if strings.Contains(content, bare) {
		t.Fatalf("site content depicts the reaudit success line without a claim id (%q); the binary prints \"reaudit: <id> applied, review_pending cleared\"", bare)
	}

	re := regexp.MustCompile(`reaudit: (\S+) applied, review_pending cleared`)
	m := re.FindStringSubmatch(content)
	if m == nil {
		t.Fatal(`site content no longer depicts a reaudit success line of the form "reaudit: <id> applied, review_pending cleared"`)
	}
	id := m[1]

	// The depicted output id must match the depicted command's id, so the
	// example is internally consistent with what the binary would print.
	//
	// The verb is "claim reaudit", not the bare "reaudit" this test pinned
	// before v0.3.0: the restructure moved lock, unlock, flag and reaudit
	// under the claim noun, so a site example showing "dossierx reaudit" is
	// now depicting a command that does not exist. Asserting the noun here is
	// what makes that a test failure rather than a plausible-looking snippet
	// a reader would paste and watch fail.
	wantCmd := "dossierx claim reaudit " + id + " --confirm"
	if !strings.Contains(content, wantCmd) {
		t.Fatalf("reaudit success line shows id %q but no matching %q command precedes it", id, wantCmd)
	}

	// The retired top-level form must not reappear anywhere in the copy. The
	// check above only proves the shaped example is right; this one catches a
	// stale invocation elsewhere on the page, which is the failure mode that
	// actually shipped in v0.2.x copy.
	if strings.Contains(content, "dossierx reaudit ") {
		t.Fatal(`site content still depicts the retired top-level "dossierx reaudit"; the verb moved under the claim noun in v0.3.0 ("dossierx claim reaudit")`)
	}
}

// TestViewerHarness_CIScopeCommentAccurate asserts the browser-harness comment
// does not claim CI runs the chromedp suite (there is no CI job for it), and
// instead states the suite is maintainer-run and not yet wired into CI.
func TestViewerHarness_CIScopeCommentAccurate(t *testing.T) {
	norm := normalizeGoComment(readRepoFile(t, filepath.Join("viewer-tests", "harness_test.go")))

	if strings.Contains(norm, "expected to export DOSSIERX_TEST_BROWSER so the suite actually runs") {
		t.Fatal("viewer-tests/harness_test.go still claims CI is expected to run the chromedp suite; there is no CI job for it")
	}
	if !strings.Contains(norm, "not yet wired into CI") {
		t.Fatal("viewer-tests/harness_test.go comment should state the chromedp suite is not yet wired into CI")
	}
	if !strings.Contains(norm, "maintainer-run") {
		t.Fatal("viewer-tests/harness_test.go comment should state the chromedp suite is maintainer-run")
	}
	if !strings.Contains(norm, "DOSSIERX_TEST_BROWSER") {
		t.Fatal("viewer-tests/harness_test.go comment should still document the DOSSIERX_TEST_BROWSER override")
	}
}
