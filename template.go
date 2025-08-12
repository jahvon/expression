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
	name      string
	text      string
	data      any
	tmpl      *template.Template
	exprCache map[string]*vm.Program
}

func NewTemplate(name string, data Data) *Template {
	t := &Template{
		name:      name,
		data:      data,
		exprCache: make(map[string]*vm.Program),
	}
	return t
}

func (t *Template) Parse(text string) error {
	t.text = text
	processed := t.preProcessExpressions(text)
	tmpl := template.New(t.name).Funcs(template.FuncMap{
		"expr": t.evalExpr, 
		"exprBool": t.evalExprBool,
		"int": func(v interface{}) int {
			switch val := v.(type) {
			case int:
				return val
			case int64:
				return int(val)
			case float64:
				return int(val)
			case string:
				if i, err := strconv.Atoi(val); err == nil {
					return i
				}
				return 0
			default:
				return 0
			}
		},
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

func (t *Template) compileExpr(expression string) (*vm.Program, error) {
	if node, ok := t.exprCache[expression]; ok {
		return node, nil
	}

	var compiled *vm.Program
	var err error
	if t.data == nil || reflect.ValueOf(t.data).IsNil() {
		compiled, err = expr.Compile(expression)
	} else {
		compiled, err = expr.Compile(expression, expr.Env(t.data))
	}
	if err != nil {
		return nil, err
	}
	t.exprCache[expression] = compiled
	return compiled, nil
}

//nolint:funlen
func (t *Template) preProcessExpressions(text string) string {
	var result strings.Builder
	remaining := text
	contextDepth := 0 // Track nested range/with blocks

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
			result.WriteString("-")
		}
		result.WriteString(" ")

		switch {
		case strings.HasPrefix(action, "if "):
			condition := strings.TrimPrefix(action, "if ")
			condition = strings.TrimSpace(condition)

			// Special cases where we should use Go's template boolean evaluation:
			// 1. Dot references within range/with contexts
			// 2. Variable references (starting with $) - includes function calls with variables
			if (contextDepth > 0 && strings.HasPrefix(condition, ".")) || strings.Contains(condition, "$") {
				result.WriteString("if ")
				result.WriteString(condition)
			} else {
				result.WriteString("if exprBool `")
				result.WriteString(condition)
				result.WriteString("`")
			}
		case strings.HasPrefix(action, "with "):
			value := strings.TrimPrefix(action, "with ")
			result.WriteString("with expr `")
			result.WriteString(strings.TrimSpace(value))
			result.WriteString("`")
			contextDepth++
		case action == "end":
			result.WriteString("end")
			if contextDepth > 0 {
				contextDepth--
			}
		case action == "else":
			result.WriteString("else")
		case strings.HasPrefix(action, "else if "):
			condition := strings.TrimPrefix(action, "else if ")
			condition = strings.TrimSpace(condition)

			// Same logic as regular if conditions
			if (contextDepth > 0 && strings.HasPrefix(condition, ".")) || strings.Contains(condition, "$") {
				result.WriteString("else if ")
				result.WriteString(condition)
			} else {
				result.WriteString("else if exprBool `")
				result.WriteString(condition)
				result.WriteString("`")
			}
		case strings.HasPrefix(action, "range "):
			value := strings.TrimPrefix(action, "range ")
			value = strings.TrimSpace(value)

			// Check if this is a range with variable assignment (contains :=)
			if strings.Contains(value, ":=") {
				// Parse out the expression part after :=
				parts := strings.Split(value, ":=")
				if len(parts) == 2 {
					vars := strings.TrimSpace(parts[0])
					expr := strings.TrimSpace(parts[1])
					result.WriteString("range ")
					result.WriteString(vars)
					result.WriteString(" := expr `")
					result.WriteString(expr)
					result.WriteString("`")
				} else {
					// Fallback: use as-is
					result.WriteString("range ")
					result.WriteString(value)
				}
			} else {
				result.WriteString("range expr `")
				result.WriteString(value)
				result.WriteString("`")
			}
			contextDepth++
		default:
			if contextDepth > 0 && (strings.HasPrefix(action, ".") || action == ".") {
				result.WriteString(action)
			} else if strings.Contains(action, ":=") {
				// Variable assignment - parse it carefully
				parts := strings.Split(action, ":=")
				if len(parts) == 2 {
					varName := strings.TrimSpace(parts[0])
					expr := strings.TrimSpace(parts[1])

					result.WriteString(varName)
					result.WriteString(" := ")

					// If the expression is just "." and we're in a context, use it directly
					// Otherwise, wrap it in expr for evaluation
					if contextDepth > 0 && expr == "." {
						result.WriteString(".")
					} else {
						result.WriteString("expr `")
						result.WriteString(expr)
						result.WriteString("`")
					}
				} else {
					// Fallback: use as-is
					result.WriteString(action)
				}
			} else if strings.HasPrefix(action, "$") {
				// Variable reference - use Go template syntax directly
				result.WriteString(action)
			} else {
				result.WriteString("expr `")
				result.WriteString(strings.TrimSpace(action))
				result.WriteString("`")
			}
		}

		result.WriteString(" ")
		if trimRight {
			result.WriteString("-")
		}
		result.WriteString("}}")

		remaining = remaining[end+2:]
	}

	return result.String()
}

func (t *Template) evalExpr(expression string) (interface{}, error) {
	program, err := t.compileExpr(expression)
	if err != nil {
		return nil, fmt.Errorf("compiling expression: %w", err)
	}

	result, err := expr.Run(program, t.data)
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
		truthy, err := strconv.ParseBool(strings.Trim(v, `"' `))
		if err != nil {
			return false, err
		}
		return truthy, nil
	default:
		return result != nil, nil
	}
}
