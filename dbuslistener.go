package main

import (
	"github.com/godbus/dbus/v5"
	"os"
	"time"

	"github.com/trbjo/goidle/logger"
)

const (
	dbusInterface = "io.github.trbjo.GoIdle"
	dbusPath	  = "/io/github/trbjo/GoIdle"
)

type GoIdleDbus struct {
	config		   *Config
	opm			  *OutputPowerManager
	userRequestsFunc func(UserRequest)
	lidEventsFunc	func(LidEvent)
	backlightFunc	func(BackLight)
}

func (o *GoIdleDbus) Suspend() *dbus.Error {
	go func() { o.userRequestsFunc(Suspend) }()
	return nil
}

func (o *GoIdleDbus) Lock() *dbus.Error {
	go func() { o.userRequestsFunc(Lock) }()
	return nil
}

func (o *GoIdleDbus) LidClose() *dbus.Error {
	go func() { o.lidEventsFunc(LidClose) }()
	return nil
}

func (o *GoIdleDbus) LidOpen() *dbus.Error {
	go func() { o.lidEventsFunc(LidOpen) }()
	return nil
}

func (o *GoIdleDbus) WifiTrust() *dbus.Error {
	o.config.AddCurrentWifi()
	o.config.Dump()
	return nil
}

func (o *GoIdleDbus) WifiDistrust() *dbus.Error {
	o.config.RemoveCurrentWifi()
	o.config.Dump()
	return nil
}

func (o *GoIdleDbus) LogDebug() *dbus.Error {
	logger.SetLogLevel("debug")
	return nil
}

func (o *GoIdleDbus) LogWarn() *dbus.Error {
	logger.SetLogLevel("warn")
	return nil
}

func (o *GoIdleDbus) LogInfo() *dbus.Error {
	logger.SetLogLevel("info")
	return nil
}

func (o *GoIdleDbus) IdleGraceDuration(graceDuration string) *dbus.Error {
	duration, err := time.ParseDuration(graceDuration)
	if err != nil {
		return dbus.NewError(err.Error(), []interface{}{})
	}
	lg.Info(graceDuration)
	o.config.IdleGraceDuration.Duration = duration
	o.config.Dump()
	return nil
}

func (o *GoIdleDbus) ToggleOutput(output string) *dbus.Error {
	lg.Info("output toggled")
	err := o.opm.ToggleOutput(output)
	if err != nil {
		lg.Error(err.Error())
	}
	return nil
}

func (o *GoIdleDbus) IdleInhibit() *dbus.Error {
	lg.Debug("IdleInhibit")
	go func() { o.userRequestsFunc(IdleInhibit) }()
	return nil
}

func (o *GoIdleDbus) IdleAllow() *dbus.Error {
	lg.Debug("IdleAllow")
	go func() { o.userRequestsFunc(IdleAllow) }()
	return nil
}

func (o *GoIdleDbus) LightIncrease() *dbus.Error {
	o.backlightFunc(Increase)
	return nil
}

func (o *GoIdleDbus) LightDecrease() *dbus.Error {
	o.backlightFunc(Decrease)
	return nil
}

func setupDbus(
	config *Config,
	opm *OutputPowerManager,
	lidEventsFunc func(LidEvent),
	userRequestsFunc func(UserRequest),
	backlightFunc func(BackLight),
) {
	conn, err := dbus.SessionBus()
	if err != nil {
		lg.Error("Failed to connect to session bus", "error", err)
		os.Exit(1)
	}

	reply, err := conn.RequestName(dbusInterface, dbus.NameFlagDoNotQueue)
	if err != nil {
		lg.Error("Failed to request name", "error", err)
		os.Exit(1)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		lg.Error("Name already taken")
		os.Exit(1)
	}

	obj := &GoIdleDbus{
		config:		   config,
		opm:			  opm,
		userRequestsFunc: userRequestsFunc,
		lidEventsFunc:	lidEventsFunc,
		backlightFunc:	backlightFunc,
	}
	conn.Export(obj, dbus.ObjectPath(dbusPath), dbusInterface)

	lg.Debug("Listening on D-Bus", "interface", dbusInterface, "path", dbusPath)
	select {}
}
