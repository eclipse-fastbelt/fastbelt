// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package workspace

import (
	"context"
	"errors"
	iofs "io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	core "typefox.dev/fastbelt"
)

// newVirtualFileSystem returns a FileSystem that serves the "file" scheme from
// the returned in-memory provider.
func newVirtualFileSystem() (FileSystem, *VirtualFileSystemProvider) {
	files := NewVirtualFileSystemProvider()
	fs := NewDefaultFileSystem()
	fs.RegisterProvider(core.FileScheme, files)
	return fs, files
}

// walkVisited returns the sorted set of visited URIs,
// applying fn (may be nil) to control the walk.
func walkVisited(t *testing.T, fs FileSystem, root core.URI, fn func(DirEntry) error) []string {
	t.Helper()
	var visited []string
	err := WalkFileSystem(context.Background(), fs, root, func(e DirEntry) error {
		visited = append(visited, e.URI.String())
		if fn != nil {
			return fn(e)
		}
		return nil
	})
	require.NoError(t, err)
	sort.Strings(visited)
	return visited
}

func TestWalkFileSystemVisitsAll(t *testing.T) {
	fs, files := newVirtualFileSystem()
	require.NoError(t, files.WriteFile(core.FileURI("/a/b/c.txt"), []byte("c")))
	require.NoError(t, files.WriteFile(core.FileURI("/a/d.txt"), []byte("d")))

	visited := walkVisited(t, fs, core.FileURI("/a"), nil)
	assert.Equal(t, []string{
		"file:///a",
		"file:///a/b",
		"file:///a/b/c.txt",
		"file:///a/d.txt",
	}, visited)
}

func TestWalkFileSystemVisitsAllInAlphabeticalOrder(t *testing.T) {
	fs, files := newVirtualFileSystem()
	require.NoError(t, files.WriteFile(core.FileURI("/workspace/a.txt"), []byte("a")))
	require.NoError(t, files.WriteFile(core.FileURI("/workspace/d.txt"), []byte("d")))
	require.NoError(t, files.WriteFile(core.FileURI("/workspace/c.txt"), []byte("c")))
	require.NoError(t, files.WriteFile(core.FileURI("/workspace/b.txt"), []byte("b")))

	visited := walkVisited(t, fs, core.FileURI("/workspace"), nil)
	assert.Equal(t, []string{
		"file:///workspace",
		"file:///workspace/a.txt",
		"file:///workspace/b.txt",
		"file:///workspace/c.txt",
		"file:///workspace/d.txt",
	}, visited)
}

func TestWalkFileSystemSingleFile(t *testing.T) {
	fs, files := newVirtualFileSystem()
	require.NoError(t, files.WriteFile(core.FileURI("/a/b/c.txt"), []byte("c")))

	visited := walkVisited(t, fs, core.FileURI("/a/b/c.txt"), nil)
	assert.Equal(t, []string{"file:///a/b/c.txt"}, visited)
}

func TestWalkFileSystemSkipDir(t *testing.T) {
	fs, files := newVirtualFileSystem()
	require.NoError(t, files.WriteFile(core.FileURI("/a/b/c.txt"), []byte("c")))
	require.NoError(t, files.WriteFile(core.FileURI("/a/d.txt"), []byte("d")))

	// Skip directory b: its contents must not be visited, but siblings are.
	visited := walkVisited(t, fs, core.FileURI("/a"), func(e DirEntry) error {
		if e.URI.String() == "file:///a/b" {
			return SkipDir
		}
		return nil
	})
	assert.Equal(t, []string{
		"file:///a",
		"file:///a/b",
		"file:///a/d.txt",
	}, visited)
}

func TestWalkFileSystemSkipFile(t *testing.T) {
	fs, files := newVirtualFileSystem()
	require.NoError(t, files.WriteFile(core.FileURI("/a/b/c.txt"), []byte("c")))
	require.NoError(t, files.WriteFile(core.FileURI("/a/d.txt"), []byte("d")))
	require.NoError(t, files.WriteFile(core.FileURI("/a/e.txt"), []byte("e")))

	// Skip file d: will prevent further siblings from being visited
	visited := walkVisited(t, fs, core.FileURI("/a"), func(e DirEntry) error {
		if e.URI.String() == "file:///a/d.txt" {
			return SkipDir
		}
		return nil
	})
	assert.Equal(t, []string{
		"file:///a",
		"file:///a/b",
		"file:///a/b/c.txt",
		"file:///a/d.txt",
	}, visited)
}

