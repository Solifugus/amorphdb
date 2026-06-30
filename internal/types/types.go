// Package types implements all MBL data types with serialization/deserialization
package types

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"
	"unicode/utf8"
)

// Type constants (imported from storage layer for consistency)
const (
	TypeText      = 0x01
	TypeNumber    = 0x02
	TypeTime      = 0x03
	TypeMoney     = 0x04
	TypePicture   = 0x05
	TypeReference = 0x06
	TypeProcedure = 0x07
	TypeWatcher   = 0x08
	TypeEmbed     = 0x09
	TypeBoolean   = 0x0A
	TypeList      = 0x0B
	TypeRecord    = 0x0C
	TypeNothing   = 0xF0
	TypeUnknown   = 0xF1
	TypeAnything  = 0xF2
)

// Time precision levels
const (
	PrecisionYear      = 1
	PrecisionMonth     = 2
	PrecisionDay       = 3
	PrecisionHour      = 4
	PrecisionMinute    = 5
	PrecisionSecond    = 6
	PrecisionSubsecond = 7
)

// Value represents any MBL value - interface implemented by all MBL types
type Value interface {
	// Serialize converts the value to binary format
	Serialize() []byte
	// TypeTag returns the type identifier for this value
	TypeTag() byte
	// String returns a string representation of the value
	String() string
}

// SerializedValue represents a value in its serialized form for storage
type SerializedValue struct {
	TypeTag byte
	Data    []byte
}

// Text represents a MBL Text value
type Text struct {
	Value string
}

// Number represents a MBL Number value (IEEE 754 float64)
type Number struct {
	Value float64
}

// Time represents a MBL Time value with precision tracking
type Time struct {
	Timestamp time.Time
	Precision byte // PrecisionYear, PrecisionMonth, etc.
}

// Money represents a MBL Money value with currency
type Money struct {
	Amount       float64 // Stored as float64 for simplicity
	CurrencyCode string  // 3-character currency code (USD, EUR, etc.)
}

// Picture represents a MBL Picture value (RGBA image data)
type Picture struct {
	Width  uint32
	Height uint32
	Data   []byte // RGBA 16-bit per channel data
}

// Reference represents a MBL Reference value (points to an attribute)
type Reference struct {
	AttributeID uint64
}

// Nothing represents the MBL Nothing singleton
type Nothing struct{}

// Unknown represents a MBL Unknown value with a reason
type Unknown struct {
	Reason string
}

// Anything represents the MBL Anything singleton
type Anything struct{}

// Boolean represents a MBL Boolean value
type Boolean struct {
	Value bool
}

// List represents a MBL List value (ordered collection)
type List struct {
	Elements []interface{}
}

// Record represents a MBL Record value (key-value pairs)
type Record struct {
	Fields map[string]interface{}
}

// Serialization methods

// Serialize converts a Text to binary format
func (t Text) Serialize() []byte {
	data := []byte(t.Value)
	buf := make([]byte, 4+len(data))
	binary.LittleEndian.PutUint32(buf[0:4], uint32(len(data)))
	copy(buf[4:], data)
	return buf
}

// TypeTag returns the type identifier for Text
func (t Text) TypeTag() byte {
	return TypeText
}

// String returns a string representation of Text
func (t Text) String() string {
	return t.Value
}

// Serialize converts a Number to binary format (IEEE 754)
func (n Number) Serialize() []byte {
	buf := make([]byte, 8)
	bits := math.Float64bits(n.Value)
	binary.LittleEndian.PutUint64(buf, bits)
	return buf
}

// TypeTag returns the type identifier for Number
func (n Number) TypeTag() byte {
	return TypeNumber
}

// String returns a string representation of Number
func (n Number) String() string {
	return fmt.Sprintf("%g", n.Value)
}

// Serialize converts a Time to binary format (8 bytes UNIX + 1 byte precision)
func (t Time) Serialize() []byte {
	buf := make([]byte, 9)
	// Store as microseconds since UNIX epoch
	micros := t.Timestamp.UnixMicro()
	binary.LittleEndian.PutUint64(buf[0:8], uint64(micros))
	buf[8] = t.Precision
	return buf
}

// TypeTag returns the type identifier for Time
func (t Time) TypeTag() byte {
	return TypeTime
}

