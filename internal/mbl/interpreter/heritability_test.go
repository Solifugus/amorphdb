package interpreter

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

func TestHeritability_AllFourTypes(t *testing.T) {
	// Test 1: Setup template with all four heritability types
	tree := storage.NewMemoryTree()
	interpreter := New(tree, 1) // agent 1

	// Setup template with different heritability modifiers
	setupStatements := []string{
		`my.template.name:(copy) = "default_name"`,
		`my.template.rate:(link) = 0.05`,
		`my.template.balance:(reset) = 1000`,
		`my.template.balance.@default = 0`, // Set the @default value for reset
		`my.template.internal:(exclude) = "secret"`,
	}

	for _, stmt := range setupStatements {
		_, err := interpreter.EvaluateExpression(stmt)
		if err != nil {
			t.Fatalf("Failed to setup template: %s, error: %v", stmt, err)
		}
	}

	// Flush to ensure values are persisted
	interpreter.flushBuffer()

	// Create instance using new()
	createInstance := `instance = new(my.template)`
	_, err := interpreter.EvaluateExpression(createInstance)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}

	// Verify instance.name exists and equals "default_name" (copy)
	nameResult, err := interpreter.EvaluateExpression(`instance.name`)
	if err != nil {
		t.Fatalf("Failed to get instance.name: %v", err)
	}
	if textValue, ok := nameResult.(types.Text); ok {
		if textValue.Value != "default_name" {
			t.Errorf("Expected instance.name='default_name', got '%s'", textValue.Value)
		}
	} else {
		t.Errorf("Expected instance.name to be Text('default_name'), got %T: %v", nameResult, nameResult)
	}

	// Verify instance.balance equals 0 (reset to @default), NOT 1000
	balanceResult, err := interpreter.EvaluateExpression(`instance.balance`)
	if err != nil {
		t.Fatalf("Failed to get instance.balance: %v", err)
	}
	if numValue, ok := balanceResult.(types.Number); ok {
		if numValue.Value != 0 {
			t.Errorf("Expected instance.balance=0 (reset to @default), got %f", numValue.Value)
		}
	} else {
		t.Errorf("Expected instance.balance to be Number(0), got %T: %v", balanceResult, balanceResult)
	}

	// Verify instance.internal does NOT exist (excluded)
	internalResult, err := interpreter.EvaluateExpression(`instance.internal`)
	if err != nil {
		t.Fatalf("Parse error for instance.internal: %v", err)
	}
	if unknown, ok := internalResult.(types.Unknown); ok {
		// This is expected - excluded fields should not exist
		t.Logf("Expected unknown result for excluded field: %v", unknown)
	} else {
		t.Errorf("Expected instance.internal to be Unknown (excluded), got %T: %v", internalResult, internalResult)
	}

	// Verify instance.rate exists (link) - just check it resolves for now
	rateResult, err := interpreter.EvaluateExpression(`instance.rate`)
	if err != nil {
		t.Fatalf("Failed to get instance.rate: %v", err)
	}
	if numValue, ok := rateResult.(types.Number); ok {
		if numValue.Value != 0.05 {
			t.Errorf("Expected instance.rate=0.05 (linked), got %f", numValue.Value)
		}
	} else {
		t.Errorf("Expected instance.rate to be Number(0.05), got %T: %v", rateResult, rateResult)
	}
}

func TestHeritability_CopyIndependence(t *testing.T) {
	// Test 2: Copy independence
	tree := storage.NewMemoryTree()
	interpreter := New(tree, 1) // agent 1

	// Setup template with copy modifier
	setupStatements := []string{
		`my.template.name:(copy) = "default_name"`,
	}

	for _, stmt := range setupStatements {
		_, err := interpreter.EvaluateExpression(stmt)
		if err != nil {
			t.Fatalf("Failed to setup template: %s, error: %v", stmt, err)
		}
	}

	// Flush to ensure values are persisted
	interpreter.flushBuffer()

	// Create instance using new()
	createInstance := `instance = new(my.template)`
	_, err := interpreter.EvaluateExpression(createInstance)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}

	// Change the template's name after instance creation
	changeTemplate := `my.template.name = "changed"`
	_, err = interpreter.EvaluateExpression(changeTemplate)
	if err != nil {
		t.Fatalf("Failed to change template name: %v", err)
	}

	// Verify instance.name is still "default_name" (independent copy)
	nameResult, err := interpreter.EvaluateExpression(`instance.name`)
	if err != nil {
		t.Fatalf("Failed to get instance.name: %v", err)
	}
	if textValue, ok := nameResult.(types.Text); ok {
		if textValue.Value != "default_name" {
			t.Errorf("Expected instance.name to remain 'default_name' (independent copy), got '%s'", textValue.Value)
		}
	} else {
		t.Errorf("Expected instance.name to be Text('default_name'), got %T: %v", nameResult, nameResult)
	}

	// Verify template.name actually changed
	templateNameResult, err := interpreter.EvaluateExpression(`my.template.name`)
	if err != nil {
		t.Fatalf("Failed to get template name: %v", err)
	}
	if textValue, ok := templateNameResult.(types.Text); ok {
		if textValue.Value != "changed" {
			t.Errorf("Expected template name to be 'changed', got '%s'", textValue.Value)
		}
	} else {
		t.Errorf("Expected template name to be Text('changed'), got %T: %v", templateNameResult, templateNameResult)
	}
}

