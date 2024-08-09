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
	curveFactor    float64
	device         string
	maxBright      int
	savedBright    int
	hasSaved       bool
	brightnessPath string
	steps          int
	dimRatio       float64
}

func NewBacklight(config *Config) (func(BackLight), error) {
	devices, err := os.ReadDir(backlightPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read backlight devices: %v", err)
	}

	if len(devices) == 0 {
		return nil, fmt.Errorf("no backlight devices found")
	}

	b := &Backlight{
		device:         devices[0].Name(),
		brightnessPath: filepath.Join(backlightPath, devices[0].Name(), "brightness"),
		curveFactor:    config.BacklightCurveFactor,
		steps:          config.BacklightSteps,
		dimRatio:       config.BacklightDimRatio,
	}

	maxBrightness, err := os.ReadFile(filepath.Join(backlightPath, b.device, "max_brightness"))
	if err != nil {
		return nil, fmt.Errorf("failed to read max brightness: %v", err)
	}
	b.maxBright, err = strconv.Atoi(strings.TrimSpace(string(maxBrightness)))
	if err != nil {
		return nil, fmt.Errorf("invalid max brightness value: %v", err)
	}

	controlChan := make(chan BackLight, 1)

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

func calculateSteps(maxBright, numSteps int, curveFactor float64) []int {
	steps := make([]int, numSteps)
	steps[0] = 1
	steps[numSteps-1] = maxBright

	for i := 1; i < numSteps-1; i++ {
		t := math.Pow(float64(i)/float64(numSteps-1), curveFactor)
		steps[i] = int(math.Round(math.Pow(float64(maxBright), t)))
	}

	// Ensure all steps are unique and in ascending order
	uniqueSteps := []int{1}
	for i := 1; i < len(steps); i++ {
		if steps[i] > uniqueSteps[len(uniqueSteps)-1] {
			uniqueSteps = append(uniqueSteps, steps[i])
		}
	}

	// If we have fewer steps than required, interpolate
	for len(uniqueSteps) < numSteps {
		for i := 1; i < len(uniqueSteps); i++ {
			if uniqueSteps[i]-uniqueSteps[i-1] > 1 {
				newStep := (uniqueSteps[i] + uniqueSteps[i-1]) / 2
				uniqueSteps = append(uniqueSteps[:i], append([]int{newStep}, uniqueSteps[i:]...)...)
				break
			}
		}
	}

	// If we somehow ended up with more steps, trim
	if len(uniqueSteps) > numSteps {
		uniqueSteps = uniqueSteps[:numSteps]
	}

	return uniqueSteps
}

func (b *Backlight) increase() {
	current, err := b.getCurrentBrightness()
	if err != nil {
		lg.Error("Error getting current brightness:", "", err)
		return
	}

	steps := calculateSteps(b.maxBright, b.steps, b.curveFactor)

	for _, step := range steps {
		if step > current {
			err = b.setBrightness(step)
			if err != nil {
				lg.Error("Error setting brightness:", "", err)
			}
			return
		}
	}
}

func (b *Backlight) decrease() {
	current, err := b.getCurrentBrightness()
	if err != nil {
		lg.Error("Error getting current brightness:", "", err)
		return
	}

	steps := calculateSteps(b.maxBright, b.steps, b.curveFactor)

	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i] < current {
			err = b.setBrightness(steps[i])
			if err != nil {
				lg.Error("Error setting brightness:", "", err)
			}
			return
		}
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

	newBrightness := int(float64(current) * b.dimRatio)
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
