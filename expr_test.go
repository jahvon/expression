package expression_test

import (
	"context"
	"testing"

	"github.com/jahvon/expression"
)

func TestIsTruthy(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		env      expression.Data
		expected bool
	}{
		{"true literal", "true", nil, true},
		{"false literal", "false", nil, false},
		{"numeric 1", "1", nil, true},
		{"numeric 0", "0", nil, false},
		{"string true", `"true"`, nil, true},
		{"string false", `"false"`, nil, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := expression.IsTruthy(test.expr, test.env)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if result != test.expected {
				t.Errorf("expected %v, got %v", test.expected, result)
			}
		})
	}
}

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		env      expression.Data
		expected interface{}
	}{
		{"addition", "1 + 1", nil, 2},
		{"boolean and", "true && false", nil, false},
		{"string concatenation", `"hello" + " " + "world"`, nil, "hello world"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := expression.Evaluate(test.expr, test.env)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if result != test.expected {
				t.Errorf("expected %v, got %v", test.expected, result)
			}
		})
	}
}

func TestEvaluateString(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		env      expression.Data
		expected string
	}{
		{"string literal", `"hello"`, nil, "hello"},
		{"string concatenation", `"foo" + "bar"`, nil, "foobar"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := expression.EvaluateString(test.expr, test.env)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if result != test.expected {
				t.Errorf("expected %s, got %s", test.expected, result)
			}
		})
	}
}

func TestDataComplexExpressions(t *testing.T) {
	data := expression.Data{
		"os":   "linux",
		"arch": "amd64",
		"ctx": struct {
			Workspace string `expr:"workspace"`
			Namespace string `expr:"namespace"`
		}{"workspace", "namespace"},
		"store": map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		"env": map[string]string{
			"ENV_VAR1": "env_value1",
			"ENV_VAR2": "env_value2",
		},
	}

	tests := []struct {
		name     string
		expr     string
		expected interface{}
	}{
		{"addition", "1 + 1", 2},
		{"boolean and", "true && false", false},
		{"string concatenation", `"hello" + " " + "world"`, "hello world"},
		{"map access", `store["key1"]`, "value1"},
		{"env access", `env["ENV_VAR1"]`, "env_value1"},
		{"os comparison", `os == "linux"`, true},
		{"arch comparison", `arch == "amd64"`, true},
		{"struct field access", `ctx.workspace == "workspace"`, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := expression.Evaluate(test.expr, data)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if result != test.expected {
				t.Errorf("expected %v, got %v", test.expected, result)
			}
		})
	}
}

func TestBuildDataExec(t *testing.T) {
	envMap := map[string]string{}
	ctx := context.Background()
	data, err := expression.BuildData(ctx, envMap)
	if err != nil {
		t.Fatalf("expected no error building data, got %v", err)
	}

	if _, exists := data["$"]; !exists {
		t.Fatal("exec function should exist in BuildData result")
	}

	result, err := expression.EvaluateString(`$("echo \"hello world\"")`, data)
	if err != nil {
		t.Fatalf("expected no error executing command, got %v", err)
	}

	expected := "hello world"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
