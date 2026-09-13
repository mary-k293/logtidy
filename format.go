package main

import (
	"regexp"
	"strconv"
	"strings"
	"text/template"
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

// syslogLayout is the traditional BSD syslog timestamp: month, day
// and time, three space-separated fields with no year. Since the
// year isn't in the line, we assume the current one.
const syslogLayout = "Jan 2 15:04:05"

// unixEpochPattern matches a bare Unix timestamp: ten digits for
// seconds, with an optional fractional part, or thirteen digits for
// milliseconds. Anything else that's all digits is more likely to be
// some other kind of field (a request ID, a byte count), so we leave
// it alone rather than risk a wrong guess.
var unixEpochPattern = regexp.MustCompile(`^\d{10}(\.\d+)?$|^\d{13}$`)

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

// DefaultFormat is the template used when the caller doesn't supply
// its own. It reproduces the plain "<timestamp> <LEVEL> <message>"
// shape logtidy has always produced.
const DefaultFormat = "{{.Timestamp}} {{.Level}} {{.Message}}"

// defaultTemplate is DefaultFormat, pre-parsed so Normalize doesn't
// re-parse it on every line.
var defaultTemplate = template.Must(template.New("default").Parse(DefaultFormat))

// Record holds the pieces Normalize can pull out of a line, for use
// as the data driving a custom -format template.
type Record struct {
	Timestamp string
	Level     string
	Message   string
}

// Normalize rewrites a single raw log line using DefaultFormat. It's
// a thin wrapper around Render kept around because it can't fail and
// so is convenient to call directly (tests, simple embedding).
func Normalize(line string) string {
	out, err := Render(line, defaultTemplate)
	if err != nil {
		// defaultTemplate only references Record's own fields, so
		// execution against a Record can't fail.
		panic(err)
	}
	return out
}

// Render extracts a timestamp and level from line, if it can find
// them, and executes tmpl against the result. Lines with neither a
// recognisable timestamp nor a recognisable level are passed through
// unchanged (whitespace collapsed) rather than run through tmpl,
// since there's nothing structured to fill the template with and the
// tool should never drop data just because it doesn't recognise the
// shape.
func Render(line string, tmpl *template.Template) (string, error) {
	clean := strings.TrimSpace(whitespaceRun.ReplaceAllString(line, " "))
	if clean == "" {
		return "", nil
	}

	rec, found := extractFields(clean)
	if !found {
		return clean, nil
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, rec); err != nil {
		return "", err
	}
	// A field left empty by extraction (e.g. no level found) leaves
	// behind a run of whitespace wherever the template placed it;
	// collapse and trim those away rather than asking every -format
	// value to account for missing fields itself.
	return strings.TrimSpace(whitespaceRun.ReplaceAllString(buf.String(), " ")), nil
}

// extractFields pulls a timestamp, level and message out of an
// already-whitespace-collapsed line. found is false if neither a
// timestamp nor a level was recognised, in which case rec is not
// meaningful.
func extractFields(clean string) (rec Record, found bool) {
	fields := strings.Split(clean, " ")

	if ts, consumed, ok := extractTimestamp(fields); ok {
		rec.Timestamp = ts.UTC().Format(time.RFC3339)
		fields = fields[consumed:]
		found = true
	}

	if len(fields) > 0 {
		if lvl, ok := extractLevel(fields[0]); ok {
			rec.Level = lvl
			fields = fields[1:]
			found = true
		}
	}

	rec.Message = strings.Join(fields, " ")
	return rec, found
}

// extractTimestamp tries to parse a timestamp from the start of
// fields: first as a three-token syslog "month day time" triple,
// then as a two-token "date time" pair, and finally as a single
// token (a full layout or a bare Unix epoch). It reports how many
// leading fields it consumed.
func extractTimestamp(fields []string) (time.Time, int, bool) {
	if len(fields) >= 3 {
		candidate := fields[0] + " " + fields[1] + " " + fields[2]
		if t, err := time.Parse(syslogLayout, candidate); err == nil {
			return t.AddDate(time.Now().Year(), 0, 0), 3, true
		}
	}
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
		if t, ok := parseUnixEpoch(fields[0]); ok {
			return t, 1, true
		}
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

// parseUnixEpoch converts a bare Unix timestamp token into a time.
// It handles whole seconds, fractional seconds, and milliseconds;
// see unixEpochPattern for how those are told apart.
func parseUnixEpoch(token string) (time.Time, bool) {
	if !unixEpochPattern.MatchString(token) {
		return time.Time{}, false
	}
	if strings.Contains(token, ".") {
		secs, err := strconv.ParseFloat(token, 64)
		if err != nil {
			return time.Time{}, false
		}
		whole := int64(secs)
		nanos := int64((secs - float64(whole)) * 1e9)
		return time.Unix(whole, nanos), true
	}
	if len(token) == 13 {
		millis, err := strconv.ParseInt(token, 10, 64)
		if err != nil {
			return time.Time{}, false
		}
		return time.UnixMilli(millis), true
	}
	secs, err := strconv.ParseInt(token, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(secs, 0), true
}

// extractLevel checks a single token against the known level names,
// stripping the brackets or trailing colon that loggers commonly
// wrap a level in (e.g. "[INFO]", "WARN:").
func extractLevel(token string) (string, bool) {
	trimmed := strings.Trim(token, "[]():")
	lvl, ok := logLevels[strings.ToUpper(trimmed)]
	return lvl, ok
}
