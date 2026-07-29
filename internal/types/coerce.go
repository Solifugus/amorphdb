// Package types implements MBL type coercion operations
package types

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// CoercionResult represents the result of a type coercion operation
type CoercionResult struct {
	Value interface{}
	Ok    bool
}

// CoerceToNumber attempts to coerce a value to Number for arithmetic operations
func CoerceToNumber(value interface{}) CoercionResult {
	switch v := value.(type) {
	case Number:
		// Already a number
		return CoercionResult{Value: v, Ok: true}

	case Text:
		// Try to parse as number
		if parsed, err := strconv.ParseFloat(v.Value, 64); err == nil {
			return CoercionResult{Value: Number{Value: parsed}, Ok: true}
		}
		// Cannot parse as number
		return CoercionResult{Value: Unknown{Reason: "cannot convert text to number: " + v.Value}, Ok: false}

	case Money:
		// Use the amount (ignoring currency for arithmetic)
		return CoercionResult{Value: Number{Value: v.Amount}, Ok: true}

	case Time:
		// Convert time to Unix timestamp
		return CoercionResult{Value: Number{Value: float64(v.Timestamp.Unix())}, Ok: true}

	default:
		// Cannot coerce other types to number
		return CoercionResult{
			Value: Unknown{Reason: fmt.Sprintf("cannot coerce %T to number", value)},
			Ok:    false,
		}
	}
}

// CoerceToText attempts to coerce a value to Text for concatenation
func CoerceToText(value interface{}) CoercionResult {
	switch v := value.(type) {
	case Text:
		// Already text
		return CoercionResult{Value: v, Ok: true}

	case Number:
		// Convert number to text
		text := strconv.FormatFloat(v.Value, 'g', -1, 64)
		return CoercionResult{Value: Text{Value: text}, Ok: true}

	case Money:
		// Convert money to text with currency
		text := fmt.Sprintf("%.2f %s", v.Amount, v.CurrencyCode)
		return CoercionResult{Value: Text{Value: text}, Ok: true}

	case Time:
		// Convert time to text representation
		text := formatTimeByPrecision(v)
		return CoercionResult{Value: Text{Value: text}, Ok: true}

	case Reference:
		// Convert reference to text (attribute ID)
		text := fmt.Sprintf("@%d", v.AttributeID)
		return CoercionResult{Value: Text{Value: text}, Ok: true}

	case Nothing:
		// Nothing becomes empty string
		return CoercionResult{Value: Text{Value: ""}, Ok: true}

	case Unknown:
		// Unknown becomes its reason
		return CoercionResult{Value: Text{Value: "Unknown: " + v.Reason}, Ok: true}


	case Anything:
		// Anything becomes a special marker
		return CoercionResult{Value: Text{Value: "Anything"}, Ok: true}

	case List:
		// Convert list to text representation
		text := "["
		for i, element := range v.Elements {
			if i > 0 {
				text += ", "
			}
			elementText := CoerceToText(element)
			if elementText.Ok {
				text += elementText.Value.(Text).Value
			} else {
				text += "Unknown"
			}
		}
		text += "]"
		return CoercionResult{Value: Text{Value: text}, Ok: true}

	case Record:
		// Convert record to text representation
		text := "{"
		first := true
		for key, value := range v.Fields {
			if !first {
				text += ", "
			}
			first = false
			valueText := CoerceToText(value)
			if valueText.Ok {
				text += fmt.Sprintf("%s: %s", key, valueText.Value.(Text).Value)
			} else {
				text += fmt.Sprintf("%s: Unknown", key)
			}
		}
		text += "}"
		return CoercionResult{Value: Text{Value: text}, Ok: true}

	default:
		// Fallback for any missed types
		return CoercionResult{
			Value: Text{Value: fmt.Sprintf("%v", value)},
			Ok:    true,
		}
	}
}

// Arithmetic operations with coercion

// Add performs MBL addition with type coercion
func Add(left, right interface{}) interface{} {
	// Check for Unknown propagation first
	if unknown, ok := left.(Unknown); ok {
		return unknown
	}
	if unknown, ok := right.(Unknown); ok {
		return unknown
	}

	leftNum := CoerceToNumber(left)
	rightNum := CoerceToNumber(right)

	if !leftNum.Ok {
		return leftNum.Value
	}
	if !rightNum.Ok {
		return rightNum.Value
	}

	l := leftNum.Value.(Number)
	r := rightNum.Value.(Number)

	return Number{Value: l.Value + r.Value}
}

