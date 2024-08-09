package main

import (
	"encoding/json"
	"io"
	"os"
	"time"
)

type WifiManager struct {
	TrustedWifis []string `json:"trusted_wifis"`
}

func (wm *WifiManager) RemoveCurrent() {
	mac, err := ExtractMac()
	if err != nil {
		lg.Error(err.Error())
		return
	}
	var newlist []string
	for _, macAddress := range wm.TrustedWifis {
		if macAddress != mac {
			newlist = append(newlist, macAddress)
		}
	}
	wm.TrustedWifis = newlist
}

func (wm *WifiManager) AddCurrent() {
	mac, err := ExtractMac()
	if err != nil {
		lg.Error(err.Error())
		return
	}
	for _, macAddress := range wm.TrustedWifis {
		if macAddress == mac {
			return
		}
	}
	wm.TrustedWifis = append(wm.TrustedWifis, mac)
	lg.Debug("successfully added wifi")
}

type Duration struct {
	time.Duration
}

func (d Duration) MarshalJSON() ([]byte, error) {
	durationString := d.Duration.String()
	return json.Marshal(durationString)
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
	BacklightCurveFactor float64      `json:"backlight_curve_factor"`
	BacklightDimRatio    float64      `json:"backlight_dim_ratio"`
	BacklightSteps       int          `json:"backlight_steps"`
	IdleGracePeriod      Duration     `json:"idle_grace_period"`
	WifiManager          *WifiManager `json:"trusted_wifi_networks"`
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
			IdleGracePeriod:      Duration{Duration: 30 * time.Second},
			WifiManager:          &WifiManager{TrustedWifis: []string{}},
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

	return config
}

func DumpConfig(path string, config *Config) {
	lg.Debug("dumping config to disk")
	jsonData, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		lg.Error(err.Error())
		return
	}
	file, err := os.Create(path)
	if err != nil {
		lg.Error(err.Error())
		return
	}
	_, err = file.Write(jsonData)
	if err != nil {
		lg.Error(err.Error())
		return
	}
	lg.Debug("wrote config")
}
