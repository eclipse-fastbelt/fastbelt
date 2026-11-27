// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package textmate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"typefox.dev/fastbelt/pluginapi"
	"typefox.dev/fastbelt/pluginapi/tokens"
)

var _ pluginapi.Plugin = (*Plugin)(nil)

// simpleModes returns one mode with a keyword and an ID token.
func simpleModes() []*tokens.TokenMode {
	tt := newTokenTypes()
	ms := modes("default")
	ms[0].Members = members(tt.keyword("if"), tt.regexp("ID", `[a-z]+`, tokens.ModifierNone))
	return ms
}

// buildArtifacts returns artifacts for a grammar named Test with output
// directory dir, and the handler collecting the warnings.
func buildArtifacts(dir string, ms []*tokens.TokenMode, languages ...*pluginapi.Language) (pluginapi.BuildArtifacts, *collectingHandler) {
	artifacts, handler := artifactsFor("Test", ms, languages...)
	artifacts.OutputDir = dir
	return artifacts, handler
}

func readGrammar(t *testing.T, path string) tmGrammar {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	var tm tmGrammar
	require.NoError(t, json.Unmarshal(content, &tm))
	return tm
}

// buildCommon runs Build with only a common file and returns the decoded
// common grammar and the warnings.
func buildCommon(t *testing.T, ms []*tokens.TokenMode, scopes map[string]string) (tmGrammar, []string) {
	t.Helper()
	dir := t.TempDir()
	artifacts, handler := buildArtifacts(dir, ms)
	require.NoError(t, New(Options{CommonFile: "common.json", Scopes: scopes}).Build(artifacts))
	return readGrammar(t, filepath.Join(dir, "common.json")), handler.messages
}

func TestBuildWritesFiles(t *testing.T) {
	dir := t.TempDir()
	ms := simpleModes()
	artifacts, _ := buildArtifacts(dir, ms,
		&pluginapi.Language{LanguageID: "first", Entry: "First", Patterns: []string{"*.one"}, EntryMode: ms[0]},
		language("second", "Second", ms[0]),
		language("third", "Third", ms[0]),
	)
	absolute := filepath.Join(t.TempDir(), "abs", "second.json")
	p := New(Options{
		CommonFile:    "syntaxes/nested/common.json",
		LanguageFiles: map[string]string{"first": "syntaxes/first.json", "second": absolute},
	})
	require.NoError(t, p.Build(artifacts))

	common := readGrammar(t, filepath.Join(dir, "syntaxes", "nested", "common.json"))
	require.Equal(t, "source.test-common", common.ScopeName)
	require.Equal(t, tmPattern{Patterns: includes("mode_default")}, common.Repository["First"])

	first := readGrammar(t, filepath.Join(dir, "syntaxes", "first.json"))
	require.Equal(t, "source.first", first.ScopeName)
	require.Equal(t, []string{"one"}, first.FileTypes)
	require.Equal(t, []tmPattern{{Include: "source.test-common#First"}}, first.Patterns)

	second := readGrammar(t, absolute)
	require.Equal(t, "source.second", second.ScopeName)

	// third has no entry in LanguageFiles: no file.
	entries, err := os.ReadDir(filepath.Join(dir, "syntaxes"))
	require.NoError(t, err)
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	require.Equal(t, []string{"first.json", "nested"}, names)
}

