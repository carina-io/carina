package exec

import (
	"os/exec"
	"syscall"
)

func ExitStatus(err error) (int, bool) {
	exitErr, ok := err.(*exec.ExitError)
	if ok {
		waitStatus, ok := exitErr.ProcessState.Sys().(syscall.WaitStatus)
		if ok {
			return waitStatus.ExitStatus(), true
		}
	}
	return 0, false
}
