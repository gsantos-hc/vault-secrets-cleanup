package seeding

import "math/rand"

type AllocationInput struct {
	NamespaceCount int
	TotalSecrets   int
	KV2Probability float64
	RNG            *rand.Rand
}

type NamespacePlan struct {
	Name   string
	Mounts []MountPlan
}

type MountPlan struct {
	Name        string
	KVVersion   int
	SecretCount int
}
