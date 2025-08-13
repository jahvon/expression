package expression_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	data := map[string]interface{}{
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

	dataMap, ok := data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be a map[string]interface{}, got %T", data)
	}
	if _, exists := dataMap["$"]; !exists {
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

func TestFileExistenceFunctions(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	testDir := filepath.Join(tempDir, "testdir")

	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	tests := []struct {
		name     string
		expr     string
		expected bool
	}{
		{"fileExists with existing file", `fileExists("` + testFile + `")`, true},
		{"fileExists with existing dir", `fileExists("` + testDir + `")`, true},
		{"fileExists with non-existing", `fileExists("/non/existing/path")`, false},
		{"dirExists with existing dir", `dirExists("` + testDir + `")`, true},
		{"dirExists with file", `dirExists("` + testFile + `")`, false},
		{"dirExists with non-existing", `dirExists("/non/existing/path")`, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := expression.Evaluate(test.expr, nil)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if result != test.expected {
				t.Errorf("expected %v, got %v", test.expected, result)
			}
		})
	}
}

func TestFileTypeFunctions(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	testDir := filepath.Join(tempDir, "testdir")

	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	tests := []struct {
		name     string
		expr     string
		expected bool
	}{
		{"isFile with file", `isFile("` + testFile + `")`, true},
		{"isFile with directory", `isFile("` + testDir + `")`, false},
		{"isFile with non-existing", `isFile("/non/existing/path")`, false},
		{"isDir with directory", `isDir("` + testDir + `")`, true},
		{"isDir with file", `isDir("` + testFile + `")`, false},
		{"isDir with non-existing", `isDir("/non/existing/path")`, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := expression.Evaluate(test.expr, nil)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if result != test.expected {
				t.Errorf("expected %v, got %v", test.expected, result)
			}
		})
	}
}

func TestPathOperationFunctions(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{"basename of file", `basename("/path/to/file.txt")`, "file.txt"},
		{"basename of directory", `basename("/path/to/dir")`, "dir"},
		{"basename of root", `basename("/")`, "/"},
		{"basename of current", `basename(".")`, "."},
		{"dirname of file", `dirname("/path/to/file.txt")`, "/path/to"},
		{"dirname of directory", `dirname("/path/to/dir")`, "/path/to"},
		{"dirname of root", `dirname("/")`, "/"},
		{"dirname of current", `dirname(".")`, "."},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := expression.EvaluateString(test.expr, nil)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if result != test.expected {
				t.Errorf("expected %q, got %q", test.expected, result)
			}
		})
	}
}

func TestFileContentFunctions(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "Hello, World!\nThis is a test file."

	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name     string
		expr     string
		expected interface{}
	}{
		{"readFile success", `readFile("` + testFile + `")`, testContent},
		{"fileSize success", `fileSize("` + testFile + `")`, int64(len(testContent))},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := expression.Evaluate(test.expr, nil)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if result != test.expected {
				t.Errorf("expected %v, got %v", test.expected, result)
			}
		})
	}
}

func TestFileTimeFunctions(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("failed to stat test file: %v", err)
	}
	expectedModTime := info.ModTime()

	time.Sleep(10 * time.Millisecond)

	tests := []struct {
		name string
		expr string
		test func(result interface{}) bool
	}{
		{
			"fileModTime returns correct time",
			`fileModTime("` + testFile + `")`,
			func(result interface{}) bool {
				modTime, ok := result.(time.Time)
				return ok && modTime.Equal(expectedModTime)
			},
		},
		{
			"fileAge returns positive duration",
			`fileAge("` + testFile + `")`,
			func(result interface{}) bool {
				age, ok := result.(time.Duration)
				return ok && age > 0
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := expression.Evaluate(test.expr, nil)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if !test.test(result) {
				t.Errorf("test failed for result: %v", result)
			}
		})
	}
}

func TestFileOperationErrors(t *testing.T) {
	tests := []struct {
		name        string
		expr        string
		expectError bool
		errorMsg    string
	}{
		{"fileExists wrong args", `fileExists()`, true, "takes exactly 1 argument"},
		{"fileExists wrong type", `fileExists(123)`, true, "requires string argument"},
		{"dirExists wrong args", `dirExists("a", "b")`, true, "takes exactly 1 argument"},
		{"dirExists wrong type", `dirExists(true)`, true, "requires string argument"},
		{"isFile wrong args", `isFile()`, true, "takes exactly 1 argument"},
		{"isFile wrong type", `isFile(123)`, true, "requires string argument"},
		{"isDir wrong args", `isDir("a", "b")`, true, "takes exactly 1 argument"},
		{"isDir wrong type", `isDir(false)`, true, "requires string argument"},
		{"basename wrong args", `basename()`, true, "takes exactly 1 argument"},
		{"basename wrong type", `basename(123)`, true, "requires string argument"},
		{"dirname wrong args", `dirname("a", "b")`, true, "takes exactly 1 argument"},
		{"dirname wrong type", `dirname(true)`, true, "requires string argument"},
		{"readFile wrong args", `readFile()`, true, "takes exactly 1 argument"},
		{"readFile wrong type", `readFile(123)`, true, "requires string argument"},
		{"readFile non-existing", `readFile("/non/existing/file")`, true, "no such file"},
		{"fileSize wrong args", `fileSize()`, true, "takes exactly 1 argument"},
		{"fileSize wrong type", `fileSize(true)`, true, "requires string argument"},
		{"fileSize non-existing", `fileSize("/non/existing/file")`, true, "no such file"},
		{"fileModTime wrong args", `fileModTime()`, true, "takes exactly 1 argument"},
		{"fileModTime wrong type", `fileModTime(123)`, true, "requires string argument"},
		{"fileModTime non-existing", `fileModTime("/non/existing/file")`, true, "no such file"},
		{"fileAge wrong args", `fileAge()`, true, "takes exactly 1 argument"},
		{"fileAge wrong type", `fileAge(false)`, true, "requires string argument"},
		{"fileAge non-existing", `fileAge("/non/existing/file")`, true, "no such file"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := expression.Evaluate(test.expr, nil)
			if test.expectError {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				if !strings.Contains(err.Error(), test.errorMsg) {
					t.Errorf("expected error to contain %q, got %v", test.errorMsg, err)
				}
			} else if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
