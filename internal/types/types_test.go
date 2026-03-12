package types

import (
	"bytes"
	"math"
	"testing"
	"time"
	"unicode/utf8"
)

// Test success criteria from development plan:
// - Round-trip each type through serialization: create → serialize → deserialize → compare
// - Text handles Unicode correctly (multi-byte characters)
// - Money preserves currency code through serialization
// - Nothing and Unknown serialize to minimal bytes
// - Unknown preserves its reason string

func TestTextRoundTrip(t *testing.T) {
	// Test basic text
	original := Text{Value: "Hello, World!"}
	value, err := CreateValue(original)
	if err != nil {
		t.Fatalf("CreateValue failed: %v", err)
	}

	// Convert interface to SerializedValue for deserialization
	serializedValue := SerializedValue{
		TypeTag: value.TypeTag(),
		Data:    value.Serialize(),
	}
	deserialized, err := DeserializeValue(serializedValue)
	if err != nil {
		t.Fatalf("DeserializeValue failed: %v", err)
	}

	result, ok := deserialized.(Text)
	if !ok {
		t.Fatalf("Deserialized value is not Text type")
	}

	if result.Value != original.Value {
		t.Errorf("Text value mismatch: got %q, want %q", result.Value, original.Value)
	}
}

func TestTextUnicode(t *testing.T) {
	// Test Unicode multi-byte characters
	testCases := []string{
		"Hello, 世界",           // Chinese characters
		"Café",                 // Accented characters
		"🌟✨",                  // Emojis
		"Москва",               // Cyrillic
		"العربية",              // Arabic
		"한국어",                 // Korean
		"",                     // Empty string
		"a",                    // Single ASCII
	}

	for _, testText := range testCases {
		t.Run("unicode_"+testText, func(t *testing.T) {
			original := Text{Value: testText}

			// Verify the string is valid UTF-8
			if !utf8.ValidString(testText) {
				t.Fatalf("Test string is not valid UTF-8: %q", testText)
			}

			value, err := CreateValue(original)
			if err != nil {
				t.Fatalf("CreateValue failed for %q: %v", testText, err)
			}

			// Convert interface to SerializedValue for deserialization
	serializedValue := SerializedValue{
		TypeTag: value.TypeTag(),
		Data:    value.Serialize(),
	}
	deserialized, err := DeserializeValue(serializedValue)
			if err != nil {
				t.Fatalf("DeserializeValue failed for %q: %v", testText, err)
			}

			result, ok := deserialized.(Text)
			if !ok {
				t.Fatalf("Deserialized value is not Text type")
			}

			if result.Value != original.Value {
				t.Errorf("Unicode text mismatch: got %q, want %q", result.Value, original.Value)
			}

			// Verify UTF-8 validity is preserved
			if !utf8.ValidString(result.Value) {
				t.Errorf("Deserialized text is not valid UTF-8: %q", result.Value)
			}
		})
	}
}

func TestNumberRoundTrip(t *testing.T) {
	testCases := []float64{
		0.0,
		1.0,
		-1.0,
		3.14159,
		-3.14159,
		1.23e10,
		-1.23e-10,
		math.MaxFloat64,
		math.SmallestNonzeroFloat64,
		math.Inf(1),  // Positive infinity
		math.Inf(-1), // Negative infinity
		math.NaN(),   // Not a number
	}

	for _, testValue := range testCases {
		t.Run("number", func(t *testing.T) {
			original := Number{Value: testValue}

			value, err := CreateValue(original)
			if err != nil {
				t.Fatalf("CreateValue failed: %v", err)
			}

			// Convert interface to SerializedValue for deserialization
	serializedValue := SerializedValue{
		TypeTag: value.TypeTag(),
		Data:    value.Serialize(),
	}
	deserialized, err := DeserializeValue(serializedValue)
			if err != nil {
				t.Fatalf("DeserializeValue failed: %v", err)
			}

			result, ok := deserialized.(Number)
			if !ok {
				t.Fatalf("Deserialized value is not Number type")
			}

			// Special handling for NaN
			if math.IsNaN(testValue) {
				if !math.IsNaN(result.Value) {
					t.Errorf("NaN not preserved: got %f", result.Value)
				}
			} else if result.Value != original.Value {
				t.Errorf("Number value mismatch: got %f, want %f", result.Value, original.Value)
			}
		})
	}
}

