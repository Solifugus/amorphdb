// End-to-end tests that MBL text literals evaluate to the correct string
// values through the interpreter, for both the simple "…" form and the
// extended _"…"_ form used to embed quote characters.
package interpreter

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/types"
)

func TestTextLiteralValues(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"simple", `x = "plain text"`, `plain text`},
		{"simple empty", `x = ""`, ``},
		{"simple with punctuation", `x = "a, b; c!"`, `a, b; c!`},
		{"extended plain", `x = _"plain text"_`, `plain text`},
		{"extended embedded quotes", `x = _"He said "Hello" to me"_`, `He said "Hello" to me`},
		{"extended trailing quote", `x = _"say "hi""_`, `say "hi"`},
		{"extended doubled run", `x = _""Complex "quoted" text""_`, `Complex "quoted" text`},
		{"extended guards quote-underscore", `x = _""has "_ inside""_`, `has "_ inside`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, interp := freshTreeInterp(t, "textlit")
			res := runMBL(t, interp, tt.src)

			got, ok := res.(types.Text)
			if !ok {
				t.Fatalf("%s evaluated to %T (%v), want types.Text", tt.src, res, res)
			}
			if got.Value != tt.want {
				t.Errorf("%s\n  got  %q\n  want %q", tt.src, got.Value, tt.want)
			}
		})
	}
}

// TestTextLiteralRoundTripsThroughStorage confirms the unwrapped value is what
// actually gets stored and read back, not the delimited token text.
func TestTextLiteralRoundTripsThroughStorage(t *testing.T) {
	_, interp := freshTreeInterp(t, "textlit-storage")

	runMBL(t, interp, `my.quote = _"She said "yes" twice"_`)
	res := runMBL(t, interp, `my.quote`)

	got, ok := res.(types.Text)
	if !ok {
		t.Fatalf("read back %T (%v), want types.Text", res, res)
	}
	if want := `She said "yes" twice`; got.Value != want {
		t.Errorf("stored value\n  got  %q\n  want %q", got.Value, want)
	}
}

// TestTextLiteralConcatenation exercises a quoted value in an expression, since
// the '&' operator works on the unwrapped text.
func TestTextLiteralConcatenation(t *testing.T) {
	_, interp := freshTreeInterp(t, "textlit-concat")

	res := runMBL(t, interp, `x = _"a "quoted" word"_ & " and more"`)

	got, ok := res.(types.Text)
	if !ok {
		t.Fatalf("evaluated to %T (%v), want types.Text", res, res)
	}
	if want := `a "quoted" word and more`; got.Value != want {
		t.Errorf("concatenation\n  got  %q\n  want %q", got.Value, want)
	}
}
