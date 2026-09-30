package serve_test

import (
	"net/http"
	"regexp"
	"testing"
)

// claim_wire_test.go pins the claim comment routes' wire BYTES (NIT-198 F7).
// The claim and brief responses share their thread fields (threadDTO,
// embedded in commentDTO after claim_id), so reordering or renaming a field
// there is a one-line edit that would reach every claim client. The viewer
// and any script reading these routes parse them as they stand; only the
// thread and reply ids and the timestamps, which each write mints, are
// normalised.

var (
	wireThreadID = regexp.MustCompile(`c-[0-9a-f]{6}`)
	wireReplyID  = regexp.MustCompile(`r-[0-9a-f]{6}`)
	wireStamp    = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`)
)

func normaliseWire(b []byte) string {
	s := wireThreadID.ReplaceAllString(string(b), "c-TID")
	s = wireReplyID.ReplaceAllString(s, "r-RID")
	return wireStamp.ReplaceAllString(s, "STAMP")
}

func TestClaimCommentRoutes_WireBytes(t *testing.T) {
	_, base, _ := startServer(t, baseConfig, map[string]string{"claims/one.yaml": draftClaim("widget.contract.one")})

	add := briefWrite(t, base, http.MethodPost, "/api/claims/widget.contract.one/comments", `{"body":"is <b>this</b> **right**?"}`, http.StatusOK)
	const wantAdd = `{
  "thread": {
    "claim_id": "widget.contract.one",
    "id": "c-TID",
    "status": "open",
    "author": "human",
    "created": "STAMP",
    "body": "is \u003cb\u003ethis\u003c/b\u003e **right**?",
    "body_html": "\u003cp\u003eis \u0026lt;b\u0026gt;this\u0026lt;/b\u0026gt; \u003cstrong\u003eright\u003c/strong\u003e?\u003c/p\u003e",
    "edited": false,
    "replies": []
  },
  "thread_id": "c-TID"
}
`
	if got := normaliseWire(add); got != wantAdd {
		t.Fatalf("POST add bytes changed:\n%s\nwant:\n%s", got, wantAdd)
	}
	tid := wireThreadID.FindString(string(add))
	briefWrite(t, base, http.MethodPost, "/api/claims/widget.contract.one/comments/"+tid+"/replies", `{"as":"agent","body":"yes"}`, http.StatusOK)
	briefWrite(t, base, http.MethodPost, "/api/claims/widget.contract.one/comments/"+tid+"/resolve", `{}`, http.StatusOK)

	_, list := do(t, http.MethodGet, base+"/api/comments", "")
	const wantList = `{
  "comments": [
    {
      "claim_id": "widget.contract.one",
      "id": "c-TID",
      "status": "resolved",
      "author": "human",
      "created": "STAMP",
      "body": "is \u003cb\u003ethis\u003c/b\u003e **right**?",
      "body_html": "\u003cp\u003eis \u0026lt;b\u0026gt;this\u0026lt;/b\u0026gt; \u003cstrong\u003eright\u003c/strong\u003e?\u003c/p\u003e",
      "edited": false,
      "replies": [
        {
          "id": "r-RID",
          "author": "agent",
          "created": "STAMP",
          "body": "yes",
          "body_html": "\u003cp\u003eyes\u003c/p\u003e",
          "edited": false
        }
      ],
      "resolved_by": "human",
      "resolved_at": "STAMP"
    }
  ]
}
`
	if got := normaliseWire(list); got != wantList {
		t.Fatalf("GET /api/comments bytes changed:\n%s\nwant:\n%s", got, wantList)
	}
}
