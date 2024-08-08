package main

import (
	"strings"

	"github.com/godbus/dbus/v5"
)

func dbusConnection() *dbus.Conn {
	conn, err := dbus.SessionBus()
	if err != nil {
		lg.Error("Failed to connect to SessionBus","", err)
		return nil
	}
	return conn
}

type Notification struct {
	Icon          string
	Summary       string
	Body          string
	ExpireTimeout int32
}

func MusicStop() {
	var names []string
	conn := dbusConnection()
	obj := conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus")
	call := obj.Call("org.freedesktop.DBus.ListNames", 0)
	if call.Err != nil {
		panic(call.Err)
	}
	var players []string
	call.Store(&names)
	for _, name := range names {
		if strings.HasPrefix(name, "org.mpris.MediaPlayer2.") {
			players = append(players, name)
		}
	}

	for _, player := range players {
		mediaPlayer := conn.Object(player, "/org/mpris/MediaPlayer2")
		lg.Debug("pausing", "", player)
		call := mediaPlayer.Call("org.mpris.MediaPlayer2.Player.Pause", 0)
		if call.Err != nil {
			lg.Error("Failed to send pause command", "", call.Err)
		}
	}
}
