# lieut

[![Build Status](https://github.com/Rican7/lieut/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/Rican7/lieut/actions/workflows/main.yml)
[![Coverage Status](https://coveralls.io/repos/github/Rican7/lieut/badge.svg)](https://coveralls.io/github/Rican7/lieut)
[![Go Report Card](https://goreportcard.com/badge/Rican7/lieut)](http://goreportcard.com/report/Rican7/lieut)
[![Go Reference](https://pkg.go.dev/badge/github.com/Rican7/lieut.svg)](https://pkg.go.dev/github.com/Rican7/lieut)
<!--[![Latest Stable Version](https://img.shields.io/github/release/Rican7/lieut.svg?style=flat)](https://github.com/Rican7/lieut/releases)-->

_Lieut, short for lieutenant, or "second-in-command" to a commander._

An opinionated, feature-limited, no external dependency, "micro-framework" for building command line applications in Go.

#### But why though?

In general, I personally don't like using frameworks... especially macro frameworks, and especially in Go.

I prefer using Go's extensive standard library, and when that doesn't quite provide what I'm looking for, I look towards smaller libraries that carry few (if any) other external dependencies and that integrate well with the standard library and its interfaces.

That being said, [the `flag` package in the standard library](https://pkg.go.dev/flag) leaves a lot to be desired, and unfortunately acts far less like a library and much more like a framework (it's library code [that calls `os.Exit()`](https://github.com/golang/go/blob/go1.21.0/src/flag/flag.go#L1168-L1171)... 😕😖). It defines a lot of higher-level application behaviors than typically expected, and those can be a little surprising.

The goal of this project is to get some of those quirks out of the way (or at least work WITH them in ways that reduce the surprises in behavior) and reduce the typical "setup" code that a command line application needs, all while working with the standard library, to expand upon it's capabilities rather than locking your application into a tree of external/third-party dependencies.


## Project Status

This project is currently in "pre-release". The API may change.
Use a tagged version or vendor this dependency if you plan on using it.


## Features

 - Relies solely on the standard library.
 - Sub-command applications (`app command`, `app othercommand`).
 - Automatic handling of typical error paths.
 - Standardized output handling of application (and command) usage, description, help, and version..
 - Help flag (`--help`) handling, with actual user-facing notice (it shows up as a flag in the options list), rather than just handling it silently..
 - Version flag (`--version`) handling with a standardized output.
 - Automatic flag handling (`--help` and `--version`) can be disabled by simply defining your own flags of the same name.
 - Global and sub-command flags with automatic merging.
 - Built-in signal handling (interrupt) with context cancellation.
 - Smart defaults, so there's less to configure.


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