func TestTimeRoundTrip(t *testing.T) {
	testCases := []Time{
		{
			Timestamp: time.Date(2026, 3, 3, 12, 30, 45, 123456789, time.UTC),
			Precision: PrecisionSubsecond,
		},
		{
			Timestamp: time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC),
			Precision: PrecisionDay,
		},
		{
			Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Precision: PrecisionYear,
		},
		{
			Timestamp: time.Unix(0, 0).UTC(), // Unix epoch
			Precision: PrecisionSecond,
		},
	}

	for _, original := range testCases {
		t.Run("time", func(t *testing.T) {
			value, err := CreateValue(original)
			if err != nil {
				t.Fatalf("CreateValue failed: %v", err)
			}

			// Convert interface to SerializedValue for deserialization
	serializedValue := SerializedValue{
		TypeTag: value.TypeTag(),
		Data:    value.Serialize(),
	}
	deserialized, err := DeserializeValue(serializedValue)
			if err != nil {
				t.Fatalf("DeserializeValue failed: %v", err)
			}

			result, ok := deserialized.(Time)
			if !ok {
				t.Fatalf("Deserialized value is not Time type")
			}

			// Compare timestamps (microsecond precision)
			originalMicros := original.Timestamp.UnixMicro()
			resultMicros := result.Timestamp.UnixMicro()

			if originalMicros != resultMicros {
				t.Errorf("Time timestamp mismatch: got %d, want %d", resultMicros, originalMicros)
			}

			if result.Precision != original.Precision {
				t.Errorf("Time precision mismatch: got %d, want %d", result.Precision, original.Precision)
			}
		})
	}
}

func TestMoneyRoundTrip(t *testing.T) {
	testCases := []Money{
		{Amount: 19.95, CurrencyCode: "USD"},
		{Amount: 1234.56, CurrencyCode: "EUR"},
		{Amount: 0.0, CurrencyCode: "JPY"},
		{Amount: -50.25, CurrencyCode: "GBP"},
	}

	for _, original := range testCases {
		t.Run("money", func(t *testing.T) {
			value, err := CreateValue(original)
			if err != nil {
				t.Fatalf("CreateValue failed: %v", err)
			}

			// Convert interface to SerializedValue for deserialization
	serializedValue := SerializedValue{
		TypeTag: value.TypeTag(),
		Data:    value.Serialize(),
	}
	deserialized, err := DeserializeValue(serializedValue)
			if err != nil {
				t.Fatalf("DeserializeValue failed: %v", err)
			}

			result, ok := deserialized.(Money)
			if !ok {
				t.Fatalf("Deserialized value is not Money type")
			}

			if result.Amount != original.Amount {
				t.Errorf("Money amount mismatch: got %f, want %f", result.Amount, original.Amount)
			}

			if result.CurrencyCode != original.CurrencyCode {
				t.Errorf("Money currency code mismatch: got %q, want %q", result.CurrencyCode, original.CurrencyCode)
			}
		})
	}
}

