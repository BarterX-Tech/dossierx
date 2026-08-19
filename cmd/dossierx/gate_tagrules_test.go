// gate_tagrules_test.go is the forge-side half of D1: the check that the forge
// itself restricts who may create this release's tag.
//
// THE GAP IT EXISTS FOR, which no other check in this repository can reach.
// Every guarantee the release gate makes is enforced by files INSIDE the
// repository being gated — the workflow GitHub runs for a tag is the one in the
// tagged tree. So a writer with push rights does not have to get past the gate;
// they can weaken it, in a copy of these very files, tag that commit, and the
// forge will run the weakened rules over it. docs/RELEASING.md carried that as
// a recorded residual — "a check cannot be its own enforcement... only a
// forge-side tag protection rule can" close it — and a recorded residual is a
// state, not a check: nothing failed when the rule was absent. This file makes
// the rule's ABSENCE a refusal. It does not, and cannot, make the rule exist.
//
// WHAT IT ASKS, AND OF WHOM. GitHub's mechanism for restricting ref creation
// is repository rulesets. (The feature that answered this question by name —
// "tag protection rules", GET /repos/{owner}/{repo}/tags/protection — was
// sunset by GitHub on 30 August 2024 and its endpoints removed; a check written
// against it would refuse every release forever, for a reason that has nothing
// to do with the release.) So the question is asked of the rulesets API, in two
// reads: GET /repos/{o}/{r}/rulesets lists every ruleset with its target and
// enforcement — org-owned rulesets that apply to the repository included, since
// includes_parents defaults to true — and GET /repos/{o}/{r}/rulesets/{id}
// yields one ruleset's ref_name conditions and its rules. A release is cleared
// only by a ruleset that is ALL of: targeted at tags, actively enforced (the
// "evaluate" mode is a dry run that restricts nobody), covering the exact ref
// about to be created by an include pattern and not un-covered by an exclude,
// and carrying BOTH a `creation` and an `update` rule. Update is required
// beside creation because it is the same gap through a different door: a
// force-moved existing tag is an update, and a tag push — force or not — is
// what fires the Release workflow. A `deletion` rule is deliberately NOT
// required: a deleted tag runs nothing and recreating it is a creation, so
// deletion is vandalism rather than this gap — docs/RELEASING.md still tells a
// maintainer to restrict it, and says there why the check does not insist.
//
// WHAT THIS FILE CANNOT PROMISE, stated here the way gate/method.yaml states
// what it cannot promise about the tool grant, because every one of these is a
// boundary a reader will otherwise assume closed:
//
//   - THE ANSWER CROSSES A NETWORK, so it can be lied to. A captive resolver, a
//     proxy, or a compromised forge can answer "restricted" about a repository
//     that is not. Nothing here authenticates the transport beyond what `gh`
//     does, and nothing here pretends the answer is more than what the wire
//     carried.
//   - IT IS A CHECK, NOT A LOCK. The rule can be deleted in the window between
//     this answer and D6's tag push, and nothing in the driver can see that
//     happen. Asking as late as D1's structure allows (it is the precondition's
//     final question) narrows the window; only the forge refusing the tag at
//     push time — which is the rule itself, not this check — closes it.
//   - WHAT IS VISIBLE DEPENDS ON THE TOKEN. A token without read access to the
//     rulesets is turned away — and GitHub answers 404, not only 403, for a
//     private repository a token cannot see — so an unreadable answer is a
//     REFUSAL that names the missing scope (classic: `repo`, `public_repo`
//     while the repository is public; fine-grained: `Metadata: read`), never a
//     pass. There is no reading of "we could not look" that clears a release.
//   - A RULE THAT EXISTS MAY STILL BE HOLLOW. Its bypass list names roles,
//     teams and apps this check cannot resolve to people; a ruleset whose
//     bypass includes everyone with write access restricts nobody and passes
//     this check. Reading the bypass list is a maintainer's work, and
//     docs/RELEASING.md says so where the rule is configured.
//   - THE PATTERN LANGUAGE IS GITHUB'S, NOT OURS. Ruleset ref conditions are
//     fnmatch-style patterns; this check interprets only the shapes it can
//     stand behind (`~ALL`, and `refs/tags/…` literals with `*`/`**`) and
//     REFUSES a pattern it cannot interpret rather than guessing — a guess
//     that is wrong in the permissive direction clears an unprotected release.
//
// WHY IT IS TEST CODE: gate_driver_test.go's "WHY IT IS TEST CODE" note is the
// whole argument and is not repeated here. Like every gate file, nothing in it
// ships in the binary and nothing in it runs on an unauthorized invocation —
// the driver asks this question through gateDriverEvidence, whose production
// implementation calls gateTagRulesVerify, and whose test fixtures inject the
// forge's answer so that no test in this package touches a network or needs a
// token.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// errGateTagsUnprotected is the ANSWER "no": the forge was reached, its
// rulesets were read and interpreted, and none of them restricts who may
// create this release's tag. It is a separate sentinel from errGateUncheckable
// for the reason errGateArchivesWrong is: the two accuse different parties
// with different recoveries. Unprotected accuses the FORGE'S CONFIGURATION,
// and the recovery is a maintainer creating the ruleset docs/RELEASING.md
// specifies — an act that cannot be performed from inside this repository.
// Uncheckable accuses the READING — no `gh`, no token, a scope the forge
// turned away, a pattern this check cannot interpret — and the recovery is to
// supply what was missing and ask again.
var errGateTagsUnprotected = errors.New("the forge does not restrict who can create this release's tag")

// gateTagRulesProcedureItem is the docs/RELEASING.md item that tells a
// maintainer what to configure. Every refusal that accuses the configuration
// names it, because "the rule is absent" without where the rule is specified
// is a refusal the operator has to go and research at the worst moment —
// the same rule gateDriverCIEvidenceTarget's refusals follow.
const gateTagRulesProcedureItem = "The forge restricts who can create release tags"

