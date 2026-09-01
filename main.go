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
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: logtidy [file ...]\n\n"+
			"logtidy reads log lines from the given files, or from stdin\n"+
			"if no files are given, and writes a normalised version to stdout.\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		if err := process(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "logtidy: %v\n", err)
			os.Exit(1)
		}
		return
	}

	for _, path := range args {
		if err := processFile(path, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "logtidy: %s: %v\n", path, err)
			os.Exit(1)
		}
	}
}

func processFile(path string, w io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return process(f, w)
}

// process copies every non-blank line from r to w, normalising each
// one along the way. A scan error (e.g. a line over the buffer size)
// stops the whole run rather than silently dropping lines.
func process(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if _, err := fmt.Fprintln(w, Normalize(line)); err != nil {
			return err
		}
	}
	return scanner.Err()
}
