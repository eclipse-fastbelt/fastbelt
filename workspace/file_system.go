// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package workspace

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"sync"

	core "typefox.dev/fastbelt"
)

// DirEntry represents a file system entry, which can be a file, directory, or/and symbolic link.
type DirEntry struct {
	// IsFile is true if the entry is a regular file.
	IsFile bool
	// IsDir is true if the entry is a directory.
	IsDir bool
	// IsSymlink is true if the entry is a symbolic link.
	IsSymlink bool
	// URI is the URI of the entry.
	URI core.URI
}

// FileSystemProvider reads data from a single backend, such as the local disk
// or an in-memory tree. A provider is registered with a [FileSystem] for a URI
// scheme and only receives URIs of that scheme, so it does not need to inspect
// the scheme itself.
//
// Errors for missing entries should wrap [fs.ErrNotExist].
type FileSystemProvider interface {
	// Stat returns the file system entry for the given URI, or an error if it does not exist.
	Stat(ctx context.Context, uri core.URI) (DirEntry, error)
	// ReadFile reads the content of the file at the given URI and returns it as a byte slice.
	ReadFile(ctx context.Context, uri core.URI) ([]byte, error)
	// ReadDir reads the contents of the directory at the given URI and returns a slice of [DirEntry] for its entries.
	ReadDir(ctx context.Context, uri core.URI) ([]DirEntry, error)
}

// FileSystem defines the interface for reading data from arbitrary file systems.
// It dispatches every request to the [FileSystemProvider] registered for the
// scheme of the requested URI.
type FileSystem interface {
	// RegisterProvider registers the [FileSystemProvider] for all URIs with the given scheme,
	// replacing any provider previously registered for that scheme.
	// The returned function removes the registration again; it does nothing if
	// the provider has been replaced in the meantime.
	RegisterProvider(scheme string, provider FileSystemProvider) (unregister func())
	// Exists checks if the given URI exists in the file system.
	Exists(ctx context.Context, uri core.URI) bool
	// Stat returns the file system entry for the given URI, or an error if it does not exist.
	Stat(ctx context.Context, uri core.URI) (DirEntry, error)
	// ReadFile reads the content of the file at the given URI and returns it as a byte slice.
	ReadFile(ctx context.Context, uri core.URI) ([]byte, error)
	// ReadDir reads the contents of the directory at the given URI and returns a slice of [DirEntry] for its entries.
	ReadDir(ctx context.Context, uri core.URI) ([]DirEntry, error)
}

// SkipDir indicates that the current directory should be skipped.
var SkipDir = fs.SkipDir

// SkipAll indicates that the entire file system walk should be aborted.
var SkipAll = fs.SkipAll

func WalkFileSystem(ctx context.Context, fs FileSystem, uri core.URI, fn func(DirEntry) error) error {
	entry, err := fs.Stat(ctx, uri)
	if err != nil {
		return err
	}
	err = walkDir(ctx, fs, entry, fn)
	if err == SkipAll || err == SkipDir {
		return nil
	}
	return err
}

func walkDir(ctx context.Context, fs FileSystem, entry DirEntry, fn func(DirEntry) error) error {
	if ctx.Err() != nil {
		// Abort the walk if the context is already cancelled.
		return ctx.Err()
	}
	if err := fn(entry); err != nil || !entry.IsDir {
		if err == SkipDir && entry.IsDir {
			return nil // Skip this dir's contents, not an error
		}
		return err
	}
	entries, err := fs.ReadDir(ctx, entry.URI)
	if err != nil {
		return err
	}
	for _, child := range entries {
		if err := walkDir(ctx, fs, child, fn); err != nil {
			if err == SkipDir {
				break // SkipDir from a child stops the remaining siblings
			}
			return err
		}
	}
	return nil
}

// DefaultFileSystem is the default [FileSystem] implementation. It holds no
// data itself, but delegates to the provider registered for a URI's scheme.
// It is safe for concurrent use; the registered providers are responsible for
// their own synchronization.
type DefaultFileSystem struct {
	mu        sync.RWMutex
	providers map[string]FileSystemProvider
}

// NewDefaultFileSystem creates a [FileSystem] without any registered providers.
func NewDefaultFileSystem() *DefaultFileSystem {
	return &DefaultFileSystem{
		providers: make(map[string]FileSystemProvider),
	}
}

// NewDiskFileSystem creates a [FileSystem] that serves the "file" scheme from the local disk
// through a [DiskFileSystemProvider].
func NewDiskFileSystem() *DefaultFileSystem {
	s := NewDefaultFileSystem()
	s.RegisterProvider(core.FileScheme, NewDiskFileSystemProvider())
	return s
}