// gateTagRulesRefPrefix is the fully-qualified form this check compares in.
// Rulesets store tag conditions fully qualified (`refs/tags/v*`), and this
// check compares ONLY that form: an unqualified pattern is refused as
// uninterpretable rather than matched against a guess about GitHub's
// normalization (see gateTagRulePatternCovers).
const gateTagRulesRefPrefix = "refs/tags/"

// ---------------------------------------------------------------------
// the shapes the forge answers in
// ---------------------------------------------------------------------

// gateTagRuleset is one ruleset as the detail endpoint returns it. Only the
// fields the decision reads are declared: a field this check does not consult
// would be a field a reader believes is consulted.
type gateTagRuleset struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Target      string `json:"target"`
	Enforcement string `json:"enforcement"`
	Conditions  struct {
		RefName struct {
			Include []string `json:"include"`
			Exclude []string `json:"exclude"`
		} `json:"ref_name"`
	} `json:"conditions"`
	Rules []struct {
		Type string `json:"type"`
	} `json:"rules"`
}

// ---------------------------------------------------------------------
// the pattern language, interpreted conservatively
// ---------------------------------------------------------------------

// gateTagRulePatternCovers reports whether one ruleset ref-name pattern covers
// ref (a fully-qualified `refs/tags/<tag>`).
//
// THE RULE IS: INTERPRET WHAT CAN BE STOOD BEHIND, REFUSE THE REST. GitHub
// documents ruleset ref conditions as fnmatch-style patterns and does not
// document every edge of that dialect; a matcher that guessed at the edges
// and guessed permissively would clear a release whose tag the rule does not
// actually cover. So:
//
//   - `~ALL` covers everything, by the API's own definition.
//   - A pattern not beginning with `refs/tags/` is REFUSED, not matched: the
//     UI stores tag patterns fully qualified, and whether GitHub would
//     qualify a bare `v*` is its normalization to know, not this check's to
//     assume in either direction.
//   - `?`, character classes and escapes are REFUSED: they are fnmatch
//     features whose exact dialect this check has not measured.
//   - `*` matches within one path segment and `**` across segments. For every
//     ref this check is ever asked about the two dialects agree — the driver
//     refuses a version containing `/` before matching anything (see
//     gateTagRulesDecide), and over a slash-free tag `[^/]*` and `.*` accept
//     the same strings — so the conservative reading costs nothing and can
//     only under-match, and an under-match refuses a release rather than
//     clearing one.
func gateTagRulePatternCovers(pattern, ref string) (bool, error) {
	if pattern == "~ALL" {
		return true, nil
	}
	if !strings.HasPrefix(pattern, gateTagRulesRefPrefix) {
		return false, fmt.Errorf("the pattern %q does not begin with %q, and this check will not guess how the forge qualifies it — a guess wrong in the permissive direction clears an unprotected release. Spell the rule fully qualified (for example %q)",
			pattern, gateTagRulesRefPrefix, gateTagRulesRefPrefix+"v*")
	}
	if i := strings.IndexAny(pattern, `?[]\{}!`); i >= 0 {
		return false, fmt.Errorf("the pattern %q uses %q, an fnmatch feature this check does not interpret. It refuses rather than guessing; use only literals, `*` and `**` in the rule, or extend gateTagRulePatternCovers with a measured reading of that syntax",
			pattern, string(pattern[i]))
	}
	var re strings.Builder
	re.WriteString("^")
	for i := 0; i < len(pattern); {
		switch {
		case strings.HasPrefix(pattern[i:], "**"):
			re.WriteString(".*")
			i += 2
		case pattern[i] == '*':
			re.WriteString("[^/]*")
			i++
		default:
			re.WriteString(regexp.QuoteMeta(string(pattern[i])))
			i++
		}
	}
	re.WriteString("$")
	matcher, err := regexp.Compile(re.String())
	if err != nil {
		// Unreachable through the translation above, which quotes every
		// literal byte; kept as a refusal rather than a panic because a
		// matcher that cannot be built is a pattern this check cannot
		// interpret, which already has a meaning here.
		return false, fmt.Errorf("the pattern %q could not be compiled for matching: %v", pattern, err)
	}
	return matcher.MatchString(ref), nil
}

// ---------------------------------------------------------------------
// the decision, a pure function of what the forge said
// ---------------------------------------------------------------------

