package main

import (
	"os"

	"github.com/mmdemirbas/mutercim/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
