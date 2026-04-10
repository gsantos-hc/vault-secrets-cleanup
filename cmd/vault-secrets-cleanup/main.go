package main

import (
	"os"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
