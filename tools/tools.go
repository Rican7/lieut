// Copyright © 2023 Trevor N. Suarez (Rican7)

//go:build tools

// Package tools provides tools for development.
//
// It follows the pattern set-forth in the wiki:
//   - https://go.dev/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
//   - https://github.com/go-modules-by-example/index/tree/4ea90b07f9/010_tools
package tools

import (
	// Tools for development
	_ "golang.org/x/lint/golint"
	_ "honnef.co/go/tools/cmd/staticcheck"
	_ "mvdan.cc/gofumpt"
)
