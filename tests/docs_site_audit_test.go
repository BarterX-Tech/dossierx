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
//
// v0.3.0 added four more, each of which shipped as prose that was simply false
// about the binary sitting next to it. They are grouped here rather than beside
// the code they describe because the failure is always the same shape — the
// text and the engine disagreeing — and because the fix is always to the text:
//
//   - README told the reader an agent "can resolve, reopen, edit or delete only
//     the threads and messages it authored itself", 66 lines after telling them
//     it cannot. v0.3.0 removed all four verbs from the CLI; an agent that
//     believes it can close its own thread opens one it cannot clear, and the
//     thread then blocks `claim lock` with no CLI verb to unblock it.
//   - The router skill enumerated seven `stopped_at` values. The binary emits
//     eight: `ledger` — the value that fires on exactly the failure this
//     release is built around — was missing, so an agent branching on it hits
//     its unknown-value arm at the worst possible moment.
//   - `.dossierx-flag-store.json` is a required tracked artifact that no doc
//     listed. A flagged claim whose flag store did not travel with it reaudits
//     to an empty proposal, and confirming that clears the human's flag having
//     applied nothing.
//   - FORMAT.md's findings table is the only public enumeration of the ledger
//     gate's rules, so a rule added in code and not added there is a refusal
//     nobody can look up.
//
// Two more were added after the v0.3.0 audit, both of the same shape and both
// bidirectional, because prose that promises a branch the binary cannot take is
// worse than prose that omits one:
//
//   - A skill's refusal table is what an agent branches on, having been told
//     never to regex `message`. `build_order_stale` was documented as the only
//     route to "re-propose, then re-lock" while the binary returned the generic
//     `build_order_refused`, so the documented recovery was unreachable.
//   - README's findings table is where a reader decides whether to trust the
//     gate. A rule in code that README never names is a gate nobody knows they
//     have; a rule README names that no code declares is one they think they do.
package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// repoRoot locates the repository root from this test source file (which lives
// at <root>/tests/) rather than from the process CWD, so everything here is
// robust to how `go test` is invoked.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed to locate the test source")
	}
	return filepath.Dir(filepath.Dir(thisFile)) // <root>/tests/<file> -> <root>
}

// readRepoFile reads a file addressed relative to the repository root.
func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	path := filepath.Join(repoRoot(t), rel)
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
// tells the truth about who runs the chromedp suite.
//
// It used to assert the opposite text — "maintainer-run", "not yet wired into
// CI" — and that assertion was correct for exactly as long as it was true: the
// suite really was executed by no CI job and no Makefile target, so a comment
// promising CI coverage would have been a lie. That hole is now closed (a
// `viewer` job in .github/workflows/ci.yml, a `make viewer-test` target), which
// makes the OLD wording the lie. Hence the inversion rather than the deletion:
// the property under test was never "the suite is unwired", it was "the comment
// matches reality", and this file is where that is enforced.
//
// The DOSSIERX_TEST_BROWSER clause is the part worth keeping either way. The
// harness t.Skip()s when it cannot resolve a browser, so a CI job that does not
// name one explicitly goes green having run nothing — the comment has to say so,
// or the next person to touch the job will helpfully remove the env var.
func TestViewerHarness_CIScopeCommentAccurate(t *testing.T) {
	norm := normalizeGoComment(readRepoFile(t, filepath.Join("viewer-tests", "harness_test.go")))

	if strings.Contains(norm, "not yet wired into CI") {
		t.Fatal(`viewer-tests/harness_test.go still says the chromedp suite is "not yet wired into CI"; ` +
			"the viewer job in .github/workflows/ci.yml runs it on every pull request")
	}
	if strings.Contains(norm, "maintainer-run") {
		t.Fatal(`viewer-tests/harness_test.go still describes the chromedp suite as "maintainer-run"; ` +
			"CI and `make viewer-test` both run it now")
	}
	if !strings.Contains(norm, "ci.yml") {
		t.Fatal("viewer-tests/harness_test.go comment should name the CI workflow that runs the suite, " +
			"so a reader can tell where its assertions actually execute")
	}
	if !strings.Contains(norm, "make viewer-test") {
		t.Fatal("viewer-tests/harness_test.go comment should name the `make viewer-test` target, " +
			"which is the only way to run this module locally — `go test ./...` does not reach a nested module")
	}
	if !strings.Contains(norm, "DOSSIERX_TEST_BROWSER") {
		t.Fatal("viewer-tests/harness_test.go comment should still document the DOSSIERX_TEST_BROWSER override")
	}
}

