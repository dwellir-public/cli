package main

import (
	"os"

	"github.com/dwellir-public/cli/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
