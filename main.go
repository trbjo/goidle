package main

import (
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/trbjo/goidle/logger"
	"github.com/trbjo/goidle/utilities"
)

var lg = logger.Slog

func setupIdleEvents(
	SM *StateManager,
	idleEventsFunc func(IdleEvent),
	backlightFunc func(BackLight),
	turnOffBacklight func(),
) {

	// active
	SM.RegisterTimeout(Active, 150*time.Second,
		func() { backlightFunc(Dim) },
		func() { backlightFunc(Restore) },
	)

	SM.RegisterTimeout(Active, 180*time.Second,
		func() { idleEventsFunc(IdleRequest) },
		func() {},
	)

	SM.RegisterTimeoutOnce(Idle, 30*time.Millisecond,
		func() {},
		func() { idleEventsFunc(TryUnlock) },
	)

	SM.RegisterTimeout(Idle, 15*time.Second,
		func() { turnOffBacklight() },
		func() { idleEventsFunc(TryUnlock) },
	)

	SM.RegisterTimeout(Idle, 20*time.Second,
		func() { idleEventsFunc(TryIdleToSuspend) },
		func() {},
	)
}

func nop() bool { return true }

func main() {
	opm, err := NewOutputPowerManager()
	if err != nil {
		lg.Error("Failed to create OutputPowerManager", "error", err)
		return
	}
	defer opm.Close()

	configPath := os.Getenv("GOIDLE_CONFIG")
	if configPath == "" {
		home := os.Getenv("HOME")
		configPath = filepath.Join(home, "/.config/goidle.json")
	}

	config := initConfig(configPath)

	lg.Info("Starting StateManager")

	idleEvents := make(chan IdleEvent)
	lidEvents := make(chan LidEvent)
	signalChannel := make(chan os.Signal, 1)
	LockUnlockAttempt := make(chan LockStatus)
	userRequests := make(chan UserRequest)

	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)

	idleManager, err := NewIdleManager("seat0")
	if err != nil {
		lg.Error("Failed to create idle manager", "error", err.Error())
		return
	}
	defer idleManager.Close()
	SM := NewStateManager(idleManager)

	LockStartUser, LockStartIdle, LockStop := CreateLockManager(config, LockUnlockAttempt)
	lidClosed := utilities.CreateLidChecker()
	SuspendFunc := CreateSuspendFunc(lidClosed)

	backlightFunc, err := NewBacklight()
	if err != nil {
		lg.Error(err.Error())
		return
	}

	backlightOff := func() {
		opm.Off()
		backlightFunc(Restore)
	}

	go setupDbus(config,
		utilities.CreateNonBlockingSender(lidEvents),
		utilities.CreateNonBlockingSender(userRequests),
		backlightFunc,
	)

	setupIdleEvents(SM, utilities.CreateNonBlockingSender(idleEvents), backlightFunc, backlightOff)
	SM.SetState(Active, 0, nop)
	go idleManager.Run()

	for {
		select {
		case swRes := <-LockUnlockAttempt:
			if swRes == LockExit {
				lg.Debug("LockExit event", "", swRes.String())
				SM.SetState(Active, 0, nop)
			}
			opm.On()
		case lidEvent := <-lidEvents:
			if lidClosed() == (lidEvent == LidClose) {
				if lidEvent == LidOpen {
					lg.Debug("got LidOpen event")
					if SM.ReadState() == Active {
						opm.On()
					}
				} else {
					lg.Debug("got LidClose event")
					if opm.NumOutputs() == 1 {
						go func() { userRequests <- Suspend }()
					} else {
						backlightOff()
					}
				}
			}
		case res := <-idleEvents:
			switch res {
			case TryUnlock:
				lg.Debug("got TryUnlock on idleEvents")
				if !LockStop() {
					opm.On()
				}
			case IdleRequest:
				SM.SetState(Idle, 0, func() bool {
					backlightOff()
					return LockStartIdle()
				})
			case TryIdleToSuspend:
				SM.SetState(Idle, 0, func() bool {
					// set or reset the idle state if the following shortcircuits:
					// when the laptop is not connected to an external monitor or running on AC,
					// this means that the idle state will loop with a timeout of 20 seconds (see above).
					// this ensures that even at some later point, if the laptop gets (dis)connected to a
					// power source/monitor we will react to those events.
					return !(opm.NumOutputs() == 1 && utilities.OnBattery() && SuspendFunc() && LockStop())
				})
			}
		case res := <-userRequests:
			lg.Debug("userRequests", "", res.String())
			switch res {
			case Lock:
				SM.SetState(Idle, 500*time.Millisecond, func() bool {
					backlightOff()
					return LockStartUser()
				})
			case Suspend:
				SM.SetState(Idle, 0, func() bool {
					backlightOff()
					// set or reset the idle state if the following shortcircuits:
					return !(LockStartIdle() && SuspendFunc() && LockStop())
				})
			case IdleInhibit:
				if SM.ReadState() == Active {
					SM.SetState(None, 0, nop)
				}
			case IdleAllow:
				if SM.ReadState() == None {
					SM.SetState(Active, 0, nop)
				}
			}
		case <-signalChannel:
			lg.Info("got shutdown signal")
			SM.SetState(None, 0, nop)
			DumpConfig(configPath, config)
			os.Exit(0)
		}
	}
}
