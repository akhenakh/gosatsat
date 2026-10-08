package main

import (
	_ "embed"
	"encoding/csv"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

//go:embed cities.csv
var cityData string

// City is a selectable reference location.
type City struct {
	Name    string
	Country string
	Lat     float64
	Lng     float64
	Alt     float64 // meters above sea level, from the 30 m GEDTM terrain model
}

// cities is the embedded world city list (23k entries), sorted by country then
// name so browsing with an empty query is geographic. Names repeat across
// countries, so every picker shows the country.
var cities = parseCities(cityData)

// cityKeys is the accent-folded, lowercased "name, country" of each city,
// parallel to cities, so the per-frame search allocates nothing. The list is
// full of accented names (Zürich, A Coruña), so an ASCII query must match them.
var cityKeys = citySearchKeys(cities)

// maxCityMatches is how many cities the pickers offer for a query.
const maxCityMatches = 12

// foldAccents lowercases s and strips combining marks, so "Zürich" matches the
// query "zurich". Display names keep their accents.
func foldAccents(s string) string {
	t := transform.Chain(norm.NFD, transform.RemoveFunc(unicode.IsMark))
	folded, _, err := transform.String(t, s)
	if err != nil {
		return strings.ToLower(s)
	}
	return strings.ToLower(folded)
}

func parseCities(data string) []City {
	records, err := csv.NewReader(strings.NewReader(data)).ReadAll()
	if err != nil {
		return nil
	}
	out := make([]City, 0, len(records))
	for _, r := range records {
		if len(r) != 5 {
			continue
		}
		lat, err1 := strconv.ParseFloat(r[2], 64)
		lng, err2 := strconv.ParseFloat(r[3], 64)
		alt, err3 := strconv.ParseFloat(r[4], 64)
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		out = append(out, City{Name: r[0], Country: r[1], Lat: lat, Lng: lng, Alt: alt})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Country != out[j].Country {
			return out[i].Country < out[j].Country
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func citySearchKeys(cities []City) []string {
	keys := make([]string, len(cities))
	for i, c := range cities {
		keys[i] = foldAccents(c.Name + ", " + c.Country)
	}
	return keys
}

func cityLabel(c City) string {
	return c.Name + ", " + c.Country
}

// matchCities returns the first maxCityMatches cities whose name or country
// contains q, case-insensitive. An empty query returns the whole list.
func matchCities(q string) []City {
	q = foldAccents(strings.TrimSpace(q))
	if q == "" {
		return cities
	}
	var out []City
	for i, key := range cityKeys {
		if strings.Contains(key, q) {
			out = append(out, cities[i])
			if len(out) >= maxCityMatches {
				break
			}
		}
	}
	return out
}
