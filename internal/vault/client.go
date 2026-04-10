package vault

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	api "github.com/hashicorp/vault/api"
)

type Config struct {
	Address   string
	Token     string
	Namespace string
}

type Client struct {
	client *api.Client
	config Config
}

func NewClient(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, errors.New("vault address is required")
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, errors.New("vault token is required")
	}

	apiCfg := api.DefaultConfig()
	apiCfg.Address = cfg.Address

	c, err := api.NewClient(apiCfg)
	if err != nil {
		return nil, fmt.Errorf("create vault api client: %w", err)
	}

	c.SetToken(cfg.Token)
	if cfg.Namespace != "" {
		c.SetNamespace(cfg.Namespace)
	}

	return &Client{client: c, config: cfg}, nil
}

func (c *Client) Health(ctx context.Context) (*api.HealthResponse, error) {
	health, err := c.client.Sys().HealthWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("vault health check: %w", err)
	}

	return health, nil
}

func (c *Client) ListNamespaces(ctx context.Context) ([]string, error) {
	secret, err := c.client.Logical().ListWithContext(ctx, "sys/namespaces")
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}
	if secret == nil || secret.Data == nil {
		return nil, nil
	}

	rawKeys, ok := secret.Data["keys"]
	if !ok {
		return nil, nil
	}

	keysAny, ok := rawKeys.([]any)
	if !ok {
		if keysStr, ok := rawKeys.([]string); ok {
			return keysStr, nil
		}
		return nil, nil
	}

	keys := make([]string, 0, len(keysAny))
	for _, item := range keysAny {
		v, ok := item.(string)
		if ok {
			keys = append(keys, v)
		}
	}

	return keys, nil
}

func (c *Client) RenewToken(ctx context.Context, increment int) error {
	_, err := c.client.Auth().Token().RenewSelfWithContext(ctx, increment)
	if err != nil {
		return fmt.Errorf("renew token: %w", err)
	}
	return nil
}

func (c *Client) StartTokenRenewal(ctx context.Context, increment int, interval time.Duration) <-chan error {
	errCh := make(chan error, 1)

	go func() {
		defer close(errCh)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				errCh <- nil
				return
			case <-ticker.C:
				if err := c.RenewToken(ctx, increment); err != nil {
					errCh <- err
					return
				}
			}
		}
	}()

	return errCh
}

func (c *Client) WithNamespace(namespace string) (*Client, error) {
	cfg := c.config
	cfg.Namespace = namespace
	return NewClient(cfg)
}

func (c *Client) Namespace() string {
	return c.client.Namespace()
}

func (c *Client) Address() string {
	return c.client.Address()
}

func (c *Client) ListMounts(ctx context.Context) (map[string]*api.MountOutput, error) {
	_ = ctx
	mounts, err := c.client.Sys().ListMounts()
	if err != nil {
		return nil, fmt.Errorf("list mounts: %w", err)
	}
	return mounts, nil
}

func (c *Client) ListSecrets(ctx context.Context, path string) (*api.Secret, error) {
	secret, err := c.client.Logical().ListWithContext(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("list secrets at %s: %w", path, err)
	}
	return secret, nil
}
