package types

import (
	"math"
	"testing"
	"time"
)

// Test success criteria from development plan:
// - `@2026-02-01 ?= @2026-02-01 15:30:00` → true
// - `@2026-02-01 ?= @2026-02-03` → false
// - `@2026-01 < @2026-02-15` → true
// - `1 + "2"` → 3 (Number)
// - `"hello" & 42` → "hello42" (Text)
// - `1 / 0` → Unknown with reason "division by zero"

func TestTimeEquality(t *testing.T) {
	// Success criteria: @2026-02-01 ?= @2026-02-01 15:30:00 → true
	dayPrecision := Time{
		Timestamp: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		Precision: PrecisionDay,
	}
	subsecondPrecision := Time{
		Timestamp: time.Date(2026, 2, 1, 15, 30, 0, 0, time.UTC),
		Precision: PrecisionSubsecond,
	}

	if !Equal(dayPrecision, subsecondPrecision) {
		t.Error("@2026-02-01 should equal @2026-02-01 15:30:00 (more precise falls within day)")
	}

	// Success criteria: @2026-02-01 ?= @2026-02-03 → false
	differentDay := Time{
		Timestamp: time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC),
		Precision: PrecisionDay,
	}

	if Equal(dayPrecision, differentDay) {
		t.Error("@2026-02-01 should not equal @2026-02-03")
	}
}

func TestTimeComparison(t *testing.T) {
	// Success criteria: @2026-01 < @2026-02-15 → true
	monthPrecision := Time{
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Precision: PrecisionMonth,
	}
	dayInFebruary := Time{
		Timestamp: time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
		Precision: PrecisionDay,
	}

	if !Less(monthPrecision, dayInFebruary) {
		t.Error("@2026-01 should be less than @2026-02-15")
	}

	// Additional test: February 15 should not be less than January (entire month)
	if Less(dayInFebruary, monthPrecision) {
		t.Error("@2026-02-15 should not be less than @2026-01 (entire month)")
	}
}

func TestTimeEqualityEdgeCases(t *testing.T) {
	testCases := []struct {
		name     string
		time1    Time
		time2    Time
		expected bool
	}{
		{
			name: "same precision same time",
			time1: Time{
				Timestamp: time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC),
				Precision: PrecisionHour,
			},
			time2: Time{
				Timestamp: time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC),
				Precision: PrecisionHour,
			},
			expected: true,
		},
		{
			name: "year precision contains specific day",
			time1: Time{
				Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				Precision: PrecisionYear,
			},
			time2: Time{
				Timestamp: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
				Precision: PrecisionDay,
			},
			expected: true,
		},
		{
			name: "minute precision at edge of hour",
			time1: Time{
				Timestamp: time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC),
				Precision: PrecisionHour,
			},
			time2: Time{
				Timestamp: time.Date(2026, 2, 1, 12, 59, 0, 0, time.UTC),
				Precision: PrecisionMinute,
			},
			expected: true,
		},
		{
			name: "outside range",
			time1: Time{
				Timestamp: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
				Precision: PrecisionDay,
			},
			time2: Time{
				Timestamp: time.Date(2026, 2, 2, 0, 0, 1, 0, time.UTC),
				Precision: PrecisionSubsecond,
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := Equal(tc.time1, tc.time2)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v for %s", tc.expected, result, tc.name)
			}
		})
	}
}

func TestArithmeticCoercion(t *testing.T) {
	// Success criteria: 1 + "2" → 3 (Number)
	result := Add(Number{Value: 1.0}, Text{Value: "2"})

	resultNum, ok := result.(Number)
	if !ok {
		t.Fatalf("1 + \"2\" should return a Number, got %T", result)
	}

	if resultNum.Value != 3.0 {
		t.Errorf("1 + \"2\" should equal 3, got %f", resultNum.Value)
	}
}

func TestConcatenationCoercion(t *testing.T) {
	// Success criteria: "hello" & 42 → "hello42" (Text)
	result := Concatenate(Text{Value: "hello"}, Number{Value: 42.0})

	resultText, ok := result.(Text)
	if !ok {
		t.Fatalf("\"hello\" & 42 should return Text, got %T", result)
	}

	if resultText.Value != "hello42" {
		t.Errorf("\"hello\" & 42 should equal \"hello42\", got %q", resultText.Value)
	}
}