// TestREADME_ThreadClosingIsViewerOnly asserts README does not tell the reader
// an agent can resolve, reopen, edit or delete its own comment threads. v0.3.0
// removed all four verbs from the CLI (cmd/dossierx/comment.go registers
// inbox, list, add and reply, and nothing else), so the claim is false in a way
// that costs an agent real work: it opens a thread believing it can close the
// thread later, the open thread blocks `claim lock` with `unresolved_comments`,
// and there is no CLI verb that clears it.
//
// The binary is the source of truth in both directions. The registration list
// is read here so that a release which puts one of the four verbs BACK on the
// CLI fails this test too — at which point README's wording is the thing that
// needs revisiting, not this assertion.
func TestREADME_ThreadClosingIsViewerOnly(t *testing.T) {
	commentCmd := readRepoFile(t, filepath.Join("cmd", "dossierx", "comment.go"))

	for _, verb := range []string{"Resolve", "Reopen", "Edit", "Delete"} {
		ctor := "newComment" + verb + "Cmd()"
		if strings.Contains(commentCmd, ctor) {
			t.Fatalf("cmd/dossierx/comment.go now registers %s, so `comment %s` is a CLI verb again; README's review-loop section says the opposite and must be updated with this test", ctor, strings.ToLower(verb))
		}
	}

	readme := readRepoFile(t, "README.md")

	// The exact sentence that shipped, and any restatement of it. "it can
	// resolve" is the load-bearing fragment: the claim is not that the rights
	// rule exists (it does), it is that the agent may exercise it.
	for _, false_ := range []string{
		"so it can resolve",
		"it can resolve, reopen",
		"can resolve, reopen, edit or delete only the threads",
	} {
		if strings.Contains(readme, false_) {
			t.Fatalf("README still tells the reader an agent can close its own threads (%q); v0.3.0 has no `comment resolve`/`reopen`/`edit`/`delete` on the CLI", false_)
		}
	}

	// And it must say where those verbs actually live, or a reader is left to
	// infer it from an absence.
	said := false
	for _, line := range strings.Split(readme, "\n") {
		l := strings.ToLower(line)
		if strings.Contains(l, "resolve, reopen, edit and delete") && strings.Contains(l, "viewer") {
			said = true
			break
		}
	}
	if !said {
		t.Fatal("README does not state that resolve, reopen, edit and delete are reachable only from the viewer (and `dossierx serve`'s HTTP API); the CLI has none of them")
	}
}

// stoppedAtValuesInBinary returns every stopped_at value cmd/dossierx can emit,
// read out of main.go rather than restated here: checkStoppedAt's returns plus
// the steps the two read-only check paths set directly.
func stoppedAtValuesInBinary(t *testing.T) map[string]bool {
	t.Helper()
	main := readRepoFile(t, filepath.Join("cmd", "dossierx", "main.go"))

	got := map[string]bool{}

	// `StoppedAt: "config"` (struct literal) and `out.StoppedAt = "ledger"`.
	assign := regexp.MustCompile(`StoppedAt\s*[:=]\s*"([a-z_]+)"`)
	for _, m := range assign.FindAllStringSubmatch(main, -1) {
		got[m[1]] = true
	}

	// checkStoppedAt's own returns. Scoped to the function body so an unrelated
	// `return "lint"` elsewhere in the file cannot widen the set.
	const fn = "func checkStoppedAt("
	i := strings.Index(main, fn)
	if i < 0 {
		t.Fatal("cmd/dossierx/main.go no longer defines checkStoppedAt; this test must be repointed at whatever replaced it")
	}
	body := main[i:]
	if j := strings.Index(body, "\n}\n"); j >= 0 {
		body = body[:j]
	}
	ret := regexp.MustCompile(`return\s+"([a-z_]+)"`)
	for _, m := range ret.FindAllStringSubmatch(body, -1) {
		got[m[1]] = true
	}

	if len(got) < 4 {
		t.Fatalf("only %d stopped_at value(s) were found in cmd/dossierx/main.go (%v); the extraction is broken, not the docs", len(got), got)
	}
	return got
}

