package utilities

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/trbjo/goidle/logger"
)

var lg = logger.Slog

func CreateNonBlockingSender[T any](ch chan T) func(T) {
	return func(msg T) {
		select {
		case ch <- msg:
			// Message sent successfully
		default:
			// Channel is full, drain it and try again
			drainChannel(ch)
			select {
			case ch <- msg:
				// Message sent after draining
			default:
				// Channel is still full or closed, message dropped
			}
		}
	}
}

func drainChannel[T any](ch chan T) {
	for {
		select {
		case <-ch:
			// Removed an item
		default:
			// Channel is empty
			return
		}
	}
}

func CreateLidChecker() func() bool {
	var lidDirs []string

	// Find lid directories only once
	dirs, err := filepath.Glob("/proc/acpi/button/lid/LID*")
	if err != nil {
		lg.Error("Error finding lid directories", "error", err.Error())
	} else if len(dirs) == 0 {
		lg.Error("No lid directories found")
	} else {
		lidDirs = dirs
	}

	// Return a closure that checks the lid state
	return func() bool {
		if len(lidDirs) == 0 {
			return false
		}

		for _, dir := range lidDirs {
			data, err := os.ReadFile(filepath.Join(dir, "state"))
			if err != nil {
				lg.Error("Error reading lid state", "error", err.Error(), "dir", dir)
				continue
			}
			if strings.Contains(string(data), "state:      closed") {
				return true
			}
		}
		return false
	}
}

func OnBattery() bool {
	path := "/sys/class/power_supply/"
	files, err := os.ReadDir(path)
	if err != nil {
		return false
	}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), "AC") {
			data, err := os.ReadFile(filepath.Join(path, file.Name(), "online"))
			if err != nil {
				return false
			}
			return strings.TrimSpace(string(data)) == "0"
		}
	}
	return false
}
