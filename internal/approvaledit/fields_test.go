package approvaledit

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// editedPair builds a released approval over `approved` with `current` on
// disk, and returns the Change the panel would draw.
func editedPair(t *testing.T, approved, current model.Claim) Change {
	t.Helper()
	current.Status = model.StatusDraft
	approved.Status = model.StatusLocked
	store := &lock.Store{}
	lock.RecordApproval(store, approved, lock.Approval{Actor: "approver", Reason: "reviewed"})
	lock.ReleaseApproval(store, approved.ID, lock.Approval{Actor: "maintainer", Reason: "rewording"})
	changes := Compute([]model.Claim{current}, store)
	change, ok := changes[current.ID]
	if !ok {
		t.Fatal("the claim must be reported as edited since its approval")
	}
	return change
}

// The case the whole field diff exists for: a claim whose PROSE never moved
// and whose structure did. Before this, the panel said "Also changed:
// rests_on." and showed nothing — naming a change the reader could not see.
func TestFieldChangesShowWhatMovedWhenTheBodyDidNot(t *testing.T) {
	approved := model.Claim{
		ID: "widget.contract.a", Facet: "contract", Body: "Unchanged prose.\n",
		RestsOn: model.RestsOnIDs("widget.contract.old"),
	}
	current := approved
	current.RestsOn = model.RestsOnIDs("widget.contract.new")

	change := editedPair(t, approved, current)
	if change.BodyChanged() {
		t.Fatal("this fixture must not move the body")
	}
	if len(change.FieldChanges) != 1 || change.FieldChanges[0].Field != "rests_on" {
		t.Fatalf("want one rests_on field change, got %+v", change.FieldChanges)
	}
	field := change.FieldChanges[0]

	var removed, added string
	for _, h := range field.Hunks {
		switch h.Op {
		case "remove":
			removed += h.HTML
		case "add":
			added += h.HTML
		}
	}
	if !strings.Contains(removed, "widget.contract.old") {
		t.Fatalf("the removed side must carry the approved value, got %q", removed)
	}
	if !strings.Contains(added, "widget.contract.new") {
		t.Fatalf("the added side must carry the current value, got %q", added)
	}
	// And the words that moved are marked, which is the difference between
	// showing two blobs and showing a diff.
	if !strings.Contains(removed, "claim-edit-word--removed") || !strings.Contains(added, "claim-edit-word--added") {
		t.Fatalf("the changed words must be marked:\n removed %q\n added %q", removed, added)
	}
}

// OtherFields and FieldChanges must describe the same set in the same order.
// A reader is told "2 fields moved" from one and shown rows from the other;
// if they could disagree the panel would contradict itself.
func TestFieldChangesAgreeWithOtherFields(t *testing.T) {
	approved := model.Claim{
		ID: "widget.contract.b", Facet: "contract", Body: "Unchanged.\n",
		RestsOn: model.RestsOnIDs("widget.contract.x"), Steps: []string{"one"},
		AuditNotes: []string{"first note"},
	}
	current := approved
	current.RestsOn = model.RestsOnIDs("widget.contract.y")
	current.Steps = []string{"one", "two"}
	current.AuditNotes = []string{"first note", "second note"}

	change := editedPair(t, approved, current)
	if len(change.OtherFields) != len(change.FieldChanges) {
		t.Fatalf("other_fields %v and field_changes %d describe different sets",
			change.OtherFields, len(change.FieldChanges))
	}
	for i, name := range change.OtherFields {
		if change.FieldChanges[i].Field != name {
			t.Fatalf("field %d: other_fields says %q, field_changes says %q",
				i, name, change.FieldChanges[i].Field)
		}
	}
	if len(change.OtherFields) != 3 {
		t.Fatalf("want rests_on, steps and audit_notes, got %v", change.OtherFields)
	}
}

