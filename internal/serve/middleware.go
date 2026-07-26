package serve

import (
	"fmt"
	"mime"
	"net"
	"net/http"
	"strconv"
)

// admission is the single trust boundary for "dossierx serve": it runs on
// EVERY request, before any handler, and rejects anything that is not a
// same-origin call from a browser tab actually pointed at this local server.
// It is deliberately strict and layered; getting any one rule wrong (most
// dangerously the "reject absent/null Origin" and "never compare Origin to
// Host" details) reopens the write API to a hostile web page, because the API
// is otherwise unauthenticated.
//
//  1. Host allowlist (every method, incl. GET / and the API). r.Host must be
//     exactly 127.0.0.1 or localhost on the listening port. This is the ONLY
//     DNS-rebinding defense: an attacker page that rebinds its own name to
//     127.0.0.1 still sends its own name in Host, so it fails here. We never
//     compare Origin against r.Host — under rebinding the attacker's origin
//     matches its own rebound host, so that comparison would wave it through.
//  2. Sec-Fetch-Site (every method, when present). Modern browsers stamp the
//     request's relationship to its initiator; only same-origin/none may pass.
//     A cross-site/cross-origin fetch is rejected even before the Origin check.
//  3. Origin allowlist (mutating methods). POST/PATCH/DELETE require Origin to
//     string-match http://127.0.0.1:<port> or http://localhost:<port>.
//     Origin: null AND an absent Origin are BOTH rejected — a form-post or
//     opaque-origin request carries no usable Origin, and accepting "no Origin"
//     is the mistake that reopens CSRF.
//  4. Content-Type (POST/PATCH). The parsed media type must be exactly
//     application/json, which kills the enctype=text/plain simple-request path
//     a cross-site form could otherwise use without a preflight.
//  5. Body cap (mutating methods). http.MaxBytesReader bounds the body.
//  6. No CORS header, ever. The server emits no Access-Control-Allow-Origin on
//     any response (success or rejection); there is nothing to relax because
//     every legitimate caller is same-origin.
func (s *Server) admission(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// (1) Host allowlist — the sole DNS-rebinding defense.
		if !s.hostAllowed(r.Host) {
			http.Error(w, "misdirected request: host not allowed", http.StatusMisdirectedRequest)
			return
		}

		// (2) Sec-Fetch-Site — when the browser sends it, only same-origin/none pass.
		if sfs := r.Header.Get("Sec-Fetch-Site"); sfs != "" && sfs != "same-origin" && sfs != "none" {
			http.Error(w, "forbidden: cross-site request", http.StatusForbidden)
			return
		}

		if isMutating(r.Method) {
			// (3) Origin allowlist — absent and "null" Origin are rejected.
			if !s.originAllowed(r.Header.Get("Origin")) {
				http.Error(w, "forbidden: origin not allowed", http.StatusForbidden)
				return
			}
			// (4) Content-Type must be application/json for POST/PATCH.
			if r.Method == http.MethodPost || r.Method == http.MethodPatch {
				if !isJSONContentType(r.Header.Get("Content-Type")) {
					http.Error(w, "unsupported media type: application/json required", http.StatusUnsupportedMediaType)
					return
				}
			}
			// (5) Cap the body a handler will read.
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		}

		// (6) No Access-Control-Allow-Origin is ever set — falling through to
		// the handler leaves the response CORS-header-free by construction.
		next.ServeHTTP(w, r)
	})
}

// isMutating reports whether method changes state and therefore needs the
// Origin/Content-Type/body-cap gates. GET/HEAD are read-only and skip them.
func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// hostAllowed reports whether r.Host names this server exactly: host part
// 127.0.0.1 or localhost, port part equal to the listening port. A Host with
// no port (net.SplitHostPort errors) is rejected — serve always runs on an
// explicit non-default port, so a portless Host cannot be a legitimate caller
// and must not be waved through.
func (s *Server) hostAllowed(host string) bool {
	h, p, err := net.SplitHostPort(host)
	if err != nil {
		return false
	}
	if p != strconv.Itoa(s.Port()) {
		return false
	}
	return h == "127.0.0.1" || h == "localhost"
}

// originAllowed reports whether origin exactly matches one of the two
// same-origin values for this server. An empty string (absent Origin) or the
// literal "null" (opaque origin) both fail this test, which is the point.
func (s *Server) originAllowed(origin string) bool {
	port := s.Port()
	return origin == fmt.Sprintf("http://127.0.0.1:%d", port) ||
		origin == fmt.Sprintf("http://localhost:%d", port)
}

// isJSONContentType reports whether ct parses to exactly the application/json
// media type. Parameters (e.g. "; charset=utf-8") are allowed and ignored; a
// missing or non-JSON type fails. Parsing (rather than a prefix match) is what
// makes "text/plain" — the CSRF-friendly simple-request type — fail cleanly.
func isJSONContentType(ct string) bool {
	mt, _, err := mime.ParseMediaType(ct)
	return err == nil && mt == "application/json"
}
