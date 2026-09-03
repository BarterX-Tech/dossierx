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

// CauseKind identifies one independent reason a claim requires review.
type CauseKind string

const (
	CauseDirectDependencyChange   CauseKind = "direct_dependency_change"
	CauseUpstreamDependencyReview CauseKind = "upstream_dependency_review"
	CauseOwnThread                CauseKind = "own_thread"
	CauseOwnFlag                  CauseKind = "own_flag"
	CauseApprovalContentDrift     CauseKind = "approval_content_drift"
	CauseApprovalMissing          CauseKind = "approval_missing"
	CauseApprovalReleased         CauseKind = "approval_released"
	CauseApprovalUnknown          CauseKind = "approval_unknown"
)

// ConditionKind identifies a condition that prevents the required dependency
// chain from being ready. Conditions are separate from review causes: an
// approved claim may consume a draft dependency, while still reporting that
// the dependency is not approved yet.
type ConditionKind string

const (
	ConditionDependencyUnapproved      ConditionKind = "dependency_unapproved"
	ConditionMissingDependency         ConditionKind = "missing_dependency"
	ConditionUnreadableDependency      ConditionKind = "unreadable_dependency"
	ConditionRetiredDependency         ConditionKind = "retired_dependency"
	ConditionUnknownHistoricalBaseline ConditionKind = "unknown_historical_baseline"
	ConditionDependencyCycle           ConditionKind = "dependency_cycle"
)

// Path is a causal path in the required rests_on graph. The first item is the
// claim whose assessment is being reported and the final item is the input
// that caused the condition or review cause.
type Path []string

// Cause is one independent review cause. Direct causes belong to the claim
// itself; inherited causes are emitted on a dependent claim and retain their
// complete path through every required prerequisite.
type Cause struct {
	Kind         CauseKind `json:"kind"`
	SourceKind   CauseKind `json:"source_kind,omitempty"`
	DependencyID string    `json:"dependency_id,omitempty"`
	Path         Path      `json:"path"`
	Detail       string    `json:"detail,omitempty"`
	Direct       bool      `json:"direct"`
	Inherited    bool      `json:"inherited"`
}

// DependencyCondition is a live readiness obstacle on a required dependency
// path. A governed_by edge is intentionally absent from this set: governance
// is a drift edge, not an approval prerequisite.
type DependencyCondition struct {
	Kind         ConditionKind `json:"kind"`
	DependencyID string        `json:"dependency_id,omitempty"`
	Path         Path          `json:"path"`
	Detail       string        `json:"detail,omitempty"`
}

// Assessment is the derived status of one claim. LocalApproved describes the
// claim's own status. DependencyReady describes only the required rests_on
// chain. Ready is true only when both are true and there are no active review
// causes. ReviewPending is the live OR of the independent causes and does not
// trust the claim's persisted ReviewPending field.
type Assessment struct {
	ClaimID       string             `json:"claim_id"`
	PolicyVersion lock.PolicyVersion `json:"policy_version"`
	LocalApproved bool               `json:"local_approved"`
	// LocallyApproved is an alias for callers that prefer the adjective form.
	// It is kept in sync with LocalApproved by Compute.
	LocallyApproved      bool                  `json:"locally_approved"`
	DependencyReady      bool                  `json:"dependency_ready"`
	Ready                bool                  `json:"ready"`
	ReviewPending        bool                  `json:"review_pending"`
	DependencyConditions []DependencyCondition `json:"dependency_conditions,omitempty"`
	// Conditions and Causes are concise aliases useful to generic consumers.
	Conditions         []DependencyCondition `json:"conditions,omitempty"`
	ReviewCauses       []Cause               `json:"review_causes,omitempty"`
	Causes             []Cause               `json:"causes,omitempty"`
	LocalReasons       []string              `json:"local_reasons,omitempty"`
	LocalApprovalIssue string                `json:"local_approval_issue,omitempty"`
}

type summary struct {
	conditions []DependencyCondition
	causes     []Cause
}