// A field is YAML, not markdown, and it reaches the browser through
// innerHTML. Anything angle-bracketed in a claim file must arrive as TEXT.
func TestFieldChangesEscapeMarkupInFieldValues(t *testing.T) {
	approved := model.Claim{
		ID: "widget.contract.c", Facet: "contract", Body: "Unchanged.\n",
		AuditNotes: []string{"before <script>alert(1)</script>"},
	}
	current := approved
	current.AuditNotes = []string{"after <script>alert(2)</script>"}

	change := editedPair(t, approved, current)
	if len(change.FieldChanges) != 1 {
		t.Fatalf("want one field change, got %+v", change.FieldChanges)
	}
	for _, h := range change.FieldChanges[0].Hunks {
		if strings.Contains(h.HTML, "<script>") {
			t.Fatalf("a field value must be escaped before it reaches innerHTML, got %q", h.HTML)
		}
		if !strings.Contains(h.HTML, "&lt;script&gt;") {
			t.Fatalf("the escaped form must still be readable as text, got %q", h.HTML)
		}
		// The only tags allowed are the mark spans this package writes.
		for _, tag := range tagsIn(h.HTML) {
			if tag != "span" && tag != "/span" {
				t.Fatalf("a field hunk may contain only mark spans, found <%s> in %q", tag, h.HTML)
			}
		}
	}
}

// Markdown syntax inside a field must stay literal — a field showing `*` is
// showing what the file holds, and rendering it as emphasis would make the
// panel disagree with the file it is quoting.
func TestFieldChangesDoNotRenderMarkdown(t *testing.T) {
	approved := model.Claim{
		ID: "widget.contract.d", Facet: "contract", Body: "Unchanged.\n",
		AuditNotes: []string{"a **bold** claim about one"},
	}
	current := approved
	current.AuditNotes = []string{"a **bold** claim about two"}

	change := editedPair(t, approved, current)
	joined := ""
	for _, h := range change.FieldChanges[0].Hunks {
		joined += h.HTML
	}
	if strings.Contains(joined, "<strong>") || strings.Contains(joined, "<em>") {
		t.Fatalf("field YAML must not be run through the markdown renderer, got %q", joined)
	}
	if !strings.Contains(joined, "**bold**") {
		t.Fatalf("the literal markdown must survive, got %q", joined)
	}
}

// A field too large to diff must still produce a ROW saying it moved. The
// alternative — dropping it — is a change the reader is never told about.
func TestFieldChangesReportAnOversizeFieldWithoutDiffingIt(t *testing.T) {
	restore := MaxFieldBytes
	defer func() { MaxFieldBytes = restore }()
	MaxFieldBytes = 64

	approved := model.Claim{
		ID: "widget.contract.e", Facet: "contract", Body: "Unchanged.\n",
		AuditNotes: []string{strings.Repeat("a", 200)},
	}
	current := approved
	current.AuditNotes = []string{strings.Repeat("b", 200)}

	change := editedPair(t, approved, current)
	if len(change.FieldChanges) != 1 {
		t.Fatalf("an oversize field must still produce a row, got %+v", change.FieldChanges)
	}
	if !change.FieldChanges[0].Truncated {
		t.Fatal("an oversize field must be marked truncated")
	}
	if len(change.FieldChanges[0].Hunks) != 0 {
		t.Fatal("a truncated field must show no comparison rather than a partial one")
	}
}

// A claim with no field movement must produce no field rows, so the panel
// does not grow an empty "what moved" section on a pure prose edit.
func TestFieldChangesAreEmptyWhenOnlyTheBodyMoved(t *testing.T) {
	approved := model.Claim{ID: "widget.contract.f", Facet: "contract", Body: "The approved wording.\n"}
	current := approved
	current.Body = "The current wording.\n"

	change := editedPair(t, approved, current)
	if !change.BodyChanged() {
		t.Fatal("this fixture must move the body")
	}
	if len(change.FieldChanges) != 0 {
		t.Fatalf("no field moved, so there must be no field rows; got %+v", change.FieldChanges)
	}
}

// tagsIn returns the tag names appearing in html, so a test can require that
// a fragment contains only the elements this package writes.
func tagsIn(html string) []string {
	var out []string
	for i := 0; i < len(html); i++ {
		if html[i] != '<' {
			continue
		}
		end := strings.IndexByte(html[i:], '>')
		if end < 0 {
			break
		}
		tag := html[i+1 : i+end]
		if space := strings.IndexAny(tag, " \t\n"); space >= 0 {
			tag = tag[:space]
		}
		out = append(out, tag)
		i += end
	}
	return out
}
