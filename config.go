package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const configVersion = 1

const (
	defaultMinElevation = 10.0
	defaultRefreshHours = 6
	// minRefreshHours keeps the periodic refresh within CelesTrak's request
	// policy (no more than one General Perturbations query every 2 hours).
	minRefreshHours = 2
)

// LocationConfig is the observer location persisted in the config file.
type LocationConfig struct {
	Name string  `json:"name,omitempty"`
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
	Alt  float64 `json:"alt"` // meters above sea level
}

// Config is the persistent application configuration, stored as JSON in the
// user config directory (XDG on Linux).
type Config struct {
	Version            int             `json:"version"`
	Location           *LocationConfig `json:"location,omitempty"`
	MinElevation       float64         `json:"minElevation"`
	TrackedSats        []int           `json:"trackedSats"`
	TleSources         []string        `json:"tleSources"`
	TransponderSources []string        `json:"transponderSources"`
	RefreshHours       int             `json:"refreshHours"`
	DarkMode           bool            `json:"darkMode"`
	// Notifications is a pointer so a missing field can default to enabled.
	Notifications *bool `json:"notifications,omitempty"`
}

// notifyEnabled reports whether OS notifications for upcoming passes are on.
func (c Config) notifyEnabled() bool { return c.Notifications == nil || *c.Notifications }

func boolPtr(v bool) *bool { return &v }

func defaultConfig() Config {
	return Config{
		Version:      configVersion,
		MinElevation: defaultMinElevation,
		RefreshHours: defaultRefreshHours,
		TleSources: []string{
			// GP in CSV (OMM fields) supports 6- and 7-digit catalog numbers;
			// the legacy TLE format cannot represent them.
			"https://celestrak.org/NORAD/elements/gp.php?GROUP=active&FORMAT=csv",
			"https://www.amsat.org/tle/dailytle.txt",
		},
		TransponderSources: []string{
			"http://www.ne.jp/asahi/hamradio/je9pel/satslist.csv",
		},
		Notifications: boolPtr(true),
	}
}

// normalize fills in defaults for fields a hand-edited or older config may miss.
func (c *Config) normalize() {
	def := defaultConfig()
	if c.Version == 0 {
		c.Version = def.Version
	}
	if c.MinElevation == 0 {
		c.MinElevation = def.MinElevation
	}
	if c.RefreshHours <= 0 {
		c.RefreshHours = def.RefreshHours
	}
	if c.RefreshHours < minRefreshHours {
		c.RefreshHours = minRefreshHours
	}
	if len(c.TleSources) == 0 {
		c.TleSources = def.TleSources
	}
	// Migrate the old CelesTrak TLE endpoint to the CSV (OMM) format, which
	// supports catalog numbers above 99999 and newer epochs.
	for i, src := range c.TleSources {
		c.TleSources[i] = migrateTleSource(src)
	}
	if len(c.TransponderSources) == 0 {
		c.TransponderSources = def.TransponderSources
	}
	if c.Notifications == nil {
		c.Notifications = def.Notifications
	}
}

// migrateTleSource rewrites the legacy CelesTrak TLE format query parameter to
// the CSV format so existing configs keep working with 6-digit catalog numbers.
func migrateTleSource(src string) string {
	low := strings.ToLower(src)
	if !strings.Contains(low, "celestrak.org") {
		return src
	}
	if !strings.Contains(low, "format=tle") && !strings.Contains(low, "format=3le") && !strings.Contains(low, "format=2le") {
		return src
	}
	r := strings.NewReplacer(
		"FORMAT=tle", "FORMAT=csv", "format=tle", "format=csv",
		"FORMAT=3LE", "FORMAT=csv", "format=3le", "format=csv",
		"FORMAT=2LE", "FORMAT=csv", "format=2le", "format=csv",
	)
	return r.Replace(src)
}

// configPath returns the path of the JSON config file inside the user config dir.
func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "gosatsat", "config.json"), nil
}

// loadConfig reads the config file. A missing file returns a default config and
// needsOnboarding=true. A corrupt file is backed up and defaults are returned,
// together with a non-nil warning for display in the UI.
func loadConfig() (cfg Config, needsOnboarding bool, warning error) {
	path, err := configPath()
	if err != nil {
		return defaultConfig(), true, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultConfig(), true, nil
	}
	if err != nil {
		return defaultConfig(), true, fmt.Errorf("read config: %w", err)
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		_ = os.Rename(path, path+".bak")
		return defaultConfig(), true, fmt.Errorf("config was invalid (%v); a backup was saved to config.json.bak", err)
	}
	c.normalize()
	needsOnboarding = c.Location == nil
	return c, needsOnboarding, nil
}

// saveConfig writes the config atomically (temp file + rename).
func saveConfig(c Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	c.Version = configVersion
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}
