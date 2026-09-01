# ansi-seq-scanner

A Go library that tokenizes raw terminal output into text and escape
sequences.

## The problem

Anything that reads terminal output it didn't produce - a log
collector, a CI transcript viewer, a screen-recording tool, a chatbot
that shells out to a real terminal - has to tell literal text apart
from control sequences (color codes, cursor moves, OSC window-title
and hyperlink sequences, and so on) before it can do anything useful
with the bytes. Most tools reach for a regex that strips `\x1b\[...m`
and call it done. That works for the SGR color codes and nothing else:
it silently swallows OSC and DCS sequences, mishandles a sequence
split across two reads, and gives no way to notice that the input
wasn't well-formed in the first place - which matters if you're
displaying or logging text that came from somewhere you don't fully
trust, since a smuggled escape sequence can rewrite terminal state or
spoof output.

This library parses the actual escape sequence grammar (CSI, OSC, DCS,
and the short ESC forms) instead of pattern-matching one case of it,
and it is strict by default: malformed or truncated input is an error,
not a best guess.

## Usage

```go
package main

import (
	"fmt"
	"log"

	"github.com/karen-m943/ansi-seq-scanner"
)

func main() {
	out := []byte("\x1b[1;32mBUILD OK\x1b[0m in 4.2s\n")

	tokens, err := ansiseq.NewScanner().Tokenize(out)
	if err != nil {
		log.Fatal(err)
	}

	for _, tok := range tokens {
		switch tok.Kind {
		case ansiseq.Text:
			fmt.Printf("text:   %q\n", tok.Text)
		case ansiseq.Escape:
			fmt.Printf("escape: %s final=%q params=%v\n",
				tok.Seq.Type, tok.Seq.Final, tok.Seq.Params)
		}
	}

	// Or just get the text back, escape sequences discarded:
	fmt.Println(ansiseq.PlainText(tokens))
}
```

By default, `Tokenize` returns a `*ParseError` the moment it hits a
byte sequence that starts with ESC but doesn't resolve to a complete,
valid sequence - an unterminated CSI sequence at the end of a buffer,
an invalid final byte, a non-numeric parameter. That's the right
behavior when you're validating output you generate yourself, or
deciding whether to trust input before you act on it.

When you're parsing terminal output you captured from somewhere else -
a log file, a pasted CI transcript, a session recording that might
have been truncated mid-sequence - failing on the first bad byte isn't
useful. Pass `WithLenient()` and malformed sequences are emitted as
literal text instead of stopping the scan:

```go
tokens, err := ansiseq.NewScanner(ansiseq.WithLenient()).Tokenize(capturedLog)
// err is nil even if capturedLog was truncated mid-escape-sequence;
// the incomplete sequence shows up as ordinary text tokens instead.
```

## What's parsed

- **CSI** - `ESC [ params intermediates final` (SGR colors, cursor
  movement, erase, etc.) with numeric parameters split out, including
  the private-marker byte (`?`, `<`, `=`, `>`) some CSI sequences use
  and the colon-separated sub-parameters some terminals use for RGB SGR
  values (`38:2:255:0:0`), exposed as `Sequence.SubParams`.
- **OSC** - `ESC ] data` terminated by BEL or ST (window title,
  hyperlinks, clipboard).
- **DCS** - `ESC P data ST` (Sixel graphics, tmux passthrough) as an
  opaque payload.
- **Simple ESC forms** - both the bare kind (`ESC 7`, `ESC c`) and the
  one-intermediate kind (`ESC ( B`).

## Current limitations

- 8-bit C1 control introducers (e.g. a raw `0x9B` in place of
  `ESC [`) aren't recognized - only the 7-bit `ESC`-prefixed forms.
- Sequence *meaning* isn't decoded. `Sequence` gives you the parsed
  grammar (type, params, final byte); mapping `params == [31]` to "set
  foreground red" is left to the caller for now.

## License

MIT, see [LICENSE](LICENSE).
