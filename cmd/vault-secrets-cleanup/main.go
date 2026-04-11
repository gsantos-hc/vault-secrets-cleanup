package main

import (
	"os"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/cli"
)

var version = "dev"

func main() {
	cli.SetBuildVersion(version)
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
