package ansiseq

import "fmt"

// SGRAttr identifies one named SGR (Select Graphic Rendition) attribute -
// what a "\x1b[...m" sequence actually asks the terminal to do.
type SGRAttr int

const (
	AttrReset SGRAttr = iota
	AttrBold
	AttrFaint
	AttrItalic
	AttrUnderline
	AttrDoubleUnderline
	AttrBlink
	AttrRapidBlink
	AttrReverse
	AttrConceal
	AttrStrikethrough
	AttrNormalIntensity // 22: cancels bold and faint
	AttrNotItalic
	AttrNotUnderlined // cancels single and double underline
	AttrNotBlinking
	AttrNotReversed
	AttrReveal // cancels conceal
	AttrNotStrikethrough
	AttrForeground
	AttrBackground
	AttrDefaultForeground
	AttrDefaultBackground
	// AttrUnknown marks a numeric SGR parameter this package does not
	// assign a name to. Param holds the raw value so callers can still
	// act on it instead of losing it.
	AttrUnknown
)

func (a SGRAttr) String() string {
	switch a {
	case AttrReset:
		return "reset"
	case AttrBold:
		return "bold"
	case AttrFaint:
		return "faint"
	case AttrItalic:
		return "italic"
	case AttrUnderline:
		return "underline"
	case AttrDoubleUnderline:
		return "double-underline"
	case AttrBlink:
		return "blink"
	case AttrRapidBlink:
		return "rapid-blink"
	case AttrReverse:
		return "reverse"
	case AttrConceal:
		return "conceal"
	case AttrStrikethrough:
		return "strikethrough"
	case AttrNormalIntensity:
		return "normal-intensity"
	case AttrNotItalic:
		return "not-italic"
	case AttrNotUnderlined:
		return "not-underlined"
	case AttrNotBlinking:
		return "not-blinking"
	case AttrNotReversed:
		return "not-reversed"
	case AttrReveal:
		return "reveal"
	case AttrNotStrikethrough:
		return "not-strikethrough"
	case AttrForeground:
		return "foreground"
	case AttrBackground:
		return "background"
	case AttrDefaultForeground:
		return "default-foreground"
	case AttrDefaultBackground:
		return "default-background"
	case AttrUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// ColorMode identifies how a Color's fields should be read.
type ColorMode int

const (
	// ColorBasic is one of the 16 terminal-defined colors: 0-7 are the
	// standard set, 8-15 are the "bright" set (SGR 90-97 / 100-107).
	ColorBasic ColorMode = iota
	// ColorIndexed is a lookup into the 256-color palette (SGR
	// "38;5;n" or "48;5;n").
	ColorIndexed
	// ColorRGB is a 24-bit true color (SGR "38;2;r;g;b" or
	// "48;2;r;g;b").
	ColorRGB
)

func (m ColorMode) String() string {
	switch m {
	case ColorBasic:
		return "basic"
	case ColorIndexed:
		return "indexed"
	case ColorRGB:
		return "rgb"
	default:
		return "unknown"
	}
}

// Color holds a decoded SGR foreground or background color. Which
// fields are meaningful depends on Mode: Index for ColorBasic and
// ColorIndexed, R/G/B for ColorRGB.
type Color struct {
	Mode    ColorMode
	Index   uint8
	R, G, B uint8
}

// Attribute is one decoded SGR instruction. Color is populated only
// when Kind is AttrForeground or AttrBackground; Param is populated
// only when Kind is AttrUnknown.
type Attribute struct {
	Kind  SGRAttr
	Color Color
	Param int
}

// simpleSGRAttrs maps SGR codes with no argument to their named
// attribute. Codes handled separately (color selection, resets that
// need argument parsing) are not listed here.
var simpleSGRAttrs = map[int]SGRAttr{
	0:  AttrReset,
	1:  AttrBold,
	2:  AttrFaint,
	3:  AttrItalic,
	4:  AttrUnderline,
	5:  AttrBlink,
	6:  AttrRapidBlink,
	7:  AttrReverse,
	8:  AttrConceal,
	9:  AttrStrikethrough,
	21: AttrDoubleUnderline,
	22: AttrNormalIntensity,
	23: AttrNotItalic,
	24: AttrNotUnderlined,
	25: AttrNotBlinking,
	27: AttrNotReversed,
	28: AttrReveal,
	29: AttrNotStrikethrough,
}

// DecodeSGR interprets an SGR ("\x1b[...m") sequence's parameters as a
// list of named attributes, in the order they appeared. An empty
// parameter list ("\x1b[m") is treated as a single AttrReset, matching
// what every real terminal does with it.
//
// DecodeSGR only accepts sequences that Tokenize has already validated
// as well-formed CSI; it returns an error if seq is not CSI "m", has a
// private marker (SGR never does), or its color parameters are
// incomplete or out of range.
func DecodeSGR(seq Sequence) ([]Attribute, error) {
	if seq.Type != SeqCSI || seq.Final != 'm' {
		return nil, fmt.Errorf("ansiseq: DecodeSGR: not an SGR sequence (type %v final %q)", seq.Type, seq.Final)
	}
	if seq.Private != 0 {
		return nil, fmt.Errorf("ansiseq: DecodeSGR: SGR sequence cannot carry a private marker %q", seq.Private)
	}

	params := seq.Params
	if len(params) == 0 {
		params = []int{0}
	}

	var attrs []Attribute
	for i := 0; i < len(params); i++ {
		p := params[i]
		if p == -1 {
			p = 0 // an omitted SGR parameter defaults to reset, same as an explicit 0
		}

		switch {
		case p == 38 || p == 48:
			var sub []int
			if i < len(seq.SubParams) {
				sub = seq.SubParams[i]
			}
			color, extra, err := decodeSGRColor(params, sub, i)
			if err != nil {
				return nil, err
			}
			kind := AttrForeground
			if p == 48 {
				kind = AttrBackground
			}
			attrs = append(attrs, Attribute{Kind: kind, Color: color})
			i += extra
		case p == 39:
			attrs = append(attrs, Attribute{Kind: AttrDefaultForeground})
		case p == 49:
			attrs = append(attrs, Attribute{Kind: AttrDefaultBackground})
		case p >= 30 && p <= 37:
			attrs = append(attrs, Attribute{Kind: AttrForeground, Color: Color{Mode: ColorBasic, Index: uint8(p - 30)}})
		case p >= 40 && p <= 47:
			attrs = append(attrs, Attribute{Kind: AttrBackground, Color: Color{Mode: ColorBasic, Index: uint8(p - 40)}})
		case p >= 90 && p <= 97:
			attrs = append(attrs, Attribute{Kind: AttrForeground, Color: Color{Mode: ColorBasic, Index: uint8(p-90) + 8}})
		case p >= 100 && p <= 107:
			attrs = append(attrs, Attribute{Kind: AttrBackground, Color: Color{Mode: ColorBasic, Index: uint8(p-100) + 8}})
		default:
			if kind, ok := simpleSGRAttrs[p]; ok {
				attrs = append(attrs, Attribute{Kind: kind})
			} else {
				attrs = append(attrs, Attribute{Kind: AttrUnknown, Param: p})
			}
		}
	}
	return attrs, nil
}

// decodeSGRColor decodes the color that follows a 38 or 48 parameter at
// params[i]. It prefers colon-separated sub-parameters (sub) when
// present, since that's the unambiguous ITU T.416 form; otherwise it
// falls back to reading the mode and components as their own
// semicolon-separated params, which is what most real terminals emit
// and accept. It returns the extra number of entries in params that
// the legacy form consumed, so the caller can skip over them; that's
// always 0 for the sub-parameter form, since those are already part of
// params[i].
func decodeSGRColor(params []int, sub []int, i int) (Color, int, error) {
	if len(sub) > 0 {
		switch sub[0] {
		case 2:
			if len(sub) < 4 {
				return Color{}, 0, fmt.Errorf("ansiseq: DecodeSGR: true-color sub-parameters need r, g, b, got %v", sub)
			}
			return decodeRGB(sub[1], sub[2], sub[3])
		case 5:
			if len(sub) < 2 {
				return Color{}, 0, fmt.Errorf("ansiseq: DecodeSGR: indexed-color sub-parameters need an index, got %v", sub)
			}
			return decodeIndexed(sub[1])
		default:
			return Color{}, 0, fmt.Errorf("ansiseq: DecodeSGR: unsupported color mode %d", sub[0])
		}
	}

	if i+1 >= len(params) {
		return Color{}, 0, fmt.Errorf("ansiseq: DecodeSGR: missing color mode after parameter %d", params[i])
	}
	switch params[i+1] {
	case 2:
		if i+4 >= len(params) {
			return Color{}, 0, fmt.Errorf("ansiseq: DecodeSGR: true-color parameters need r, g, b after mode 2")
		}
		c, _, err := decodeRGB(params[i+2], params[i+3], params[i+4])
		return c, 4, err
	case 5:
		if i+2 >= len(params) {
			return Color{}, 0, fmt.Errorf("ansiseq: DecodeSGR: indexed-color parameters need an index after mode 5")
		}
		c, _, err := decodeIndexed(params[i+2])
		return c, 2, err
	default:
		return Color{}, 0, fmt.Errorf("ansiseq: DecodeSGR: unsupported color mode %d", params[i+1])
	}
}

func decodeRGB(r, g, b int) (Color, int, error) {
	if r < 0 || r > 255 || g < 0 || g > 255 || b < 0 || b > 255 {
		return Color{}, 0, fmt.Errorf("ansiseq: DecodeSGR: true-color component out of range (%d, %d, %d)", r, g, b)
	}
	return Color{Mode: ColorRGB, R: uint8(r), G: uint8(g), B: uint8(b)}, 0, nil
}

func decodeIndexed(n int) (Color, int, error) {
	if n < 0 || n > 255 {
		return Color{}, 0, fmt.Errorf("ansiseq: DecodeSGR: indexed color out of range (%d)", n)
	}
	return Color{Mode: ColorIndexed, Index: uint8(n)}, 0, nil
}