// RegisterProvider registers the [FileSystemProvider] for all URIs with the given scheme.
func (s *DefaultFileSystem) RegisterProvider(scheme string, provider FileSystemProvider) func() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providers[scheme] = provider
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.providers[scheme] == provider {
			delete(s.providers, scheme)
		}
	}
}

// provider returns the provider registered for the scheme of the given URI.
// The returned error wraps [fs.ErrNotExist] if there is none.
func (s *DefaultFileSystem) provider(uri core.URI) (FileSystemProvider, error) {
	s.mu.RLock()
	provider, ok := s.providers[uri.Scheme()]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no file system provider for scheme %q: %w", uri.Scheme(), fs.ErrNotExist)
	}
	return provider, nil
}

// Exists checks if the given URI exists in the file system.
func (s *DefaultFileSystem) Exists(ctx context.Context, uri core.URI) bool {
	_, err := s.Stat(ctx, uri)
	return err == nil
}

// Stat returns the file system entry for the given URI, or an error if it does not exist.
func (s *DefaultFileSystem) Stat(ctx context.Context, uri core.URI) (DirEntry, error) {
	provider, err := s.provider(uri)
	if err != nil {
		return DirEntry{}, err
	}
	return provider.Stat(ctx, uri)
}

// ReadFile reads the content of the file at the given URI and returns it as a byte slice.
func (s *DefaultFileSystem) ReadFile(ctx context.Context, uri core.URI) ([]byte, error) {
	provider, err := s.provider(uri)
	if err != nil {
		return nil, err
	}
	return provider.ReadFile(ctx, uri)
}

// ReadDir reads the contents of the directory at the given URI and returns a slice of [DirEntry] for its entries.
func (s *DefaultFileSystem) ReadDir(ctx context.Context, uri core.URI) ([]DirEntry, error) {
	provider, err := s.provider(uri)
	if err != nil {
		return nil, err
	}
	return provider.ReadDir(ctx, uri)
}

// DiskFileSystemProvider is a [FileSystemProvider] that reads from the local disk using the [os] package.
// It is meant to be registered for the "file" scheme.
type DiskFileSystemProvider struct{}

// NewDiskFileSystemProvider creates a new instance of [DiskFileSystemProvider].
func NewDiskFileSystemProvider() *DiskFileSystemProvider {
	return &DiskFileSystemProvider{}
}

// Stat returns the file system entry for the given URI, or an error if it does not exist.
func (s *DiskFileSystemProvider) Stat(ctx context.Context, uri core.URI) (DirEntry, error) {
	info, err := os.Lstat(uri.FilePath())
	if err != nil {
		return DirEntry{}, err
	}
	return DirEntry{
		IsFile:    info.Mode().IsRegular(),
		IsDir:     info.IsDir(),
		IsSymlink: info.Mode()&os.ModeSymlink != 0,
		URI:       uri,
	}, nil
}

// ReadFile reads the content of the file at the given URI and returns it as a byte slice.
func (s *DiskFileSystemProvider) ReadFile(ctx context.Context, uri core.URI) ([]byte, error) {
	return os.ReadFile(uri.FilePath())
}

// ReadDir reads the contents of the directory at the given URI and returns a slice of [DirEntry] for its entries.
func (s *DiskFileSystemProvider) ReadDir(ctx context.Context, uri core.URI) ([]DirEntry, error) {
	entries, err := os.ReadDir(uri.FilePath())
	if err != nil {
		return nil, err
	}
	result := make([]DirEntry, len(entries))
	for i, entry := range entries {
		result[i] = DirEntry{
			IsFile:    entry.Type().IsRegular(),
			IsDir:     entry.IsDir(),
			IsSymlink: entry.Type()&os.ModeSymlink != 0,
			URI:       uri.JoinPath(entry.Name()),
		}
	}
	return result, nil
}

// virtualDirEntry is a single node in the file system trie. A node is either a
// directory (children != nil) or a file (children == nil, content holds the
// bytes). The empty string key in the parent's children map is never used;
// children are keyed by path segment.
type virtualDirEntry struct {
	name     string
	content  []byte                      // non-nil => file
	children map[string]*virtualDirEntry // non-nil => directory
}

func (e *virtualDirEntry) isDir() bool { return e.children != nil }

func newVirtualDir(name string) *virtualDirEntry {
	return &virtualDirEntry{name: name, children: map[string]*virtualDirEntry{}}
}

// VirtualFileSystemProvider is an in-memory [FileSystemProvider]. It keeps a
// single directory trie that is addressed by URI path only: the scheme is
// determined by the registration with a [FileSystem]. Register separate
// instances to keep the contents of multiple schemes apart.
//
// Note: not safe for concurrent use; wrap with a workspace lock if shared.
type VirtualFileSystemProvider struct {
	root *virtualDirEntry
}

