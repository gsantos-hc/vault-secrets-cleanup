package cli

import "testing"

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
