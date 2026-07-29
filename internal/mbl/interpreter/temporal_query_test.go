// Tests for as-of temporal queries — path[@time] reads the value that was in
// effect at an instant rather than the current one.
//
// DEVPLAN Step 25.
package interpreter

import (
	"fmt"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/types"
)

// subsecondLiteral renders an exact instant as an MBL time literal at subsecond
// precision, so a query can target a moment between two writes.
func subsecondLiteral(t time.Time) string {
	return "@" + t.UTC().Format("2006-01-02 15:04:05.000000")
}

// TestAsOfReturnsHistoricalValue is the headline behaviour: the whole point of a
// temporal database. A query aimed between two writes must return the earlier
// value, not the current one.
func TestAsOfReturnsHistoricalValue(t *testing.T) {
	_, interp := freshTreeInterp(t, "asof-history")

	runMBL(t, interp, `my.account.balance = 100`)
	time.Sleep(10 * time.Millisecond)
	between := time.Now().UTC()
	time.Sleep(10 * time.Millisecond)
	runMBL(t, interp, `my.account.balance = 250`)

	// Current value is unaffected by the temporal machinery.
	if got := runMBL(t, interp, `my.account.balance`); !isNumber(got, 250) {
		t.Fatalf("current value = %v, want 250", got)
	}

	// The value in effect at `between` was 100.
	src := fmt.Sprintf("my.account.balance[%s]", subsecondLiteral(between))
	got := runMBL(t, interp, src)
	if !isNumber(got, 100) {
		t.Errorf("%s = %v, want 100 — the historical value was not returned", src, got)
	}
}

// TestAsOfAfterLastWriteReturnsCurrent checks the boundary in the other
// direction.
func TestAsOfAfterLastWriteReturnsCurrent(t *testing.T) {
	_, interp := freshTreeInterp(t, "asof-current")

	runMBL(t, interp, `my.bal = 100`)
	runMBL(t, interp, `my.bal = 250`)
	time.Sleep(5 * time.Millisecond)

	src := fmt.Sprintf("my.bal[%s]", subsecondLiteral(time.Now().UTC()))
	if got := runMBL(t, interp, src); !isNumber(got, 250) {
		t.Errorf("%s = %v, want 250", src, got)
	}
}

// TestAsOfBeforeFirstInstanceIsUnknown is the primary failure case: there is no
// honest answer, so the query must say so rather than return the oldest value.
func TestAsOfBeforeFirstInstanceIsUnknown(t *testing.T) {
	_, interp := freshTreeInterp(t, "asof-before")
	runMBL(t, interp, `my.bal = 100`)

	got := runMBL(t, interp, `my.bal[@1990-01-01]`)
	unk, ok := got.(types.Unknown)
	if !ok {
		t.Fatalf("query before the first instance = %T (%v), want Unknown", got, got)
	}
	if unk.Reason == "" {
		t.Error("Unknown carries no reason")
	}
}

// TestAsOfPrecisionUsesEndOfWindow guards the subtlest correctness point in this
// step. @2026-01-15 covers the whole of 15 January, so "as of" that date means
// the most recent value at or before the END of the day. Treating the literal as
// midnight would return the previous day's value — a wrong answer that looks
// entirely plausible.
func TestAsOfPrecisionUsesEndOfWindow(t *testing.T) {
	_, interp := freshTreeInterp(t, "asof-precision")

	runMBL(t, interp, `my.bal = 100`)
	time.Sleep(5 * time.Millisecond)
	runMBL(t, interp, `my.bal = 250`)

	// Today at day precision. Both writes happened today, so the end of today's
	// window is after both, and the answer must be the later value. If the
	// implementation used the start of the day instead, this would fail to find
	// any instance at all.
	today := time.Now().UTC().Format("2006-01-02")
	got := runMBL(t, interp, fmt.Sprintf("my.bal[@%s]", today))
	if !isNumber(got, 250) {
		t.Errorf("my.bal[@%s] = %v, want 250 — precision window end not used", today, got)
	}

	// Year precision spans further still and must behave the same way.
	year := time.Now().UTC().Format("2006")
	got = runMBL(t, interp, fmt.Sprintf("my.bal[@%s]", year))
	if !isNumber(got, 250) {
		t.Errorf("my.bal[@%s] = %v, want 250", year, got)
	}
}

// TestAsOfAcceptsAStoredTime covers the path form: the bracket may hold a path
// that reads as a time, not only a literal.
func TestAsOfAcceptsAStoredTime(t *testing.T) {
	_, interp := freshTreeInterp(t, "asof-path")

	runMBL(t, interp, `my.bal = 100`)
	time.Sleep(10 * time.Millisecond)
	between := time.Now().UTC()
	time.Sleep(10 * time.Millisecond)
	runMBL(t, interp, `my.bal = 250`)

	runMBL(t, interp, "my.cutoff = "+subsecondLiteral(between))
	if got := runMBL(t, interp, `my.bal[my.cutoff]`); !isNumber(got, 100) {
		t.Errorf("my.bal[my.cutoff] = %v, want 100", got)
	}
}

// TestNonTemporalBracketsUnaffected is the regression guard. Temporal detection
// runs before the base is evaluated, so it must not capture index, filter or
// key-selector brackets.
func TestNonTemporalBracketsUnaffected(t *testing.T) {
	t.Run("index", func(t *testing.T) {
		_, interp := freshTreeInterp(t, "br-index")
		runMBL(t, interp, `x = [10, 20, 30]`)
		if got := runMBL(t, interp, `x[0]`); !isNumber(got, 10) {
			t.Errorf("x[0] = %v, want 10 — index bracket broke", got)
		}
		if got := runMBL(t, interp, `x[2]`); !isNumber(got, 30) {
			t.Errorf("x[2] = %v, want 30 — index bracket broke", got)
		}
	})

	t.Run("key selector", func(t *testing.T) {
		_, interp := freshTreeInterp(t, "br-key")
		runMBL(t, interp, `my.users.alice = "A"`)
		runMBL(t, interp, `my.name = "alice"`)
		got := runMBL(t, interp, `my.users[my.name]`)
		text, ok := got.(types.Text)
		if !ok || text.Value != "A" {
			t.Errorf("key-selector bracket broke: %T %v", got, got)
		}
	})

	t.Run("filter is not evaluated speculatively", func(t *testing.T) {
		// '=' is assignment in MBL. Temporal detection must never evaluate a
		// filter expression, or a query would perform a write as a side effect.
		_, interp := freshTreeInterp(t, "br-filter")
		runMBL(t, interp, `my.marker = "untouched"`)
		runMBL(t, interp, `my.rows = 1`)
		_ = runMBL(t, interp, `my.rows[my.marker = "written"]`)

		got := runMBL(t, interp, `my.marker`)
		text, ok := got.(types.Text)
		if !ok || text.Value != "untouched" {
			t.Errorf("a bracket filter performed a write: my.marker = %v", got)
		}
	})
}

func isNumber(v interface{}, want float64) bool {
	n, ok := v.(types.Number)
	return ok && n.Value == want
}