func TestHeritability_LinkSharing(t *testing.T) {
	// Test 3: Link sharing
	tree := storage.NewMemoryTree()
	interpreter := New(tree, 1) // agent 1

	// Setup template with link modifier
	setupStatements := []string{
		`my.template.rate:(link) = 0.05`,
	}

	for _, stmt := range setupStatements {
		_, err := interpreter.EvaluateExpression(stmt)
		if err != nil {
			t.Fatalf("Failed to setup template: %s, error: %v", stmt, err)
		}
	}

	// Flush to ensure values are persisted
	interpreter.flushBuffer()

	// Create instance using new()
	createInstance := `instance = new(my.template)`
	_, err := interpreter.EvaluateExpression(createInstance)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}

	// Change the template's rate after instance creation
	changeTemplate := `my.template.rate = 0.10`
	_, err = interpreter.EvaluateExpression(changeTemplate)
	if err != nil {
		t.Fatalf("Failed to change template rate: %v", err)
	}

	// Flush to ensure the change is persisted
	interpreter.flushBuffer()

	// Verify instance.rate is now 0.10 (shared via link)
	rateResult, err := interpreter.EvaluateExpression(`instance.rate`)
	if err != nil {
		t.Fatalf("Failed to get instance.rate: %v", err)
	}
	if numValue, ok := rateResult.(types.Number); ok {
		if numValue.Value != 0.10 {
			t.Errorf("Expected instance.rate to be 0.10 (shared via link), got %f", numValue.Value)
		}
	} else {
		t.Errorf("Expected instance.rate to be Number(0.10), got %T: %v", rateResult, rateResult)
	}
}

func TestHeritability_MetaAttributeReadable(t *testing.T) {
	// Test 4: Verify @heritability meta attribute is readable
	tree := storage.NewMemoryTree()
	interpreter := New(tree, 1) // agent 1

	// Setup template with different heritability modifiers
	setupStatements := []string{
		`my.template.name:(copy) = "default_name"`,
		`my.template.rate:(link) = 0.05`,
		`my.template.balance:(reset) = 1000`,
		`my.template.balance.@default = 0`, // Set the @default value for reset
		`my.template.internal:(exclude) = "secret"`,
	}

	for _, stmt := range setupStatements {
		_, err := interpreter.EvaluateExpression(stmt)
		if err != nil {
			t.Fatalf("Failed to setup template: %s, error: %v", stmt, err)
		}
	}

	// Flush to ensure values are persisted
	interpreter.flushBuffer()

	// First, let's test that the basic fields are stored correctly
	nameResult, err := interpreter.EvaluateExpression(`my.template.name`)
	if err != nil {
		t.Fatalf("Failed to get my.template.name: %v", err)
	}
	t.Logf("my.template.name = %T: %v", nameResult, nameResult)

	// Verify @heritability meta attributes
	tests := []struct {
		path     string
		expected string
	}{
		{`my.template.name.@heritability`, "copy"},
		{`my.template.rate.@heritability`, "link"},
		{`my.template.balance.@heritability`, "reset"},
		{`my.template.internal.@heritability`, "exclude"},
	}

	for _, test := range tests {
		result, err := interpreter.EvaluateExpression(test.path)
		if err != nil {
			t.Fatalf("Failed to get %s: %v", test.path, err)
		}
		if textValue, ok := result.(types.Text); ok {
			if textValue.Value != test.expected {
				t.Errorf("Expected %s='%s', got '%s'", test.path, test.expected, textValue.Value)
			}
		} else {
			t.Errorf("Expected %s to be Text('%s'), got %T: %v", test.path, test.expected, result, result)
		}
	}

	// Verify @default meta attribute for reset field
	defaultResult, err := interpreter.EvaluateExpression(`my.template.balance.@default`)
	if err != nil {
		t.Fatalf("Failed to get my.template.balance.@default: %v", err)
	}
	if numValue, ok := defaultResult.(types.Number); ok {
		if numValue.Value != 0 {
			t.Errorf("Expected my.template.balance.@default=0, got %f", numValue.Value)
		}
	} else {
		t.Errorf("Expected my.template.balance.@default to be Number(0), got %T: %v", defaultResult, defaultResult)
	}
}