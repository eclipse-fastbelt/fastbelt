// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package grammar

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/jsonrpc2"
	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/server"
	"typefox.dev/fastbelt/util/service"
	"typefox.dev/fastbelt/workspace"
)

type binderFunc func(ctx context.Context, conn *jsonrpc2.Connection) (jsonrpc2.ConnectionOptions, error)

func (f binderFunc) Bind(ctx context.Context, conn *jsonrpc2.Connection) (jsonrpc2.ConnectionOptions, error) {
	return f(ctx, conn)
}

// fileSystemClient answers the file system requests like an LSP client
// would, backed by an in-memory file system. Entries listed in symlinks are
// reported as symbolic links.
type fileSystemClient struct {
	fs       *workspace.VirtualFileSystem
	symlinks map[string]bool
}

func (c *fileSystemClient) fileType(entry workspace.DirEntry) FileType {
	fileType := FileTypeFile
	if entry.IsDir {
		fileType = FileTypeDirectory
	}
	if c.symlinks[entry.URI.String()] {
		fileType |= FileTypeSymbolicLink
	}
	return fileType
}

func (c *fileSystemClient) Handle(ctx context.Context, req *jsonrpc2.Request) (any, error) {
	var params FileSystemParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, err
	}
	uri := core.ParseURI(params.URI)
	switch req.Method {
	case FileSystemStatMethod:
		entry, err := c.fs.Stat(ctx, uri)
		if err != nil {
			return nil, err
		}
		return FileSystemStatResult{Type: c.fileType(entry)}, nil
	case FileSystemReadFileMethod:
		content, err := c.fs.ReadFile(ctx, uri)
		if err != nil {
			return nil, err
		}
		return FileSystemReadFileResult{Content: content}, nil
	case FileSystemReadDirectoryMethod:
		entries, err := c.fs.ReadDir(ctx, uri)
		if err != nil {
			return nil, err
		}
		result := FileSystemReadDirectoryResult{Entries: []FileSystemDirectoryEntry{}}
		for _, entry := range entries {
			name := entry.URI.Path()[len(uri.Path())+1:]
			result.Entries = append(result.Entries, FileSystemDirectoryEntry{Name: name, Type: c.fileType(entry)})
		}
		return result, nil
	}
	return nil, jsonrpc2.ErrMethodNotFound
}

// connectFileSystemClient creates a [ClientFileSystem] whose connection is
// served by client.
func connectFileSystemClient(t *testing.T, client *fileSystemClient) workspace.FileSystem {
	t.Helper()
	ctx := context.Background()
	listener, err := jsonrpc2.NetPipe(ctx)
	require.NoError(t, err)
	clientSide, err := jsonrpc2.Serve(ctx, listener, binderFunc(func(context.Context, *jsonrpc2.Connection) (jsonrpc2.ConnectionOptions, error) {
		return jsonrpc2.ConnectionOptions{Handler: jsonrpc2.HandlerFunc(client.Handle)}, nil
	}))
	require.NoError(t, err)

	sc := service.NewContainer()
	sc.Put(&server.Connection{})
	sc.Seal()
	fs := NewClientFileSystem(sc)

	// Without a connection, every operation fails.
	_, err = fs.Stat(ctx, core.ParseURI("file:///ws"))
	require.Error(t, err)

	conn, err := jsonrpc2.Dial(ctx, listener.Dialer(), binderFunc(func(context.Context, *jsonrpc2.Connection) (jsonrpc2.ConnectionOptions, error) {
		return jsonrpc2.ConnectionOptions{}, nil
	}))
	require.NoError(t, err)
	sc.MustGet[*server.Connection]().Value = conn
	t.Cleanup(func() {
		_ = conn.Close()
		_ = listener.Close()
		_ = clientSide.Wait()
	})
	return fs
}

func TestClientFileSystem(t *testing.T) {
	vfs := workspace.NewVirtualFileSystem()
	file := core.ParseURI("file:///ws/my grammar.fb")
	binary := core.ParseURI("file:///ws/sub/data.bin")
	link := core.ParseURI("file:///ws/link")
	require.NoError(t, vfs.WriteFile(file, []byte("grammar Ünicode;")))
	require.NoError(t, vfs.WriteFile(binary, []byte{0, 0xff, 0xfe, '\n'}))
	require.NoError(t, vfs.Mkdir(link))
	fs := connectFileSystemClient(t, &fileSystemClient{fs: vfs, symlinks: map[string]bool{link.String(): true}})
	ctx := context.Background()
	root := core.ParseURI("file:///ws")

	entry, err := fs.Stat(ctx, root)
	require.NoError(t, err)
	assert.Equal(t, workspace.DirEntry{IsDir: true, URI: root}, entry)

	entry, err = fs.Stat(ctx, file)
	require.NoError(t, err)
	assert.Equal(t, workspace.DirEntry{IsFile: true, URI: file}, entry)

	assert.True(t, fs.Exists(ctx, file))
	assert.False(t, fs.Exists(ctx, core.ParseURI("file:///ws/missing.fb")))
	_, err = fs.ReadFile(ctx, core.ParseURI("file:///ws/missing.fb"))
	require.Error(t, err)

	content, err := fs.ReadFile(ctx, file)
	require.NoError(t, err)
	assert.Equal(t, "grammar Ünicode;", string(content))

	content, err = fs.ReadFile(ctx, binary)
	require.NoError(t, err)
	assert.Equal(t, []byte{0, 0xff, 0xfe, '\n'}, content)

	entries, err := fs.ReadDir(ctx, root)
	require.NoError(t, err)
	require.Len(t, entries, 3)
	// A symbolic link is neither file nor directory, like on disk.
	assert.Equal(t, workspace.DirEntry{IsSymlink: true}, withoutURI(entries[0]))
	assert.True(t, link.Equal(entries[0].URI))
	assert.Equal(t, workspace.DirEntry{IsFile: true}, withoutURI(entries[1]))
	assert.True(t, file.Equal(entries[1].URI), "got %s", entries[1].URI)
	assert.Equal(t, workspace.DirEntry{IsDir: true}, withoutURI(entries[2]))

	_, err = fs.ReadDir(ctx, core.ParseURI("file:///nowhere"))
	require.Error(t, err)
}

func withoutURI(entry workspace.DirEntry) workspace.DirEntry {
	entry.URI = nil
	return entry
}

// The initializer must be able to load a workspace folder through the client.
func TestClientFileSystemWalk(t *testing.T) {
	vfs := workspace.NewVirtualFileSystem()
	require.NoError(t, vfs.WriteFile(core.ParseURI("vscode-vfs://github/ws/a.fb"), []byte("a")))
	require.NoError(t, vfs.WriteFile(core.ParseURI("vscode-vfs://github/ws/sub/b.fb"), []byte("b")))
	fs := connectFileSystemClient(t, &fileSystemClient{fs: vfs})

	var visited []string
	err := workspace.WalkFileSystem(context.Background(), fs, core.ParseURI("vscode-vfs://github/ws"), func(entry workspace.DirEntry) error {
		if entry.IsFile {
			visited = append(visited, entry.URI.String())
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"vscode-vfs://github/ws/a.fb", "vscode-vfs://github/ws/sub/b.fb"}, visited)

	err = workspace.WalkFileSystem(context.Background(), fs, core.ParseURI("vscode-vfs://github/missing"), func(workspace.DirEntry) error { return nil })
	require.Error(t, err)
	assert.NotErrorIs(t, err, os.ErrNotExist, "errors of the client are opaque")
}
