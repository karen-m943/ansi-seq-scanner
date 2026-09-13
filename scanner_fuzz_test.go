package ansiseq

import (
	"strings"
	"testing"
)

// reconstruct concatenates a token stream's raw bytes back into the
// original input. Both Text.Text and Sequence.Raw are always exact
// slices of the source data, so this must round-trip regardless of
// what the data contained.
func reconstruct(tokens []Token) string {
	var b strings.Builder
	for _, tok := range tokens {
		if tok.Kind == Text {
			b.WriteString(tok.Text)
		} else {
			b.WriteString(tok.Seq.Raw)
		}
	}
	return b.String()
}

func FuzzTokenize(f *testing.F) {
	seeds := []string{
		"",
		"plain text, no escapes at all",
		"\x1b[31mhello\x1b[0m",
		"\x1b[;1m",
		"\x1b[38:2:255:0:0m",
		"\x1b[1;4:3m",
		"\x1b[4::3m",
		"\x1b]0;window title\x07",
		"\x1b]0;window title\x1b\\",
		"\x1bPq payload\x1b\\",
		"\x1b(B",
		"\x1b7",
		"\x9B31mhello",
		"\x9D0;title\x9C",
		"\x90payload\x9C",
		"\x1b",
		"\x1b[",
		"\x1b[31",
		"\x1b[31;",
		"\x1b]unterminated",
		"\x1bPunterminated",
		"\x1b\x1b\x1b",
		"\x9B",
		"\x1b[?25h\x1b[?25l",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Lenient mode is documented to never error and to fall back to
		// literal text a byte at a time, so it must always reconstruct
		// the exact input no matter how malformed.
		lenient, err := NewScanner(WithLenient()).Tokenize(data)
		if err != nil {
			t.Fatalf("lenient mode returned an error: %v", err)
		}
		if got, want := reconstruct(lenient), string(data); got != want {
			t.Fatalf("lenient reconstruction mismatch\n got: %q\nwant: %q", got, want)
		}

		// Strict mode must never panic. When it succeeds, the token
		// stream must still cover the input exactly.
		strict, err := NewScanner().Tokenize(data)
		if err == nil {
			if got, want := reconstruct(strict), string(data); got != want {
				t.Fatalf("strict reconstruction mismatch\n got: %q\nwant: %q", got, want)
			}
		}
	})
}
