package ansiseq

import "strings"

// TokenKind identifies what a Token holds.
type TokenKind int

const (
	// Text is a run of ordinary bytes with no escape sequences in it.
	Text TokenKind = iota
	// Escape is one fully parsed escape sequence.
	Escape
)

func (k TokenKind) String() string {
	switch k {
	case Text:
		return "text"
	case Escape:
		return "escape"
	default:
		return "unknown"
	}
}

// SeqType identifies which family of escape sequence a Sequence came from.
type SeqType int

const (
	// SeqCSI is ESC [ params intermediates final (cursor moves, SGR colors, ...).
	SeqCSI SeqType = iota
	// SeqOSC is ESC ] data (BEL | ST) (window title, hyperlinks, ...).
	SeqOSC
	// SeqDCS is ESC P data ST (device control strings, e.g. Sixel, tmux passthrough).
	SeqDCS
	// SeqSimple is a two- or three-byte escape with no parameter block,
	// e.g. ESC 7 (save cursor) or ESC ( B (select ASCII charset).
	SeqSimple
)

func (t SeqType) String() string {
	switch t {
	case SeqCSI:
		return "CSI"
	case SeqOSC:
		return "OSC"
	case SeqDCS:
		return "DCS"
	case SeqSimple:
		return "simple"
	default:
		return "unknown"
	}
}

// Sequence holds the parsed structure of one escape sequence. Which fields
// are populated depends on Type: Params/Private/Intermediates belong to
// SeqCSI and SeqSimple, Data belongs to SeqOSC and SeqDCS.
type Sequence struct {
	Type SeqType
	Raw  string // exact bytes consumed, ESC through the final byte inclusive

	// CSI / simple
	Private       byte  // leading private-marker byte (one of < = > ?), 0 if none
	Params        []int // numeric parameters; an omitted parameter is -1
	Intermediates []byte
	Final         byte

	// OSC / DCS
	Data string // payload between the introducer and the terminator
}

// Token is either a run of ordinary text or one parsed escape sequence.
type Token struct {
	Kind TokenKind
	Text string
	Seq  Sequence
}

// PlainText concatenates the Text of every Text token, discarding every
// escape sequence. This is the common case for turning captured terminal
// output into something safe to log, diff, or search.
func PlainText(tokens []Token) string {
	var b strings.Builder
	for _, t := range tokens {
		if t.Kind == Text {
			b.WriteString(t.Text)
		}
	}
	return b.String()
}
