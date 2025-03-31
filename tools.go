//go:build tools
// +build tools

// This package imports things required by build tools, to force `go mod` to see them as dependencies
// go tool is there 1.24 but not for internal script/tools. This pattern is still needed
package tools

import (
	_ "github.com/evanw/esbuild/pkg/api"
	_ "golang.org/x/net/html"
)
