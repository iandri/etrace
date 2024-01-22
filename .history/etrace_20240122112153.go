package etrace

import (
	"fmt"
	"math"
	"runtime"
	"strings"

	"errors"
)

/*
CleanPath function is applied to file paths before adding them to a stacktrace.
By default, it makes the path relative to the $GOPATH environment variable.

To remove some additional prefix like "github.com" from file paths in
stacktraces, use something like:

	stacktrace.CleanPath = func(path string) string {
		path = cleanpath.RemoveGoPath(path)
		path = strings.TrimPrefix(path, "github.com/")
		return path
	}
*/
var CleanPath = RemoveGoPath

/*
NewError is a drop-in replacement for fmt.Errorf that includes line number
information. The canonical call looks like this:

	if !IsOkay(arg) {
		return stacktrace.NewError("Expected %v to be okay", arg)
	}
*/
func NewError(msg string, vals ...interface{}) error {
	e := fmt.Errorf(msg, vals...)
	return create(e, NoCode, NoStatusCode, "")
}

func Wrap(cause error) error {
	if cause == nil {
		// Allow calling Propagate without checking whether there is error
		return nil
	}
	return create(cause, NoCode, NoStatusCode, "")
}

func WrapWithCode(code ErrorCode, cause error) error {
	if cause == nil {
		// Allow calling PropagateWithCode without checking whether there is error
		return nil
	}
	return create(cause, code, NoStatusCode, "")
}

func WrapWithStatusCode(code int, cause error) error {
	if cause == nil {
		// Allow calling PropagateWithCode without checking whether there is error
		return nil
	}
	return create(cause, NoCode, code, "")
}

/*
Propagate wraps an error to include line number information. The msg and vals
arguments work like the ones for fmt.Errorf.

The message passed to Propagate should describe the action that failed,
resulting in the cause. The canonical call looks like this:

	result, err := process(arg)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to process %v", arg)
	}

To write the message, ask yourself "what does this call do?" What does
process(arg) do? It processes ${arg}, so the message is that we failed to
process ${arg}.

Pay attention that the message is not redundant with the one in err. If it is
not possible to add any useful contextual information beyond what is already
included in an error, msg can be an empty string:

	func Something() error {
		mutex.Lock()
		defer mutex.Unlock()

		err := reallySomething()
		return stacktrace.Propagate(err, "")
	}

If cause is nil, Propagate returns nil. This allows elision of some "if err !=
nil" checks.
*/
func Propagate(cause error, msg string, vals ...interface{}) error {
	if cause == nil {
		e := fmt.Errorf(msg, vals...)
		// return create(e, NoCode, NoStatusCode, "")
		// Allow calling Propagate without checking whether there is error
		return nil
	}
	return create(cause, NoCode, NoStatusCode, msg, vals...)
}

func PropagateWithCode(cause error, code ErrorCode, msg string, vals ...interface{}) error {
	if cause == nil {
		// Allow calling PropagateWithCode without checking whether there is error
		return nil
	}
	return create(cause, code, NoStatusCode, msg, vals...)
}

func PropagateWithStatusCode(cause error, code int, msg string, vals ...interface{}) error {
	if cause == nil {
		// Allow calling PropagateWithCode without checking whether there is error
		return nil
	}
	return create(cause, NoCode, code, msg, vals...)
}

/*
ErrorCode is a code that can be attached to an error as it is passed/propagated
up the stack.

There is no predefined set of error codes. You define the ones relevant to your
application:

	const (
		EcodeManifestNotFound = stacktrace.ErrorCode(iota)
		EcodeBadInput
		EcodeTimeout
	)

The one predefined error code is NoCode, which has a value of math.MaxUint16.
Avoid using that value as an error code.

An ordinary stacktrace.Propagate call preserves the error code of an error.
*/
type ErrorCode uint16
type ErrorStatusCode int

/*
NoCode is the error code of errors with no code explicitly attached.
*/
const NoCode ErrorCode = math.MaxUint16

/*
NewErrorWithCode is similar to NewError but also attaches an error code.
*/
func NewErrorWithCode(code ErrorCode, msg string, vals ...interface{}) error {
	e := fmt.Errorf(msg, vals...)
	return create(e, code, NoStatusCode, "")
}

