// markdown_sanity.go implements the "markdown-sanity" lint: it says out loud
// what the body renderer does silently.
//
// internal/render/markdown is a degrade-to-literal-text parser by design. An
// unclosed fence is not a fence, an unclosed code span is a run of literal
// backticks, a rejected link scheme is inert escaped text, a list indent that
// matches no open level snaps down to the nearest enclosing one, and "#" and
// "##" are reserved for the viewer's own chrome and render with their hashes
// showing. Every one of those is the RIGHT rendering decision — nothing is
// dropped and nothing unsafe is emitted — and every one of them is invisible
// to the author, who wrote markup and got prose. This lint is the other half
// of that contract: the renderer never fails loudly, so something else has to.
//
// ONE LINT, ELEVEN FINDING KINDS. The package's convention is one rule per
// file, and this file keeps it: markdown-sanity is a single Lint whose Name()
// is "markdown-sanity", emitting its finding kinds as distinct messages. It is
// not eleven Lints, because they are eleven symptoms of one question — "does
// this body render as its author meant it to?" — and a project that wanted to
// silence "unclosed code span" but not "rejected link scheme" would be
// silencing a craft warning and a security error with the same switch, which
// the severity split below exists to prevent.
//
// THE SEVERITY SPLIT (gate 0, decision A5) IS LOAD-BEARING, not cosmetic.
// lint.RunAll fills an unset Severity with SeverityError at a single choke
// point (lint.go), and internal/lock.Lock runs RunAll over the WHOLE claim
// corpus and refuses to lock if any error-severity finding exists ANYWHERE.
// So a craft finding left at the default would not merely be noisy — the first
// stray backtick in any draft in the project would freeze every lock in that
// project, including the locks the author needs in order to go fix it. Hence:
//
//	ERROR   (security-relevant) : rejected link scheme, rejected image src,
//	                              image in a comment body.
//	WARNING (craft)             : unclosed fence, unclosed code span,
//	                              unmatched emphasis/strike/underscore run,
//	                              dangling backslash, unresolvable list
//	                              indentation, malformed pipe table, and a
//	                              reserved "#"/"##" heading.
//
// Every Finding below sets Severity EXPLICITLY. None of them relies on the
// default, in either direction — including the error ones, where the default
// would happen to be right — so that reading any one of them tells you which
// half of the split it is in without also having to know RunAll's behaviour.
//
// The fourth error-severity finding the gate-0 decision names, "asset-scope
// violation", is asset_scope.go's, not this file's: it is the one rule in the
// engine that needs to know where a claim LIVES on disk, and folding a
// filesystem question into the parser mirror would put two unrelated failure
// modes behind one rule name. Its findings are error-severity for the reason
// stated there.
//
// SURFACES. What is scanned, and why not more:
//
//   - Body and each Steps entry get the full block+inline pass. They are the
//     image-permitting surfaces (amendment A3's per-surface permission table),
//     so an image found there is checked for a legal src rather than refused.
//   - Each Rows cell's string value gets the INLINE pass only. A cell is
//     inline-only by construction — RenderInline is the table-cell entry point
//     and never reaches the block scanner — so a fence or a heading in a cell
//     is not a defect, it is just text.
//   - Each comment (and reply) body is scanned for IMAGES ONLY. Comments are
//     reviewer-authored review discussion, not claim content: an author cannot
//     fix a reviewer's stray backtick by editing their claim, so craft
//     warnings there would be unactionable noise attached to the wrong file.
//     The image refusal is different in kind — it is a security boundary, and
//     it is the one thing about a comment body that the claim's own lint run
//     is the right place to surface.
//   - Governed.Reason and RawHTML are not scanned: the first renders no
//     images and carries no markdown ceiling worth warning about, and the
//     second is raw-html-scope's entirely.
package lint

import (
	"fmt"
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, MarkdownSanity{})
}

// MarkdownSanity is the "markdown-sanity" lint.
type MarkdownSanity struct{}

// Name returns this lint's rule name.
func (MarkdownSanity) Name() string { return "markdown-sanity" }

