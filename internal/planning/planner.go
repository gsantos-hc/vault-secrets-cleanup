package planning

import (
	"time"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/correlation"
	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
)

type Config struct {
	StalenessConfig StalenessConfig
	ExclusionConfig ExclusionConfig
	IncludeUnknown  bool
}

type Planner struct {
	stalenessCalc  *StalenessCalculator
	exclusionRules *ExclusionEngine
	includeUnknown bool
}

func NewPlanner(config Config) *Planner {
	return &Planner{
		stalenessCalc:  NewStalenessCalculator(config.StalenessConfig),
		exclusionRules: NewExclusionEngine(config.ExclusionConfig),
		includeUnknown: config.IncludeUnknown,
	}
}

func (p *Planner) GeneratePlan(inventory *vpb.Inventory, accessMap map[string]*vpb.AccessRecord, cfg *vpb.PlanConfig) (*vpb.DeletionPlan, error) {
	plan := &vpb.DeletionPlan{
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Config:    cfg,
		Actions:   []*vpb.SecretAction{},
		Stats:     &vpb.PlanStats{},
	}

	if inventory == nil {
		return plan, nil
	}

	for _, namespace := range inventory.Namespaces {
		for _, mount := range namespace.Mounts {
			for _, secret := range mount.Secrets {
				action := &vpb.SecretAction{
					NamespacePath: namespace.Path,
					MountPath:     mount.Path,
					MountAccessor: mount.Accessor,
					SecretPath:    secret.Path,
					Status:        "pending",
				}

				excluded, reason := p.exclusionRules.IsExcluded(secret, namespace, mount)
				if excluded {
					action.Category = "excluded"
					action.Reason = reason
					plan.Actions = append(plan.Actions, action)
					p.updateStats(plan.Stats, action)
					continue
				}

				access := accessMap[correlation.BuildKey(namespace, mount, secret)]
				category, reason, days := p.stalenessCalc.Calculate(secret, namespace, mount, access)
				action.Category = category
				action.Reason = reason
				action.DaysSinceAccess = int32(days)
				if access != nil {
					action.LastAccessed = access.LastAccessed
				}

				plan.Actions = append(plan.Actions, action)
				p.updateStats(plan.Stats, action)
			}
		}
	}

	return plan, nil
}

func (p *Planner) updateStats(stats *vpb.PlanStats, action *vpb.SecretAction) {
	stats.TotalSecrets++

	switch action.Category {
	case "stale":
		stats.StaleCount++
		stats.ToDeleteCount++
	case "unknown":
		stats.UnknownCount++
		if p.includeUnknown {
			stats.ToDeleteCount++
		}
	case "active":
		stats.ActiveCount++
	case "excluded":
		stats.ExcludedCount++
	}
}
