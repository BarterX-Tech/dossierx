package serve_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// brief_comments_test.go pins the /api/briefs/{id}/comments routes (NIT-198)
// at the HTTP boundary. The thread operations themselves are internal/comments'
// (TestBriefThreads_TheSameOpsAndRightsAsAClaim); what only a request can
// show is the routing — a brief addressed by its id and nothing else, a brief
// and a claim sharing one id never crossing — the wire shape, and the rights
// and error codes as a browser receives them. Admission (Host, Origin,
// Content-Type, Sec-Fetch-Site) for every brief write is swept with the claim
// writes by mutatingEndpoints in serve_test.go.

type briefThreadWire struct {
	BriefID    string `json:"brief_id"`
	Path       string `json:"path"`
	ClaimID    string `json:"claim_id"`
	ID         string `json:"id"`
	Status     string `json:"status"`
	Author     string `json:"author"`
	Body       string `json:"body"`
	BodyHTML   string `json:"body_html"`
	Edited     bool   `json:"edited"`
	ResolvedBy string `json:"resolved_by"`
	Replies    []struct {
		ID       string `json:"id"`
		Author   string `json:"author"`
		BodyHTML string `json:"body_html"`
	} `json:"replies"`
}

func listBriefThreads(t *testing.T, base, id, query string) []briefThreadWire {
	t.Helper()
	resp, data := do(t, http.MethodGet, base+"/api/briefs/"+id+"/comments"+query, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET brief %s threads: %d %s", id, resp.StatusCode, data)
	}
	var out struct {
		Comments []briefThreadWire `json:"comments"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode brief list: %v (%s)", err, data)
	}
	return out.Comments
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func briefWrite(t *testing.T, base, method, path, body string, wantStatus int) []byte {
	t.Helper()
	mods := allowedMutating(base)
	if method == http.MethodDelete {
		mods = []reqMod{setHeader("Origin", base)}
	}
	resp, data := do(t, method, base+path, body, mods...)
	if resp.StatusCode != wantStatus {
		t.Fatalf("%s %s: got %d, want %d (%s)", method, path, resp.StatusCode, wantStatus, data)
	}
	return data
}

// TestBriefComments_TheHumanOpensRepliesAndResolves walks one thread through
// every brief route: the human opens it, the agent replies and is refused the
// resolve, the human resolves it (the open list empties), reopens it, edits
// it, the agent deletes its own reply, and the human deletes the thread. Each
// write lands in the brief's frontmatter and nowhere else; the claim routes'
// list never shows it.
func TestBriefComments_TheHumanOpensRepliesAndResolves(t *testing.T) {
	_, base, root := startServer(t, baseConfig, admissionFiles())
	claimsBefore := snapshotClaims(t, root)
	briefFile := filepath.Join(root, "briefs", "widget", "flow.md")

	data := briefWrite(t, base, http.MethodPost, "/api/briefs/widget.flow/comments", `{"body":"is this still the flow?"}`, http.StatusOK)
	var added struct {
		ThreadID string          `json:"thread_id"`
		Thread   briefThreadWire `json:"thread"`
	}
	if err := json.Unmarshal(data, &added); err != nil {
		t.Fatal(err)
	}
	th := added.Thread
	if th.BriefID != "widget.flow" || th.Path != "briefs/widget/flow.md" || th.ClaimID != "" || th.Author != "human" || th.Status != "open" || th.ID != added.ThreadID {
		t.Fatalf("added thread = %+v (thread_id %q)", th, added.ThreadID)
	}
	tid := added.ThreadID
	raw, err := os.ReadFile(briefFile)
	if err != nil || !strings.Contains(string(raw), "is this still the flow?") || !strings.HasSuffix(string(raw), "---\n# Flow\n\nText.\n") {
		t.Fatalf("the thread must be written into the brief's frontmatter and leave its body alone:\n%s (%v)", raw, err)
	}
	for path, b := range claimsBefore {
		if strings.Contains(path, string(filepath.Separator)+"claims"+string(filepath.Separator)) {
			if now := mustReadFile(t, path); now != string(b) {
				t.Fatalf("a brief comment rewrote claim file %s", path)
			}
		}
	}
	if got := len(listBriefThreads(t, base, "widget.flow", "?open=1")); got != 1 {
		t.Fatalf("open threads on the brief = %d, want 1", got)
	}
	_, claimList := do(t, http.MethodGet, base+"/api/comments", "")
	if strings.Contains(string(claimList), "is this still the flow?") || strings.Contains(string(claimList), "widget.flow") {
		t.Fatalf("GET /api/comments is the claims' list and must not carry a brief's thread: %s", claimList)
	}

	briefWrite(t, base, http.MethodPost, "/api/briefs/widget.flow/comments/"+tid+"/replies", `{"as":"agent","body":"yes, unchanged"}`, http.StatusOK)
	refused := briefWrite(t, base, http.MethodPost, "/api/briefs/widget.flow/comments/"+tid+"/resolve", `{"as":"agent"}`, http.StatusForbidden)
	assertErrorCode(t, refused, "rights_denied")

	data = briefWrite(t, base, http.MethodPost, "/api/briefs/widget.flow/comments/"+tid+"/resolve", `{}`, http.StatusOK)
	var resolved struct {
		Thread briefThreadWire `json:"thread"`
	}
	if err := json.Unmarshal(data, &resolved); err != nil || resolved.Thread.Status != "resolved" || resolved.Thread.ResolvedBy != "human" {
		t.Fatalf("resolve answered %+v (%v)", resolved.Thread, err)
	}
	for _, open := range listBriefThreads(t, base, "widget.flow", "?open=1") {
		if open.ID == tid {
			t.Fatalf("a resolved thread is still listed open")
		}
	}
	again := briefWrite(t, base, http.MethodPost, "/api/briefs/widget.flow/comments/"+tid+"/resolve", `{}`, http.StatusConflict)
	assertErrorCode(t, again, "thread_resolved")

	briefWrite(t, base, http.MethodPost, "/api/briefs/widget.flow/comments/"+tid+"/reopen", `{}`, http.StatusOK)
	data = briefWrite(t, base, http.MethodPatch, "/api/briefs/widget.flow/comments/"+tid, `{"body":"is this the flow now?"}`, http.StatusOK)
	var edited struct {
		Thread briefThreadWire `json:"thread"`
	}
	if err := json.Unmarshal(data, &edited); err != nil || !edited.Thread.Edited || edited.Thread.Body != "is this the flow now?" || len(edited.Thread.Replies) != 1 {
		t.Fatalf("edit answered %+v (%v)", edited.Thread, err)
	}
	rid := edited.Thread.Replies[0].ID
	briefWrite(t, base, http.MethodDelete, "/api/briefs/widget.flow/comments/"+tid+"?reply="+rid+"&as=agent", "", http.StatusOK)
	denied := briefWrite(t, base, http.MethodDelete, "/api/briefs/widget.flow/comments/"+tid+"?as=agent", "", http.StatusForbidden)
	assertErrorCode(t, denied, "rights_denied")
	data = briefWrite(t, base, http.MethodDelete, "/api/briefs/widget.flow/comments/"+tid, "", http.StatusOK)
	if !strings.Contains(string(data), `"deleted": true`) {
		t.Fatalf("delete answered %s", data)
	}
	if all := listBriefThreads(t, base, "widget.flow", ""); len(all) != 0 {
		t.Fatalf("after the delete the brief still holds %+v", all)
	}
	missing := briefWrite(t, base, http.MethodPost, "/api/briefs/widget.flow/comments/"+tid+"/reopen", `{}`, http.StatusNotFound)
	assertErrorCode(t, missing, "thread_not_found")
}

// TestBriefComments_AnIDAndOnlyAnID: a brief route names a brief by its
// <folder>.<slug> id. Its path — the other spelling internal/briefs' Lookup
// accepts, reachable here as an escaped %2F — and a ./-prefixed id are
// brief_not_found, as is an id that names no brief; nothing is written.
func TestBriefComments_AnIDAndOnlyAnID(t *testing.T) {
	_, base, root := startServer(t, baseConfig, admissionFiles())
	before := snapshotClaims(t, root)
	for _, ref := range []string{"briefs%2Fwidget%2Fflow.md", "widget%2Fflow.md", ".%2Fwidget.flow", "widget%5Cflow", "widget.nope"} {
		resp, data := do(t, http.MethodGet, base+"/api/briefs/"+ref+"/comments", "")
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("GET with ref %q: got %d, want 404 (%s)", ref, resp.StatusCode, data)
		}
		assertErrorCode(t, data, "brief_not_found")
		data = briefWrite(t, base, http.MethodPost, "/api/briefs/"+ref+"/comments", `{"body":"x"}`, http.StatusNotFound)
		assertErrorCode(t, data, "brief_not_found")
	}
	assertClaimsUnchanged(t, before, root)
}

// TestBriefComments_ABriefAndAClaimSharingAnIDNeverCross: a project claim and
// a brief are both two dot-separated segments, so briefs/project/flow.md and
// the project claim project.flow share the id "project.flow". A thread opened
// on either route lands on that route's subject only, and each list shows only
// its own.
func TestBriefComments_ABriefAndAClaimSharingAnIDNeverCross(t *testing.T) {
	files := admissionFiles()
	files["briefs/project/flow.md"] = "---\nsummary: The project flow.\nstatus: draft\nrests_on: []\n---\n# Flow\n\nText.\n"
	files["project-claims/flow.yaml"] = "id: project.flow\nscope: project\nstatus: draft\nlayout: card\nsummary: A project claim sharing the brief's id.\n" +
		"body: |\n  a project claim.\nrests_on:\n  none: true\n  reason: fixture\n"
	_, base, root := startServer(t, baseConfig, files)

	briefWrite(t, base, http.MethodPost, "/api/briefs/project.flow/comments", `{"body":"on the brief"}`, http.StatusOK)
	briefWrite(t, base, http.MethodPost, "/api/claims/project.flow/comments", `{"body":"on the claim"}`, http.StatusOK)

	briefRaw := mustReadFile(t, filepath.Join(root, "briefs", "project", "flow.md"))
	claimRaw := mustReadFile(t, filepath.Join(root, "project-claims", "flow.yaml"))
	if !strings.Contains(briefRaw, "on the brief") || strings.Contains(briefRaw, "on the claim") {
		t.Fatalf("the brief file holds the wrong threads:\n%s", briefRaw)
	}
	if !strings.Contains(claimRaw, "on the claim") || strings.Contains(claimRaw, "on the brief") {
		t.Fatalf("the claim file holds the wrong threads:\n%s", claimRaw)
	}
	onBrief := listBriefThreads(t, base, "project.flow", "")
	if len(onBrief) != 1 || onBrief[0].Body != "on the brief" {
		t.Fatalf("the brief's list = %+v", onBrief)
	}
	_, claimList := do(t, http.MethodGet, base+"/api/comments", "")
	if strings.Contains(string(claimList), "on the brief") || !strings.Contains(string(claimList), "on the claim") {
		t.Fatalf("the claims' list crossed with the brief's: %s", claimList)
	}
}

// TestBriefComments_AHostileBodyIsInert: a brief thread's body_html comes from
// the one markdown renderer, as a claim's does, on the write's answer and on
// the list.
func TestBriefComments_AHostileBodyIsInert(t *testing.T) {
	_, base, _ := startServer(t, baseConfig, admissionFiles())
	body, err := json.Marshal(map[string]string{"body": `<img src=x onerror=alert(1)>`})
	if err != nil {
		t.Fatal(err)
	}
	data := briefWrite(t, base, http.MethodPost, "/api/briefs/widget.flow/comments", string(body), http.StatusOK)
	var added struct {
		Thread briefThreadWire `json:"thread"`
	}
	if err := json.Unmarshal(data, &added); err != nil {
		t.Fatal(err)
	}
	assertEscaped(t, "brief POST body_html", added.Thread.BodyHTML)
	for _, th := range listBriefThreads(t, base, "widget.flow", "") {
		assertEscaped(t, "brief GET body_html", th.BodyHTML)
	}
}
