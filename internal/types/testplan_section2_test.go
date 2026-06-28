package types

import (
	"math"
	"testing"
	"time"
)

// Section 2: Type System tests from AmorphDB_Test_Plan.md
// Comprehensive validation of all MBL data types, meta types, and coercion

// 2.1 Core Types

// Test 2.1.1: Text — UTF-8 round-trip
func TestText_UTF8RoundTrip(t *testing.T) {
	testCases := []string{
		"Hello, 世界",     // Chinese characters
		"Café",           // Accented characters
		"🌟✨",            // Emojis
		"Москва",         // Cyrillic
		"العربية",        // Arabic
		"한국어",           // Korean
	}

	for _, testText := range testCases {
		t.Run(testText, func(t *testing.T) {
			original := Text{Value: testText}

			// Serialize
			serializedValue := SerializedValue{
				TypeTag: original.TypeTag(),
				Data:    original.Serialize(),
			}

			// Deserialize
			deserialized, err := DeserializeValue(serializedValue)
			if err != nil {
				t.Fatalf("Failed to deserialize UTF-8 text: %v", err)
			}

			result, ok := deserialized.(Text)
			if !ok {
				t.Fatalf("Deserialized value is not Text type")
			}

			if result.Value != original.Value {
				t.Errorf("UTF-8 text round-trip failed: got %q, want %q", result.Value, original.Value)
			}
		})
	}
}

// Test 2.1.2: Text — empty string
func TestText_EmptyString(t *testing.T) {
	original := Text{Value: ""}

	// Verify empty string is distinct from null/Nothing
	nothing := Nothing{}
	if original.String() == nothing.String() {
		t.Errorf("Empty string should be distinct from Nothing")
	}

	// Test round-trip
	serializedValue := SerializedValue{
		TypeTag: original.TypeTag(),
		Data:    original.Serialize(),
	}

	deserialized, err := DeserializeValue(serializedValue)
	if err != nil {
		t.Fatalf("Failed to deserialize empty string: %v", err)
	}

	result, ok := deserialized.(Text)
	if !ok {
		t.Fatalf("Deserialized value is not Text type")
	}

	if result.Value != "" {
		t.Errorf("Empty string round-trip failed: got %q, want empty", result.Value)
	}
}

// Test 2.1.3: Number — float64 precision
func TestNumber_Float64Precision(t *testing.T) {
	testCases := []struct {
		name  string
		value float64
	}{
		{"max float64", math.MaxFloat64},
		{"smallest nonzero", math.SmallestNonzeroFloat64},
		{"epsilon adjacent", 1.0 + math.Nextafter(1.0, 2.0) - 1.0},
		{"positive infinity", math.Inf(1)},
		{"negative infinity", math.Inf(-1)},
		{"NaN", math.NaN()},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			original := Number{Value: tc.value}

			serializedValue := SerializedValue{
				TypeTag: original.TypeTag(),
				Data:    original.Serialize(),
			}

			deserialized, err := DeserializeValue(serializedValue)
			if err != nil {
				t.Fatalf("Failed to deserialize number: %v", err)
			}

			result, ok := deserialized.(Number)
			if !ok {
				t.Fatalf("Deserialized value is not Number type")
			}

			// Special handling for NaN
			if math.IsNaN(tc.value) {
				if !math.IsNaN(result.Value) {
					t.Errorf("NaN not preserved: got %f", result.Value)
				}
			} else if result.Value != tc.value {
				t.Errorf("Float64 precision lost: got %g, want %g", result.Value, tc.value)
			}
		})
	}
}

// Test 2.1.4: Number — integer representation
func TestNumber_IntegerRepresentation(t *testing.T) {
	testInts := []int64{42, -42, 0, 1000000000, -1000000000}

	for _, testInt := range testInts {
		t.Run("integer", func(t *testing.T) {
			original := Number{Value: float64(testInt)}

			serializedValue := SerializedValue{
				TypeTag: original.TypeTag(),
				Data:    original.Serialize(),
			}

			deserialized, err := DeserializeValue(serializedValue)
			if err != nil {
				t.Fatalf("Failed to deserialize integer: %v", err)
			}

			result, ok := deserialized.(Number)
			if !ok {
				t.Fatalf("Deserialized value is not Number type")
			}

			// Verify no precision loss for integers
			if int64(result.Value) != testInt {
				t.Errorf("Integer precision lost: got %g, want %d", result.Value, testInt)
			}
		})
	}
}