// gateTagRulesDecide is the whole verdict, over answers already fetched.
//
// It is a pure function on purpose — the same separation gateDriverMode draws:
// everything network-shaped lives in gateTagRulesForge, and every refusal this
// function can make is constructible in a test from a struct literal, with no
// forge, no token and no network. slug and total are here only so the
// refusals can say what was looked at: a refusal that names nothing sends the
// operator to re-derive the search this function already did.
//
// THE ORDER OF THE ANSWERS. A qualifying ruleset clears the release even when
// some OTHER ruleset carries a pattern this check cannot interpret — the
// protection is established, and the uninterpretable rule is somebody else's
// business. Only when NO ruleset qualifies does interpretation failure become
// the verdict, and then it is errGateUncheckable (the reading failed) rather
// than errGateTagsUnprotected (the reading succeeded and the answer is no),
// because the two demand different recoveries and accusing the configuration
// on a pattern nobody could read accuses the wrong party.
func gateTagRulesDecide(version, slug string, total int, rulesets []gateTagRuleset) error {
	// The one shape of version this matcher's conservatism is not free over: a
	// tag containing `/` is where `*`-within-a-segment and `*`-across-segments
	// part ways, including on the EXCLUDE side, where under-matching is the
	// dangerous direction. No release this repository cuts carries one — D1's
	// own version derivation admits only vX.Y.Z — but this function is the
	// last reader before the comparison, so it is where the assumption is
	// enforced rather than relied on.
	if strings.ContainsAny(version, "/*?[]\\{}!~") || strings.TrimSpace(version) == "" || version != strings.TrimSpace(version) {
		return fmt.Errorf("%w: the release was named %q, which this check cannot form a tag ref from and match conservatively — a `/` or a glob character in the tag is where fnmatch dialects diverge, on the exclude side too, where a wrong guess clears an unprotected release",
			errGateUncheckable, version)
	}
	ref := gateTagRulesRefPrefix + version

	var (
		examined        []string // why each tag-targeted ruleset does not qualify
		uninterpretable []string // patterns this check refused to guess about
	)
	disqualify := func(rs gateTagRuleset, why string) {
		examined = append(examined, fmt.Sprintf("  ruleset %q (id %d): %s", rs.Name, rs.ID, why))
	}
	tagTargeted := 0
	for _, rs := range rulesets {
		if rs.Target != "tag" {
			continue
		}
		tagTargeted++
		if rs.Enforcement != "active" {
			disqualify(rs, fmt.Sprintf("enforcement is %q, and anything but \"active\" restricts nobody — \"evaluate\" is a dry run and \"disabled\" is off", rs.Enforcement))
			continue
		}

		covered, unreadable := false, false
		for _, p := range rs.Conditions.RefName.Include {
			ok, err := gateTagRulePatternCovers(p, ref)
			if err != nil {
				uninterpretable = append(uninterpretable, fmt.Sprintf("  ruleset %q (id %d) include: %v", rs.Name, rs.ID, err))
				unreadable = true
				continue
			}
			if ok {
				covered = true
				break
			}
		}
		if !covered {
			if unreadable {
				disqualify(rs, "no interpretable include pattern covers "+ref+" (and at least one pattern was refused as uninterpretable — see below)")
			} else {
				disqualify(rs, fmt.Sprintf("no include pattern covers %s (includes: %q)", ref, rs.Conditions.RefName.Include))
			}
			continue
		}

		// The exclude side inverts the safe direction: an exclude this check
		// under-matched would count a ruleset as protecting a tag the forge
		// exempts from it. So an exclude that COVERS the ref disqualifies,
		// and an exclude that cannot be interpreted disqualifies too — this
		// ruleset cannot be stood behind either way, and if no other one
		// qualifies the refusal below says which pattern was unreadable.
		excluded := false
		for _, p := range rs.Conditions.RefName.Exclude {
			ok, err := gateTagRulePatternCovers(p, ref)
			if err != nil {
				uninterpretable = append(uninterpretable, fmt.Sprintf("  ruleset %q (id %d) exclude: %v", rs.Name, rs.ID, err))
				excluded = true // not known to exclude — but not known NOT to, which is the same verdict here
				continue
			}
			if ok {
				excluded = true
				break
			}
		}
		if excluded {
			disqualify(rs, fmt.Sprintf("an exclude pattern covers %s, or carries a shape this check refuses to guess about (excludes: %q)", ref, rs.Conditions.RefName.Exclude))
			continue
		}

		var creation, update bool
		var types []string
		for _, rule := range rs.Rules {
			types = append(types, rule.Type)
			creation = creation || rule.Type == "creation"
			update = update || rule.Type == "update"
		}
		switch {
		case !creation && !update:
			disqualify(rs, fmt.Sprintf("it covers the tag and restricts neither creation nor update (rules: %q) — a ruleset with no creation rule lets anyone with push rights mint the tag, which is the exact gap this check exists for", types))
		case !creation:
			disqualify(rs, fmt.Sprintf("it covers the tag and carries no `creation` rule (rules: %q), so anyone with push rights can still mint it", types))
		case !update:
			disqualify(rs, fmt.Sprintf("it covers the tag and carries no `update` rule (rules: %q), so an existing tag can still be force-moved onto other content — and a tag push, force or not, is what fires the Release workflow", types))
		default:
			// Qualifying: active, tag-targeted, covering, not excluded, and
			// restricting both doors. This is the pass, and it is the ONLY
			// pass — everything else in this function is a refusal.
			return nil
		}
	}

	if len(uninterpretable) > 0 {
		return fmt.Errorf("%w: no ruleset on %s could be shown to restrict creating %s, and at least one pattern was refused rather than guessed about:\n%s\n"+
			"This is a failure of the READING, not a verdict on the configuration: respell the pattern in a form this check interprets (`~ALL`, or `refs/tags/…` with `*`/`**`), or extend the matcher with a measured reading — never by assuming the permissive direction. The rule this repository expects is specified in docs/RELEASING.md under %q",
			errGateUncheckable, slug, version, strings.Join(uninterpretable, "\n"), gateTagRulesProcedureItem)
	}

	detail := "no ruleset targets tags at all"
	if tagTargeted > 0 {
		detail = fmt.Sprintf("%d ruleset(s) target tags and none of them qualifies:\n%s", tagTargeted, strings.Join(examined, "\n"))
	}
	return fmt.Errorf("%w: %s lists %d ruleset(s) and %s\n"+
		"So the forge will accept refs/tags/%s from ANYONE with push rights — including one who first weakened the gate in the tagged tree, which is the residual this check exists to refuse. "+
		"The fix is a maintainer's and lives on the forge, not in this tree: an ACTIVE tag ruleset whose include covers %sv* and whose rules restrict `creation` and `update`, specified step by step in docs/RELEASING.md under %q. This driver publishes nothing until it exists",
		errGateTagsUnprotected, slug, total, detail, version, gateTagRulesRefPrefix, gateTagRulesProcedureItem)
}

// ---------------------------------------------------------------------
// reading the forge
// ---------------------------------------------------------------------

// gateTagRulesForge fetches rulesets from the forge. Both fields are SEAMS
// WITH A PRODUCTION DEFAULT, the shape gateArchivesForge established and for
// its reason: every refusal below must be constructible in a test without a
// network, and the zero value — which is what production uses — means `gh`.
type gateTagRulesForge struct {
	// list fetches GET repos/<slug>/rulesets, every page. Nil means drive `gh`.
	// The bytes may be ONE JSON array or SEVERAL CONCATENATED — `gh api
	// --paginate` emits one array per page back to back — and the reader
	// below accepts both, so a fixture handing in a single array exercises
	// the same path production takes.
	list func(slug string) ([]byte, error)
	// detail fetches GET repos/<slug>/rulesets/<id>: one ruleset, with the
	// conditions and rules the list omits. Nil means drive `gh`.
	detail func(slug string, id int64) ([]byte, error)
}

