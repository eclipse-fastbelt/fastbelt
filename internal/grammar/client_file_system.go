// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package grammar

import (
	"context"
	"errors"
	"fmt"

	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/server"
	"typefox.dev/fastbelt/util/service"
	"typefox.dev/fastbelt/workspace"
	"typefox.dev/lsp"
)

// Note: This file system implementation via custom LSP requests can be removed,
// when the LSP feature for this is available: https://github.com/microsoft/vscode-languageserver-node/pull/1825

// The JSON-RPC methods of the requests a [ClientFileSystem] sends to the
// client. They are not part of the LSP specification, so the client has to
// register a handler for each of them.
const (
	// FileSystemStatMethod takes [FileSystemParams] and returns a [FileSystemStatResult].
	FileSystemStatMethod = "fastbelt/fs/stat"
	// FileSystemReadFileMethod takes [FileSystemParams] and returns a [FileSystemReadFileResult].
	FileSystemReadFileMethod = "fastbelt/fs/readFile"
	// FileSystemReadDirectoryMethod takes [FileSystemParams] and returns a [FileSystemReadDirectoryResult].
	FileSystemReadDirectoryMethod = "fastbelt/fs/readDirectory"
)

// FileType describes a file system entry of the client. It is a bit set with
// the values of the FileType enumeration of the VS Code API: a symbolic link
// has [FileTypeSymbolicLink] set in addition to the type of its target.
type FileType uint32

const (
	// FileTypeUnknown is the type of an entry that is neither a file nor a directory.
	FileTypeUnknown FileType = 0
	// FileTypeFile marks a regular file.
	FileTypeFile FileType = 1
	// FileTypeDirectory marks a directory.
	FileTypeDirectory FileType = 2
	// FileTypeSymbolicLink marks a symbolic link.
	FileTypeSymbolicLink FileType = 64
)

// FileSystemParams are the parameters of all file system requests.
type FileSystemParams struct {
	// URI identifies the file or directory to access.
	URI lsp.URI `json:"uri"`
}

// FileSystemStatResult is the result of a [FileSystemStatMethod] request.
// The client answers with an error if the entry does not exist.
type FileSystemStatResult struct {
	Type FileType `json:"type"`
}

// FileSystemReadFileResult is the result of a [FileSystemReadFileMethod] request.
type FileSystemReadFileResult struct {
	// Content holds the bytes of the file, base64-encoded in JSON.
	Content []byte `json:"content"`
}

// FileSystemReadDirectoryResult is the result of a [FileSystemReadDirectoryMethod] request.
type FileSystemReadDirectoryResult struct {
	Entries []FileSystemDirectoryEntry `json:"entries"`
}

// FileSystemDirectoryEntry is a direct child of a directory.
type FileSystemDirectoryEntry struct {
	// Name is the name of the entry within its directory.
	Name string   `json:"name"`
	Type FileType `json:"type"`
}

// ClientFileSystem is a [workspace.FileSystem] that reads from the file
// system of the LSP client. Each operation sends one request over the
// [server.Connection] and waits for its response; see [FileSystemStatMethod],
// [FileSystemReadFileMethod], and [FileSystemReadDirectoryMethod].
//
// It serves language servers that cannot access the files themselves, such as
// a js/wasm server running in a Web Worker, and works for every URI scheme
// the client can read. As on disk, a symbolic link is reported as neither
// file nor directory, so walking a directory does not follow links.
//
// The file system can only be used once the connection is established, that
// is from the initialize request on.
type ClientFileSystem struct {
	sc *service.Container
}

// NewClientFileSystem creates a [ClientFileSystem]. It replaces the default
// [workspace.FileSystem] in the js/wasm build of the grammar language server.
func NewClientFileSystem(sc *service.Container) workspace.FileSystem {
	return &ClientFileSystem{sc: sc}
}

// Exists checks if the client has a file system entry for the given URI.
func (s *ClientFileSystem) Exists(ctx context.Context, uri core.URI) bool {
	_, err := s.Stat(ctx, uri)
	return err == nil
}

// Stat returns the file system entry for the given URI, or an error if the client cannot find it.
func (s *ClientFileSystem) Stat(ctx context.Context, uri core.URI) (workspace.DirEntry, error) {
	var result FileSystemStatResult
	if err := s.call(ctx, FileSystemStatMethod, uri, &result); err != nil {
		return workspace.DirEntry{}, err
	}
	return clientDirEntry(uri, result.Type), nil
}

// ReadFile requests the content of the file at the given URI from the client.
func (s *ClientFileSystem) ReadFile(ctx context.Context, uri core.URI) ([]byte, error) {
	var result FileSystemReadFileResult
	if err := s.call(ctx, FileSystemReadFileMethod, uri, &result); err != nil {
		return nil, err
	}
	return result.Content, nil
}

// ReadDir requests the entries of the directory at the given URI from the client.
func (s *ClientFileSystem) ReadDir(ctx context.Context, uri core.URI) ([]workspace.DirEntry, error) {
	var result FileSystemReadDirectoryResult
	if err := s.call(ctx, FileSystemReadDirectoryMethod, uri, &result); err != nil {
		return nil, err
	}
	entries := make([]workspace.DirEntry, len(result.Entries))
	for i, entry := range result.Entries {
		entries[i] = clientDirEntry(uri.JoinPath(entry.Name), entry.Type)
	}
	return entries, nil
}

func (s *ClientFileSystem) call(ctx context.Context, method string, uri core.URI, result any) error {
	connection, err := s.sc.Get[*server.Connection]()
	if err != nil {
		return err
	}
	if connection.Value == nil {
		return errors.New("client file system: no connection to the client")
	}
	params := FileSystemParams{URI: uri.String()}
	if err := connection.Value.Call(ctx, method, params).Await(ctx, result); err != nil {
		return fmt.Errorf("%s %s: %w", method, uri.StringUnencoded(), err)
	}
	return nil
}

func clientDirEntry(uri core.URI, fileType FileType) workspace.DirEntry {
	symlink := fileType&FileTypeSymbolicLink != 0
	return workspace.DirEntry{
		IsFile:    !symlink && fileType&FileTypeFile != 0,
		IsDir:     !symlink && fileType&FileTypeDirectory != 0,
		IsSymlink: symlink,
		URI:       uri,
	}
}
