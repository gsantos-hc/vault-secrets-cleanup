// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package seeding

import (
	"errors"
	"fmt"
	"math/rand"
)

const maxMountsPerNamespace = 5

func Allocate(in AllocationInput) ([]NamespacePlan, error) {
	if in.NamespaceCount <= 0 {
		return nil, errors.New("namespace count must be > 0")
	}
	if in.TotalSecrets <= 0 {
		return nil, errors.New("total secrets must be > 0")
	}
	if in.KV2Probability < 0 || in.KV2Probability > 1 {
		return nil, errors.New("kv2 probability must be between 0 and 1")
	}

	rng := in.RNG
	if rng == nil {
		rng = rand.New(rand.NewSource(rand.Int63()))
	}

	mountCounts := randomMountCounts(rng, in.NamespaceCount)
	if in.TotalSecrets > 0 && allZero(mountCounts) {
		mountCounts[rng.Intn(len(mountCounts))] = 1
	}

	nsSecretCounts := allocateSecretsAcrossMountableNamespaces(rng, in.TotalSecrets, mountCounts)
	result := make([]NamespacePlan, 0, in.NamespaceCount)
	for i := 0; i < in.NamespaceCount; i++ {
		nsName := fmt.Sprintf("seed-ns-%03d", i+1)
		nsSecrets := nsSecretCounts[i]
		mountCount := mountCounts[i]
		if mountCount == 0 {
			result = append(result, NamespacePlan{Name: nsName, Mounts: nil})
			continue
		}

		mountSecretCounts := allocateMountSecrets(rng, nsSecrets, mountCount)
		mounts := make([]MountPlan, 0, mountCount)
		for m := 0; m < mountCount; m++ {
			mounts = append(mounts, MountPlan{
				Name:        fmt.Sprintf("seed-mount-%03d-%03d", i+1, m+1),
				KVVersion:   pickKVVersion(rng, in.KV2Probability),
				SecretCount: mountSecretCounts[m],
			})
		}

		result = append(result, NamespacePlan{Name: nsName, Mounts: mounts})
	}

	return result, nil
}

func pickKVVersion(rng *rand.Rand, kv2Probability float64) int {
	if rng.Float64() < kv2Probability {
		return 2
	}
	return 1
}

func randomMountCounts(rng *rand.Rand, namespaces int) []int {
	counts := make([]int, namespaces)
	for i := 0; i < namespaces; i++ {
		counts[i] = rng.Intn(maxMountsPerNamespace + 1)
	}
	return counts
}

func allZero(values []int) bool {
	for _, value := range values {
		if value != 0 {
			return false
		}
	}
	return true
}

func allocateSecretsAcrossMountableNamespaces(rng *rand.Rand, total int, mountCounts []int) []int {
	parts := make([]int, len(mountCounts))
	if total == 0 {
		return parts
	}

	eligible := make([]int, 0, len(mountCounts))
	for i, count := range mountCounts {
		if count > 0 {
			eligible = append(eligible, i)
		}
	}

	if len(eligible) == 0 {
		return parts
	}

	for i := 0; i < total; i++ {
		nsIndex := eligible[rng.Intn(len(eligible))]
		parts[nsIndex]++
	}

	return parts
}

func allocateMountSecrets(rng *rand.Rand, total int, mounts int) []int {
	if total == 0 {
		return make([]int, mounts)
	}
	if total >= mounts {
		return allocatePositiveParts(rng, total, mounts)
	}

	parts := make([]int, mounts)
	for i := 0; i < total; i++ {
		parts[rng.Intn(mounts)]++
	}
	return parts
}

func allocatePositiveParts(rng *rand.Rand, total int, parts int) []int {
	result := make([]int, parts)
	for i := range result {
		result[i] = 1
	}

	remaining := total - parts
	for i := 0; i < remaining; i++ {
		result[rng.Intn(parts)]++
	}
	return result
}
