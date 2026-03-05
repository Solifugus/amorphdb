// Package types implements MBL type coercion operations
package types

import (
	"fmt"
	"math"
	"strconv"
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

// Divide performs MBL division with type coercion
func Divide(left, right interface{}) interface{} {
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

// formatTimeByPrecision formats a Time value according to its precision level
func formatTimeByPrecision(t Time) string {
	switch t.Precision {
	case PrecisionYear:
		return fmt.Sprintf("@%04d", t.Timestamp.Year())
	case PrecisionMonth:
		return fmt.Sprintf("@%04d-%02d", t.Timestamp.Year(), t.Timestamp.Month())
	case PrecisionDay:
		return fmt.Sprintf("@%04d-%02d-%02d", t.Timestamp.Year(), t.Timestamp.Month(), t.Timestamp.Day())
	case PrecisionHour:
		return fmt.Sprintf("@%04d-%02d-%02d %02d:00:00",
			t.Timestamp.Year(), t.Timestamp.Month(), t.Timestamp.Day(), t.Timestamp.Hour())
	case PrecisionMinute:
		return fmt.Sprintf("@%04d-%02d-%02d %02d:%02d:00",
			t.Timestamp.Year(), t.Timestamp.Month(), t.Timestamp.Day(),
			t.Timestamp.Hour(), t.Timestamp.Minute())
	case PrecisionSecond:
		return fmt.Sprintf("@%04d-%02d-%02d %02d:%02d:%02d",
			t.Timestamp.Year(), t.Timestamp.Month(), t.Timestamp.Day(),
			t.Timestamp.Hour(), t.Timestamp.Minute(), t.Timestamp.Second())
	case PrecisionSubsecond:
		return fmt.Sprintf("@%s", t.Timestamp.Format("2006-01-02 15:04:05.000000"))
	default:
		return fmt.Sprintf("@%s", t.Timestamp.Format("2006-01-02 15:04:05.000000"))
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