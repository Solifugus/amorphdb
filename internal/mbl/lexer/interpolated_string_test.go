// Tests for the interpolating text literal form, ~"…"~ . It scans identically
// to the extended _"…"_ form; only the sigil and the resulting token type
// differ. Splitting the {path} placeholders is the parser's job.
package lexer

import "testing"

func TestInterpolatingLiteralsTokenize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantBody string
	}{
		{"simple", `~"hello"~`, `hello`},
		{"with placeholder", `~"my name is {my.user.name}"~`, `my name is {my.user.name}`},
		{"several placeholders", `~"{a.b} and {c.d}"~`, `{a.b} and {c.d}`},
		{"escaped brace", `~"literal {{ brace"~`, `literal {{ brace`},
		{"embedded quotes", `~"He said "hi" to {my.user.name}"~`, `He said "hi" to {my.user.name}`},
		{"doubled run", `~""guards a "~ sequence""~`, `guards a "~ sequence`},
		{"multi line", "~\"first {my.a}\nsecond\"~", "first {my.a}\nsecond"},
		{"empty", `~""""~`, ``},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			tok := l.NextToken()

			if tok.Type != INTERP_TEXT {
				t.Fatalf("token type = %v, want INTERP_TEXT (literal %q)", tok.Type, tok.Literal)
			}
			if tok.Literal != tt.input {
				t.Fatalf("literal = %q, want the whole input %q", tok.Literal, tt.input)
			}
			body, ok := UnquoteText(tok.Literal)
			if !ok {
				t.Fatalf("UnquoteText did not recognise %q", tok.Literal)
			}
			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

// TestInterpolatingAndExtendedFormsAreDistinct guards that the two sigils do not
// cross-terminate: _"…"_ must not be closed by "~ , and vice versa.
func TestInterpolatingAndExtendedFormsAreDistinct(t *testing.T) {
	l := New(`_"extended"_`)
	if tok := l.NextToken(); tok.Type != TEXT {
		t.Errorf(`_"extended"_ = %v, want TEXT`, tok.Type)
	}

	l = New(`~"interpolating"~`)
	if tok := l.NextToken(); tok.Type != INTERP_TEXT {
		t.Errorf(`~"interpolating"~ = %v, want INTERP_TEXT`, tok.Type)
	}

	// A literal opened with ~ is not closed by the extended sigil.
	l = New(`~"mismatched"_`)
	if tok := l.NextToken(); tok.Type != ILLEGAL {
		t.Errorf(`~"mismatched"_ = %v, want ILLEGAL`, tok.Type)
	}

	// A literal opened with _ is not closed by the interpolating sigil.
	l = New(`_"mismatched"~`)
	if tok := l.NextToken(); tok.Type != ILLEGAL {
		t.Errorf(`_"mismatched"~ = %v, want ILLEGAL`, tok.Type)
	}
}

func TestUnterminatedInterpolatingLiteralIsIllegal(t *testing.T) {
	for _, input := range []string{`~"no closing`, `~"closed wrong"`, `~""abc""`} {
		l := New(input)
		if tok := l.NextToken(); tok.Type != ILLEGAL {
			t.Errorf("%s: token type = %v, want ILLEGAL", input, tok.Type)
		}
	}
}

// TestTildeAloneIsNotAString guards the lookahead: '~' is only a string sigil
// when a quote follows it.
func TestTildeAloneIsNotAString(t *testing.T) {
	l := New(`~ x`)
	tok := l.NextToken()
	if tok.Type == INTERP_TEXT || tok.Type == TEXT {
		t.Errorf("bare ~ lexed as a string literal (%v)", tok.Type)
	}
}
