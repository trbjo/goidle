package main

import (
	"sync"
)

type SafeState[T any] struct {
	value T
	mu    sync.RWMutex
}

func NewSafeState[T any](initialValue T) *SafeState[T] {
	return &SafeState[T]{
		value: initialValue,
	}
}

func (s *SafeState[T]) Set(newState T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value = newState
}

func (s *SafeState[T]) Get() T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.value
}
