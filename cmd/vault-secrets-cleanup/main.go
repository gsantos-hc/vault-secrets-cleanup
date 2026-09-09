// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/cli"
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cli.SetBuildVersion(version)
	if err := cli.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