// String returns a string representation of Time
func (t Time) String() string {
	return fmt.Sprintf("@%s", t.Timestamp.Format("2006-01-02 15:04:05"))
}

// Serialize converts Money to binary format (8 bytes amount + currency code)
func (m Money) Serialize() []byte {
	buf := make([]byte, 11) // 8 bytes amount + 3 bytes currency
	bits := math.Float64bits(m.Amount)
	binary.LittleEndian.PutUint64(buf[0:8], bits)
	copy(buf[8:11], []byte(m.CurrencyCode)[:3]) // Ensure exactly 3 bytes
	return buf
}

// TypeTag returns the type identifier for Money
func (m Money) TypeTag() byte {
	return TypeMoney
}

// String returns a string representation of Money
func (m Money) String() string {
	return fmt.Sprintf("¤%s%.2f", m.CurrencyCode, m.Amount)
}

// Serialize converts Picture to binary format (dimensions + RGBA data)
func (p Picture) Serialize() []byte {
	buf := make([]byte, 8+len(p.Data))
	binary.LittleEndian.PutUint32(buf[0:4], p.Width)
	binary.LittleEndian.PutUint32(buf[4:8], p.Height)
	copy(buf[8:], p.Data)
	return buf
}

// TypeTag returns the type identifier for Picture
func (p Picture) TypeTag() byte {
	return TypePicture
}

// String returns a string representation of Picture
func (p Picture) String() string {
	return fmt.Sprintf("Picture(%dx%d)", p.Width, p.Height)
}

// Serialize converts Reference to binary format (8 bytes)
func (r Reference) Serialize() []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, r.AttributeID)
	return buf
}

// TypeTag returns the type identifier for Reference
func (r Reference) TypeTag() byte {
	return TypeReference
}

// String returns a string representation of Reference
func (r Reference) String() string {
	return fmt.Sprintf("&%d", r.AttributeID)
}

// Serialize converts Nothing to binary format (empty)
func (n Nothing) Serialize() []byte {
	return []byte{}
}

// TypeTag returns the type identifier for Nothing
func (n Nothing) TypeTag() byte {
	return TypeNothing
}

// String returns a string representation of Nothing
func (n Nothing) String() string {
	return "Nothing"
}

// Serialize converts Unknown to binary format (reason text with length prefix)
func (u Unknown) Serialize() []byte {
	reasonData := []byte(u.Reason)
	buf := make([]byte, 4+len(reasonData))
	binary.LittleEndian.PutUint32(buf[0:4], uint32(len(reasonData)))
	copy(buf[4:], reasonData)
	return buf
}

// TypeTag returns the type identifier for Unknown
func (u Unknown) TypeTag() byte {
	return TypeUnknown
}

// String returns a string representation of Unknown
func (u Unknown) String() string {
	return fmt.Sprintf("Unknown(%s)", u.Reason)
}

// Serialize converts Anything to binary format (empty)
func (a Anything) Serialize() []byte {
	return []byte{}
}

// TypeTag returns the type identifier for Anything
func (a Anything) TypeTag() byte {
	return TypeAnything
}

// String returns a string representation of Anything
func (a Anything) String() string {
	return "Anything"
}

// Serialize converts Boolean to binary format (1 byte)
func (b Boolean) Serialize() []byte {
	if b.Value {
		return []byte{1}
	}
	return []byte{0}
}

// TypeTag returns the type identifier for Boolean
func (b Boolean) TypeTag() byte {
	return TypeBoolean
}

// String returns a string representation of Boolean
func (b Boolean) String() string {
	if b.Value {
		return "true"
	}
	return "false"
}

// Serialize converts List to binary format (count + serialized elements)
func (l List) Serialize() []byte {
	buf := make([]byte, 4) // Start with element count
	binary.LittleEndian.PutUint32(buf[0:4], uint32(len(l.Elements)))

	for _, element := range l.Elements {
		elementValue, err := CreateValue(element)
		if err != nil {
			// Skip invalid elements - could add error handling here
			continue
		}

		// Add type tag + length + data
		elementData := elementValue.Serialize()
		elementBuf := make([]byte, 1+4+len(elementData))
		elementBuf[0] = elementValue.TypeTag()
		binary.LittleEndian.PutUint32(elementBuf[1:5], uint32(len(elementData)))
		copy(elementBuf[5:], elementData)

		buf = append(buf, elementBuf...)
	}

	return buf
}

