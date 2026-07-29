// Package types implements MBL type comparison operations
package types

import (
	"math"
	"time"
)

// ComparisonResult represents the result of a comparison operation
type ComparisonResult int

const (
	ComparisonLess ComparisonResult = iota - 1
	ComparisonEqual
	ComparisonGreater
	ComparisonInvalid // For incomparable types
)

// String returns a string representation of the comparison result
func (r ComparisonResult) String() string {
	switch r {
	case ComparisonLess:
		return "less"
	case ComparisonEqual:
		return "equal"
	case ComparisonGreater:
		return "greater"
	case ComparisonInvalid:
		return "invalid"
	default:
		return "unknown"
	}
}

// Compare performs MBL comparison between two values
// Returns ComparisonInvalid if types cannot be compared
func Compare(left, right interface{}) ComparisonResult {
	// Handle same-type comparisons first
	switch l := left.(type) {
	case Text:
		if r, ok := right.(Text); ok {
			return compareTexts(l, r)
		}
	case Number:
		if r, ok := right.(Number); ok {
			return compareNumbers(l, r)
		}
	case Time:
		if r, ok := right.(Time); ok {
			return compareTimes(l, r)
		}
	case Money:
		if r, ok := right.(Money); ok {
			return compareMoneys(l, r)
		}
	case Reference:
		if r, ok := right.(Reference); ok {
			return compareReferences(l, r)
		}
	case Nothing:
		if _, ok := right.(Nothing); ok {
			return ComparisonEqual // All Nothing values are equal
		}
	case Anything:
		if _, ok := right.(Anything); ok {
			return ComparisonEqual // All Anything values are equal
		}
	case Unknown:
		if r, ok := right.(Unknown); ok {
			return compareUnknowns(l, r)
		}
	case Boolean:
		if r, ok := right.(Boolean); ok {
			return compareBooleans(l, r)
		}
	case List:
		if r, ok := right.(List); ok {
			return compareLists(l, r)
		}
	case Record:
		if r, ok := right.(Record); ok {
			return compareRecords(l, r)
		}
	}

	// Types don't match - comparison is invalid
	return ComparisonInvalid
}

// Equal performs MBL equality comparison (?=)
// This implements the special Time equality rules
func Equal(left, right interface{}) bool {
	// Handle Time's special equality rules
	if l, ok := left.(Time); ok {
		if r, ok := right.(Time); ok {
			return timeEquals(l, r)
		}
	}

	// For all other types, use regular comparison
	return Compare(left, right) == ComparisonEqual
}

// Less performs MBL less-than comparison (<)
func Less(left, right interface{}) bool {
	return Compare(left, right) == ComparisonLess
}

// Greater performs MBL greater-than comparison (>)
func Greater(left, right interface{}) bool {
	return Compare(left, right) == ComparisonGreater
}

// LessEqual performs MBL less-than-or-equal comparison (<=)
func LessEqual(left, right interface{}) bool {
	result := Compare(left, right)
	return result == ComparisonLess || result == ComparisonEqual
}

// GreaterEqual performs MBL greater-than-or-equal comparison (>=)
func GreaterEqual(left, right interface{}) bool {
	result := Compare(left, right)
	return result == ComparisonGreater || result == ComparisonEqual
}

// Individual type comparison functions

func compareTexts(left, right Text) ComparisonResult {
	if left.Value < right.Value {
		return ComparisonLess
	} else if left.Value > right.Value {
		return ComparisonGreater
	}
	return ComparisonEqual
}

func compareNumbers(left, right Number) ComparisonResult {
	// Handle special float cases
	if math.IsNaN(left.Value) || math.IsNaN(right.Value) {
		return ComparisonInvalid // NaN is not comparable
	}

	if left.Value < right.Value {
		return ComparisonLess
	} else if left.Value > right.Value {
		return ComparisonGreater
	}
	return ComparisonEqual
}

func compareTimes(left, right Time) ComparisonResult {
	// For ordering comparisons, use the Time comparison rules from the spec:
	// - The more precise value is compared against the range of the less precise one

	if left.Precision == right.Precision {
		// Same precision: compare normally
		if left.Timestamp.Before(right.Timestamp) {
			return ComparisonLess
		} else if left.Timestamp.After(right.Timestamp) {
			return ComparisonGreater
		}
		return ComparisonEqual
	}

	// Different precisions - determine which is more precise
	var morePrecise, lessPrecise Time
	if left.Precision > right.Precision {
		morePrecise = left
		lessPrecise = right
	} else {
		morePrecise = right
		lessPrecise = left
		// Swap comparison result later
	}

	// Get the time range for the less precise value
	rangeStart, rangeEnd := getTimeRange(lessPrecise)

	// Compare more precise time against the range
	var result ComparisonResult
	if morePrecise.Timestamp.Before(rangeStart) {
		result = ComparisonLess
	} else if morePrecise.Timestamp.After(rangeEnd) {
		result = ComparisonGreater
	} else {
		// More precise time falls within the range - overlap case
		// For ordering, we need to be more specific based on which end
		if morePrecise.Timestamp.Equal(rangeStart) {
			result = ComparisonEqual
		} else if morePrecise.Timestamp.Equal(rangeEnd) {
			result = ComparisonEqual
		} else {
			// Inside range - this is ambiguous for ordering
			result = ComparisonEqual
		}
	}

	// Swap result if we swapped the operands
	if left.Precision < right.Precision {
		if result == ComparisonLess {
			result = ComparisonGreater
		} else if result == ComparisonGreater {
			result = ComparisonLess
		}
	}

	return result
}

