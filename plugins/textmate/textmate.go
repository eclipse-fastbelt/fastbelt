// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package textmate

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"typefox.dev/fastbelt/pluginapi"
)

// Options configures the TextMate plugin. See [New].
type Options struct {
	// CommonFile is the path of the common grammar shared by all languages.
	// It is required.
	CommonFile string
	// LanguageFiles maps language IDs to the path of their grammar. Languages
	// of the build without an entry get no grammar file.
	LanguageFiles map[string]string
	// Scopes maps token names to TextMate scopes, overriding the derived
	// scopes.
	Scopes map[string]string
}

// Plugin generates TextMate grammars. Create it with [New].
type Plugin struct {
	commonFile    string
	languageFiles map[string]string
	scopes        map[string]string
}

// New creates a TextMate plugin. The maps in options are copied, so later
// changes to them do not affect the plugin.
func New(options Options) *Plugin {
	return &Plugin{
		commonFile:    options.CommonFile,
		languageFiles: maps.Clone(options.LanguageFiles),
		scopes:        maps.Clone(options.Scopes),
	}
}

// Name returns "textmate".
func (p *Plugin) Name() string {
	return "textmate"
}

// Build writes the common grammar to the common file and the grammar of
// each language with an entry in the language files. Relative paths are
// resolved against artifacts.OutputDir; parent directories are created.
//
// Build fails without writing anything if the common file or a language
// file is empty, a language file is configured for a language that is not
// part of the build, two paths are the same after cleaning, or the scope name
// of a language equals the common scope name. Paths are compared as strings,
// so symbolic links and case-insensitive file systems are not taken into
// account. An I/O error while writing can leave some files written.
func (p *Plugin) Build(artifacts pluginapi.BuildArtifacts) error {
	files, err := p.resolveFiles(artifacts)
	if err != nil {
		return err
	}
	common, err := generateCommon(artifacts, p.scopes)
	if err != nil {
		return err
	}
	contents := []string{common}
	for _, language := range files.languages {
		content, err := generateLanguage(artifacts, language)
		if err != nil {
			return err
		}
		contents = append(contents, content)
	}
	for i, path := range files.paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(contents[i]), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// resolvedFiles lists the files Build writes: paths[0] is the common file,
// paths[i+1] the file of languages[i].
type resolvedFiles struct {
	paths     []string
	languages []*pluginapi.Language
}

// resolveFiles resolves the configured paths against artifacts.OutputDir and
// validates them, see [Plugin.Build]. Languages are in build order.
func (p *Plugin) resolveFiles(artifacts pluginapi.BuildArtifacts) (resolvedFiles, error) {
	if p.commonFile == "" {
		return resolvedFiles{}, errors.New("CommonFile is empty")
	}
	known := map[string]bool{}
	for _, language := range artifacts.Languages {
		known[language.LanguageID] = true
	}
	for _, id := range slices.Sorted(maps.Keys(p.languageFiles)) {
		if !known[id] {
			return resolvedFiles{}, fmt.Errorf("LanguageFiles: %q is not a language of the build", id)
		}
	}

	resolve := func(path string) string {
		if !filepath.IsAbs(path) {
			path = filepath.Join(artifacts.OutputDir, path)
		}
		return filepath.Clean(path)
	}
	commonPath := resolve(p.commonFile)
	files := resolvedFiles{paths: []string{commonPath}}
	owners := map[string]string{commonPath: "the common file"}
	commonScope := CommonScopeName(artifacts.Grammar.Name)
	for _, language := range artifacts.Languages {
		file, ok := p.languageFiles[language.LanguageID]
		if !ok {
			continue
		}
		owner := fmt.Sprintf("the file of language %q", language.LanguageID)
		if file == "" {
			return resolvedFiles{}, fmt.Errorf("%s is empty", owner)
		}
		if scope := LanguageScopeName(language.LanguageID); scope == commonScope {
			return resolvedFiles{}, fmt.Errorf("scope name %s of language %q equals the common scope name", scope, language.LanguageID)
		}
		path := resolve(file)
		if other, ok := owners[path]; ok {
			return resolvedFiles{}, fmt.Errorf("%s and %s are the same file %s", other, owner, path)
		}
		owners[path] = owner
		files.paths = append(files.paths, path)
		files.languages = append(files.languages, language)
	}
	return files, nil
}

// LanguageScopeName returns the scope name of the grammar of a language:
// source.<languageID>.
func LanguageScopeName(languageID string) string {
	return "source." + languageID
}

// CommonScopeName returns the scope name of the common grammar of a
// Fastbelt grammar: source.<grammar>-common, with the grammar name in lower
// case.
func CommonScopeName(grammarName string) string {
	return "source." + lowerGrammarName(grammarName) + "-common"
}

// lowerGrammarName returns the grammar name as used in TextMate scope names:
// in lower case.
func lowerGrammarName(grammarName string) string {
	return strings.ToLower(grammarName)
}
