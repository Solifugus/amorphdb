// Round-trip tests for the READ_AT wire messages, which carry temporal queries
// (my.balance[@2026-01-01]) between the client and the daemon.
//
// The message types and type tags had existed since the protocol was written,
// but neither codec did — so the client stubbed ReadAt out and every temporal
// query failed with "ReadAt not implemented in protocol client". See DEVPLAN
// Step 30.
package protocol

import (
	"reflect"
	"testing"

	"github.com/solifugus/amorphdb/internal/storage"
)

func TestReadAtMessageRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		path      []string
		timestamp int64
	}{
		{"typical", []string{"world", "agent", "1", "balance"}, 1767225600000000},
		{"single segment", []string{"x"}, 0},
		{"negative timestamp (pre-epoch)", []string{"my", "old"}, -631238400000000},
		{"large timestamp", []string{"my", "future"}, 1893456000000000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := &ReadAtMessage{Path: tt.path, Timestamp: tt.timestamp}

			encoded, err := EncodeReadAtMessage(original)
			if err != nil {
				t.Fatalf("Failed to encode ReadAtMessage: %v", err)
			}

			decoded, err := DecodeReadAtMessage(encoded)
			if err != nil {
				t.Fatalf("Failed to decode ReadAtMessage: %v", err)
			}

			if !reflect.DeepEqual(decoded.Path, original.Path) {
				t.Errorf("Path mismatch: expected %v, got %v", original.Path, decoded.Path)
			}
			// The timestamp is the whole point of this message — a silently
			// mangled one would return the wrong historical value rather than
			// an error.
			if decoded.Timestamp != original.Timestamp {
				t.Errorf("Timestamp mismatch: expected %d, got %d", original.Timestamp, decoded.Timestamp)
			}
		})
	}
}

func TestReadAtResponseMessageRoundTrip(t *testing.T) {
	original := &ReadAtResponseMessage{
		Value: storage.Value{
			Data:    []byte("historical value"),
			TypeTag: storage.TypeText,
		},
	}

	encoded, err := EncodeReadAtResponseMessage(original)
	if err != nil {
		t.Fatalf("Failed to encode ReadAtResponseMessage: %v", err)
	}

	decoded, err := DecodeReadAtResponseMessage(encoded)
	if err != nil {
		t.Fatalf("Failed to decode ReadAtResponseMessage: %v", err)
	}

	if !reflect.DeepEqual(decoded.Value, original.Value) {
		t.Errorf("Value mismatch: expected %v, got %v", original.Value, decoded.Value)
	}
}

// TestReadAtResponseCarriesTimeValues matters because a temporal query commonly
// returns a Time, and Time is the type whose serialization needs 9 bytes.
func TestReadAtResponseCarriesTimeValues(t *testing.T) {
	original := &ReadAtResponseMessage{
		Value: storage.Value{
			// 8 bytes of microseconds plus the precision byte.
			Data:    []byte{0, 1, 2, 3, 4, 5, 6, 7, 3},
			TypeTag: storage.TypeTime,
		},
	}

	encoded, err := EncodeReadAtResponseMessage(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeReadAtResponseMessage(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !reflect.DeepEqual(decoded.Value, original.Value) {
		t.Errorf("Time value mismatch: expected %v, got %v", original.Value, decoded.Value)
	}
	if len(decoded.Value.Data) != 9 {
		t.Errorf("Time payload lost bytes: got %d, want 9", len(decoded.Value.Data))
	}
}

// NOTE: there is deliberately no unknown-field forward-compatibility test here.
// The protocol cannot currently skip an unknown field that is not length-prefixed
// — skipField (decode.go) treats only tags 0x01-0x10 as length-prefixed and skips
// a single byte for anything else, which desynchronises the reader. Its own
// comment calls this "a simplified implementation". That is a pre-existing
// protocol limitation, not specific to READ_AT, and it means adding new fields to
// any message is not safely backward compatible.

// --- CHILDREN ---

func TestChildrenMessageRoundTrip(t *testing.T) {
	original := &ChildrenMessage{Path: []string{"world", "agent", "1"}}

	encoded, err := EncodeChildrenMessage(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeChildrenMessage(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(decoded.Path, original.Path) {
		t.Errorf("Path mismatch: expected %v, got %v", original.Path, decoded.Path)
	}
}

func TestChildrenResponseRoundTrip(t *testing.T) {
	attr := func(id uint64, name string) storage.NamedAttribute {
		return storage.NamedAttribute{
			Attribute: storage.Attribute{ID: id, LabelValueID: id + 1, FirstInstanceID: id + 2},
			Name:      name,
		}
	}
	cases := []struct {
		name     string
		children []storage.NamedAttribute
	}{
		{"empty", []storage.NamedAttribute{}},
		{"one", []storage.NamedAttribute{attr(1, "name")}},
		{"several", []storage.NamedAttribute{attr(10, "name"), attr(20, "age"), attr(30, "city")}},
		{"empty name", []storage.NamedAttribute{attr(1, "")}},
		{"unicode name", []storage.NamedAttribute{attr(1, "café ünïcödé")}},
		{"dotted name", []storage.NamedAttribute{attr(1, "css/app.css")}},
		{"large ids", []storage.NamedAttribute{
			{Attribute: storage.Attribute{ID: ^uint64(0), LabelValueID: ^uint64(0) - 1, FirstInstanceID: 1 << 62}, Name: "x"},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := &ChildrenResponseMessage{Children: tc.children}

			encoded, err := EncodeChildrenResponseMessage(original)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			decoded, err := DecodeChildrenResponseMessage(encoded)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if len(decoded.Children) != len(tc.children) {
				t.Fatalf("count = %d, want %d", len(decoded.Children), len(tc.children))
			}
			for i := range tc.children {
				if decoded.Children[i] != tc.children[i] {
					t.Errorf("child %d = %+v, want %+v", i, decoded.Children[i], tc.children[i])
				}
				// The name is the reason this message exists; a silently empty
				// one would leave the client unable to build a record.
				if decoded.Children[i].Name != tc.children[i].Name {
					t.Errorf("child %d name = %q, want %q", i, decoded.Children[i].Name, tc.children[i].Name)
				}
			}
		})
	}
}

// TestDecodeChildrenRejectsImpossibleCount guards against a corrupt or hostile
// length prefix causing a large allocation.
func TestDecodeChildrenRejectsImpossibleCount(t *testing.T) {
	// Tag 0x01, then a count of 1,000,000 with no attribute data following.
	payload := []byte{0x01, 0x00, 0x0F, 0x42, 0x40}
	if _, err := DecodeChildrenResponseMessage(payload); err == nil {
		t.Error("expected an error for a count larger than the payload")
	}
}

// TestDecodeChildrenRejectsOversizedName guards the per-child name length the
// same way as the count: a corrupt length must not drive a large allocation.
func TestDecodeChildrenRejectsOversizedName(t *testing.T) {
	var buf []byte
	buf = append(buf, 0x01)                   // tag
	buf = append(buf, 0x00, 0x00, 0x00, 0x01) // count = 1
	buf = append(buf, make([]byte, 32)...)    // four uint64 fields
	buf = append(buf, 0xFF, 0xFF, 0xFF, 0xFF) // name length = 4294967295
	if _, err := DecodeChildrenResponseMessage(buf); err == nil {
		t.Error("expected an error for a name length larger than the payload")
	}
}
