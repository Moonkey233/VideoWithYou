package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultPublicPort = 21314
	ProfileVersion    = 2
)

type MPCCommands struct {
	PlayPause string `json:"play_pause"`
	Play      string `json:"play"`
	Pause     string `json:"pause"`
	RateUp    string `json:"rate_up"`
	RateDown  string `json:"rate_down"`
	Seek      string `json:"seek"`
	SetRate   string `json:"set_rate"`
}

type MPCConfig struct {
	BaseURL       string      `json:"base_url"`
	Username      string      `json:"username"`
	Password      string      `json:"password"`
	VariablesPath string      `json:"variables_path"`
	Commands      MPCCommands `json:"commands"`
	TimeoutMS     int64       `json:"timeout_ms"`
}

type ConnectionConfig struct {
	DirectURL        string `json:"direct_url"`
	CloudDialAddress string `json:"cloud_dial_address"`
	TLSCAPEM         string `json:"tls_ca_pem,omitempty"`
	DirectTimeoutMS  int64  `json:"direct_timeout_ms"`
	CloudTimeoutMS   int64  `json:"cloud_timeout_ms"`
	RetryDelayMS     int64  `json:"retry_delay_ms"`
	AccessToken      string `json:"access_token"`
	ClientInstanceID string `json:"client_instance_id"`
	SessionToken     string `json:"session_token"`
}

type TLSConfig struct {
	Mode        string `json:"mode"`
	Domain      string `json:"domain"`
	Email       string `json:"email"`
	CacheDir    string `json:"cache_dir"`
	HTTPAddress string `json:"http_address"`
	CAFile      string `json:"ca_file,omitempty"`
	CAKeyFile   string `json:"ca_key_file,omitempty"`
	CertFile    string `json:"cert_file"`
	KeyFile     string `json:"key_file"`
}

type ServerConfig struct {
	Enabled            bool      `json:"enabled"`
	ListenAddress      string    `json:"listen_address"`
	Path               string    `json:"path"`
	AccessToken        string    `json:"access_token"`
	ReconnectGraceSec  int64     `json:"reconnect_grace_sec"`
	HostIdleTimeoutSec int64     `json:"host_idle_timeout_sec"`
	TLS                TLSConfig `json:"tls"`
}

type RelayConfig struct {
	Enabled             bool   `json:"enabled"`
	SSHAddress          string `json:"ssh_address"`
	SSHUser             string `json:"ssh_user"`
	PrivateKeyPath      string `json:"private_key_path"`
	HostKeyPinPath      string `json:"host_key_pin_path"`
	RemoteListenAddress string `json:"remote_listen_address"`
	ReconnectDelaySec   int64  `json:"reconnect_delay_sec"`
}

type Config struct {
	// ServerURL remains for importing v2 configurations. V3 connection selection
	// is controlled by Connection.
	ServerURL                  string           `json:"server_url,omitempty"`
	Connection                 ConnectionConfig `json:"connection"`
	Server                     ServerConfig     `json:"server"`
	Relay                      RelayConfig      `json:"relay"`
	DisplayName                string           `json:"display_name"`
	ExtListenAddr              string           `json:"ext_listen_addr"`
	ExtListenPath              string           `json:"ext_listen_path"`
	ExtIdleTimeoutSec          int64            `json:"ext_idle_timeout_sec"`
	EndpointInactiveTimeoutSec int64            `json:"endpoint_inactive_timeout_sec"`
	Endpoint                   string           `json:"endpoint"`
	FollowURL                  bool             `json:"follow_url"`
	TickMS                     int64            `json:"tick_ms"`
	HardSeekThresholdMS        int64            `json:"hard_seek_threshold_ms"`
	DeadzoneMS                 int64            `json:"deadzone_ms"`
	SoftRateEnabled            bool             `json:"soft_rate_enabled"`
	SoftRateThresholdMS        int64            `json:"soft_rate_threshold_ms"`
	SoftRateAdjust             float64          `json:"soft_rate_adjust"`
	SoftRateMaxMS              int64            `json:"soft_rate_max_ms"`
	OffsetMS                   int64            `json:"offset_ms"`
	TimeSyncIntervalSec        int64            `json:"time_sync_interval_sec"`
	MPC                        MPCConfig        `json:"mpc"`
}

type ClientProfile struct {
	Version          int    `json:"version"`
	DirectURL        string `json:"direct_url"`
	CloudDialAddress string `json:"cloud_dial_address"`
	AccessToken      string `json:"access_token"`
	TLSCAPEM         string `json:"tls_ca_pem"`
}

