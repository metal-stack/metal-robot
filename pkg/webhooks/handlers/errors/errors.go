package handlerrors

import (
	"fmt"
	"strings"
)

type (
	SkipErr struct {
		msg string
	}
)

func Skip(format string, a ...any) SkipErr {
	return SkipErr{
		msg: fmt.Sprintf("skipping because: "+format, a...),
	}
}

func SkipOnlyActions(actions ...string) SkipErr {
	return SkipErr{
		msg: fmt.Sprintf("skipping because only reacting to actions of type(s): %s", strings.Join(actions, ", ")),
	}
}

func (s SkipErr) Error() string {
	return s.msg
}