func TestPictureRoundTrip(t *testing.T) {
	// Test with small RGBA data
	testData := []byte{
		0xFF, 0x00, 0x00, 0xFF, // Red pixel
		0x00, 0xFF, 0x00, 0xFF, // Green pixel
		0x00, 0x00, 0xFF, 0xFF, // Blue pixel
		0xFF, 0xFF, 0xFF, 0xFF, // White pixel
	}

	original := Picture{
		Width:  2,
		Height: 2,
		Data:   testData,
	}

	value, err := CreateValue(original)
	if err != nil {
		t.Fatalf("CreateValue failed: %v", err)
	}

	// Convert interface to SerializedValue for deserialization
	serializedValue := SerializedValue{
		TypeTag: value.TypeTag(),
		Data:    value.Serialize(),
	}
	deserialized, err := DeserializeValue(serializedValue)
	if err != nil {
		t.Fatalf("DeserializeValue failed: %v", err)
	}

	result, ok := deserialized.(Picture)
	if !ok {
		t.Fatalf("Deserialized value is not Picture type")
	}

	if result.Width != original.Width {
		t.Errorf("Picture width mismatch: got %d, want %d", result.Width, original.Width)
	}

	if result.Height != original.Height {
		t.Errorf("Picture height mismatch: got %d, want %d", result.Height, original.Height)
	}

	if !bytes.Equal(result.Data, original.Data) {
		t.Errorf("Picture data mismatch: got %v, want %v", result.Data, original.Data)
	}
}

func TestReferenceRoundTrip(t *testing.T) {
	original := Reference{AttributeID: 12345678901234567890}

	value, err := CreateValue(original)
	if err != nil {
		t.Fatalf("CreateValue failed: %v", err)
	}

	// Convert interface to SerializedValue for deserialization
	serializedValue := SerializedValue{
		TypeTag: value.TypeTag(),
		Data:    value.Serialize(),
	}
	deserialized, err := DeserializeValue(serializedValue)
	if err != nil {
		t.Fatalf("DeserializeValue failed: %v", err)
	}

	result, ok := deserialized.(Reference)
	if !ok {
		t.Fatalf("Deserialized value is not Reference type")
	}

	if result.AttributeID != original.AttributeID {
		t.Errorf("Reference AttributeID mismatch: got %d, want %d", result.AttributeID, original.AttributeID)
	}
}

func TestNothingRoundTrip(t *testing.T) {
	original := Nothing{}

	value, err := CreateValue(original)
	if err != nil {
		t.Fatalf("CreateValue failed: %v", err)
	}

	// Verify Nothing serializes to minimal bytes (empty)
	serializedData := value.Serialize()
	if len(serializedData) != 0 {
		t.Errorf("Nothing should serialize to empty data, got %d bytes", len(serializedData))
	}

	// Convert interface to SerializedValue for deserialization
	serializedValue := SerializedValue{
		TypeTag: value.TypeTag(),
		Data:    value.Serialize(),
	}
	deserialized, err := DeserializeValue(serializedValue)
	if err != nil {
		t.Fatalf("DeserializeValue failed: %v", err)
	}

	_, ok := deserialized.(Nothing)
	if !ok {
		t.Fatalf("Deserialized value is not Nothing type")
	}
}

func TestUnknownRoundTrip(t *testing.T) {
	testCases := []string{
		"division by zero",
		"invalid operation",
		"",
		"Unicode reason: 错误信息",
	}

	for _, reason := range testCases {
		t.Run("unknown_"+reason, func(t *testing.T) {
			original := Unknown{Reason: reason}

			value, err := CreateValue(original)
			if err != nil {
				t.Fatalf("CreateValue failed: %v", err)
			}

			// Verify Unknown preserves reason and uses minimal encoding
			expectedSize := 4 + len([]byte(reason)) // 4 bytes length + reason
			serializedData := value.Serialize()
			if len(serializedData) != expectedSize {
				t.Errorf("Unknown serialization size mismatch: got %d, want %d", len(serializedData), expectedSize)
			}

			// Convert interface to SerializedValue for deserialization
	serializedValue := SerializedValue{
		TypeTag: value.TypeTag(),
		Data:    value.Serialize(),
	}
	deserialized, err := DeserializeValue(serializedValue)
			if err != nil {
				t.Fatalf("DeserializeValue failed: %v", err)
			}

			result, ok := deserialized.(Unknown)
			if !ok {
				t.Fatalf("Deserialized value is not Unknown type")
			}

			if result.Reason != original.Reason {
				t.Errorf("Unknown reason mismatch: got %q, want %q", result.Reason, original.Reason)
			}
		})
	}
}

