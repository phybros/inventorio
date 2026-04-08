package main

import (
	"testing"
)

func TestFormatSI(t *testing.T) {
	cases := []struct{ v float64; unit, want string }{
		{5100, "Ω", "5.1 kΩ"},
		{5100000, "Ω", "5.1 MΩ"},
		{47, "Ω", "47 Ω"},
		{1e-7, "F", "100 nF"},
		{22e-12, "F", "22 pF"},
		{1e-3, "F", "1 mF"},
		{1000000, "Hz", "1 MHz"},
		{0.001, "A", "1 mA"},
		{50, "%", "50 %"},
		{0, "F", "0 F"},
	}
	for _, c := range cases {
		got := FormatSI(c.v, c.unit)
		if got != c.want {
			t.Errorf("FormatSI(%g, %q) = %q, want %q", c.v, c.unit, got, c.want)
		}
	}
}

func TestParseSI(t *testing.T) {
	cases := []struct{ input string; want float64 }{
		{"5.1k", 5100},
		{"100n", 100e-9},
		{"22p", 22e-12},
		{"1M", 1e6},
		{"1u", 1e-6},
		{"5100", 5100},
		{"5.1kΩ", 5100},
		{"100nF", 100e-9},
	}
	for _, c := range cases {
		got, err := ParseSI(c.input)
		if err != nil {
			t.Errorf("ParseSI(%q) error: %v", c.input, err)
			continue
		}
		ratio := got / c.want
		if ratio < 0.9999 || ratio > 1.0001 {
			t.Errorf("ParseSI(%q) = %g, want %g", c.input, got, c.want)
		}
	}
}
