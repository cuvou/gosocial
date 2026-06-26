package ratelimit

import "fmt"

// ErrLockedOut is a lock-out error, sent when the user is currently throttled and
// the website is NOT to even test their password if logging in.
type ErrLockedOut struct {
	message string
}

// ErrDeferred is a rate limit error which can be shown to the user AFTER their current
// login attempt is tried. For example, if they've failed to log in a few times and aren't
// currently in a LockedOut cooldown period, you can test their password and then show
// them the deferred error message at the end of the request.
type ErrDeferred struct {
	message  string
	deferred bool
}

func NewLockedOutError(message string, v ...interface{}) ErrLockedOut {
	return ErrLockedOut{
		message: fmt.Sprintf(message, v...),
	}
}

func NewDeferredError(message string, v ...interface{}) ErrDeferred {
	return ErrDeferred{
		message:  fmt.Sprintf(message, v...),
		deferred: true,
	}
}

// IsDeferredError returns whether a ratelimit error is deferred.
func IsDeferredError(err error) bool {
	if err2, ok := err.(ErrDeferred); ok {
		return err2.deferred
	}
	return false
}

func (e ErrLockedOut) Error() string {
	return e.message
}

func (e ErrDeferred) Error() string {
	return e.message
}
