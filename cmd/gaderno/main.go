package main

import (
	"os"

	"github.com/lucasew/gaderno/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
