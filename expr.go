package expression

import (
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
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
	if data == nil || reflect.ValueOf(data).IsNil() {
		program, err = expr.Compile(ex)
	} else {
		program, err = expr.Compile(ex, expr.Env(data))
	}
	if err != nil {
		return nil, err
	}

	// convert Data to map[string]interface{} for expr.Run
	var runData interface{} = data
	if data != nil {
		runData = map[string]interface{}(data)
	}

	output, err := expr.Run(program, runData)
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
	str, ok := output.(string)
	if !ok {
		return "", fmt.Errorf("expected string, got %T", output)
	}
	return str, nil
}

type Data map[string]interface{}

func BuildData(envMap map[string]string, kvPairs ...interface{}) (Data, error) {
	kvMap := make(map[string]interface{})
	if len(kvPairs)%2 != 0 {
		return Data{}, fmt.Errorf("uneven number of key-value pairs")
	}

	for i := 0; i < len(kvPairs); i += 2 {
		key, ok := kvPairs[i].(string)
		if !ok {
			return Data{}, fmt.Errorf("key must be a string, got %T", kvPairs[i])
		}
		value := kvPairs[i+1]
		kvMap[key] = value
	}

	kvMap["os"] = runtime.GOOS
	kvMap["arch"] = runtime.GOARCH
	kvMap["env"] = envMap

	return kvMap, nil
}
