// End-to-end tests that MBL time literals evaluate to types.Time with the
// precision the author actually supplied, that every documented literal form
// parses, and that text coercion shows only the components that were specified.
//
// DEVPLAN Step 19.
package interpreter

import (
	"strings"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/mbl/lexer"
	"github.com/solifugus/amorphdb/internal/mbl/parser"
	"github.com/solifugus/amorphdb/internal/types"
)

// TestTimeLiteralProducesTimeType is the core regression: a time literal used to
// evaluate to types.Text reading "2026-01-15 00:00:00 +0000 UTC" because the
// parser stored a bare Go time.Time.
func TestTimeLiteralProducesTimeType(t *testing.T) {
	_, interp := freshTreeInterp(t, "timelit")
	res := runMBL(t, interp, `x = @2026-01-15`)

	got, ok := res.(types.Time)
	if !ok {
		t.Fatalf("@2026-01-15 evaluated to %T (%v), want types.Time", res, res)
	}
	if got.Precision != types.PrecisionDay {
		t.Errorf("precision = %d, want PrecisionDay (%d)", got.Precision, types.PrecisionDay)
	}
	if y, m, d := got.Timestamp.Date(); y != 2026 || m != time.January || d != 15 {
		t.Errorf("timestamp = %v, want 2026-01-15", got.Timestamp)
	}
	if got.Timestamp.Location() != time.UTC {
		t.Errorf("location = %v, want UTC", got.Timestamp.Location())
	}
}

// TestAllDocumentedPrecisionsParse covers the three forms that used to be parse
// errors despite being documented: @2026, @2026-01 and @2026-01-15 14:30.
func TestAllDocumentedPrecisionsParse(t *testing.T) {
	tests := []struct {
		src  string
		want byte
	}{
		{`x = @2026`, types.PrecisionYear},
		{`x = @2026-01`, types.PrecisionMonth},
		{`x = @2026-01-15`, types.PrecisionDay},
		{`x = @2026-01-15 14:30`, types.PrecisionMinute},
		{`x = @2026-01-15 14:30:22`, types.PrecisionSecond},
		{`x = @2026-01-15 14:30:22.500`, types.PrecisionSubsecond},
	}

	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			p := parser.New(lexer.New(tt.src))
			prog := p.ParseProgram()
			if errs := p.Errors(); len(errs) != 0 {
				t.Fatalf("parse errors: %v", errs)
			}

			_, interp := freshTreeInterp(t, "prec")
			res, err := interp.Interpret(prog)
			if err != nil {
				t.Fatalf("interpret: %v", err)
			}

			got, ok := res.(types.Time)
			if !ok {
				t.Fatalf("evaluated to %T (%v), want types.Time", res, res)
			}
			if got.Precision != tt.want {
				t.Errorf("precision = %d, want %d", got.Precision, tt.want)
			}
		})
	}
}

// TestTimeComparison is the regression for the lexer bug where the space after a
// date was consumed unconditionally, so the literal became "@2026-01-15 " and
// failed to parse. Any time literal followed by an operator hit this.
func TestTimeComparison(t *testing.T) {
	tests := []struct {
		src  string
		want bool
	}{
		{`x = @2026-01-15 > @2026-01-01`, true},
		{`x = @2026-01-01 > @2026-01-15`, false},
		{`x = @2026-01-15 < @2026-02-01`, true},
		{`x = @2026-01-15 ?= @2026-01-15`, true},
	}

	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			p := parser.New(lexer.New(tt.src))
			prog := p.ParseProgram()
			if errs := p.Errors(); len(errs) != 0 {
				t.Fatalf("parse errors: %v", errs)
			}

			_, interp := freshTreeInterp(t, "cmp")
			res, err := interp.Interpret(prog)
			if err != nil {
				t.Fatalf("interpret: %v", err)
			}

			got, ok := res.(types.Boolean)
			if !ok {
				t.Fatalf("evaluated to %T (%v), want types.Boolean", res, res)
			}
			if got.Value != tt.want {
				t.Errorf("= %v, want %v", got.Value, tt.want)
			}
		})
	}
}