// gh runs one read-only `gh api` call, stdout and stderr kept apart: the
// answer is JSON and the diagnosis is prose, and CombinedOutput would hand the
// JSON decoder a document with a warning spliced into it.
func gateTagRulesGH(args ...string) ([]byte, error) {
	// gateArchivesForgeCLI, not a second spelling: D7 already established that
	// `gh` is the one tool this pipeline drives at the forge, because it is
	// the tool docs/RELEASING.md tells a maintainer to verify a release with.
	tool, err := exec.LookPath(gateArchivesForgeCLI)
	if err != nil {
		return nil, fmt.Errorf("%w: `%s` is not on PATH (%v), so the forge's tag rulesets cannot be read and this check cannot run. "+
			"It FAILS rather than skips: a release published without confirming the forge restricts its tag is the unchecked publish this check exists to stop. "+
			"Install the GitHub CLI (https://cli.github.com) and authenticate it with `%s auth login` before authorizing a release",
			errGateUncheckable, gateArchivesForgeCLI, err, gateArchivesForgeCLI)
	}
	cmd := exec.Command(tool, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, gateTagRulesForgeRefusal(strings.Join(append([]string{gateArchivesForgeCLI}, args...), " "),
			err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// gateTagRulesForgeRefusal turns a failed forge read into a refusal that says
// what to fix, and it is a pure function of the failure's text so that every
// branch is reachable from a test.
//
// THE 403 AND THE 404 BOTH NAME THE SCOPE, and the 404 doing so is the point:
// GitHub answers 404, not 403, for a private repository a token cannot see, so
// "not found" is the most common costume an insufficient token wears. A
// refusal that read 404 as "wrong slug" would send the operator to check a
// remote URL that is right while the token stays wrong.
func gateTagRulesForgeRefusal(invocation string, err error, stderr string) error {
	scope := "a classic token needs the `repo` scope (`public_repo` suffices while the repository is public); a fine-grained token needs the repository `Metadata: read` permission"
	switch {
	case strings.Contains(stderr, "HTTP 401"):
		return fmt.Errorf("%w: the forge rejected the credential outright (`%s`): %v\n%s\n"+
			"No token was accepted at all, so nothing about the tag rulesets could be read; run `%s auth login` (or repair GH_TOKEN) and re-run. An unauthenticated answer is a refusal, never a pass",
			errGateUncheckable, invocation, err, stderr, gateArchivesForgeCLI)
	case strings.Contains(stderr, "HTTP 403"):
		return fmt.Errorf("%w: the forge turned the token away from the rulesets (`%s`): %v\n%s\n"+
			"The token authenticated and lacks the scope for this read — %s. Grant that scope and re-run; a check the token cannot make is a FAILED check, never a skipped one",
			errGateUncheckable, invocation, err, stderr, scope)
	case strings.Contains(stderr, "HTTP 404"):
		return fmt.Errorf("%w: the forge answered \"not found\" for the rulesets (`%s`): %v\n%s\n"+
			"Two states wear this answer and only one of them is about the path: the repository does not exist under that slug, OR it is private and the token cannot see it — GitHub answers 404, not 403, for that, so check the token's scope (%s) before the remote URL",
			errGateUncheckable, invocation, err, stderr, scope)
	}
	return fmt.Errorf("%w: the forge's tag rulesets could not be read (`%s`): %v\n%s\n"+
		"Whatever the cause — network, rate limit, an API shape this check has not met — the release is refused rather than published over an unread answer",
		errGateUncheckable, invocation, err, stderr)
}

// rulesets is both reads: the list, then one detail fetch per TAG-targeted
// entry. Branch- and push-targeted rulesets are counted and never fetched —
// their details cannot make a tag protected, and N needless API calls during
// a release is how a rate limit becomes a release incident.
func (f gateTagRulesForge) rulesets(slug string) (total int, tag []gateTagRuleset, err error) {
	list := f.list
	if list == nil {
		list = func(slug string) ([]byte, error) {
			// --paginate, because the list endpoint pages and a qualifying
			// ruleset on page two of an unread listing is a protected release
			// refused for a truncation nobody chose — the narrowed-coverage
			// shape CLAUDE.md forbids, arrived at by default page size.
			return gateTagRulesGH("api", "--paginate", "repos/"+slug+"/rulesets")
		}
	}
	blob, err := list(slug)
	if err != nil {
		return 0, nil, err
	}

	// One decoder, run to EOF: a single array (a fixture, or one page) and
	// several arrays back to back (`gh api --paginate`) decode identically.
	var summaries []gateTagRuleset
	dec := json.NewDecoder(bytes.NewReader(blob))
	for {
		var page []gateTagRuleset
		if err := dec.Decode(&page); err == io.EOF {
			break
		} else if err != nil {
			return 0, nil, fmt.Errorf("%w: the forge's ruleset listing for %s could not be decoded (%v), so whether the tag is protected is unknown — and unknown is a refusal, not a pass. The undecodable answer begins: %.200s",
				errGateUncheckable, slug, err, strings.TrimSpace(string(blob)))
		}
		summaries = append(summaries, page...)
	}

	detail := f.detail
	if detail == nil {
		detail = func(slug string, id int64) ([]byte, error) {
			return gateTagRulesGH("api", fmt.Sprintf("repos/%s/rulesets/%d", slug, id))
		}
	}
	for _, s := range summaries {
		if s.Target != "tag" {
			continue
		}
		blob, err := detail(slug, s.ID)
		if err != nil {
			return 0, nil, err
		}
		var full gateTagRuleset
		if err := json.Unmarshal(blob, &full); err != nil {
			return 0, nil, fmt.Errorf("%w: ruleset %d of %s could not be decoded (%v), and it targets tags, so it might be the very rule this check is looking for — refusing is the only reading that does not guess",
				errGateUncheckable, s.ID, slug, err)
		}
		tag = append(tag, full)
	}
	return len(summaries), tag, nil
}

// gateTagRulesSource is the production reader; a zero gateTagRulesForge means
// `gh` for both reads. It is a package-level var for gateArchivesSource's
// reason — a test that must reach the wired path can substitute it — though
// every test in this file constructs its own forge value instead and none
// mutates this.
var gateTagRulesSource = gateTagRulesForge{}

// ---------------------------------------------------------------------
// which repository to ask about
// ---------------------------------------------------------------------

// gateTagRulesParseRemote reads owner/repo out of a git remote URL, and
// refuses everything it cannot read. The refusals matter more than the
// parsing: a local path (every fixture origin in this package), a foreign
// forge, or an unrecognized shape must each become an errGateUncheckable that
// says why — a parser that "did its best" here would ask GitHub's API about a
// repository that is not the one the tag will land on.
func gateTagRulesParseRemote(url string) (slug string, err error) {
	trimmed := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(url), "/"), ".git")
	var host, path string
	switch {
	case strings.HasPrefix(trimmed, "https://"), strings.HasPrefix(trimmed, "http://"):
		rest := trimmed[strings.Index(trimmed, "://")+3:]
		if i := strings.Index(rest, "/"); i > 0 {
			host, path = rest[:i], rest[i+1:]
		}
	case strings.HasPrefix(trimmed, "ssh://"):
		rest := strings.TrimPrefix(trimmed, "ssh://")
		if at := strings.Index(rest, "@"); at >= 0 {
			rest = rest[at+1:]
		}
		if i := strings.Index(rest, "/"); i > 0 {
			host, path = rest[:i], rest[i+1:]
		}
	case strings.Contains(trimmed, "@") && strings.Contains(trimmed, ":") && !strings.Contains(trimmed, "://"):
		// scp-like: git@github.com:owner/repo
		rest := trimmed[strings.Index(trimmed, "@")+1:]
		if i := strings.Index(rest, ":"); i > 0 {
			host, path = rest[:i], rest[i+1:]
		}
	}
	if host == "" || path == "" {
		return "", fmt.Errorf("%w: the remote URL %q could not be parsed as a forge URL, so there is no repository to ask about tag rulesets. "+
			"This is the shape of a filesystem-path remote — every test fixture's origin, and any mirror-by-path layout — and a release cannot be published to one of those anyway",
			errGateUncheckable, url)
	}
	if !strings.EqualFold(host, "github.com") {
		return "", fmt.Errorf("%w: the remote %q is on %s, and this check speaks GitHub's rulesets API and no other forge's. "+
			"It refuses rather than assuming another forge is configured equivalently — an assumption in that direction is precisely a check that did not happen reading as one that did",
			errGateUncheckable, url, host)
	}
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("%w: the remote URL %q does not carry an owner/repo path, so the rulesets of no repository can be asked about", errGateUncheckable, url)
	}
	return parts[0] + "/" + parts[1], nil
}

