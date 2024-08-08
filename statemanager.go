package main

import (
	"sync"
	"time"

	"github.com/rajveermalviya/go-wayland/wayland/staging/ext-idle-notify-v1"
)

type TimeoutHandler struct {
	Notification *ext_idle_notify.IdleNotification
	State        StateValue
	Timeout      time.Duration
	OnIdle       func()
	OnResume     func()
}

type StateManager struct {
	idleManager  *IdleManager
	timeouts     []*TimeoutHandler
	currentState *SafeState[StateValue]
	mu           sync.Mutex
}

func NewStateManager(idleManager *IdleManager) *StateManager {
	return &StateManager{
		idleManager:  idleManager,
		timeouts:     make([]*TimeoutHandler, 0),
		currentState: NewSafeState[StateValue](None),
	}
}

func (sm *StateManager) RegisterTimeout(state StateValue, timeout time.Duration, onIdle, onResume func()) {
	sm.register(state, timeout, onIdle, onResume, false)
}

func (sm *StateManager) RegisterTimeoutOnce(state StateValue, timeout time.Duration, onIdle, onResume func()) {
	sm.register(state, timeout, onIdle, onResume, true)
}

func (sm *StateManager) register(state StateValue, timeout time.Duration, onIdle, onResume func(), runOnce bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	h := &TimeoutHandler{
		Timeout: timeout,
		OnIdle:  onIdle,
		State:   state,
	}

	h.OnResume = func() {
		sm.mu.Lock()
		defer sm.mu.Unlock()

		if sm.currentState.Get() == state {
			onResume()
			if runOnce {
				sm.idleManager.UnregisterIdleTimeout(h.Notification)
				h.Notification = nil
			}
		}
	}
	sm.timeouts = append(sm.timeouts, h)
}

func (sm *StateManager) ReadState() StateValue {
	return sm.currentState.Get()
}

func (sm *StateManager) SetState(newState StateValue, duration time.Duration, stateFunc func() bool) {
	if sm.currentState.Get() == newState {
		lg.Debug("state already active, resetting", "state", newState.String())
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, handler := range sm.timeouts {
		if sm.currentState.Get() == handler.State && handler.Notification != nil {
			sm.idleManager.UnregisterIdleTimeout(handler.Notification)
			handler.Notification = nil
		}
	}

	lg.Debug("Successfully stopped old state", "state", sm.currentState.Get().String())
	sm.currentState.Set(None)

	timer := time.NewTimer(duration)

	if !stateFunc() {
		timer.Stop()
		lg.Debug("Did not meet criteria for", "statefunc", newState.String())
		return
	}

	<-timer.C

	for _, handler := range sm.timeouts {
		if newState == handler.State {
			handler.Notification = sm.idleManager.RegisterIdleTimeout(handler.Timeout, handler.OnIdle, handler.OnResume)
		}
	}

	sm.currentState.Set(newState)
	lg.Debug("Successfully set new state", "state", newState.String())

}