func TestWalkFileSystemSkipAll(t *testing.T) {
	fs, files := newVirtualFileSystem()
	require.NoError(t, files.WriteFile(core.FileURI("/a/b/c.txt"), []byte("c")))
	require.NoError(t, files.WriteFile(core.FileURI("/a/d.txt"), []byte("d")))

	count := 0
	err := WalkFileSystem(context.Background(), fs, core.FileURI("/a"), func(e DirEntry) error {
		count++
		return SkipAll
	})
	require.NoError(t, err)
	assert.Equal(t, 1, count) // aborted after the root
}

func TestWalkFileSystemErrorPropagates(t *testing.T) {
	fs, files := newVirtualFileSystem()
	require.NoError(t, files.WriteFile(core.FileURI("/a/b/c.txt"), []byte("c")))

	sentinel := errors.New("boom")
	err := WalkFileSystem(context.Background(), fs, core.FileURI("/a"), func(e DirEntry) error {
		return sentinel
	})
	assert.ErrorIs(t, err, sentinel)
}

func TestWalkFileSystemMissingRoot(t *testing.T) {
	fs, _ := newVirtualFileSystem()
	err := WalkFileSystem(context.Background(), fs, core.FileURI("/nope"), func(e DirEntry) error {
		return nil
	})
	assert.Error(t, err)
}

func TestVirtualFileSystemWriteCreatesParents(t *testing.T) {
	ctx := context.Background()
	fs, files := newVirtualFileSystem()
	file := core.FileURI("/a/b/c.txt")

	require.NoError(t, files.WriteFile(file, []byte("hello")))

	content, err := fs.ReadFile(ctx, file)
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), content)

	// Intermediate parents are real, listable directories.
	dir := core.FileURI("/a/b")
	stat, err := fs.Stat(ctx, dir)
	require.NoError(t, err)
	assert.True(t, stat.IsDir)
}