func TestDivisionByZero(t *testing.T) {
	// Success criteria: 1 / 0 → Unknown with reason "division by zero"
	result := Divide(Number{Value: 1.0}, Number{Value: 0.0})

	resultUnknown, ok := result.(Unknown)
	if !ok {
		t.Fatalf("1 / 0 should return Unknown, got %T", result)
	}

	if resultUnknown.Reason != "division by zero" {
		t.Errorf("1 / 0 should have reason \"division by zero\", got %q", resultUnknown.Reason)
	}
}

func TestNumberComparison(t *testing.T) {
	testCases := []struct {
		name     string
		left     Number
		right    Number
		expected ComparisonResult
	}{
		{"equal numbers", Number{Value: 5.0}, Number{Value: 5.0}, ComparisonEqual},
		{"left less", Number{Value: 3.0}, Number{Value: 5.0}, ComparisonLess},
		{"left greater", Number{Value: 7.0}, Number{Value: 5.0}, ComparisonGreater},
		{"negative numbers", Number{Value: -3.0}, Number{Value: -1.0}, ComparisonLess},
		{"zero comparison", Number{Value: 0.0}, Number{Value: 1.0}, ComparisonLess},
		{"infinity", Number{Value: math.Inf(1)}, Number{Value: 1000.0}, ComparisonGreater},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := Compare(tc.left, tc.right)
			if result != tc.expected {
				t.Errorf("Compare(%f, %f) = %v, want %v", tc.left.Value, tc.right.Value, result, tc.expected)
			}
		})
	}
}

func TestNaNComparison(t *testing.T) {
	nan := Number{Value: math.NaN()}
	num := Number{Value: 5.0}

	// NaN should not be comparable
	result := Compare(nan, num)
	if result != ComparisonInvalid {
		t.Errorf("NaN comparison should return ComparisonInvalid, got %v", result)
	}

	result = Compare(num, nan)
	if result != ComparisonInvalid {
		t.Errorf("Comparison with NaN should return ComparisonInvalid, got %v", result)
	}

	result = Compare(nan, nan)
	if result != ComparisonInvalid {
		t.Errorf("NaN to NaN comparison should return ComparisonInvalid, got %v", result)
	}
}

func TestTextComparison(t *testing.T) {
	testCases := []struct {
		name     string
		left     Text
		right    Text
		expected ComparisonResult
	}{
		{"equal texts", Text{"hello"}, Text{"hello"}, ComparisonEqual},
		{"alphabetical less", Text{"apple"}, Text{"banana"}, ComparisonLess},
		{"alphabetical greater", Text{"zebra"}, Text{"apple"}, ComparisonGreater},
		{"empty strings", Text{""}, Text{""}, ComparisonEqual},
		{"empty vs non-empty", Text{""}, Text{"a"}, ComparisonLess},
		{"unicode comparison", Text{"世"}, Text{"界"}, ComparisonLess}, // Chinese characters: 世(U+4E16) < 界(U+754C)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := Compare(tc.left, tc.right)
			if result != tc.expected {
				t.Errorf("Compare(%q, %q) = %v, want %v", tc.left.Value, tc.right.Value, result, tc.expected)
			}
		})
	}
}

func TestMoneyComparison(t *testing.T) {
	usd1 := Money{Amount: 100.0, CurrencyCode: "USD"}
	usd2 := Money{Amount: 200.0, CurrencyCode: "USD"}
	eur1 := Money{Amount: 100.0, CurrencyCode: "EUR"}

	// Same currency
	result := Compare(usd1, usd2)
	if result != ComparisonLess {
		t.Errorf("$100 should be less than $200")
	}

	// Different currency - incomparable
	result = Compare(usd1, eur1)
	if result != ComparisonInvalid {
		t.Errorf("USD and EUR should not be comparable")
	}
}

func TestReferenceComparison(t *testing.T) {
	ref1 := Reference{AttributeID: 100}
	ref2 := Reference{AttributeID: 200}

	result := Compare(ref1, ref2)
	if result != ComparisonLess {
		t.Errorf("Reference 100 should be less than Reference 200")
	}

	result = Compare(ref1, ref1)
	if result != ComparisonEqual {
		t.Errorf("Reference should equal itself")
	}
}

func TestUnknownComparison(t *testing.T) {
	unknown1 := Unknown{Reason: "error A"}
	unknown2 := Unknown{Reason: "error B"}

	result := Compare(unknown1, unknown2)
	if result != ComparisonLess {
		t.Errorf("Unknown with \"error A\" should be less than \"error B\"")
	}
}

