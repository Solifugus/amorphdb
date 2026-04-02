package ari

import "fmt"

// ASTNode is the base interface for all AST nodes
type ASTNode interface {
	String() string
}

// ARISpec is the root of the AST representing a complete ARI specification
type ARISpec struct {
	CustomTypes []CustomType
	Sections    []Section
}

func (as ARISpec) String() string {
	return fmt.Sprintf("ARISpec{CustomTypes: %v, Sections: %v}", as.CustomTypes, as.Sections)
}

// Section represents a section definition in ARI
type Section struct {
	Name       string
	StartsWith *Pattern // optional starts("pattern")
	EndsWith   *Pattern // optional ends("pattern")
	Fields     []Field
	Sections   []Section // nested sections
	Break      *Break    // optional break rule
}

func (s Section) String() string {
	return fmt.Sprintf("Section{Name: %s, StartsWith: %v, EndsWith: %v, Fields: %v, Sections: %v, Break: %v}",
		s.Name, s.StartsWith, s.EndsWith, s.Fields, s.Sections, s.Break)
}

// Field represents a field definition within a section
type Field struct {
	Name     string
	Anchors  []Anchor
	Patterns []Pattern
}

func (f Field) String() string {
	return fmt.Sprintf("Field{Name: %s, Anchors: %v, Patterns: %v}", f.Name, f.Anchors, f.Patterns)
}

// Anchor represents a spatial relationship descriptor
type Anchor struct {
	Direction Direction
	Distance  Distance
	Pattern   Pattern
}

func (a Anchor) String() string {
	return fmt.Sprintf("Anchor{Direction: %v, Distance: %v, Pattern: %v}", a.Direction, a.Distance, a.Pattern)
}

// Direction represents spatial directions
type Direction int

const (
	DirectionLeft Direction = iota
	DirectionRight
	DirectionUp
	DirectionDown
	DirectionSame
	DirectionFlush
)

func (d Direction) String() string {
	switch d {
	case DirectionLeft:
		return "left"
	case DirectionRight:
		return "right"
	case DirectionUp:
		return "up"
	case DirectionDown:
		return "down"
	case DirectionSame:
		return "same"
	case DirectionFlush:
		return "flush"
	default:
		return "unknown"
	}
}

// Distance represents distance specifications
type Distance struct {
	Type DistanceType
	Min  int
	Max  int // -1 for open-ended
}

type DistanceType int

const (
	DistanceExact DistanceType = iota // 5
	DistanceRange                     // 2-10
	DistanceOpenMin                   // 3-
	DistanceOpenMax                   // -8
	DistanceFlush                     // flush
)

func (d Distance) String() string {
	switch d.Type {
	case DistanceExact:
		return fmt.Sprintf("%d", d.Min)
	case DistanceRange:
		return fmt.Sprintf("%d-%d", d.Min, d.Max)
	case DistanceOpenMin:
		return fmt.Sprintf("%d-", d.Min)
	case DistanceOpenMax:
		return fmt.Sprintf("-%d", d.Max)
	case DistanceFlush:
		return "flush"
	default:
		return "unknown"
	}
}

// Pattern represents different types of pattern matching
type Pattern struct {
	Type PatternType
	Text string // for text literals, regex patterns, etc.
}

type PatternType int

const (
	PatternText PatternType = iota // "literal text"
	PatternRegex                   // /regex/
	PatternRegexWithTransform      // /regex/replacement/
	PatternBuiltinDate
	PatternBuiltinMoney
	PatternBuiltinInteger
	PatternBuiltinDecimal
	PatternBuiltinSSN
	PatternBuiltinISODate
	PatternCustomType // reference to a custom type
)

func (p Pattern) String() string {
	switch p.Type {
	case PatternText:
		return fmt.Sprintf(`"%s"`, p.Text)
	case PatternRegex:
		return fmt.Sprintf(`/%s/`, p.Text)
	case PatternRegexWithTransform:
		return fmt.Sprintf(`%s`, p.Text) // already includes /pattern/replacement/
	case PatternBuiltinDate:
		return "date"
	case PatternBuiltinMoney:
		return "money"
	case PatternBuiltinInteger:
		return "integer"
	case PatternBuiltinDecimal:
		return "decimal"
	case PatternBuiltinSSN:
		return "ssn"
	case PatternBuiltinISODate:
		return "ISO_DATE"
	case PatternCustomType:
		return p.Text
	default:
		return "unknown"
	}
}

// Break represents break rules for repeating records
type Break struct {
	Type BreakType
	Pattern *Pattern // optional pattern for explicit breaks
}

type BreakType int

const (
	BreakOnReMatch BreakType = iota // break (re-match of first field)
	BreakOnPattern                  // break on "pattern" or break on /regex/
)

func (b Break) String() string {
	switch b.Type {
	case BreakOnReMatch:
		return "break"
	case BreakOnPattern:
		if b.Pattern != nil {
			return fmt.Sprintf("break on %v", b.Pattern)
		}
		return "break on <nil>"
	default:
		return "unknown break"
	}
}

// CustomType represents a custom type definition
type CustomType struct {
	Name         string
	Patterns     []CustomPattern
	OutputType   string
}

func (ct CustomType) String() string {
	return fmt.Sprintf("CustomType{Name: %s, Patterns: %v, OutputType: %s}",
		ct.Name, ct.Patterns, ct.OutputType)
}

// CustomPattern represents a pattern mapping within a custom type
type CustomPattern struct {
	Pattern     Pattern
	Transform   string // transformation/formatting rule
	OutputType  string // optional type specification
	Modifiers   []string // e.g., "strip", "negate"
}

func (cp CustomPattern) String() string {
	return fmt.Sprintf("CustomPattern{Pattern: %v, Transform: %s, OutputType: %s, Modifiers: %v}",
		cp.Pattern, cp.Transform, cp.OutputType, cp.Modifiers)
}