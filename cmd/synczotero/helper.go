package main

import "emperror.dev/emperror"
import "emperror.dev/errors"

func GetErrorStacktrace(err error) errors.StackTrace {
	type stackTracer interface {
		StackTrace() errors.StackTrace
	}

	e := emperror.ExposeStackTrace(err)
	st, ok := e.(stackTracer)
	if !ok {
		return nil
	}

	stack := st.StackTrace()
	if len(stack) > 2 {
		stack = stack[:len(stack)-2]
	}

	return stack
	// fmt.Printf("%+v", st[0:2]) // top two frames
}