// TypeTag returns the type identifier for List
func (l List) TypeTag() byte {
	return TypeList
}

// String returns a string representation of List
func (l List) String() string {
	return fmt.Sprintf("List(%d elements)", len(l.Elements))
}

// Serialize converts Record to binary format (count + serialized field pairs)
func (r Record) Serialize() []byte {
	buf := make([]byte, 4) // Start with field count
	binary.LittleEndian.PutUint32(buf[0:4], uint32(len(r.Fields)))

	for key, value := range r.Fields {
		// Serialize the key as text
		keyData := Text{Value: key}.Serialize()

		// Serialize the value
		valueValue, err := CreateValue(value)
		if err != nil {
			// Skip invalid fields
			continue
		}
		valueData := valueValue.Serialize()

		// Add key length + key + value type + value length + value data
		fieldBuf := make([]byte, 4+len(keyData)+1+4+len(valueData))
		offset := 0

		// Key length and data
		binary.LittleEndian.PutUint32(fieldBuf[offset:offset+4], uint32(len(keyData)))
		offset += 4
		copy(fieldBuf[offset:offset+len(keyData)], keyData)
		offset += len(keyData)

		// Value type tag
		fieldBuf[offset] = valueValue.TypeTag()
		offset++

		// Value length and data
		binary.LittleEndian.PutUint32(fieldBuf[offset:offset+4], uint32(len(valueData)))
		offset += 4
		copy(fieldBuf[offset:], valueData)

		buf = append(buf, fieldBuf...)
	}

	return buf
}

// TypeTag returns the type identifier for Record
func (r Record) TypeTag() byte {
	return TypeRecord
}

// String returns a string representation of Record
func (r Record) String() string {
	return fmt.Sprintf("Record(%d fields)", len(r.Fields))
}

// Deserialization functions

// DeserializeText deserializes binary data to Text
func DeserializeText(data []byte) (Text, error) {
	if len(data) < 4 {
		return Text{}, fmt.Errorf("text data too short")
	}

	length := binary.LittleEndian.Uint32(data[0:4])
	if len(data) < int(4+length) {
		return Text{}, fmt.Errorf("text data truncated")
	}

	textBytes := data[4 : 4+length]
	if !utf8.Valid(textBytes) {
		return Text{}, fmt.Errorf("invalid UTF-8 in text data")
	}

	return Text{Value: string(textBytes)}, nil
}

// DeserializeNumber deserializes binary data to Number
func DeserializeNumber(data []byte) (Number, error) {
	if len(data) != 8 {
		return Number{}, fmt.Errorf("number data must be 8 bytes")
	}

	bits := binary.LittleEndian.Uint64(data)
	value := math.Float64frombits(bits)

	return Number{Value: value}, nil
}

// DeserializeTime deserializes binary data to Time
func DeserializeTime(data []byte) (Time, error) {
	if len(data) != 9 {
		return Time{}, fmt.Errorf("time data must be 9 bytes")
	}

	micros := int64(binary.LittleEndian.Uint64(data[0:8]))
	precision := data[8]

	// Validate precision level
	if precision < PrecisionYear || precision > PrecisionSubsecond {
		return Time{}, fmt.Errorf("invalid time precision: %d", precision)
	}

	timestamp := time.UnixMicro(micros).UTC()

	return Time{
		Timestamp: timestamp,
		Precision: precision,
	}, nil
}

// DeserializeMoney deserializes binary data to Money
func DeserializeMoney(data []byte) (Money, error) {
	if len(data) != 11 {
		return Money{}, fmt.Errorf("money data must be 11 bytes")
	}

	bits := binary.LittleEndian.Uint64(data[0:8])
	amount := math.Float64frombits(bits)
	currency := string(data[8:11])

	return Money{
		Amount:       amount,
		CurrencyCode: currency,
	}, nil
}

// DeserializePicture deserializes binary data to Picture
func DeserializePicture(data []byte) (Picture, error) {
	if len(data) < 8 {
		return Picture{}, fmt.Errorf("picture data too short")
	}

	width := binary.LittleEndian.Uint32(data[0:4])
	height := binary.LittleEndian.Uint32(data[4:8])
	imageData := data[8:]

	return Picture{
		Width:  width,
		Height: height,
		Data:   imageData,
	}, nil
}

