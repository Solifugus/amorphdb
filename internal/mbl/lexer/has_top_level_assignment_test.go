package lexer

import "testing"

// TestHasTopLevelAssignment verifies the depth-aware assignment detector that
// lets the parser tell a bracketed assignment target apart from a bracket
// filter read. The detector scans from the lexer's current position, so create
// the lexer fresh (position 0) for each case.
func TestHasTopLevelAssignment(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		// Plain assignments.
		{`my.x = 5`, true},
		{`world.zone.data[i] = "x"`, true},
		{`world.apps.myapp.tokens[token].identity = username`, true},
		{`my.computer.network.web.pwa["app.example.com"].enabled = true`, true},
		{`pwa["a"].assets["b"] = c`, true},

		// Filter reads: the '=' is inside the brackets only.
		{`my.employees[department = "Engineering", salary > 80000]`, false},
		{`my.orders[priority ?= "urgent"]`, false},
		{`my.staff[active = true and level = 2]`, false},

		// '=' inside a string literal must not count, and a bracketed value may
		// contain ] / = without breaking depth tracking.
		{`world.x["a=b]c"]`, false},
		{`world.x["a=b]c"] = 1`, true},

		// Comparison operators are not assignments.
		{`if my.x == 5:`, false},
		{`if my.x >= 5:`, false},
		{`if my.x != 5:`, false},

		// No assignment at all.
		{`my.cache.recent[0]`, false},
		{`my.computer.output("hi")`, false},
	}

	for _, c := range cases {
		got := New(c.input).HasTopLevelAssignment()
		if got != c.want {
			t.Errorf("HasTopLevelAssignment(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}
