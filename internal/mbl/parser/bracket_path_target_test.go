// Tests for bracket key-selectors in assignment-target paths, e.g.
//
//	my.computer.network.web.pwa["host.example.com"].enabled = true
//	world.apps.myapp.tokens[token].identity = username
//	world.zone.data[i] = "x"
//
// A [key] descends into the child named by the key's value. A literal key is
// baked into the path's Parts; a non-literal key (identifier or expression) is
// recorded in Dynamic for the interpreter to evaluate at write time. Bracket
// filter reads (person[name = "bob", age > 18]) must NOT be drawn into this
// path — they remain BracketFilterExpression reads.
package parser

import (
	"testing"
)

func firstAssignment(t *testing.T, input string) *AssignmentStatement {
	t.Helper()
	program, errors := parseProgram(input)
	if len(errors) != 0 {
		t.Fatalf("parser errors for %q: %v", input, errors)
	}
	if len(program.Statements) != 1 {
		t.Fatalf("expected exactly 1 statement for %q, got %d: %s", input, len(program.Statements), program.String())
	}
	as, ok := program.Statements[0].(*AssignmentStatement)
	if !ok {
		t.Fatalf("expected *AssignmentStatement for %q, got %T: %s", input, program.Statements[0], program.String())
	}
	return as
}

func TestBracketTarget_StaticStringKey_DottedDomain(t *testing.T) {
	// The PWA-enable form. The dotted domain must become a single path
	// component (Option-A convention) with its surrounding quotes stripped.
	as := firstAssignment(t, `my.computer.network.web.pwa["app.example.com"].enabled = true`)

	want := []string{"my", "computer", "network", "web", "pwa", "app.example.com", "enabled"}
	if len(as.Name.Parts) != len(want) {
		t.Fatalf("Parts = %v, want %v", as.Name.Parts, want)
	}
	for i := range want {
		if as.Name.Parts[i] != want[i] {
			t.Fatalf("Parts[%d] = %q, want %q (full %v)", i, as.Name.Parts[i], want[i], as.Name.Parts)
		}
	}
	if len(as.Name.Dynamic) != 0 {
		t.Fatalf("static key must not be Dynamic, got %v", as.Name.Dynamic)
	}
}

func TestBracketTarget_MultipleStaticKeys_NestedAsset(t *testing.T) {
	// pwa["domain"].assets["css/app.css"] — the manual nested-asset form. The
	// slash is preserved verbatim inside a single component.
	as := firstAssignment(t, `my.computer.network.web.pwa["app.example.com"].assets["css/app.css"] = "body{}"`)
	want := []string{"my", "computer", "network", "web", "pwa", "app.example.com", "assets", "css/app.css"}
	if len(as.Name.Parts) != len(want) {
		t.Fatalf("Parts = %v, want %v", as.Name.Parts, want)
	}
	for i := range want {
		if as.Name.Parts[i] != want[i] {
			t.Fatalf("Parts[%d] = %q, want %q", i, as.Name.Parts[i], want[i])
		}
	}
}

func TestBracketTarget_DynamicIdentifierKey(t *testing.T) {
	// world.apps.myapp.tokens[token].identity = username — the dominant auth
	// form. The token segment is dynamic; trailing .identity is static.
	as := firstAssignment(t, `world.apps.myapp.tokens[token].identity = username`)
	want := []string{"world", "apps", "myapp", "tokens", "", "identity"}
	if len(as.Name.Parts) != len(want) {
		t.Fatalf("Parts = %v (len %d), want len %d", as.Name.Parts, len(as.Name.Parts), len(want))
	}
	if len(as.Name.Dynamic) != 1 {
		t.Fatalf("expected exactly one dynamic segment, got %v", as.Name.Dynamic)
	}
	expr, ok := as.Name.Dynamic[4]
	if !ok {
		t.Fatalf("expected dynamic segment at index 4, got %v", as.Name.Dynamic)
	}
	if expr.String() != "token" {
		t.Fatalf("dynamic key expr = %q, want %q", expr.String(), "token")
	}
}

func TestBracketTarget_MetaAttributeAfterDynamicKey(t *testing.T) {
	// world.apps.myapp.tokens[token].@ttl = 100 — meta attribute after a
	// dynamic key (used by the reference login watcher).
	as := firstAssignment(t, `world.apps.myapp.tokens[token].@ttl = 100`)
	last := as.Name.Parts[len(as.Name.Parts)-1]
	if last != "@ttl" {
		t.Fatalf("last segment = %q, want %q (full %v)", last, "@ttl", as.Name.Parts)
	}
	if _, ok := as.Name.Dynamic[4]; !ok {
		t.Fatalf("expected dynamic token segment at index 4, got %v", as.Name.Dynamic)
	}
}

func TestBracketTarget_NumberLiteralKey(t *testing.T) {
	as := firstAssignment(t, `world.zone.data[0] = "x"`)
	if last := as.Name.Parts[len(as.Name.Parts)-1]; last != "0" {
		t.Fatalf("last segment = %q, want %q", last, "0")
	}
	if len(as.Name.Dynamic) != 0 {
		t.Fatalf("number literal key must be static, got %v", as.Name.Dynamic)
	}
}

func TestBracketFilterRead_NotMisroutedAsAssignment(t *testing.T) {
	// The '=' lives inside the brackets, so this is a filter READ, not an
	// assignment. It must parse as a single expression statement carrying a
	// BracketFilterExpression — never as an AssignmentStatement.
	input := `my.employees[department = "Engineering", salary > 80000]`
	program, errors := parseProgram(input)
	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}
	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d: %s", len(program.Statements), program.String())
	}
	if as, ok := program.Statements[0].(*AssignmentStatement); ok {
		t.Fatalf("filter read was misrouted to an assignment: %s", as.String())
	}
	es, ok := program.Statements[0].(*ExpressionStatement)
	if !ok {
		t.Fatalf("expected *ExpressionStatement, got %T", program.Statements[0])
	}
	if _, ok := es.Expression.(*BracketFilterExpression); !ok {
		t.Fatalf("expected BracketFilterExpression, got %T", es.Expression)
	}
}

func TestBracketTarget_StringValueRoundTripsInPathString(t *testing.T) {
	// Dynamic segments render with brackets in String() so debug output keeps
	// the original intent.
	as := firstAssignment(t, `world.zone.data[i] = 5`)
	if got := as.Name.String(); got != "world.zone.data[i]" {
		t.Fatalf("String() = %q, want %q", got, "world.zone.data[i]")
	}
}
