package main

import (
	"math"
	"testing"
)

func TestImportTemplatesCompile(t *testing.T) {
	NewRenderer()
}

func TestSuggestEnumValueIDMatchesAvailableValues(t *testing.T) {
	values := []EnumValue{
		{ID: "a", Value: "custom-small", DisplayName: "0603"},
		{ID: "b", Value: "custom-medium", DisplayName: "0805"},
	}

	if got := suggestEnumValueID("CAP CER 0.1UF 50V X7R 0805", values); got != "b" {
		t.Fatalf("expected enum ID b, got %q", got)
	}
}

func TestSuggestEnumValueIDMatchesAbbreviationsAndSymbols(t *testing.T) {
	values := []EnumValue{
		{ID: "film", Value: "filmish", DisplayName: "Film"},
		{ID: "ceramic", Value: "ceramicish", DisplayName: "Ceramic"},
		{ID: "onepct", Value: "one_percent", DisplayName: "1%"},
	}

	if got := suggestEnumValueID("CAP CER 0.1UF 50V X7R 0805", values[:2]); got != "ceramic" {
		t.Fatalf("expected ceramic enum ID, got %q", got)
	}
	if got := suggestEnumValueID("RES 2K OHM 1% 1/8W 0805", values[2:]); got != "onepct" {
		t.Fatalf("expected onepct enum ID, got %q", got)
	}
}

func TestSuggestNumericToken(t *testing.T) {
	capUnit := "F"
	voltUnit := "V"
	ohmUnit := "Ω"
	wattUnit := "W"
	hzUnit := "Hz"

	tests := []struct {
		name string
		desc string
		attr AttributeDefinition
		want float64
	}{
		{
			name: "capacitance",
			desc: "CAP CER 0.1UF 50V X7R 0805",
			attr: AttributeDefinition{Name: "capacitance", DisplayName: "Capacitance", Unit: &capUnit},
			want: 1e-7,
		},
		{
			name: "voltage",
			desc: "CAP CER 0.1UF 50V X7R 0805",
			attr: AttributeDefinition{Name: "voltage_rating", DisplayName: "Voltage Rating", Unit: &voltUnit},
			want: 50,
		},
		{
			name: "resistance",
			desc: "RES 2K OHM 1% 1/8W 0805",
			attr: AttributeDefinition{Name: "resistance", DisplayName: "Resistance", Unit: &ohmUnit},
			want: 2000,
		},
		{
			name: "power",
			desc: "RES 2K OHM 1% 1/8W 0805",
			attr: AttributeDefinition{Name: "power_rating", DisplayName: "Power Rating", Unit: &wattUnit},
			want: 0.125,
		},
		{
			name: "frequency",
			desc: "XTAL OSC XO 1.0000MHZ TTL TH",
			attr: AttributeDefinition{Name: "frequency", DisplayName: "Frequency", Unit: &hzUnit},
			want: 1000000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := suggestNumericToken(tt.desc, tt.attr, *tt.attr.Unit)
			got, err := ParseSI(raw)
			if err != nil {
				t.Fatalf("ParseSI(%q): %v", raw, err)
			}
			if math.Abs(got-tt.want) > 1e-12 {
				t.Fatalf("got %g, want %g from %q", got, tt.want, raw)
			}
		})
	}
}

func TestSuggestVoltageTokenHandlesMinMax(t *testing.T) {
	voltUnit := "V"

	maxAttr := AttributeDefinition{Name: "supply_voltage_max", DisplayName: "Supply Voltage Max", Unit: &voltUnit}
	minAttr := AttributeDefinition{Name: "supply_voltage_min", DisplayName: "Supply Voltage Min", Unit: &voltUnit}

	if got := suggestNumericToken("IC TXRX NON-INVERT 6V 20-SOIC", minAttr, "V"); got != "" {
		t.Fatalf("single voltage should not populate min, got %q", got)
	}
	if got := suggestNumericToken("IC TXRX NON-INVERT 6V 20-SOIC", maxAttr, "V"); got != "6V" {
		t.Fatalf("single voltage should populate max, got %q", got)
	}

	minRaw := suggestNumericToken("IC TXRX NON-INVERT 2V-6V 20-SOIC", minAttr, "V")
	maxRaw := suggestNumericToken("IC TXRX NON-INVERT 2V-6V 20-SOIC", maxAttr, "V")
	if minRaw != "2V" || maxRaw != "6V" {
		t.Fatalf("range should populate min/max, got min=%q max=%q", minRaw, maxRaw)
	}
}

func TestSuggestCategoryIDMatchesCrystalsAndOscillators(t *testing.T) {
	categories := []CategoryListItem{
		{ID: "res", Path: "Resistors"},
		{ID: "xtal", Path: "Crystals & Oscillators"},
	}

	if got := suggestCategoryID("XTAL OSC XO 1.0000MHZ TTL TH", categories); got != "xtal" {
		t.Fatalf("expected xtal category, got %q", got)
	}
}

func TestSuggestCategoryIDMatchesMouserLogicDescriptions(t *testing.T) {
	logicDesc := "Digital logic integrated circuits"
	interfaceDesc := "Bus, line, and protocol interface ICs"
	categories := []CategoryListItem{
		{ID: "logic", Path: "Logic ICs", Description: &logicDesc},
		{ID: "interface", Path: "Interface ICs", Description: &interfaceDesc},
		{ID: "gates", Path: "Logic ICs / Gates"},
		{ID: "mux", Path: "Logic ICs / Encoders, Decoders, Multiplexers & Demultiplexers"},
	}

	tests := []struct {
		desc string
		want string
	}{
		{
			desc: "Bus Transceivers Bus Transceivers LOGIC IC SERIESTRANSCEIVER",
			want: "interface",
		},
		{
			desc: "Encoders, Decoders, Multiplexers & Demultiplexers Encoders, Decoders, Multiplexers & Demultiplexers CMOS LOGIC IC SERIES SOIC16",
			want: "mux",
		},
		{
			desc: "Inverters Inverters LOGIC IC SERIESHEX SCHMITT IN",
			want: "gates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := suggestCategoryID(tt.desc, categories); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
