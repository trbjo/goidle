package main

import (
	"github.com/godbus/dbus/v5"
	"os/exec"
)

func CreateCustomSuspendFunc(lidClosedChecker func() bool, SuspendCommand []string) func() bool {
	return func() bool {
		lg.Info("Entering suspend")
		for {
			suspend := exec.Command(SuspendCommand[0], SuspendCommand[1:]...)
			if err := suspend.Start(); err != nil {
				lg.Error("Error starting suspend", "error", err.Error())
				return false
			}

			suspend.Wait()
			if lidClosedChecker() {
				lg.Info("Lid is still closed, suspending again")
				continue
			} else {
				lg.Info("Exiting suspend")
				return true
			}
		}
	}
}

func CreateSuspendFunc(lidClosedChecker func() bool, customSuspendCommand []string) func() bool {
	if len(customSuspendCommand) > 0 {
		return CreateCustomSuspendFunc(lidClosedChecker, customSuspendCommand)
	}
	return CreateSystemdSuspendFunc(lidClosedChecker)
}

func CreateSystemdSuspendFunc(lidClosedChecker func() bool) func() bool {
	return func() bool {
		lg.Info("Entering systemd suspend")

		conn, err := dbus.ConnectSystemBus()
		if err != nil {
			lg.Error("Failed to connect to system bus", "error", err.Error())
			return false
		}
		defer conn.Close()

		// Set up signal matching
		match := dbus.WithMatchInterface("org.freedesktop.login1.Manager")
		err = conn.AddMatchSignal(match)
		if err != nil {
			lg.Error("Failed to add match for signal", "error", err.Error())
			return false
		}

		signalChan := make(chan *dbus.Signal, 10)
		conn.Signal(signalChan)

		defer func() {
			conn.RemoveSignal(signalChan)
			conn.RemoveMatchSignal(match)
		}()

		for {
			// Trigger suspend
			obj := conn.Object("org.freedesktop.login1", "/org/freedesktop/login1")
			call := obj.Call("org.freedesktop.login1.Manager.Suspend", 0, false)
			if call.Err != nil {
				lg.Error("Failed to suspend via DBus", "error", call.Err.Error())
				return false
			}

			// Wait for the suspension to complete
			for {
				select {
				case signal := <-signalChan:
					if signal.Name == "org.freedesktop.login1.Manager.PrepareForSleep" {
						preparing := signal.Body[0].(bool)
						if !preparing {
							lg.Info("System has resumed from suspend")
							if lidClosedChecker() {
								lg.Info("Lid is still closed, suspending again")
								break
							} else {
								lg.Info("Exiting suspend")
								return true
							}
						}
					}
				}
			}
		}
	}
}
