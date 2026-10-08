package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/akhenakh/sgp4"
)

var httpClient = &http.Client{Timeout: 45 * time.Second}

const userAgent = "gosatsat/1.0 (+https://github.com/akhenakh/gosatsat)"

// fetchTLEs downloads and parses all orbital-data sources, merging the
// results. It understands CelesTrak's GP CSV and OMM JSON formats (which
// support catalog numbers above 99999) as well as legacy TLE text (e.g. AMSAT).
// Later sources win on a NORAD conflict; sources that fail are reported but do
// not invalidate the ones that succeeded.
func fetchTLEs(ctx context.Context, urls []string) (map[int]*sgp4.TLE, map[string]int, []string) {
	type result struct {
		tles        map[int]*sgp4.TLE
		nameToNorad map[string]int
		err         string
	}
	results := make([]result, len(urls))
	var wg sync.WaitGroup
	for i, u := range urls {
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			tles, names, err := fetchTLESource(ctx, u)
			if err != nil {
				results[i] = result{err: fmt.Sprintf("%s: %v", hostOf(u), err)}
				return
			}
			results[i] = result{tles: tles, nameToNorad: names}
		}(i, u)
	}
	wg.Wait()

	tles := make(map[int]*sgp4.TLE)
	nameToNorad := make(map[string]int)
	var errs []string
	for _, r := range results {
		if r.err != "" {
			errs = append(errs, r.err)
			continue
		}
		for id, tle := range r.tles {
			tles[id] = tle
			if name := strings.ToUpper(strings.TrimSpace(tle.Name)); name != "" {
				nameToNorad[name] = id
			}
		}
	}
	return tles, nameToNorad, errs
}

func fetchTLESource(ctx context.Context, url string) (map[int]*sgp4.TLE, map[string]int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return parseOrbitalData(data)
}

// parseOrbitalData detects the payload format and parses it. JSON OMM and GP
// CSV both carry numeric catalog ids, so they are not limited to 5 digits.
func parseOrbitalData(data []byte) (map[int]*sgp4.TLE, map[string]int, error) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if len(trimmed) == 0 {
		return nil, nil, fmt.Errorf("empty response")
	}
	switch {
	case trimmed[0] == '[' || trimmed[0] == '{':
		omms, err := sgp4.ParseOMMs(trimmed)
		if err != nil {
			return nil, nil, err
		}
		return ommsToTLEs(omms)
	case isGPCSVHeader(trimmed):
		omms, err := sgp4.ParseOMMsCSV(bytes.NewReader(trimmed))
		if err != nil {
			return nil, nil, err
		}
		return ommsToTLEs(omms)
	default:
		return parseTLEs(bytes.NewReader(trimmed))
	}
}

// isGPCSVHeader reports whether the first line is a CelesTrak GP CSV header.
func isGPCSVHeader(data []byte) bool {
	line := data
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		line = data[:i]
	}
	up := bytes.ToUpper(line)
	return bytes.Contains(up, []byte("OBJECT_NAME")) && bytes.Contains(up, []byte("NORAD_CAT_ID"))
}

// ommsToTLEs converts parsed OMM records to TLE element sets, keyed by catalog
// number. It relies on the library's ToTLE, which tolerates a missing or
// malformed object id.
func ommsToTLEs(omms []sgp4.OMM) (map[int]*sgp4.TLE, map[string]int, error) {
	tles := make(map[int]*sgp4.TLE, len(omms))
	names := make(map[string]int, len(omms))
	for i := range omms {
		tle, err := omms[i].ToTLE()
		if err != nil || tle.SatelliteNumber == 0 {
			continue
		}
		tles[tle.SatelliteNumber] = tle
		if n := strings.ToUpper(strings.TrimSpace(tle.Name)); n != "" {
			names[n] = tle.SatelliteNumber
		}
	}
	return tles, names, nil
}

