package lock

import (
	"fmt"
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// DependencyCondition is a non-local condition attached to an otherwise
// admissible approval. It says why a claim is not dependency-ready without
// pretending that its reviewed statement was rejected.
type DependencyCondition struct {
	DependencyID string   `json:"dependency_id"`
	Kind         string   `json:"kind"`
	Path         []string `json:"path"`
	Detail       string   `json:"detail"`
}

// CandidateVerdict is one member's result from the shared set evaluator.
// LocalAdmissible is intentionally separate from Conditions: a local approval
// may be valid against a readable draft dependency while still not ready for
// integrated use.
type CandidateVerdict struct {
	ClaimID         string                `json:"claim_id"`
	LocalAdmissible bool                  `json:"local_admissible"`
	Refusals        []string              `json:"refusals"`
	Conditions      []DependencyCondition `json:"dependency_conditions"`
	OpenThreads     []string              `json:"open_threads,omitempty"`
	LintFindings    []lint.Finding        `json:"lint_findings,omitempty"`
}

// SetEvaluation is the one policy answer used by one-claim and group preview
// and write paths. UnrelatedFindings are disclosed but never turn an unrelated
// candidate into a hostage.
type SetEvaluation struct {
	RequestedIDs      []string           `json:"requested_ids"`
	PolicyVersion     PolicyVersion      `json:"policy_version"`
	Verdicts          []CandidateVerdict `json:"verdicts"`
	UnrelatedFindings []lint.Finding     `json:"unrelated_findings"`
}

// SemanticConflict is an explicit human/agent finding about incompatible
// meanings. The evaluator never derives it from comparable hashes: a hash can
// prove bytes differ, not that two statements contradict each other.
type SemanticConflict struct {
	ClaimID      string `json:"claim_id"`
	DependencyID string `json:"dependency_id,omitempty"`
	Detail       string `json:"detail"`
}

func (e SetEvaluation) Allowed() bool {
	for _, verdict := range e.Verdicts {
		if !verdict.LocalAdmissible {
			return false
		}
	}
	return true
}

// EvaluateSet evaluates the full final candidate state. A requested singleton
// and a requested group take exactly the same route. This function does not
// write claims or stores and does not infer semantic compatibility from a hash;
// semantic contradictions remain a human-review refusal supplied by callers.
func EvaluateSet(claims []model.Claim, requestedIDs []string, cfg *config.Config, store *Store) SetEvaluation {
	return EvaluateSetWithSemanticConflicts(claims, requestedIDs, cfg, store, nil)
}

// EvaluateSetWithSemanticConflicts evaluates the policy with explicit,
// reviewable semantic findings supplied by the caller. A conflict refuses
// approval and names human review as the required recovery; it is never
// inferred from a hash or silently cleared by a snapshot refresh.
func EvaluateSetWithSemanticConflicts(claims []model.Claim, requestedIDs []string, cfg *config.Config, store *Store, conflicts []SemanticConflict) SetEvaluation {
	ids := uniqueIDs(requestedIDs)
	result := SetEvaluation{RequestedIDs: ids, PolicyVersion: PolicyLegacy}
	if store != nil {
		result.PolicyVersion = store.PolicyVersion
	}
	requested := make(map[string]bool, len(ids))
	for _, id := range ids {
		requested[id] = true
	}

	candidate := make([]model.Claim, len(claims))
	copy(candidate, claims)
	for i := range candidate {
		if requested[candidate[i].ID] {
			candidate[i].Status = model.StatusLocked
			candidate[i].ReviewPending = false
		}
	}

	allFindings := lint.RunAll(candidate, cfg)
	byID := make(map[string]model.Claim, len(claims))
	for _, claim := range claims {
		byID[claim.ID] = claim
	}
	for _, id := range ids {
		verdict := CandidateVerdict{ClaimID: id, LocalAdmissible: true}
		claim, present := byID[id]
		if !present {
			verdict.LocalAdmissible = false
			verdict.Refusals = append(verdict.Refusals, "claim_not_found")
			result.Verdicts = append(result.Verdicts, verdict)
			continue
		}
		if claim.Status == model.StatusLocked {
			verdict.LocalAdmissible = false
			verdict.Refusals = append(verdict.Refusals, "already_locked")
		}
		if store != nil {
			if rec, ok := store.Record(id); ok && !rec.Released() {
				verdict.LocalAdmissible = false
				verdict.Refusals = append(verdict.Refusals, "standing_ledger_record")
			}
			if store.LedgerRecordDeleted(claim) {
				verdict.LocalAdmissible = false
				verdict.Refusals = append(verdict.Refusals, "ledger_record_deleted")
			}
			if store.CommentDigestUnrecorded(claim) {
				verdict.LocalAdmissible = false
				verdict.Refusals = append(verdict.Refusals, "comment_digest_unrecorded")
			}
		}
		if len(claim.OpenThreadIDs()) > 0 {
			verdict.LocalAdmissible = false
			verdict.Refusals = append(verdict.Refusals, "unresolved_comments")
			verdict.OpenThreads = claim.OpenThreadIDs()
		}
		for _, conflict := range conflicts {
			if conflict.ClaimID != id {
				continue
			}
			verdict.LocalAdmissible = false
			detail := "semantic_contradiction_requires_human_review"
			if conflict.DependencyID != "" {
				detail += ":" + conflict.DependencyID
			}
			if conflict.Detail != "" {
				detail += ":" + conflict.Detail
			}
			verdict.Refusals = append(verdict.Refusals, detail)
		}
		for _, finding := range allFindings {
			if finding.Severity == lint.SeverityWarning && !(finding.LintName == "roll-up" && finding.ClaimID == id) {
				continue
			}
			// Local approval deliberately replaces only the old "rests_on must
			// already be locked" doctrine. Other graph/integrity lints keep
			// their ordinary force; dependency readiness carries the visible
			// condition this one rule used to hide by refusing the approval.
			if store != nil && store.LocalApprovalEnabled() && finding.LintName == "rest-on-locked" {
				continue
			}
			if findingAffects(finding, id, claim.Module) {
				verdict.LocalAdmissible = false
				verdict.Refusals = append(verdict.Refusals, "lint:"+finding.LintName)
				verdict.LintFindings = append(verdict.LintFindings, finding)
			}
		}
		for _, depID := range claim.RestsOn.IDs {
			dep, ok := byID[depID]
			if !ok {
				verdict.LocalAdmissible = false
				verdict.Refusals = append(verdict.Refusals, "missing_dependency:"+depID)
				continue
			}
			if restCycleFrom(id, depID, byID) {
				verdict.LocalAdmissible = false
				verdict.Refusals = append(verdict.Refusals, "dependency_cycle:"+depID)
				continue
			}
			// Local approval relaxes a required edge only when its input is
			// readable. A retired claim is deliberately no longer a usable
			// premise, and an unrecognised lifecycle value cannot be treated as
			// readable merely because it loaded as YAML. These are required-edge
			// refusals; unrelated lifecycle lint stays scoped to its own claim.
			switch strings.ToLower(strings.TrimSpace(string(dep.Status))) {
			case "retired":
				verdict.LocalAdmissible = false
				verdict.Refusals = append(verdict.Refusals, "retired_dependency:"+depID)
				continue
			case "", string(model.StatusDraft), string(model.StatusLocked):
				// A normal draft is an admissible local-approval condition below.
			default:
				verdict.LocalAdmissible = false
				verdict.Refusals = append(verdict.Refusals, "unreadable_dependency:"+depID)
				continue
			}
			if store == nil || !store.LocalApprovalEnabled() {
				if !candidateLocked(depID, candidate) {
					verdict.LocalAdmissible = false
					verdict.Refusals = append(verdict.Refusals, "dependency_not_locked:"+depID)
				}
			} else if !candidateLocked(depID, candidate) {
				verdict.Conditions = append(verdict.Conditions, DependencyCondition{
					DependencyID: depID,
					Kind:         "dependency_unapproved",
					Path:         []string{id, depID},
					Detail:       "approved locally against a readable dependency that is not approved",
				})
			}
		}
		verdict.Refusals = uniqueStrings(verdict.Refusals)
		result.Verdicts = append(result.Verdicts, verdict)
	}
	for _, finding := range allFindings {
		if finding.Severity == lint.SeverityWarning {
			continue
		}
		if store != nil && store.LocalApprovalEnabled() && finding.LintName == "rest-on-locked" {
			continue
		}
		related := false
		for _, id := range ids {
			if findingAffects(finding, id, byID[id].Module) {
				related = true
				break
			}
		}
		if !related {
			result.UnrelatedFindings = append(result.UnrelatedFindings, finding)
		}
	}
	return result
}

// findingAffects is the scoping rule: a finding blocks a candidate when it
// names the candidate, or — for the module-manifest rule, whose ClaimID is
// the MODULE ("" = project-wide) and never a claim — when it is about the
// candidate's own module. A module with no valid manifest.yaml has no
// lockable claims (NIT-22); a defect in another module's file is not this
// candidate's to fix and does not hold it hostage.
func findingAffects(f lint.Finding, id, module string) bool {
	if f.LintName == lint.ModuleManifestLintName {
		return f.ClaimID == "" || f.ClaimID == module
	}
	return f.ClaimID == id || strings.Contains(f.Message, id)
}

func candidateLocked(id string, claims []model.Claim) bool {
	for _, claim := range claims {
		if claim.ID == id {
			return claim.Status == model.StatusLocked
		}
	}
	return false
}

func restCycleFrom(root, next string, claims map[string]model.Claim) bool {
	seen := map[string]bool{}
	var visit func(string) bool
	visit = func(id string) bool {
		if id == root {
			return true
		}
		if seen[id] {
			return false
		}
		seen[id] = true
		claim, ok := claims[id]
		if !ok {
			return false
		}
		for _, dep := range claim.RestsOn.IDs {
			if visit(dep) {
				return true
			}
		}
		return false
	}
	return visit(next)
}

func uniqueIDs(ids []string) []string {
	out := uniqueStrings(ids)
	sort.Strings(out)
	return out
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func (v CandidateVerdict) Error() error {
	if v.LocalAdmissible {
		return nil
	}
	return fmt.Errorf("lock: candidate %q refused: %s", v.ClaimID, strings.Join(v.Refusals, ", "))
}