func TestVirtualFileSystemReadDir(t *testing.T) {
	ctx := context.Background()
	fs, files := newVirtualFileSystem()
	require.NoError(t, files.WriteFile(core.FileURI("/a/b/c.txt"), []byte("hello")))

	entries, err := fs.ReadDir(ctx, core.FileURI("/a/b"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.True(t, entries[0].IsFile)
}

func TestVirtualFileSystemOverwrite(t *testing.T) {
	ctx := context.Background()
	fs, files := newVirtualFileSystem()
	file := core.FileURI("/a/b/c.txt")

	require.NoError(t, files.WriteFile(file, []byte("hello")))
	require.NoError(t, files.WriteFile(file, []byte("world")))

	content, err := fs.ReadFile(ctx, file)
	require.NoError(t, err)
	assert.Equal(t, []byte("world"), content)
}

func TestVirtualFileSystemMkdirExistingFails(t *testing.T) {
	fs, files := newVirtualFileSystem()
	require.NoError(t, files.WriteFile(core.FileURI("/a/b/c.txt"), []byte("hello")))

	assert.Error(t, files.Mkdir(core.FileURI("/a/b")))
	assert.True(t, fs.Exists(context.Background(), core.FileURI("/a/b")))
}

func TestVirtualFileSystemFileDirTypeMismatch(t *testing.T) {
	ctx := context.Background()
	fs, files := newVirtualFileSystem()
	require.NoError(t, files.WriteFile(core.FileURI("/a/b/c.txt"), []byte("hello")))
	dir := core.FileURI("/a/b")

	// Cannot overwrite a directory with a file.
	assert.Error(t, files.WriteFile(dir, []byte("x")))
	// Cannot read a directory as a file.
	_, err := fs.ReadFile(ctx, dir)
	assert.Error(t, err)
}

func TestVirtualFileSystemDeleteRecursive(t *testing.T) {
	ctx := context.Background()
	fs, files := newVirtualFileSystem()
	file := core.FileURI("/a/b/c.txt")
	require.NoError(t, files.WriteFile(file, []byte("hello")))

	require.NoError(t, files.Delete(core.FileURI("/a")))
	assert.False(t, fs.Exists(ctx, file))
	assert.False(t, fs.Exists(ctx, core.FileURI("/a/b")))
}

func TestVirtualFileSystemDeleteMissingFails(t *testing.T) {
	files := NewVirtualFileSystemProvider()
	assert.Error(t, files.Delete(core.FileURI("/a")))
}

func TestVirtualFileSystemDeleteRootClears(t *testing.T) {
	ctx := context.Background()
	fs, files := newVirtualFileSystem()
	require.NoError(t, files.WriteFile(core.FileURI("/a/b/c.txt"), []byte("hello")))

	require.NoError(t, files.Delete(core.FileURI("/")))
	assert.False(t, fs.Exists(ctx, core.FileURI("/a")))
	// The root itself remains and is usable again.
	assert.True(t, fs.Exists(ctx, core.FileURI("/")))
	require.NoError(t, files.WriteFile(core.FileURI("/x.txt"), []byte("x")))
	assert.True(t, fs.Exists(ctx, core.FileURI("/x.txt")))
}

func TestFileSystemDispatchesByScheme(t *testing.T) {
	ctx := context.Background()
	fs := NewDefaultFileSystem()
	disk := NewVirtualFileSystemProvider()
	memory := NewVirtualFileSystemProvider()
	fs.RegisterProvider("file", disk)
	fs.RegisterProvider("memory", memory)

	fileURI := core.ParseURI("file:///a/x.txt")
	memoryURI := core.ParseURI("memory:///a/x.txt")
	require.NoError(t, disk.WriteFile(fileURI, []byte("from disk")))
	require.NoError(t, memory.WriteFile(memoryURI, []byte("from memory")))
	require.NoError(t, memory.WriteFile(core.ParseURI("memory:///a/only.txt"), nil))

	content, err := fs.ReadFile(ctx, fileURI)
	require.NoError(t, err)
	assert.Equal(t, "from disk", string(content))

	content, err = fs.ReadFile(ctx, memoryURI)
	require.NoError(t, err)
	assert.Equal(t, "from memory", string(content))

	// Contents of the providers are kept apart.
	assert.True(t, fs.Exists(ctx, core.ParseURI("memory:///a/only.txt")))
	assert.False(t, fs.Exists(ctx, core.ParseURI("file:///a/only.txt")))

	// Entries keep the scheme they were requested with.
	entries, err := fs.ReadDir(ctx, core.ParseURI("memory:///a"))
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "memory", entries[0].URI.Scheme())
	assert.Equal(t, "/a/only.txt", entries[0].URI.Path())
	assert.Equal(t, "memory", entries[1].URI.Scheme())
	assert.Equal(t, "/a/x.txt", entries[1].URI.Path())
}

func TestFileSystemUnknownScheme(t *testing.T) {
	ctx := context.Background()
	fs := NewDefaultFileSystem()
	uri := core.ParseURI("memory:///a/x.txt")

	assert.False(t, fs.Exists(ctx, uri))
	_, err := fs.Stat(ctx, uri)
	assert.ErrorIs(t, err, iofs.ErrNotExist)
	_, err = fs.ReadFile(ctx, uri)
	assert.ErrorIs(t, err, iofs.ErrNotExist)
	_, err = fs.ReadDir(ctx, uri)
	assert.ErrorIs(t, err, iofs.ErrNotExist)
}

func TestFileSystemRegisterProviderReplaces(t *testing.T) {
	ctx := context.Background()
	fs := NewDefaultFileSystem()
	uri := core.ParseURI("memory:///x.txt")
	first := NewVirtualFileSystemProvider()
	second := NewVirtualFileSystemProvider()
	require.NoError(t, first.WriteFile(uri, []byte("first")))

	unregisterFirst := fs.RegisterProvider("memory", first)
	assert.True(t, fs.Exists(ctx, uri))

	unregisterSecond := fs.RegisterProvider("memory", second)
	assert.False(t, fs.Exists(ctx, uri))

	// Unregistering a replaced provider must not remove its replacement.
	unregisterFirst()
	require.NoError(t, second.WriteFile(uri, []byte("second")))
	assert.True(t, fs.Exists(ctx, uri))

	unregisterSecond()
	assert.False(t, fs.Exists(ctx, uri))
}

func TestDiskFileSystemReadsLocalFiles(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))

	fs := NewDiskFileSystem()
	root := core.FileURI(dir)

	stat, err := fs.Stat(ctx, root)
	require.NoError(t, err)
	assert.True(t, stat.IsDir)

	content, err := fs.ReadFile(ctx, root.JoinPath("a.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(content))

	entries, err := fs.ReadDir(ctx, root)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.True(t, entries[0].IsFile)
	assert.True(t, entries[1].IsDir)

	assert.False(t, fs.Exists(ctx, root.JoinPath("missing.txt")))
	// Other schemes are not served from disk.
	assert.False(t, fs.Exists(ctx, core.ParseURI("memory:///a.txt")))
}
