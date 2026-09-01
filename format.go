package main

import (
	"regexp"
	"strings"
	"time"
)

var whitespaceRun = regexp.MustCompile(`\s+`)

// timestampLayouts covers the formats we currently recognise. Each
// layout is tagged implicitly by whether it contains a space: that's
// how extractTimestamp decides whether to try it against one leading
// field or two.
var timestampLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006/01/02 15:04:05",
	"01/02/2006 15:04:05",
}

// logLevels maps every spelling we accept to its canonical form.
var logLevels = map[string]string{
	"TRACE":       "TRACE",
	"DEBUG":       "DEBUG",
	"INFO":        "INFO",
	"INFORMATION": "INFO",
	"WARN":        "WARN",
	"WARNING":     "WARN",
	"ERROR":       "ERROR",
	"ERR":         "ERROR",
	"FATAL":       "FATAL",
	"CRIT":        "FATAL",
	"CRITICAL":    "FATAL",
}

// Normalize rewrites a single raw log line as "<RFC3339 timestamp>
// <LEVEL> <message>" wherever it can confidently find a leading
// timestamp and level. Lines that don't start that way are passed
// through with whitespace collapsed, so the tool never drops data
// just because it doesn't recognise the shape.
func Normalize(line string) string {
	clean := strings.TrimSpace(whitespaceRun.ReplaceAllString(line, " "))
	if clean == "" {
		return ""
	}

	fields := strings.Split(clean, " ")
	var out []string

	if ts, consumed, ok := extractTimestamp(fields); ok {
		out = append(out, ts.UTC().Format(time.RFC3339))
		fields = fields[consumed:]
	}

	if len(fields) > 0 {
		if lvl, ok := extractLevel(fields[0]); ok {
			out = append(out, lvl)
			fields = fields[1:]
		}
	}

	if message := strings.Join(fields, " "); message != "" {
		out = append(out, message)
	}

	if len(out) == 0 {
		return clean
	}
	return strings.Join(out, " ")
}

// extractTimestamp tries to parse a timestamp from the start of
// fields, first as a two-token "date time" pair and then as a single
// token. It reports how many leading fields it consumed.
func extractTimestamp(fields []string) (time.Time, int, bool) {
	if len(fields) >= 2 {
		candidate := fields[0] + " " + fields[1]
		for _, layout := range timestampLayouts {
			if strings.Contains(layout, " ") {
				if t, err := time.Parse(layout, candidate); err == nil {
					return t, 2, true
				}
			}
		}
	}
	if len(fields) >= 1 {
		for _, layout := range timestampLayouts {
			if !strings.Contains(layout, " ") {
				if t, err := time.Parse(layout, fields[0]); err == nil {
					return t, 1, true
				}
			}
		}
	}
	return time.Time{}, 0, false
}

// extractLevel checks a single token against the known level names,
// stripping the brackets or trailing colon that loggers commonly
// wrap a level in (e.g. "[INFO]", "WARN:").
func extractLevel(token string) (string, bool) {
	trimmed := strings.Trim(token, "[]():")
	lvl, ok := logLevels[strings.ToUpper(trimmed)]
	return lvl, ok
}
