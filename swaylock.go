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

func CreateSwaylockManager(
	config *Config,
	SwaylockChan chan<- SwaylockStatus,
) (func() bool, func() bool, func() bool) {
	var mu sync.Mutex
	var idleLockStartedAt unix.Timespec
	var isSwaylockRunning atomic.Int64
	swaylockStopRequest := make(chan bool)

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

	sendNonBlockingMessage := utilities.CreateNonBlockingSender(swaylockStopRequest)

	start := func(userInitiated bool) bool {
		if isSwaylockRunning.Load() != 0 {
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

		swaylock := exec.Command("/usr/bin/hyprlock")
		devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			lg.Error("could not open /dev/null for suspend", "error", err.Error())
		} else {
			swaylock.Stderr = devNull
			swaylock.Stdout = devNull
		}

		if err := swaylock.Start(); err != nil {
			lg.Error("Error starting swaylock", "error", err.Error())
			return false
		}

		instanceId := int64(swaylock.Process.Pid)
		isSwaylockRunning.Store(instanceId)

		go func() {
			swaylock.Wait()
			sendNonBlockingMessage(false)
			isSwaylockRunning.Store(0)
			SwaylockChan <- SwaylockExit
		}()

		go func() {
			lg.Debug("before swaylockStopRequest")
			manualStop := <-swaylockStopRequest
			lg.Debug("after swaylockStopRequest")
			if manualStop {
				syscall.Kill(int(instanceId), syscall.SIGUSR1)
			}
		}()
		return true
	}

	tryStop := func() bool {
		instanceId := isSwaylockRunning.Load()
		if instanceId == 0 {
			return true
		}

		lg.Debug("unlock request for swaylock")
		mu.Lock()
		defer mu.Unlock()

		if getTimeDelta(timeSinceBoot(), idleLockStartedAt) < config.IdleGracePeriod.Duration {
			lg.Debug("TIMEOUT unlock")
			sendNonBlockingMessage(true)
			return true
		}

		go NetWatcher(config.WifiManager.TrustedWifis, func(success bool) {
			if isSwaylockRunning.Load() != instanceId {
				return
			}

			if success {
				lg.Debug("before WIFI unlock")
				sendNonBlockingMessage(true)
				lg.Debug("after WIFI unlock")
			} else {
				SwaylockChan <- UnlockFailed
				lg.Debug("Not connected to trusted wifi")
			}
		})
		return false
	}

	return func() bool { return start(true) }, func() bool { return start(false) }, tryStop
}
