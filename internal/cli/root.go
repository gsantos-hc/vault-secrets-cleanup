package cli

import (
	"context"
	"fmt"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/config"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/logging"
	"github.com/spf13/cobra"
)

var (
	configPath   string
	vaultAddr    string
	vaultToken   string
	progressFlag string
	loadedCfg    *config.Config
	loadedLog    *logging.Logger
	buildVersion = "dev"
)

var rootCmd = &cobra.Command{
	Use:   "vault-secrets-cleanup",
	Short: "Vault Secrets Cleanup helps identify and remove stale Vault secrets",
	Long:  "Vault Secrets Cleanup is a CLI for discovering stale secrets from Vault audit patterns and safely executing cleanup plans.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		help, _ := cmd.Flags().GetBool("help")
		if help || cmd.Name() == "help" {
			return nil
		}

		cfg, err := config.Load(configPath, map[string]any{
			"vault.address": vaultAddr,
			"vault.token":   vaultToken,
		})
		if err != nil {
			return err
		}

		logger, err := logging.New(logging.Config{
			Level:  cfg.Logging.Level,
			Format: cfg.Logging.Format,
		})
		if err != nil {
			return err
		}

		loadedCfg = cfg
		loadedLog = logger
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	rootCmd.Version = buildVersion
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to YAML configuration file")
	rootCmd.PersistentFlags().StringVar(&vaultAddr, "vault-addr", "", "Vault address (overrides config/env)")
	rootCmd.PersistentFlags().StringVar(&vaultToken, "vault-token", "", "Vault token (overrides config/env)")
	rootCmd.PersistentFlags().StringVar(&progressFlag, "progress", "auto", "Progress output mode (auto|tty|log|off)")
	rootCmd.AddCommand(newValidateCmd())
	rootCmd.AddCommand(newDiscoverCmd())
	rootCmd.AddCommand(newAnalyzeCmd())
	rootCmd.AddCommand(newImportCmd())
	rootCmd.AddCommand(newPlanCmd())
	rootCmd.AddCommand(newReportCmd())
	rootCmd.AddCommand(newExecuteCmd())
}

func Execute() error {
	return ExecuteContext(context.Background())
}

func ExecuteContext(ctx context.Context) error {
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		return fmt.Errorf("execute root command: %w", err)
	}

	return nil
}

func GetConfig() *config.Config {
	return loadedCfg
}

func GetLogger() *logging.Logger {
	return loadedLog
}

func SetBuildVersion(version string) {
	if version == "" {
		return
	}
	buildVersion = version
	rootCmd.Version = version
}

func GetProgressMode() string {
	return progressFlag
}
