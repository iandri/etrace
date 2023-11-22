package stacktrace

import (
	"errors"
)

/*
RootCause unwraps the original error that caused the current one.

	_, err := f()
	if perr, ok := stacktrace.RootCause(err).(*ParsingError); ok {
		showError(perr.Line, perr.Column, perr.Text)
	}
*/
func RootCause(err error) error {
	var st *Stacktrace
	for {
		if !errors.As(err, &st) {
			return err
		}
		if st.cause == nil {
			return errors.New(st.message)
		}
		err = st.cause
	}
}