// Subtract performs MBL subtraction with type coercion
func Subtract(left, right interface{}) interface{} {
	// Check for Unknown propagation first
	if unknown, ok := left.(Unknown); ok {
		return unknown
	}
	if unknown, ok := right.(Unknown); ok {
		return unknown
	}

	leftNum := CoerceToNumber(left)
	rightNum := CoerceToNumber(right)

	if !leftNum.Ok {
		return leftNum.Value
	}
	if !rightNum.Ok {
		return rightNum.Value
	}

	l := leftNum.Value.(Number)
	r := rightNum.Value.(Number)

	return Number{Value: l.Value - r.Value}
}

// Multiply performs MBL multiplication with type coercion
func Multiply(left, right interface{}) interface{} {
	// Check for Unknown propagation first
	if unknown, ok := left.(Unknown); ok {
		return unknown
	}
	if unknown, ok := right.(Unknown); ok {
		return unknown
	}

	leftNum := CoerceToNumber(left)
	rightNum := CoerceToNumber(right)

	if !leftNum.Ok {
		return leftNum.Value
	}
	if !rightNum.Ok {
		return rightNum.Value
	}

	l := leftNum.Value.(Number)
	r := rightNum.Value.(Number)

	return Number{Value: l.Value * r.Value}
}

// Power performs MBL exponentiation with type coercion
func Power(left, right interface{}) interface{} {
	// Check for Unknown propagation first
	if unknown, ok := left.(Unknown); ok {
		return unknown
	}
	if unknown, ok := right.(Unknown); ok {
		return unknown
	}

	leftNum := CoerceToNumber(left)
	rightNum := CoerceToNumber(right)

	if !leftNum.Ok {
		return leftNum.Value
	}
	if !rightNum.Ok {
		return rightNum.Value
	}

	l := leftNum.Value.(Number)
	r := rightNum.Value.(Number)

	result := math.Pow(l.Value, r.Value)

	// Check for infinity or NaN
	if math.IsInf(result, 0) || math.IsNaN(result) {
		return Unknown{Reason: "exponentiation result out of range"}
	}

	return Number{Value: result}
}

// Divide performs MBL division with type coercion
func Divide(left, right interface{}) interface{} {
	// Check for Unknown propagation first
	if unknown, ok := left.(Unknown); ok {
		return unknown
	}
	if unknown, ok := right.(Unknown); ok {
		return unknown
	}

	leftNum := CoerceToNumber(left)
	rightNum := CoerceToNumber(right)

	if !leftNum.Ok {
		return leftNum.Value
	}
	if !rightNum.Ok {
		return rightNum.Value
	}

	l := leftNum.Value.(Number)
	r := rightNum.Value.(Number)

	// Check for division by zero
	if r.Value == 0 {
		return Unknown{Reason: "division by zero"}
	}

	return Number{Value: l.Value / r.Value}
}

// Modulo performs MBL modulo with type coercion
func Modulo(left, right interface{}) interface{} {
	// Check for Unknown propagation first
	if unknown, ok := left.(Unknown); ok {
		return unknown
	}
	if unknown, ok := right.(Unknown); ok {
		return unknown
	}

	leftNum := CoerceToNumber(left)
	rightNum := CoerceToNumber(right)

	if !leftNum.Ok {
		return leftNum.Value
	}
	if !rightNum.Ok {
		return rightNum.Value
	}

	l := leftNum.Value.(Number)
	r := rightNum.Value.(Number)

	// Check for modulo by zero
	if r.Value == 0 {
		return Unknown{Reason: "modulo by zero"}
	}

	// Go's math.Mod for floating point modulo
	return Number{Value: math.Mod(l.Value, r.Value)}
}

// Concatenate performs MBL concatenation (&) with type coercion
func Concatenate(left, right interface{}) interface{} {
	leftText := CoerceToText(left)
	rightText := CoerceToText(right)

	// Text coercion should always succeed, but check anyway
	if !leftText.Ok {
		return leftText.Value
	}
	if !rightText.Ok {
		return rightText.Value
	}

	l := leftText.Value.(Text)
	r := rightText.Value.(Text)

	return Text{Value: l.Value + r.Value}
}

// Helper functions

// timeLayouts pairs each accepted time literal layout with the precision it
// implies. Order matters: the first layout that parses wins, so they run from
// least to most specific. This is the single source of truth for time literal
// syntax — both the MBL parser and the interpreter parse through
// ParseTimeLiteral so the two can never disagree about what a literal means.
var timeLayouts = []struct {
	layout    string
	precision byte
}{
	{"2006", PrecisionYear},
	{"2006-01", PrecisionMonth},
	{"2006-01-02", PrecisionDay},
	{"2006-01-02 15", PrecisionHour},
	{"2006-01-02 15:04", PrecisionMinute},
	{"2006-01-02 15:04:05", PrecisionSecond},
	{"2006-01-02 15:04:05.000000", PrecisionSubsecond},
	{"2006-01-02T15:04:05", PrecisionSecond},
	{"2006-01-02T15:04:05Z07:00", PrecisionSecond},
}