// parseTLEs parses a stream of TLEs. It accepts the usual 3-line (name, 1, 2)
// format and tolerates 2-line blocks without a name.
func parseTLEs(r io.Reader) (map[int]*sgp4.TLE, map[string]int, error) {
	tles := make(map[int]*sgp4.TLE)
	names := make(map[string]int)

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var name, line1, line2 string
	flush := func() {
		if line1 == "" || line2 == "" {
			name, line1, line2 = "", "", ""
			return
		}
		lines := []string{line1, line2}
		if strings.TrimSpace(name) != "" {
			lines = []string{strings.TrimSpace(name), line1, line2}
		}
		tle, err := sgp4.ParseTLELines(lines)
		if err == nil && tle.SatelliteNumber != 0 {
			tles[tle.SatelliteNumber] = tle
			if n := strings.ToUpper(strings.TrimSpace(tle.Name)); n != "" {
				names[n] = tle.SatelliteNumber
			}
		}
		name, line1, line2 = "", "", ""
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flush()
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "1 ") && len(trimmed) >= 60:
			if line1 != "" {
				flush()
			}
			line1 = trimmed
		case strings.HasPrefix(trimmed, "2 ") && len(trimmed) >= 60:
			line2 = trimmed
			flush()
		default:
			if line1 != "" || line2 != "" {
				flush()
			}
			name = trimmed
		}
	}
	flush()
	if err := scanner.Err(); err != nil {
		return tles, names, err
	}
	return tles, names, nil
}

// fetchTransponders downloads and parses the JE9PEL semicolon-separated list.
// Only active transponders are kept, matching websat behavior.
func fetchTransponders(ctx context.Context, urls []string) (map[int][]Transponder, []string) {
	out := make(map[int][]Transponder)
	var errs []string
	for _, u := range urls {
		tps, err := fetchTransponderSource(ctx, u)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", hostOf(u), err))
			continue
		}
		for id, list := range tps {
			out[id] = append(out[id], list...)
		}
	}
	return out, errs
}

func fetchTransponderSource(ctx context.Context, url string) (map[int][]Transponder, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	reader := csv.NewReader(resp.Body)
	reader.Comma = ';'
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1

	local := make(map[int][]Transponder)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if len(record) < 8 {
			continue
		}
		norad, err := strconv.Atoi(strings.TrimSpace(record[1]))
		if err != nil {
			continue
		}
		tp := Transponder{
			SatelliteName: strings.TrimSpace(record[0]),
			NoradID:       norad,
			Uplink:        strings.TrimSpace(record[2]),
			Downlink:      strings.TrimSpace(record[3]),
			Beacon:        strings.TrimSpace(record[4]),
			Mode:          strings.TrimSpace(record[5]),
			Callsign:      strings.TrimSpace(record[6]),
			Status:        strings.TrimSpace(record[7]),
		}
		if !strings.EqualFold(tp.Status, "active") {
			continue
		}
		local[norad] = append(local[norad], tp)
	}
	return local, nil
}

func hostOf(rawURL string) string {
	s := rawURL
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	return s
}

// mergeSatellites folds fetched TLEs and transponders into the stable satellite
// store, reusing existing Sat objects so selection/row identity survives.
func (s *State) mergeSatellites(tles map[int]*sgp4.TLE, nameToNorad map[string]int, transponders map[int][]Transponder) {
	seen := make(map[int]bool, len(tles))
	for id, tle := range tles {
		seen[id] = true
		sat := s.sats[id]
		if sat == nil {
			sat = &Sat{NoradID: id}
			s.sats[id] = sat
		}
		sat.Name = strings.TrimSpace(tle.Name)
		sat.TLE = tle
		if tps, ok := transponders[id]; ok {
			sat.Transponders = tps
		}
	}
	// Drop sats that vanished from every source.
	for id := range s.sats {
		if !seen[id] {
			delete(s.sats, id)
		}
	}

	s.satOrder = s.satOrder[:0]
	for _, sat := range s.sats {
		s.satOrder = append(s.satOrder, sat)
	}
	sort.Slice(s.satOrder, func(i, j int) bool {
		return strings.ToUpper(s.satOrder[i].Name) < strings.ToUpper(s.satOrder[j].Name)
	})
}
