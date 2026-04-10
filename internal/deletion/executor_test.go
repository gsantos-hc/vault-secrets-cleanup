package deletion

import (
	"context"
	"errors"
	"testing"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/retry"
	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/stretchr/testify/require"
)

type mockSecretDeleter struct {
	kvVersion int
	calls     []string
}

func (m *mockSecretDeleter) KVVersion(_ context.Context, namespace, mountPath string) (int, error) {
	m.calls = append(m.calls, "kv-version:"+namespace+":"+mountPath)
	return m.kvVersion, nil
}

func (m *mockSecretDeleter) DeleteKVv1(_ context.Context, namespace, mountPath, secretPath string) error {
	m.calls = append(m.calls, "kv1:"+namespace+":"+mountPath+":"+secretPath)
	return nil
}

func (m *mockSecretDeleter) DeleteKVv2(_ context.Context, namespace, mountPath, secretPath string) error {
	m.calls = append(m.calls, "kv2:"+namespace+":"+mountPath+":"+secretPath)
	return nil
}

func TestExecutor_Delete_DryRun(t *testing.T) {
	deleter := &mockSecretDeleter{kvVersion: 2}
	exec := NewExecutor(ExecutorConfig{Deleter: deleter, RetryConfig: retry.DefaultConfig(), DryRun: true})

	action := &vpb.SecretAction{NamespacePath: "team-a", MountPath: "secret/", SecretPath: "app/key"}
	require.NoError(t, exec.Delete(context.Background(), action))
	require.Empty(t, deleter.calls)
}

func TestExecutor_Delete_KVv1(t *testing.T) {
	deleter := &mockSecretDeleter{kvVersion: 1}
	exec := NewExecutor(ExecutorConfig{Deleter: deleter, RetryConfig: retry.DefaultConfig()})

	action := &vpb.SecretAction{NamespacePath: "team-a", MountPath: "kv1/", SecretPath: "app/key"}
	require.NoError(t, exec.Delete(context.Background(), action))
	require.Contains(t, deleter.calls, "kv1:team-a:kv1/:app/key")
}

func TestExecutor_Delete_KVv2(t *testing.T) {
	deleter := &mockSecretDeleter{kvVersion: 2}
	exec := NewExecutor(ExecutorConfig{Deleter: deleter, RetryConfig: retry.DefaultConfig()})

	action := &vpb.SecretAction{NamespacePath: "team-a", MountPath: "secret/", SecretPath: "app/key"}
	require.NoError(t, exec.Delete(context.Background(), action))
	require.Contains(t, deleter.calls, "kv2:team-a:secret/:app/key")
}

func TestExecutor_Delete_UnknownKVVersion(t *testing.T) {
	deleter := &mockSecretDeleter{kvVersion: 9}
	exec := NewExecutor(ExecutorConfig{Deleter: deleter, RetryConfig: retry.DefaultConfig()})

	action := &vpb.SecretAction{NamespacePath: "team-a", MountPath: "secret/", SecretPath: "app/key"}
	err := exec.Delete(context.Background(), action)
	require.Error(t, err)
}

func TestExecutor_Delete_RetriesRetryableError(t *testing.T) {
	attempts := 0
	deleter := &mockSecretDeleterWithFailures{
		kvVersion: 2,
		deleteKVv2Fn: func() error {
			attempts++
			if attempts < 3 {
				return errors.New("503 temporary")
			}
			return nil
		},
	}
	exec := NewExecutor(ExecutorConfig{Deleter: deleter, RetryConfig: retry.Config{MaxAttempts: 3}})

	action := &vpb.SecretAction{NamespacePath: "team-a", MountPath: "secret/", SecretPath: "app/key"}
	require.NoError(t, exec.Delete(context.Background(), action))
	require.Equal(t, 3, attempts)
}

type mockSecretDeleterWithFailures struct {
	kvVersion    int
	deleteKVv2Fn func() error
}

func (m *mockSecretDeleterWithFailures) KVVersion(_ context.Context, _ string, _ string) (int, error) {
	return m.kvVersion, nil
}

func (m *mockSecretDeleterWithFailures) DeleteKVv1(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

func (m *mockSecretDeleterWithFailures) DeleteKVv2(_ context.Context, _ string, _ string, _ string) error {
	if m.deleteKVv2Fn != nil {
		return m.deleteKVv2Fn()
	}
	return nil
}
