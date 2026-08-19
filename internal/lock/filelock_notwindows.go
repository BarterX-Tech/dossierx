//go:build !windows

package lock

// lockOpenIsTransient is a constant false everywhere but Windows, and the
// constant is the point rather than a stub awaiting an implementation.
//
// unlink(2) is atomic: the directory entry is gone when it returns, so a lock
// path is either present — which surfaces as EEXIST and is already the retry
// case — or absent, in which case the next O_CREATE|O_EXCL open wins it. There
// is no in-between state for a poll to wait out.
//
// So EACCES here is a real permission problem: a claims_dir the caller cannot
// write, a read-only mount, a directory owned by another user. Retrying it
// would replace an immediate, accurate refusal with ten seconds of silence
// followed by a vaguer one, which is a worse answer to a question the operating
// system already answered correctly.
func lockOpenIsTransient(error) bool { return false }
