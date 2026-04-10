package planning

import (
	"testing"
	"time"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/stretchr/testify/require"
)

func TestStalenessCalculatorCalculate(t *testing.T) {
	calc := NewStalenessCalculator(StalenessConfig{
		DefaultPeriod: "24h",
		NamespacePolicies: map[string]string{
			"prod/*": "5m",
		},
	})

	secret := &vpb.Secret{Path: "app/config"}
	ns := &vpb.Namespace{Path: "prod/team-a"}
	mount := &vpb.Mount{Path: "secret/"}

	t.Run("unknown when no access", func(t *testing.T) {
		category, reason, days := calc.Calculate(secret, ns, mount, nil)
		require.Equal(t, "unknown", category)
		require.Contains(t, reason, "No access record")
		require.Equal(t, -1, days)
	})

	t.Run("stale when over threshold", func(t *testing.T) {
		access := &vpb.AccessRecord{LastAccessed: time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339)}
		category, reason, days := calc.Calculate(secret, ns, mount, access)
		require.Equal(t, "stale", category)
		require.Equal(t, 0, days)
		require.Equal(t, "Not accessed in 10m (threshold: 5m)", reason)
	})

	t.Run("active when within threshold", func(t *testing.T) {
		access := &vpb.AccessRecord{LastAccessed: time.Now().UTC().Add(-3 * time.Minute).Format(time.RFC3339)}
		category, reason, days := calc.Calculate(secret, &vpb.Namespace{Path: "dev/team-a"}, mount, access)
		require.Equal(t, "active", category)
		require.GreaterOrEqual(t, days, 0)
		require.Equal(t, "Accessed 3m ago", reason)
	})
}

func TestParsePeriod(t *testing.T) {
	t.Run("supports day suffix", func(t *testing.T) {
		d, err := parsePeriod("365d")
		require.NoError(t, err)
		require.Equal(t, 365*24*time.Hour, d)
	})

	t.Run("supports go duration syntax", func(t *testing.T) {
		d, err := parsePeriod("5m")
		require.NoError(t, err)
		require.Equal(t, 5*time.Minute, d)
	})

	t.Run("fails for invalid values", func(t *testing.T) {
		_, err := parsePeriod("wat")
		require.Error(t, err)
	})
}
