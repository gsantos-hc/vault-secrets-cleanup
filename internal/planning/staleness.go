package planning

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
)

type StalenessConfig struct {
	DefaultPeriod     string
	NamespacePolicies map[string]string
}

type StalenessCalculator struct {
	defaultPeriod     time.Duration
	namespacePolicies map[string]time.Duration
}

func NewStalenessCalculator(config StalenessConfig) *StalenessCalculator {
	defaultPeriod, err := parsePeriod(config.DefaultPeriod)
	if err != nil {
		defaultPeriod = 365 * 24 * time.Hour
	}

	policies := config.NamespacePolicies
	if policies == nil {
		policies = map[string]string{}
	}

	parsedPolicies := map[string]time.Duration{}
	for pattern, rawPeriod := range policies {
		period, err := parsePeriod(rawPeriod)
		if err != nil {
			continue
		}
		parsedPolicies[pattern] = period
	}

	return &StalenessCalculator{defaultPeriod: defaultPeriod, namespacePolicies: parsedPolicies}
}

func (s *StalenessCalculator) Calculate(secret *vpb.Secret, namespace *vpb.Namespace, mount *vpb.Mount, access *vpb.AccessRecord) (string, string, int) {
	_ = secret
	_ = mount

	if access == nil {
		return "unknown", "No access record found in audit logs", -1
	}

	lastAccessed, err := time.Parse(time.RFC3339, access.LastAccessed)
	if err != nil {
		return "unknown", fmt.Sprintf("Invalid timestamp: %s", access.LastAccessed), -1
	}

	elapsed := time.Since(lastAccessed)
	days := int(elapsed.Hours() / 24)
	threshold := s.thresholdForNamespace(namespace.GetPath())
	if elapsed > threshold {
		return "stale", fmt.Sprintf("Not accessed in %d days (threshold: %s)", days, threshold), days
	}

	return "active", fmt.Sprintf("Accessed %d days ago", days), days
}

func (s *StalenessCalculator) thresholdForNamespace(namespacePath string) time.Duration {
	if threshold, ok := s.namespacePolicies[namespacePath]; ok {
		return threshold
	}

	for pattern, threshold := range s.namespacePolicies {
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(namespacePath, prefix) {
				return threshold
			}
		}
	}

	return s.defaultPeriod
}

func parsePeriod(raw string) (time.Duration, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, errors.New("period cannot be empty")
	}

	if strings.HasSuffix(value, "d") {
		daysRaw := strings.TrimSuffix(value, "d")
		days, err := strconv.Atoi(daysRaw)
		if err != nil || days <= 0 {
			return 0, fmt.Errorf("invalid day period %q", raw)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	if duration <= 0 {
		return 0, fmt.Errorf("duration must be > 0")
	}

	return duration, nil
}