func TestBuildErrors(t *testing.T) {
	tests := []struct {
		name          string
		grammarName   string
		commonFile    string
		languageFiles map[string]string
		err           string
	}{
		{name: "empty common file", commonFile: "", err: "CommonFile is empty"},
		{
			name: "unknown language", commonFile: "common.json",
			languageFiles: map[string]string{"nope": "nope.json", "also": "also.json"},
			err:           `LanguageFiles: "also" is not a language of the build`,
		},
		{
			name: "empty language file", commonFile: "common.json",
			languageFiles: map[string]string{"test": ""},
			err:           `the file of language "test" is empty`,
		},
		{
			name: "language file equals common file", commonFile: "syntaxes/common.json",
			languageFiles: map[string]string{"test": "syntaxes/../syntaxes/./common.json"},
			err:           "same file",
		},
		{
			name: "two language files equal", commonFile: "common.json",
			languageFiles: map[string]string{"test": "same.json", "other": "same.json"},
			err:           "same file",
		},
		{
			name: "language scope equals common scope", grammarName: "Test", commonFile: "common.json",
			languageFiles: map[string]string{"test-common": "lang.json"},
			err:           `scope name source.test-common of language "test-common" equals the common scope name`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			ms := simpleModes()
			artifacts, _ := buildArtifacts(dir, ms,
				language("test", "Foo", ms[0]), language("other", "Bar", ms[0]), language("test-common", "Baz", ms[0]))
			err := New(Options{CommonFile: test.commonFile, LanguageFiles: test.languageFiles}).Build(artifacts)
			require.ErrorContains(t, err, test.err)
			// Nothing is written.
			entries, readErr := os.ReadDir(dir)
			require.NoError(t, readErr)
			require.Empty(t, entries)
		})
	}
}

func TestBuildDoesNotWriteOnGenerationError(t *testing.T) {
	dir := t.TempDir()
	tt := newTokenTypes()
	ms := modes("default")
	ms[0].Members = members(tt.regexp("BAD", `[a-`, tokens.ModifierNone))
	artifacts, _ := buildArtifacts(dir, ms)

	err := New(Options{CommonFile: "common.json", LanguageFiles: map[string]string{"test": "test.json"}}).Build(artifacts)
	require.ErrorContains(t, err, "token BAD")
	entries, readErr := os.ReadDir(dir)
	require.NoError(t, readErr)
	require.Empty(t, entries)
}

func TestBuildPushedModeEndsAtPopTokens(t *testing.T) {
	tt := newTokenTypes()
	leftBrace := tt.keyword("{")
	rightBrace := tt.keyword("}")
	semicolon := tt.keyword(";")
	id := tt.regexp("ID", `[a-z]+`, tokens.ModifierNone)
	ms := modes("default", "BLOCK")
	ms[0].Members = append([]*tokens.TokenModeMember{command(leftBrace, tokens.CommandPush, ms[1])}, members(id)...)
	ms[1].Members = append([]*tokens.TokenModeMember{
		command(rightBrace, tokens.CommandPop, nil),
		command(semicolon, tokens.CommandPop, nil),
	}, members(id)...)
	tm, warnings := buildCommon(t, ms, nil)

	require.Empty(t, warnings)
	rule := tm.Repository["token_LeftBrace_push_BLOCK"]
	require.Equal(t, `\{`, rule.Begin)
	require.Equal(t, `(\})|(;)`, rule.End)
	require.Equal(t, includes("mode_BLOCK"), rule.Patterns)
	require.Equal(t, includes("token_LeftBrace_push_BLOCK", "token_ID"), tm.Repository["mode_default"].Patterns)
}

func TestBuildModeSwitchEndsBeforeParentPopTokens(t *testing.T) {
	tt := newTokenTypes()
	leftParen := tt.keyword("(")
	rightParen := tt.keyword(")")
	rightBracket := tt.keyword("]")
	arrow := tt.keyword("->")
	id := tt.regexp("ID", `[a-z]+`, tokens.ModifierNone)
	ms := modes("default", "ARGS", "OTHER")
	def, args, other := ms[0], ms[1], ms[2]
	def.Members = append([]*tokens.TokenModeMember{command(leftParen, tokens.CommandPush, args)}, members(id)...)
	args.Members = append([]*tokens.TokenModeMember{
		command(rightParen, tokens.CommandPop, nil),
		command(arrow, tokens.CommandMode, other),
	}, members(id)...)
	other.Members = append([]*tokens.TokenModeMember{command(rightBracket, tokens.CommandPop, nil)}, members(id)...)
	tm, warnings := buildCommon(t, ms, nil)

	rule := tm.Repository["token_DashGreaterThan_mode_OTHER_in_ARGS"]
	require.Equal(t, `->`, rule.Begin)
	// A lookahead: the parent's pop token is left for the push rule to end.
	require.Equal(t, `(?=\))`, rule.End)
	require.Equal(t, includes("mode_OTHER"), rule.Patterns)
	require.Equal(t, []string{
		"TextMate: DashGreaterThan switches from token mode ARGS to OTHER, which have different pop tokens; " +
			"highlighting of OTHER ends at the pop tokens of ARGS",
	}, warnings)
}

