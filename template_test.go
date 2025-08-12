package expression_test

import (
	"strings"
	"testing"

	"github.com/jahvon/expression"
)

func setupTestData() (expression.Data, *expression.Template) {
	data := map[string]interface{}{
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

	t.Run("handles dot in if condition inside range", func(t *testing.T) {
		data := map[string]interface{}{
			"items": []bool{true, false, true},
		}
		tmpl := expression.NewTemplate("test", data)
		template := `
{{- range items }}
{{- if . }}
yes
{{- else }}
no
{{- end }}
{{- end }}`
		err := tmpl.Parse(template)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		result, err := tmpl.ExecuteToString()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		expected := "yes\nno\nyes"
		if strings.TrimSpace(result) != expected {
			t.Errorf("expected '%s', got '%s'", expected, strings.TrimSpace(result))
		}
	})

	t.Run("handles dot in if condition inside with", func(t *testing.T) {
		data := map[string]interface{}{
			"flag": true,
		}
		tmpl := expression.NewTemplate("test", data)
		template := `
{{- with flag }}
{{- if . }}
enabled
{{- else }}
disabled
{{- end }}
{{- end }}`
		err := tmpl.Parse(template)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		result, err := tmpl.ExecuteToString()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		expected := "enabled"
		if strings.TrimSpace(result) != expected {
			t.Errorf("expected '%s', got '%s'", expected, strings.TrimSpace(result))
		}
	})

	t.Run("handles complex if conditions with dot fields inside range", func(t *testing.T) {
		data := map[string]interface{}{
			"items": []map[string]interface{}{
				{"name": "item1", "active": true},
				{"name": "item2", "active": false},
				{"name": "item3", "active": true},
			},
		}
		tmpl := expression.NewTemplate("test", data)
		template := `
{{- range items }}
{{- if .active }}
{{ .name }}: active
{{- else }}
{{ .name }}: inactive
{{- end }}
{{- end }}`
		err := tmpl.Parse(template)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		result, err := tmpl.ExecuteToString()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		expected := "item1: active\nitem2: inactive\nitem3: active"
		if strings.TrimSpace(result) != expected {
			t.Errorf("expected '%s', got '%s'", expected, strings.TrimSpace(result))
		}
	})

	t.Run("handles variable assignment with dot", func(t *testing.T) {
		data := map[string]interface{}{
			"message": "Hello World",
		}
		tmpl := expression.NewTemplate("test", data)
		template := `
{{- with message }}
{{- $msg := . }}
Message: {{ $msg }}
{{- end }}`
		err := tmpl.Parse(template)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		result, err := tmpl.ExecuteToString()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		expected := "Message: Hello World"
		if strings.TrimSpace(result) != expected {
			t.Errorf("expected '%s', got '%s'", expected, strings.TrimSpace(result))
		}
	})

	t.Run("handles variable assignment with len function", func(t *testing.T) {
		data := map[string]interface{}{
			"tasks": []string{"task1", "task2", "task3"},
		}
		tmpl := expression.NewTemplate("test", data)
		template := `
{{- $taskCount := len(tasks) }}
Total tasks: {{ $taskCount }}`
		err := tmpl.Parse(template)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		result, err := tmpl.ExecuteToString()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		expected := "Total tasks: 3"
		if strings.TrimSpace(result) != expected {
			t.Errorf("expected '%s', got '%s'", expected, strings.TrimSpace(result))
		}
	})

	t.Run("handles range with index and value variables", func(t *testing.T) {
		data := map[string]interface{}{
			"items": []string{"apple", "banana", "cherry"},
		}
		tmpl := expression.NewTemplate("test", data)
		template := `
{{- range $index, $item := items }}
{{ $index }}: {{ $item }}
{{- end }}`
		err := tmpl.Parse(template)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		result, err := tmpl.ExecuteToString()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		expected := "0: apple\n1: banana\n2: cherry"
		if strings.TrimSpace(result) != expected {
			t.Errorf("expected '%s', got '%s'", expected, strings.TrimSpace(result))
		}
	})

	t.Run("handles range with complex data and field access", func(t *testing.T) {
		data := map[string]interface{}{
			"data": []map[string]interface{}{
				{"content": "Task 1", "description": "First task"},
				{"content": "Task 2", "description": ""},
				{"content": "Task 3", "description": "Third task"},
			},
		}
		tmpl := expression.NewTemplate("test", data)
		template := `
{{- range $index, $task := data }}
## □ {{ $task.content }}

{{- if $task.description }}
*{{ $task.description }}*
{{- end }}
{{- end }}`
		err := tmpl.Parse(template)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		result, err := tmpl.ExecuteToString()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		expected := "## □ Task 1\n*First task*\n## □ Task 2\n## □ Task 3\n*Third task*"
		if strings.TrimSpace(result) != expected {
			t.Errorf("expected '%s', got '%s'", expected, strings.TrimSpace(result))
		}
	})

	t.Run("handles simple function call with variables", func(t *testing.T) {
		data := map[string]interface{}{
			"tasks": []map[string]interface{}{
				{"content": "Task 1", "priority": "4"},
			},
		}
		tmpl := expression.NewTemplate("test", data)
		template := `
{{- range $index, $task := tasks }}
- **{{ $task.content }}**: {{ if $task.priority }}Has priority{{ else }}No priority{{ end }}
{{- end }}`
		err := tmpl.Parse(template)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		result, err := tmpl.ExecuteToString()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		expected := "- **Task 1**: Has priority"
		if strings.TrimSpace(result) != expected {
			t.Errorf("expected '%s', got '%s'", expected, strings.TrimSpace(result))
		}
	})

	t.Run("handles variables in expr expressions with priority logic", func(t *testing.T) {
		data := map[string]interface{}{
			"tasks": []map[string]interface{}{
				{"content": "Task 1", "priority": "4"},
				{"content": "Task 2", "priority": "3"},
				{"content": "Task 3", "priority": "2"},
				{"content": "Task 4", "priority": "1"},
			},
		}
		tmpl := expression.NewTemplate("test", data)
		template := `
{{- range $index, $task := tasks }}
- **{{ $task.content }}**: {{ if eq $task.priority "4" }}🔴 High{{ else if eq $task.priority "3" }}🟡 Medium{{ else if eq $task.priority "2" }}🔵 Low{{ else }}⚪ None{{ end }}
{{- end }}`
		err := tmpl.Parse(template)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		result, err := tmpl.ExecuteToString()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		expected := "- **Task 1**: 🔴 High\n- **Task 2**: 🟡 Medium\n- **Task 3**: 🔵 Low\n- **Task 4**: ⚪ None"
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
