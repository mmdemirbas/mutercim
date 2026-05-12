package pyhelper

import "runtime"

// runtimeCallerImpl exists in its own file so the import of runtime stays
// scoped tightly. Returns the path of the calling test source file.
func runtimeCallerImpl() (uintptr, string, int, bool) {
	return runtime.Caller(2)
}
