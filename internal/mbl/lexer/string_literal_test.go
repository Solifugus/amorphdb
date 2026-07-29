// Tests for MBL text literals in both forms: the simple "…" form and the
// extended _"…"_ form, which exists so a literal can carry quote characters
// without escaping.
package lexer

import "testing"

// TestSimpleStringLiterals covers the traditional form. The first quote
// encountered closes the literal.
func TestSimpleStringLiterals(t *testing.T) {
	tests := []struct {
		input     string
		wantLit   string
		wantValue string
	}{
		{`"hello"`, `"hello"`, `hello`},
		{`""`, `""`, ``},
		{`"with spaces and , punctuation!"`, `"with spaces and , punctuation!"`, `with spaces and , punctuation!`},
		{`"unicode ünïcödé ✓"`, `"unicode ünïcödé ✓"`, `unicode ünïcödé ✓`},
		{`"trailing underscore_"`, `"trailing underscore_"`, `trailing underscore_`},
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != TEXT {
			t.Errorf("%s: token type = %v, want TEXT", tt.input, tok.Type)
			continue
		}
		if tok.Literal != tt.wantLit {
			t.Errorf("%s: literal = %q, want %q", tt.input, tok.Literal, tt.wantLit)
		}
		got, ok := UnquoteText(tok.Literal)
		if !ok {
			t.Errorf("%s: UnquoteText did not recognise %q", tt.input, tok.Literal)
			continue
		}
		if got != tt.wantValue {
			t.Errorf("%s: value = %q, want %q", tt.input, got, tt.wantValue)
		}
	}
}

// TestExtendedStringLiterals covers _"…"_ , including content containing quotes,
// which is the reason the form exists.
func TestExtendedStringLiterals(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValue string
	}{
		{"plain content", `_"hello"_`, `hello`},
		{"embedded quotes", `_"He said "Hello" to me"_`, `He said "Hello" to me`},
		{"spec example", `_"I said "Hello World", didn't I?"_`, `I said "Hello World", didn't I?`},
		{"content ends with quote", `_"say "hi""_`, `say "hi"`},
		{"content starts with quote", `_""hi" he said"_`, `"hi" he said`},
		{"run backs off when it cannot close", `_""only one quote"_`, `"only one quote`},
		{"doubled run", `_""Complex "quoted" text""_`, `Complex "quoted" text`},
		{"doubled run guards underscore", `_""ends with quote-underscore "_ inside""_`, `ends with quote-underscore "_ inside`},
		{"empty via doubled run", `_""""_`, ``},
		{"quote adjacent to text", `_"a"b"_`, `a"b`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			tok := l.NextToken()

			if tok.Type != TEXT {
				t.Fatalf("token type = %v, want TEXT (literal %q)", tok.Type, tok.Literal)
			}
			if tok.Literal != tt.input {
				t.Fatalf("literal = %q, want the whole input %q", tok.Literal, tt.input)
			}
			got, ok := UnquoteText(tok.Literal)
			if !ok {
				t.Fatalf("UnquoteText did not recognise %q", tok.Literal)
			}
			if got != tt.wantValue {
				t.Errorf("value = %q, want %q", got, tt.wantValue)
			}
		})
	}
}

// TestUnterminatedStringsAreIllegal covers the primary failure case for both
// forms: reaching EOF before the closing delimiter.
func TestUnterminatedStringsAreIllegal(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple unterminated", `"no closing quote`},
		{"lone quote", `"`},
		{"extended unterminated", `_"no closing delimiter`},
		{"extended closed without underscore", `_"closed wrong"`},
		{"extended run with no underscore at all", `_""abc""`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			tok := l.NextToken()
			if tok.Type != ILLEGAL {
				t.Errorf("token type = %v, want ILLEGAL (literal %q)", tok.Type, tok.Literal)
			}
		})
	}
}

// TestUnderscoreStillStartsIdentifiers guards the lookahead added for _" — a
// leading underscore that is not followed by a quote must still lex as an
// identifier.
func TestUnderscoreStillStartsIdentifiers(t *testing.T) {
	tests := []string{`_foo`, `_`, `_1abc`, `_foo_bar`}

	for _, input := range tests {
		l := New(input)
		tok := l.NextToken()
		if tok.Type == TEXT || tok.Type == ILLEGAL {
			t.Errorf("%s: token type = %v, want an identifier-like token", input, tok.Type)
		}
		if tok.Literal != input {
			t.Errorf("%s: literal = %q, want %q", input, tok.Literal, input)
		}
	}
}

// TestStringLiteralsInStatementContext checks the literals lex correctly when
// followed by other tokens, not just in isolation.
func TestStringLiteralsInStatementContext(t *testing.T) {
	input := `my.a = "plain"` + "\n" + `my.b = _"has "quotes" inside"_`

	l := New(input)
	var texts []string
	for {
		tok := l.NextToken()
		if tok.Type == EOF {
			break
		}
		if tok.Type == ILLEGAL {
			t.Fatalf("unexpected ILLEGAL token: %q", tok.Literal)
		}
		if tok.Type == TEXT {
			value, ok := UnquoteText(tok.Literal)
			if !ok {
				t.Fatalf("UnquoteText did not recognise %q", tok.Literal)
			}
			texts = append(texts, value)
		}
	}

	want := []string{`plain`, `has "quotes" inside`}
	if len(texts) != len(want) {
		t.Fatalf("got %d text tokens (%q), want %d", len(texts), texts, len(want))
	}
	for i := range want {
		if texts[i] != want[i] {
			t.Errorf("text %d = %q, want %q", i, texts[i], want[i])
		}
	}
}

// TestUnquoteTextPassesThroughNonLiterals confirms strings that are already
// values are returned unchanged, which convertToMBLType relies on.
func TestUnquoteTextPassesThroughNonLiterals(t *testing.T) {
	for _, input := range []string{`bare`, ``, `"`, `_`, `_notaquote`, `half"`} {
		got, ok := UnquoteText(input)
		if ok {
			t.Errorf("%q: recognised as a delimited literal, want pass-through", input)
		}
		if got != input {
			t.Errorf("%q: returned %q, want the input unchanged", input, got)
		}
	}
}