// gateTagRulesRepoSlug asks git which repository the tag will actually land
// on: the URL of the remote D6 pushes to, in the checkout being released —
// never a hardcoded slug, which would keep passing after a fork or a rename
// while the tag lands somewhere this check never looked.
func gateTagRulesRepoSlug(dir, remote string) (string, error) {
	url, err := gateGit(dir, "remote", "get-url", remote)
	if err != nil {
		return "", fmt.Errorf("%w: the URL of remote %q could not be read in %s, so which forge repository would receive the tag is unknown — and a check about an unknown repository is no check: %w",
			errGateUncheckable, remote, dir, err)
	}
	return gateTagRulesParseRemote(url)
}

// gateTagRulesVerify is the production entry, and the whole of it is the three
// functions above in order: which repository, what the forge says about it,
// and what that means for this release. It is what gateDriverWired answers
// D1's TagRules question with.
func gateTagRulesVerify(dir, remote, version string) error {
	slug, err := gateTagRulesRepoSlug(dir, remote)
	if err != nil {
		return err
	}
	total, tag, err := gateTagRulesSource.rulesets(slug)
	if err != nil {
		return err
	}
	return gateTagRulesDecide(version, slug, total, tag)
}

// ---------------------------------------------------------------------
// the tests — every one of them offline, every response injected
// ---------------------------------------------------------------------

// gateTagRulesQualifying is the ruleset docs/RELEASING.md tells a maintainer
// to create, as the decision function sees it.
func gateTagRulesQualifying() gateTagRuleset {
	rs := gateTagRuleset{ID: 1, Name: "release tags", Target: "tag", Enforcement: "active"}
	rs.Conditions.RefName.Include = []string{"refs/tags/v*"}
	rs.Rules = []struct {
		Type string `json:"type"`
	}{{Type: "creation"}, {Type: "update"}, {Type: "deletion"}}
	return rs
}

// TestTheTagRulePatternMatcherFailsClosed holds the matcher to its own rule:
// interpret what can be stood behind, refuse the rest, and never guess in the
// permissive direction.
func TestTheTagRulePatternMatcherFailsClosed(t *testing.T) {
	const ref = "refs/tags/v9.9.9"
	for _, tc := range []struct {
		pattern string
		covers  bool
		refuses bool
	}{
		{pattern: "~ALL", covers: true},
		{pattern: "refs/tags/v*", covers: true},
		{pattern: "refs/tags/v9.9.9", covers: true},
		{pattern: "refs/tags/**", covers: true},
		{pattern: "refs/tags/v8.*", covers: false},
		{pattern: "refs/tags/release-*", covers: false},
		// Unqualified: whether GitHub would qualify these is GitHub's
		// normalization to know, and both directions of guessing are wrong —
		// permissive clears an unprotected release, restrictive accuses a
		// configuration that may be fine. So: refused.
		{pattern: "v*", refuses: true},
		{pattern: "~DEFAULT_BRANCH", refuses: true},
		{pattern: "refs/heads/main", refuses: true},
		{pattern: "", refuses: true},
		// fnmatch features this check has not measured: refused, named.
		{pattern: "refs/tags/v?.?.?", refuses: true},
		{pattern: "refs/tags/v[0-9]*", refuses: true},
	} {
		got, err := gateTagRulePatternCovers(tc.pattern, ref)
		switch {
		case tc.refuses && err == nil:
			t.Errorf("pattern %q was interpreted (covers=%v); it must be refused, because whatever it means is a guess and a permissive guess clears an unprotected release", tc.pattern, got)
		case !tc.refuses && err != nil:
			t.Errorf("pattern %q was refused (%v); it is one of the shapes this check stands behind", tc.pattern, err)
		case !tc.refuses && got != tc.covers:
			t.Errorf("pattern %q against %s: got covers=%v, want %v", tc.pattern, ref, got, tc.covers)
		}
	}
}

