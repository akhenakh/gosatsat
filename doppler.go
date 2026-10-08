package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const speedOfLightKmS = 299792.458 // km/s

// parseFrequencyMHz converts a frequency string (e.g. "435.310" or
// "145.800/145.900") to float64 MHz, taking the first listed frequency.
func parseFrequencyMHz(freqStr string) float64 {
	if freqStr == "" {
		return 0
	}
	val, err := strconv.ParseFloat(strings.TrimSpace(strings.Split(freqStr, "/")[0]), 64)
	if err != nil {
		return 0
	}
	return val
}

// calculateDopplerShift returns the observed frequency of a source while the
// observer and source have the given range rate (km/s, positive = receding).
func calculateDopplerShift(frequencyMHz, rangeRateKmS float64) float64 {
	if frequencyMHz == 0 {
		return 0
	}
	denominator := speedOfLightKmS - rangeRateKmS
	if math.Abs(denominator) < 1e-9 {
		return frequencyMHz
	}
	return frequencyMHz * (speedOfLightKmS / denominator)
}

// uplinkCorrection returns the transmit frequency that arrives on target when
// the station pre-corrects for the given range rate.
func uplinkCorrection(frequencyMHz, rangeRateKmS float64) float64 {
	if frequencyMHz == 0 {
		return 0
	}
	return frequencyMHz * (speedOfLightKmS / (speedOfLightKmS + rangeRateKmS))
}

// formatFrequency renders a MHz value, or "N/A" when unset.
func formatFrequency(freqMHz float64) string {
	if freqMHz == 0 {
		return "N/A"
	}
	return fmt.Sprintf("%.3f MHz", freqMHz)
}
