package expression

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"text/template"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// Template wraps text/template but evaluates expressions using expr instead
type Template struct {
	name         string
	text         string
	data         any
	tmpl         *template.Template
	exprCache    map[string]*vm.Program
	templateVars map[string]interface{}
}

func NewTemplate(name string, data Data) *Template {
	return &Template{
		name:         name,
		data:         data,
		exprCache:    make(map[string]*vm.Program),
		templateVars: make(map[string]interface{}),
	}
}

func (t *Template) Parse(text string) error {
	t.text = text
	processed := t.preProcessExpressions(text)

	tmpl := template.New(t.name).Funcs(template.FuncMap{
		"expr":     t.evalExpr,
		"exprBool": t.evalExprBool,
		"setVar":   t.setTemplateVar,
	})

	parsed, err := tmpl.Parse(processed)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	t.tmpl = parsed
	return nil
}

func (t *Template) ParseFile(file string) error {
	text, err := os.ReadFile(filepath.Clean(file))
	if err != nil {
		return fmt.Errorf("reading template file %s: %w", file, err)
	}
	return t.Parse(string(text))
}

func (t *Template) Execute(wr io.Writer) error {
	if t.tmpl == nil {
		return fmt.Errorf("template not parsed")
	}
	return t.tmpl.Execute(wr, t.data)
}

func (t *Template) ExecuteToString() (string, error) {
	var buf bytes.Buffer
	err := t.Execute(&buf)
	return buf.String(), err
}

func (t *Template) preProcessExpressions(text string) string {
	var result strings.Builder
	remaining := text
	contextDepth := 0

	for {
		start := strings.Index(remaining, "{{")
		if start == -1 {
			result.WriteString(remaining)
			break
		}
		result.WriteString(remaining[:start])

		end := strings.Index(remaining[start:], "}}")
		if end == -1 {
			result.WriteString(remaining[start:])
			break
		}
		end += start

		action := remaining[start+2 : end]
		trimLeft := strings.HasPrefix(action, "-")
		trimRight := strings.HasSuffix(action, "-")
		action = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(action, "-"), "-"))

		result.WriteString("{{")
		if trimLeft {
			result.WriteString("- ")
		} else {
			result.WriteString(" ")
		}

		processedAction := t.processAction(action, contextDepth)
		result.WriteString(processedAction)

		// Update context depth
		if strings.HasPrefix(action, "range ") || strings.HasPrefix(action, "with ") {
			contextDepth++
		} else if action == "end" {
			if contextDepth > 0 {
				contextDepth--
			}
		}

		if trimRight {
			result.WriteString(" -")
		} else {
			result.WriteString(" ")
		}
		result.WriteString("}}")

		remaining = remaining[end+2:]
	}

	return result.String()
}

func (t *Template) processAction(action string, contextDepth int) string {
	action = strings.TrimSpace(action)

	// Control structures
	if strings.HasPrefix(action, "if ") {
		condition := strings.TrimPrefix(action, "if ")
		condition = strings.TrimSpace(condition)

		if t.isGoSyntax(condition, contextDepth) {
			return "if " + condition
		}
		return "if exprBool `" + condition + "`"
	}

	if strings.HasPrefix(action, "else if ") {
		condition := strings.TrimPrefix(action, "else if ")
		condition = strings.TrimSpace(condition)

		if t.isGoSyntax(condition, contextDepth) {
			return "else if " + condition
		}
		return "else if exprBool `" + condition + "`"
	}

	// With and range structures
	if strings.HasPrefix(action, "with ") {
		value := strings.TrimPrefix(action, "with ")
		value = strings.TrimSpace(value)

		if t.isGoSyntax(value, contextDepth) {
			return "with " + value
		}
		return "with expr `" + value + "`"
	}

	if strings.HasPrefix(action, "range ") {
		value := strings.TrimPrefix(action, "range ")
		value = strings.TrimSpace(value)

		if strings.Contains(value, ":=") {
			parts := strings.Split(value, ":=")
			if len(parts) == 2 {
				vars := strings.TrimSpace(parts[0])
				e := strings.TrimSpace(parts[1])
				if t.isGoSyntax(e, contextDepth) {
					return "range " + vars + " := " + e
				}
				return "range " + vars + " := expr `" + e + "`"
			}
		}

		if t.isGoSyntax(value, contextDepth) {
			return "range " + value
		}
		return "range expr `" + value + "`"
	}

	// Variable assignment
	if strings.Contains(action, ":=") {
		parts := strings.Split(action, ":=")
		if len(parts) == 2 {
			varName := strings.TrimSpace(parts[0])
			e := strings.TrimSpace(parts[1])

			if t.isGoSyntax(e, contextDepth) {
				return fmt.Sprintf("%s := (setVar %q %s)", varName, strings.TrimPrefix(varName, "$"), e)
			}
			return fmt.Sprintf("%s := (setVar %q (expr `%s`))", varName, strings.TrimPrefix(varName, "$"), e)
		}
	}

	// Simple keywords
	if action == "end" || action == "else" {
		return action
	}

	// Regular expressions
	if t.isGoSyntax(action, contextDepth) {
		return action
	}
	return "expr `" + action + "`"
}

func (t *Template) isGoSyntax(expression string, contextDepth int) bool {
	expression = strings.TrimSpace(expression)

	if strings.Contains(expression, "$") {
		return true
	}

	if contextDepth > 0 && (strings.HasPrefix(expression, ".") || expression == ".") {
		return true // dot notation in nested contexts (range/with)
	}

	return false
}

func (t *Template) setTemplateVar(name string, value interface{}) interface{} {
	t.templateVars[name] = value
	return value
}

func (t *Template) compileExpr(expression string) (*vm.Program, error) {
	if node, ok := t.exprCache[expression]; ok {
		return node, nil
	}

	env := t.createExprEnvironment()
	compiled, err := expr.Compile(expression, expr.Env(env))
	if err != nil {
		return nil, err
	}

	t.exprCache[expression] = compiled
	return compiled, nil
}

func (t *Template) createExprEnvironment() map[string]interface{} {
	env := make(map[string]interface{})

	if t.data != nil {
		val := reflect.ValueOf(t.data)
		if val.Kind() == reflect.Map {
			for _, key := range val.MapKeys() {
				if key.Kind() == reflect.String {
					env[key.String()] = val.MapIndex(key).Interface()
				}
			}
		}
	}

	for name, value := range t.templateVars {
		env[name] = value
	}

	return env
}

func (t *Template) evalExpr(expression string) (interface{}, error) {
	program, err := t.compileExpr(expression)
	if err != nil {
		return nil, fmt.Errorf("compiling expression: %w", err)
	}

	env := t.createExprEnvironment()
	result, err := expr.Run(program, env)
	if err != nil {
		return nil, fmt.Errorf("evaluating expression: %w", err)
	}

	return result, nil
}

func (t *Template) evalExprBool(expression string) (bool, error) {
	result, err := t.evalExpr(expression)
	if err != nil {
		return false, err
	}

	switch v := result.(type) {
	case bool:
		return v, nil
	case int, int64, float64, uint, uint64:
		return v != 0, nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return false, nil
		}
		if b, err := strconv.ParseBool(trimmed); err == nil {
			return b, nil
		}
		return true, nil // Non-empty strings are truthy
	case nil:
		return false, nil
	default:
		val := reflect.ValueOf(result)
		if !val.IsValid() {
			return false, nil
		}
		return !val.IsZero(), nil
	}
}