func TestAnythingRoundTrip(t *testing.T) {
	original := Anything{}

	value, err := CreateValue(original)
	if err != nil {
		t.Fatalf("CreateValue failed: %v", err)
	}

	// Verify Anything serializes to minimal bytes (empty)
	serializedData := value.Serialize()
	if len(serializedData) != 0 {
		t.Errorf("Anything should serialize to empty data, got %d bytes", len(serializedData))
	}

	// Convert interface to SerializedValue for deserialization
	serializedValue := SerializedValue{
		TypeTag: value.TypeTag(),
		Data:    value.Serialize(),
	}
	deserialized, err := DeserializeValue(serializedValue)
	if err != nil {
		t.Fatalf("DeserializeValue failed: %v", err)
	}

	_, ok := deserialized.(Anything)
	if !ok {
		t.Fatalf("Deserialized value is not Anything type")
	}
}

func TestTypeTagConsistency(t *testing.T) {
	// Verify that type tags are preserved correctly
	testCases := []struct {
		value       interface{}
		expectedTag byte
	}{
		{Text{Value: "test"}, TypeText},
		{Number{Value: 42.0}, TypeNumber},
		{Time{Timestamp: time.Now().UTC(), Precision: PrecisionSecond}, TypeTime},
		{Money{Amount: 19.95, CurrencyCode: "USD"}, TypeMoney},
		{Picture{Width: 1, Height: 1, Data: []byte{0x00}}, TypePicture},
		{Reference{AttributeID: 123}, TypeReference},
		{Nothing{}, TypeNothing},
		{Unknown{Reason: "test"}, TypeUnknown},
		{Anything{}, TypeAnything},
	}

	for _, tc := range testCases {
		t.Run("type_tag", func(t *testing.T) {
			value, err := CreateValue(tc.value)
			if err != nil {
				t.Fatalf("CreateValue failed: %v", err)
			}

			if value.TypeTag() != tc.expectedTag {
				t.Errorf("Type tag mismatch: got 0x%02x, want 0x%02x", value.TypeTag(), tc.expectedTag)
			}
		})
	}
}

// Test edge cases and error conditions

func TestDeserializationErrors(t *testing.T) {
	// Test various malformed data scenarios
	testCases := []struct {
		name    string
		value   SerializedValue
		wantErr bool
	}{
		{
			name:    "invalid text data too short",
			value:   SerializedValue{TypeTag: TypeText, Data: []byte{0x01}}, // Missing length data
			wantErr: true,
		},
		{
			name:    "invalid number data wrong size",
			value:   SerializedValue{TypeTag: TypeNumber, Data: []byte{0x01, 0x02, 0x03}}, // Not 8 bytes
			wantErr: true,
		},
		{
			name:    "invalid time data wrong size",
			value:   SerializedValue{TypeTag: TypeTime, Data: []byte{0x01, 0x02, 0x03}}, // Not 9 bytes
			wantErr: true,
		},
		{
			name:    "invalid time precision",
			value:   SerializedValue{TypeTag: TypeTime, Data: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xFF}}, // Invalid precision
			wantErr: true,
		},
		{
			name:    "invalid money data wrong size",
			value:   SerializedValue{TypeTag: TypeMoney, Data: []byte{0x01, 0x02, 0x03}}, // Not 11 bytes
			wantErr: true,
		},
		{
			name:    "nothing with data",
			value:   SerializedValue{TypeTag: TypeNothing, Data: []byte{0x01}}, // Should be empty
			wantErr: true,
		},
		{
			name:    "anything with data",
			value:   SerializedValue{TypeTag: TypeAnything, Data: []byte{0x01}}, // Should be empty
			wantErr: true,
		},
		{
			name:    "unknown type tag",
			value:   SerializedValue{TypeTag: 0xFF, Data: []byte{}}, // Unknown type
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DeserializeValue(tc.value)
			if tc.wantErr && err == nil {
				t.Errorf("Expected error for %s, got none", tc.name)
			} else if !tc.wantErr && err != nil {
				t.Errorf("Unexpected error for %s: %v", tc.name, err)
			}
		})
	}
}