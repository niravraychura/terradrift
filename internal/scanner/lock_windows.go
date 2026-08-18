//go:build windows

package scanner

func processExists(pid int) bool {
	// Windows process liveness checks are not implemented for local locks;
	// treat the lock as held so operators remove it deliberately.
	return pid > 0
}