// Check scans every markdown-bearing surface of every claim; see this file's
// doc comment for the surface list and the severity split.
func (MarkdownSanity) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		if c.Body != "" {
			findings = append(findings, mdSanityBlockSurface(c.ID, "body", c.Body)...)
		}
		for i, step := range c.Steps {
			if step == "" {
				continue
			}
			findings = append(findings, mdSanityBlockSurface(c.ID, fmt.Sprintf("steps[%d]", i), step)...)
		}
		for i, row := range c.Rows {
			// Row is a map, so its keys are visited in sorted order: a lint
			// report is compared byte-for-byte by internal/catalog and by the
			// CLI's JSON envelope, and map iteration order would make it
			// differ between two runs over identical input.
			for _, k := range mdSortedRowKeys(row) {
				s, ok := row[k].(string)
				if !ok || s == "" {
					continue
				}
				// A cell is single-line by construction, so it carries no hard
				// break offsets: nil is exact here, not a simplification.
				findings = append(findings, mdSanityInlineSurface(c.ID, fmt.Sprintf("rows[%d].%s", i, k), s, nil, 0)...)
			}
		}
		findings = append(findings, mdSanityComments(c)...)
	}
	return findings
}

// mdSortedRowKeys returns a row's keys in a stable order.
func mdSortedRowKeys(row model.Row) []string {
	keys := make([]string, 0, len(row))
	for k := range row {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// mdLinedFinding is a finding plus the source line it belongs to, so one
// surface's findings can be reported in reading order.
type mdLinedFinding struct {
	line int
	f    Finding
}

// mdSanityBlockSurface runs the full block pass plus the inline pass over
// every prose run it produces.
//
// Findings come out in SOURCE-LINE order, not in the order the two passes
// happen to produce them. The block pass finishes before the inline pass
// starts, so without the sort a reader walking a body's findings top to bottom
// would see line 6 before line 3 — which reads as a bug in the tool at exactly
// the moment the author is trying to trust it.
func mdSanityBlockSurface(claimID, surface, source string) []Finding {
	var lined []mdLinedFinding
	add := func(line int, f Finding) { lined = append(lined, mdLinedFinding{line: line, f: f}) }
	scan := mdScanBlocks(strings.Split(source, "\n"), 1, true)

	for _, line := range scan.unclosedFences {
		add(line, mdWarn(claimID, fmt.Sprintf(
			"%s: unclosed code fence — the ``` run opened on line %d is never closed, so it is not a fence at all: the marker line falls through as ordinary text and the code below it is rendered as prose",
			surface, line)))
	}
	for _, line := range scan.reservedHeadings {
		add(line, mdWarn(claimID, fmt.Sprintf(
			"%s line %d: reserved heading level — \"#\" and \"##\" are reserved for the viewer's own chrome, so this line renders as a paragraph with its hashes still showing; use \"###\" or deeper",
			surface, line)))
	}
	for _, iss := range scan.indentIssues {
		add(iss.line, mdWarn(claimID, fmt.Sprintf(
			"%s line %d: unresolvable list indentation — this marker is indented %d column(s), which is neither deep enough to nest inside the open item nor equal to any open level's marker column, so it snaps down to the nearest enclosing level instead of nesting where it is drawn",
			surface, iss.line, iss.width)))
	}
	for _, iss := range scan.tableIssues {
		add(iss.line, mdWarn(claimID, fmt.Sprintf(
			"%s line %d: malformed pipe table — %s",
			surface, iss.line, iss.reason)))
	}
	for _, line := range scan.danglingSlashes {
		add(line, mdWarn(claimID, fmt.Sprintf(
			"%s line %d: dangling backslash — a trailing backslash spells a hard line break, but this one ends its block and so has nothing to break to; it renders as a literal backslash",
			surface, line)))
	}

	for _, run := range scan.runs {
		text, breaks := run.joined()
		for _, f := range mdSanityInlineSurface(claimID, surface, text, breaks, run.firstLine()) {
			add(run.firstLine(), f)
		}
	}

	// Stable, so that two findings on the SAME line keep the order the passes
	// produced them in and the report stays byte-identical between runs.
	sort.SliceStable(lined, func(i, j int) bool { return lined[i].line < lined[j].line })
	findings := make([]Finding, 0, len(lined))
	for _, l := range lined {
		findings = append(findings, l.f)
	}
	return findings
}

// mdSanityInlineSurface runs the inline pass over one already-joined block of
// text. breaks are the block's hard-break offsets (a code span may not span
// one, so the lint must know them or it would call a span closed that the
// renderer renders literally). line is the 1-based source line the text starts
// on, or 0 for a surface (a table cell) that has no meaningful line of its own.
func mdSanityInlineSurface(claimID, surface, text string, breaks []int, line int) []Finding {
	var findings []Finding
	at := surface
	if line > 0 {
		at = fmt.Sprintf("%s line %d", surface, line)
	}

	in := mdScanInline(text, breaks)
	for _, runLen := range in.unclosedSpans {
		findings = append(findings, mdWarn(claimID, fmt.Sprintf(
			"%s: unclosed code span — a run of %d backtick(s) has no matching closing run of exactly that length, so the backticks render as literal characters",
			at, runLen)))
	}
	for _, ch := range in.unbalanced {
		findings = append(findings, mdWarn(claimID, fmt.Sprintf(
			"%s: unmatched %q run — the delimiter has no partner on this block, and emphasis/strikethrough are outside this renderer's subset in any case, so it renders as a literal character",
			at, string(ch))))
	}
	for _, url := range in.links {
		if !mdAllowedScheme(url) {
			findings = append(findings, mdErr(claimID, fmt.Sprintf(
				"%s: rejected link scheme in href %q — only http, https, mailto and scheme-less relative/#fragment hrefs become anchors; this one renders as inert escaped text with no link",
				at, url)))
		}
	}
	for _, img := range in.images {
		if !mdImageSrcLegal(img.src) {
			findings = append(findings, mdErr(claimID, fmt.Sprintf(
				"%s: rejected image src %q — an image src must be a relative path with no scheme, no \"//\"-style authority (a backslash counts as a slash), no leading \"/\", no \"..\" segment and no \"#\" or \"?\"; this one renders as escaped literal text with no image",
				at, img.src)))
		}
	}
	return findings
}

// mdSanityComments reports images on the comment surface. Images are refused
// there by construction — markdown.Render, the entry point every comment path
// uses, renders no images at all, and only a claim body opts in through a
// distinct entry point — so this finding is not "an image would be unsafe
// here", it is "you wrote an image on a surface that will render your source
// as escaped literal text and never tell you".
func mdSanityComments(c model.Claim) []Finding {
	var findings []Finding
	report := func(where, body string) {
		for _, img := range mdImagesIn(body) {
			findings = append(findings, mdErr(c.ID, fmt.Sprintf(
				"%s: image in a comment body — images are refused on the comment surface; ![%s](%s) renders as escaped literal text. Move the image into the claim body, which is the only image-permitting surface",
				where, img.alt, img.src)))
		}
	}
	for i, cm := range c.Comments {
		report(fmt.Sprintf("comments[%d]", i), cm.Body)
		for j, rp := range cm.Replies {
			report(fmt.Sprintf("comments[%d].replies[%d]", i, j), rp.Body)
		}
	}
	return findings
}

// mdWarn builds a CRAFT finding. Severity is set explicitly — see the severity
// split in this file's doc comment for why relying on the package default here
// would freeze every lock in a consuming project.
func mdWarn(claimID, msg string) Finding {
	return Finding{
		LintName: "markdown-sanity",
		ClaimID:  claimID,
		Message:  msg,
		Severity: SeverityWarning,
	}
}

// mdErr builds a SECURITY-RELEVANT finding. Severity is set explicitly even
// though SeverityError is the package default, so that the split is legible
// from the call site rather than only from RunAll's normalization.
func mdErr(claimID, msg string) Finding {
	return Finding{
		LintName: "markdown-sanity",
		ClaimID:  claimID,
		Message:  msg,
		Severity: SeverityError,
	}
}
