package seeding

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	vaultpkg "github.com/gsantos-hc/vault-secrets-cleanup/internal/vault"
	"github.com/stretchr/testify/require"
)

func TestComposeNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		base    string
		child   string
		want    string
		wantErr bool
	}{
		{name: "empty base returns child", base: "", child: "ns", want: "ns"},
		{name: "empty child returns base", base: "admin", child: "", want: "admin"},
		{name: "joins base and child", base: "admin", child: "ns", want: "admin/ns"},
		{name: "child already has base prefix", base: "admin", child: "admin/ns", want: "admin/ns"},
		{name: "child equals base", base: "admin", child: "admin", want: "admin"},
		{name: "both empty returns empty", base: "", child: "", want: ""},
		{name: "traversal with leading dot-dot", base: "admin", child: "../other", wantErr: true},
		{name: "traversal in middle of path", base: "admin", child: "foo/../other", wantErr: true},
		{name: "traversal only dot-dot", base: "admin", child: "..", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := composeNamespace(tc.base, tc.child)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

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
