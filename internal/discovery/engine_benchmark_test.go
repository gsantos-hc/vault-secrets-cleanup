// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package discovery

import (
	"fmt"
	"testing"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
)

func BenchmarkEngine_calculateStats(b *testing.B) {
	inventory := buildBenchmarkInventory(50, 4, 200)
	engine := NewEngine(Config{})

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		engine.calculateStats(inventory)
	}
}

func buildBenchmarkInventory(namespaces, mountsPerNamespace, secretsPerMount int) *vpb.Inventory {
	result := &vpb.Inventory{Namespaces: make([]*vpb.Namespace, 0, namespaces)}
	for i := 0; i < namespaces; i++ {
		ns := &vpb.Namespace{Path: fmt.Sprintf("ns-%d", i)}
		for j := 0; j < mountsPerNamespace; j++ {
			mount := &vpb.Mount{Path: fmt.Sprintf("secret-%d/", j), Version: 2}
			for k := 0; k < secretsPerMount; k++ {
				mount.Secrets = append(mount.Secrets, &vpb.Secret{Path: fmt.Sprintf("app/%d/%d", j, k)})
			}
			ns.Mounts = append(ns.Mounts, mount)
		}
		result.Namespaces = append(result.Namespaces, ns)
	}
	return result
}
