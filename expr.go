package expression

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

func IsTruthy(ex string, data Data) (bool, error) {
	output, err := Evaluate(ex, data)
	if err != nil {
		return false, err
	}

	switch v := output.(type) {
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
		return false, nil
	}
}

func Evaluate(ex string, data Data) (interface{}, error) {
	var program *vm.Program
	var err error
	opts := additionalFunctions()
	if data != nil && !reflect.ValueOf(data).IsNil() {
		opts = append(opts, expr.Env(data))
	}
	program, err = expr.Compile(ex, opts...)
	if err != nil {
		return nil, err
	}

	output, err := expr.Run(program, data)
	if err != nil {
		return nil, err
	}
	return output, nil
}

func EvaluateString(ex string, data Data) (string, error) {
	output, err := Evaluate(ex, data)
	if err != nil {
		return "", err
	}
	switch o := output.(type) {
	case string:
		return o, nil
	case int, int64, float64, uint, uint64:
		return fmt.Sprintf("%v", o), nil
	case bool:
		return strconv.FormatBool(o), nil
	case []byte:
		return string(o), nil
	default:
		if output == nil {
			return "", nil
		}
		if reflect.TypeOf(output).Kind() == reflect.Ptr && reflect.ValueOf(output).IsNil() {
			return "", nil // Handle nil pointer gracefully
		}
		if reflect.TypeOf(output).Kind() == reflect.Map ||
			reflect.TypeOf(output).Kind() == reflect.Slice ||
			reflect.TypeOf(output).Kind() == reflect.Array {
			return fmt.Sprintf("%v", output), nil
		}
	}
	return "", fmt.Errorf("unexpected output type %T from expression %q", output, ex)
}

type Data interface{}

// BuildData constructs a Data object from a context, environment map, and key-value pairs.
// It provides the following variables by default:
// - `os`: string for the  operating system (e.g., "linux", "darwin")
// - `arch`: string for the architecture (e.g., "amd64", "arm64")
// - `env`: the environment variables passed in the envMap
// - `$`: a function that takes a shell command as input and returns its output as a string
func BuildData(ctx context.Context, envMap map[string]string, kvPairs ...interface{}) (Data, error) {
	kvMap := make(map[string]interface{})
	if len(kvPairs)%2 != 0 {
		return nil, fmt.Errorf("uneven number of key-value pairs")
	}

	for i := 0; i < len(kvPairs); i += 2 {
		key, ok := kvPairs[i].(string)
		if !ok {
			return nil, fmt.Errorf("key must be a string, got %T", kvPairs[i])
		}
		value := kvPairs[i+1]
		kvMap[key] = value
	}

	kvMap["os"] = runtime.GOOS
	kvMap["arch"] = runtime.GOARCH
	kvMap["env"] = envMap
	kvMap["$"] = func(command string) (string, error) {
		output, err := execute(ctx, command, environmentToSlice(envMap))
		if err != nil {
			return "", fmt.Errorf("command failed: %v, output: %s", err, output)
		}
		return strings.TrimSpace(output), nil
	}

	return kvMap, nil
}

func execute(ctx context.Context, cmd string, envList []string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	parser := syntax.NewParser()
	reader := strings.NewReader(strings.TrimSpace(cmd))
	prog, err := parser.Parse(reader, "")
	if err != nil {
		return "", fmt.Errorf("unable to parse command - %w", err)
	}

	if envList == nil {
		envList = make([]string, 0)
	}
	envList = append(os.Environ(), envList...)

	stdOutBuffer := &strings.Builder{}
	stdErrBuffer := &strings.Builder{}

	runner, err := interp.New(
		interp.Env(expand.ListEnviron(envList...)),
		interp.StdIO(
			os.Stdin,
			stdOutBuffer,
			stdErrBuffer,
		),
	)
	if err != nil {
		return "", fmt.Errorf("unable to create runner - %w", err)
	}

	err = runner.Run(ctx, prog)
	if err != nil {
		var exitStatus interp.ExitStatus
		if errors.As(err, &exitStatus) {
			return stdErrBuffer.String(), fmt.Errorf("command exited with non-zero status %w", exitStatus)
		}
		return stdErrBuffer.String(), fmt.Errorf("encountered an error executing command - %w", err)
	}
	output := stdOutBuffer.String()
	if stderr := stdErrBuffer.String(); stderr != "" {
		output += "\n" + stderr
	}
	return strings.TrimSpace(output), nil
}