// Compute derives current readiness for every supplied claim. The returned
// map is keyed by claim id. Claims with duplicate IDs are assessed using the
// first declaration, matching the rest of DossierX's claim lookup behavior.
// The input slices, store, and flag store are never mutated.
func Compute(claims []model.Claim, store *lock.Store, flags *reaudit.FlagStore) map[string]Assessment {
	byID := make(map[string]model.Claim, len(claims))
	for _, c := range claims {
		if _, exists := byID[c.ID]; !exists {
			byID[c.ID] = c
		}
	}

	result := make(map[string]Assessment, len(byID))
	for id, c := range byID {
		local := localSummary(c, claims, store, flags, byID)
		collected := collect(c.ID, []string{c.ID}, map[string]bool{c.ID: true}, byID, local, claims, store, flags)
		conditions := sortConditions(collected.conditions)
		causes := sortCauses(collected.causes)
		approval := approvalState(c, store)
		localApproved := c.Status == model.StatusLocked && approval.valid
		reasons := localReasons(c)
		if c.Status == model.StatusLocked && !approval.valid {
			reasons = append(reasons, approval.detail)
		}
		assessment := Assessment{
			ClaimID:              id,
			PolicyVersion:        policyVersion(store),
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

func localReasons(c model.Claim) []string {
	if c.Status == model.StatusLocked {
		return nil
	}
	return []string{fmt.Sprintf("claim status %q is not locally approved", c.Status)}
}

type approvalCheck struct {
	valid  bool
	kind   CauseKind
	detail string
}

// approvalState checks the standing lock-ledger record rather than trusting a
// claim's mutable status field. LockedClaimHash includes every persisted claim
// field that the approval signs, while ContentHash intentionally answers the
// separate dependent-staleness question.
func approvalState(c model.Claim, store *lock.Store) approvalCheck {
	if c.Status != model.StatusLocked {
		return approvalCheck{detail: fmt.Sprintf("claim status %q is not locally approved", c.Status)}
	}
	if store == nil {
		return approvalCheck{kind: CauseApprovalUnknown, detail: "no lock store is available to establish a standing approval"}
	}
	record, ok := store.Record(c.ID)
	if !ok {
		return approvalCheck{kind: CauseApprovalMissing, detail: "no standing lock-ledger approval exists for this locked claim"}
	}
	if record.Subject != lock.SubjectClaim {
		return approvalCheck{kind: CauseApprovalUnknown, detail: "the lock-ledger record is not a claim approval"}
	}
	if record.Released() {
		return approvalCheck{kind: CauseApprovalReleased, detail: "the lock-ledger approval was released and is not standing"}
	}
	if record.Hash == "" || record.Hash != lock.LockedClaimHash(c) {
		return approvalCheck{kind: CauseApprovalContentDrift, detail: "current claim content differs from its standing lock approval"}
	}
	return approvalCheck{valid: true}
}

// localSummary computes only causes owned by id and conditions that are
// intrinsic to its own dependency evidence. collect adds transitive required
// chain information and converts child causes into inherited causes.
func localSummary(c model.Claim, claims []model.Claim, store *lock.Store, flags *reaudit.FlagStore, byID map[string]model.Claim) summary {
	var out summary
	if c.Status == model.StatusLocked {
		approval := approvalState(c, store)
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
			if contains(c.RestsOn, depID) {
				out.conditions = append(out.conditions, DependencyCondition{
					Kind: ConditionMissingDependency, DependencyID: depID,
					Path: Path{c.ID, depID}, Detail: "required dependency is missing",
				})
			}
			continue
		}
		if !contains(c.RestsOn, depID) {
			// mirrors and governed_by are comparable drift inputs, but neither
			// edge creates an approval prerequisite.
			if stored, known := baseline(store, c.ID, depID); known && stored != lock.ContentHash(dep) {
				out.causes = append(out.causes, Cause{
					Kind: CauseDirectDependencyChange, SourceKind: CauseDirectDependencyChange,
					DependencyID: depID, Path: Path{c.ID, depID}, Direct: true,
					Detail: "dependency content differs from the reviewed baseline",
				})
			}
			continue
		}
		state := dependencyState(dep)
		if state == ConditionRetiredDependency {
			out.conditions = append(out.conditions, DependencyCondition{Kind: state, DependencyID: depID, Path: Path{c.ID, depID}, Detail: "required dependency is retired"})
		} else if state == ConditionUnreadableDependency {
			out.conditions = append(out.conditions, DependencyCondition{Kind: state, DependencyID: depID, Path: Path{c.ID, depID}, Detail: "required dependency is unreadable"})
		}
		if state == "" {
			if stored, known := baseline(store, c.ID, depID); !known {
				out.conditions = append(out.conditions, DependencyCondition{
					Kind: ConditionUnknownHistoricalBaseline, DependencyID: depID,
					Path: Path{c.ID, depID}, Detail: "no historical content baseline is available",
				})
			} else if stored != lock.ContentHash(dep) {
				out.causes = append(out.causes, Cause{
					Kind: CauseDirectDependencyChange, SourceKind: CauseDirectDependencyChange,
					DependencyID: depID, Path: Path{c.ID, depID}, Direct: true,
					Detail: "dependency content differs from the reviewed baseline",
				})
			}
		}
	}
	return out
}

func baseline(store *lock.Store, dependent, dependency string) (string, bool) {
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

func policyVersion(store *lock.Store) lock.PolicyVersion {
	if store == nil {
		// LoadStore treats a missing store as a new project. This default also
		// keeps a read-only assessment useful before the first store is saved.
		return lock.PolicyLocalApprovalV1
	}
	return store.PolicyVersion
}

// collect walks only rests_on. Its path-relative summaries make each causal
// path independent, so B->A->C and B->D->C remain two visible causes. The
// active set cuts cycles before recursion; no malformed graph can recurse
// forever.
func collect(id string, path []string, active map[string]bool, byID map[string]model.Claim, out summary, claims []model.Claim, store *lock.Store, flags *reaudit.FlagStore) summary {
	c := byID[id]
	for _, depID := range unique(c.RestsOn) {
		dep, exists := byID[depID]
		if !exists {
			out.conditions = append(out.conditions, DependencyCondition{Kind: ConditionMissingDependency, DependencyID: depID, Path: appendPath(path[:1], depID), Detail: "required dependency is missing"})
			continue
		}
		state := dependencyState(dep)
		if state == ConditionRetiredDependency || state == ConditionUnreadableDependency {
			out.conditions = append(out.conditions, DependencyCondition{Kind: state, DependencyID: depID, Path: appendPath(path[:1], depID), Detail: "required dependency cannot be consumed"})
			continue
		}
		if dep.Status != model.StatusLocked {
			out.conditions = append(out.conditions, DependencyCondition{Kind: ConditionDependencyUnapproved, DependencyID: depID, Path: appendPath(path[:1], depID), Detail: "required dependency is not locally approved"})
		} else if approval := approvalState(dep, store); !approval.valid {
			out.conditions = append(out.conditions, DependencyCondition{
				Kind: ConditionDependencyUnapproved, DependencyID: depID,
				Path:   appendPath(path[:1], depID),
				Detail: "required dependency has no valid standing approval",
			})
		}
		if active[depID] {
			out.conditions = append(out.conditions, DependencyCondition{Kind: ConditionDependencyCycle, DependencyID: depID, Path: cyclePath(path, depID), Detail: "required dependency cycle"})
			continue
		}
		child := localSummary(dep, claims, store, flags, byID)
		child = collect(depID, append(path, depID), withActive(active, depID), byID, child, claims, store, flags)
		for _, condition := range child.conditions {
			condition.Path = appendPath(path[:1], condition.Path...)
			out.conditions = append(out.conditions, condition)
		}
		for _, cause := range child.causes {
			cause.Kind = CauseUpstreamDependencyReview
			if cause.SourceKind == "" {
				cause.SourceKind = cause.Kind
			}
			cause.Direct = false
			cause.Inherited = true
			cause.Path = appendPath(path[:1], cause.Path...)
			out.causes = append(out.causes, cause)
		}
	}
	return out
}

func dependencyState(c model.Claim) ConditionKind {
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

func withActive(active map[string]bool, id string) map[string]bool {
	next := make(map[string]bool, len(active)+1)
	for k, v := range active {
		next[k] = v
	}
	next[id] = true
	return next
}

func cyclePath(path []string, target string) Path {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == target {
			cycle := append([]string(nil), path[i+1:]...)
			cycle = append(cycle, target)
			return Path(cycle)
		}
	}
	return Path{target}
}

func appendPath(prefix []string, suffix ...string) Path {
	out := make(Path, 0, len(prefix)+len(suffix))
	out = append(out, prefix...)
	out = append(out, suffix...)
	return out
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func unique(values []string) []string {
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

func sortConditions(values []DependencyCondition) []DependencyCondition {
	values = dedupeConditions(values)
	sort.SliceStable(values, func(i, j int) bool {
		a, b := values[i], values[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.DependencyID != b.DependencyID {
			return a.DependencyID < b.DependencyID
		}
		if pathString(a.Path) != pathString(b.Path) {
			return pathString(a.Path) < pathString(b.Path)
		}
		return a.Detail < b.Detail
	})
	return values
}

func sortCauses(values []Cause) []Cause {
	values = dedupeCauses(values)
	sort.SliceStable(values, func(i, j int) bool {
		a, b := values[i], values[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.DependencyID != b.DependencyID {
			return a.DependencyID < b.DependencyID
		}
		if pathString(a.Path) != pathString(b.Path) {
			return pathString(a.Path) < pathString(b.Path)
		}
		return a.Detail < b.Detail
	})
	return values
}

func pathString(path Path) string { return strings.Join(path, "\x00") }

func dedupeConditions(values []DependencyCondition) []DependencyCondition {
	seen := map[string]bool{}
	out := make([]DependencyCondition, 0, len(values))
	for _, value := range values {
		key := fmt.Sprintf("%s\x00%s\x00%s\x00%s", value.Kind, value.DependencyID, pathString(value.Path), value.Detail)
		if seen[key] {
			continue
		}
		seen[key] = true
		value.Path = append(Path(nil), value.Path...)
		out = append(out, value)
	}
	return out
}

func dedupeCauses(values []Cause) []Cause {
	seen := map[string]bool{}
	out := make([]Cause, 0, len(values))
	for _, value := range values {
		key := fmt.Sprintf("%s\x00%s\x00%s\x00%s", value.Kind, value.DependencyID, pathString(value.Path), value.Detail)
		if seen[key] {
			continue
		}
		seen[key] = true
		value.Path = append(Path(nil), value.Path...)
		out = append(out, value)
	}
	return out
}