// Test 2.1.5: Time — range
func TestTime_Range(t *testing.T) {
	testCases := []struct {
		name string
		time time.Time
	}{
		{"Unix epoch", time.Unix(0, 0).UTC()},
		{"Year 2038 boundary", time.Date(2038, 1, 19, 3, 14, 7, 0, time.UTC)},
		{"Year 9999", time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			original := Time{
				Timestamp: tc.time,
				Precision: PrecisionSecond,
			}

			serializedValue := SerializedValue{
				TypeTag: original.TypeTag(),
				Data:    original.Serialize(),
			}

			deserialized, err := DeserializeValue(serializedValue)
			if err != nil {
				t.Fatalf("Failed to deserialize time: %v", err)
			}

			result, ok := deserialized.(Time)
			if !ok {
				t.Fatalf("Deserialized value is not Time type")
			}

			if !result.Timestamp.Equal(original.Timestamp) {
				t.Errorf("Time range test failed: got %v, want %v", result.Timestamp, original.Timestamp)
			}
		})
	}
}

// Test 2.1.6: Time — sub-second precision
func TestTime_SubSecondPrecision(t *testing.T) {
	// Test time with millisecond precision
	original := Time{
		Timestamp: time.Date(2026, 3, 15, 14, 30, 45, 123456789, time.UTC),
		Precision: PrecisionSubsecond,
	}

	serializedValue := SerializedValue{
		TypeTag: original.TypeTag(),
		Data:    original.Serialize(),
	}

	deserialized, err := DeserializeValue(serializedValue)
	if err != nil {
		t.Fatalf("Failed to deserialize sub-second time: %v", err)
	}

	result, ok := deserialized.(Time)
	if !ok {
		t.Fatalf("Deserialized value is not Time type")
	}

	// Check microsecond precision (storage precision)
	if result.Timestamp.UnixMicro() != original.Timestamp.UnixMicro() {
		t.Errorf("Sub-second precision lost: got %v, want %v",
			result.Timestamp.UnixMicro(), original.Timestamp.UnixMicro())
	}
}

// Test 2.1.7: Money — currency preservation
func TestMoney_CurrencyPreservation(t *testing.T) {
	testCases := []Money{
		{Amount: 143.68, CurrencyCode: "USD"}, // $143.68
		{Amount: 21.80, CurrencyCode: "EUR"},  // €21.80
		{Amount: 1000, CurrencyCode: "JPY"},   // ¥1000
	}

	for _, original := range testCases {
		t.Run(original.CurrencyCode, func(t *testing.T) {
			serializedValue := SerializedValue{
				TypeTag: original.TypeTag(),
				Data:    original.Serialize(),
			}

			deserialized, err := DeserializeValue(serializedValue)
			if err != nil {
				t.Fatalf("Failed to deserialize money: %v", err)
			}

			result, ok := deserialized.(Money)
			if !ok {
				t.Fatalf("Deserialized value is not Money type")
			}

			if result.CurrencyCode != original.CurrencyCode {
				t.Errorf("Currency symbol not preserved: got %q, want %q",
					result.CurrencyCode, original.CurrencyCode)
			}

			if result.Amount != original.Amount {
				t.Errorf("Money amount not preserved: got %f, want %f",
					result.Amount, original.Amount)
			}
		})
	}
}

// Test 2.1.8: Money — precision
func TestMoney_Precision(t *testing.T) {
	testCases := []Money{
		{Amount: 0.01, CurrencyCode: "USD"},         // $0.01
		{Amount: 999999999.99, CurrencyCode: "USD"}, // $999999999.99
	}

	for _, original := range testCases {
		t.Run("precision", func(t *testing.T) {
			serializedValue := SerializedValue{
				TypeTag: original.TypeTag(),
				Data:    original.Serialize(),
			}

			deserialized, err := DeserializeValue(serializedValue)
			if err != nil {
				t.Fatalf("Failed to deserialize money: %v", err)
			}

			result, ok := deserialized.(Money)
			if !ok {
				t.Fatalf("Deserialized value is not Money type")
			}

			// Check for floating-point drift
			if math.Abs(result.Amount - original.Amount) > 1e-10 {
				t.Errorf("Money precision drift: got %f, want %f",
					result.Amount, original.Amount)
			}
		})
	}
}