func environmentToSlice(env map[string]string) []string {
	for k, v := range env {
		if strings.Contains(v, "$") || strings.Contains(v, "{") {
			env[k] = os.ExpandEnv(v)
		}
	}

	var envSlice []string
	for key, value := range env {
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", key, value))
	}
	return envSlice
}

func additionalFunctions() []expr.Option {
	return []expr.Option{
		// File existence and type checking
		expr.Function("fileExists", func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return false, fmt.Errorf("fileExists() takes exactly 1 argument")
			}
			path, ok := params[0].(string)
			if !ok {
				return false, fmt.Errorf("fileExists() requires string argument")
			}
			_, err := os.Stat(path)
			return err == nil, nil
		}),

		expr.Function("dirExists", func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return false, fmt.Errorf("dirExists() takes exactly 1 argument")
			}
			path, ok := params[0].(string)
			if !ok {
				return false, fmt.Errorf("dirExists() requires string argument")
			}
			info, err := os.Stat(path)
			return err == nil && info.IsDir(), nil
		}),
		expr.Function("isFile", func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return false, fmt.Errorf("isFile() takes exactly 1 argument")
			}
			path, ok := params[0].(string)
			if !ok {
				return false, fmt.Errorf("isFile() requires string argument")
			}
			info, err := os.Stat(path)
			return err == nil && !info.IsDir(), nil
		}),
		expr.Function("isDir", func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return false, fmt.Errorf("isDir() takes exactly 1 argument")
			}
			path, ok := params[0].(string)
			if !ok {
				return false, fmt.Errorf("isDir() requires string argument")
			}
			info, err := os.Stat(path)
			return err == nil && info.IsDir(), nil
		}),

		// Path operations
		expr.Function("basename", func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return "", fmt.Errorf("basename() takes exactly 1 argument")
			}
			path, ok := params[0].(string)
			if !ok {
				return "", fmt.Errorf("basename() requires string argument")
			}
			return filepath.Base(path), nil
		}),
		expr.Function("dirname", func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return "", fmt.Errorf("dirname() takes exactly 1 argument")
			}
			path, ok := params[0].(string)
			if !ok {
				return "", fmt.Errorf("dirname() requires string argument")
			}
			return filepath.Dir(path), nil
		}),

		// File content operations
		expr.Function("readFile", func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return "", fmt.Errorf("readFile() takes exactly 1 argument")
			}
			path, ok := params[0].(string)
			if !ok {
				return "", fmt.Errorf("readFile() requires string argument")
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			return string(content), nil
		}),
		expr.Function("fileSize", func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return int64(0), fmt.Errorf("fileSize() takes exactly 1 argument")
			}
			path, ok := params[0].(string)
			if !ok {
				return int64(0), fmt.Errorf("fileSize() requires string argument")
			}
			info, err := os.Stat(path)
			if err != nil {
				return int64(0), err
			}
			return info.Size(), nil
		}),

		// File time operations
		expr.Function("fileModTime", func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return time.Time{}, fmt.Errorf("fileModTime() takes exactly 1 argument")
			}
			path, ok := params[0].(string)
			if !ok {
				return time.Time{}, fmt.Errorf("fileModTime() requires string argument")
			}
			info, err := os.Stat(path)
			if err != nil {
				return time.Time{}, err
			}
			return info.ModTime(), nil
		}),

		expr.Function("fileAge", func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return time.Duration(0), fmt.Errorf("fileAge() takes exactly 1 argument")
			}
			path, ok := params[0].(string)
			if !ok {
				return time.Duration(0), fmt.Errorf("fileAge() requires string argument")
			}
			info, err := os.Stat(path)
			if err != nil {
				return time.Duration(0), err
			}
			return time.Since(info.ModTime()), nil
		}),
	}
}
