// Package readiness derives the difference between a claim's local approval
// and the readiness of the required chain it consumes.
//
// This package is deliberately a read-only calculation. It does not alter a
// claim's persisted review_pending bit, baselines, or flags. In particular,
// a saved review_pending bit is not evidence of a current cause: causes are
// recomputed from the claims, the current content hashes, and the flag store.
package readiness

import (
	"fmt"
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/reaudit"
)

func oracleCompute(claims []model.Claim, store *lock.Store, flags *reaudit.FlagStore) map[string]Assessment {
	byID := make(map[string]model.Claim, len(claims))
	for _, c := range claims {
		if _, exists := byID[c.ID]; !exists {
			byID[c.ID] = c
		}
	}

	result := make(map[string]Assessment, len(byID))
	for id, c := range byID {
		local := oraclelocalSummary(c, claims, store, flags, byID)
		collected := oraclecollect(c.ID, []string{c.ID}, map[string]bool{c.ID: true}, byID, local, claims, store, flags)
		conditions := oraclesortConditions(collected.conditions)
		causes := oraclesortCauses(collected.causes)
		approval := oracleapprovalState(c, store)
		localApproved := c.Status == model.StatusLocked && approval.valid
		reasons := oraclelocalReasons(c)
		if c.Status == model.StatusLocked && !approval.valid {
			reasons = append(reasons, approval.detail)
		}
		assessment := Assessment{
			ClaimID:              id,
			PolicyVersion:        oraclepolicyVersion(store),
			LocalApproved:        localApproved,
			LocallyApproved:      localApproved,
			DependencyReady:      len(conditions) == 0,
			ReviewPending:        len(causes) > 0,
			DependencyConditions: conditions,
			Conditions:           append([]DependencyCondition(nil), conditions...),
			ReviewCauses:         causes,
			Causes:               append([]Cause(nil), causes...),
			LocalReasons:         reasons,
			LocalApprovalIssue:   approval.detail,
		}
		assessment.Ready = assessment.LocalApproved && assessment.DependencyReady && !assessment.ReviewPending
		result[id] = assessment
	}
	return result
}

func oraclelocalReasons(c model.Claim) []string {
	if c.Status == model.StatusLocked {
		return nil
	}
	return []string{fmt.Sprintf("claim status %q is not locally approved", c.Status)}
}

type oracleapprovalCheck struct {
	valid  bool
	kind   CauseKind
	detail string
}

// oracleapprovalState checks the standing lock-ledger record rather than trusting a
// claim's mutable status field. LockedClaimHash includes every persisted claim
// field that the approval signs, while ContentHash intentionally answers the
// separate dependent-staleness question.
func oracleapprovalState(c model.Claim, store *lock.Store) oracleapprovalCheck {
	if c.Status != model.StatusLocked {
		return oracleapprovalCheck{detail: fmt.Sprintf("claim status %q is not locally approved", c.Status)}
	}
	if store == nil {
		return oracleapprovalCheck{kind: CauseApprovalUnknown, detail: "no lock store is available to establish a standing approval"}
	}
	record, ok := store.Record(c.ID)
	if !ok {
		return oracleapprovalCheck{kind: CauseApprovalMissing, detail: "no standing lock-ledger approval exists for this locked claim"}
	}
	if record.Subject != lock.SubjectClaim {
		return oracleapprovalCheck{kind: CauseApprovalUnknown, detail: "the lock-ledger record is not a claim approval"}
	}
	if record.Released() {
		return oracleapprovalCheck{kind: CauseApprovalReleased, detail: "the lock-ledger approval was released and is not standing"}
	}
	if record.Hash == "" || record.Hash != lock.LockedClaimHash(c) {
		return oracleapprovalCheck{kind: CauseApprovalContentDrift, detail: "current claim content differs from its standing lock approval"}
	}
	return oracleapprovalCheck{valid: true}
}

