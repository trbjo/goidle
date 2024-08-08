package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rajveermalviya/go-wayland/wayland/client"
	"github.com/trbjo/goidle/wlroutput"
)

// OutputPowerManager manages the power state of outputs
type OutputPowerManager struct {
	display  *client.Display
	registry *client.Registry
	manager  *wlroutput.OutputPowerManagerV1
	outputs  map[*client.Output]*wlroutput.OutputPowerV1
	mu       sync.Mutex
}

// NewOutputPowerManager creates a new OutputPowerManager
func NewOutputPowerManager() (*OutputPowerManager, error) {
	display, err := client.Connect("")
	if err != nil {
		return nil, err
	}

	registry, err := display.GetRegistry()
	if err != nil {
		display.Context().Close()
		return nil, err
	}

	opm := &OutputPowerManager{
		display:  display,
		registry: registry,
		outputs:  make(map[*client.Output]*wlroutput.OutputPowerV1),
	}

	if err := opm.initialize(); err != nil {
		display.Context().Close()
		return nil, err
	}

	return opm, nil
}

func (opm *OutputPowerManager) initialize() error {
	var managerName, managerVersion uint32
	var outputsToAdd []*client.Output

	opm.registry.SetGlobalHandler(func(e client.RegistryGlobalEvent) {
		switch e.Interface {
		case "zwlr_output_power_manager_v1":
			managerName = e.Name
			managerVersion = e.Version
		case "wl_output":
			output := client.NewOutput(opm.display.Context())
			if err := opm.registry.Bind(e.Name, e.Interface, e.Version, output); err != nil {
				fmt.Printf("Failed to bind output: %v\n", err)
				return
			}
			outputsToAdd = append(outputsToAdd, output)
		}
	})

	// Perform roundtrips to ensure all global handlers are called
	opm.displayRoundTrip()
	opm.displayRoundTrip()

	if managerName == 0 {
		return fmt.Errorf("failed to find zwlr_output_power_manager_v1 interface")
	}

	opm.manager = wlroutput.NewOutputPowerManagerV1(opm.display.Context())
	if err := opm.registry.Bind(managerName, "zwlr_output_power_manager_v1", managerVersion, opm.manager); err != nil {
		return fmt.Errorf("failed to bind output power manager: %w", err)
	}

	// Now that the manager is initialized, add the outputs
	for _, output := range outputsToAdd {
		opm.addOutput(output)
	}

	return nil
}

func (opm *OutputPowerManager) displayRoundTrip() {
	callback, err := opm.display.Sync()
	if err != nil {
		fmt.Printf("Unable to get sync callback: %v\n", err)
		return
	}
	defer callback.Destroy()

	done := false
	callback.SetDoneHandler(func(_ client.CallbackDoneEvent) {
		done = true
	})

	for !done {
		opm.display.Context().Dispatch()
	}
}

func (opm *OutputPowerManager) addOutput(output *client.Output) {
	opm.mu.Lock()
	defer opm.mu.Unlock()

	if opm.manager == nil {
		fmt.Println("Output power manager is not initialized")
		return
	}

	outputPower, err := opm.manager.GetOutputPower(output)
	if err != nil {
		fmt.Printf("Failed to get output power: %v\n", err)
		return
	}

	outputPower.SetModeHandler(func(e wlroutput.OutputPowerV1ModeEvent) {
		fmt.Printf("Output power mode changed: %d\n", e.Mode)
	})

	outputPower.SetFailedHandler(func(e wlroutput.OutputPowerV1FailedEvent) {
		fmt.Printf("Output power failed\n")
		opm.removeOutput(output)
	})

	opm.outputs[output] = outputPower
}

func (opm *OutputPowerManager) removeOutput(output *client.Output) {
	opm.mu.Lock()
	defer opm.mu.Unlock()

	if outputPower, ok := opm.outputs[output]; ok {
		outputPower.Destroy()
		delete(opm.outputs, output)
	}
}

func (opm *OutputPowerManager) NumOutputs() int {
	return len(opm.outputs)
}

func (opm *OutputPowerManager) On() {
	opm.action(wlroutput.OutputPowerV1ModeOn)
}

func (opm *OutputPowerManager) Off() {
	opm.action(wlroutput.OutputPowerV1ModeOff)
}

func (opm *OutputPowerManager) action(mode wlroutput.OutputPowerV1Mode) {
	var reconnectAttempts int
	for reconnectAttempts < 3 {
		var failedOutputs []*client.Output
		for output := range opm.outputs {
			err := opm.SetOutputPowerMode(output, mode)
			if err != nil {
				if strings.Contains(err.Error(), "broken pipe") {
					failedOutputs = append(failedOutputs, output)
				} else {
					fmt.Printf("Failed to set output power mode: %v\n", err)
				}
			}
		}

		if len(failedOutputs) > 0 {
			fmt.Println("Connection lost. Attempting to reconnect...")
			err := opm.reconnect()
			if err != nil {
				fmt.Printf("Failed to reconnect: %v\n", err)
				reconnectAttempts++
				time.Sleep(time.Second * 2) // Wait before retrying
				continue
			}
			fmt.Println("Reconnected successfully")
			// Retry setting power mode for failed outputs
			for _, output := range failedOutputs {
				err := opm.SetOutputPowerMode(output, mode)
				if err != nil {
					fmt.Printf("Failed to set output power mode after reconnection: %v\n", err)
				}
			}
		}

		break
	}

	if reconnectAttempts == 3 {
		fmt.Println("Failed to reconnect after multiple attempts")
	}
}

func (opm *OutputPowerManager) SetOutputPowerMode(output *client.Output, mode wlroutput.OutputPowerV1Mode) error {
	opm.mu.Lock()
	defer opm.mu.Unlock()

	outputPower, ok := opm.outputs[output]
	if !ok {
		return fmt.Errorf("output not found")
	}
	return outputPower.SetMode(uint32(mode))
}

func (opm *OutputPowerManager) Close() {
	opm.mu.Lock()
	defer opm.mu.Unlock()

	for _, outputPower := range opm.outputs {
		outputPower.Destroy()
	}
	opm.manager.Destroy()
	opm.display.Context().Close()
}

func (opm *OutputPowerManager) reconnect() error {
	opm.Close()

	display, err := client.Connect("")
	if err != nil {
		return err
	}

	registry, err := display.GetRegistry()
	if err != nil {
		display.Context().Close()
		return err
	}

	opm.display = display
	opm.registry = registry
	opm.outputs = make(map[*client.Output]*wlroutput.OutputPowerV1)

	return opm.initialize()
}
