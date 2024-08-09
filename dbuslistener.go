package main

import (
    "github.com/godbus/dbus/v5"
    "os"
    "time"

    "github.com/trbjo/goidle/logger"
)

const (
    dbusInterface = "io.github.trbjo.WaylandListener"
    dbusPath      = "/io/github/trbjo/WaylandListener"
)

type WaylandListener struct {
    config           *Config
    userRequestsFunc func(UserRequest)
    lidEventsFunc    func(LidEvent)
    backlightFunc    func(BackLight)
}

func (o *WaylandListener) Suspend() *dbus.Error {
    go func() { o.userRequestsFunc(Suspend) }()
    return nil
}

func (o *WaylandListener) Lock() *dbus.Error {
    go func() { o.userRequestsFunc(Lock) }()
    return nil
}

func (o *WaylandListener) LidClose() *dbus.Error {
    go func() { o.lidEventsFunc(LidClose) }()
    return nil
}

func (o *WaylandListener) LidOpen() *dbus.Error {
    go func() { o.lidEventsFunc(LidOpen) }()
    return nil
}

func (o *WaylandListener) WifiTrust() *dbus.Error {
    o.config.WifiManager.AddCurrent()
    o.config.Dump()
    return nil
}

func (o *WaylandListener) WifiDistrust() *dbus.Error {
    o.config.WifiManager.RemoveCurrent()
    o.config.Dump()
    return nil
}

func (o *WaylandListener) LogDebug() *dbus.Error {
    logger.SetLogLevel("debug")
    return nil
}

func (o *WaylandListener) LogWarn() *dbus.Error {
    logger.SetLogLevel("warn")
    return nil
}

func (o *WaylandListener) LogInfo() *dbus.Error {
    logger.SetLogLevel("info")
    return nil
}

func (o *WaylandListener) IdleGraceDuration(graceDuration string) *dbus.Error {
    duration, err := time.ParseDuration(graceDuration)
    if err != nil {
        return dbus.NewError(err.Error(), []interface{}{})
    }
    lg.Info(graceDuration)
    o.config.IdleGraceDuration.Duration = duration
    o.config.Dump()
    return nil
}

func (o *WaylandListener) IdleInhibit() *dbus.Error {
    lg.Debug("IdleInhibit")
    go func() { o.userRequestsFunc(IdleInhibit) }()
    return nil
}

func (o *WaylandListener) IdleAllow() *dbus.Error {
    lg.Debug("IdleAllow")
    go func() { o.userRequestsFunc(IdleAllow) }()
    return nil
}

func (o *WaylandListener) LightIncrease() *dbus.Error {
    o.backlightFunc(Increase)
    return nil
}

func (o *WaylandListener) LightDecrease() *dbus.Error {
    o.backlightFunc(Decrease)
    return nil
}

// case "screen_toggle":
// fmt.Fprintf(&cmdBuilder, utilities.Backlight(utilities.Toggle))

func setupDbus(
    config *Config,
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

    obj := &WaylandListener{
        config:           config,
        userRequestsFunc: userRequestsFunc,
        lidEventsFunc:    lidEventsFunc,
        backlightFunc:    backlightFunc,
    }
    conn.Export(obj, dbus.ObjectPath(dbusPath), dbusInterface)

    lg.Debug("Listening on D-Bus", "interface", dbusInterface, "path", dbusPath)
    select {}
}