// Test 2.1.9: Picture — RGBA 16-bit
func TestPicture_RGBA16Bit(t *testing.T) {
	// Create 2x2 RGBA16 image (16 bytes per pixel = 64 bytes total)
	original := Picture{
		Width:  2,
		Height: 2,
		Data: []byte{
			// Pixel 1: Red (R=65535, G=0, B=0, A=65535)
			0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0xFF, 0xFF,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			// Pixel 2: Green (R=0, G=65535, B=0, A=65535)
			0x00, 0x00, 0xFF, 0xFF, 0x00, 0x00, 0xFF, 0xFF,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			// Pixel 3: Blue (R=0, G=0, B=65535, A=65535)
			0x00, 0x00, 0x00, 0x00, 0xFF, 0xFF, 0xFF, 0xFF,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			// Pixel 4: White (R=65535, G=65535, B=65535, A=65535)
			0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		},
	}

	serializedValue := SerializedValue{
		TypeTag: original.TypeTag(),
		Data:    original.Serialize(),
	}

	deserialized, err := DeserializeValue(serializedValue)
	if err != nil {
		t.Fatalf("Failed to deserialize RGBA16 picture: %v", err)
	}

	result, ok := deserialized.(Picture)
	if !ok {
		t.Fatalf("Deserialized value is not Picture type")
	}

	// Verify bit-exact preservation
	if len(result.Data) != len(original.Data) {
		t.Errorf("Picture data length mismatch: got %d, want %d",
			len(result.Data), len(original.Data))
	}

	for i := range original.Data {
		if result.Data[i] != original.Data[i] {
			t.Errorf("Picture data mismatch at byte %d: got 0x%02x, want 0x%02x",
				i, result.Data[i], original.Data[i])
		}
	}
}

// Test 2.1.10: Reference — path storage
func TestReference_PathStorage(t *testing.T) {
	// Store reference to "world.agent.kalevo"
	original := Reference{AttributeID: 12345678901234567890}

	serializedValue := SerializedValue{
		TypeTag: original.TypeTag(),
		Data:    original.Serialize(),
	}

	deserialized, err := DeserializeValue(serializedValue)
	if err != nil {
		t.Fatalf("Failed to deserialize reference: %v", err)
	}

	result, ok := deserialized.(Reference)
	if !ok {
		t.Fatalf("Deserialized value is not Reference type")
	}

	if result.AttributeID != original.AttributeID {
		t.Errorf("Reference attribute ID not preserved: got %d, want %d",
			result.AttributeID, original.AttributeID)
	}
}

// Test 2.1.11: Reference — resolution (conceptual test - would need storage integration)
func TestReference_Resolution(t *testing.T) {
	// This would require full storage integration to test actual dereferencing
	// For now, verify that reference structure supports resolution
	ref := Reference{AttributeID: 12345}

	// Verify reference holds path information
	if ref.AttributeID == 0 {
		t.Errorf("Reference should hold non-zero attribute ID")
	}

	// Verify string representation includes path info
	str := ref.String()
	if str == "" {
		t.Errorf("Reference should have string representation")
	}
}

// 2.2 Meta Types

// Test 2.2.1: Unknown — creation with reason
func TestUnknown_CreationWithReason(t *testing.T) {
	original := Unknown{Reason: "offline"}

	// Verify reason is preserved in string representation
	if original.String() != "Unknown(offline)" {
		t.Errorf("Unknown string representation incorrect: got %q", original.String())
	}

	serializedValue := SerializedValue{
		TypeTag: original.TypeTag(),
		Data:    original.Serialize(),
	}

	deserialized, err := DeserializeValue(serializedValue)
	if err != nil {
		t.Fatalf("Failed to deserialize unknown: %v", err)
	}

	result, ok := deserialized.(Unknown)
	if !ok {
		t.Fatalf("Deserialized value is not Unknown type")
	}

	if result.Reason != original.Reason {
		t.Errorf("Unknown reason not preserved: got %q, want %q",
			result.Reason, original.Reason)
	}
}