// TestTheTagRulesDecisionRefusesEveryUnprotectedShape walks the shapes a
// forge's answer can take. The rows are the point: each is a configuration
// that LOOKS like protection from one angle — a ruleset exists, a ruleset
// targets tags, a ruleset even covers the pattern — and the decision must
// refuse every one of them, saying which angle is missing.
func TestTheTagRulesDecisionRefusesEveryUnprotectedShape(t *testing.T) {
	const version, slug = "v9.9.9", "example/repo"

	mutate := func(edit func(*gateTagRuleset)) gateTagRuleset {
		rs := gateTagRulesQualifying()
		edit(&rs)
		return rs
	}

	for _, tc := range []struct {
		name     string
		rulesets []gateTagRuleset
		total    int
		sentinel error
		fragment string
	}{
		{
			name:     "no rulesets at all",
			sentinel: errGateTagsUnprotected,
			fragment: "no ruleset targets tags at all",
		},
		{
			name:     "only branch rulesets",
			rulesets: []gateTagRuleset{mutate(func(rs *gateTagRuleset) { rs.Target = "branch" })},
			total:    1,
			sentinel: errGateTagsUnprotected,
			fragment: "no ruleset targets tags at all",
		},
		{
			name:     "the tag ruleset is a dry run",
			rulesets: []gateTagRuleset{mutate(func(rs *gateTagRuleset) { rs.Enforcement = "evaluate" })},
			total:    1,
			sentinel: errGateTagsUnprotected,
			fragment: "dry run",
		},
		{
			name:     "the tag ruleset is disabled",
			rulesets: []gateTagRuleset{mutate(func(rs *gateTagRuleset) { rs.Enforcement = "disabled" })},
			total:    1,
			sentinel: errGateTagsUnprotected,
			fragment: `enforcement is "disabled"`,
		},
		{
			name: "the pattern covers other tags",
			rulesets: []gateTagRuleset{mutate(func(rs *gateTagRuleset) {
				rs.Conditions.RefName.Include = []string{"refs/tags/release-*"}
			})},
			total:    1,
			sentinel: errGateTagsUnprotected,
			fragment: "no include pattern covers refs/tags/" + version,
		},
		{
			name: "an exclude exempts the release tag",
			rulesets: []gateTagRuleset{mutate(func(rs *gateTagRuleset) {
				rs.Conditions.RefName.Exclude = []string{"refs/tags/v9.*"}
			})},
			total:    1,
			sentinel: errGateTagsUnprotected,
			fragment: "exclude pattern covers",
		},
		{
			name: "creation is not restricted",
			rulesets: []gateTagRuleset{mutate(func(rs *gateTagRuleset) {
				rs.Rules = rs.Rules[1:] // update, deletion
			})},
			total:    1,
			sentinel: errGateTagsUnprotected,
			fragment: "no `creation` rule",
		},
		{
			name: "update is not restricted",
			rulesets: []gateTagRuleset{mutate(func(rs *gateTagRuleset) {
				rs.Rules = rs.Rules[:1] // creation only
			})},
			total:    1,
			sentinel: errGateTagsUnprotected,
			fragment: "no `update` rule",
		},
		{
			name: "the only candidate carries a pattern this check cannot read",
			rulesets: []gateTagRuleset{mutate(func(rs *gateTagRuleset) {
				rs.Conditions.RefName.Include = []string{"refs/tags/v[0-9]*"}
			})},
			total: 1,
			// The reading failed, not the configuration: accusing the
			// configuration over a pattern nobody could read accuses the
			// wrong party and sends the maintainer to re-create a rule that
			// may be fine.
			sentinel: errGateUncheckable,
			fragment: "refused rather than guessed",
		},
		{
			name: "an exclude this check cannot read poisons the candidate",
			rulesets: []gateTagRuleset{mutate(func(rs *gateTagRuleset) {
				rs.Conditions.RefName.Exclude = []string{"refs/tags/v[89].*"}
			})},
			total:    1,
			sentinel: errGateUncheckable,
			fragment: "refused rather than guessed",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := gateTagRulesDecide(version, slug, tc.total, tc.rulesets)
			if err == nil {
				t.Fatal("the decision cleared a release whose tag nothing on the forge restricts; this is the residual docs/RELEASING.md recorded, read back as closed")
			}
			if !errors.Is(err, tc.sentinel) {
				t.Errorf("wrong sentinel: got %v", err)
			}
			if !strings.Contains(err.Error(), tc.fragment) {
				t.Errorf("the refusal does not say what is missing (want %q).\nIt reads:\n%v", tc.fragment, err)
			}
			if errors.Is(err, errGateTagsUnprotected) && !strings.Contains(err.Error(), gateTagRulesProcedureItem) {
				t.Errorf("a refusal accusing the configuration must name the docs/RELEASING.md item that specifies the fix (%q), or the operator researches it at the worst moment.\nIt reads:\n%v",
					gateTagRulesProcedureItem, err)
			}
		})
	}

	// And the one pass: the ruleset the procedure specifies clears the
	// release, even beside rulesets that do not.
	noise := gateTagRulesQualifying()
	noise.ID, noise.Enforcement = 2, "evaluate"
	if err := gateTagRulesDecide(version, slug, 2, []gateTagRuleset{noise, gateTagRulesQualifying()}); err != nil {
		t.Errorf("the exact configuration docs/RELEASING.md specifies was refused: %v", err)
	}

	// A version this matcher's conservatism is not free over is refused
	// before any pattern is consulted.
	if err := gateTagRulesDecide("v9/9", slug, 1, []gateTagRuleset{gateTagRulesQualifying()}); !errors.Is(err, errGateUncheckable) {
		t.Errorf("a version containing `/` must be refused as unmatchable — it is where fnmatch dialects diverge on the exclude side; got %v", err)
	}
}

