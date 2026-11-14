package main

import (
	"os"

	"github.com/ryangerardwilson/tesseract/internal/cli"
)

func main() {
	os.Exit(cli.RunInvoked(os.Args[0], os.Args[1:], nil, os.Stdout, os.Stderr))
}
