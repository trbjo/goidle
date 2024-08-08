package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/rajveermalviya/go-wayland/wayland/client"
	"github.com/rajveermalviya/go-wayland/wayland/staging/ext-idle-notify-v1"
)

type SeatInfo struct {
	name string
	seat *client.Seat
}

type IdleManager struct {
	display       *client.Display
	registry      *client.Registry
	idleManager   *ext_idle_notify.IdleNotifier
	defaultSeat   *client.Seat
	notifications map[*ext_idle_notify.IdleNotification]struct{}
	mu            sync.Mutex
}

func NewIdleManager(seatName string) (*IdleManager, error) {
	display, err := client.Connect("")
	if err != nil {
		return nil, err
	}

	registry, err := display.GetRegistry()
	if err != nil {
		display.Context().Close()
		return nil, err
	}

	im := &IdleManager{
		display:       display,
		notifications: make(map[*ext_idle_notify.IdleNotification]struct{}),
		registry:      registry,
	}

	if err := im.initialize(seatName); err != nil {
		display.Context().Close()
		return nil, err
	}

	return im, nil
}

func (im *IdleManager) initialize(seatName string) error {
	var idleManagerName, idleManagerVersion uint32
	var seats []*SeatInfo

	im.registry.SetGlobalHandler(func(e client.RegistryGlobalEvent) {
		switch e.Interface {
		case "ext_idle_notifier_v1":
			idleManagerName = e.Name
			idleManagerVersion = e.Version
		case "wl_seat":
			seat := client.NewSeat(im.display.Context())
			if err := im.registry.Bind(e.Name, e.Interface, e.Version, seat); err != nil {
				lg.Error("Failed to bind seat", err)
				return
			}

			seatInfo := &SeatInfo{seat: seat}
			seat.SetNameHandler(func(e client.SeatNameEvent) {
				seatInfo.name = e.Name
				if e.Name == seatName {
					im.defaultSeat = seat
				}
			})
			seats = append(seats, seatInfo)
		}
	})

	// Perform roundtrips to ensure all global handlers are called
	im.displayRoundTrip()
	im.displayRoundTrip()

	if idleManagerName == 0 || len(seats) == 0 {
		return fmt.Errorf("failed to find required interfaces")
	}

	if im.defaultSeat == nil {
		for _, seatInfo := range seats {
			if seatInfo.name != "" {
				im.defaultSeat = seatInfo.seat
				break
			}
		}
	}

	if im.defaultSeat == nil {
		return fmt.Errorf("no valid seat found")
	}

	im.idleManager = ext_idle_notify.NewIdleNotifier(im.display.Context())
	if err := im.registry.Bind(idleManagerName, "ext_idle_notifier_v1", idleManagerVersion, im.idleManager); err != nil {
		return fmt.Errorf("failed to bind idle manager: %w", err)
	}

	return nil
}

func (im *IdleManager) displayRoundTrip() {
	// Get display sync callback
	callback, err := im.display.Sync()
	if err != nil {
		lg.Error("unable to get sync callback", "error", err.Error())
	}
	defer func() {
		if err2 := callback.Destroy(); err2 != nil {
			lg.Error("unable to destroy callback", "error", err2.Error())
		}
	}()

	done := false
	callback.SetDoneHandler(func(_ client.CallbackDoneEvent) {
		done = true
	})

	// Wait for callback to return
	for !done {
		im.display.Context().Dispatch()
	}
}

func (im *IdleManager) RegisterIdleTimeout(timeout time.Duration, onIdle func(), onResume func()) *ext_idle_notify.IdleNotification {
	im.mu.Lock()
	defer im.mu.Unlock()

	timeoutMs := uint32(timeout / time.Millisecond)
	idleNotification, err := im.idleManager.GetIdleNotification(timeoutMs, im.defaultSeat)
	if err != nil {
		lg.Error("Failed to register idle timeout", "error", err.Error())
		return nil
	}

	idleNotification.SetIdledHandler(func(e ext_idle_notify.IdleNotificationIdledEvent) {
		onIdle()
	})

	idleNotification.SetResumedHandler(func(e ext_idle_notify.IdleNotificationResumedEvent) {
		onResume()
	})

	im.notifications[idleNotification] = struct{}{}
	return idleNotification
}

func (im *IdleManager) UnregisterIdleTimeout(notification *ext_idle_notify.IdleNotification) {
	im.mu.Lock()
	defer im.mu.Unlock()

	if _, exists := im.notifications[notification]; !exists {
		lg.Warn("No notification found")
		return
	}

	err := notification.Destroy()
	if err != nil {
		lg.Error("Failed to destroy idle notification", "error", err.Error())
	}

	delete(im.notifications, notification)
}

func (im *IdleManager) Run() error {
	for {
		if err := im.display.Context().Dispatch(); err != nil {
			return err
		}
	}
}

func (im *IdleManager) Close() {
	im.display.Context().Close()
}