// ParseTimeLiteral parses an MBL time literal into a Time, inferring the
// precision from the form supplied. A leading '@' is optional. Parsing is done
// in UTC — times are stored as UTC and only rendering converts to a zone.
//
// Precision records what the author actually specified: @2026-01-15 is
// day-precision and genuinely does not know what hour it is. Callers must not
// treat the unspecified components as zero.
func ParseTimeLiteral(s string) (Time, error) {
	s = strings.TrimPrefix(s, "@")

	for _, l := range timeLayouts {
		parsed, err := time.ParseInLocation(l.layout, s, time.UTC)
		if err != nil {
			continue
		}

		precision := l.precision
		// Go's Parse accepts a fractional second after the seconds field even
		// when the layout omits it, so "…:22.5" matches the plain seconds
		// layout. A fractional part means the author specified subseconds.
		if precision == PrecisionSecond && strings.Contains(s, ".") {
			precision = PrecisionSubsecond
		}

		return Time{Timestamp: parsed, Precision: precision}, nil
	}

	return Time{}, fmt.Errorf("cannot parse time: %s", s)
}

// formatTimeByPrecision formats a Time value according to its precision level.
//
// The result carries no '@' sigil: that is source syntax, and this function
// feeds text coercion, whose output lands in sentences and user interfaces.
// Time.String() keeps the '@' form for REPL display and round-tripping.
//
// Only the components the value actually specifies are shown — a day-precision
// time renders "2026-01-15", never "2026-01-15 00:00:00". Hour precision is the
// one concession: it renders ":00" minutes because a bare trailing hour reads as
// truncated rather than deliberate.
func formatTimeByPrecision(t Time) string {
	switch t.Precision {
	case PrecisionYear:
		return fmt.Sprintf("%04d", t.Timestamp.Year())
	case PrecisionMonth:
		return fmt.Sprintf("%04d-%02d", t.Timestamp.Year(), t.Timestamp.Month())
	case PrecisionDay:
		return fmt.Sprintf("%04d-%02d-%02d", t.Timestamp.Year(), t.Timestamp.Month(), t.Timestamp.Day())
	case PrecisionHour:
		return fmt.Sprintf("%04d-%02d-%02d %02d:00",
			t.Timestamp.Year(), t.Timestamp.Month(), t.Timestamp.Day(), t.Timestamp.Hour())
	case PrecisionMinute:
		return fmt.Sprintf("%04d-%02d-%02d %02d:%02d",
			t.Timestamp.Year(), t.Timestamp.Month(), t.Timestamp.Day(),
			t.Timestamp.Hour(), t.Timestamp.Minute())
	case PrecisionSecond:
		return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d",
			t.Timestamp.Year(), t.Timestamp.Month(), t.Timestamp.Day(),
			t.Timestamp.Hour(), t.Timestamp.Minute(), t.Timestamp.Second())
	case PrecisionSubsecond:
		return t.Timestamp.Format("2006-01-02 15:04:05.000000")
	default:
		return t.Timestamp.Format("2006-01-02 15:04:05.000000")
	}
}

// Logical operations (for completeness)

// LogicalAnd performs MBL logical AND
func LogicalAnd(left, right interface{}) bool {
	return isTruthy(left) && isTruthy(right)
}

// LogicalOr performs MBL logical OR
func LogicalOr(left, right interface{}) bool {
	return isTruthy(left) || isTruthy(right)
}

// LogicalNot performs MBL logical NOT
func LogicalNot(value interface{}) bool {
	return !isTruthy(value)
}

// isTruthy determines if a value is "truthy" in MBL
func isTruthy(value interface{}) bool {
	switch v := value.(type) {
	case Number:
		return v.Value != 0.0
	case Text:
		return v.Value != ""
	case Boolean:
		return v.Value
	case Nothing:
		return false
	case Unknown:
		return false
	case Time:
		// Zero time (Unix epoch) is false, any other time is true
		return !v.Timestamp.IsZero() && v.Timestamp.Unix() != 0
	case Anything:
		return true
	case List:
		return len(v.Elements) > 0
	case Record:
		return len(v.Fields) > 0
	default:
		return true // Most values are truthy
	}
}