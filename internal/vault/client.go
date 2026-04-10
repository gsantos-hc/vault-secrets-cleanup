package vault

import (
	"context"
	"errors"
	"fmt"
	"path"
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

func (c *Client) Delete(ctx context.Context, secretPath string) error {
	_, err := c.client.Logical().DeleteWithContext(ctx, secretPath)
	if err != nil {
		return fmt.Errorf("delete secret at %s: %w", secretPath, err)
	}
	return nil
}

func (c *Client) KVVersion(ctx context.Context, mountPath string) (int, error) {
	mounts, err := c.ListMounts(ctx)
	if err != nil {
		return 0, err
	}

	normalized := normalizeMountPath(mountPath)
	mount, ok := mounts[normalized]
	if !ok {
		return 0, fmt.Errorf("mount %q not found", normalized)
	}
	if mount.Type != "kv" {
		return 0, fmt.Errorf("mount %q is not kv", normalized)
	}

	if mount.Options != nil && mount.Options["version"] == "2" {
		return 2, nil
	}
	return 1, nil
}

func normalizeMountPath(path string) string {
	trimmed := strings.TrimSpace(path)
	trimmed = strings.TrimPrefix(trimmed, "/")
	if trimmed == "" {
		return ""
	}
	if !strings.HasSuffix(trimmed, "/") {
		trimmed += "/"
	}
	return trimmed
}

func (c *Client) CreateNamespace(ctx context.Context, namespace string) error {
	ns := strings.Trim(strings.TrimSpace(namespace), "/")
	if ns == "" {
		return errors.New("namespace is required")
	}

	_, err := c.client.Logical().WriteWithContext(ctx, path.Join("sys/namespaces", ns), map[string]any{})
	if err != nil {
		return fmt.Errorf("create namespace %q: %w", ns, err)
	}

	return nil
}

func (c *Client) EnableKVMount(ctx context.Context, mountPath string, kvVersion int) error {
	mount := strings.Trim(strings.TrimSpace(mountPath), "/")
	if mount == "" {
		return errors.New("mount path is required")
	}
	if kvVersion != 1 && kvVersion != 2 {
		return fmt.Errorf("invalid kv version %d", kvVersion)
	}

	_, err := c.client.Logical().WriteWithContext(ctx, path.Join("sys/mounts", mount), map[string]any{
		"type": "kv",
		"options": map[string]any{
			"version": fmt.Sprintf("%d", kvVersion),
		},
	})
	if err != nil {
		return fmt.Errorf("enable kv mount %q: %w", mount, err)
	}

	return nil
}

func (c *Client) WriteKVSecret(ctx context.Context, mountPath, secretPath string, data map[string]any, kvVersion int) error {
	mount := strings.Trim(strings.TrimSpace(mountPath), "/")
	secret := strings.Trim(strings.TrimSpace(secretPath), "/")
	if mount == "" {
		return errors.New("mount path is required")
	}
	if secret == "" {
		return errors.New("secret path is required")
	}
	if kvVersion != 1 && kvVersion != 2 {
		return fmt.Errorf("invalid kv version %d", kvVersion)
	}

	writePath := path.Join(mount, secret)
	payload := data
	if payload == nil {
		payload = map[string]any{}
	}

	if kvVersion == 2 {
		writePath = path.Join(mount, "data", secret)
		payload = map[string]any{"data": payload}
	}

	_, err := c.client.Logical().WriteWithContext(ctx, writePath, payload)
	if err != nil {
		return fmt.Errorf("write secret %q on mount %q: %w", secret, mount, err)
	}

	return nil
}
