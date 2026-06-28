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

// formatResultWithHint formats any MBL value for user display with a specific hint
func (r *REPL) formatResultWithHint(result interface{}, hint DisplayHint) string {
	if result == nil {
		return ""
	}

	switch v := result.(type) {
	case types.Nothing:
		return "" // Nothing produces no output
	case types.Text:
		return r.formatScalarValue(v.Value, "", 0, hint) // TODO: timestamp and author need storage layer enhancement
	case types.Number:
		return r.formatScalarValue(r.formatNumber(v.Value), "", 0, hint)
	case types.Boolean:
		return r.formatScalarValue(r.formatBoolean(v.Value), "", 0, hint)
	case types.Time:
		return r.formatScalarValue(r.formatTime(v.Timestamp, int(v.Precision)), "", 0, hint)
	case types.Money:
		return r.formatScalarValue(r.formatMoney(v.Amount, v.CurrencyCode), "", 0, hint)
	case types.List:
		return r.formatListWithHint(v, hint)
	case types.Record:
		return r.formatRecordWithHint(v, hint)
	case types.Reference:
		return r.formatScalarValue(r.formatReference(v.AttributeID), "", 0, hint)
	case types.Unknown:
		return r.formatScalarValue(r.formatUnknown(v.Reason), "", 0, hint)
	case string:
		// Raw string result (e.g., from output function)
		return v
	default:
		// Fallback for unexpected types
		return fmt.Sprintf("%v", result)
	}
}

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

// formatScalarValue formats scalar values with timestamp and author context
func (r *REPL) formatScalarValue(value string, agent string, timestamp int64, hint DisplayHint) string {
	// TODO: Storage layer enhancement needed to provide timestamp and author metadata
	// For now, format without temporal context until storage interface is extended
	if hint == DisplayTree || hint == DisplayList {
		return value // Tree and list views show raw values
	}

	// TODO: When storage provides metadata, format as:
	// return fmt.Sprintf("%s  (@%s by %s)", value, formatTimestamp(timestamp), agent)
	return value
}

// formatListWithHint formats MBL lists with display hint
func (r *REPL) formatListWithHint(list types.List, hint DisplayHint) string {
	if len(list.Elements) == 0 {
		return "[]"
	}

	switch hint {
	case DisplayTable:
		return r.formatListAsTable(list)
	case DisplayTree:
		return r.formatListAsTree(list)
	case DisplayList:
		return r.formatListAsList(list)
	case DisplayAuto:
		// Auto-infer: if homogeneous records, use table; for simple values use compact format
		if r.isHomogeneousRecordList(list) {
			return r.formatListAsTable(list)
		}
		// For simple value lists (numbers, strings, etc), use compact format
		return r.formatList(list)
	default:
		return r.formatList(list) // Fallback to original format
	}
}

// formatRecordWithHint formats MBL records with display hint
func (r *REPL) formatRecordWithHint(record types.Record, hint DisplayHint) string {
	if len(record.Fields) == 0 {
		return "{}"
	}

	switch hint {
	case DisplayTable:
		// Single record as table (transpose: fields as rows)
		return r.formatRecordAsTable(record)
	case DisplayTree:
		return r.formatRecordAsTree(record)
	case DisplayList:
		return r.formatRecordAsList(record)
	case DisplayAuto:
		// Auto-infer: use key-value block format (spec requirement)
		return r.formatRecordAsKeyValue(record)
	default:
		return r.formatRecord(record) // Fallback to original format
	}
}

// isHomogeneousRecordList checks if all list elements are records with similar field names
func (r *REPL) isHomogeneousRecordList(list types.List) bool {
	if len(list.Elements) < 2 {
		return false
	}

	// Check if first element is a record
	firstRecord, ok := list.Elements[0].(types.Record)
	if !ok {
		return false
	}

	// Check if all elements are records with similar fields
	for i := 1; i < len(list.Elements); i++ {
		record, ok := list.Elements[i].(types.Record)
		if !ok {
			return false
		}

		// For auto-inference, require at least 50% field overlap
		if r.fieldSimilarity(firstRecord, record) < 0.5 {
			return false
		}
	}

	return true
}

// fieldSimilarity calculates field overlap between two records (0.0 to 1.0)
func (r *REPL) fieldSimilarity(record1, record2 types.Record) float64 {
	if len(record1.Fields) == 0 && len(record2.Fields) == 0 {
		return 1.0
	}

	commonFields := 0
	totalFields := len(record1.Fields)
	if len(record2.Fields) > totalFields {
		totalFields = len(record2.Fields)
	}

	for field := range record1.Fields {
		if _, exists := record2.Fields[field]; exists {
			commonFields++
		}
	}

	if totalFields == 0 {
		return 1.0
	}

	return float64(commonFields) / float64(totalFields)
}

