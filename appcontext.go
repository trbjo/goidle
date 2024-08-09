package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"time"
)

type Duration struct {
	time.Duration
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration.String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	duration, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = duration
	return nil
}

type Config struct {
	BacklightCurveFactor       float64  `json:"backlight_curve_factor"`
	BacklightDimRatio          float64  `json:"backlight_dim_ratio"`
	BacklightSteps             int      `json:"backlight_steps"`
	IdleGraceDuration          Duration `json:"idle_grace_duration"`
	LockCommand                []string `json:"lock_command"`
	LockInitIgnoreInputTimeout Duration `json:"lock_init_ignore_input_timeout"`
	TimeoutActiveDim           Duration `json:"timeout_active_dim"`
	TimeoutActiveToIdle        Duration `json:"timeout_active_to_idle"`
	TimeoutIdleBacklightOff    Duration `json:"timeout_idle_backlight_off"`
	TimeoutIdleToSuspend       Duration `json:"timeout_idle_to_suspend"`
	SuspendCommand             []string `json:"suspend_command"`
	TrustedWifis               []string `json:"trusted_wifi_networks"`

	path string
}

func (c *Config) RemoveCurrentWifi() {
	mac, err := ExtractMac()
	if err != nil {
		lg.Error(err.Error())
		return
	}
	var newlist []string
	for _, macAddress := range c.TrustedWifis {
		if macAddress != mac {
			newlist = append(newlist, macAddress)
		}
	}
	c.TrustedWifis = newlist
}

func (c *Config) AddCurrentWifi() {
	mac, err := ExtractMac()
	if err != nil {
		lg.Error(err.Error())
		return
	}
	for _, macAddress := range c.TrustedWifis {
		if macAddress == mac {
			return
		}
	}
	c.TrustedWifis = append(c.TrustedWifis, mac)
	lg.Debug("successfully added wifi")
}

func loadConfigFromFile(configPath string) (*Config, error) {
	_, err := os.Stat(configPath)
	if err != nil {
		lg.Error("file does not exist", "error", err.Error())
		return nil, err
	}
	var config Config
	jsonFile, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(byteValue, &config)
	if err != nil {
		return nil, err
	}

	config.path = configPath
	return &config, nil
}

func initConfig(configPath string) *Config {
	config, err := loadConfigFromFile(configPath)
	if err != nil {
		lg.Info("Failed to load config, creating a new one")
		config = &Config{
			IdleGraceDuration:       Duration{Duration: 30 * time.Second},
			TrustedWifis:            []string{},
			path:                    configPath,
		}
	}

	// Set default values if not specified in the loaded config
	if config.BacklightCurveFactor == 0 {
		config.BacklightCurveFactor = 0.5
	}

	if config.BacklightDimRatio == 0 {
		config.BacklightDimRatio = 0.2
	}

	if config.BacklightSteps == 0 {
		config.BacklightSteps = 16
	}

	if config.TimeoutActiveDim.Duration == 0 {
		config.TimeoutActiveDim = Duration{Duration: 150 * time.Second}
		lg.Info("timeout_active_dim not set, using 150s")
	}

	if config.TimeoutActiveToIdle.Duration == 0 {
		config.TimeoutActiveToIdle = Duration{Duration: config.TimeoutActiveDim.Duration + 30*time.Second}
		lg.Info("timeout_active_to_idle not set, using timeout_active_dim + 30s",
			"value", config.TimeoutActiveToIdle.Duration.String())
	}

	if config.TimeoutIdleBacklightOff.Duration == 0 {
		config.TimeoutIdleBacklightOff = Duration{Duration: 15 * time.Second}
		lg.Info("timeout_idle_backlight_off not set, using 15s")
	}

	if config.TimeoutIdleToSuspend.Duration == 0 {
		config.TimeoutIdleToSuspend = Duration{Duration: config.TimeoutIdleBacklightOff.Duration + 5*time.Second}
		lg.Info("timeout_idle_to_suspend not set, using timeout_idle_backlight_off + 5s",
			"value", config.TimeoutIdleToSuspend.Duration.String())
	}

	if len(config.LockCommand) == 0 {
		config.LockCommand = getDefaultLockCommand()
	}

	if config.LockInitIgnoreInputTimeout.Duration == 0 {
		config.LockInitIgnoreInputTimeout = Duration{Duration: 1 * time.Second}
	}

	return config
}

func getDefaultLockCommand() []string {
	if _, err := exec.LookPath("hyprlock"); err == nil {
		return []string{"hyprlock"}
	}
	if _, err := exec.LookPath("swaylock"); err == nil {
		return []string{"swaylock"}
	}
	if _, err := exec.LookPath("waylock"); err == nil {
		return []string{"waylock"}
	}
	panic("No screenlocker found, not starting")
}

func (c *Config) Dump() {
	lg.Debug("dumping config to disk")
	jsonData, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		lg.Error(err.Error())
		return
	}
	file, err := os.Create(c.path)
	if err != nil {
		lg.Error(err.Error())
		return
	}
	defer file.Close()
	_, err = file.Write(jsonData)
	if err != nil {
		lg.Error(err.Error())
		return
	}
	lg.Debug("wrote config")
}