// TestTimeCoercionOmitsUnspecifiedComponents checks the headline behaviour: a
// day-precision time renders no 00:00:00, and no '@' sigil leaks into output.
func TestTimeCoercionOmitsUnspecifiedComponents(t *testing.T) {
	tests := []struct {
		src  string
		want string
	}{
		{`x = "on " & @2026-01-15`, `on 2026-01-15`},
		{`x = "at " & @2026-01-15 14:30`, `at 2026-01-15 14:30`},
		{`x = "in " & @2026`, `in 2026`},
		{`x = "in " & @2026-01`, `in 2026-01`},
		{`x = "at " & @2026-01-15 14:30:22`, `at 2026-01-15 14:30:22`},
	}

	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			_, interp := freshTreeInterp(t, "coerce")
			res := runMBL(t, interp, tt.src)

			got, ok := res.(types.Text)
			if !ok {
				t.Fatalf("evaluated to %T (%v), want types.Text", res, res)
			}
			if got.Value != tt.want {
				t.Errorf("\n  got  %q\n  want %q", got.Value, tt.want)
			}
			if strings.Contains(got.Value, "@") {
				t.Errorf("coerced text leaked the @ sigil: %q", got.Value)
			}
			if strings.Contains(got.Value, "UTC") || strings.Contains(got.Value, "+0000") {
				t.Errorf("coerced text leaked Go's zone rendering: %q", got.Value)
			}
		})
	}
}

// TestTimeInterpolationMatchesConcat ties Step 19 to the interpolating literal
// added earlier: both paths must render a time identically.
func TestTimeInterpolationMatchesConcat(t *testing.T) {
	for _, lit := range []string{`@2026-01-15`, `@2026-01-15 14:30`, `@2026`} {
		_, interp := freshTreeInterp(t, "interp")
		runMBL(t, interp, `my.d = `+lit)

		viaInterp := runMBL(t, interp, `~"on {my.d}"~`)
		viaConcat := runMBL(t, interp, `"on " & my.d`)

		gotI, ok := viaInterp.(types.Text)
		if !ok {
			t.Fatalf("%s: interpolation gave %T", lit, viaInterp)
		}
		gotC, ok := viaConcat.(types.Text)
		if !ok {
			t.Fatalf("%s: concatenation gave %T", lit, viaConcat)
		}
		if gotI.Value != gotC.Value {
			t.Errorf("%s: interpolation %q != concatenation %q", lit, gotI.Value, gotC.Value)
		}
	}
}

// TestNowIsUTCWithValidPrecision covers the fourth defect: now() left Precision
// at 0, outside the valid 1-7 range, and carried the machine's local zone.
func TestNowIsUTCWithValidPrecision(t *testing.T) {
	_, interp := freshTreeInterp(t, "now")
	res := runMBL(t, interp, `x = now()`)

	got, ok := res.(types.Time)
	if !ok {
		t.Fatalf("now() evaluated to %T (%v), want types.Time", res, res)
	}
	if got.Precision < types.PrecisionYear || got.Precision > types.PrecisionSubsecond {
		t.Errorf("precision = %d, outside the valid range %d-%d",
			got.Precision, types.PrecisionYear, types.PrecisionSubsecond)
	}
	if got.Precision != types.PrecisionSubsecond {
		t.Errorf("precision = %d, want PrecisionSubsecond (%d)", got.Precision, types.PrecisionSubsecond)
	}
	if got.Timestamp.Location() != time.UTC {
		t.Errorf("location = %v, want UTC", got.Timestamp.Location())
	}
}

// NOTE: there is deliberately no storage round-trip test here. Persisting a
// time is broken by a pre-existing storage defect — internal/storage/values.go
// getFixedTypeSize reports TypeTime as 8 bytes while types.Time.Serialize writes
// 9 (timestamp plus the precision byte), so the precision is truncated and the
// read fails with "time data must be 9 bytes". This predates Step 19 (verified
// against stashed code: `my.t = now()` fails identically), and Step 19's scope
// explicitly excludes storage serialization. See DEVPLAN Step 19a.

// TestMalformedTimeLiteralIsAnError is the primary failure case.
//
// Note `@not-a-date` is deliberately absent: the lexer treats '@' followed by a
// letter as a meta-attribute, so it lexes as `@not - a - date`, an expression,
// not a malformed time literal.
func TestMalformedTimeLiteralIsAnError(t *testing.T) {
	for _, src := range []string{
		`x = @2026-13-45`,
		`x = @2026-02-30`,
	} {
		p := parser.New(lexer.New(src))
		p.ParseProgram()
		if len(p.Errors()) == 0 {
			t.Errorf("%s: expected a parse error", src)
		}
	}
}
