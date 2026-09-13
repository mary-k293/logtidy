package main

import (
	"fmt"
	"testing"
	"text/template"
	"time"
)

func TestNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "syslog timestamp gets the current year",
			in:   "Jan 5 09:03:44 ERROR disk failure",
			want: fmt.Sprintf("%d-01-05T09:03:44Z ERROR disk failure", time.Now().Year()),
		},
		{
			name: "syslog timestamp with zero-padded day",
			in:   "Jan 05 09:03:44 ERROR disk failure",
			want: fmt.Sprintf("%d-01-05T09:03:44Z ERROR disk failure", time.Now().Year()),
		},
		{
			name: "unix epoch seconds",
			in:   "1704445424 INFO service ready",
			want: "2024-01-05T09:03:44Z INFO service ready",
		},
		{
			name: "unix epoch milliseconds",
			in:   "1704445424000 INFO service ready",
			want: "2024-01-05T09:03:44Z INFO service ready",
		},
		{
			name: "unix epoch fractional seconds",
			in:   "1704445424.5 INFO service ready",
			want: "2024-01-05T09:03:44Z INFO service ready",
		},
		{
			name: "space separated date and time with lowercase level",
			in:   "2024-01-05 14:22:01   error    disk full on /dev/sda1",
			want: "2024-01-05T14:22:01Z ERROR disk full on /dev/sda1",
		},
		{
			name: "rfc3339 timestamp with uppercase level",
			in:   "2024-01-05T09:03:44Z INFO    request completed in 12ms",
			want: "2024-01-05T09:03:44Z INFO request completed in 12ms",
		},
		{
			name: "bracketed level",
			in:   "2024-01-05T09:03:44Z [WARN] disk usage high",
			want: "2024-01-05T09:03:44Z WARN disk usage high",
		},
		{
			name: "level with trailing colon",
			in:   "2024-01-05T09:03:44Z ERR: connection refused",
			want: "2024-01-05T09:03:44Z ERROR connection refused",
		},
		{
			name: "slash separated date",
			in:   "2024/01/05 09:03:44 CRITICAL out of memory",
			want: "2024-01-05T09:03:44Z FATAL out of memory",
		},
		{
			name: "us style date",
			in:   "01/05/2024 09:03:44 DEBUG starting worker",
			want: "2024-01-05T09:03:44Z DEBUG starting worker",
		},
		{
			name: "unstructured text is whitespace collapsed only",
			in:   "   just some unstructured text with   extra   spaces",
			want: "just some unstructured text with extra spaces",
		},
		{
			name: "timestamp with no recognisable level keeps the token as message",
			in:   "2024-01-05T09:03:44Z something happened",
			want: "2024-01-05T09:03:44Z something happened",
		},
		{
			name: "level with no timestamp",
			in:   "ERROR disk full",
			want: "ERROR disk full",
		},
		{
			name: "timestamp and level but no message",
			in:   "2024-01-05T09:03:44Z INFO",
			want: "2024-01-05T09:03:44Z INFO",
		},
		{
			name: "blank line",
			in:   "   ",
			want: "",
		},
		{
			name: "informational level spelled out is canonicalised",
			in:   "2024-01-05T09:03:44Z INFORMATION service ready",
			want: "2024-01-05T09:03:44Z INFO service ready",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Normalize(tc.in); got != tc.want {
				t.Errorf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestExtractTimestamp(t *testing.T) {
	t.Run("two token layout consumes two fields", func(t *testing.T) {
		fields := []string{"2024-01-05", "14:22:01", "error", "disk", "full"}
		ts, consumed, ok := extractTimestamp(fields)
		if !ok {
			t.Fatal("expected a timestamp match")
		}
		if consumed != 2 {
			t.Errorf("consumed = %d, want 2", consumed)
		}
		want := time.Date(2024, 1, 5, 14, 22, 1, 0, time.UTC)
		if !ts.Equal(want) {
			t.Errorf("ts = %v, want %v", ts, want)
		}
	})

	t.Run("single token layout consumes one field", func(t *testing.T) {
		fields := []string{"2024-01-05T09:03:44Z", "INFO", "ready"}
		ts, consumed, ok := extractTimestamp(fields)
		if !ok {
			t.Fatal("expected a timestamp match")
		}
		if consumed != 1 {
			t.Errorf("consumed = %d, want 1", consumed)
		}
		want := time.Date(2024, 1, 5, 9, 3, 44, 0, time.UTC)
		if !ts.Equal(want) {
			t.Errorf("ts = %v, want %v", ts, want)
		}
	})

	t.Run("no match returns false", func(t *testing.T) {
		fields := []string{"not", "a", "timestamp"}
		if _, _, ok := extractTimestamp(fields); ok {
			t.Error("expected no timestamp match")
		}
	})

	t.Run("empty fields returns false", func(t *testing.T) {
		if _, _, ok := extractTimestamp(nil); ok {
			t.Error("expected no timestamp match on empty input")
		}
	})

	t.Run("syslog layout consumes three fields", func(t *testing.T) {
		fields := []string{"Mar", "9", "08:15:00", "INFO", "ready"}
		ts, consumed, ok := extractTimestamp(fields)
		if !ok {
			t.Fatal("expected a timestamp match")
		}
		if consumed != 3 {
			t.Errorf("consumed = %d, want 3", consumed)
		}
		want := time.Date(time.Now().Year(), 3, 9, 8, 15, 0, 0, time.UTC)
		if !ts.Equal(want) {
			t.Errorf("ts = %v, want %v", ts, want)
		}
	})

	t.Run("unix epoch seconds consumes one field", func(t *testing.T) {
		fields := []string{"1704445424", "INFO", "ready"}
		ts, consumed, ok := extractTimestamp(fields)
		if !ok {
			t.Fatal("expected a timestamp match")
		}
		if consumed != 1 {
			t.Errorf("consumed = %d, want 1", consumed)
		}
		want := time.Date(2024, 1, 5, 9, 3, 44, 0, time.UTC)
		if !ts.Equal(want) {
			t.Errorf("ts = %v, want %v", ts, want)
		}
	})

	t.Run("short number is not mistaken for an epoch", func(t *testing.T) {
		fields := []string{"12345", "requests", "served"}
		if _, _, ok := extractTimestamp(fields); ok {
			t.Error("expected no timestamp match")
		}
	})
}

func TestRender(t *testing.T) {
	csv := template.Must(template.New("csv").Parse("{{.Timestamp}},{{.Level}},{{.Message}}"))

	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "custom template fills all three fields",
			in:   "2024-01-05T09:03:44Z ERROR disk failure",
			want: "2024-01-05T09:03:44Z,ERROR,disk failure",
		},
		{
			name: "missing level leaves an empty field rather than shifting columns",
			in:   "2024-01-05T09:03:44Z something happened",
			want: "2024-01-05T09:03:44Z,,something happened",
		},
		{
			name: "unstructured line bypasses the template entirely",
			in:   "just some unstructured text",
			want: "just some unstructured text",
		},
		{
			name: "blank line stays blank",
			in:   "   ",
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Render(tc.in, csv)
			if err != nil {
				t.Fatalf("Render(%q) returned error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("Render(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestRenderInvalidTemplateField(t *testing.T) {
	tmpl := template.Must(template.New("bad").Parse("{{.Timestemp}} {{.Level}} {{.Message}}"))
	if _, err := Render("2024-01-05T09:03:44Z ERROR disk failure", tmpl); err == nil {
		t.Error("expected an error for a template referencing an unknown field")
	}
}

func TestExtractLevel(t *testing.T) {
	cases := []struct {
		token string
		want  string
		ok    bool
	}{
		{"ERROR", "ERROR", true},
		{"error", "ERROR", true},
		{"[INFO]", "INFO", true},
		{"WARN:", "WARN", true},
		{"(DEBUG)", "DEBUG", true},
		{"WARNING", "WARN", true},
		{"CRIT", "FATAL", true},
		{"CRITICAL", "FATAL", true},
		{"INFORMATION", "INFO", true},
		{"ERR", "ERROR", true},
		{"notalevel", "", false},
		{"", "", false},
	}

	for _, tc := range cases {
		got, ok := extractLevel(tc.token)
		if ok != tc.ok || got != tc.want {
			t.Errorf("extractLevel(%q) = (%q, %v), want (%q, %v)", tc.token, got, ok, tc.want, tc.ok)
		}
	}
}
