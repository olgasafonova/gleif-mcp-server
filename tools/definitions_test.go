package tools

import (
	"strings"
	"testing"
)

func TestAllToolsNotEmpty(t *testing.T) {
	if len(AllTools) == 0 {
		t.Error("AllTools should not be empty")
	}
}

func TestAllToolsHaveRequiredFields(t *testing.T) {
	for i, tool := range AllTools {
		t.Run(tool.Name, func(t *testing.T) {
			if tool.Name == "" {
				t.Errorf("tool[%d]: Name is required", i)
			}
			if tool.Title == "" {
				t.Errorf("tool[%d] %s: Title is required", i, tool.Name)
			}
			if tool.Description == "" {
				t.Errorf("tool[%d] %s: Description is required", i, tool.Name)
			}
			if tool.Category == "" {
				t.Errorf("tool[%d] %s: Category is required", i, tool.Name)
			}
		})
	}
}

func TestToolCategories(t *testing.T) {
	validCategories := map[string]bool{
		"lookup":     true,
		"validation": true,
		"search":     true,
		"ownership":  true,
		"utility":    true,
		"issuers":    true,
		"compliance": true,
	}

	for _, tool := range AllTools {
		t.Run(tool.Name, func(t *testing.T) {
			if !validCategories[tool.Category] {
				t.Errorf("tool %q has unknown category %q", tool.Name, tool.Category)
			}
		})
	}
}

func TestAllToolsReadOnly(t *testing.T) {
	for _, tool := range AllTools {
		t.Run(tool.Name, func(t *testing.T) {
			if !tool.ReadOnly {
				t.Errorf("tool %q should be ReadOnly (GLEIF is a read-only API)", tool.Name)
			}
		})
	}
}

func TestToolCount(t *testing.T) {
	expectedCount := 12
	if len(AllTools) != expectedCount {
		t.Errorf("expected %d tools, got %d", expectedCount, len(AllTools))
	}
}

func TestToolNamesUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, tool := range AllTools {
		if seen[tool.Name] {
			t.Errorf("duplicate tool name: %q", tool.Name)
		}
		seen[tool.Name] = true
	}
}

func TestToolDescriptionsHaveUseWhen(t *testing.T) {
	for _, tool := range AllTools {
		t.Run(tool.Name, func(t *testing.T) {
			if !strings.Contains(tool.Description, "USE WHEN") {
				t.Errorf("tool %q description missing USE WHEN clause", tool.Name)
			}
		})
	}
}

func TestParametersHaveRequiredFields(t *testing.T) {
	for _, tool := range AllTools {
		for j, param := range tool.Parameters {
			t.Run(tool.Name+"/"+param.Name, func(t *testing.T) {
				if param.Name == "" {
					t.Errorf("tool %q param[%d]: Name is required", tool.Name, j)
				}
				if param.Type == "" {
					t.Errorf("tool %q param %q: Type is required", tool.Name, param.Name)
				}
				if param.Description == "" {
					t.Errorf("tool %q param %q: Description is required", tool.Name, param.Name)
				}
			})
		}
	}
}

func TestParameterTypesValid(t *testing.T) {
	validTypes := map[string]bool{
		"string":  true,
		"integer": true,
		"boolean": true,
		"number":  true,
		"array":   true,
	}

	for _, tool := range AllTools {
		for _, param := range tool.Parameters {
			t.Run(tool.Name+"/"+param.Name, func(t *testing.T) {
				if !validTypes[param.Type] {
					t.Errorf("tool %q param %q has invalid type %q", tool.Name, param.Name, param.Type)
				}
			})
		}
	}
}

func TestGetToolByNameDefinitions(t *testing.T) {
	tool := GetToolByName("lei_lookup")
	if tool == nil {
		t.Fatal("GetToolByName(\"lei_lookup\") returned nil")
	}
	if tool.Name != "lei_lookup" {
		t.Errorf("expected name lei_lookup, got %q", tool.Name)
	}

	if GetToolByName("nonexistent_tool") != nil {
		t.Error("GetToolByName should return nil for unknown tools")
	}
}
