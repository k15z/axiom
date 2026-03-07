package main

import (
	"github.com/k15z/axiom/internal/cli"
)

// version is set at build time via ldflags:
//
//	go build -ldflags "-X main.version=v1.0.0"
var version = "dev"

func main() {
	cli.SetVersion(version)
	cli.Execute()
}
