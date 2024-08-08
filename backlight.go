package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/trbjo/goidle/utilities"
)

const backlightPath = "/sys/class/backlight"

type Backlight struct {
	device         string
	maxBright      int
	savedBright    int
	hasSaved       bool
	brightnessPath string
}

func NewBacklight() (func(BackLight), error) {
	devices, err := os.ReadDir(backlightPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read backlight devices: %v", err)
	}

	if len(devices) == 0 {
		return nil, fmt.Errorf("no backlight devices found")
	}

	b := &Backlight{device: devices[0].Name(),
		brightnessPath: filepath.Join(backlightPath, devices[0].Name(), "brightness"),
	}

	maxBrightness, err := os.ReadFile(filepath.Join(backlightPath, b.device, "max_brightness"))
	if err != nil {
		return nil, fmt.Errorf("failed to read max brightness: %v", err)
	}
	b.maxBright, err = strconv.Atoi(strings.TrimSpace(string(maxBrightness)))
	if err != nil {
		return nil, fmt.Errorf("invalid max brightness value: %v", err)
	}

	controlChan := make(chan BackLight, 1) // Buffered channel to make sends non-blocking

	go b.controlLoop(controlChan)

	sendNonBlockingMessage := utilities.CreateNonBlockingSender(controlChan)
	return sendNonBlockingMessage, nil
}

func (b *Backlight) controlLoop(controlChan <-chan BackLight) {
	for command := range controlChan {
		switch command {
		case Increase:
			b.increase()
		case Decrease:
			b.decrease()
		case Dim:
			b.dim()
		case Restore:
			b.restore()
		}
	}
}

func (b *Backlight) setBrightness(brightness int) error {
	return os.WriteFile(b.brightnessPath, []byte(strconv.Itoa(brightness)), 0644)
}

func (b *Backlight) getCurrentBrightness() (int, error) {
	data, err := os.ReadFile(b.brightnessPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read current brightness: %v", err)
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func (b *Backlight) increase() {
	current, err := b.getCurrentBrightness()
	if err != nil {
		lg.Error("Error getting current brightness:", "", err)
		return
	}

	// Handle very low brightness values
	if current < 1 {
		newBrightness := 1
		err = b.setBrightness(newBrightness)
		if err != nil {
			lg.Error("Error setting brightness:", "", err)
		}
		return
	}

	currentStep := math.Log(float64(current)) / math.Log(float64(b.maxBright))
	nextStep := currentStep + 0.0625 // 1/16 for smooth steps

	if nextStep > 1 {
		nextStep = 1
	}

	newBrightness := int(math.Pow(float64(b.maxBright), nextStep))

	// Ensure we always increase by at least 1
	if newBrightness <= current {
		newBrightness = current + 1
	}

	// Make sure we don't exceed the maximum brightness
	if newBrightness > b.maxBright {
		newBrightness = b.maxBright
	}

	err = b.setBrightness(newBrightness)
	if err != nil {
		lg.Error("Error setting brightness:", "", err)
	}
}

func (b *Backlight) decrease() {
	current, err := b.getCurrentBrightness()
	if err != nil {
		lg.Error("Error getting current brightness:", "", err)
		return
	}

	currentStep := math.Log(float64(current)) / math.Log(float64(b.maxBright))
	nextStep := currentStep - 0.0625 // 1/16 for smooth steps

	if nextStep < 0 {
		nextStep = 0
	}

	newBrightness := int(math.Pow(float64(b.maxBright), nextStep))
	if newBrightness < 1 {
		newBrightness = 1
	}

	err = b.setBrightness(newBrightness)
	if err != nil {
		lg.Error("Error setting brightness:", "", err)
	}
}

func (b *Backlight) dim() {
	current, err := b.getCurrentBrightness()
	if err != nil {
		lg.Error("Error getting current brightness:", "", err)
		return
	}

	b.savedBright = current
	b.hasSaved = true

	newBrightness := int(float64(current) * 0.2) // Dim to 20% of current brightness
	if newBrightness < 1 {
		newBrightness = 1
	}

	err = b.setBrightness(newBrightness)
	if err != nil {
		lg.Error("Error setting brightness:", "", err)
	}
}

func (b *Backlight) restore() {
	if b.hasSaved {
		err := b.setBrightness(b.savedBright)
		if err != nil {
			lg.Error("Error restoring brightness:", "", err)
		}
		b.hasSaved = false
	}
}
