package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rajveermalviya/go-wayland/wayland/client"
	"github.com/trbjo/goidle/wlroutput"
)

type OutputPowerManager struct {
	display  *client.Display
	registry *client.Registry
	manager  *wlroutput.OutputPowerManagerV1
	outputs  map[string]*outputInfo
	mu	   sync.Mutex
	running  bool
	stopCh   chan struct{}
}

type outputInfo struct {
	output *client.Output
	power  *wlroutput.OutputPowerV1
	name   string
	mode   wlroutput.OutputPowerV1Mode
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
		outputs:  make(map[string]*outputInfo),
	}

	if err := opm.initialize(); err != nil {
		display.Context().Close()
		return nil, err
	}

	opm.StartEventLoop()

	return opm, nil
}

func (opm *OutputPowerManager) initialize() error {
	var managerName, managerVersion uint32
	pendingOutputs := make([]*client.Output, 0)

	opm.registry.SetGlobalHandler(func(e client.RegistryGlobalEvent) {
		switch e.Interface {
		case "zwlr_output_power_manager_v1":
			managerName = e.Name
			managerVersion = e.Version
			opm.manager = wlroutput.NewOutputPowerManagerV1(opm.display.Context())
			if err := opm.registry.Bind(managerName, "zwlr_output_power_manager_v1", managerVersion, opm.manager); err != nil {
				fmt.Printf("Failed to bind output power manager: %v\n", err)
				return
			}
			// Set up any pending outputs now that the manager is initialized
			for _, output := range pendingOutputs {
				opm.setupOutput(output)
			}
			pendingOutputs = nil // Clear the pending outputs
		case "wl_output":
			output := client.NewOutput(opm.display.Context())
			if err := opm.registry.Bind(e.Name, e.Interface, e.Version, output); err != nil {
				fmt.Printf("Failed to bind output: %v\n", err)
				return
			}
			if opm.manager != nil {
				// If the manager is already initialized, set up the output immediately
				opm.setupOutput(output)
			} else {
				// Otherwise, add it to the pending outputs
				pendingOutputs = append(pendingOutputs, output)
			}
		}
	})

	// Perform roundtrips to ensure all global handlers are called
	opm.displayRoundTrip()
	opm.displayRoundTrip()

	if managerName == 0 {
		return fmt.Errorf("failed to find zwlr_output_power_manager_v1 interface")
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

func (opm *OutputPowerManager) setupOutput(output *client.Output) {

	outputPower, err := opm.manager.GetOutputPower(output)
	if err != nil {
		fmt.Printf("Failed to get output power: %v\n", err)
		return
	}

	info := &outputInfo{
		output: output,
		power:  outputPower,
	}

	output.SetNameHandler(func(e client.OutputNameEvent) {
		opm.mu.Lock()
		defer opm.mu.Unlock()

		info.name = e.Name
		opm.outputs[e.Name] = info
		lg.Debug(fmt.Sprintf("Added output: %s", e.Name))
	})

	outputPower.SetModeHandler(func(e wlroutput.OutputPowerV1ModeEvent) {
		opm.mu.Lock()
		defer opm.mu.Unlock()

		info.mode = wlroutput.OutputPowerV1Mode(e.Mode)
		lg.Debug(fmt.Sprintf("Output power mode changed for %s: %s", info.name, info.mode))
	})

	outputPower.SetFailedHandler(func(e wlroutput.OutputPowerV1FailedEvent) {
		lg.Debug(fmt.Sprintf("Output power failed for %s", info.name))
		opm.removeOutput(info.name)
	})
}

func (opm *OutputPowerManager) removeOutput(name string) {
	opm.mu.Lock()
	defer opm.mu.Unlock()

	if info, ok := opm.outputs[name]; ok {
		info.power.Destroy()
		delete(opm.outputs, name)
	}
}

func (opm *OutputPowerManager) NumOutputs() int {
	opm.mu.Lock()
	defer opm.mu.Unlock()
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
		var failedOutputs []string
		opm.mu.Lock()
		for name, info := range opm.outputs {
			err := info.power.SetMode(uint32(mode))
			if err != nil {
				if strings.Contains(err.Error(), "broken pipe") {
					failedOutputs = append(failedOutputs, name)
				} else {
					fmt.Printf("Failed to set output power mode for %s: %v\n", name, err)
				}
			}
			// The actual mode change will be confirmed by the SetModeHandler
		}
		opm.mu.Unlock()

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
			opm.mu.Lock()
			for _, name := range failedOutputs {
				if info, ok := opm.outputs[name]; ok {
					err := info.power.SetMode(uint32(mode))
					if err != nil {
						fmt.Printf("Failed to set output power mode after reconnection for %s: %v\n", name, err)
					}
				}
			}
			opm.mu.Unlock()
		}

		break
	}

	if reconnectAttempts == 3 {
		fmt.Println("Failed to reconnect after multiple attempts")
	}
}

func (opm *OutputPowerManager) ToggleOutput(name string) error {
	opm.mu.Lock()
	defer opm.mu.Unlock()

	info, ok := opm.outputs[name]
	if !ok {
		return fmt.Errorf("output %s not found", name)
	}

	var newMode wlroutput.OutputPowerV1Mode
	if info.mode == wlroutput.OutputPowerV1ModeOn {
		newMode = wlroutput.OutputPowerV1ModeOff
	} else {
		newMode = wlroutput.OutputPowerV1ModeOn
	}

	err := info.power.SetMode(uint32(newMode))
	if err != nil {
		return fmt.Errorf("failed to set output power mode for %s: %w", name, err)
	}

	// The actual mode change will be confirmed by the SetModeHandler

	return nil
}

func (opm *OutputPowerManager) ListOutputNames() []string {
	opm.mu.Lock()
	defer opm.mu.Unlock()

	names := make([]string, 0, len(opm.outputs))
	for name := range opm.outputs {
		names = append(names, name)
	}
	return names
}

func (opm *OutputPowerManager) StartEventLoop() {
	opm.mu.Lock()
	if opm.running {
		opm.mu.Unlock()
		return
	}
	opm.running = true
	opm.stopCh = make(chan struct{})
	opm.mu.Unlock()

	go func() {
		for {
			select {
			case <-opm.stopCh:
				return
			default:
				opm.display.Context().Dispatch()
			}
		}
	}()
}

func (opm *OutputPowerManager) StopEventLoop() {
	opm.mu.Lock()
	defer opm.mu.Unlock()
	if !opm.running {
		return
	}
	close(opm.stopCh)
	opm.running = false
}



func (opm *OutputPowerManager) Close() {
	opm.StopEventLoop()

	opm.mu.Lock()
	defer opm.mu.Unlock()

	for _, info := range opm.outputs {
		info.power.Destroy()
	}
	if opm.manager != nil {
		opm.manager.Destroy()
	}
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
	opm.outputs = make(map[string]*outputInfo)

	return opm.initialize()
}
