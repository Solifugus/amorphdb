package ari

import (
	"testing"
)

func TestParser_SimpleSection(t *testing.T) {
	input := `section bank_report:
    field report_date: right 10 up 1: date
    field bank_id: right 5 down 1: "ID"`

	lexer := NewLexer(input)
	parser := NewParser(lexer)
	spec := parser.ParseARISpec()

	if len(parser.Errors()) != 0 {
		t.Fatalf("parser errors: %v", parser.Errors())
	}

	if len(spec.Sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(spec.Sections))
	}

	section := spec.Sections[0]
	if section.Name != "bank_report" {
		t.Fatalf("expected section name 'bank_report', got '%s'", section.Name)
	}

	if len(section.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(section.Fields))
	}

	// Test first field
	field1 := section.Fields[0]
	if field1.Name != "report_date" {
		t.Fatalf("expected field name 'report_date', got '%s'", field1.Name)
	}

	if len(field1.Anchors) != 2 {
		t.Fatalf("expected 2 anchors for report_date, got %d", len(field1.Anchors))
	}

	// Test first anchor (right 10)
	anchor1 := field1.Anchors[0]
	if anchor1.Direction != DirectionRight {
		t.Fatalf("expected direction 'right', got %v", anchor1.Direction)
	}
	if anchor1.Distance.Type != DistanceExact || anchor1.Distance.Min != 10 {
		t.Fatalf("expected distance '10', got %v", anchor1.Distance)
	}

	// Test second anchor (up 1)
	anchor2 := field1.Anchors[1]
	if anchor2.Direction != DirectionUp {
		t.Fatalf("expected direction 'up', got %v", anchor2.Direction)
	}
	if anchor2.Distance.Type != DistanceExact || anchor2.Distance.Min != 1 {
		t.Fatalf("expected distance '1', got %v", anchor2.Distance)
	}

	// Test patterns
	if len(field1.Patterns) != 1 {
		t.Fatalf("expected 1 pattern for report_date, got %d", len(field1.Patterns))
	}
	if field1.Patterns[0].Type != PatternBuiltinDate {
		t.Fatalf("expected pattern type 'date', got %v", field1.Patterns[0].Type)
	}
}

func TestParser_SectionWithStartsEnds(t *testing.T) {
	input := `section transactions starts("==== transactions ====") ends("==== end of transactions ===="):
    field transaction_type: right 6 same: "TYPE"
    break`

	lexer := NewLexer(input)
	parser := NewParser(lexer)
	spec := parser.ParseARISpec()

	if len(parser.Errors()) != 0 {
		t.Fatalf("parser errors: %v", parser.Errors())
	}

	if len(spec.Sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(spec.Sections))
	}

	section := spec.Sections[0]
	if section.Name != "transactions" {
		t.Fatalf("expected section name 'transactions', got '%s'", section.Name)
	}

	if section.StartsWith == nil {
		t.Fatalf("expected starts pattern, got nil")
	}
	if section.StartsWith.Type != PatternText || section.StartsWith.Text != "==== transactions ====" {
		t.Fatalf("expected starts pattern '==== transactions ====', got %v", section.StartsWith)
	}

	if section.EndsWith == nil {
		t.Fatalf("expected ends pattern, got nil")
	}
	if section.EndsWith.Type != PatternText || section.EndsWith.Text != "==== end of transactions ====" {
		t.Fatalf("expected ends pattern '==== end of transactions ====', got %v", section.EndsWith)
	}

	if section.Break == nil {
		t.Fatalf("expected break rule, got nil")
	}
	if section.Break.Type != BreakOnReMatch {
		t.Fatalf("expected break on re-match, got %v", section.Break.Type)
	}
}

func TestParser_CustomType(t *testing.T) {
	input := `type DATE:
    /(\d{2})\/(\d{2})\/(\d{4})/ -> "$3-$1-$2"
    /(\d{4})-(\d{2})-(\d{2})/  -> "$1-$2-$3"
    output: ISO_DATE`

	lexer := NewLexer(input)
	parser := NewParser(lexer)
	spec := parser.ParseARISpec()

	if len(parser.Errors()) != 0 {
		t.Fatalf("parser errors: %v", parser.Errors())
	}

	if len(spec.CustomTypes) != 1 {
		t.Fatalf("expected 1 custom type, got %d", len(spec.CustomTypes))
	}

	customType := spec.CustomTypes[0]
	if customType.Name != "DATE" {
		t.Fatalf("expected custom type name 'DATE', got '%s'", customType.Name)
	}

	if len(customType.Patterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(customType.Patterns))
	}

	// Test first pattern
	pattern1 := customType.Patterns[0]
	if pattern1.Pattern.Type != PatternRegexWithTransform {
		t.Fatalf("expected regex with transform, got %v", pattern1.Pattern.Type)
	}
	if pattern1.Transform != "$3-$1-$2" {
		t.Fatalf("expected transform '$3-$1-$2', got '%s'", pattern1.Transform)
	}

	if customType.OutputType != "ISO_DATE" {
		t.Fatalf("expected output type 'ISO_DATE', got '%s'", customType.OutputType)
	}
}

