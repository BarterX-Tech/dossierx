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
// that caused the condition or review cause. When multiple routes reach the
// same independent cause or condition, readiness emits a single deterministic
// representative path (the shortest path with a lexicographic tie-break)
// rather than enumerating every route.
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
//
// Graph traversal is bounded: readiness evaluates independent graph facts
// and emits a deterministic representative path (shortest path with a stable
// lexicographic tie-break) for each fact, strictly avoiding exponential path
// enumeration on dense dependency graphs.
func Compute(claims []model.Claim, store *lock.Store, flags *reaudit.FlagStore) map[string]Assessment {
	byID := make(map[string]model.Claim, len(claims))
	for _, c := range claims {
		if _, exists := byID[c.ID]; !exists {
			byID[c.ID] = c
		}
	}

	// Precompute strongly connected components (SCCs) to detect cycles.
	sccs := findSCCs(byID)
	sccNodesMap := make(map[string]map[string]bool)
	for _, scc := range sccs {
		isCyclic := len(scc) > 1 || (len(scc) == 1 && contains(byID[scc[0]].RestsOn, scc[0]))
		if isCyclic {
			sccID := scc[0] // canonical ID (scc is sorted)
			nodes := make(map[string]bool, len(scc))
			for _, node := range scc {
				nodes[node] = true
			}
			sccNodesMap[sccID] = nodes
		}
	}

	// Precompute local summaries and approval states for all claims once to avoid
	// repeated hashing and guarantee single-source-of-truth semantic consistency.
	localSummaries := make(map[string]summary, len(byID))
	approvalStates := make(map[string]approvalCheck, len(byID))
	for id, c := range byID {
		localSummaries[id] = localSummary(c, claims, store, flags, byID)
		approvalStates[id] = approvalState(c, store)
	}

	cycleCache := make(map[string][]string)

	result := make(map[string]Assessment, len(byID))
	for id, c := range byID {
		approval := approvalStates[id]
		localApproved := c.Status == model.StatusLocked && approval.valid
		reasons := localReasons(c)
		if c.Status == model.StatusLocked && !approval.valid {
			reasons = append(reasons, approval.detail)
		}

		local := localSummaries[id]

		// BFS to find shortest representative paths to all reachable nodes in rests_on.
		dist := map[string]int{id: 0}
		paths := map[string][]string{id: {id}}
		currentLevel := []string{id}

		bestMissing := make(map[string][]string)

		for len(currentLevel) > 0 {
			var nextLevel []string
			nextLevelSet := make(map[string]bool)
			bestParent := make(map[string]string)

			for _, uID := range currentLevel {
				uClaim := byID[uID]
				uPath := paths[uID]
				uDeps := unique(uClaim.RestsOn)
				sort.Strings(uDeps)

				for _, depID := range uDeps {
					_, exists := byID[depID]
					if !exists {
						candLen := len(uPath) + 1
						if existing, seen := bestMissing[depID]; !seen || candLen < len(existing) || (candLen == len(existing) && pathLess(uPath, existing[:len(existing)-1])) {
							bestMissing[depID] = appendPath(uPath, depID)
						}
						continue
					}

					if _, seen := dist[depID]; seen {
						continue
					}

					if prevParent, has := bestParent[depID]; !has || pathLess(paths[uID], paths[prevParent]) {
						bestParent[depID] = uID
					}
					if !nextLevelSet[depID] {
						nextLevelSet[depID] = true
						nextLevel = append(nextLevel, depID)
					}
				}
			}

			for depID, pID := range bestParent {
				dist[depID] = dist[pID] + 1
				paths[depID] = appendPath(paths[pID], depID)
			}

			currentLevel = nextLevel
		}

		// Assemble deduplicated condition and cause records
		conditionMap := make(map[string]DependencyCondition)
		causeMap := make(map[string]Cause)

		// 1. Direct causes and conditions on c itself
		for _, cause := range local.causes {
			key := fmt.Sprintf("%s\x00%s\x00%s", cause.SourceKind, id, cause.DependencyID)
			causeMap[key] = cause
		}
		for _, condition := range local.conditions {
			var key string
			if condition.Kind == ConditionUnknownHistoricalBaseline {
				key = fmt.Sprintf("%s\x00%s\x00%s", condition.Kind, id, condition.DependencyID)
			} else {
				key = fmt.Sprintf("%s\x00%s", condition.Kind, condition.DependencyID)
			}
			conditionMap[key] = condition
		}

		// 2. Transitive facts from reachable nodes in rests_on
		for uID, p := range paths {
			if uID == id {
				continue
			}
			dep := byID[uID]

			// Unapproved dependency
			if dep.Status != model.StatusLocked {
				key := fmt.Sprintf("%s\x00%s", ConditionDependencyUnapproved, uID)
				if _, exists := conditionMap[key]; !exists {
					conditionMap[key] = DependencyCondition{
						Kind: ConditionDependencyUnapproved, DependencyID: uID,
						Path: appendPath(p), Detail: "required dependency is not locally approved",
					}
				}
			} else if app := approvalStates[uID]; !app.valid {
				key := fmt.Sprintf("%s\x00%s", ConditionDependencyUnapproved, uID)
				if _, exists := conditionMap[key]; !exists {
					conditionMap[key] = DependencyCondition{
						Kind: ConditionDependencyUnapproved, DependencyID: uID,
						Path: appendPath(p), Detail: "required dependency has no valid standing approval",
					}
				}
			}

			// Retired / unreadable dependency
			state := dependencyState(dep)
			if state == ConditionRetiredDependency || state == ConditionUnreadableDependency {
				key := fmt.Sprintf("%s\x00%s", state, uID)
				if _, exists := conditionMap[key]; !exists {
					detail := "required dependency cannot be consumed"
					if len(p) == 2 {
						if state == ConditionRetiredDependency {
							detail = "required dependency is retired"
						} else {
							detail = "required dependency is unreadable"
						}
					}
					conditionMap[key] = DependencyCondition{
						Kind: state, DependencyID: uID,
						Path: appendPath(p), Detail: detail,
					}
				}
			}

			// Upstream review causes from dep: inherit directly from dep's localSummary
			// to guarantee single-source-of-truth consistency.
			uSummary := localSummaries[uID]
			for _, cause := range uSummary.causes {
				sourceKind := cause.SourceKind
				if sourceKind == "" {
					sourceKind = cause.Kind
				}
				key := fmt.Sprintf("%s\x00%s\x00%s", sourceKind, uID, cause.DependencyID)
				if _, exists := causeMap[key]; !exists {
					var inheritedPath Path
					if len(cause.Path) > 1 {
						inheritedPath = appendPath(p, cause.Path[1:]...)
					} else {
						inheritedPath = appendPath(p)
					}
					causeMap[key] = Cause{
						Kind:         CauseUpstreamDependencyReview,
						SourceKind:   sourceKind,
						DependencyID: cause.DependencyID,
						Path:         inheritedPath,
						Detail:       cause.Detail,
						Direct:       false,
						Inherited:    true,
					}
				}
			}

			// Direct edge conditions intrinsic to dep (e.g. unknown historical baselines)
			for _, condition := range uSummary.conditions {
				if condition.Kind == ConditionUnknownHistoricalBaseline {
					key := fmt.Sprintf("%s\x00%s\x00%s", condition.Kind, uID, condition.DependencyID)
					if _, exists := conditionMap[key]; !exists {
						var inheritedPath Path
						if len(condition.Path) > 1 {
							inheritedPath = appendPath(p, condition.Path[1:]...)
						} else {
							inheritedPath = appendPath(p)
						}
						conditionMap[key] = DependencyCondition{
							Kind:         condition.Kind,
							DependencyID: condition.DependencyID,
							Path:         inheritedPath,
							Detail:       condition.Detail,
						}
					}
				}
			}
		}

		// 3. Missing dependencies
		for mID, candPath := range bestMissing {
			key := fmt.Sprintf("%s\x00%s", ConditionMissingDependency, mID)
			if _, exists := conditionMap[key]; !exists {
				conditionMap[key] = DependencyCondition{
					Kind: ConditionMissingDependency, DependencyID: mID,
					Path: appendPath(candPath), Detail: "required dependency is missing",
				}
			}
		}

		// 4. Reachable cyclic SCCs: globally minimize the complete witness path
		// len(prefix + cycle) across all reachable entries in the SCC.
		for _, scc := range sccs {
			isCyclic := len(scc) > 1 || (len(scc) == 1 && contains(byID[scc[0]].RestsOn, scc[0]))
			if !isCyclic {
				continue
			}
			sccID := scc[0]
			var bestWitness Path
			var bestEntry string

			for _, entry := range scc {
				p, ok := paths[entry]
				if !ok {
					continue
				}
				cycleK, cached := cycleCache[entry]
				if !cached {
					cycleK = getShortestCycle(entry, sccNodesMap[sccID], byID)
					cycleCache[entry] = cycleK
				}
				var candWitness Path
				if entry == id {
					candWitness = appendPath(cycleK)
				} else {
					candWitness = appendPath(p)
					if len(cycleK) > 1 {
						candWitness = append(candWitness, cycleK[1:]...)
					}
				}
				if bestWitness == nil || len(candWitness) < len(bestWitness) || (len(candWitness) == len(bestWitness) && pathLess(candWitness, bestWitness)) {
					bestWitness = candWitness
					bestEntry = entry
				}
			}

			if bestWitness != nil {
				key := fmt.Sprintf("%s\x00%s", ConditionDependencyCycle, sccID)
				if _, exists := conditionMap[key]; !exists {
					conditionMap[key] = DependencyCondition{
						Kind:         ConditionDependencyCycle,
						DependencyID: bestEntry,
						Path:         bestWitness,
						Detail:       "required dependency cycle",
					}
				}
			}
		}

		rawConditions := make([]DependencyCondition, 0, len(conditionMap))
		for _, cond := range conditionMap {
			rawConditions = append(rawConditions, cond)
		}
		rawCauses := make([]Cause, 0, len(causeMap))
		for _, cause := range causeMap {
			rawCauses = append(rawCauses, cause)
		}

		conditions := sortConditions(rawConditions)
		causes := sortCauses(rawCauses)

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

// pathLess defines the stable deterministic tie-break for equal-length paths:
// element-by-element lexicographical comparison, with shorter paths ranking
// before longer paths.
func pathLess(a, b []string) bool {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

type sccState struct {
	index   int
	indices map[string]int
	lowlink map[string]int
	onStack map[string]bool
	stack   []string
	sccs    [][]string
}

// findSCCs computes strongly connected components in the rests_on graph using
// Tarjan's algorithm. Node ordering is deterministic.
func findSCCs(byID map[string]model.Claim) [][]string {
	state := &sccState{
		indices: make(map[string]int, len(byID)),
		lowlink: make(map[string]int, len(byID)),
		onStack: make(map[string]bool, len(byID)),
	}
	nodes := make([]string, 0, len(byID))
	for id := range byID {
		nodes = append(nodes, id)
	}
	sort.Strings(nodes)

	var strongconnect func(string)
	strongconnect = func(v string) {
		state.indices[v] = state.index
		state.lowlink[v] = state.index
		state.index++
		state.stack = append(state.stack, v)
		state.onStack[v] = true

		c := byID[v]
		deps := unique(c.RestsOn)
		sort.Strings(deps)

		for _, w := range deps {
			if _, exists := byID[w]; !exists {
				continue
			}
			if _, seen := state.indices[w]; !seen {
				strongconnect(w)
				if state.lowlink[w] < state.lowlink[v] {
					state.lowlink[v] = state.lowlink[w]
				}
			} else if state.onStack[w] {
				if state.indices[w] < state.lowlink[v] {
					state.lowlink[v] = state.indices[w]
				}
			}
		}

		if state.lowlink[v] == state.indices[v] {
			var scc []string
			for {
				w := state.stack[len(state.stack)-1]
				state.stack = state.stack[:len(state.stack)-1]
				state.onStack[w] = false
				scc = append(scc, w)
				if w == v {
					break
				}
			}
			sort.Strings(scc)
			state.sccs = append(state.sccs, scc)
		}
	}

	for _, v := range nodes {
		if _, seen := state.indices[v]; !seen {
			strongconnect(v)
		}
	}
	return state.sccs
}

// getShortestCycle finds the shortest valid directed cycle starting and ending
// at start using only edges within sccNodes. Every hop is a verified real edge
// in rests_on, ensuring the emitted cycle witness is valid and closed.
func getShortestCycle(start string, sccNodes map[string]bool, byID map[string]model.Claim) []string {
	startClaim, ok := byID[start]
	if !ok {
		return []string{start, start}
	}
	for _, depID := range unique(startClaim.RestsOn) {
		if depID == start {
			return []string{start, start}
		}
	}

	type queueItem struct {
		node string
		path []string
	}
	var queue []queueItem
	visited := map[string]int{start: 0}

	startNeighbors := unique(startClaim.RestsOn)
	sort.Strings(startNeighbors)

	for _, depID := range startNeighbors {
		if sccNodes[depID] {
			visited[depID] = 1
			queue = append(queue, queueItem{node: depID, path: []string{start, depID}})
		}
	}

	var bestCycle []string
	foundLen := -1

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if foundLen != -1 && len(curr.path) >= foundLen {
			break
		}

		c, exists := byID[curr.node]
		if !exists {
			continue
		}

		cNeighbors := unique(c.RestsOn)
		sort.Strings(cNeighbors)

		for _, nextID := range cNeighbors {
			if !sccNodes[nextID] {
				continue
			}
			if nextID == start {
				cand := append(append([]string(nil), curr.path...), start)
				if bestCycle == nil || len(cand) < len(bestCycle) || (len(cand) == len(bestCycle) && pathLess(cand, bestCycle)) {
					bestCycle = cand
					foundLen = len(cand)
				}
				continue
			}
			if foundLen != -1 {
				continue
			}
			if _, seen := visited[nextID]; !seen {
				visited[nextID] = len(curr.path)
				candPath := append(append([]string(nil), curr.path...), nextID)
				queue = append(queue, queueItem{node: nextID, path: candPath})
			}
		}
	}

	if bestCycle != nil {
		return bestCycle
	}
	return []string{start, start}
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
		if a.SourceKind != b.SourceKind {
			return a.SourceKind < b.SourceKind
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
		key := fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s", value.Kind, value.SourceKind, value.DependencyID, pathString(value.Path), value.Detail)
		if seen[key] {
			continue
		}
		seen[key] = true
		value.Path = append(Path(nil), value.Path...)
		out = append(out, value)
	}
	return out
}
