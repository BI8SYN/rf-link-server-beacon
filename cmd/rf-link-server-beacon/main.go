package main

import (
	"os"

	"github.com/BI8SYN/rf-link-server-beacon/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, version))
}