func compareMoneys(left, right Money) ComparisonResult {
	// Only compare if same currency
	if left.CurrencyCode != right.CurrencyCode {
		return ComparisonInvalid
	}

	if left.Amount < right.Amount {
		return ComparisonLess
	} else if left.Amount > right.Amount {
		return ComparisonGreater
	}
	return ComparisonEqual
}

func compareReferences(left, right Reference) ComparisonResult {
	if left.AttributeID < right.AttributeID {
		return ComparisonLess
	} else if left.AttributeID > right.AttributeID {
		return ComparisonGreater
	}
	return ComparisonEqual
}

func compareUnknowns(left, right Unknown) ComparisonResult {
	// Compare by reason string
	if left.Reason < right.Reason {
		return ComparisonLess
	} else if left.Reason > right.Reason {
		return ComparisonGreater
	}
	return ComparisonEqual
}

func compareBooleans(left, right Boolean) ComparisonResult {
	// false < true
	if left.Value == right.Value {
		return ComparisonEqual
	} else if !left.Value && right.Value {
		return ComparisonLess
	} else {
		return ComparisonGreater
	}
}

func compareLists(left, right List) ComparisonResult {
	// Compare lists element by element
	minLength := len(left.Elements)
	if len(right.Elements) < minLength {
		minLength = len(right.Elements)
	}

	// Compare common elements
	for i := 0; i < minLength; i++ {
		result := Compare(left.Elements[i], right.Elements[i])
		if result != ComparisonEqual {
			return result
		}
	}

	// If all common elements are equal, compare lengths
	if len(left.Elements) < len(right.Elements) {
		return ComparisonLess
	} else if len(left.Elements) > len(right.Elements) {
		return ComparisonGreater
	}

	return ComparisonEqual
}

func compareRecords(left, right Record) ComparisonResult {
	// Records are equal if they have the same fields with equal values
	if len(left.Fields) != len(right.Fields) {
		if len(left.Fields) < len(right.Fields) {
			return ComparisonLess
		}
		return ComparisonGreater
	}

	// Check if all fields match
	for key, leftValue := range left.Fields {
		rightValue, exists := right.Fields[key]
		if !exists {
			return ComparisonInvalid // Different field sets
		}

		result := Compare(leftValue, rightValue)
		if result != ComparisonEqual {
			return result
		}
	}

	return ComparisonEqual
}

// timeEquals implements MBL Time equality (?=) rules
// The more precise value falls within the range of the less precise one
func timeEquals(left, right Time) bool {
	if left.Precision == right.Precision {
		// Same precision: exact equality
		return left.Timestamp.Equal(right.Timestamp)
	}

	// Different precisions - determine which is more precise
	var morePrecise, lessPrecise Time
	if left.Precision > right.Precision {
		morePrecise = left
		lessPrecise = right
	} else {
		morePrecise = right
		lessPrecise = left
	}

	// Get the time range for the less precise value
	rangeStart, rangeEnd := getTimeRange(lessPrecise)

	// Check if more precise time falls within the range
	return !morePrecise.Timestamp.Before(rangeStart) && !morePrecise.Timestamp.After(rangeEnd)
}

// getTimeRange returns the start and end times for a time value at its precision level
// TimeRange returns the window a Time covers given its precision: @2026-01-15
// spans the whole of 15 January, @2026 spans the whole year. Exported for
// temporal queries, where "as of @2026-01-15" means the most recent instance at
// or before the *end* of that day — treating it as midnight would silently
// return the previous day's value.
func TimeRange(t Time) (start, end time.Time) {
	return getTimeRange(t)
}

func getTimeRange(t Time) (start, end time.Time) {
	switch t.Precision {
	case PrecisionYear:
		start = time.Date(t.Timestamp.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
		end = time.Date(t.Timestamp.Year()+1, time.January, 1, 0, 0, 0, 0, time.UTC).Add(-time.Nanosecond)

	case PrecisionMonth:
		year, month, _ := t.Timestamp.Date()
		start = time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 1, 0).Add(-time.Nanosecond)

	case PrecisionDay:
		year, month, day := t.Timestamp.Date()
		start = time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 0, 1).Add(-time.Nanosecond)

	case PrecisionHour:
		year, month, day := t.Timestamp.Date()
		hour := t.Timestamp.Hour()
		start = time.Date(year, month, day, hour, 0, 0, 0, time.UTC)
		end = start.Add(time.Hour).Add(-time.Nanosecond)

	case PrecisionMinute:
		year, month, day := t.Timestamp.Date()
		hour, min := t.Timestamp.Hour(), t.Timestamp.Minute()
		start = time.Date(year, month, day, hour, min, 0, 0, time.UTC)
		end = start.Add(time.Minute).Add(-time.Nanosecond)

	case PrecisionSecond:
		year, month, day := t.Timestamp.Date()
		hour, min, sec := t.Timestamp.Clock()
		start = time.Date(year, month, day, hour, min, sec, 0, time.UTC)
		end = start.Add(time.Second).Add(-time.Nanosecond)

	case PrecisionSubsecond:
		// Subsecond precision - use exact timestamp
		start = t.Timestamp
		end = t.Timestamp

	default:
		// Invalid precision - return zero times
		start = time.Time{}
		end = time.Time{}
	}

	return start, end
}