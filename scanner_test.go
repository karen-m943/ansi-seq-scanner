package ansiseq

import (
	"errors"
	"reflect"
	"testing"
)

func TestTokenizeCSI(t *testing.T) {
	in := []byte("\x1b[31mhello\x1b[0m")
	tokens, err := NewScanner().Tokenize(in)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	if len(tokens) != 3 {
		t.Fatalf("got %d tokens, want 3: %+v", len(tokens), tokens)
	}

	red := tokens[0]
	if red.Kind != Escape || red.Seq.Type != SeqCSI || red.Seq.Final != 'm' {
		t.Fatalf("token 0 = %+v, want CSI 'm'", red)
	}
	if !reflect.DeepEqual(red.Seq.Params, []int{31}) {
		t.Fatalf("params = %v, want [31]", red.Seq.Params)
	}

	if tokens[1].Kind != Text || tokens[1].Text != "hello" {
		t.Fatalf("token 1 = %+v, want text %q", tokens[1], "hello")
	}

	reset := tokens[2]
	if reset.Kind != Escape || !reflect.DeepEqual(reset.Seq.Params, []int{0}) {
		t.Fatalf("token 2 = %+v, want CSI params [0]", reset)
	}
}

func TestTokenizeOmittedParam(t *testing.T) {
	// "\x1b[;1m" has an omitted first parameter, which must become -1
	// rather than 0 - terminals treat "omitted" and "explicit zero"
	// differently for some SGR codes.
	tokens, err := NewScanner().Tokenize([]byte("\x1b[;1m"))
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	want := []int{-1, 1}
	if !reflect.DeepEqual(tokens[0].Seq.Params, want) {
		t.Fatalf("params = %v, want %v", tokens[0].Seq.Params, want)
	}
}

func TestTokenizeOSC(t *testing.T) {
	tokens, err := NewScanner().Tokenize([]byte("\x1b]0;window title\x07"))
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	if len(tokens) != 1 || tokens[0].Seq.Type != SeqOSC {
		t.Fatalf("tokens = %+v, want single OSC token", tokens)
	}
	if tokens[0].Seq.Data != "0;window title" {
		t.Fatalf("data = %q, want %q", tokens[0].Seq.Data, "0;window title")
	}
}

func TestTokenizeSimple(t *testing.T) {
	tokens, err := NewScanner().Tokenize([]byte("\x1b(B"))
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	seq := tokens[0].Seq
	if seq.Type != SeqSimple || seq.Final != 'B' || string(seq.Intermediates) != "(" {
		t.Fatalf("seq = %+v, want simple '(' 'B'", seq)
	}
}

func TestStrictRejectsUnterminatedCSI(t *testing.T) {
	_, err := NewScanner().Tokenize([]byte("\x1b[31"))
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("err = %v, want *ParseError", err)
	}
	if parseErr.Offset != 0 {
		t.Fatalf("offset = %d, want 0", parseErr.Offset)
	}
}

func TestLenientPassesThroughMalformedInput(t *testing.T) {
	in := []byte("\x1b[31")
	tokens, err := NewScanner(WithLenient()).Tokenize(in)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	if got := PlainText(tokens); got != string(in) {
		t.Fatalf("PlainText = %q, want %q", got, string(in))
	}
	for _, tok := range tokens {
		if tok.Kind != Text {
			t.Fatalf("token %+v, want all tokens to be text in lenient mode here", tok)
		}
	}
}

func TestPlainTextStripsEscapes(t *testing.T) {
	in := []byte("\x1b[1mbold\x1b[0m and \x1b]0;title\x07plain")
	tokens, err := NewScanner().Tokenize(in)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	if got, want := PlainText(tokens), "bold and plain"; got != want {
		t.Fatalf("PlainText = %q, want %q", got, want)
	}
}
