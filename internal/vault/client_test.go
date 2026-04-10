package vault

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewClient_ValidatesRequiredFields(t *testing.T) {
	_, err := NewClient(Config{})
	require.Error(t, err)
}

func TestHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/sys/health", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"initialized": true,
			"sealed":      false,
		})
	}))
	defer srv.Close()

	client, err := NewClient(Config{Address: srv.URL, Token: "test-token"})
	require.NoError(t, err)

	health, err := client.Health(context.Background())
	require.NoError(t, err)
	require.True(t, health.Initialized)
	require.False(t, health.Sealed)
}

func TestListNamespaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/sys/namespaces", r.URL.Path)
		require.Equal(t, "true", r.URL.Query().Get("list"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"keys": []string{"team-a/", "team-b/"},
			},
		})
	}))
	defer srv.Close()

	client, err := NewClient(Config{Address: srv.URL, Token: "test-token"})
	require.NoError(t, err)

	ns, err := client.ListNamespaces(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"team-a/", "team-b/"}, ns)
}

func TestWithNamespace(t *testing.T) {
	client, err := NewClient(Config{Address: "https://vault.example.com", Token: "token"})
	require.NoError(t, err)

	nsClient, err := client.WithNamespace("team-a")
	require.NoError(t, err)
	require.Equal(t, "team-a", nsClient.Namespace())
}

func TestRenewToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/auth/token/renew-self", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"auth": map[string]any{"lease_duration": 3600}})
	}))
	defer srv.Close()

	client, err := NewClient(Config{Address: srv.URL, Token: "test-token"})
	require.NoError(t, err)

	err = client.RenewToken(context.Background(), 3600)
	require.NoError(t, err)
}

func TestStartTokenRenewal_StopsOnContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"auth": map[string]any{"lease_duration": 3600}})
	}))
	defer srv.Close()

	client, err := NewClient(Config{Address: srv.URL, Token: "test-token"})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := client.StartTokenRenewal(ctx, 3600, 10*time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting token renewal loop to stop")
	}
}