// TestTheTagRulesForgeErrorsNameTheMissingScope holds the refusal wording for
// the reads that cannot be made. The 404 row is the one to read: GitHub
// answers 404, not 403, for a private repository a token cannot see, so a
// refusal that read it as "wrong URL" would send the operator away from the
// actual fix.
func TestTheTagRulesForgeErrorsNameTheMissingScope(t *testing.T) {
	for _, tc := range []struct {
		name, stderr string
		fragments    []string
	}{
		{
			name:      "401: no credential accepted",
			stderr:    "gh: Bad credentials (HTTP 401)",
			fragments: []string{"auth login", "never a pass"},
		},
		{
			name:      "403: the token lacks the scope",
			stderr:    "gh: Resource not accessible by personal access token (HTTP 403)",
			fragments: []string{"`repo` scope", "Metadata: read", "FAILED check, never a skipped one"},
		},
		{
			name:      "404: the costume an insufficient token wears",
			stderr:    "gh: Not Found (HTTP 404)",
			fragments: []string{"private and the token cannot see it", "`repo` scope", "Metadata: read"},
		},
		{
			name:      "anything else: quoted whole, refused whole",
			stderr:    "error connecting to api.github.com",
			fragments: []string{"refused rather than published over an unread answer", "error connecting to api.github.com"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := gateTagRulesForgeRefusal("gh api repos/example/repo/rulesets", errors.New("exit status 1"), tc.stderr)
			if !errors.Is(err, errGateUncheckable) {
				t.Fatalf("a read that could not be made must be uncheckable; got %v", err)
			}
			for _, fragment := range tc.fragments {
				if !strings.Contains(err.Error(), fragment) {
					t.Errorf("the refusal does not carry %q.\nIt reads:\n%v", fragment, err)
				}
			}
		})
	}
}

// TestTheTagRulesSlugIsReadFromTheRemote pins the parser to the URL shapes git
// remotes actually take, and to refusing the rest by name.
func TestTheTagRulesSlugIsReadFromTheRemote(t *testing.T) {
	for _, tc := range []struct {
		url, slug, refusal string
	}{
		{url: "https://github.com/BarterX-Tech/dossierx.git", slug: "BarterX-Tech/dossierx"},
		{url: "https://github.com/BarterX-Tech/dossierx", slug: "BarterX-Tech/dossierx"},
		{url: "git@github.com:BarterX-Tech/dossierx.git", slug: "BarterX-Tech/dossierx"},
		{url: "ssh://git@github.com/BarterX-Tech/dossierx.git", slug: "BarterX-Tech/dossierx"},
		{url: "https://gitlab.com/owner/repo.git", refusal: "no other forge"},
		{url: "/Users/somebody/fixtures/origin", refusal: "filesystem-path"},
		{url: "https://github.com/only-an-owner", refusal: "does not carry an owner/repo path"},
		{url: "", refusal: "could not be parsed"},
	} {
		slug, err := gateTagRulesParseRemote(tc.url)
		switch {
		case tc.refusal != "":
			if err == nil {
				t.Errorf("%q parsed to %q; it must be refused — asking GitHub about a slug read out of this URL asks about a repository the tag will not land on", tc.url, slug)
			} else if !errors.Is(err, errGateUncheckable) || !strings.Contains(err.Error(), tc.refusal) {
				t.Errorf("%q: refusal must be uncheckable and say %q; got %v", tc.url, tc.refusal, err)
			}
		case err != nil:
			t.Errorf("%q was refused (%v); it is an ordinary remote shape", tc.url, err)
		case slug != tc.slug:
			t.Errorf("%q parsed to %q, want %q", tc.url, slug, tc.slug)
		}
	}
}

// TestTheTagRulesReaderAcceptsTheForgesPagingAndRefusesWhatItCannotDecode
// exercises the reader over injected bytes: `gh api --paginate` emits one JSON
// array per page back to back, a fixture emits one array, and both must decode
// to the same listing — while an undecodable answer is a refusal, because a
// listing this check could not read might hold the very rule it is looking
// for.
func TestTheTagRulesReaderAcceptsTheForgesPagingAndRefusesWhatItCannotDecode(t *testing.T) {
	detailFor := func(t *testing.T) (func(string, int64) ([]byte, error), *[]int64) {
		t.Helper()
		var asked []int64
		return func(slug string, id int64) ([]byte, error) {
			asked = append(asked, id)
			blob, err := json.Marshal(gateTagRulesQualifying())
			return blob, err
		}, &asked
	}

	t.Run("two pages, and only the tag-targeted entries cost a detail fetch", func(t *testing.T) {
		detail, asked := detailFor(t)
		forge := gateTagRulesForge{
			list: func(string) ([]byte, error) {
				// Two concatenated arrays, exactly as --paginate emits them.
				return []byte(`[{"id":1,"target":"branch"},{"id":2,"target":"tag"}][{"id":3,"target":"tag"}]`), nil
			},
			detail: detail,
		}
		total, tag, err := forge.rulesets("example/repo")
		if err != nil {
			t.Fatalf("a well-formed paged listing was refused: %v", err)
		}
		if total != 3 || len(tag) != 2 {
			t.Errorf("got %d total / %d tag-targeted; want 3 / 2", total, len(tag))
		}
		if len(*asked) != 2 || (*asked)[0] != 2 || (*asked)[1] != 3 {
			t.Errorf("detail was fetched for %v; only the tag-targeted ids 2 and 3 can make a tag protected, and needless calls during a release are how a rate limit becomes a release incident", *asked)
		}
	})

	t.Run("an undecodable listing is a refusal", func(t *testing.T) {
		forge := gateTagRulesForge{list: func(string) ([]byte, error) { return []byte(`<html>rate limited</html>`), nil }}
		if _, _, err := forge.rulesets("example/repo"); !errors.Is(err, errGateUncheckable) {
			t.Errorf("an answer this check cannot read must refuse the release; got %v", err)
		}
	})

	t.Run("an undecodable tag ruleset is a refusal, not a skipped entry", func(t *testing.T) {
		forge := gateTagRulesForge{
			list:   func(string) ([]byte, error) { return []byte(`[{"id":7,"target":"tag"}]`), nil },
			detail: func(string, int64) ([]byte, error) { return []byte(`not json`), nil },
		}
		if _, _, err := forge.rulesets("example/repo"); !errors.Is(err, errGateUncheckable) {
			t.Errorf("a tag-targeted ruleset this check could not decode might be the very rule it is looking for; skipping it narrows coverage silently. Got %v", err)
		}
	})

	t.Run("a failed detail read fails the whole question", func(t *testing.T) {
		wanted := errors.New("the forge hung up")
		forge := gateTagRulesForge{
			list:   func(string) ([]byte, error) { return []byte(`[{"id":7,"target":"tag"}]`), nil },
			detail: func(string, int64) ([]byte, error) { return nil, wanted },
		}
		if _, _, err := forge.rulesets("example/repo"); !errors.Is(err, wanted) {
			t.Errorf("the detail read's own error must surface whole; got %v", err)
		}
	})
}

