// Command logtidy reads log lines and rewrites each one into a
// consistent "timestamp level message" shape wherever it can find
// enough structure to do so.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
)

func main() {
	format := flag.String("format", DefaultFormat, "Go text/template used to render each parsed line; "+
		"fields available are .Timestamp, .Level and .Message")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: logtidy [file ...]\n\n"+
			"logtidy reads log lines from the given files, or from stdin\n"+
			"if no files are given, and writes a normalised version to stdout.\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	tmpl, err := parseFormat(*format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logtidy: %v\n", err)
		os.Exit(1)
	}

	args := flag.Args()
	if len(args) == 0 {
		if err := process(os.Stdin, os.Stdout, tmpl); err != nil {
			fmt.Fprintf(os.Stderr, "logtidy: %v\n", err)
			os.Exit(1)
		}
		return
	}

	for _, path := range args {
		if err := processFile(path, os.Stdout, tmpl); err != nil {
			fmt.Fprintf(os.Stderr, "logtidy: %s: %v\n", path, err)
			os.Exit(1)
		}
	}
}

// parseFormat parses a -format value and checks that it actually
// executes against a Record. Parse alone doesn't catch a reference to
// a field Record doesn't have (e.g. a typo like .Timestemp); that's
// only detected at execution time, so we run it once here against a
// throwaway Record rather than let it fail partway through the input.
func parseFormat(format string) (*template.Template, error) {
	tmpl, err := template.New("format").Parse(format)
	if err != nil {
		return nil, fmt.Errorf("invalid -format: %w", err)
	}
	if err := tmpl.Execute(io.Discard, Record{}); err != nil {
		return nil, fmt.Errorf("invalid -format: %w", err)
	}
	return tmpl, nil
}

func processFile(path string, w io.Writer, tmpl *template.Template) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return process(f, w, tmpl)
}

// process copies every non-blank line from r to w, rendering each one
// through tmpl along the way. A scan error (e.g. a line over the
// buffer size) stops the whole run rather than silently dropping
// lines.
func process(r io.Reader, w io.Writer, tmpl *template.Template) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		out, err := Render(line, tmpl)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, out); err != nil {
			return err
		}
	}
	return scanner.Err()
}