// TestSkillRouter_StoppedAtValuesMatchTheBinary pins the router skill's
// stopped_at enumeration to the value set the binary actually emits.
//
// The skill instructs an agent to branch on stopped_at, so a value the skill
// does not list is a value the agent has no arm for. The one that shipped
// missing was `ledger`, which fires precisely when a locked claim moved outside
// the approval path — and it is the value that carries the good news (the
// catalog and viewer WERE regenerated; only the commit is refused), so its
// absence cost the agent the one fact it exists to convey.
func TestSkillRouter_StoppedAtValuesMatchTheBinary(t *testing.T) {
	skill := readRepoFile(t, filepath.Join("skills", "dossierx", "SKILL.md"))

	const anchor = "`stopped_at` names the pipeline step a partial run reached"
	i := strings.Index(skill, anchor)
	if i < 0 {
		t.Fatalf("skills/dossierx/SKILL.md no longer documents stopped_at with the phrase %q", anchor)
	}
	rest := skill[i:]
	open := strings.Index(rest, "(")
	closeIdx := strings.Index(rest, ")")
	if open < 0 || closeIdx < open {
		t.Fatal("skills/dossierx/SKILL.md's stopped_at sentence no longer carries a parenthesised value list")
	}
	documented := map[string]bool{}
	for _, tok := range regexp.MustCompile("`([a-z_]+)`").FindAllStringSubmatch(rest[open:closeIdx], -1) {
		documented[tok[1]] = true
	}

	for value := range stoppedAtValuesInBinary(t) {
		if !documented[value] {
			t.Errorf("cmd/dossierx emits stopped_at %q but skills/dossierx/SKILL.md does not list it; an agent branching on stopped_at as the skill instructs falls into its unknown-value arm", value)
		}
	}
	for value := range documented {
		if !stoppedAtValuesInBinary(t)[value] {
			t.Errorf("skills/dossierx/SKILL.md lists stopped_at %q, which cmd/dossierx never emits", value)
		}
	}
}

// TestTrackedStoresAreDocumentedEverywhereTheyAreEnumerated asserts that every
// project-root store the engine writes appears in each place that enumerates
// "the files you must commit".
//
// The store names are derived from the engine rather than restated, so adding a
// fourth store fails this test until the four enumerations below have been
// updated. `.dossierx-flag-store.json` is the one that shipped missing from all
// four: it is required for `claim reaudit` to have anything to propose, and a
// review_pending claim that arrives without its flag entry reaudits to an empty
// proposal whose --confirm clears the human's flag having applied nothing.
func TestTrackedStoresAreDocumentedEverywhereTheyAreEnumerated(t *testing.T) {
	stores := map[string]bool{}
	storeLit := regexp.MustCompile(`"(\.dossierx-[a-z-]+\.json)"`)
	for _, rel := range []string{
		filepath.Join("internal", "check", "check.go"),
		filepath.Join("internal", "serve", "server.go"),
	} {
		for _, m := range storeLit.FindAllStringSubmatch(readRepoFile(t, rel), -1) {
			stores[m[1]] = true
		}
	}
	for _, want := range []string{".dossierx-lock-store.json", ".dossierx-flag-store.json"} {
		if !stores[want] {
			t.Fatalf("the engine no longer names %s where this test reads it; repoint the extraction before trusting the assertions below", want)
		}
	}

	// Every surface that tells someone which files to commit. Missing from any
	// one of them is enough: a reader following the CI template does not also
	// read the hook installer's closing message.
	for _, rel := range []string{
		"README.md",
		"FORMAT.md",
		filepath.Join("scripts", "ci", "dossierx-check.yml"),
		filepath.Join("scripts", "install-git-hook.sh"),
	} {
		doc := readRepoFile(t, rel)
		for store := range stores {
			if !strings.Contains(doc, store) {
				t.Errorf("%s never mentions %s, which the engine writes to the project root and which has to travel with the claims", rel, store)
			}
		}
	}
}

