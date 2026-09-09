// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package audit

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ulikunitz/xz"
)

func TestOpenFile_Plain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0o644))

	r, err := OpenFile(path)
	require.NoError(t, err)
	defer r.Close()

	data, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, "hello", string(data))
}

func TestOpenFile_Gzip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log.gz")
	f, err := os.Create(path)
	require.NoError(t, err)
	zw := gzip.NewWriter(f)
	_, err = zw.Write([]byte("hello gzip"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.NoError(t, f.Close())

	r, err := OpenFile(path)
	require.NoError(t, err)
	defer r.Close()

	data, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, "hello gzip", string(data))
}

func TestOpenFile_XZ(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log.xz")
	f, err := os.Create(path)
	require.NoError(t, err)
	zw, err := xz.NewWriter(f)
	require.NoError(t, err)
	_, err = zw.Write([]byte("hello xz"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.NoError(t, f.Close())

	r, err := OpenFile(path)
	require.NoError(t, err)
	defer r.Close()

	data, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, "hello xz", string(data))
}
