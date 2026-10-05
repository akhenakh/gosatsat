package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/akhenakh/sgp4"
)

const cacheVersion = 2

// tleCacheFile is the on-disk cache of the last successful download, stored
// next to the config so the app can render immediately while it refetches. It
// stores the parsed element sets (not raw TLE text) so catalog numbers above
// 99999, which have no valid TLE representation, survive a round trip.
type tleCacheFile struct {
	Version      int                   `json:"version"`
	SavedAt      time.Time             `json:"savedAt"`
	TLEs         []sgp4.TLE            `json:"tles"`
	Transponders map[int][]Transponder `json:"transponders,omitempty"`
}

func cachePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "gosatsat", "tle-cache.json"), nil
}

// saveTLECache atomically writes the merged element sets and transponders.
func saveTLECache(tles map[int]*sgp4.TLE, tps map[int][]Transponder) error {
	path, err := cachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	ids := make([]int, 0, len(tles))
	for id := range tles {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	entry := tleCacheFile{Version: cacheVersion, SavedAt: time.Now(), Transponders: tps, TLEs: make([]sgp4.TLE, 0, len(ids))}
	for _, id := range ids {
		if tle := tles[id]; tle != nil {
			entry.TLEs = append(entry.TLEs, *tle)
		}
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write cache: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace cache: %w", err)
	}
	return nil
}

// loadTLECache reads the cache. A missing or unusable cache returns nil maps
// and a nil error, so startup just proceeds to a network fetch.
func loadTLECache() (map[int]*sgp4.TLE, map[string]int, map[int][]Transponder, time.Time, error) {
	path, err := cachePath()
	if err != nil {
		return nil, nil, nil, time.Time{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil, time.Time{}, nil
	}
	if err != nil {
		return nil, nil, nil, time.Time{}, err
	}

	var entry tleCacheFile
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, nil, nil, time.Time{}, fmt.Errorf("parse cache: %w", err)
	}

	tles := make(map[int]*sgp4.TLE, len(entry.TLEs))
	names := make(map[string]int, len(entry.TLEs))
	for i := range entry.TLEs {
		tle := &entry.TLEs[i]
		if tle.SatelliteNumber == 0 {
			continue
		}
		tles[tle.SatelliteNumber] = tle
		if n := strings.ToUpper(strings.TrimSpace(tle.Name)); n != "" {
			names[n] = tle.SatelliteNumber
		}
	}
	if len(tles) == 0 {
		return nil, nil, nil, time.Time{}, nil
	}
	return tles, names, entry.Transponders, entry.SavedAt, nil
}
