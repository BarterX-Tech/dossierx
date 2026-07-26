package cliout

import (
	"errors"
	"fmt"
)

// CodedError carries an envelope Error through Go's ordinary error chain, so a
// command body can attach a machine code at the point where it actually knows
// what went wrong — which is the only place that knowledge exists — without
// changing any function signature between there and the top of the program.
//
// It is deliberately BOTH: the wrapped cause stays reachable through Unwrap, so
// every existing errors.Is sentinel check keeps working, and Error() returns
// exactly the string fmt.Errorf would have produced, so the text-mode terminal
// output is byte-identical to what the same call site printed before it was
// given a code. That second property is what let the v0.3.0 contract be added
// under the existing golden CLI fixtures rather than alongside them.
type CodedError struct {
	// E is the envelope error this failure serializes as. Never nil for a
	// CodedError built through this package's constructors.
	E *Error
	// cause is the underlying error, preserved so errors.Is/As still see
	// whatever sentinel the original call site wrapped.
	cause error
}

func (e *CodedError) Error() string { return e.E.Message }

// Unwrap exposes the original cause so errors.Is on a package sentinel
// (config.ErrNotFound, comments.ErrRightsDenied, ...) still matches after a code
// has been attached.
func (e *CodedError) Unwrap() error { return e.cause }

// Errorf builds a coded error whose message is the formatted string, exactly as
// fmt.Errorf would produce it. %w works and is the normal case: the sentinel it
// wraps stays visible to errors.Is, and the code makes the same fact visible to
// a machine without a regex.
func Errorf(code Code, format string, a ...any) *CodedError {
	cause := fmt.Errorf(format, a...)
	return &CodedError{E: &Error{Code: code, Message: cause.Error()}, cause: cause}
}

// Wrap attaches a code to an error that already has the message it should
// report — the common shape at a call site that catches a sentinel from a lower
// package and only wants to classify it, not reword it.
func Wrap(err error, code Code) *CodedError {
	if err == nil {
		return nil
	}
	return &CodedError{E: &Error{Code: code, Message: err.Error()}, cause: err}
}

// WithHint attaches a literal next command to run. Hints are commands, not
// advice: "run: dossierx check", never "you should lint first".
func (e *CodedError) WithHint(hint string) *CodedError {
	e.E.Hint = hint
	return e
}

// WithDetails attaches the structured specifics behind the message — the
// offending ids, the finding list, the open thread ids — so a caller does not
// have to parse them back out of the prose.
func (e *CodedError) WithDetails(details any) *CodedError {
	e.E.Details = details
	return e
}

// WithExit pins this failure's process exit status, overriding the code's
// default from ExitCode. Used only where a historical call site's status is
// pinned by a test or documented, and the code's default would change it.
func (e *CodedError) WithExit(status int) *CodedError {
	e.E.Exit = status
	return e
}

// As extracts the envelope Error from anywhere in err's chain, or returns nil
// if nothing along it carries a code. Callers use it to prefer an explicitly
// attached code over their own fallback classification.
func As(err error) *Error {
	var coded *CodedError
	if errors.As(err, &coded) {
		return coded.E
	}
	return nil
}
