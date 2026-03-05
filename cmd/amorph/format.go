// Package main implements result formatting for MBL types
package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/solifugus/amorphdb/internal/types"
)

// formatResult formats any MBL value for user display
func (r *REPL) formatResult(result interface{}) string {
	if result == nil {
		return ""
	}

	switch v := result.(type) {
	case types.Nothing:
		return "" // Nothing produces no output
	case types.Text:
		return v.Value
	case types.Number:
		return r.formatNumber(v.Value)
	case types.Boolean:
		return r.formatBoolean(v.Value)
	case types.Time:
		return r.formatTime(v.Timestamp, int(v.Precision))
	case types.Money:
		return r.formatMoney(v.Amount, v.CurrencyCode)
	case types.List:
		return r.formatList(v)
	case types.Record:
		return r.formatRecord(v)
	case types.Reference:
		return r.formatReference(v.AttributeID)
	case types.Unknown:
		return r.formatUnknown(v.Reason)
	case string:
		// Raw string result (e.g., from output function)
		return v
	default:
		// Fallback for unexpected types
		return fmt.Sprintf("%v", result)
	}
}

// formatNumber formats numeric values avoiding unnecessary decimals
func (r *REPL) formatNumber(value float64) string {
	// Check if it's effectively an integer
	if value == math.Trunc(value) && math.Abs(value) < 1e15 {
		return strconv.FormatFloat(value, 'f', 0, 64)
	}
	// Format with appropriate precision
	return strconv.FormatFloat(value, 'g', -1, 64)
}

// formatBoolean formats boolean values
func (r *REPL) formatBoolean(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

// formatTime formats time values with appropriate precision
func (r *REPL) formatTime(value time.Time, precision int) string {
	switch precision {
	case types.PrecisionYear:
		return value.Format("2006")
	case types.PrecisionMonth:
		return value.Format("2006-01")
	case types.PrecisionDay:
		return value.Format("2006-01-02")
	case types.PrecisionHour:
		return value.Format("2006-01-02 15")
	case types.PrecisionMinute:
		return value.Format("2006-01-02 15:04")
	case types.PrecisionSecond:
		return value.Format("2006-01-02 15:04:05")
	case types.PrecisionSubsecond:
		return value.Format("2006-01-02 15:04:05.000")
	default:
		return value.Format(time.RFC3339)
	}
}

// formatMoney formats monetary values
func (r *REPL) formatMoney(amount float64, currency string) string {
	if currency == "" {
		currency = "USD" // Default currency
	}
	return fmt.Sprintf("%.2f %s", amount, currency)
}

// formatList formats MBL lists
func (r *REPL) formatList(list types.List) string {
	if len(list.Elements) == 0 {
		return "[]"
	}

	var elements []string
	for _, elem := range list.Elements {
		elements = append(elements, r.formatResult(elem))
	}

	return "[" + strings.Join(elements, ", ") + "]"
}

// formatRecord formats MBL records
func (r *REPL) formatRecord(record types.Record) string {
	if len(record.Fields) == 0 {
		return "{}"
	}

	var fields []string
	for key, value := range record.Fields {
		formattedValue := r.formatResult(value)
		// Quote keys if they contain special characters
		if r.needsQuoting(key) {
			key = fmt.Sprintf("\"%s\"", key)
		}
		fields = append(fields, fmt.Sprintf("%s: %s", key, formattedValue))
	}

	return "{" + strings.Join(fields, ", ") + "}"
}

// formatReference formats attribute references
func (r *REPL) formatReference(attributeID uint64) string {
	return fmt.Sprintf("ref:%d", attributeID)
}

// formatUnknown formats Unknown values with their reason
func (r *REPL) formatUnknown(reason string) string {
	return fmt.Sprintf("Unknown: %s", reason)
}

// needsQuoting checks if a record key needs quoting
func (r *REPL) needsQuoting(key string) bool {
	// Simple check: if key contains spaces or special characters
	for _, ch := range key {
		if ch == ' ' || ch == ':' || ch == ',' || ch == '{' || ch == '}' {
			return true
		}
	}
	return false
}