func TestBuildCommentsBeforeKeywords(t *testing.T) {
	tt := newTokenTypes()
	ms := modes("default")
	ms[0].Members = append(members(tt.keyword("/"), tt.keyword("-"), tt.regexp("ID", `[a-z]+`, tokens.ModifierNone)),
		member(tt.regexp("SL_COMMENT", `\/\/[^\n]*`, tokens.ModifierComment), tokens.ModifierComment),
		member(tt.regexp("DOC", `--[^\n]*`, tokens.ModifierHidden), tokens.ModifierHidden),
	)
	tm, _ := buildCommon(t, ms, map[string]string{"DOC": "comment.line.documentation"})

	require.Equal(t, includes("token_SL_COMMENT_comment", "token_DOC_hidden", "keywords_default", "token_ID"),
		tm.Repository["mode_default"].Patterns)
	require.Equal(t, tmPattern{Name: "comment.line.test", Match: `//[^\n]*`}, tm.Repository["token_SL_COMMENT_comment"])
	require.Equal(t, tmPattern{Name: "comment.line.documentation.test", Match: `--[^\n]*`}, tm.Repository["token_DOC_hidden"])
}

func TestBuildStringEscapes(t *testing.T) {
	tt := newTokenTypes()
	ms := modes("default")
	ms[0].Members = members(tt.regexp("STRING", `"(\\.|[^"\\])*"|'(\\.|[^'\\])*'`, tokens.ModifierNone))
	tm, _ := buildCommon(t, ms, nil)

	escape := []tmPattern{{Match: `\\.`, Name: "constant.character.escape.test"}}
	require.Equal(t, tmPattern{Patterns: []tmPattern{
		{Name: "string.quoted.double.test", Begin: `"`, End: `"`, Patterns: escape},
		{Name: "string.quoted.single.test", Begin: `'`, End: `'`, Patterns: escape},
	}}, tm.Repository["token_STRING"])
}

func TestBuildScopeOverrides(t *testing.T) {
	tt := newTokenTypes()
	ms := modes("default")
	ms[0].Members = members(tt.keyword("if"), tt.regexp("ID", `[a-z]+`, tokens.ModifierNone),
		tt.regexp("INT", `[0-9]+`, tokens.ModifierNone))
	tm, warnings := buildCommon(t, ms, map[string]string{"ID": "variable.other", "INT": "", "if": "keyword.other"})

	require.Empty(t, warnings)
	require.Equal(t, tmPattern{Match: `[a-z]+`, Name: "variable.other.test"}, tm.Repository["token_ID"])
	// An empty override removes the scope.
	require.Equal(t, tmPattern{Match: `[0-9]+`}, tm.Repository["token_INT"])
	require.Equal(t, tmPattern{Match: `\bif\b`, Name: "keyword.other.test"}, tm.Repository["keywords_default"])
}

func TestBuildWarnsAboutUnknownOverride(t *testing.T) {
	_, warnings := buildCommon(t, simpleModes(), map[string]string{"ID": "variable", "MISSING": "x"})

	require.Equal(t, []string{"TextMate: scope override for unknown token MISSING"}, warnings)
}