// TestTheDriverRefusesWhenTheForgeDoesNotRestrictTagCreation is the
// integration row: the forge-rule question is a D1 clause, so a forge that
// answers "unprotected" — or cannot be read — stops an otherwise perfect
// release before anything is merged, tagged or pushed. The tree in this
// fixture is green, self-consistent and CI-evidenced; the ONLY thing wrong is
// outside the repository, which is the entire point of the clause.
func TestTheDriverRefusesWhenTheForgeDoesNotRestrictTagCreation(t *testing.T) {
	const version = "v9.9.9"
	for _, tc := range []struct {
		name     string
		answer   error
		sentinel error
	}{
		{
			name:     "the forge restricts nobody",
			answer:   fmt.Errorf("%w: example/repo lists 0 ruleset(s)", errGateTagsUnprotected),
			sentinel: errGateTagsUnprotected,
		},
		{
			name:     "the forge could not be read",
			answer:   fmt.Errorf("%w: the token lacks the scope", errGateUncheckable),
			sentinel: errGateUncheckable,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := gateDriverFixture(t, version)
			mainWas := gateDriverRemoteHead(t, repo, repo.Base)

			ev := gateDriverGreenEvidence()
			ev.tagRules = tc.answer
			run := gateDriverPublish(gateDriverAuthorized(t, repo, version), repo, ev)

			if run.Err == nil {
				t.Fatal("the driver published a release onto a forge that accepts the tag from anyone with push rights — the exact residual this precondition exists to refuse")
			}
			if run.Failed != "D1" {
				t.Errorf("the run stopped at %s; the forge rule is a precondition and must refuse before D2's merge, while a failure is still free", run.Failed)
			}
			if !errors.Is(run.Err, tc.sentinel) {
				t.Errorf("the refusal must carry the check's own sentinel; got %v", run.Err)
			}
			gateDriverAssertNothingPublished(t, repo, version, mainWas)
		})
	}

	// And the unwired evidence source refuses this question like its others:
	// by saying it cannot answer, never by assuming a restriction nobody read.
	if err := (gateDriverUnwired{}).TagRules("origin", version); !errors.Is(err, errGateUncheckable) {
		t.Errorf("gateDriverUnwired must answer the forge-rule question with a refusal; got %v", err)
	}
}

// TestTheWrittenProcedureNamesTheForgeRuleTheDriverRequires keeps
// docs/RELEASING.md and this check telling one story. The document is itself a
// gated surface — anything written there must be true of the code — and the
// maintainer configures the rule FROM the document, so a document that named a
// weaker rule than the decision requires would walk them straight into the
// refusal it exists to prevent.
func TestTheWrittenProcedureNamesTheForgeRuleTheDriverRequires(t *testing.T) {
	root := surfaceRepoRoot(t)
	procedure := gateReadRepoFile(t, root, "docs/RELEASING.md")

	item := regexp.MustCompile(`(?s)- \[ \] \*\*` + regexp.QuoteMeta(gateTagRulesProcedureItem) + `\.\*\*.*?\n- \[ \] `).FindString(procedure)
	if item == "" {
		t.Fatalf("docs/RELEASING.md carries no item titled **%s.**. The refusals in gate_tagrules_test.go send a maintainer to that item by name, and the rule cannot be created from inside this repository — without the item, the refusal is an accusation with no instructions attached",
			gateTagRulesProcedureItem)
	}

	for _, want := range []struct{ fragment, why string }{
		{"refs/tags/v*", "the pattern the maintainer configures must be the fully-qualified one the matcher interprets"},
		{"creation", "the decision requires a creation rule, so the document must tell the maintainer to restrict creations"},
		{"update", "the decision requires an update rule — a force-moved tag fires the Release workflow too — so the document must say to restrict updates"},
		{"Active", "the decision refuses \"evaluate\" and \"disabled\" enforcement, so the document must name Active"},
		{"rulesets", "the mechanism is rulesets — the older tag protection rules were sunset with their API — and a maintainer sent to the retired feature configures nothing"},
		{"gh api", "the item owes the maintainer a runnable read-back command, or verifying the configuration means clicking through a UI from memory"},
		{"bypass", "the check does not read the bypass list, and the document must say so and hand that reading to the maintainer — a boundary implied is a boundary assumed closed"},
	} {
		if !strings.Contains(item, want.fragment) {
			t.Errorf("the %q item no longer says %q. %s.\nThe item reads:\n%s", gateTagRulesProcedureItem, want.fragment, want.why, item)
		}
	}
}
