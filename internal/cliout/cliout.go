// Package cliout is the machine contract DossierX speaks to the agent that
// operates it: one JSON envelope shape for every command, one snake_case
// error-code vocabulary, and one exit-code table.
//
// Why this is a package and not a couple of structs inside cmd/dossierx: the
// error-code vocabulary was NOT invented here. internal/serve's writeError
// already had one (rights_denied, thread_not_found, unsafe_body, ...) because
// the viewer's fetch() calls needed something stable to branch on, while the
// CLI grew a parallel set of English sentences describing the very same
// conditions. A skill that has to answer "did this fail because the human owns
// that thread?" then needs two dialects — a JSON code when the browser asked,
// a regex over prose when the terminal asked. Hosting the vocabulary here,
// ABOVE both callers, makes the two surfaces literally the same constants:
// internal/serve writes Code values onto the wire, cmd/dossierx writes the same
// Code values into the envelope's error.code, and a skill learns one table.
//
// This package deliberately knows nothing about cobra, net/http, or any
// DossierX domain type (model.Claim, lint.Finding, ...). It depends on encoding/
// json and fmt and nothing else, so both cmd/dossierx and internal/serve — and
// internal/serve's own dependencies — can import it with no risk of an import
// cycle, and so the contract can be reasoned about without reading the engine.
package cliout

import (
	"encoding/json"
	"io"
)

// Envelope is the single JSON document a machine-facing DossierX command
// prints: exactly one per invocation, on stdout, on success AND on failure.
// "Exactly one, always, on stdout" is the whole point — an agent can read one
// document and branch, instead of scraping interleaved prose and guessing
// whether the run got far enough to matter.
//
// The field set is intentionally tiny and closed:
//
//	ok         — did the command do what it was asked to do
//	command    — the command path WITHOUT the binary name ("claim lock"), so a
//	             skill can correlate a response with the call it made even when
//	             several run concurrently. Empty only when no command resolved
//	             at all, i.e. the caller named one that does not exist.
//	data       — the command's own payload; shape is per-command and documented
//	             with that command, never a free-form string
//	warnings   — non-fatal notes the text renderer would have printed; present
//	             on successful runs too, which is exactly when they are easy to
//	             miss
//	error      — populated if and only if ok is false
//	stopped_at — see StoppedAt
//
// data is deliberately allowed on a FAILED envelope. A fail-fast pipeline that
// got three steps in has produced real, useful output; discarding it because
// step four failed would force the agent to re-run the whole thing to find out
// what already happened.
type Envelope struct {
	OK       bool     `json:"ok"`
	Command  string   `json:"command"`
	Data     any      `json:"data,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	Error    *Error   `json:"error,omitempty"`

	// StoppedAt names the pipeline step a fail-fast command stopped at. It
	// exists for "dossierx check", which runs lint -> catalog -> render ->
	// scan and returns at the first failure: "ok: false" alone cannot tell an
	// agent whether the viewer on disk is stale-but-valid (stopped at scan) or
	// was never written at all (stopped at lint), and those call for different
	// next moves. Empty on success and on every command that is not a pipeline.
	// See cmd/dossierx's checkStoppedAt for the value set check emits.
	StoppedAt string `json:"stopped_at,omitempty"`
}

// Error is the failure half of the envelope. Code is what a skill branches on
// and is the ONLY field promised to be stable; Message is for the human reading
// the transcript, Details carries structured specifics (the offending ids, the
// finding list) when there are any, and Hint is a literal next command to run
// when one exists — "run: dossierx check", not "you should probably lint".
type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
	Hint    string `json:"hint,omitempty"`

	// Exit is the process exit status this failure must produce, and is never
	// serialized: it is a property of the invocation, not of the document.
	//
	// It exists as an explicit override because DossierX's exit codes predate
	// the code vocabulary and are pinned by tests and documented in the README,
	// so a code's DEFAULT exit status (see ExitCode) is occasionally not the
	// status a particular historical call site must keep producing. A call site
	// that must preserve a specific status sets this; everything else leaves it
	// zero and inherits ExitCode(Code).
	Exit int `json:"-"`
}

// ExitStatus is the process exit status for this failure: the explicit Exit
// override when one was set, otherwise the code's default from ExitCode.
func (e *Error) ExitStatus() int {
	if e == nil {
		return 0
	}
	if e.Exit != 0 {
		return e.Exit
	}
	return ExitCode(e.Code)
}

// Success builds an ok envelope for command (the binary-less command path,
// e.g. "claim lock") carrying data and any non-fatal warnings.
func Success(command string, data any, warnings []string) Envelope {
	return Envelope{OK: true, Command: command, Data: data, Warnings: warnings}
}

// Failure builds a not-ok envelope for command carrying e. Callers that also
// have a partial result (check's fail-fast steps) set Data and StoppedAt on the
// returned value afterwards.
func Failure(command string, e *Error) Envelope {
	return Envelope{OK: false, Command: command, Error: e}
}

// Write encodes env as the one JSON document of a command run.
//
// Two-space indentation matches what "dossierx lint --json" and internal/serve's
// writeJSON already emit, so the whole product looks like one tool rather than
// three. encoding/json's default HTML escaping is deliberately left ON: envelope
// payloads carry claim bodies and comment text, and a stray "<" inside a string
// that is later interpolated into a page is a hazard we do not need to think
// about twice. Encode appends the trailing newline, so stdout ends cleanly and a
// line-oriented reader sees a complete record.
func Write(w io.Writer, env Envelope) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}
