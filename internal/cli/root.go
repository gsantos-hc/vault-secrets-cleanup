package cli

import (
	"fmt"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/config"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/logging"
	"github.com/spf13/cobra"
)

var (
	configPath string
	vaultAddr  string
	vaultToken string
	loadedCfg  *config.Config
	loadedLog  *logging.Logger
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
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to YAML configuration file")
	rootCmd.PersistentFlags().StringVar(&vaultAddr, "vault-addr", "", "Vault address (overrides config/env)")
	rootCmd.PersistentFlags().StringVar(&vaultToken, "vault-token", "", "Vault token (overrides config/env)")
	rootCmd.AddCommand(newValidateCmd())
	rootCmd.AddCommand(newDiscoverCmd())
	rootCmd.AddCommand(newAnalyzeCmd())
	rootCmd.AddCommand(newImportCmd())
}

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
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