// formatListAsTable formats a homogeneous record list as a table
func (r *REPL) formatListAsTable(list types.List) string {
	if len(list.Elements) == 0 {
		return "[]"
	}

	// Gather all unique field names from all records
	allFields := make(map[string]bool)
	for _, elem := range list.Elements {
		if record, ok := elem.(types.Record); ok {
			for field := range record.Fields {
				allFields[field] = true
			}
		}
	}

	// Convert to sorted slice for consistent ordering
	var fieldNames []string
	for field := range allFields {
		fieldNames = append(fieldNames, field)
	}

	if len(fieldNames) == 0 {
		return r.formatList(list) // Fall back to normal list format
	}

	// Build table header
	var builder strings.Builder
	builder.WriteString("  #   ")
	for _, field := range fieldNames {
		builder.WriteString(fmt.Sprintf("%-12s ", field))
	}
	builder.WriteString("\n")

	// Build table rows
	for i, elem := range list.Elements {
		builder.WriteString(fmt.Sprintf("  %-3d ", i))
		if record, ok := elem.(types.Record); ok {
			for _, field := range fieldNames {
				if value, exists := record.Fields[field]; exists {
					formattedValue := r.formatResult(value)
					// Truncate long values for table display
					if len(formattedValue) > 10 {
						formattedValue = formattedValue[:10] + "..."
					}
					builder.WriteString(fmt.Sprintf("%-12s ", formattedValue))
				} else {
					builder.WriteString(fmt.Sprintf("%-12s ", "")) // Empty cell for missing field
				}
			}
		} else {
			// Non-record element in list
			formattedValue := r.formatResult(elem)
			if len(formattedValue) > 10 {
				formattedValue = formattedValue[:10] + "..."
			}
			builder.WriteString(fmt.Sprintf("%-12s ", formattedValue))
			// Fill remaining columns with empty
			for j := 1; j < len(fieldNames); j++ {
				builder.WriteString(fmt.Sprintf("%-12s ", ""))
			}
		}
		builder.WriteString("\n")
	}

	return strings.TrimSpace(builder.String())
}

// formatListAsTree formats a list as an indented tree
func (r *REPL) formatListAsTree(list types.List) string {
	var builder strings.Builder
	for i, elem := range list.Elements {
		builder.WriteString(fmt.Sprintf("[%d]\n", i))
		elemStr := r.formatResult(elem)
		// Indent each line of the element
		lines := strings.Split(elemStr, "\n")
		for _, line := range lines {
			builder.WriteString(fmt.Sprintf("    %s\n", line))
		}
	}
	return strings.TrimSpace(builder.String())
}

// formatListAsList formats a list as one item per line
func (r *REPL) formatListAsList(list types.List) string {
	var builder strings.Builder
	for i, elem := range list.Elements {
		builder.WriteString(fmt.Sprintf("[%d] %s\n", i, r.formatResult(elem)))
	}
	return strings.TrimSpace(builder.String())
}

// formatRecordAsTable formats a single record as a table (fields as rows)
func (r *REPL) formatRecordAsTable(record types.Record) string {
	var builder strings.Builder
	builder.WriteString("field        value\n")
	builder.WriteString("------------ --------\n")
	for key, value := range record.Fields {
		formattedValue := r.formatResult(value)
		builder.WriteString(fmt.Sprintf("%-12s %s\n", key, formattedValue))
	}
	return strings.TrimSpace(builder.String())
}

// formatRecordAsTree formats a record as an indented tree
func (r *REPL) formatRecordAsTree(record types.Record) string {
	var builder strings.Builder
	for key, value := range record.Fields {
		builder.WriteString(fmt.Sprintf("%s\n", key))
		valueStr := r.formatResult(value)
		// Indent each line of the value
		lines := strings.Split(valueStr, "\n")
		for _, line := range lines {
			builder.WriteString(fmt.Sprintf("    %s\n", line))
		}
	}
	return strings.TrimSpace(builder.String())
}

// formatRecordAsList formats a record as one field per line
func (r *REPL) formatRecordAsList(record types.Record) string {
	var builder strings.Builder
	for key, value := range record.Fields {
		builder.WriteString(fmt.Sprintf("%s: %s\n", key, r.formatResult(value)))
	}
	return strings.TrimSpace(builder.String())
}

// formatRecordAsKeyValue formats a record as key-value block (spec default for records)
func (r *REPL) formatRecordAsKeyValue(record types.Record) string {
	var builder strings.Builder

	// TODO: If record has its own scalar value, show it first as "value: <val>"
	// This requires storage layer enhancement to distinguish record value from fields

	// Show fields as key-value pairs
	for key, value := range record.Fields {
		formattedValue := r.formatResult(value)
		builder.WriteString(fmt.Sprintf("%-12s %s\n", key+":", formattedValue))
	}

	return strings.TrimSpace(builder.String())
}