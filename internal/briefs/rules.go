package briefs

import (
	"fmt"
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// The brief rule set. It is a registry of its own, beside lint.Registry and
// never inside it, because the claim Lint interface takes []model.Claim and
// nothing else: a brief is not a claim, and widening that interface to carry
// one would teach every claim rule about a second kind of document. surface.json
// inventories this set under brief_rules; testdata/fixture-coverage/brief holds
// one fixture per rule, and tests/brief_fixtures_test.go requires exactly that.
//
// Every rule's findings ride in `check`'s lint_findings, keyed by `lint` like
// every other finding, so an agent's recovery table needs no second shape. A
// brief finding's claim_id is the PATH of the file it is about
// ("briefs/checkout/flow.md"), or of the folder ("briefs/checkout/") or the tree
// ("briefs/") for a cap on either: there is no claim to name, a path is what a
// reader opens, and a path can never be mistaken for a claim id (claim ids hold
// no slash). The constitution's findings set the precedent with the literal
// "constitution".
const (
	RuleShape            = "brief-shape"
	RuleFrontmatter      = "brief-frontmatter"
	RuleWordCap          = "brief-word-cap"
	RuleImageCap         = "brief-image-cap"
	RuleFolderCap        = "brief-folder-cap"
	RuleTotalCap         = "brief-total-cap"
	RuleRestsOnUnknown   = "brief-rests-on-unknown"
	RuleRestsOnDuplicate = "brief-rests-on-duplicate"
)

// Rule is one entry of the brief rule set.
type Rule struct {
	Name     string
	Severity lint.Severity
}

// Rules is every brief rule, in report order: the shape of the tree first, then
// each file's frontmatter, then the caps from the smallest scope to the
// largest, then the two rules that need the claims.
//
// Every cap is an ERROR and final: `check` refuses the project until the brief
// is split or trimmed, or the human raises the cap. Only the duplicate rule is
// a WARNING — two briefs resting on exactly the same claims may well be one
// brief, but that is a question for a reader, not a refusal.
var Rules = []Rule{
	{RuleShape, lint.SeverityError},
	{RuleFrontmatter, lint.SeverityError},
	{RuleWordCap, lint.SeverityError},
	{RuleImageCap, lint.SeverityError},
	{RuleFolderCap, lint.SeverityError},
	{RuleTotalCap, lint.SeverityError},
	{RuleRestsOnUnknown, lint.SeverityError},
	{RuleRestsOnDuplicate, lint.SeverityWarning},
}

// RuleNames is Rules' names, sorted — the form surface.json inventories.
func RuleNames() []string {
	out := make([]string, 0, len(Rules))
	for _, r := range Rules {
		out = append(out, r.Name)
	}
	sort.Strings(out)
	return out
}

func severityOf(rule string) lint.Severity {
	for _, r := range Rules {
		if r.Name == rule {
			return r.Severity
		}
	}
	return lint.SeverityError
}

func ruleOrder(rule string) int {
	for i, r := range Rules {
		if r.Name == rule {
			return i
		}
	}
	return len(Rules)
}

// capApproval is the sentence every cap finding ends with, in module-claim-cap's
// words: the recovery is the author's, the override is the human's.
func capApproval(key string) string {
	return fmt.Sprintf("raising %s in project.config.yaml is the human's call, only on their explicit approval", key)
}

// checkCaps raises the four cap rules. It runs inside FromFiles because none of
// them needs anything but the tree: the counts are the tree's own.
func (s *Set) checkCaps() {
	c := s.Caps
	for _, b := range s.Briefs {
		if b.Words > c.Words {
			s.add(RuleWordCap, b.Path, "brief is %d words, over the cap of %d; split it into two briefs or trim it; %s", b.Words, c.Words, capApproval("max_brief_words"))
		}
		if len(b.Images) > c.Images {
			s.add(RuleImageCap, b.Path, "brief references %d images, over the cap of %d; keep the figures that carry the argument; %s", len(b.Images), c.Images, capApproval("max_brief_images"))
		}
		for _, img := range b.Images {
			if img.Present && img.Bytes > int64(c.ImageBytes) {
				s.add(RuleImageCap, b.Path, "image %q is %d bytes, over the cap of %d bytes; export it smaller; %s", img.Name, img.Bytes, c.ImageBytes, capApproval("max_brief_image_bytes"))
			}
		}
	}
	for _, f := range s.Folders {
		if f.Count > c.PerFolder {
			s.add(RuleFolderCap, dirPath(s.DisplayDir, f.Name), "folder %q holds %d briefs, over the cap of %d; split it into two folders or retire briefs; images do not count; %s", f.Name, f.Count, c.PerFolder, capApproval("max_briefs_per_folder"))
		}
	}
	if total := len(s.Briefs); total > c.Total {
		s.add(RuleTotalCap, dirPath(s.DisplayDir), "the project holds %d briefs, over the cap of %d; retire or merge briefs; images do not count; %s", total, c.Total, capApproval("max_briefs"))
	}
}

// Findings returns every brief finding for this set against claims — the
// discovery, frontmatter and cap findings the tree produced on its own, plus
// the two rules that need to know which claims exist — ordered by rule (Rules'
// order) and then by path, so two runs over one tree print the same bytes.
//
// claims is read and never changed, and nothing here feeds back into a claim:
// a brief resting on a claim is a fact about the brief.
func (s *Set) Findings(claims []model.Claim) []lint.Finding {
	if s == nil {
		return nil
	}
	out := append([]lint.Finding(nil), s.findings...)
	out = append(out, s.restsOnFindings(claims)...)
	sortFindings(out)
	return out
}

// TreeFindings is every finding the tree raised on its own — shape (an
// unreadable entry among them), frontmatter and the caps — in Findings' order,
// without the two rules that need the claims. It is what `brief list` and
// `brief show` report beside the briefs they read, so neither answers "no
// briefs" for a tree it could not fully read; those two commands load no
// claims, so a brief stays readable while a claim file is broken.
func (s *Set) TreeFindings() []lint.Finding {
	if s == nil {
		return nil
	}
	out := append([]lint.Finding(nil), s.findings...)
	sortFindings(out)
	return out
}

func sortFindings(out []lint.Finding) {
	sort.SliceStable(out, func(i, j int) bool {
		oi, oj := ruleOrder(out[i].LintName), ruleOrder(out[j].LintName)
		if oi != oj {
			return oi < oj
		}
		return out[i].ClaimID < out[j].ClaimID
	})
}

// restsOnFindings raises brief-rests-on-unknown for every id a brief rests on
// that no claim carries, and brief-rests-on-duplicate for every brief whose
// non-empty rests_on set is identical to another's. Two briefs that rest on
// nothing are not duplicates of each other: an empty set says nothing about
// what the briefs are about.
func (s *Set) restsOnFindings(claims []model.Claim) []lint.Finding {
	known := make(map[string]bool, len(claims))
	for _, c := range claims {
		known[c.ID] = true
	}
	var out []lint.Finding
	bySet := map[string][]string{}
	var keys []string
	for _, b := range s.Briefs {
		for _, id := range b.RestsOn {
			if !known[id] {
				out = append(out, lint.Finding{
					LintName: RuleRestsOnUnknown,
					ClaimID:  b.Path,
					Severity: severityOf(RuleRestsOnUnknown),
					Message:  fmt.Sprintf("rests_on names %q, which is not a claim; name a claim id that exists (dossierx claim list --match), or remove the entry", id),
				})
			}
		}
		if len(b.RestsOn) == 0 {
			continue
		}
		ids := append([]string(nil), b.RestsOn...)
		sort.Strings(ids)
		key := strings.Join(ids, "\x00")
		if _, ok := bySet[key]; !ok {
			keys = append(keys, key)
		}
		bySet[key] = append(bySet[key], b.Path)
	}
	for _, key := range keys {
		paths := bySet[key]
		if len(paths) < 2 {
			continue
		}
		for _, p := range paths {
			var others []string
			for _, q := range paths {
				if q != p {
					others = append(others, q)
				}
			}
			out = append(out, lint.Finding{
				LintName: RuleRestsOnDuplicate,
				ClaimID:  p,
				Severity: severityOf(RuleRestsOnDuplicate),
				Message:  fmt.Sprintf("rests_on is exactly the same set as %s; two briefs about the same claims may be one brief — merge them, or give each the claims it is actually about", strings.Join(others, ", ")),
			})
		}
	}
	return out
}
