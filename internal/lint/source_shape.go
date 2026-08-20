// source_shape.go implements the "source-shape" lint: the structural
// legality of every entry in a claim's sources[]. It is to model.Source what
// status-shape is to model.Status and rows-shape is to model.Row — the place
// enum legality and field-combination legality live in this codebase, rather
// than the decoder.
//
// That placement is a decision, not an accident, and it matters more here than
// it does for status. model.Source's own doc comment says nothing about a
// Source is validated by the type: a malformed citation must yield a
// reportable finding, not a load error that takes the whole project down. A
// project whose viewer will not build because one author typo'd one accessed_on
// is a project whose authors learn to keep provenance out of the claim file,
// which is the exact behaviour the Source type exists to reverse.
//
// WHAT THIS RULE OWNS, and what it deliberately hands off:
//
//   - It owns ref legality (positive, unique within the claim), kind legality
//     (the closed external/internal enum), Title's presence, and CROSS-KIND
//     FIELD MISUSE — an internal source carrying a url, an external one
//     carrying a sha256.
//   - It does NOT check whether an external source is anchored
//     (source-external-unanchored) or whether an internal one still matches
//     what it hashed (source-internal-drift). Those are about whether the
//     citation is CHECKABLE and whether it is still TRUE; this one is about
//     whether the entry is a well-formed thing at all, and running it first
//     means the other two never have to reason about a half-filled entry.
//   - It does NOT check that markers and refs agree in either direction; that
//     is source-ref-undefined and source-ref-unused.
//
// WHY CROSS-KIND MISUSE IS A FINDING AND NOT A SHRUG. A url on an internal
// source is not merely redundant. Kind selects which anchor the engine
// enforces, so an internal entry carrying url + accessed_on and no sha256 reads
// to a human exactly like a properly anchored external citation while being
// checked by nothing — the author believes they recorded an anchor, the gate
// believes there was nothing to check, and both are wrong in the same
// direction. The same argument runs the other way for a sha256 on an external
// source: a hash of something the engine cannot read is a hash nothing will
// ever recompute. Every extra field of the wrong kind is one more thing a
// reader can mistake for provenance.
//
// The record_id/.jsonl check belongs here for the same reason. RecordID is
// defined as a match against a JSONL record's top-level "id" (see its doc
// comment); on a file that is not JSONL there is no such record, so the field
// pins nothing and the sha256 beside it silently pins the whole file instead.
// The author asked for a narrow anchor and got a broad one, which is precisely
// the noisy-drift failure RecordID was introduced to avoid.
//
// Severity is ERROR throughout. Every finding here describes a citation that
// either cannot be checked or will be checked against the wrong thing, and a
// citation nothing checks is worse than no citation at all: it carries the
// authority of provenance with none of the substance.
package lint

import (
	"fmt"
	"path"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, sourceShapeLint{})
}

type sourceShapeLint struct{}

// Name returns this lint's rule name.
func (sourceShapeLint) Name() string { return "source-shape" }

// jsonlExt is the only extension a RecordID may narrow into. It is a literal
// rather than a config setting for the same reason asset-scope's "assets" is:
// a per-project answer to "which files hold addressable records" would have to
// be consulted before a reviewer could tell whether a citation is anchored.
const jsonlExt = ".jsonl"

// Check reports every structurally illegal sources[] entry on every claim.
//
// Findings are emitted in claim order and then source order, and a single
// entry may produce several — an entry that is wrong about its kind AND
// missing its title is wrong about two things, and collapsing them into one
// message would send the author back for a second round after the first fix.
func (sourceShapeLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		// firstUseOfRef maps a ref to the index of the entry that claimed it,
		// so the duplicate message can name both halves of the collision
		// rather than just the second one.
		firstUseOfRef := make(map[int]int, len(c.Sources))

		for i, s := range c.Sources {
			where := fmt.Sprintf("sources[%d]", i)
			add := func(msg string) {
				findings = append(findings, Finding{
					LintName: "source-shape",
					ClaimID:  c.ID,
					Severity: SeverityError,
					Message:  where + ": " + msg,
				})
			}

			if s.Ref < 1 {
				add(fmt.Sprintf("ref %d is not positive; refs start at 1 because the body addresses them as \"[1]\", \"[2]\"", s.Ref))
			} else if first, dup := firstUseOfRef[s.Ref]; dup {
				add(fmt.Sprintf("ref %d is already used by sources[%d]; a \"[%d]\" marker in the body would name two different sources and the reader cannot tell which", s.Ref, first, s.Ref))
			} else {
				firstUseOfRef[s.Ref] = i
			}

			if strings.TrimSpace(s.Title) == "" {
				add("title is required; a citation the reader cannot identify without dereferencing it is a footnote that only works online")
			}

			switch s.Kind {
			case model.SourceKindExternal:
				findings = append(findings, sourceForbiddenFields(c, where,
					"external", "anchored by url + accessed_on",
					[]sourceField{
						{"path", s.Path},
						{"record_id", s.RecordID},
						{"sha256", s.SHA256},
					})...)

			case model.SourceKindInternal:
				findings = append(findings, sourceForbiddenFields(c, where,
					"internal", "anchored by path (+ optional record_id) and sha256",
					[]sourceField{
						{"url", s.URL},
						{"accessed_on", s.AccessedOn},
					})...)

				if strings.TrimSpace(s.Path) == "" {
					add("kind internal requires path; without a file to read there is nothing for the sha256 to pin and nothing for source-internal-drift to check")
				} else if s.RecordID != "" && !strings.EqualFold(path.Ext(s.Path), jsonlExt) {
					add(fmt.Sprintf("record_id narrows a citation to one record of a JSONL file, but path %q is not %s; as written the sha256 beside it pins the WHOLE file, so the author asked for a narrow anchor and got a broad one", s.Path, jsonlExt))
				}

			case "":
				add("kind is required; must be one of: external, internal")

			default:
				add(fmt.Sprintf("invalid kind %q; must be one of: external, internal. The vocabulary is closed on purpose — kind selects which anchor the engine enforces, not what medium the source is", string(s.Kind)))
			}
		}
	}
	return findings
}

// sourceField is one named field carrying an authored value, used to report
// cross-kind misuse without repeating the message five times.
type sourceField struct {
	name  string
	value string
}

// sourceForbiddenFields reports every field in fields that carries a value on
// a source of the given kind. The message names the anchor the kind actually
// uses, because the fix is almost never "delete the field" — it is "you meant
// the other kind", and an author who is told which anchor applies can see that
// for themselves.
func sourceForbiddenFields(c model.Claim, where, kind, anchor string, fields []sourceField) []Finding {
	var findings []Finding
	for _, f := range fields {
		if strings.TrimSpace(f.value) == "" {
			continue
		}
		findings = append(findings, Finding{
			LintName: "source-shape",
			ClaimID:  c.ID,
			Severity: SeverityError,
			Message: fmt.Sprintf(
				"%s: kind %s must not set %s — a %s source is %s, so %s records an anchor nothing will ever check while reading to a human like one that is checked",
				where, kind, f.name, kind, anchor, f.name),
		})
	}
	return findings
}
