package main

import (
	"os"

	"github.com/mmdemirbas/mutercim/internal/cli"
	"github.com/mmdemirbas/mutercim/internal/lang/ar"
)

func main() {
	// Register language profiles. Explicit because the project forbids
	// init() functions. Adding a new language is one import + one
	// Register() call here.
	ar.Register()

	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
