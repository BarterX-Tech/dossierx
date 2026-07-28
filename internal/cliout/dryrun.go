package cliout

// DryRun is what every mutating DossierX verb returns under --dry-run: the
// whole answer to "if I ran this for real, what happens?", computed without
// touching a single byte on disk.
//
// It exists because the approval loop this release is built around requires the
// agent to show its work BEFORE asking the human for a yes. "I'm going to lock
// widget.contract.one" is not enough to approve; "I'm going to lock it, it
// passes lint, it has no open threads, and it will flip two dependents to
// review_pending" is. Each field answers one of the questions a reviewer
// actually asks:
//
//	Would          what the verb is
//	From/To        the state transition, when there is one
//	Preconditions  every gate that was evaluated, PASSING ONES INCLUDED — the
//	               passes are the evidence, and an agent that reports only
//	               failures is asking to be trusted rather than showing why
//	Missing        the names of the preconditions that did not hold, plus any
//	               required input that was not supplied
//	SideEffects    what ELSE changes — the dependents that get flagged, the
//	               files that get rewritten. The part a human cannot infer.
//	Proposed       the concrete values that would be written
//	Blocked        true if the real run would refuse
//
// Blocked true is NOT a command failure: the dry run did exactly what it was
// asked and answered "no". It exits 0 with ok:true, and the caller branches on
// data.blocked. A dry run that exited non-zero would be indistinguishable from
// a dry run that crashed.
//
// Relationship to reaudit's --confirm. They are different mechanisms and never
// collide, by rule: --dry-run NEVER writes, and --dry-run always wins. "reaudit
// <id>" is a preview (its historical behavior, unchanged), "reaudit <id>
// --confirm" applies, and "reaudit <id> --dry-run --confirm" previews what the
// apply WOULD do and applies nothing. So --confirm is reaudit's apply gate and
// --dry-run is the universal preview; passing both is legal and always safe.
type DryRun struct {
	Would         string         `json:"would"`
	From          string         `json:"from,omitempty"`
	To            string         `json:"to,omitempty"`
	Preconditions []Precondition `json:"preconditions"`
	SideEffects   []string       `json:"side_effects"`
	Missing       []string       `json:"missing"`
	Proposed      map[string]any `json:"proposed,omitempty"`
	Blocked       bool           `json:"blocked"`
}

// Precondition is one evaluated gate: a stable snake_case Name a skill can
// branch on, whether it held, and a human-readable Detail explaining the
// verdict (present on failures, and on passes where the value is interesting —
// "0 error-level findings").
type Precondition struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

// NewDryRun starts a dry-run report for the given "would" phrase. The three
// list fields are initialized empty rather than nil so they marshal as [] and
// never as null: a consumer should be able to range over them unconditionally.
func NewDryRun(would string) *DryRun {
	return &DryRun{
		Would:         would,
		Preconditions: []Precondition{},
		SideEffects:   []string{},
		Missing:       []string{},
	}
}

// Transition records the state change the verb performs.
func (d *DryRun) Transition(from, to string) *DryRun {
	d.From, d.To = from, to
	return d
}

// Require records one evaluated gate. A failing gate additionally lands in
// Missing and blocks the run — the two are kept in sync here rather than at
// each call site so a caller cannot report a failed precondition and forget to
// mark the verb blocked.
func (d *DryRun) Require(name string, ok bool, detail string) *DryRun {
	d.Preconditions = append(d.Preconditions, Precondition{Name: name, OK: ok, Detail: detail})
	if !ok {
		d.Missing = append(d.Missing, name)
		d.Blocked = true
	}
	return d
}

// Lacking records a required input that was not supplied at all (a missing
// --reason). Unlike Require this is not a gate that was evaluated against
// project state; it is the caller's invocation being incomplete.
func (d *DryRun) Lacking(what string) *DryRun {
	d.Missing = append(d.Missing, what)
	d.Blocked = true
	return d
}

// Effect records something the real run changes BEYOND its stated target — the
// blast radius. Callers should list these even when they are obvious to the
// implementer, because they are the part a reviewer cannot derive.
func (d *DryRun) Effect(effect string) *DryRun {
	d.SideEffects = append(d.SideEffects, effect)
	return d
}

// Propose records a concrete value the real run would write.
func (d *DryRun) Propose(key string, value any) *DryRun {
	if d.Proposed == nil {
		d.Proposed = map[string]any{}
	}
	d.Proposed[key] = value
	return d
}
