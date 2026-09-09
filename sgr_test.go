package ansiseq

import (
	"reflect"
	"testing"
)

func decodeOne(t *testing.T, in string) []Attribute {
	t.Helper()
	tokens, err := NewScanner().Tokenize([]byte(in))
	if err != nil {
		t.Fatalf("Tokenize(%q): %v", in, err)
	}
	if len(tokens) != 1 || tokens[0].Kind != Escape {
		t.Fatalf("Tokenize(%q) = %+v, want a single escape token", in, tokens)
	}
	attrs, err := DecodeSGR(tokens[0].Seq)
	if err != nil {
		t.Fatalf("DecodeSGR(%q): %v", in, err)
	}
	return attrs
}

func TestDecodeSGRSimpleAttrs(t *testing.T) {
	attrs := decodeOne(t, "\x1b[1;4;7m")
	want := []Attribute{
		{Kind: AttrBold},
		{Kind: AttrUnderline},
		{Kind: AttrReverse},
	}
	if !reflect.DeepEqual(attrs, want) {
		t.Fatalf("attrs = %+v, want %+v", attrs, want)
	}
}

func TestDecodeSGREmptyMeansReset(t *testing.T) {
	attrs := decodeOne(t, "\x1b[m")
	want := []Attribute{{Kind: AttrReset}}
	if !reflect.DeepEqual(attrs, want) {
		t.Fatalf("attrs = %+v, want %+v", attrs, want)
	}
}

func TestDecodeSGROmittedParamMeansReset(t *testing.T) {
	attrs := decodeOne(t, "\x1b[;1m")
	want := []Attribute{{Kind: AttrReset}, {Kind: AttrBold}}
	if !reflect.DeepEqual(attrs, want) {
		t.Fatalf("attrs = %+v, want %+v", attrs, want)
	}
}

func TestDecodeSGRBasicForeground(t *testing.T) {
	attrs := decodeOne(t, "\x1b[31m")
	want := []Attribute{{Kind: AttrForeground, Color: Color{Mode: ColorBasic, Index: 1}}}
	if !reflect.DeepEqual(attrs, want) {
		t.Fatalf("attrs = %+v, want %+v", attrs, want)
	}
}

func TestDecodeSGRBrightBackground(t *testing.T) {
	attrs := decodeOne(t, "\x1b[102m")
	want := []Attribute{{Kind: AttrBackground, Color: Color{Mode: ColorBasic, Index: 10}}}
	if !reflect.DeepEqual(attrs, want) {
		t.Fatalf("attrs = %+v, want %+v", attrs, want)
	}
}

func TestDecodeSGRDefaultColors(t *testing.T) {
	attrs := decodeOne(t, "\x1b[39;49m")
	want := []Attribute{{Kind: AttrDefaultForeground}, {Kind: AttrDefaultBackground}}
	if !reflect.DeepEqual(attrs, want) {
		t.Fatalf("attrs = %+v, want %+v", attrs, want)
	}
}

func TestDecodeSGRIndexedColorLegacyForm(t *testing.T) {
	attrs := decodeOne(t, "\x1b[38;5;208m")
	want := []Attribute{{Kind: AttrForeground, Color: Color{Mode: ColorIndexed, Index: 208}}}
	if !reflect.DeepEqual(attrs, want) {
		t.Fatalf("attrs = %+v, want %+v", attrs, want)
	}
}

func TestDecodeSGRTrueColorLegacyForm(t *testing.T) {
	attrs := decodeOne(t, "\x1b[48;2;10;20;30m")
	want := []Attribute{{Kind: AttrBackground, Color: Color{Mode: ColorRGB, R: 10, G: 20, B: 30}}}
	if !reflect.DeepEqual(attrs, want) {
		t.Fatalf("attrs = %+v, want %+v", attrs, want)
	}
}

func TestDecodeSGRTrueColorSubParamForm(t *testing.T) {
	attrs := decodeOne(t, "\x1b[38:2:255:0:0m")
	want := []Attribute{{Kind: AttrForeground, Color: Color{Mode: ColorRGB, R: 255, G: 0, B: 0}}}
	if !reflect.DeepEqual(attrs, want) {
		t.Fatalf("attrs = %+v, want %+v", attrs, want)
	}
}

func TestDecodeSGRColorThenMoreAttrs(t *testing.T) {
	// The legacy color form must consume exactly its own parameters and
	// leave the loop positioned so the next real parameter still gets
	// decoded, not skipped or reprocessed as part of the color.
	attrs := decodeOne(t, "\x1b[38;5;208;1m")
	want := []Attribute{
		{Kind: AttrForeground, Color: Color{Mode: ColorIndexed, Index: 208}},
		{Kind: AttrBold},
	}
	if !reflect.DeepEqual(attrs, want) {
		t.Fatalf("attrs = %+v, want %+v", attrs, want)
	}
}

func TestDecodeSGRUnknownParam(t *testing.T) {
	attrs := decodeOne(t, "\x1b[73m")
	want := []Attribute{{Kind: AttrUnknown, Param: 73}}
	if !reflect.DeepEqual(attrs, want) {
		t.Fatalf("attrs = %+v, want %+v", attrs, want)
	}
}

func TestDecodeSGRTruncatedTrueColorErrors(t *testing.T) {
	tokens, err := NewScanner().Tokenize([]byte("\x1b[38;2;255m"))
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	if _, err := DecodeSGR(tokens[0].Seq); err == nil {
		t.Fatalf("DecodeSGR: got nil error, want error for truncated true-color parameters")
	}
}

func TestDecodeSGRRejectsNonSGRSequence(t *testing.T) {
	tokens, err := NewScanner().Tokenize([]byte("\x1b[2J"))
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	if _, err := DecodeSGR(tokens[0].Seq); err == nil {
		t.Fatalf("DecodeSGR: got nil error, want error for non-SGR CSI sequence")
	}
}

func TestDecodeSGRRejectsPrivateMarker(t *testing.T) {
	seq := Sequence{Type: SeqCSI, Private: '?', Params: []int{1}, Final: 'm'}
	if _, err := DecodeSGR(seq); err == nil {
		t.Fatalf("DecodeSGR: got nil error, want error for private-marker CSI sequence")
	}
}
