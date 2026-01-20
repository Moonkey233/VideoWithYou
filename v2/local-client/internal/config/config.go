package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type PotPlayerHotkeys struct {
	PlayPause string `json:"play_pause"`
	RateUp    string `json:"rate_up"`
	RateDown  string `json:"rate_down"`
}

type PotPlayerConfig struct {
	Path    string           `json:"path"`
	Hotkeys PotPlayerHotkeys `json:"hotkeys"`
}

type Config struct {
	ServerURL           string          `json:"server_url"`
	DisplayName         string          `json:"display_name"`
	ExtListenAddr       string          `json:"ext_listen_addr"`
	ExtListenPath       string          `json:"ext_listen_path"`
	ExtIdleTimeoutSec   int64           `json:"ext_idle_timeout_sec"`
	KeepRoomOnIdle      bool            `json:"keep_room_on_idle"`
	HostIdleReportSec   int64           `json:"host_idle_report_sec"`
	Endpoint            string          `json:"endpoint"`
	FollowURL           bool            `json:"follow_url"`
	TickMS              int64           `json:"tick_ms"`
	HardSeekThresholdMS int64           `json:"hard_seek_threshold_ms"`
	DeadzoneMS          int64           `json:"deadzone_ms"`
	SoftRateEnabled     bool            `json:"soft_rate_enabled"`
	SoftRateThresholdMS int64           `json:"soft_rate_threshold_ms"`
	SoftRateAdjust      float64         `json:"soft_rate_adjust"`
	SoftRateMaxMS       int64           `json:"soft_rate_max_ms"`
	OffsetMS            int64           `json:"offset_ms"`
	TimeSyncIntervalSec int64           `json:"time_sync_interval_sec"`
	PotPlayer           PotPlayerConfig `json:"potplayer"`
}

func DefaultConfig() Config {
	return Config{
		ServerURL:           "ws://127.0.0.1:2333/ws",
		DisplayName:         "",
		ExtListenAddr:       "127.0.0.1:27111",
		ExtListenPath:       "/ext",
		ExtIdleTimeoutSec:   30,
		KeepRoomOnIdle:      true,
		HostIdleReportSec:   180,
		Endpoint:            "browser",
		FollowURL:           true,
		TickMS:              500,
		HardSeekThresholdMS: 1000,
		DeadzoneMS:          200,
		SoftRateEnabled:     true,
		SoftRateThresholdMS: 600,
		SoftRateAdjust:      0.02,
		SoftRateMaxMS:       3000,
		OffsetMS:            0,
		TimeSyncIntervalSec: 600,
		PotPlayer: PotPlayerConfig{
			Path: "",
			Hotkeys: PotPlayerHotkeys{
				PlayPause: "SPACE",
				RateUp:    "CTRL+UP",
				RateDown:  "CTRL+DOWN",
			},
		},
	}
}

func LoadConfig(path string) (Config, error) {
	if path == "" {
		return Config{}, errors.New("config path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			if err := SaveConfig(path, cfg); err != nil {
				return cfg, err
			}
			return cfg, nil
		}
		return Config{}, err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func SaveConfig(path string, cfg Config) error {
	if path == "" {
		return errors.New("config path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
