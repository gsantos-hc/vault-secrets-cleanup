package reporting

import (
	"fmt"
	"strconv"
	"time"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
)

func actionLastAccessAge(action *vpb.SecretAction) string {
	if action == nil {
		return "unknown"
	}

	if ts := action.GetLastAccessed(); ts != "" {
		parsed, err := time.Parse(time.RFC3339, ts)
		if err == nil {
			return formatPeriod(time.Since(parsed)) + " ago"
		}
	}

	if days := action.GetDaysSinceAccess(); days >= 0 {
		return fmt.Sprintf("%dd ago", days)
	}

	return "unknown"
}

func formatPeriod(duration time.Duration) string {
	if duration < 0 {
		duration = -duration
	}

	if duration < time.Minute {
		return "<1m"
	}

	if duration < time.Hour {
		minutes := int(duration.Round(time.Minute) / time.Minute)
		if minutes < 1 {
			minutes = 1
		}
		return strconv.Itoa(minutes) + "m"
	}

	if duration < 24*time.Hour {
		hours := int(duration.Round(time.Hour) / time.Hour)
		if hours < 1 {
			hours = 1
		}
		return strconv.Itoa(hours) + "h"
	}

	days := int(duration.Round(24*time.Hour) / (24 * time.Hour))
	if days < 1 {
		days = 1
	}
	return strconv.Itoa(days) + "d"
}