// DeserializeReference deserializes binary data to Reference
func DeserializeReference(data []byte) (Reference, error) {
	if len(data) != 8 {
		return Reference{}, fmt.Errorf("reference data must be 8 bytes")
	}

	attributeID := binary.LittleEndian.Uint64(data)

	return Reference{AttributeID: attributeID}, nil
}

// DeserializeNothing deserializes binary data to Nothing
func DeserializeNothing(data []byte) (Nothing, error) {
	if len(data) != 0 {
		return Nothing{}, fmt.Errorf("nothing data must be empty")
	}
	return Nothing{}, nil
}

// DeserializeUnknown deserializes binary data to Unknown
func DeserializeUnknown(data []byte) (Unknown, error) {
	if len(data) < 4 {
		return Unknown{}, fmt.Errorf("unknown data too short")
	}

	length := binary.LittleEndian.Uint32(data[0:4])
	if len(data) < int(4+length) {
		return Unknown{}, fmt.Errorf("unknown data truncated")
	}

	reasonBytes := data[4 : 4+length]
	if !utf8.Valid(reasonBytes) {
		return Unknown{}, fmt.Errorf("invalid UTF-8 in unknown reason")
	}

	return Unknown{Reason: string(reasonBytes)}, nil
}

// DeserializeAnything deserializes binary data to Anything
func DeserializeAnything(data []byte) (Anything, error) {
	if len(data) != 0 {
		return Anything{}, fmt.Errorf("anything data must be empty")
	}
	return Anything{}, nil
}

// DeserializeBoolean deserializes binary data to Boolean
func DeserializeBoolean(data []byte) (Boolean, error) {
	if len(data) != 1 {
		return Boolean{}, fmt.Errorf("boolean data must be 1 byte")
	}
	return Boolean{Value: data[0] != 0}, nil
}

// DeserializeList deserializes binary data to List
func DeserializeList(data []byte) (List, error) {
	if len(data) < 4 {
		return List{}, fmt.Errorf("list data too short")
	}

	count := binary.LittleEndian.Uint32(data[0:4])
	elements := make([]interface{}, 0, count)

	offset := 4
	for i := uint32(0); i < count && offset < len(data); i++ {
		if offset+5 > len(data) {
			return List{}, fmt.Errorf("list element data truncated")
		}

		// Read type tag + length
		typeTag := data[offset]
		offset++
		length := binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4

		if offset+int(length) > len(data) {
			return List{}, fmt.Errorf("list element data truncated")
		}

		// Deserialize element
		elementValue := SerializedValue{TypeTag: typeTag, Data: data[offset : offset+int(length)]}
		element, err := DeserializeValue(elementValue)
		if err != nil {
			return List{}, fmt.Errorf("failed to deserialize list element: %v", err)
		}

		elements = append(elements, element)
		offset += int(length)
	}

	return List{Elements: elements}, nil
}

// DeserializeRecord deserializes binary data to Record
func DeserializeRecord(data []byte) (Record, error) {
	if len(data) < 4 {
		return Record{}, fmt.Errorf("record data too short")
	}

	count := binary.LittleEndian.Uint32(data[0:4])

	// Do not preallocate based on the untrusted count: a malformed or
	// non-record byte slice can carry a wildly large count (e.g. garbage
	// bytes read as a uint32), and make(map, count) would attempt an enormous
	// allocation before the per-field bounds checks below ever run. The loop
	// is bounded by offset < len(data), so a sane initial capacity is safe;
	// every real field consumes at least 9 bytes (4 key-len + 1 type + 4
	// value-len), giving a tight upper bound on the achievable field count.
	capHint := count
	if max := uint32(len(data) / 9); capHint > max {
		capHint = max
	}
	fields := make(map[string]interface{}, capHint)

	offset := 4
	for i := uint32(0); i < count && offset < len(data); i++ {
		if offset+4 > len(data) {
			return Record{}, fmt.Errorf("record field data truncated")
		}

		// Read key length and data
		keyLength := binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4

		if offset+int(keyLength) > len(data) {
			return Record{}, fmt.Errorf("record field key truncated")
		}

		keyText, err := DeserializeText(data[offset : offset+int(keyLength)])
		if err != nil {
			return Record{}, fmt.Errorf("failed to deserialize record key: %v", err)
		}
		offset += int(keyLength)

		// Read value type tag + length + data
		if offset+5 > len(data) {
			return Record{}, fmt.Errorf("record field value truncated")
		}

		valueType := data[offset]
		offset++
		valueLength := binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4

		if offset+int(valueLength) > len(data) {
			return Record{}, fmt.Errorf("record field value data truncated")
		}

		valueValue := SerializedValue{TypeTag: valueType, Data: data[offset : offset+int(valueLength)]}
		value, err := DeserializeValue(valueValue)
		if err != nil {
			return Record{}, fmt.Errorf("failed to deserialize record value: %v", err)
		}

		fields[keyText.Value] = value
		offset += int(valueLength)
	}

	return Record{Fields: fields}, nil
}

