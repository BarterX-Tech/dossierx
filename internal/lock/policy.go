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
			if findingAffects(finding, id) {
				verdict.LocalAdmissible = false
				verdict.Refusals = append(verdict.Refusals, "lint:"+finding.LintName)
			}
		}
		for _, depID := range claim.RestsOn {
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
			if cfg != nil && cfg.HubGatingEnabled() && dep.Facet == cfg.DoctrineFacet && !candidateLocked(depID, candidate) {
				verdict.LocalAdmissible = false
				verdict.Refusals = append(verdict.Refusals, "doctrine_dependency_not_locked:"+depID)
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
			if findingAffects(finding, id) {
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

func findingAffects(f lint.Finding, id string) bool {
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
		for _, dep := range claim.RestsOn {
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
