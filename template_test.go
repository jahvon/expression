package expression_test

import (
	"strings"
	"testing"

	"github.com/jahvon/expression"
)

func setupTestData() (expression.Data, *expression.Template) {
	data := expression.Data{
		"os":         "linux",
		"arch":       "amd64",
		"store":      map[string]interface{}{"key1": "value1", "key2": 2},
		"ctx":        map[string]interface{}{"workspace": "test_workspace", "namespace": "test_namespace"},
		"workspaces": []string{"test_workspace", "other_workspace"},
		"executables": []map[string]interface{}{
			{"name": "exec1", "tags": []string{"tag"}, "type": "serial"},
			{"name": "exec2", "tags": []string{}, "type": "exec"},
			{"name": "exec3", "tags": []string{"tag", "tag2"}, "type": "exec"},
		},
		"featureEnabled": true,
	}
	tmpl := expression.NewTemplate("test", data)
	return data, tmpl
}

func TestExprEvaluation(t *testing.T) {
	t.Run("evaluates simple expressions", func(t *testing.T) {
		_, tmpl := setupTestData()
		err := tmpl.Parse("{{ ctx.workspace }}")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		result, err := tmpl.ExecuteToString()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if result != "test_workspace" {
			t.Errorf("expected 'test_workspace', got '%s'", result)
		}
	})

	t.Run("evaluates boolean expressions", func(t *testing.T) {
		_, tmpl := setupTestData()
		err := tmpl.Parse("{{ os == \"linux\" && arch == \"amd64\" }}")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		result, err := tmpl.ExecuteToString()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if result != "true" {
			t.Errorf("expected 'true', got '%s'", result)
		}
	})

	t.Run("evaluates arithmetic expressions", func(t *testing.T) {
		_, tmpl := setupTestData()
		err := tmpl.Parse("{{ store[\"key2\"] * 2 }}")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		result, err := tmpl.ExecuteToString()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if result != "4" {
			t.Errorf("expected '4', got '%s'", result)
		}
	})
}

func TestControlStructures(t *testing.T) {
	t.Run("handles if/else with expr conditions", func(t *testing.T) {
		_, tmpl := setupTestData()
		template := `
			{{- if featureEnabled && ctx.workspace == "test_workspace" }}
			Matched
			{{- else }}
			Unmatched
			{{- end }}
		`
		err := tmpl.Parse(template)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		result, err := tmpl.ExecuteToString()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if strings.TrimSpace(result) != "Matched" {
			t.Errorf("expected 'Matched', got '%s'", strings.TrimSpace(result))
		}
	})

	t.Run("handles range with expr", func(t *testing.T) {
		_, tmpl := setupTestData()
		template := `
{{- range filter(executables, {.type == "exec"}) }}
{{ .name }}: {{ .tags }}
{{- end }}
		`
		err := tmpl.Parse(template)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		result, err := tmpl.ExecuteToString()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		expected := "exec2: []\nexec3: [tag tag2]"
		if strings.TrimSpace(result) != expected {
			t.Errorf("expected '%s', got '%s'", expected, strings.TrimSpace(result))
		}
	})

	t.Run("handles with using expr", func(t *testing.T) {
		_, tmpl := setupTestData()
		template := `
{{- with ctx }}
Workspace: {{ .workspace }}
Namespace: {{ .namespace }}
{{- end }}
		`
		err := tmpl.Parse(template)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		result, err := tmpl.ExecuteToString()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		expected := "Workspace: test_workspace\nNamespace: test_namespace"
		if strings.TrimSpace(result) != expected {
			t.Errorf("expected '%s', got '%s'", expected, strings.TrimSpace(result))
		}
	})

	t.Run("handles nested control structures with expr", func(t *testing.T) {
		t.Skip("nested control structures not supported yet")
		_, tmpl := setupTestData()
		template := `
{{- range executables }}
{{- $exec := . }}
{{- if len($exec.tags) > 0 }}
{{ .name }}: {{ .type }}
{{- end }}
{{- end }}
		`
		err := tmpl.Parse(template)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		result, err := tmpl.ExecuteToString()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		expected := "Item 1: 12.089 (with tax)\nItem 3: 16.5 (with tax)"
		if strings.TrimSpace(result) != expected {
			t.Errorf("expected '%s', got '%s'", expected, strings.TrimSpace(result))
		}
	})
}

func TestErrorHandling(t *testing.T) {
	t.Run("handles invalid expressions", func(t *testing.T) {
		_, tmpl := setupTestData()
		err := tmpl.Parse("{{ unknown.field }}")
		if err != nil {
			t.Fatalf("expected no parse error, got %v", err)
		}

		_, err = tmpl.ExecuteToString()
		if err == nil {
			t.Error("expected execution error, got nil")
		}
	})

	t.Run("handles invalid syntax in if conditions", func(t *testing.T) {
		_, tmpl := setupTestData()
		err := tmpl.Parse("{{ if 1 ++ \"2\" }}invalid{{end}}")
		if err != nil {
			t.Fatalf("expected no parse error, got %v", err)
		}

		_, err = tmpl.ExecuteToString()
		if err == nil {
			t.Error("expected execution error, got nil")
		}
	})
}

func TestTemplateWithTrimMarkers(t *testing.T) {
	t.Run("handles trim markers in range", func(t *testing.T) {
		_, tmpl := setupTestData()
		template := `start
{{- range workspaces }}
{{ . }}
{{- end }}
end`
		err := tmpl.Parse(template)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		result, err := tmpl.ExecuteToString()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		expected := "start\ntest_workspace\nother_workspace\nend"
		if result != expected {
			t.Errorf("expected '%s', got '%s'", expected, result)
		}
	})

	t.Run("handles trim markers in if/else", func(t *testing.T) {
		_, tmpl := setupTestData()
		template := `start
{{- if featureEnabled }}
enabled
{{- else }}
disabled
{{- end }}
end`
		err := tmpl.Parse(template)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		result, err := tmpl.ExecuteToString()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		expected := "start\nenabled\nend"
		if result != expected {
			t.Errorf("expected '%s', got '%s'", expected, result)
		}
	})
}