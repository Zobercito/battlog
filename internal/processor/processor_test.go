package processor

import (
	"path/filepath"
	"testing"

	"battlog/internal/config"
)

func TestSplitJSONObjects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple object",
			input: `{"a":1}{"b":2}`,
			want:  []string{`{"a":1}`, `{"b":2}`},
		},
		{
			name:  "nested braces",
			input: `{"a":{"b":1}}{"c":2}`,
			want:  []string{`{"a":{"b":1}}`, `{"c":2}`},
		},
		{
			name:  "braces inside string value",
			input: `{"name":"test}value"}{"b":2}`,
			want:  []string{`{"name":"test}value"}`, `{"b":2}`},
		},
		{
			name:  "braces inside string with nested",
			input: `{"data":"{nested}","x":1}{"y":2}`,
			want:  []string{`{"data":"{nested}","x":1}`, `{"y":2}`},
		},
		{
			name:  "single object with escaped quote",
			input: `{"name":"test\"{value}","x":1}`,
			want:  []string{`{"name":"test\"{value}","x":1}`},
		},
		{
			name:  "empty input",
			input: ``,
			want:  nil,
		},
		{
			name:  "truncated object",
			input: `{"a":1`,
			want:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitJSONObjects([]byte(tc.input))
			if len(got) != len(tc.want) {
				t.Fatalf("got %d parts, want %d\ngot:  %q\nwant: %q", len(got), len(tc.want), got, tc.want)
			}
			for i := range got {
				if string(got[i]) != tc.want[i] {
					t.Errorf("part %d: got %q, want %q", i, string(got[i]), tc.want[i])
				}
			}
		})
	}
}

func TestParseJSONLines(t *testing.T) {
	t.Parallel()

	input := []byte(`{"ts":1}
{"ts":2}
{"ts":3}`)
	records := parseJSONLines(input)
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
}

func TestParseJSONLinesWithCommaSeparator(t *testing.T) {
	t.Parallel()

	input := []byte(`{"ts":1},
{"ts":2},
{"ts":3}`)
	records := parseJSONLines(input)
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
}

func TestParseJSONLinesEmpty(t *testing.T) {
	t.Parallel()

	if r := parseJSONLines([]byte{}); len(r) != 0 {
		t.Fatalf("expected 0, got %d", len(r))
	}
	if r := parseJSONLines([]byte("\n\n\n")); len(r) != 0 {
		t.Fatalf("expected 0 from whitespace, got %d", len(r))
	}
}

func TestParseSessionJSON(t *testing.T) {
	t.Parallel()

	t.Run("valid JSON with records array", func(t *testing.T) {
		content := []byte(`{"meta":{},"records":[{"ts":1},{"ts":2}]}`)
		records, err := parseSessionJSON(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(records) != 2 {
			t.Fatalf("expected 2 records, got %d", len(records))
		}
	})

	t.Run("fallback to line parsing for non-wrapper JSON", func(t *testing.T) {
		content := []byte(`{"ts":1}
{"ts":2}`)
		records, err := parseSessionJSON(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(records) != 2 {
			t.Fatalf("expected 2 records, got %d", len(records))
		}
	})

	t.Run("empty data", func(t *testing.T) {
		_, err := parseSessionJSON([]byte{})
		if err == nil {
			t.Fatal("expected error for empty data")
		}
	})
}

func TestProcessSessionLogsEmptyDir(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	ProcessSessionLogs(cfg)
}

func TestScanLine(t *testing.T) {
	t.Parallel()

	data := []byte("line1\nline2\nline3")
	line, next := scanLine(data, 0)
	if string(line) != "line1" {
		t.Errorf("expected line1, got %q", line)
	}
	if next != 6 {
		t.Errorf("expected next=6, got %d", next)
	}

	line, next = scanLine(data, next)
	if string(line) != "line2" {
		t.Errorf("expected line2, got %q", line)
	}

	line, next = scanLine(data, next)
	if string(line) != "line3" {
		t.Errorf("expected line3, got %q", line)
	}
	if next != len(data) {
		t.Errorf("expected next=%d, got %d", len(data), next)
	}

	line, next = scanLine(data, next)
	if line != nil {
		t.Errorf("expected nil at end, got %q", line)
	}
}

func testConfig(dir string) config.Config {
	return config.Config{
		LogsRoot:    dir,
		LogDir:      filepath.Join(dir, "current"),
		MasterDir:   filepath.Join(dir, "master"),
		ArchiveDir:  filepath.Join(dir, "archive"),
		ControlFile: filepath.Join(dir, "archivos_procesados.txt"),
		LockFile:    filepath.Join(dir, ".organizar.lock"),
	}
}
