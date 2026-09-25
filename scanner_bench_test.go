package ansiseq

import (
	"strings"
	"testing"
)

// plainTextData returns n bytes of ordinary text with no escape sequences,
// so the benchmark measures the cost of the byte-scan loop alone.
func plainTextData(n int) []byte {
	const line = "the quick brown fox jumps over the lazy dog\n"
	var b strings.Builder
	b.Grow(n + len(line))
	for b.Len() < n {
		b.WriteString(line)
	}
	return []byte(b.String()[:n])
}

// coloredLogData returns n bytes shaped like real colorized log output: a
// colored level tag, plain-text message, and a reset on every line. This is
// the shape most callers actually feed the scanner.
func coloredLogData(n int) []byte {
	const line = "\x1b[32mINFO\x1b[0m request completed in 12ms path=/health\n"
	var b strings.Builder
	b.Grow(n + len(line))
	for b.Len() < n {
		b.WriteString(line)
	}
	return []byte(b.String()[:n])
}

// mixedSequenceData returns n bytes exercising every sequence family this
// package parses - CSI (SGR and cursor moves), OSC (title), and DCS - packed
// tightly with little text in between, so the benchmark stresses parseEscape
// itself rather than the plain-text fast path.
func mixedSequenceData(n int) []byte {
	const chunk = "\x1b[1;31merror\x1b[0m: \x1b]0;build failed\x07\x1b[2K\x1b[38:2:255:128:0mretrying\x1b[0m\x1bPq...\x1b\\\n"
	var b strings.Builder
	b.Grow(n + len(chunk))
	for b.Len() < n {
		b.WriteString(chunk)
	}
	return []byte(b.String()[:n])
}

// malformedData returns n bytes of introducer bytes that never resolve to a
// valid sequence, forcing lenient mode down its one-byte-at-a-time fallback
// path for the entire input.
func malformedData(n int) []byte {
	const chunk = "\x1b[\x1b]\x1bP\x1b"
	var b strings.Builder
	b.Grow(n + len(chunk))
	for b.Len() < n {
		b.WriteString(chunk)
	}
	return []byte(b.String()[:n])
}

func runTokenizeBenchmark(b *testing.B, data []byte, opts ...Option) {
	b.Helper()
	s := NewScanner(opts...)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.Tokenize(data); err != nil {
			b.Fatalf("Tokenize: %v", err)
		}
	}
}

func BenchmarkTokenizePlainText(b *testing.B) {
	runTokenizeBenchmark(b, plainTextData(1<<20))
}

func BenchmarkTokenizeColoredLog(b *testing.B) {
	runTokenizeBenchmark(b, coloredLogData(1<<20))
}

func BenchmarkTokenizeMixedSequences(b *testing.B) {
	runTokenizeBenchmark(b, mixedSequenceData(1<<20))
}

func BenchmarkTokenizeLenientMalformed(b *testing.B) {
	runTokenizeBenchmark(b, malformedData(1<<20), WithLenient())
}

func BenchmarkStrip(b *testing.B) {
	data := coloredLogData(1 << 20)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Strip(data)
	}
}

func BenchmarkDecodeSGR(b *testing.B) {
	s := NewScanner()
	tokens, err := s.Tokenize([]byte("\x1b[1;38:2:255:128:0;4m"))
	if err != nil {
		b.Fatalf("Tokenize: %v", err)
	}
	seq := tokens[0].Seq
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := DecodeSGR(seq); err != nil {
			b.Fatalf("DecodeSGR: %v", err)
		}
	}
}