// Helper functions

// CreateValue creates a Value interface from any MBL type or Go primitive
func CreateValue(mblType interface{}) (Value, error) {
	// If it's already a Value type, return it directly
	if v, ok := mblType.(Value); ok {
		return v, nil
	}

	// Handle MBL types
	switch v := mblType.(type) {
	case Text:
		return v, nil
	case *Text:
		return *v, nil
	case Number:
		return v, nil
	case *Number:
		return *v, nil
	case Time:
		return v, nil
	case *Time:
		return *v, nil
	case Money:
		return v, nil
	case *Money:
		return *v, nil
	case Picture:
		return v, nil
	case *Picture:
		return *v, nil
	case Reference:
		return v, nil
	case *Reference:
		return *v, nil
	case Nothing:
		return v, nil
	case *Nothing:
		return *v, nil
	case Unknown:
		return v, nil
	case *Unknown:
		return *v, nil
	case Anything:
		return v, nil
	case *Anything:
		return *v, nil
	case Boolean:
		return v, nil
	case *Boolean:
		return *v, nil
	case List:
		return v, nil
	case *List:
		return *v, nil
	case Record:
		return v, nil
	case *Record:
		return *v, nil

	// Handle Go primitive types
	case string:
		return Text{Value: v}, nil
	case float64:
		return Number{Value: v}, nil
	case float32:
		return Number{Value: float64(v)}, nil
	case int:
		return Number{Value: float64(v)}, nil
	case int64:
		return Number{Value: float64(v)}, nil
	case int32:
		return Number{Value: float64(v)}, nil
	case uint:
		return Number{Value: float64(v)}, nil
	case uint32:
		return Number{Value: float64(v)}, nil
	case uint64:
		// Agent/author IDs are uint64 throughout the system; represent as Number.
		return Number{Value: float64(v)}, nil
	case bool:
		return Boolean{Value: v}, nil

	default:
		return nil, fmt.Errorf("unsupported type: %T", mblType)
	}
}

// DeserializeValue deserializes a SerializedValue to the appropriate MBL type
func DeserializeValue(v SerializedValue) (interface{}, error) {
	switch v.TypeTag {
	case TypeText:
		return DeserializeText(v.Data)
	case TypeNumber:
		return DeserializeNumber(v.Data)
	case TypeTime:
		return DeserializeTime(v.Data)
	case TypeMoney:
		return DeserializeMoney(v.Data)
	case TypePicture:
		return DeserializePicture(v.Data)
	case TypeReference:
		return DeserializeReference(v.Data)
	case TypeNothing:
		return DeserializeNothing(v.Data)
	case TypeUnknown:
		return DeserializeUnknown(v.Data)
	case TypeAnything:
		return DeserializeAnything(v.Data)
	case TypeBoolean:
		return DeserializeBoolean(v.Data)
	case TypeList:
		return DeserializeList(v.Data)
	case TypeRecord:
		return DeserializeRecord(v.Data)
	default:
		return nil, fmt.Errorf("unknown type tag: 0x%02x", v.TypeTag)
	}
}

// ToSerializedValue converts a Value interface to SerializedValue for storage
func ToSerializedValue(v Value) SerializedValue {
	return SerializedValue{
		TypeTag: v.TypeTag(),
		Data:    v.Serialize(),
	}
}