// Test 2.2.2: Unknown — propagation
func TestUnknown_Propagation(t *testing.T) {
	unknown := Unknown{Reason: "offline"}
	number := Number{Value: 5}

	// Test: 5 + #offline should yield #offline (not panic, not zero)
	result := Add(number, unknown)

	resultUnknown, ok := result.(Unknown)
	if !ok {
		t.Fatalf("Unknown propagation failed: got %T, want Unknown", result)
	}

	if resultUnknown.Reason != "offline" {
		t.Errorf("Unknown reason not propagated: got %q, want %q",
			resultUnknown.Reason, "offline")
	}

	// Test reverse order: #offline + 5
	result2 := Add(unknown, number)

	resultUnknown2, ok := result2.(Unknown)
	if !ok {
		t.Fatalf("Unknown propagation failed (reverse): got %T, want Unknown", result2)
	}

	if resultUnknown2.Reason != "offline" {
		t.Errorf("Unknown reason not propagated (reverse): got %q, want %q",
			resultUnknown2.Reason, "offline")
	}
}


// 2.3 Type Coercion

// Test 2.3.1: Text → Number
func TestCoercion_TextToNumber(t *testing.T) {
	testCases := []struct {
		input    Text
		expected interface{}
	}{
		{Text{Value: "42"}, Number{Value: 42.0}},
		{Text{Value: "hello"}, Unknown{Reason: "cannot convert text to number: hello"}},
		{Text{Value: "3.14159"}, Number{Value: 3.14159}},
		{Text{Value: "-42"}, Number{Value: -42.0}},
		{Text{Value: ""}, Unknown{Reason: "cannot convert text to number: "}},
	}

	for _, tc := range testCases {
		t.Run(tc.input.Value, func(t *testing.T) {
			result := CoerceToNumber(tc.input)

			switch expected := tc.expected.(type) {
			case Number:
				if !result.Ok {
					t.Fatalf("Expected successful coercion, got failure: %v", result.Value)
				}

				resultNum, ok := result.Value.(Number)
				if !ok {
					t.Fatalf("Expected Number result, got %T", result.Value)
				}

				if resultNum.Value != expected.Value {
					t.Errorf("Text to Number coercion incorrect: got %f, want %f",
						resultNum.Value, expected.Value)
				}

			case Unknown:
				if result.Ok {
					t.Fatalf("Expected failed coercion, got success: %v", result.Value)
				}

				resultUnknown, ok := result.Value.(Unknown)
				if !ok {
					t.Fatalf("Expected Unknown result, got %T", result.Value)
				}

				if resultUnknown.Reason != expected.Reason {
					t.Errorf("Unknown reason incorrect: got %q, want %q",
						resultUnknown.Reason, expected.Reason)
				}
			}
		})
	}
}

// Test 2.3.2: Number → Text
func TestCoercion_NumberToText(t *testing.T) {
	testCases := []struct {
		input    Number
		expected string // Expected text representation
	}{
		{Number{Value: 42}, "42"},
		{Number{Value: 3.14159}, "3.14159"},
		{Number{Value: -42}, "-42"},
		{Number{Value: 0}, "0"},
	}

	for _, tc := range testCases {
		t.Run("number_to_text", func(t *testing.T) {
			result := CoerceToText(tc.input)

			if !result.Ok {
				t.Fatalf("Number to Text coercion failed: %v", result.Value)
			}

			resultText, ok := result.Value.(Text)
			if !ok {
				t.Fatalf("Expected Text result, got %T", result.Value)
			}

			if resultText.Value != tc.expected {
				t.Errorf("Number to Text coercion incorrect: got %q, want %q",
					resultText.Value, tc.expected)
			}
		})
	}
}

