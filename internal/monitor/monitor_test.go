package monitor

import (
	"testing"
)

func TestParseTimeToMinutes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  int
	}{
		{"1.5 hours", 90},
		{"2.0 hours", 120},
		{"0.5 hours", 30},
		{"0 hours", 0},
		{"", 0},
	}

	for _, tc := range tests {
		got := parseTimeToMinutes(tc.input)
		if got != tc.want {
			t.Errorf("parseTimeToMinutes(%q) = %d; want %d", tc.input, got, tc.want)
		}
	}
}

func TestParseFrequency(t *testing.T) {
	t.Parallel()

	cur, max := parseFrequency("2.4 GHz / 4.0 GHz max")
	if cur != 2.4 {
		t.Errorf("expected 2.4, got %f", cur)
	}
	if max != 4.0 {
		t.Errorf("expected 4.0, got %f", max)
	}
}

func TestParseFrequencySingle(t *testing.T) {
	t.Parallel()

	cur, max := parseFrequency("2.4 GHz / 4.0 GHz max")
	_ = cur
	_ = max
}

func TestParseMemory(t *testing.T) {
	t.Parallel()

	used, total := parseMemory("4.2 GB / 8 GB (53%)")
	if used != 4.2 {
		t.Errorf("expected 4.2, got %f", used)
	}
	if total != 8.0 {
		t.Errorf("expected 8.0, got %f", total)
	}
}

func TestParseSwap(t *testing.T) {
	t.Parallel()

	if v := parseSwap("2048 MB"); v != 2048 {
		t.Errorf("expected 2048, got %d", v)
	}
	if v := parseSwap("0 MB"); v != 0 {
		t.Errorf("expected 0, got %d", v)
	}
	if v := parseSwap(""); v != 0 {
		t.Errorf("expected 0, got %d", v)
	}
}

func TestParseProcess(t *testing.T) {
	t.Parallel()

	proc := parseProcess("1. firefox : 15.2% (256 MB)")
	if proc.Name != "firefox" {
		t.Errorf("expected firefox, got %q", proc.Name)
	}
	if proc.CPUPct != 15.2 {
		t.Errorf("expected 15.2, got %f", proc.CPUPct)
	}
	if proc.MemMB != 256 {
		t.Errorf("expected 256, got %d", proc.MemMB)
	}
}

func TestParseProcessEmpty(t *testing.T) {
	t.Parallel()

	proc := parseProcess("")
	if proc.Name != "" {
		t.Errorf("expected empty, got %q", proc.Name)
	}
}

func TestFillBatteryData(t *testing.T) {
	t.Parallel()

	bat := map[string]string{
		"percentage":  "85",
		"state":       "charging",
		"energy-rate": "15.5 W",
		"voltage":     "12.3 V",
	}
	entry := &LogEntry{}
	fillBatteryData(bat, nil, entry)

	if entry.Battery.Percentage != 85 {
		t.Errorf("expected 85, got %d", entry.Battery.Percentage)
	}
	if entry.Battery.State != "charging" {
		t.Errorf("expected charging, got %s", entry.Battery.State)
	}
	if entry.Battery.PowerW != 15.5 {
		t.Errorf("expected 15.5, got %f", entry.Battery.PowerW)
	}
	if entry.Battery.Voltage != 12.3 {
		t.Errorf("expected 12.3, got %f", entry.Battery.Voltage)
	}
}
