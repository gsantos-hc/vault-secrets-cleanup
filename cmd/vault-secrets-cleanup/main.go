package main

import (
	"os"

	"github.com/yourusername/vault-secrets-cleanup/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
