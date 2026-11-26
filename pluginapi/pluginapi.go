// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package pluginapi

// Plugin produces additional output from the artifacts of a Fastbelt build.
// See the package documentation for when and how plugins are called.
type Plugin interface {
	// Name identifies the plugin in error messages.
	Name() string
	// Build runs the plugin. The artifacts are shared with other plugins and
	// must not be modified.
	Build(artifacts BuildArtifacts) error
}

// BuildArtifacts holds everything a Fastbelt build produced that plugins may
// use. It is read-only by contract.
type BuildArtifacts struct {
	// InputDir is the absolute path of the directory containing the .fb grammar files.
	InputDir string
	// OutputDir is the absolute path of the directory the generated Go files
	// were written to. Plugins resolve relative output paths against it.
	OutputDir string
	// Package is the Go package name of the generated code.
	Package string
	// Languages are the languages configured for the build, in configured
	// order. There is at least one.
	Languages []*Language
}
