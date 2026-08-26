// Package ansiseq tokenizes raw terminal output into text and escape
// sequences. It parses the sequence grammar (CSI, OSC, DCS, and the
// short two/three-byte ESC forms) without interpreting what any given
// sequence means - that interpretation is left to the caller.
package ansiseq

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	esc = 0x1B
	bel = 0x07

	// maxParams and maxParamValue bound how much a single malformed CSI
	// sequence can make the scanner allocate. They are set well above
	// anything a real terminal program emits (xterm caps at 16 params).
	maxParams     = 32
	maxParamValue = 16384
)

// ParseError is returned by Tokenize in strict mode when the input
// contains a byte sequence that does not match the escape sequence
// grammar. Offset is the index into the input where the offending
// escape sequence started (the ESC byte itself).
type ParseError struct {
	Offset int
	Reason string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("ansiseq: at byte %d: %s", e.Offset, e.Reason)
}

// Scanner tokenizes byte streams containing terminal escape sequences.
// The zero value is ready to use in strict mode; use NewScanner with
// WithLenient to relax validation.
type Scanner struct {
	lenient bool
}

// Option configures a Scanner.
type Option func(*Scanner)

// WithLenient turns off strict validation. Malformed or unterminated
// escape sequences are passed through as literal text instead of
// causing Tokenize to return an error. Reach for this when you are
// parsing terminal output you did not generate and cannot trust to be
// well-formed - a captured session log, a pasted CI transcript - and
// would rather see garbage than lose the whole stream to one bad byte.
func WithLenient() Option {
	return func(s *Scanner) { s.lenient = true }
}