// oraclelocalSummary computes only causes owned by id and conditions that are
// intrinsic to its own dependency evidence. oraclecollect adds transitive required
// chain information and converts child causes into inherited causes.
func oraclelocalSummary(c model.Claim, claims []model.Claim, store *lock.Store, flags *reaudit.FlagStore, byID map[string]model.Claim) summary {
	var out summary
	if c.Status == model.StatusLocked {
		approval := oracleapprovalState(c, store)
		if !approval.valid {
			out.causes = append(out.causes, Cause{
				Kind: approval.kind, SourceKind: approval.kind, Path: Path{c.ID}, Direct: true,
				Detail: approval.detail,
			})
		}
	}
	if c.HasOpenThreads() {
		out.causes = append(out.causes, Cause{
			Kind: CauseOwnThread, SourceKind: CauseOwnThread, Path: Path{c.ID}, Direct: true,
			Detail: strings.Join(c.OpenThreadIDs(), ","),
		})
	}
	if flags != nil {
		if flag, ok := flags.Flags[c.ID]; ok {
			out.causes = append(out.causes, Cause{
				Kind: CauseOwnFlag, SourceKind: CauseOwnFlag, Path: Path{c.ID}, Direct: true,
				Detail: flag.Reason,
			})
		}
	}
	if c.Status != model.StatusLocked {
		return out
	}
	for _, depID := range lock.BaselineDependencyIDs(c) {
		dep, exists := byID[depID]
		if !exists {
			// A missing governed_by or mirrors input is still reported by the
			// relevant integrity/lint gate; it is deliberately not turned into
			// an approval prerequisite here. rests_on is the required chain.
			if oraclecontains(c.RestsOn, depID) {
				out.conditions = append(out.conditions, DependencyCondition{
					Kind: ConditionMissingDependency, DependencyID: depID,
					Path: Path{c.ID, depID}, Detail: "required dependency is missing",
				})
			}
			continue
		}
		if !oraclecontains(c.RestsOn, depID) {
			// mirrors and governed_by are comparable drift inputs, but neither
			// edge creates an approval prerequisite.
			if stored, known := oraclebaseline(store, c.ID, depID); known && stored != lock.ContentHash(dep) {
				out.causes = append(out.causes, Cause{
					Kind: CauseDirectDependencyChange, SourceKind: CauseDirectDependencyChange,
					DependencyID: depID, Path: Path{c.ID, depID}, Direct: true,
					Detail: "dependency content differs from the reviewed oraclebaseline",
				})
			}
			continue
		}
		state := oracledependencyState(dep)
		if state == ConditionRetiredDependency {
			out.conditions = append(out.conditions, DependencyCondition{Kind: state, DependencyID: depID, Path: Path{c.ID, depID}, Detail: "required dependency is retired"})
		} else if state == ConditionUnreadableDependency {
			out.conditions = append(out.conditions, DependencyCondition{Kind: state, DependencyID: depID, Path: Path{c.ID, depID}, Detail: "required dependency is unreadable"})
		}
		if state == "" {
			if stored, known := oraclebaseline(store, c.ID, depID); !known {
				out.conditions = append(out.conditions, DependencyCondition{
					Kind: ConditionUnknownHistoricalBaseline, DependencyID: depID,
					Path: Path{c.ID, depID}, Detail: "no historical content oraclebaseline is available",
				})
			} else if stored != lock.ContentHash(dep) {
				out.causes = append(out.causes, Cause{
					Kind: CauseDirectDependencyChange, SourceKind: CauseDirectDependencyChange,
					DependencyID: depID, Path: Path{c.ID, depID}, Direct: true,
					Detail: "dependency content differs from the reviewed oraclebaseline",
				})
			}
		}
	}
	return out
}

func oraclebaseline(store *lock.Store, dependent, dependency string) (string, bool) {
	if store == nil {
		return "", false
	}
	if hash, ok := store.Baseline(dependent, dependency); ok {
		return hash, true
	}
	if receipt, ok := store.Receipt(dependent, dependency); ok && receipt.Hash != "" {
		return receipt.Hash, true
	}
	return "", false
}

func oraclepolicyVersion(store *lock.Store) lock.PolicyVersion {
	if store == nil {
		// LoadStore treats a missing store as a new project. This default also
		// keeps a read-only assessment useful before the first store is saved.
		return lock.PolicyLocalApprovalV1
	}
	return store.PolicyVersion
}

// oraclecollect walks only rests_on. Its path-relative summaries make each causal
// path independent, so B->A->C and B->D->C remain two visible causes. The
// active set cuts cycles before recursion; no malformed graph can recurse
// forever.
func oraclecollect(id string, path []string, active map[string]bool, byID map[string]model.Claim, out summary, claims []model.Claim, store *lock.Store, flags *reaudit.FlagStore) summary {
	c := byID[id]
	for _, depID := range oracleunique(c.RestsOn) {
		dep, exists := byID[depID]
		if !exists {
			out.conditions = append(out.conditions, DependencyCondition{Kind: ConditionMissingDependency, DependencyID: depID, Path: oracleappendPath(oraclecurrentNode(path), depID), Detail: "required dependency is missing"})
			continue
		}
		state := oracledependencyState(dep)
		if state == ConditionRetiredDependency || state == ConditionUnreadableDependency {
			out.conditions = append(out.conditions, DependencyCondition{Kind: state, DependencyID: depID, Path: oracleappendPath(oraclecurrentNode(path), depID), Detail: "required dependency cannot be consumed"})
			continue
		}
		if dep.Status != model.StatusLocked {
			out.conditions = append(out.conditions, DependencyCondition{Kind: ConditionDependencyUnapproved, DependencyID: depID, Path: oracleappendPath(oraclecurrentNode(path), depID), Detail: "required dependency is not locally approved"})
		} else if approval := oracleapprovalState(dep, store); !approval.valid {
			out.conditions = append(out.conditions, DependencyCondition{
				Kind: ConditionDependencyUnapproved, DependencyID: depID,
				Path:   oracleappendPath(oraclecurrentNode(path), depID),
				Detail: "required dependency has no valid standing approval",
			})
		}
		if active[depID] {
			out.conditions = append(out.conditions, DependencyCondition{Kind: ConditionDependencyCycle, DependencyID: depID, Path: oraclecyclePath(path, depID), Detail: "required dependency cycle"})
			continue
		}
		child := oraclelocalSummary(dep, claims, store, flags, byID)
		child = oraclecollect(depID, append(path, depID), oraclewithActive(active, depID), byID, child, claims, store, flags)
		for _, condition := range child.conditions {
			condition.Path = oracleappendPath(oraclecurrentNode(path), condition.Path...)
			out.conditions = append(out.conditions, condition)
		}
		for _, cause := range child.causes {
			cause.Kind = CauseUpstreamDependencyReview
			if cause.SourceKind == "" {
				cause.SourceKind = cause.Kind
			}
			cause.Direct = false
			cause.Inherited = true
			cause.Path = oracleappendPath(oraclecurrentNode(path), cause.Path...)
			out.causes = append(out.causes, cause)
		}
	}
	return out
}