func DefaultConfig() Config {
	publicURL := fmt.Sprintf("wss://ipv6.moonkey.top:%d/ws", DefaultPublicPort)
	return Config{
		ServerURL: publicURL,
		Connection: ConnectionConfig{
			DirectURL:        publicURL,
			CloudDialAddress: fmt.Sprintf("moonkey.top:%d", DefaultPublicPort),
			DirectTimeoutMS:  3000,
			CloudTimeoutMS:   5000,
			RetryDelayMS:     2000,
		},
		Server: ServerConfig{
			Enabled:            false,
			ListenAddress:      fmt.Sprintf("[::]:%d", DefaultPublicPort),
			Path:               "/ws",
			ReconnectGraceSec:  30,
			HostIdleTimeoutSec: 600,
			TLS: TLSConfig{
				Mode:   "local_ca",
				Domain: "ipv6.moonkey.top",
			},
		},
		Relay: RelayConfig{
			Enabled:             false,
			SSHAddress:          "moonkey.top:22",
			SSHUser:             "videowithyou",
			RemoteListenAddress: fmt.Sprintf("0.0.0.0:%d", DefaultPublicPort),
			ReconnectDelaySec:   3,
		},
		DisplayName:                "",
		ExtListenAddr:              "127.0.0.1:23333",
		ExtListenPath:              "/ext",
		ExtIdleTimeoutSec:          30,
		EndpointInactiveTimeoutSec: 600,
		Endpoint:                   "browser",
		FollowURL:                  true,
		TickMS:                     500,
		HardSeekThresholdMS:        2000,
		DeadzoneMS:                 1000,
		SoftRateEnabled:            true,
		SoftRateThresholdMS:        1500,
		SoftRateAdjust:             0.02,
		SoftRateMaxMS:              2000,
		OffsetMS:                   0,
		TimeSyncIntervalSec:        600,
		MPC: MPCConfig{
			BaseURL:       "http://127.0.0.1:13579",
			VariablesPath: "/variables.html",
			Commands: MPCCommands{
				PlayPause: "POST /command.html|wm_command=889&null=0",
				Play:      "POST /command.html|wm_command=887&null=0",
				Pause:     "POST /command.html|wm_command=888&null=0",
				RateUp:    "POST /command.html|wm_command=895&null=0",
				RateDown:  "POST /command.html|wm_command=894&null=0",
				Seek:      "POST /command.html|wm_command=-1&position={hhmmss}",
			},
			TimeoutMS: 800,
		},
	}
}

func RuntimeDir() (string, error) {
	if localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); localAppData != "" {
		return filepath.Join(localAppData, "VideoWithYou"), nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "VideoWithYou"), nil
}

func DefaultConfigPath() (string, error) {
	root, err := RuntimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "config.json"), nil
}

func DefaultLogDir() (string, error) {
	root, err := RuntimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "logs"), nil
}

func DefaultExtensionDir() (string, error) {
	root, err := RuntimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "extension"), nil
}