// TestFormatDocumentsEveryLedgerRule asserts FORMAT.md's findings table names
// every rule the ledger gate can emit.
//
// That table is the only public enumeration of the gate's vocabulary, and each
// rule string is what a caller reads out of `data.ledger_findings[].rule` — so a
// rule that exists in code and not in FORMAT.md is a refusal with no
// documentation to look it up in. The rules are read out of the packages that
// declare them.
//
// The assertion runs in BOTH directions, and the second direction is the one
// that was learned the hard way: a `comment-ledger-absent` row was written into
// this table describing a rule no package declared. That is worse than an
// undocumented rule, not better — an undocumented rule fires and confuses
// someone, whereas a documented-but-nonexistent one silently promises a gate
// that is not there, in the security section of a security release. Prose
// follows code in both directions: every rule must be documented, and every
// rule documented must exist.
func TestFormatDocumentsEveryLedgerRule(t *testing.T) {
	ruleConst := regexp.MustCompile(`Rule[A-Za-z]+\s*=\s*"([a-z-]+)"`)

	rules := map[string]string{} // rule -> file that declares it
	for _, rel := range []string{
		filepath.Join("internal", "lock", "audit.go"),
		filepath.Join("internal", "check", "ledger.go"),
	} {
		for _, m := range ruleConst.FindAllStringSubmatch(readRepoFile(t, rel), -1) {
			rules[m[1]] = rel
		}
	}
	if len(rules) < 5 {
		t.Fatalf("only %d ledger rule constant(s) found (%v); the extraction is broken, not FORMAT.md", len(rules), rules)
	}

	format := readRepoFile(t, "FORMAT.md")
	for rule, declaredIn := range rules {
		if !strings.Contains(format, "`"+rule+"`") {
			t.Errorf("%s declares the ledger rule %q but FORMAT.md's findings table does not name it; the rule string is what callers read out of data.ledger_findings[].rule", declaredIn, rule)
		}
	}

	// The reverse direction is scoped to the TABLE ROWS, not the whole file:
	// the prose around the table deliberately discusses one name that is not a
	// rule (there is no `comment-ledger-absent`, and FORMAT says so and why),
	// and a whole-file scan could not tell "this rule exists" from "this rule
	// pointedly does not".
	for _, rule := range formatFindingsTableRules(t, format) {
		if _, declared := rules[rule]; !declared {
			t.Errorf("FORMAT.md's findings table documents the ledger rule %q, but no Rule* constant in internal/lock/audit.go or internal/check/ledger.go declares it — the table promises a gate that does not exist", rule)
		}
	}
}

// formatFindingsTableRules returns the rule names in the first column of
// FORMAT.md's "### The findings" table, and nothing else in the document.
func formatFindingsTableRules(t *testing.T, format string) []string {
	t.Helper()

	const heading = "### The findings"
	i := strings.Index(format, heading)
	if i < 0 {
		t.Fatalf("FORMAT.md no longer has a %q section; repoint this test at whatever replaced it", heading)
	}
	body := format[i+len(heading):]
	if j := strings.Index(body, "\n### "); j >= 0 {
		body = body[:j]
	}

	row := regexp.MustCompile("^\\|\\s*`([a-z-]+)`\\s*\\|")
	var found []string
	for _, line := range strings.Split(body, "\n") {
		if m := row.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			found = append(found, m[1])
		}
	}
	if len(found) < 5 {
		t.Fatalf("only %d row(s) parsed out of FORMAT.md's findings table (%v); the row extraction is broken, not the table", len(found), found)
	}
	return found
}