func TestCrossTypeComparison(t *testing.T) {
	// Different types should not be comparable
	text := Text{Value: "hello"}
	number := Number{Value: 42.0}

	result := Compare(text, number)
	if result != ComparisonInvalid {
		t.Errorf("Text and Number should not be comparable")
	}
}

func TestSingletonComparisons(t *testing.T) {
	// Nothing comparisons
	nothing1 := Nothing{}
	nothing2 := Nothing{}
	result := Compare(nothing1, nothing2)
	if result != ComparisonEqual {
		t.Errorf("All Nothing values should be equal")
	}

	// Anything comparisons
	anything1 := Anything{}
	anything2 := Anything{}
	result = Compare(anything1, anything2)
	if result != ComparisonEqual {
		t.Errorf("All Anything values should be equal")
	}

	// Cross-singleton comparison should be invalid
	result = Compare(nothing1, anything1)
	if result != ComparisonInvalid {
		t.Errorf("Nothing and Anything should not be comparable")
	}
}

func TestCoercionToNumber(t *testing.T) {
	testCases := []struct {
		name     string
		input    interface{}
		expected interface{}
		ok       bool
	}{
		{"number stays number", Number{Value: 42.0}, Number{Value: 42.0}, true},
		{"text number", Text{Value: "3.14"}, Number{Value: 3.14}, true},
		{"text non-number", Text{Value: "hello"}, Unknown{}, false},
		{"money to number", Money{Amount: 19.95, CurrencyCode: "USD"}, Number{Value: 19.95}, true},
		{"reference invalid", Reference{AttributeID: 123}, Unknown{}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := CoerceToNumber(tc.input)
			if result.Ok != tc.ok {
				t.Errorf("CoerceToNumber(%v).Ok = %v, want %v", tc.input, result.Ok, tc.ok)
				return
			}

			if tc.ok {
				expectedNum := tc.expected.(Number)
				resultNum := result.Value.(Number)
				if resultNum.Value != expectedNum.Value {
					t.Errorf("CoerceToNumber(%v) = %f, want %f", tc.input, resultNum.Value, expectedNum.Value)
				}
			}
		})
	}
}

func TestCoercionToText(t *testing.T) {
	testCases := []struct {
		name     string
		input    interface{}
		expected string
		ok       bool
	}{
		{"text stays text", Text{Value: "hello"}, "hello", true},
		{"number to text", Number{Value: 42.0}, "42", true},
		{"money to text", Money{Amount: 19.95, CurrencyCode: "USD"}, "19.95 USD", true},
		{"nothing to empty", Nothing{}, "", true},
		{"unknown to reason", Unknown{Reason: "test error"}, "Unknown: test error", true},
		{"anything to marker", Anything{}, "Anything", true},
		{"reference to text", Reference{AttributeID: 123}, "@123", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := CoerceToText(tc.input)
			if result.Ok != tc.ok {
				t.Errorf("CoerceToText(%v).Ok = %v, want %v", tc.input, result.Ok, tc.ok)
				return
			}

			resultText := result.Value.(Text)
			if resultText.Value != tc.expected {
				t.Errorf("CoerceToText(%v) = %q, want %q", tc.input, resultText.Value, tc.expected)
			}
		})
	}
}

func TestTimeToTextCoercion(t *testing.T) {
	testCases := []struct {
		name     string
		time     Time
		expected string
	}{
		{
			name: "year precision",
			time: Time{
				Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				Precision: PrecisionYear,
			},
			expected: "@2026",
		},
		{
			name: "month precision",
			time: Time{
				Timestamp: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
				Precision: PrecisionMonth,
			},
			expected: "@2026-02",
		},
		{
			name: "day precision",
			time: Time{
				Timestamp: time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
				Precision: PrecisionDay,
			},
			expected: "@2026-02-15",
		},
		{
			name: "hour precision",
			time: Time{
				Timestamp: time.Date(2026, 2, 15, 14, 0, 0, 0, time.UTC),
				Precision: PrecisionHour,
			},
			expected: "@2026-02-15 14:00:00",
		},
		{
			name: "minute precision",
			time: Time{
				Timestamp: time.Date(2026, 2, 15, 14, 30, 0, 0, time.UTC),
				Precision: PrecisionMinute,
			},
			expected: "@2026-02-15 14:30:00",
		},
		{
			name: "second precision",
			time: Time{
				Timestamp: time.Date(2026, 2, 15, 14, 30, 45, 0, time.UTC),
				Precision: PrecisionSecond,
			},
			expected: "@2026-02-15 14:30:45",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := CoerceToText(tc.time)
			if !result.Ok {
				t.Fatalf("CoerceToText failed for time")
			}

			resultText := result.Value.(Text)
			if resultText.Value != tc.expected {
				t.Errorf("Time to text: got %q, want %q", resultText.Value, tc.expected)
			}
		})
	}
}

