// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package seeding

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllocate_EnforcesExactTotalAndNamespaceCount(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	plan, err := Allocate(AllocationInput{
		NamespaceCount: 5,
		TotalSecrets:   100,
		KV2Probability: 0.9,
		RNG:            rng,
	})
	require.NoError(t, err)
	require.Len(t, plan, 5)

	total := 0
	for _, ns := range plan {
		require.NotEmpty(t, ns.Name)
		require.LessOrEqual(t, len(ns.Mounts), 5)
		nsTotal := 0
		for _, m := range ns.Mounts {
			require.NotEmpty(t, m.Name)
			require.GreaterOrEqual(t, m.SecretCount, 0)
			require.Contains(t, []int{1, 2}, m.KVVersion)
			nsTotal += m.SecretCount
		}
		if len(ns.Mounts) == 0 {
			require.Equal(t, 0, nsTotal)
		}
		total += nsTotal
	}

	require.Equal(t, 100, total)
}

func TestAllocate_IsDeterministicWithSeed(t *testing.T) {
	in := AllocationInput{
		NamespaceCount: 3,
		TotalSecrets:   20,
		KV2Probability: 0.9,
	}

	leftRNG := rand.New(rand.NewSource(99))
	rightRNG := rand.New(rand.NewSource(99))

	left, err := Allocate(AllocationInput{
		NamespaceCount: in.NamespaceCount,
		TotalSecrets:   in.TotalSecrets,
		KV2Probability: in.KV2Probability,
		RNG:            leftRNG,
	})
	require.NoError(t, err)

	right, err := Allocate(AllocationInput{
		NamespaceCount: in.NamespaceCount,
		TotalSecrets:   in.TotalSecrets,
		KV2Probability: in.KV2Probability,
		RNG:            rightRNG,
	})
	require.NoError(t, err)

	require.Equal(t, left, right)
}

func TestAllocate_RejectsInvalidInput(t *testing.T) {
	_, err := Allocate(AllocationInput{NamespaceCount: 0, TotalSecrets: 10, KV2Probability: 0.9})
	require.ErrorContains(t, err, "namespace count")

	_, err = Allocate(AllocationInput{NamespaceCount: 1, TotalSecrets: 0, KV2Probability: 0.9})
	require.ErrorContains(t, err, "total secrets")

	_, err = Allocate(AllocationInput{NamespaceCount: 1, TotalSecrets: 1, KV2Probability: 2})
	require.ErrorContains(t, err, "kv2 probability")
}

func TestAllocate_MountCountIsAlwaysBetweenZeroAndTwenty(t *testing.T) {
	for seed := int64(1); seed <= 200; seed++ {
		plan, err := Allocate(AllocationInput{
			NamespaceCount: 1,
			TotalSecrets:   200,
			KV2Probability: 0.9,
			RNG:            rand.New(rand.NewSource(seed)),
		})
		require.NoError(t, err)
		require.Len(t, plan, 1)
		require.LessOrEqual(t, len(plan[0].Mounts), 5)
	}
}
