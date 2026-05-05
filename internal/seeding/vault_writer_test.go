package seeding

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	vaultpkg "github.com/gsantos-hc/vault-secrets-cleanup/internal/vault"
	"github.com/stretchr/testify/require"
)

func TestVaultClientWriter_EnableKVMount_ComposesBaseNamespace(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/v1/sys/mounts/seed-kv", r.URL.Path)
		require.Equal(t, "admin/seed-ns-003", r.Header.Get("X-Vault-Namespace"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()

	client, err := vaultpkg.NewClient(vaultpkg.Config{Address: srv.URL, Token: "test-token", Namespace: "admin"})
	require.NoError(t, err)

	writer := NewVaultClientWriter(client)
	err = writer.EnableKVMount(context.Background(), "seed-ns-003", "seed-kv", 2)
	require.NoError(t, err)
}
