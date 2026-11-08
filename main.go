package main

import (
	"fmt"
	"os"

	"similar-images/pkg/cli"
	"similar-images/pkg/desktop"
)

func main() {
	err := cli.Run(os.Args[1:], os.Stdout, os.Stderr, desktop.Launch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
