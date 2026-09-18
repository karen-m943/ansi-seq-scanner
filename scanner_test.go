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

func TestTokenizeCSISubParams(t *testing.T) {
	// "\x1b[38:2:255:0:0m" is the SGR true-color form: parameter 38
	// carries three colon-separated sub-parameters (colorspace 2, then
	// R, G, B).
	tokens, err := NewScanner().Tokenize([]byte("\x1b[38:2:255:0:0m"))
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	seq := tokens[0].Seq
	if !reflect.DeepEqual(seq.Params, []int{38}) {
		t.Fatalf("params = %v, want [38]", seq.Params)
	}
	if !reflect.DeepEqual(seq.SubParams, [][]int{{2, 255, 0, 0}}) {
		t.Fatalf("subParams = %v, want [[2 255 0 0]]", seq.SubParams)
	}
}

func TestTokenizeCSISubParamsMixedWithPlainParams(t *testing.T) {
	// A field with no colon must leave SubParams nil at that index, even
	// when other fields in the same sequence do have sub-parameters.
	tokens, err := NewScanner().Tokenize([]byte("\x1b[1;4:3m"))
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	seq := tokens[0].Seq
	if !reflect.DeepEqual(seq.Params, []int{1, 4}) {
		t.Fatalf("params = %v, want [1 4]", seq.Params)
	}
	if seq.SubParams[0] != nil {
		t.Fatalf("subParams[0] = %v, want nil", seq.SubParams[0])
	}
	if !reflect.DeepEqual(seq.SubParams[1], []int{3}) {
		t.Fatalf("subParams[1] = %v, want [3]", seq.SubParams[1])
	}
}

func TestTokenizeCSISubParamsOmittedValue(t *testing.T) {
	// An omitted sub-parameter ("4::3") becomes -1, same convention as
	// an omitted top-level parameter.
	tokens, err := NewScanner().Tokenize([]byte("\x1b[4::3m"))
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	seq := tokens[0].Seq
	if !reflect.DeepEqual(seq.SubParams[0], []int{-1, 3}) {
		t.Fatalf("subParams[0] = %v, want [-1 3]", seq.SubParams[0])
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

func TestTokenizeC1CSI(t *testing.T) {
	// 0x9B is the 8-bit C1 form of "ESC [".
	tokens, err := NewScanner().Tokenize([]byte("\x9B31mhello"))
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	seq := tokens[0].Seq
	if seq.Type != SeqCSI || seq.Final != 'm' || !reflect.DeepEqual(seq.Params, []int{31}) {
		t.Fatalf("seq = %+v, want CSI 'm' params [31]", seq)
	}
	if seq.Raw != "\x9B31m" {
		t.Fatalf("raw = %q, want %q", seq.Raw, "\x9B31m")
	}
	if tokens[1].Kind != Text || tokens[1].Text != "hello" {
		t.Fatalf("token 1 = %+v, want text %q", tokens[1], "hello")
	}
}

func TestTokenizeC1OSC(t *testing.T) {
	// 0x9D is the 8-bit C1 form of "ESC ]"; 0x9C (ST) terminates it
	// without needing "ESC \".
	tokens, err := NewScanner().Tokenize([]byte("\x9D0;title\x9C"))
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	seq := tokens[0].Seq
	if seq.Type != SeqOSC || seq.Data != "0;title" || seq.Final != 0x9C {
		t.Fatalf("seq = %+v, want OSC data %q final 0x9C", seq, "0;title")
	}
}

func TestTokenizeC1DCS(t *testing.T) {
	// 0x90 is the 8-bit C1 form of "ESC P"; 0x9C (ST) terminates it.
	// 'q' is the Sixel command byte; everything after it up to the
	// terminator is opaque data.
	tokens, err := NewScanner().Tokenize([]byte("\x90qpayload\x9C"))
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	seq := tokens[0].Seq
	if seq.Type != SeqDCS || seq.Command != 'q' || seq.Data != "payload" || seq.Final != 0x9C {
		t.Fatalf("seq = %+v, want DCS command 'q' data %q final 0x9C", seq, "payload")
	}
}

func TestTokenizeDCSHeader(t *testing.T) {
	// DECRQSS-style header: params "1;2", intermediate '$', command
	// 'q', followed by opaque data terminated by ESC \.
	tokens, err := NewScanner().Tokenize([]byte("\x1bP1;2$qpayload\x1b\\"))
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	seq := tokens[0].Seq
	if seq.Type != SeqDCS {
		t.Fatalf("type = %v, want SeqDCS", seq.Type)
	}
	if !reflect.DeepEqual(seq.Params, []int{1, 2}) {
		t.Fatalf("params = %v, want [1 2]", seq.Params)
	}
	if string(seq.Intermediates) != "$" {
		t.Fatalf("intermediates = %q, want %q", seq.Intermediates, "$")
	}
	if seq.Command != 'q' {
		t.Fatalf("command = %q, want 'q'", seq.Command)
	}
	if seq.Data != "payload" {
		t.Fatalf("data = %q, want %q", seq.Data, "payload")
	}
	if seq.Final != '\\' {
		t.Fatalf("final = %q, want '\\\\'", seq.Final)
	}
}

func TestStrictRejectsDCSWithoutCommandByte(t *testing.T) {
	_, err := NewScanner().Tokenize([]byte("\x1bP\x9C"))
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("err = %v, want *ParseError", err)
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

func TestStrip(t *testing.T) {
	in := []byte("\x1b[1mbold\x1b[0m and \x1b]0;title\x07plain")
	if got, want := Strip(in), "bold and plain"; got != want {
		t.Fatalf("Strip = %q, want %q", got, want)
	}
}

func TestStripToleratesMalformedInput(t *testing.T) {
	// Strip is meant for the "just give me the text" case, so it must
	// never fail even on truncated or invalid escape sequences - unlike
	// Tokenize in strict mode, which would return a *ParseError here.
	in := []byte("keep\x1b[31")
	if got, want := Strip(in), "keep\x1b[31"; got != want {
		t.Fatalf("Strip = %q, want %q", got, want)
	}
}