// NewVirtualFileSystemProvider creates an empty [VirtualFileSystemProvider].
func NewVirtualFileSystemProvider() *VirtualFileSystemProvider {
	return &VirtualFileSystemProvider{
		root: newVirtualDir(""),
	}
}

// segments splits a URI path into non-empty path segments.
func segments(uri core.URI) []string {
	return strings.FieldsFunc(uri.Path(), func(r rune) bool { return r == '/' })
}

// find walks the trie for the given URI and returns the node, or nil if any
// segment is missing or traverses through a file.
func (s *VirtualFileSystemProvider) find(uri core.URI) *virtualDirEntry {
	node := s.root
	for _, seg := range segments(uri) {
		if !node.isDir() {
			return nil
		}
		next, ok := node.children[seg]
		if !ok {
			return nil
		}
		node = next
	}
	return node
}

// mkdirAll walks segs from the root, creating missing directories. It
// returns an error if a segment collides with an existing file.
func (s *VirtualFileSystemProvider) mkdirAll(segs []string) (*virtualDirEntry, error) {
	node := s.root
	for _, seg := range segs {
		child, ok := node.children[seg]
		if !ok {
			child = newVirtualDir(seg)
			node.children[seg] = child
		} else if !child.isDir() {
			return nil, os.ErrExist
		}
		node = child
	}
	return node, nil
}

func entryFor(node *virtualDirEntry, uri core.URI) DirEntry {
	return DirEntry{
		IsFile: !node.isDir(),
		IsDir:  node.isDir(),
		URI:    uri,
	}
}

func (s *VirtualFileSystemProvider) Stat(ctx context.Context, uri core.URI) (DirEntry, error) {
	node := s.find(uri)
	if node == nil {
		return DirEntry{}, os.ErrNotExist
	}
	return entryFor(node, uri), nil
}

func (s *VirtualFileSystemProvider) ReadFile(ctx context.Context, uri core.URI) ([]byte, error) {
	node := s.find(uri)
	if node == nil || node.isDir() {
		return nil, os.ErrNotExist
	}
	return node.content, nil
}

func (s *VirtualFileSystemProvider) ReadDir(ctx context.Context, uri core.URI) ([]DirEntry, error) {
	node := s.find(uri)
	if node == nil || !node.isDir() {
		return nil, os.ErrNotExist
	}
	result := make([]DirEntry, 0, len(node.children))
	for name, child := range node.children {
		result = append(result, entryFor(child, uri.JoinPath(name)))
	}
	// Produce stable order for testing and deterministic behavior
	sort.Slice(result, func(i, j int) bool {
		return result[i].URI.String() < result[j].URI.String()
	})
	return result, nil
}

func (s *VirtualFileSystemProvider) Mkdir(uri core.URI) error {
	segs := segments(uri)
	if len(segs) == 0 {
		return os.ErrExist // root always exists
	}
	parent, err := s.mkdirAll(segs[:len(segs)-1])
	if err != nil {
		return err
	}
	last := segs[len(segs)-1]
	if _, ok := parent.children[last]; ok {
		return os.ErrExist
	}
	parent.children[last] = newVirtualDir(last)
	return nil
}

func (s *VirtualFileSystemProvider) WriteFile(uri core.URI, content []byte) error {
	segs := segments(uri)
	if len(segs) == 0 {
		return os.ErrInvalid // cannot write the root
	}
	parent, err := s.mkdirAll(segs[:len(segs)-1])
	if err != nil {
		return err
	}
	last := segs[len(segs)-1]
	if existing, ok := parent.children[last]; ok && existing.isDir() {
		return os.ErrInvalid // target is a directory
	}
	if content == nil {
		content = []byte{}
	}
	parent.children[last] = &virtualDirEntry{name: last, content: content}
	return nil
}

// Delete removes the file or directory at the given URI, including all of its
// contents. Deleting the root empties the provider; the root itself remains.
func (s *VirtualFileSystemProvider) Delete(uri core.URI) error {
	segs := segments(uri)
	if len(segs) == 0 {
		s.root = newVirtualDir("")
		return nil
	}
	dir := s.root
	for _, seg := range segs[:len(segs)-1] {
		child, ok := dir.children[seg]
		if !ok || !child.isDir() {
			return os.ErrNotExist
		}
		dir = child
	}
	last := segs[len(segs)-1]
	if _, ok := dir.children[last]; !ok {
		return os.ErrNotExist
	}
	delete(dir.children, last)
	return nil
}
