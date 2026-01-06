package main

import (
	"fmt"
	"os"

	"remindd/internal/app"
)

func main() {
	exitCode := app.Run(os.Args, os.Stdout, os.Stderr)
	if exitCode != 0 {
		// Ensure non-empty status on abnormal exits.
		fmt.Fprintln(os.Stderr)
	}
	os.Exit(exitCode)
}