// NewScanner builds a Scanner. With no options it is strict: any byte
// sequence that starts with ESC but does not resolve to a complete,
// valid sequence causes Tokenize to stop and return a *ParseError.
func NewScanner(opts ...Option) *Scanner {
	s := &Scanner{}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Tokenize walks data and returns it as a sequence of text and escape
// tokens, in order. In strict mode (the default) it returns a
// *ParseError on the first malformed or unterminated escape sequence
// and no tokens. In lenient mode it never errors: an ESC byte that
// cannot be parsed as a full sequence is emitted as a one-byte text
// token and scanning continues from the next byte.
func (s *Scanner) Tokenize(data []byte) ([]Token, error) {
	var tokens []Token
	textStart := 0
	i := 0
	for i < len(data) {
		if data[i] != esc {
			i++
			continue
		}
		if i > textStart {
			tokens = append(tokens, Token{Kind: Text, Text: string(data[textStart:i])})
		}
		seq, next, err := parseEscape(data, i)
		if err != nil {
			if !s.lenient {
				return nil, err
			}
			tokens = append(tokens, Token{Kind: Text, Text: string(data[i])})
			i++
			textStart = i
			continue
		}
		tokens = append(tokens, Token{Kind: Escape, Seq: seq})
		i = next
		textStart = i
	}
	if textStart < len(data) {
		tokens = append(tokens, Token{Kind: Text, Text: string(data[textStart:])})
	}
	return tokens, nil
}

// parseEscape parses one escape sequence starting at data[start], where
// data[start] == esc. It returns the parsed Sequence and the index of
// the byte just past it.
func parseEscape(data []byte, start int) (Sequence, int, error) {
	if start+1 >= len(data) {
		return Sequence{}, 0, &ParseError{Offset: start, Reason: "ESC at end of input with no following byte"}
	}
	switch data[start+1] {
	case '[':
		return parseCSI(data, start)
	case ']':
		return parseOSC(data, start)
	case 'P':
		return parseDCS(data, start)
	default:
		return parseSimple(data, start)
	}
}

// parseCSI parses ESC [ params intermediates final.
func parseCSI(data []byte, start int) (Sequence, int, error) {
	i := start + 2

	paramStart := i
	for i < len(data) && data[i] >= 0x30 && data[i] <= 0x3F {
		i++
	}
	private, paramBytes := splitPrivateMarker(data[paramStart:i])

	intStart := i
	for i < len(data) && data[i] >= 0x20 && data[i] <= 0x2F {
		i++
	}
	intermediates := data[intStart:i]

	if i >= len(data) {
		return Sequence{}, 0, &ParseError{Offset: start, Reason: "unterminated CSI sequence (no final byte)"}
	}
	final := data[i]
	if final < 0x40 || final > 0x7E {
		return Sequence{}, 0, &ParseError{Offset: start, Reason: fmt.Sprintf("invalid CSI final byte 0x%02X", final)}
	}
	i++

	params, err := parseParams(paramBytes)
	if err != nil {
		return Sequence{}, 0, &ParseError{Offset: start, Reason: err.Error()}
	}

	return Sequence{
		Type:          SeqCSI,
		Raw:           string(data[start:i]),
		Private:       private,
		Params:        params,
		Intermediates: append([]byte(nil), intermediates...),
		Final:         final,
	}, i, nil
}

// splitPrivateMarker pulls a leading private-marker byte (< = > ?) off
// a CSI parameter block, if present.
func splitPrivateMarker(b []byte) (marker byte, rest []byte) {
	if len(b) > 0 {
		switch b[0] {
		case '<', '=', '>', '?':
			return b[0], b[1:]
		}
	}
	return 0, b
}

// parseParams splits a CSI parameter block on ';'. An empty field
// (two consecutive ';', or an empty block) becomes -1, meaning
// "omitted" - callers should treat that as the sequence's documented
// default for that position. Colon-separated sub-parameters are not
// yet supported and are rejected as non-numeric.
func parseParams(b []byte) ([]int, error) {
	if len(b) == 0 {
		return nil, nil
	}
	parts := strings.Split(string(b), ";")
	if len(parts) > maxParams {
		return nil, fmt.Errorf("too many CSI parameters (%d)", len(parts))
	}
	params := make([]int, len(parts))
	for idx, p := range parts {
		if p == "" {
			params[idx] = -1
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("non-numeric CSI parameter %q", p)
		}
		if n < 0 || n > maxParamValue {
			return nil, fmt.Errorf("CSI parameter %d out of range", n)
		}
		params[idx] = n
	}
	return params, nil
}

// parseOSC parses ESC ] data, terminated by BEL or ESC \ (ST).
func parseOSC(data []byte, start int) (Sequence, int, error) {
	i := start + 2
	dataStart := i
	for i < len(data) {
		switch {
		case data[i] == bel:
			return Sequence{
				Type:  SeqOSC,
				Raw:   string(data[start : i+1]),
				Data:  string(data[dataStart:i]),
				Final: bel,
			}, i + 1, nil
		case data[i] == esc:
			if i+1 < len(data) && data[i+1] == '\\' {
				return Sequence{
					Type:  SeqOSC,
					Raw:   string(data[start : i+2]),
					Data:  string(data[dataStart:i]),
					Final: '\\',
				}, i + 2, nil
			}
			return Sequence{}, 0, &ParseError{Offset: start, Reason: "ESC inside OSC string not followed by '\\' (malformed terminator)"}
		default:
			i++
		}
	}
	return Sequence{}, 0, &ParseError{Offset: start, Reason: "unterminated OSC sequence"}
}

// parseDCS parses ESC P data, terminated by ESC \ (ST). Unlike OSC,
// DCS never terminates on BEL. The parameter/intermediate prefix that
// can precede a DCS payload is treated as part of Data for now.
func parseDCS(data []byte, start int) (Sequence, int, error) {
	i := start + 2
	dataStart := i
	for i < len(data) {
		if data[i] == esc {
			if i+1 < len(data) && data[i+1] == '\\' {
				return Sequence{
					Type:  SeqDCS,
					Raw:   string(data[start : i+2]),
					Data:  string(data[dataStart:i]),
					Final: '\\',
				}, i + 2, nil
			}
			return Sequence{}, 0, &ParseError{Offset: start, Reason: "ESC inside DCS string not followed by '\\' (malformed terminator)"}
		}
		i++
	}
	return Sequence{}, 0, &ParseError{Offset: start, Reason: "unterminated DCS sequence"}
}

// parseSimple parses the short escape forms that are neither CSI, OSC,
// nor DCS: a bare final byte (ESC 7, ESC c, ...) or one intermediate
// byte followed by a final byte (ESC ( B, ESC ) 0, ...).
func parseSimple(data []byte, start int) (Sequence, int, error) {
	i := start + 1
	b := data[i]

	if b >= 0x20 && b <= 0x2F {
		if i+1 >= len(data) {
			return Sequence{}, 0, &ParseError{Offset: start, Reason: "unterminated escape sequence (missing final byte after intermediate)"}
		}
		final := data[i+1]
		if final < 0x30 || final > 0x7E {
			return Sequence{}, 0, &ParseError{Offset: start, Reason: fmt.Sprintf("invalid final byte 0x%02X after intermediate", final)}
		}
		return Sequence{
			Type:          SeqSimple,
			Raw:           string(data[start : i+2]),
			Intermediates: []byte{b},
			Final:         final,
		}, i + 2, nil
	}

	if b >= 0x30 && b <= 0x7E {
		return Sequence{
			Type:  SeqSimple,
			Raw:   string(data[start : i+1]),
			Final: b,
		}, i + 1, nil
	}

	return Sequence{}, 0, &ParseError{Offset: start, Reason: fmt.Sprintf("invalid byte 0x%02X after ESC", b)}
}
