//go:build tools

// Package tools pins build-time tool dependencies so `go mod tidy` keeps
// them in go.sum. Never imported by application code.
package tools

import (
	_ "github.com/ogen-go/ogen/cmd/ogen"
)
