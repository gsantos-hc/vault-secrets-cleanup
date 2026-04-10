package cli

import (
	"context"
	"fmt"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/config"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/vault"
	api "github.com/hashicorp/vault/api"
	"github.com/spf13/cobra"
)

type validationClient interface {
	Health(ctx context.Context) (*api.HealthResponse, error)
	ListNamespaces(ctx context.Context) ([]string, error)
}

type validateOptions struct {
	checkConnectivity bool
	checkPermissions  bool
}

var newValidationClient = func(cfg *config.Config) (validationClient, error) {
	return vault.NewClient(vault.Config{
		Address: cfg.Vault.Address,
		Token:   cfg.Vault.Token,
	})
}

func newValidateCmd() *cobra.Command {
	opts := validateOptions{}

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate Vault connectivity and permissions",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := GetConfig()
			if cfg == nil {
				return fmt.Errorf("configuration not loaded")
			}

			if !opts.checkConnectivity && !opts.checkPermissions {
				opts.checkConnectivity = true
				opts.checkPermissions = true
			}

			if err := runValidate(cmd.Context(), cfg, opts); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Validation successful")
			return nil
		},
	}

	cmd.Flags().BoolVar(&opts.checkConnectivity, "check-connectivity", true, "Validate connectivity to Vault health endpoint")
	cmd.Flags().BoolVar(&opts.checkPermissions, "check-permissions", true, "Validate list permissions for namespaces")

	return cmd
}

func runValidate(ctx context.Context, cfg *config.Config, opts validateOptions) error {
	client, err := newValidationClient(cfg)
	if err != nil {
		return fmt.Errorf("create validation client: %w", err)
	}

	if opts.checkConnectivity {
		health, err := client.Health(ctx)
		if err != nil {
			return fmt.Errorf("connectivity check failed: %w", err)
		}
		if health.Sealed {
			return fmt.Errorf("connectivity check failed: vault is sealed")
		}
	}

	if opts.checkPermissions {
		if _, err := client.ListNamespaces(ctx); err != nil {
			return fmt.Errorf("permissions check failed: %w", err)
		}
	}

	return nil
}
