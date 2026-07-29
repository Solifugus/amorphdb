// End-to-end tests for interpolating text literals, ~"…{my.path}…"~ .
// Interpolation is path-only and concatenates through the same coercion as the
// & operator.
package interpreter

import (
	"strings"
	"testing"

	"github.com/solifugus/amorphdb/internal/mbl/lexer"
	"github.com/solifugus/amorphdb/internal/mbl/parser"
	"github.com/solifugus/amorphdb/internal/types"
)

func TestInterpolationValues(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			"single placeholder",
			`my.user.name = "Alice"` + "\n" + `x = ~"my name is {my.user.name} and I like that name"~`,
			`my name is Alice and I like that name`,
		},
		{
			"placeholder at start and end",
			`my.a = "A"` + "\n" + `my.b = "B"` + "\n" + `x = ~"{my.a} middle {my.b}"~`,
			`A middle B`,
		},
		{
			"no placeholders",
			`x = ~"just text"~`,
			`just text`,
		},
		{
			"escaped brace",
			`x = ~"a {{ literal brace"~`,
			`a { literal brace`,
		},
		{
			"unmatched closing brace is literal",
			`x = ~"a } brace"~`,
			`a } brace`,
		},
		{
			"embedded quotes and a path",
			`my.user.name = "Alice"` + "\n" + `x = ~"He said "hi" to {my.user.name}"~`,
			`He said "hi" to Alice`,
		},
		{
			"whitespace inside braces is trimmed",
			`my.a = "X"` + "\n" + `x = ~"v={ my.a }"~`,
			`v=X`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, interp := freshTreeInterp(t, "interp")
			res := runMBL(t, interp, tt.src)

			got, ok := res.(types.Text)
			if !ok {
				t.Fatalf("evaluated to %T (%v), want types.Text", res, res)
			}
			if got.Value != tt.want {
				t.Errorf("\n  got  %q\n  want %q", got.Value, tt.want)
			}
		})
	}
}

// TestInterpolationCoercionMatchesConcat is the important guarantee: a value
// rendered by interpolation is identical to the same value rendered by &.
func TestInterpolationCoercionMatchesConcat(t *testing.T) {
	values := []string{`42`, `3.5`, `true`, `$12.50`, `@2026-01-15`, `"text"`}

	for _, v := range values {
		_, interp := freshTreeInterp(t, "coerce")

		runMBL(t, interp, `my.v = `+v)
		viaInterp := runMBL(t, interp, `~"[{my.v}]"~`)
		viaConcat := runMBL(t, interp, `"[" & my.v & "]"`)

		gotI, ok := viaInterp.(types.Text)
		if !ok {
			t.Fatalf("%s: interpolation gave %T", v, viaInterp)
		}
		gotC, ok := viaConcat.(types.Text)
		if !ok {
			t.Fatalf("%s: concatenation gave %T", v, viaConcat)
		}
		if gotI.Value != gotC.Value {
			t.Errorf("%s: interpolation %q != concatenation %q", v, gotI.Value, gotC.Value)
		}
	}
}

// TestInterpolationMultiLine covers the long-template case: newlines inside the
// literal are preserved and placeholders still resolve.
func TestInterpolationMultiLine(t *testing.T) {
	src := "my.user.name = \"Alice\"\nmy.order.id = 7\nx = ~\"Dear {my.user.name},\n\nYour order {my.order.id} has shipped.\n\nThanks!\"~"

	_, interp := freshTreeInterp(t, "multiline")
	res := runMBL(t, interp, src)

	got, ok := res.(types.Text)
	if !ok {
		t.Fatalf("evaluated to %T (%v), want types.Text", res, res)
	}
	want := "Dear Alice,\n\nYour order 7 has shipped.\n\nThanks!"
	if got.Value != want {
		t.Errorf("\n  got  %q\n  want %q", got.Value, want)
	}
}

// TestInterpolationRoundTripsThroughStorage confirms the resolved text is what
// gets stored, not the template.
func TestInterpolationRoundTripsThroughStorage(t *testing.T) {
	_, interp := freshTreeInterp(t, "interp-storage")

	runMBL(t, interp, `my.user.name = "Alice"`)
	runMBL(t, interp, `my.greeting = ~"Hello {my.user.name}"~`)
	res := runMBL(t, interp, `my.greeting`)

	got, ok := res.(types.Text)
	if !ok {
		t.Fatalf("read back %T (%v), want types.Text", res, res)
	}
	if want := `Hello Alice`; got.Value != want {
		t.Errorf("stored value\n  got  %q\n  want %q", got.Value, want)
	}
}

// TestInterpolationRejectsNonPaths is the primary failure case: interpolation is
// deliberately path-only, so expressions and calls must be parse errors rather
// than silently evaluated.
func TestInterpolationRejectsNonPaths(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr string
	}{
		{"arithmetic", `x = ~"sum {1 + 2}"~`, "only paths may be interpolated"},
		{"call", `x = ~"call {foo(1)}"~`, "only paths may be interpolated"},
		{"literal", `x = ~"lit {42}"~`, "only paths may be interpolated"},
		{"unclosed brace", `x = ~"unclosed {my.a"~`, "unclosed {"},
		{"empty braces", `x = ~"empty {}"~`, "empty {}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(lexer.New(tt.src))
			p.ParseProgram()

			errs := p.Errors()
			if len(errs) == 0 {
				t.Fatalf("expected a parse error for %s", tt.src)
			}
			joined := strings.Join(errs, "; ")
			if !strings.Contains(joined, tt.wantErr) {
				t.Errorf("errors %q do not mention %q", joined, tt.wantErr)
			}
		})
	}
}

// TestInterpolationOfMissingPathIsAbsorbed documents that an unresolved path
// does not make the whole literal Unknown — it renders like & does.
func TestInterpolationOfMissingPathIsAbsorbed(t *testing.T) {
	_, interp := freshTreeInterp(t, "missing")
	res := runMBL(t, interp, `x = ~"value: {my.missing.thing}"~`)

	got, ok := res.(types.Text)
	if !ok {
		t.Fatalf("evaluated to %T (%v), want types.Text", res, res)
	}
	if !strings.HasPrefix(got.Value, "value: ") {
		t.Errorf("literal text was lost: %q", got.Value)
	}
}