// TestSkillErrorCodesAreDeclaredAndEmitted asserts that every `error.code` a
// skill's refusal table tells an agent to branch on both EXISTS as a
// cliout.Code and is actually emitted by the engine.
//
// The skills instruct an agent to branch on `code` and to "never regex message
// or hint", which makes a documented-but-unemitted code a dead branch with no
// fallback: the agent hits a refusal whose real code carries recoveries that do
// not apply, and its only remaining move is the one the router forbids. That
// shipped — `build_order_stale` was documented as the answer to a stale locked
// build order (and as the ONLY route to "re-propose, then re-lock"), while the
// binary returned the generic `build_order_refused`, whose three documented
// recoveries all assume something the stale case has already satisfied.
//
// Both directions are checked, for the same reason FORMAT.md's rule table is
// checked both ways: an undocumented code confuses a reader once, whereas a
// documented code that nothing emits silently promises a branch that can never
// be taken.
//
// "Emitted" is deliberately loose — any non-test reference to the constant from
// cmd/ or internal/ outside the cliout package itself. Pinning the exact call
// shape would make this test a second copy of the code it is auditing; what it
// is really asserting is that the constant is wired to something at all.
func TestSkillErrorCodesAreDeclaredAndEmitted(t *testing.T) {
	root := repoRoot(t)

	// The vocabulary, read out of the one file that declares it.
	declared := map[string]string{} // code string -> Go constant name
	codeConst := regexp.MustCompile(`(Code[A-Za-z]+)\s+Code\s*=\s*"([a-z_]+)"`)
	for _, m := range codeConst.FindAllStringSubmatch(readRepoFile(t, filepath.Join("internal", "cliout", "codes.go")), -1) {
		declared[m[2]] = m[1]
	}
	if len(declared) < 20 {
		t.Fatalf("only %d error code constant(s) found; the extraction is broken, not the skills", len(declared))
	}

	// Which constants anything outside internal/cliout actually reaches for.
	emitted := map[string]bool{}
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			// The declaring package names every constant by definition.
			if strings.Contains(filepath.ToSlash(path), "/internal/cliout/") {
				return nil
			}
			src, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			for code, name := range declared {
				if strings.Contains(string(src), "cliout."+name) {
					emitted[code] = true
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}

	for skill, codes := range skillTableErrorCodes(t, root) {
		for _, code := range codes {
			name, ok := declared[code]
			if !ok {
				t.Errorf("%s documents error.code %q, which internal/cliout/codes.go does not declare; an agent branching on it as the skill instructs takes a branch the binary can never reach", skill, code)
				continue
			}
			if !emitted[code] {
				t.Errorf("%s documents error.code %q (cliout.%s), but nothing under cmd/ or internal/ outside the cliout package ever returns it; the skill's recovery for it is unreachable and the agent gets some other code's recoveries instead", skill, code, name)
			}
		}
	}
}

// skillTableErrorCodes returns, per skill bundle, the codes named in the column
// of a markdown table whose header says "code".
//
// Scoped to that column on purpose: the skills discuss `review_pending` (a claim
// field), `usage` (a word) and command names in ordinary prose, and a whole-file
// scan could not tell "branch on this code" from "this is the field it sets".
func skillTableErrorCodes(t *testing.T, root string) map[string][]string {
	t.Helper()

	token := regexp.MustCompile("`([a-z][a-z0-9_]*)`")
	out := map[string][]string{}

	entries, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil {
		t.Fatalf("reading skills/: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		rel := filepath.Join("skills", e.Name(), "SKILL.md")
		doc := readRepoFile(t, rel)

		codeCol := -1
		for _, line := range strings.Split(doc, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "|") {
				codeCol = -1 // a table ended; the next one re-declares its columns
				continue
			}
			cells := splitTableRow(line)
			if strings.HasPrefix(strings.TrimSpace(cells[0]), "---") {
				continue // the header separator
			}
			// A header row is the one naming the column; every row after it in
			// the same table is data.
			if codeCol < 0 {
				for i, c := range cells {
					if strings.Contains(strings.ToLower(c), "code") {
						codeCol = i
					}
				}
				continue
			}
			if codeCol >= len(cells) {
				continue
			}
			for _, m := range token.FindAllStringSubmatch(cells[codeCol], -1) {
				out[rel] = append(out[rel], m[1])
			}
		}
	}

	if len(out) == 0 {
		t.Fatal("no skill documents an error.code table; the table extraction is broken, not the skills")
	}
	return out
}

// splitTableRow splits a markdown table row into its cells, dropping the empty
// strings the leading and trailing pipes produce.
func splitTableRow(line string) []string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// TestREADMENamesEveryLedgerRule is FORMAT.md's bidirectional pin, extended to
// the document most readers actually reach first.
//
// FORMAT.md is the reference; README is where someone deciding whether to trust
// the gate reads what it catches. A rule that exists in code but appears in
// neither is a refusal with no documentation at all, and the failure mode is the
// one this release is built to avoid — a reader concluding the gate covers
// something it does not, or (worse) that it does not cover something it does.
//
// Containment over the whole file rather than a table-row scan: `lock-ledger-absent`
// and `lock-ledger-unreadable` are deliberately explained in the prose above
// README's findings table rather than listed inside it.
func TestREADMENamesEveryLedgerRule(t *testing.T) {
	ruleConst := regexp.MustCompile(`Rule[A-Za-z]+\s*=\s*"([a-z-]+)"`)

	rules := map[string]string{} // rule -> file that declares it
	for _, rel := range []string{
		filepath.Join("internal", "lock", "audit.go"),
		filepath.Join("internal", "check", "ledger.go"),
	} {
		for _, m := range ruleConst.FindAllStringSubmatch(readRepoFile(t, rel), -1) {
			rules[m[1]] = rel
		}
	}
	if len(rules) < 5 {
		t.Fatalf("only %d ledger rule constant(s) found (%v); the extraction is broken, not README.md", len(rules), rules)
	}

	readme := readRepoFile(t, "README.md")
	for rule, declaredIn := range rules {
		if !strings.Contains(readme, "`"+rule+"`") {
			t.Errorf("%s declares the ledger rule %q but README.md never names it; the integrity section is the only place a reader learns what the gate catches", declaredIn, rule)
		}
	}
}
