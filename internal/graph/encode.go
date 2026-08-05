// encode.go is the payload's one and only serializer. There is exactly one
// encoder for this document — the render path's inline block and "dossierx
// serve"'s GET /api/graph both call THIS function — so there is exactly one
// escaping rule to keep correct. See the package doc comment for why that
// matters and what is forbidden.
package graph

import (
	"encoding/json"
	"fmt"
)

// Encode marshals p to the bytes that are injected into the rendered
// document and returned by GET /api/graph.
//
// It uses json.Marshal, whose HTML escaping is on and cannot be turned off,
// rather than json.Encoder, whose escaping is a settable toggle. That choice
// is the security property of this feature, not a stylistic one: the bytes
// land inside a <script type="application/json"> block where html/template
// applies no escaping of its own, so the < that json.Marshal writes for
// a '<' is the only thing between an author-authored claim label and a
// script-tag breakout. json.Encoder is not used here, and never should be.
//
// It also means output has no trailing newline (json.Encoder appends one),
// which keeps the rendered document byte-identical between the two call
// sites for the same corpus — a property internal/serve's handler test pins.
func Encode(p Payload) ([]byte, error) {
	b, err := json.Marshal(p)
	if err != nil {
		// Unreachable with the current wire types — every field is a
		// string, bool, int or a slice of those — but Marshal's error is
		// propagated rather than dropped so that adding a field with a
		// custom marshaler later cannot fail silently.
		return nil, fmt.Errorf("graph: encode payload: %w", err)
	}
	return b, nil
}
