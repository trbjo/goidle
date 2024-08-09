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
	BacklightCurveFactor float64  `json:"backlight_curve_factor"`
	BacklightDimRatio    float64  `json:"backlight_dim_ratio"`
	BacklightSteps       int      `json:"backlight_steps"`
	IdleGraceDuration    Duration `json:"idle_grace_duration"`
	LockCommand          []string `json:"lock_command"`
	TrustedWifis         []string `json:"trusted_wifi_networks"`
	path                 string
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
		return &Config{
			BacklightCurveFactor: 0.5,
			BacklightDimRatio:    0.2,
			BacklightSteps:       16,
			IdleGraceDuration:    Duration{Duration: 30 * time.Second},
			LockCommand:          getDefaultLockCommand(),
			TrustedWifis:         []string{},
			path:                 configPath,
		}
	}

	// Set default values if not specified in the loaded config
	if config.BacklightSteps == 0 {
		config.BacklightSteps = 16
	}

	if config.BacklightDimRatio == 0 {
		config.BacklightDimRatio = 0.2
	}

	if config.BacklightCurveFactor == 0 {
		config.BacklightCurveFactor = 0.5
	}

	if len(config.LockCommand) == 0 {
		config.LockCommand = getDefaultLockCommand()
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