func TestArithmeticOperations(t *testing.T) {
	// Test all arithmetic operations with various coercions
	testCases := []struct {
		name      string
		operation func(interface{}, interface{}) interface{}
		left      interface{}
		right     interface{}
		expected  interface{}
	}{
		{"add numbers", Add, Number{Value: 5.0}, Number{Value: 3.0}, Number{Value: 8.0}},
		{"add number and text", Add, Number{Value: 1.0}, Text{Value: "2"}, Number{Value: 3.0}},
		{"subtract numbers", Subtract, Number{Value: 10.0}, Number{Value: 3.0}, Number{Value: 7.0}},
		{"multiply numbers", Multiply, Number{Value: 4.0}, Number{Value: 3.0}, Number{Value: 12.0}},
		{"divide numbers", Divide, Number{Value: 12.0}, Number{Value: 3.0}, Number{Value: 4.0}},
		{"modulo numbers", Modulo, Number{Value: 10.0}, Number{Value: 3.0}, Number{Value: 1.0}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.operation(tc.left, tc.right)

			switch expected := tc.expected.(type) {
			case Number:
				resultNum, ok := result.(Number)
				if !ok {
					t.Fatalf("Expected Number result, got %T", result)
				}
				if resultNum.Value != expected.Value {
					t.Errorf("Expected %f, got %f", expected.Value, resultNum.Value)
				}
			default:
				t.Fatalf("Unexpected expected type: %T", tc.expected)
			}
		})
	}
}

func TestErrorOperations(t *testing.T) {
	// Test operations that should produce errors
	testCases := []struct {
		name      string
		operation func(interface{}, interface{}) interface{}
		left      interface{}
		right     interface{}
		wantError string
	}{
		{"divide by zero", Divide, Number{Value: 1.0}, Number{Value: 0.0}, "division by zero"},
		{"modulo by zero", Modulo, Number{Value: 5.0}, Number{Value: 0.0}, "modulo by zero"},
		{"add invalid text", Add, Number{Value: 1.0}, Text{Value: "hello"}, "cannot convert text to number: hello"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.operation(tc.left, tc.right)

			resultUnknown, ok := result.(Unknown)
			if !ok {
				t.Fatalf("Expected Unknown result, got %T", result)
			}

			if resultUnknown.Reason != tc.wantError {
				t.Errorf("Expected error %q, got %q", tc.wantError, resultUnknown.Reason)
			}
		})
	}
}

func TestLogicalOperations(t *testing.T) {
	testCases := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		{"number zero is falsy", Number{Value: 0.0}, false},
		{"number non-zero is truthy", Number{Value: 5.0}, true},
		{"empty text is falsy", Text{Value: ""}, false},
		{"non-empty text is truthy", Text{Value: "hello"}, true},
		{"nothing is falsy", Nothing{}, false},
		{"unknown is falsy", Unknown{Reason: "error"}, false},
		{"anything is truthy", Anything{}, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isTruthy(tc.value)
			if result != tc.expected {
				t.Errorf("isTruthy(%v) = %v, want %v", tc.value, result, tc.expected)
			}

			// Test logical NOT
			notResult := LogicalNot(tc.value)
			if notResult != !tc.expected {
				t.Errorf("LogicalNot(%v) = %v, want %v", tc.value, notResult, !tc.expected)
			}
		})
	}

	// Test logical AND and OR
	truthy := Text{Value: "hello"}
	falsy := Text{Value: ""}

	if !LogicalAnd(truthy, truthy) {
		t.Error("true AND true should be true")
	}
	if LogicalAnd(truthy, falsy) {
		t.Error("true AND false should be false")
	}
	if LogicalAnd(falsy, truthy) {
		t.Error("false AND true should be false")
	}
	if LogicalAnd(falsy, falsy) {
		t.Error("false AND false should be false")
	}

	if !LogicalOr(truthy, truthy) {
		t.Error("true OR true should be true")
	}
	if !LogicalOr(truthy, falsy) {
		t.Error("true OR false should be true")
	}
	if !LogicalOr(falsy, truthy) {
		t.Error("false OR true should be true")
	}
	if LogicalOr(falsy, falsy) {
		t.Error("false OR false should be false")
	}
}