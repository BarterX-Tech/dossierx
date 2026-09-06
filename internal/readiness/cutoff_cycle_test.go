package readiness

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestAdversarialInvalidBoundaryCycles(t *testing.T) {
	for _, status := range []model.Status{"retired", "unreadable"} {
		for _, self := range []bool{true, false} {
			name := string(status) + "/two-node"
			if self {
				name = string(status) + "/self"
			}
			t.Run(name, func(t *testing.T) {
				x := lockedClaim("x", "b")
				b := lockedClaim("b", "c")
				c := lockedClaim("c", "b")
				b.Status = status
				if self {
					b.RestsOn = []string{"b"}
				}
				claims := []model.Claim{x, b, c}
				s := standingStore(claims...)
				recordBaseline(s, "x", b)
				old := oracleCompute(claims, s, nil)["x"]
				got := Compute(claims, s, nil)["x"]
				for _, v := range old.Conditions {
					if v.Kind == ConditionDependencyCycle {
						t.Fatal("baseline must stop before cycle")
					}
				}
				for _, v := range got.Conditions {
					if v.Kind == ConditionDependencyCycle {
						t.Fatalf("cycle escaped invalid-boundary cutoff: %+v; all conditions=%+v", v, got.Conditions)
					}
				}
			})
		}
	}
}

func TestAdversarialInvalidBridgeDoesNotMergeCycles(t *testing.T) {
	// After cutting at retired r, a and z are separate cyclic components.
	x := lockedClaim("x", "a", "z")
	a := lockedClaim("a", "a", "r")
	r := lockedClaim("r", "z")
	r.Status = "retired"
	z := lockedClaim("z", "z", "a")
	claims := []model.Claim{x, a, r, z}
	s := standingStore(claims...)
	for _, c := range claims {
		for _, d := range claims {
			for _, id := range c.RestsOn {
				if id == d.ID {
					recordBaseline(s, c.ID, d)
				}
			}
		}
	}
	got := Compute(claims, s, nil)["x"]
	cycles := 0
	for _, c := range got.Conditions {
		if c.Kind == ConditionDependencyCycle {
			cycles++
		}
	}
	if cycles != 2 {
		t.Fatalf("retired bridge must not merge two independently blocking readable cycles: want 2, got %d; %+v", cycles, got.Conditions)
	}
}

// An invalid root can inspect readable inputs, but cannot be entered as a
// prerequisite, even when the raw edges return to it. An alternate valid route
// must retain its cycle when the shortest raw route crosses an invalid node.
func TestInvalidRootAndAlternateCycleRoute(t *testing.T) {
	for _, status := range []model.Status{"retired", "unreadable", "unknown"} {
		t.Run(string(status), func(t *testing.T) {
			x := lockedClaim("x", "b", "d")
			b := lockedClaim("b", "c")
			b.Status = status
			d := lockedClaim("d", "c")
			c := lockedClaim("c", "b", "c")
			claims := []model.Claim{x, b, c, d}
			got := Compute(claims, standingStore(claims...), nil)
			for root, want := range map[string]Path{"x": {"x", "d", "c", "c"}, "b": {"b", "c", "c"}} {
				count := 0
				for _, cond := range got[root].Conditions {
					if cond.Kind == ConditionDependencyCycle {
						count++
						if !reflect.DeepEqual(cond.Path, want) {
							t.Fatalf("root=%s want=%v got=%v", root, want, cond.Path)
						}
					}
				}
				if count != 1 {
					t.Fatalf("root=%s want one readable cycle got=%d", root, count)
				}
				if got[root].DependencyReady || got[root].Ready {
					t.Fatalf("cycle must block root %s", root)
				}
			}
		})
	}
}

func TestInvalidRootRemainsABlockingPrerequisite(t *testing.T) {
	for _, status := range []model.Status{"retired", "unreadable"} {
		for _, self := range []bool{true, false} {
			t.Run(string(status)+fmt.Sprint(self), func(t *testing.T) {
				root := lockedClaim("root", "root")
				root.Status = status
				a := lockedClaim("a", "root")
				want := Path{"root", "root"}
				if !self {
					root.RestsOn = []string{"a"}
					want = Path{"root", "a", "root"}
				}
				claims := []model.Claim{root, a}
				store := standingStore(claims...)
				got := Compute(claims, store, nil)["root"]
				old := oracleCompute(claims, store, nil)["root"]
				if old.DependencyReady {
					t.Fatal("baseline must retain invalid root boundary")
				}
				if got.DependencyReady {
					t.Fatalf("invalid root consumed as its own prerequisite: %+v", got)
				}
				found := false
				for _, cond := range got.Conditions {
					if cond.Kind == ConditionDependencyCycle {
						t.Fatalf("cycle re-entered invalid root: %+v", cond)
					}
					if cond.DependencyID == "root" && (cond.Kind == ConditionRetiredDependency || cond.Kind == ConditionUnreadableDependency) {
						found = true
						if !reflect.DeepEqual(cond.Path, want) {
							t.Fatalf("want=%v got=%v", want, cond.Path)
						}
					}
				}
				if !found {
					t.Fatal("missing invalid root condition")
				}
			})
		}
	}
}