func LoadConfig(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		return Config{}, errors.New("config path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			if err := cfg.EnsureIdentity(); err != nil {
				return cfg, err
			}
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
	normalize(&cfg)
	if err := cfg.EnsureIdentity(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func normalize(cfg *Config) {
	defaults := DefaultConfig()
	if cfg.Connection.DirectURL == "" {
		cfg.Connection.DirectURL = defaults.Connection.DirectURL
	}
	if cfg.Connection.CloudDialAddress == "" {
		cfg.Connection.CloudDialAddress = defaults.Connection.CloudDialAddress
	}
	if cfg.Connection.DirectTimeoutMS <= 0 {
		cfg.Connection.DirectTimeoutMS = defaults.Connection.DirectTimeoutMS
	}
	if cfg.Connection.CloudTimeoutMS <= 0 {
		cfg.Connection.CloudTimeoutMS = defaults.Connection.CloudTimeoutMS
	}
	if cfg.Connection.RetryDelayMS <= 0 {
		cfg.Connection.RetryDelayMS = defaults.Connection.RetryDelayMS
	}
	if cfg.Server.ListenAddress == "" {
		cfg.Server.ListenAddress = defaults.Server.ListenAddress
	}
	if cfg.Server.Path == "" {
		cfg.Server.Path = defaults.Server.Path
	}
	if cfg.Server.ReconnectGraceSec <= 0 {
		cfg.Server.ReconnectGraceSec = defaults.Server.ReconnectGraceSec
	}
	if cfg.Server.HostIdleTimeoutSec <= 0 {
		cfg.Server.HostIdleTimeoutSec = defaults.Server.HostIdleTimeoutSec
	}
	if cfg.Server.TLS.Mode == "" {
		cfg.Server.TLS.Mode = defaults.Server.TLS.Mode
	}
	if cfg.Server.TLS.Domain == "" {
		cfg.Server.TLS.Domain = defaults.Server.TLS.Domain
	}
	if cfg.Server.TLS.HTTPAddress == "" {
		cfg.Server.TLS.HTTPAddress = defaults.Server.TLS.HTTPAddress
	}
	if cfg.Relay.SSHAddress == "" {
		cfg.Relay.SSHAddress = defaults.Relay.SSHAddress
	}
	if cfg.Relay.SSHUser == "" {
		cfg.Relay.SSHUser = defaults.Relay.SSHUser
	}
	if cfg.Relay.RemoteListenAddress == "" {
		cfg.Relay.RemoteListenAddress = defaults.Relay.RemoteListenAddress
	}
	if cfg.Relay.ReconnectDelaySec <= 0 {
		cfg.Relay.ReconnectDelaySec = defaults.Relay.ReconnectDelaySec
	}
	if cfg.ExtListenAddr == "" {
		cfg.ExtListenAddr = defaults.ExtListenAddr
	}
	if cfg.ExtListenPath == "" {
		cfg.ExtListenPath = defaults.ExtListenPath
	}
}

func (cfg *Config) EnsureIdentity() error {
	var err error
	if cfg.Connection.ClientInstanceID == "" {
		cfg.Connection.ClientInstanceID, err = randomToken(18)
		if err != nil {
			return err
		}
	}
	if cfg.Connection.SessionToken == "" {
		cfg.Connection.SessionToken, err = randomToken(32)
		if err != nil {
			return err
		}
	}
	return nil
}

func (cfg *Config) EnableOwnerMode(runtimeDir string) error {
	if err := cfg.EnsureIdentity(); err != nil {
		return err
	}
	cfg.Server.Enabled = true
	cfg.Relay.Enabled = true
	if cfg.Server.AccessToken == "" {
		token, err := randomToken(32)
		if err != nil {
			return err
		}
		cfg.Server.AccessToken = token
	}
	cfg.Connection.AccessToken = cfg.Server.AccessToken
	if cfg.Relay.PrivateKeyPath == "" {
		cfg.Relay.PrivateKeyPath = filepath.Join(runtimeDir, "ssh", "id_ed25519")
	}
	if cfg.Relay.HostKeyPinPath == "" {
		cfg.Relay.HostKeyPinPath = filepath.Join(runtimeDir, "ssh", "host_key.pin")
	}
	if cfg.Server.TLS.CacheDir == "" {
		cfg.Server.TLS.CacheDir = filepath.Join(runtimeDir, "certs")
	}
	if cfg.Server.TLS.CAFile == "" {
		cfg.Server.TLS.CAFile = filepath.Join(cfg.Server.TLS.CacheDir, "owner-ca.pem")
	}
	if cfg.Server.TLS.CAKeyFile == "" {
		cfg.Server.TLS.CAKeyFile = filepath.Join(cfg.Server.TLS.CacheDir, "owner-ca-key.pem")
	}
	if cfg.Server.TLS.CertFile == "" {
		cfg.Server.TLS.CertFile = filepath.Join(cfg.Server.TLS.CacheDir, "server.pem")
	}
	if cfg.Server.TLS.KeyFile == "" {
		cfg.Server.TLS.KeyFile = filepath.Join(cfg.Server.TLS.CacheDir, "server-key.pem")
	}
	return nil
}

func (cfg Config) ClientProfile() ClientProfile {
	return ClientProfile{
		Version:          ProfileVersion,
		DirectURL:        cfg.Connection.DirectURL,
		CloudDialAddress: cfg.Connection.CloudDialAddress,
		AccessToken:      cfg.Server.AccessToken,
		TLSCAPEM:         cfg.Connection.TLSCAPEM,
	}
}

func (cfg *Config) ApplyProfile(profile ClientProfile) error {
	if profile.Version != ProfileVersion {
		return fmt.Errorf("unsupported profile version %d", profile.Version)
	}
	if strings.TrimSpace(profile.DirectURL) == "" || strings.TrimSpace(profile.CloudDialAddress) == "" {
		return errors.New("profile is missing connection endpoints")
	}
	if strings.TrimSpace(profile.TLSCAPEM) == "" {
		return errors.New("profile is missing the owner CA certificate")
	}
	cfg.Connection.DirectURL = profile.DirectURL
	cfg.Connection.CloudDialAddress = profile.CloudDialAddress
	cfg.Connection.AccessToken = profile.AccessToken
	cfg.Connection.TLSCAPEM = profile.TLSCAPEM
	cfg.Server.Enabled = false
	cfg.Relay.Enabled = false
	cfg.Server.AccessToken = ""
	cfg.Connection.ClientInstanceID = ""
	cfg.Connection.SessionToken = ""
	return cfg.EnsureIdentity()
}

func LoadProfile(path string) (ClientProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ClientProfile{}, err
	}
	var profile ClientProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return ClientProfile{}, err
	}
	return profile, nil
}

func SaveProfile(path string, profile ClientProfile) error {
	return writeJSON(path, profile, 0o600)
}

func SaveConfig(path string, cfg Config) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("config path is empty")
	}
	return writeJSON(path, cfg, 0o600)
}

func writeJSON(path string, value any, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, perm)
}

func randomToken(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
