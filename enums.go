package main

import (
	"strconv"
)

func (t StateValue) String() string {
	switch t {
	case Active:
		return "Active"
	case Idle:
		return "Idle"
	case None:
		return "No state"
	default:
		t := strconv.Itoa(int(t))
		return t
	}
}
func (t UserRequest) String() string {
	switch t {
	case Lock:
		return "Lock"
	case Suspend:
		return "Suspend"
	default:
		t := strconv.Itoa(int(t))
		return t

	}
}
func (t LockStatus) String() string {
	switch t {
	case LockExit:
		return "LockExit"
	case UnlockFailed:
		return "UnlockFailed"
	default:
		t := strconv.Itoa(int(t))
		return t

	}
}

type IdleEvent int
type LidEvent int
type StateValue int
type LockStatus int
type UserRequest int
type BackLight int

const (
	Suspend     UserRequest = 1
	Lock        UserRequest = 2
	IdleInhibit UserRequest = 4
	IdleAllow   UserRequest = 8

	TryUnlock        IdleEvent = 16
	IdleRequest      IdleEvent = 32
	TryIdleToSuspend IdleEvent = 64

	LidClose LidEvent = 128
	LidOpen  LidEvent = 256

	LockExit LockStatus = 512
	UnlockFailed LockStatus = 1024

	Active StateValue = 2048
	Idle   StateValue = 4096
	None   StateValue = 16384

	Increase BackLight = 32768
	Decrease BackLight = 65536
	Dim      BackLight = 262144
	Restore  BackLight = 524288
)