func TestParser_DistanceSpecifications(t *testing.T) {
	input := `section test:
    field exact: right 5: "test"
    field range: right 2-10: "test"
    field open_min: right 3-: "test"
    field open_max: right -8: "test"
    field flush_right: right flush: "test"`

	lexer := NewLexer(input)
	parser := NewParser(lexer)
	spec := parser.ParseARISpec()

	if len(parser.Errors()) != 0 {
		t.Fatalf("parser errors: %v", parser.Errors())
	}

	section := spec.Sections[0]
	fields := section.Fields

	// Test exact distance
	if fields[0].Anchors[0].Distance.Type != DistanceExact || fields[0].Anchors[0].Distance.Min != 5 {
		t.Fatalf("exact distance wrong: %v", fields[0].Anchors[0].Distance)
	}

	// Test range distance
	if fields[1].Anchors[0].Distance.Type != DistanceRange ||
		fields[1].Anchors[0].Distance.Min != 2 ||
		fields[1].Anchors[0].Distance.Max != 10 {
		t.Fatalf("range distance wrong: %v", fields[1].Anchors[0].Distance)
	}

	// Test open minimum
	if fields[2].Anchors[0].Distance.Type != DistanceOpenMin ||
		fields[2].Anchors[0].Distance.Min != 3 ||
		fields[2].Anchors[0].Distance.Max != -1 {
		t.Fatalf("open min distance wrong: %v", fields[2].Anchors[0].Distance)
	}

	// Test open maximum
	if fields[3].Anchors[0].Distance.Type != DistanceOpenMax ||
		fields[3].Anchors[0].Distance.Min != 0 ||
		fields[3].Anchors[0].Distance.Max != 8 {
		t.Fatalf("open max distance wrong: %v", fields[3].Anchors[0].Distance)
	}

	// Test flush
	if fields[4].Anchors[0].Distance.Type != DistanceFlush {
		t.Fatalf("flush distance wrong: %v", fields[4].Anchors[0].Distance)
	}
}

func TestParser_MultiplePatterns(t *testing.T) {
	input := `section test:
    field multi: right 5: "text", /regex/, date, money`

	lexer := NewLexer(input)
	parser := NewParser(lexer)
	spec := parser.ParseARISpec()

	if len(parser.Errors()) != 0 {
		t.Fatalf("parser errors: %v", parser.Errors())
	}

	field := spec.Sections[0].Fields[0]
	if len(field.Patterns) != 4 {
		t.Fatalf("expected 4 patterns, got %d", len(field.Patterns))
	}

	// Test pattern types
	patterns := field.Patterns
	if patterns[0].Type != PatternText {
		t.Fatalf("expected text pattern, got %v", patterns[0].Type)
	}
	if patterns[1].Type != PatternRegex {
		t.Fatalf("expected regex pattern, got %v", patterns[1].Type)
	}
	if patterns[2].Type != PatternBuiltinDate {
		t.Fatalf("expected date pattern, got %v", patterns[2].Type)
	}
	if patterns[3].Type != PatternBuiltinMoney {
		t.Fatalf("expected money pattern, got %v", patterns[3].Type)
	}
}

func TestParser_BreakOnPattern(t *testing.T) {
	input := `section test:
    field item: right 5: "test"
    break on "---"`

	lexer := NewLexer(input)
	parser := NewParser(lexer)
	spec := parser.ParseARISpec()

	if len(parser.Errors()) != 0 {
		t.Fatalf("parser errors: %v", parser.Errors())
	}

	section := spec.Sections[0]
	if section.Break == nil {
		t.Fatalf("expected break rule, got nil")
	}

	if section.Break.Type != BreakOnPattern {
		t.Fatalf("expected break on pattern, got %v", section.Break.Type)
	}

	if section.Break.Pattern == nil {
		t.Fatalf("expected break pattern, got nil")
	}

	if section.Break.Pattern.Type != PatternText || section.Break.Pattern.Text != "---" {
		t.Fatalf("expected break pattern '---', got %v", section.Break.Pattern)
	}
}