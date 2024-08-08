package main

import (
	"os"
	"os/exec"
	"sync/atomic"
	"time"

	"golang.org/x/sys/unix"

	"github.com/trbjo/goidle/utilities"
	"sync"
	"syscall"
)

func CreateLockManager(
	config *Config,
	LockChan chan<- LockStatus,
) (func() bool, func() bool, func() bool) {
	var mu sync.Mutex
	var idleLockStartedAt unix.Timespec
	var isLockRunning atomic.Int64
	LockStopRequest := make(chan bool)

	getTimeDelta := func(currentTs, startTs unix.Timespec) time.Duration {
		return (time.Duration(currentTs.Sec*1e9+currentTs.Nsec) - time.Duration(startTs.Sec*1e9+startTs.Nsec)).Round(time.Millisecond)
	}

	timeSinceBoot := func() unix.Timespec {
		var ts unix.Timespec
		err := unix.ClockGettime(unix.CLOCK_BOOTTIME, &ts)
		if err != nil {
			lg.Error("Error getting current CLOCK_BOOTTIME", "error", err.Error())
			os.Exit(128)
		}
		return ts
	}

	sendNonBlockingMessage := utilities.CreateNonBlockingSender(LockStopRequest)

	start := func(userInitiated bool) bool {
		if isLockRunning.Load() != 0 {
			return true
		}
		mu.Lock()
		defer mu.Unlock()

		if userInitiated {
			idleLockStartedAt = unix.Timespec{
				Sec:  timeSinceBoot().Sec - int64(config.IdleGracePeriod.Seconds()) - 1,
				Nsec: 0,
			}
		} else {
			idleLockStartedAt = timeSinceBoot()
		}

		MusicStop()

		lockCommand := exec.Command("/usr/bin/hyprlock")
		devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			lg.Error("could not open /dev/null for suspend", "error", err.Error())
		} else {
			lockCommand.Stderr = devNull
			lockCommand.Stdout = devNull
		}

		if err := lockCommand.Start(); err != nil {
			lg.Error("Error starting lockCommand", "error", err.Error())
			return false
		}

		instanceId := int64(lockCommand.Process.Pid)
		isLockRunning.Store(instanceId)

		go func() {
			lockCommand.Wait()
			sendNonBlockingMessage(false)
			isLockRunning.Store(0)
			LockChan <- LockExit
		}()

		go func() {
			lg.Debug("before LockStopRequest")
			manualStop := <-LockStopRequest
			lg.Debug("after LockStopRequest")
			if manualStop {
				syscall.Kill(int(instanceId), syscall.SIGUSR1)
			}
		}()
		return true
	}

	tryStop := func() bool {
		instanceId := isLockRunning.Load()
		if instanceId == 0 {
			return true
		}

		lg.Debug("unlock request for lockCommand")
		mu.Lock()
		defer mu.Unlock()

		if getTimeDelta(timeSinceBoot(), idleLockStartedAt) < config.IdleGracePeriod.Duration {
			lg.Debug("TIMEOUT unlock")
			sendNonBlockingMessage(true)
			return true
		}

		go NetWatcher(config.WifiManager.TrustedWifis, func(success bool) {
			if isLockRunning.Load() != instanceId {
				return
			}

			if success {
				lg.Debug("before WIFI unlock")
				sendNonBlockingMessage(true)
				lg.Debug("after WIFI unlock")
			} else {
				LockChan <- UnlockFailed
				lg.Debug("Not connected to trusted wifi")
			}
		})
		return false
	}

	return func() bool { return start(true) }, func() bool { return start(false) }, tryStop
}