func NewErrorWithStatusCode(statusCode int, msg string, vals ...interface{}) error {
	e := fmt.Errorf(msg, vals...)
	return create(e, NoCode, statusCode, "")
}

/*
PropagateWithCode is similar to Propagate but also attaches an error code.

	_, err := os.Stat(manifestPath)
	if os.IsNotExist(err) {
		return stacktrace.PropagateWithCode(err, EcodeManifestNotFound, "")
	}
*/

/*
NewMessageWithCode returns an error that prints just like fmt.Errorf with no
line number, but including a code. The error code mechanism can be useful by
itself even where stack traces with line numbers are not warranted.

	ttl := req.URL.Query().Get("ttl")
	if ttl == "" {
		return 0, stacktrace.NewMessageWithCode(EcodeBadInput, "Missing ttl query parameter")
	}
*/
func NewMessageWithCode(code ErrorCode, msg string, vals ...interface{}) error {
	return &Stacktrace{
		message: fmt.Sprintf(msg, vals...),
		code:    code,
	}
}

func NewMessageWithStatusCode(code int, msg string, vals ...interface{}) error {
	return &Stacktrace{
		message:    fmt.Sprintf(msg, vals...),
		statusCode: code,
	}
}

/*
GetCode extracts the error code from a stacktrace error, including wrapped ones.

	for i := 0; i < attempts; i++ {
		err := Do()
		if stacktrace.GetCode(err) != EcodeTimeout {
			return err
		}
		// try a few more times
	}
	return stacktrace.NewError("timed out after %d attempts", attempts)

GetCode returns the special value stacktrace.NoCode if err is nil or if there is
no error code attached to err.
*/
func GetCode(err error) ErrorCode {
	var trace *Stacktrace
	if errors.As(err, &trace) {
		return trace.code
	}

	return NoCode
}

func GetStatusCode(err error) int {
	var trace *Stacktrace
	if errors.As(err, &trace) {
		return trace.statusCode
	}

	return NoStatusCode
}

type Stacktrace struct {
	message    string
	cause      error
	code       ErrorCode
	statusCode int
	//statusCode ErrorStatusCode
	file     string
	function string
	line     int
}

func create(cause error, code ErrorCode, statusCode int, msg string, vals ...interface{}) error {
	// If no error code specified, inherit error code from the cause.
	if code == NoCode {
		code = GetCode(cause)
	}

	if statusCode == NoStatusCode {
		statusCode = GetStatusCode(cause)
	}

	//if statusCode == StatusBadGateway || statusCode == 0 {
	//	statusCode = GetStatusCode(cause)
	//}

	err := &Stacktrace{
		message:    fmt.Sprintf(msg, vals...),
		cause:      cause,
		code:       code,
		statusCode: statusCode,
		//statusCode: statusCode,
	}

	// Caller of create is NewError or Propagate, so user's code is 2 up.
	pc, file, line, ok := runtime.Caller(2)
	if !ok {
		return err
	}
	if CleanPath != nil {
		file = CleanPath(file)
	}
	err.file, err.line = file, line

	f := runtime.FuncForPC(pc)
	if f == nil {
		return err
	}
	err.function = ShortFuncName(f)

	return err
}

/* "FuncName" or "Receiver.MethodName" */
func ShortFuncName(f *runtime.Func) string {
	// f.Name() is like one of these:
	// - "github.com/palantir/shield/package.FuncName"
	// - "github.com/palantir/shield/package.Receiver.MethodName"
	// - "github.com/palantir/shield/package.(*PtrReceiver).MethodName"
	longName := f.Name()

	withoutPath := longName[strings.LastIndex(longName, "/")+1:]
	withoutPackage := withoutPath[strings.Index(withoutPath, ".")+1:]

	shortName := withoutPackage
	shortName = strings.Replace(shortName, "(", "", 1)
	shortName = strings.Replace(shortName, "*", "", 1)
	shortName = strings.Replace(shortName, ")", "", 1)

	return shortName
}

func (st *Stacktrace) Error() string {
	return fmt.Sprint(st)
}

// ExitCode returns the exit code associated with the stacktrace error based on its error code. If the error code is
// NoCode, return 1 (default); otherwise, returns the value of the error code.
func (st *Stacktrace) ExitCode() int {
	if st.code == NoCode {
		return 1
	}
	return int(st.code)
}
