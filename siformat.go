package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// siUnits is the set of units that should use SI prefix formatting.
var siUnits = map[string]bool{
	"Ω": true, "F": true, "H": true, "Hz": true,
	"V": true, "W": true, "A": true, "S": true,
}

type siPrefix struct {
	multiplier float64
	symbol     string
}

var siPrefixes = []siPrefix{
	{1e12, "T"},
	{1e9, "G"},
	{1e6, "M"},
	{1e3, "k"},
	{1, ""},
	{1e-3, "m"},
	{1e-6, "µ"},
	{1e-9, "n"},
	{1e-12, "p"},
}

// FormatSI formats a numeric value with the appropriate SI prefix for display.
// e.g. FormatSI(5100, "Ω") → "5.1 kΩ", FormatSI(1e-7, "F") → "100 nF"
// Units not in the siUnits set are formatted with %g and the unit appended.
func FormatSI(v float64, unit string) string {
	if !siUnits[unit] {
		if unit == "" {
			return strconv.FormatFloat(v, 'g', 4, 64)
		}
		return fmt.Sprintf("%g %s", v, unit)
	}
	prefix, scaled := chooseSIPrefix(v)
	num := strconv.FormatFloat(scaled, 'g', 4, 64)
	if prefix == "" {
		return num + " " + unit
	}
	return num + " " + prefix + unit
}

// FormatSIInput formats a value for use in a form input field — prefix notation
// without the unit symbol, so the user can edit it directly.
// e.g. FormatSIInput(5100, "Ω") → "5.1k", FormatSIInput(1e-7, "F") → "100n"
// Falls back to %g for non-SI units or when unit is empty.
func FormatSIInput(v float64, unit string) string {
	if !siUnits[unit] {
		return strconv.FormatFloat(v, 'g', 4, 64)
	}
	prefix, scaled := chooseSIPrefix(v)
	return strconv.FormatFloat(scaled, 'g', 4, 64) + prefix
}

// chooseSIPrefix picks the best SI prefix for v and returns the prefix symbol
// and the scaled value. For zero, returns ("", 0).
func chooseSIPrefix(v float64) (string, float64) {
	if v == 0 {
		return "", 0
	}
	abs := math.Abs(v)
	for _, p := range siPrefixes {
		if abs >= p.multiplier*0.9999 { // small epsilon to avoid floating-point edge cases
			return p.symbol, v / p.multiplier
		}
	}
	// Smaller than pico: use pico anyway
	return "p", v / 1e-12
}

// ParseSI parses a string that may include an SI prefix suffix into a float64.
// Accepts "5.1k", "100n", "22p", "1M", "1u", "1µ", plain floats, and values
// with trailing unit symbols like "5.1kΩ" or "100nF".
func ParseSI(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}

	// Strip known unit symbols from the end
	s = stripUnitSuffix(s)
	if s == "" {
		return 0, fmt.Errorf("no numeric content")
	}

	// Decode the last rune to check for an SI prefix
	last, size := utf8.DecodeLastRuneInString(s)
	multiplier := siPrefixMultiplier(last)
	if multiplier != 0 {
		numStr := strings.TrimSpace(s[:len(s)-size])
		f, err := strconv.ParseFloat(numStr, 64)
		if err == nil {
			return f * multiplier, nil
		}
	}

	// No prefix or prefix didn't help — try parsing as plain float
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse %q as a number", s)
	}
	return f, nil
}

// siPrefixMultiplier returns the multiplier for a known SI prefix rune,
// or 0 if the rune is not a recognised prefix.
func siPrefixMultiplier(r rune) float64 {
	switch r {
	case 'T':
		return 1e12
	case 'G':
		return 1e9
	case 'M':
		return 1e6
	case 'k', 'K':
		return 1e3
	case 'm':
		return 1e-3
	case 'u', 'µ', 'μ': // ASCII u, micro sign U+00B5, Greek small mu U+03BC
		return 1e-6
	case 'n':
		return 1e-9
	case 'p':
		return 1e-12
	}
	return 0
}

// stripUnitSuffix removes trailing unit symbols (Ω, F, H, Hz, V, W, A, S, %)
// from a string, leaving the numeric part and any SI prefix.
func stripUnitSuffix(s string) string {
	unitSuffixes := []string{"Hz", "Ω", "F", "H", "V", "W", "A", "S", "%"}
	for _, u := range unitSuffixes {
		if strings.HasSuffix(s, u) {
			trimmed := strings.TrimRight(s[:len(s)-len(u)], " ")
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return s
}