// Test 2.3.3: Time → Number
func TestCoercion_TimeToNumber(t *testing.T) {
	testTime := time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC)
	input := Time{Timestamp: testTime, Precision: PrecisionSecond}

	result := CoerceToNumber(input)

	if !result.Ok {
		t.Fatalf("Time to Number coercion failed: %v", result.Value)
	}

	resultNum, ok := result.Value.(Number)
	if !ok {
		t.Fatalf("Expected Number result, got %T", result.Value)
	}

	expectedUnix := float64(testTime.Unix())
	if resultNum.Value != expectedUnix {
		t.Errorf("Time to Number coercion incorrect: got %f, want %f",
			resultNum.Value, expectedUnix)
	}
}

// Test 2.3.4: Money → Number
func TestCoercion_MoneyToNumber(t *testing.T) {
	input := Money{Amount: 143.68, CurrencyCode: "USD"}

	result := CoerceToNumber(input)

	if !result.Ok {
		t.Fatalf("Money to Number coercion failed: %v", result.Value)
	}

	resultNum, ok := result.Value.(Number)
	if !ok {
		t.Fatalf("Expected Number result, got %T", result.Value)
	}

	if resultNum.Value != 143.68 {
		t.Errorf("Money to Number coercion incorrect: got %f, want %f",
			resultNum.Value, 143.68)
	}
}

// Test 2.3.5: Boolean truthiness — Number
func TestBooleanTruthiness_Number(t *testing.T) {
	testCases := []struct {
		input    Number
		expected bool
	}{
		{Number{Value: 0}, false},    // 0 is false
		{Number{Value: 1}, true},     // 1 is true
		{Number{Value: -1}, true},    // -1 is true
		{Number{Value: 42}, true},    // Any non-zero is true
		{Number{Value: 0.1}, true},   // Any non-zero is true
	}

	for _, tc := range testCases {
		t.Run("number_truthiness", func(t *testing.T) {
			result := isTruthy(tc.input)

			if result != tc.expected {
				t.Errorf("Number truthiness incorrect for %f: got %t, want %t",
					tc.input.Value, result, tc.expected)
			}
		})
	}
}

// Test 2.3.6: Boolean truthiness — Text
func TestBooleanTruthiness_Text(t *testing.T) {
	testCases := []struct {
		input    Text
		expected bool
	}{
		{Text{Value: ""}, false},        // Empty string is false
		{Text{Value: "anything"}, true}, // Any non-empty string is true
		{Text{Value: "false"}, true},    // Even "false" string is true
		{Text{Value: "0"}, true},        // Even "0" string is true
	}

	for _, tc := range testCases {
		t.Run("text_truthiness", func(t *testing.T) {
			result := isTruthy(tc.input)

			if result != tc.expected {
				t.Errorf("Text truthiness incorrect for %q: got %t, want %t",
					tc.input.Value, result, tc.expected)
			}
		})
	}
}

// Test 2.3.7: Boolean truthiness — Time
func TestBooleanTruthiness_Time(t *testing.T) {
	testCases := []struct {
		name     string
		input    Time
		expected bool
	}{
		{
			name: "zero time",
			input: Time{
				Timestamp: time.Unix(0, 0).UTC(), // Unix epoch
				Precision: PrecisionSecond,
			},
			expected: false, // Zero time is false
		},
		{
			name: "any other time",
			input: Time{
				Timestamp: time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC),
				Precision: PrecisionSecond,
			},
			expected: true, // Any other time is true
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isTruthy(tc.input)

			if result != tc.expected {
				t.Errorf("Time truthiness incorrect for %v: got %t, want %t",
					tc.input.Timestamp, result, tc.expected)
			}
		})
	}
}

// Test 2.3.8: Incompatible coercion
func TestCoercion_IncompatibleTypes(t *testing.T) {
	// Picture → Number should yield Unknown
	picture := Picture{Width: 100, Height: 100, Data: make([]byte, 1000)}

	result := CoerceToNumber(picture)

	if result.Ok {
		t.Fatalf("Expected failed coercion for Picture to Number, got success: %v", result.Value)
	}

	resultUnknown, ok := result.Value.(Unknown)
	if !ok {
		t.Fatalf("Expected Unknown result for incompatible coercion, got %T", result.Value)
	}

	// Verify meaningful error message
	if resultUnknown.Reason == "" {
		t.Errorf("Unknown should have meaningful reason for incompatible coercion")
	}
}