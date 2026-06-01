package system

import (
	"testing"
)

func TestParseBatteryFloat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		wantVal float64
		wantOK  bool
	}{
		{"42.5", 42.5, true},
		{"0", 0, true},
		{"", 0, false},
		{"invalid", 0, false},
	}

	for _, tc := range tests {
		val, ok := parseBatteryFloat(tc.input)
		if ok != tc.wantOK {
			t.Errorf("parseBatteryFloat(%q) ok=%v, want %v", tc.input, ok, tc.wantOK)
		}
		if ok && val != tc.wantVal {
			t.Errorf("parseBatteryFloat(%q) = %f, want %f", tc.input, val, tc.wantVal)
		}
	}
}

func TestEnrichBatteryInfoWear(t *testing.T) {
	t.Parallel()

	defaultMax := 1000

	t.Run("normal wear", func(t *testing.T) {
		m := BatteryInfo{
			"energy-full":        "36.0 Wh",
			"energy-full-design": "40.0 Wh",
		}
		enrichBatteryInfo(m, defaultMax)
		if m["wear_level"] == "unknown" {
			t.Fatal("wear_level should be computed")
		}
		if m["wear_level"] != "10.0%" {
			t.Errorf("expected 10.0%%, got %s", m["wear_level"])
		}
	})

	t.Run("no wear", func(t *testing.T) {
		m := BatteryInfo{
			"energy-full":        "40.0 Wh",
			"energy-full-design": "40.0 Wh",
		}
		enrichBatteryInfo(m, defaultMax)
		if m["wear_level"] != "0.0%" {
			t.Errorf("expected 0.0%%, got %s", m["wear_level"])
		}
	})

	t.Run("cannot compute", func(t *testing.T) {
		m := BatteryInfo{}
		enrichBatteryInfo(m, defaultMax)
		if m["wear_level"] != "unknown" {
			t.Errorf("expected unknown, got %s", m["wear_level"])
		}
	})

	t.Run("wear clamped to 0", func(t *testing.T) {
		m := BatteryInfo{
			"energy-full":        "50.0 Wh", // higher than design? shouldn't happen but guard
			"energy-full-design": "40.0 Wh",
		}
		enrichBatteryInfo(m, defaultMax)
		if m["wear_level"] != "0.0%" {
			t.Errorf("expected 0.0%%, got %s", m["wear_level"])
		}
	})

	t.Run("wear clamped to 100", func(t *testing.T) {
		m := BatteryInfo{
			"energy-full":        "1.0 Wh",
			"energy-full-design": "40.0 Wh",
		}
		enrichBatteryInfo(m, defaultMax)
		if m["wear_level"] != "97.5%" {
			t.Errorf("expected 97.5%%, got %s", m["wear_level"])
		}
	})
}

func TestEnrichBatteryInfoCyclesRemaining(t *testing.T) {
	t.Parallel()

	defaultMax := 1000

	t.Run("normal cycles with default max", func(t *testing.T) {
		m := BatteryInfo{
			"charge-cycles": "350",
			"energy-full":   "40.0 Wh",
		}
		enrichBatteryInfo(m, defaultMax)
		if m["cycles_remaining"] != "650" {
			t.Errorf("expected 650, got %s", m["cycles_remaining"])
		}
	})

	t.Run("normal cycles with custom max", func(t *testing.T) {
		m := BatteryInfo{
			"charge-cycles": "350",
			"energy-full":   "40.0 Wh",
		}
		enrichBatteryInfo(m, 500)
		if m["cycles_remaining"] != "150" {
			t.Errorf("expected 150, got %s", m["cycles_remaining"])
		}
	})

	t.Run("exceeded max cycles", func(t *testing.T) {
		m := BatteryInfo{
			"charge-cycles": "1200",
			"energy-full":   "40.0 Wh",
		}
		enrichBatteryInfo(m, defaultMax)
		if m["cycles_remaining"] != "0" {
			t.Errorf("expected 0, got %s", m["cycles_remaining"])
		}
	})

	t.Run("no cycles data", func(t *testing.T) {
		m := BatteryInfo{}
		enrichBatteryInfo(m, defaultMax)
		if _, exists := m["cycles_remaining"]; exists {
			t.Errorf("cycles_remaining should not be set when no charge-cycles")
		}
	})
}

func TestGetPowerProfile(t *testing.T) {
	// Verifica que getPowerProfile retorna un valor sin panic.
	// Retorna "unknown" si powerprofilesctl no está instalado, lo cual es válido.
	result := getPowerProfile()
	if result == "" {
		t.Error("power profile should not be empty")
	}
}