func oracledependencyState(c model.Claim) ConditionKind {
	switch strings.ToLower(strings.TrimSpace(string(c.Status))) {
	case "retired":
		return ConditionRetiredDependency
	case "draft", "locked", "":
		return ""
	default:
		// Claim.Status has only draft/locked in the current schema. An
		// unrecognised lifecycle value is unsafe to consume and is treated as
		// unreadable until an explicit migration or repair resolves it.
		return ConditionUnreadableDependency
	}
}

func oraclewithActive(active map[string]bool, id string) map[string]bool {
	next := make(map[string]bool, len(active)+1)
	for k, v := range active {
		next[k] = v
	}
	next[id] = true
	return next
}

func oraclecurrentNode(path []string) []string {
	if len(path) == 0 {
		return nil
	}
	return []string{path[len(path)-1]}
}

func oraclecyclePath(path []string, target string) Path {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == target {
			cycle := append([]string(nil), path[i+1:]...)
			cycle = append(cycle, target)
			if len(cycle) == 1 {
				cycle = append(cycle, target)
			}
			return Path(cycle)
		}
	}
	return Path{target}
}

func oracleappendPath(prefix []string, suffix ...string) Path {
	out := make(Path, 0, len(prefix)+len(suffix))
	out = append(out, prefix...)
	out = append(out, suffix...)
	return out
}

func oraclecontains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func oracleunique(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func oraclesortConditions(values []DependencyCondition) []DependencyCondition {
	values = oraclededupeConditions(values)
	sort.SliceStable(values, func(i, j int) bool {
		a, b := values[i], values[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.DependencyID != b.DependencyID {
			return a.DependencyID < b.DependencyID
		}
		if oraclepathString(a.Path) != oraclepathString(b.Path) {
			return oraclepathString(a.Path) < oraclepathString(b.Path)
		}
		return a.Detail < b.Detail
	})
	return values
}

func oraclesortCauses(values []Cause) []Cause {
	values = oraclededupeCauses(values)
	sort.SliceStable(values, func(i, j int) bool {
		a, b := values[i], values[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.DependencyID != b.DependencyID {
			return a.DependencyID < b.DependencyID
		}
		if oraclepathString(a.Path) != oraclepathString(b.Path) {
			return oraclepathString(a.Path) < oraclepathString(b.Path)
		}
		return a.Detail < b.Detail
	})
	return values
}

func oraclepathString(path Path) string { return strings.Join(path, "\x00") }

func oraclededupeConditions(values []DependencyCondition) []DependencyCondition {
	seen := map[string]bool{}
	out := make([]DependencyCondition, 0, len(values))
	for _, value := range values {
		key := fmt.Sprintf("%s\x00%s\x00%s\x00%s", value.Kind, value.DependencyID, oraclepathString(value.Path), value.Detail)
		if seen[key] {
			continue
		}
		seen[key] = true
		value.Path = append(Path(nil), value.Path...)
		out = append(out, value)
	}
	return out
}

func oraclededupeCauses(values []Cause) []Cause {
	seen := map[string]bool{}
	out := make([]Cause, 0, len(values))
	for _, value := range values {
		key := fmt.Sprintf("%s\x00%s\x00%s\x00%s", value.Kind, value.DependencyID, oraclepathString(value.Path), value.Detail)
		if seen[key] {
			continue
		}
		seen[key] = true
		value.Path = append(Path(nil), value.Path...)
		out = append(out, value)
	}
	return out
}
