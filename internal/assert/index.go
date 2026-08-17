package assert

import (
	"fmt"
	"runtime"
	"strings"
)

func impl(condition bool, format string, args ...any) {
	if !condition {
		pc, file, line, ok := runtime.Caller(2)
		funcName := "unknown"

		if ok {
			if fn := runtime.FuncForPC(pc); fn != nil {
				fullName := fn.Name()

				if lastSlash := strings.LastIndex(fullName, "/"); lastSlash != -1 {
					funcName = fullName[lastSlash+1:]
				} else {
					funcName = fullName
				}
			}
		}

		msg := fmt.Sprintf(format, args...)
		panic(fmt.Sprintf("assertion failed: %s\n  at %s() [%s:%d]", msg, funcName, file, line))
	}
}

func NotNil[T any](pointer *T, format string, args ...any) {
	impl(pointer != nil, format, args...)
}
