package main

import (
	"os/exec"
)

func CreateSuspendFunc(lidClosedChecker func() bool) func() bool {
	return func() bool {
		lg.Info("Entering suspend")
		for {
			suspend := exec.Command("/usr/local/bin/sleep_program")
			if err := suspend.Start(); err != nil {
				lg.Error("Error starting suspend", "error", err.Error())
				return false
			}

			suspend.Wait()
			if lidClosedChecker() {
				lg.Info("Lid is still closed, suspending")
				continue
			} else {
				lg.Info("Exiting suspend")
				return true
			}
		}
	}
}
