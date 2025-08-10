# Expression

[![Go Report Card](https://goreportcard.com/badge/github.com/jahvon/expression)](https://goreportcard.com/report/github.com/jahvon/expression)
[![Go Reference](https://pkg.go.dev/badge/github.com/jahvon/expression.svg)](https://pkg.go.dev/github.com/jahvon/expression)

A Go package that provides powerful expression evaluation and templating capabilities, 
built on top of the [expr](https://github.com/expr-lang/expr) language.

**Installation**

```bash
go get github.com/jahvon/expression
```

## Basic Expression Evaluation

```go
package main

import (
    "fmt"
    "github.com/jahvon/expression"
)

func main() {
    // Create data context
    data := expression.Data{
        "name": "John",
        "age":  30,
    }

    // Evaluate expressions
    result, err := expression.Evaluate("name + ' is ' + string(age)", data)
    if err != nil {
        panic(err)
    }
    fmt.Println(result) // Output: John is 30

    // Check truthiness
    truthy, err := expression.IsTruthy("age > 18", data)
    if err != nil {
        panic(err)
    }
    fmt.Println(truthy) // Output: true
}
```

### Template Processing

The template engine extends Go's `text/template` with Expr expression evaluation:

```go
package main

import (
    "fmt"
    "github.com/jahvon/expression"
)

func main() {
    data := expression.Data{
        "user":    "Alice",
        "enabled": true,
        "items":   []string{"apple", "banana", "orange"},
    }

    tmpl := expression.NewTemplate("example", data)
    
    templateText := `
Hello {{user}}!
It's {{$("time")}}

{{if enabled}}
Your account is active.
Items:
{{range items}}
- {{.}}
{{end}}
{{end}}
`

    err := tmpl.Parse(templateText)
    if err != nil {
        panic(err)
    }

    result, err := tmpl.ExecuteToString()
    if err != nil {
        panic(err)
    }
    
    fmt.Println(result)
}
```

## Contributing

Contributions are welcome! Please ensure all tests pass:

```bash
go test ./...
```
