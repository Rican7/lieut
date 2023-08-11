# lieut

[![Build Status](https://github.com/Rican7/lieut/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/Rican7/lieut/actions/workflows/main.yml)
[![Coverage Status](https://coveralls.io/repos/github/Rican7/lieut/badge.svg)](https://coveralls.io/github/Rican7/lieut)
[![Go Report Card](https://goreportcard.com/badge/Rican7/lieut)](http://goreportcard.com/report/Rican7/lieut)
[![Go Reference](https://pkg.go.dev/badge/github.com/Rican7/lieut.svg)](https://pkg.go.dev/github.com/Rican7/lieut)
<!--[![Latest Stable Version](https://img.shields.io/github/release/Rican7/lieut.svg?style=flat)](https://github.com/Rican7/lieut/releases)-->

Lieut, short for lieutenant, or "second-in-command" to a commander.

An opinionated, feature-limited, no external dependency, "micro-framework" for building command line applications in Go.


## Project Status

This project is currently in "pre-release". The API may change.
Use a tagged version or vendor this dependency if you plan on using it.


## Example

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/Rican7/lieut"
)

func main() {
	do := func(ctx context.Context, arguments []string) error {
		_, err := fmt.Println(arguments)

		return err
	}

	app := lieut.NewSingleCommandApp(
		lieut.AppInfo{Name: "example"},
		do,
		flag.CommandLine,
		os.Stdout,
		os.Stderr,
	)

	exitCode := app.Run(context.Background(), os.Args[1:])

	os.Exit(exitCode)
}
```

For more examples, see [the documentation](https://pkg.go.dev/github.com/Rican7/lieut).
