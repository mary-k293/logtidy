# logtidy

Log lines from different services never look the same. Timestamps show
up in half a dozen formats, levels are spelled `warn`, `WARNING`, or
`[W]`, and someone's debug print left three spaces between fields.
That's fine for a human skimming one file, but it makes grep, sort,
and anything downstream that expects a consistent shape harder than
it should be.

`logtidy` is a small command-line tool that reads log lines and
rewrites each one into `<RFC3339 timestamp> <LEVEL> <message>`
wherever it can find a timestamp and a level at the start of the
line. Lines it doesn't recognise are still cleaned up (whitespace
collapsed) and passed through untouched, so nothing gets dropped.

## Usage

Read from one or more files:

```
logtidy app.log
logtidy app.log app.log.1
```

Or pipe input in, which is the common case when tailing a live
service or chaining it after another tool:

```
tail -f app.log | logtidy
cat *.log | logtidy > combined.normalised.log
```

## Example

Input:

```
2024-01-05 14:22:01   error    disk full on /dev/sda1
2024-01-05T09:03:44Z INFO    request completed in 12ms
   just some unstructured text with   extra   spaces
```

Output:

```
2024-01-05T14:22:01Z ERROR disk full on /dev/sda1
2024-01-05T09:03:44Z INFO request completed in 12ms
just some unstructured text with extra spaces
```

## Building

Requires Go 1.22 or later and nothing else — no third-party
dependencies.

```
go build -o logtidy .
```

## Status

Early. The timestamp and level detection cover the common cases but
not every log format in the wild. See the issues for what's planned
next.
