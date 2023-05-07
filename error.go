package espat

import "errors"

type Error struct {
	Dev string
	Cmd string
	Err error
}

func (e *Error) Unwrap() error {
	return e.Err
}

func (e *Error) Error() string {
	return e.Dev + ": " + e.Cmd + ": " + e.Err.Error()
}

func (e *Error) Timeout() bool {
	_, to := e.Err.(timeoutError)
	return to
}

// ErrorESP represents an error code returned by ESP-AT. It is returned in the
// Error.Err field.
type ErrorESP struct {
	Code string
}

func (e *ErrorESP) Error() string {
	return e.Code
}

type timeoutError struct{}

func (e timeoutError) Error() string { return "timeout" }
func (e timeoutError) Timeout() bool { return true }

// Errors that may be returned in the Error.Err field.
var (
	ErrTimeout  = &timeoutError{}
	ErrParse    = errors.New("parse")
	ErrArgType  = errors.New("argument type")
	ErrUnkConn  = errors.New("unknown connection")
)
