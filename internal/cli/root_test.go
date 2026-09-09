// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"io"
	"testing"
)

func TestSetBuildVersionUpdatesRootCommandVersion(t *testing.T) {
	oldBuildVersion := buildVersion
	oldRootVersion := rootCmd.Version
	t.Cleanup(func() {
		buildVersion = oldBuildVersion
		rootCmd.Version = oldRootVersion
	})

	SetBuildVersion("v1.2.3")

	if got := rootCmd.Version; got != "v1.2.3" {
		t.Fatalf("rootCmd.Version = %q, want %q", got, "v1.2.3")
	}
}

func TestSetBuildVersionIgnoresEmptyValue(t *testing.T) {
	oldBuildVersion := buildVersion
	oldRootVersion := rootCmd.Version
	t.Cleanup(func() {
		buildVersion = oldBuildVersion
		rootCmd.Version = oldRootVersion
	})

	SetBuildVersion("v9.9.9")
	SetBuildVersion("")

	if got := rootCmd.Version; got != "v9.9.9" {
		t.Fatalf("rootCmd.Version = %q, want %q", got, "v9.9.9")
	}
}

func TestRootCommand_HasProgressFlag(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("progress")
	if flag == nil {
		t.Fatalf("expected progress flag to exist")
	}
	if got := flag.DefValue; got != "auto" {
		t.Fatalf("progress default = %q, want %q", got, "auto")
	}
}

func TestExecuteContext_Help(t *testing.T) {
	oldOut := rootCmd.OutOrStdout()
	oldErr := rootCmd.ErrOrStderr()
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	t.Cleanup(func() {
		rootCmd.SetOut(oldOut)
		rootCmd.SetErr(oldErr)
		rootCmd.SetArgs(nil)
	})

	rootCmd.SetArgs([]string{"--help"})
	err := ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}
}
