// Copyright 2025 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"typefox.dev/fastbelt/internal/grammar"
	"typefox.dev/fastbelt/pluginapi"
	"typefox.dev/fastbelt/pluginapi/tokens"
)

// writeGrammarDir writes the given (name, source) pairs into a fresh temp
// directory and returns its path.
func writeGrammarDir(t *testing.T, nameSourcePairs ...string) string {
	t.Helper()
	dir := t.TempDir()
	for i := 0; i < len(nameSourcePairs); i += 2 {
		path := filepath.Join(dir, nameSourcePairs[i])
		require.NoError(t, os.WriteFile(path, []byte(nameSourcePairs[i+1]), 0644))
	}
	return dir
}

const multiTokens = `
	token ID: /[_a-zA-Z][\w_]*/
	hidden token WS: /\s+/
`

// A rule and the interface that declares its return type conventionally share
// a name, and the interface may live in another file.
func TestLoadGrammarDirMergesFiles(t *testing.T) {
	dir := writeGrammarDir(t,
		"a.fb", `
			grammar Multi
			interface Greeting { Name string }
			interface Farewell { Name string }
			entry Greeting: "hello" Name=ID
		`+multiTokens,
		"b.fb", `
			grammar Multi
			Farewell: "goodbye" Name=ID
		`,
	)

	g, err := LoadGrammarDir(dir)
	require.NoError(t, err)
	require.Len(t, g.Rules(), 2)
	require.Len(t, g.Interfaces(), 2)
	require.Len(t, g.Terminals(), 2)
	// Rules are sorted by name regardless of file order, and the implicit
	// return type of Farewell is found in the merged grammar.
	farewell := g.Rules()[0]
	require.Equal(t, "Farewell", farewell.Name())
	require.Equal(t, "Greeting", g.Rules()[1].Name())
	require.NotNil(t, grammar.FindReturnType(farewell, t.Context()))
}

func TestLoadGrammarDirKeepsTokenModesAndInfixRules(t *testing.T) {
	dir := writeGrammarDir(t,
		"a.fb", `
			grammar Multi
			interface Expr {}
			interface Primary extends Expr { Value string }
			interface Binary extends Expr { Left Expr Operator string Right Expr }
			entry Expr returns Expr: Binary
			Primary: Value=ID
			token ID: /[_a-zA-Z][\w_]*/
			hidden token WS: /\s+/
			token mode default {
				ID
				hidden WS
				"+" -> push(Inner)
			}
		`,
		"b.fb", `
			grammar Multi
			infix Binary on Primary returns Expr: "+"
			token mode Inner {
				ID -> pop
			}
		`,
	)

	g, err := LoadGrammarDir(dir)
	require.NoError(t, err)
	require.Len(t, g.TokenModes(), 2)
	require.Len(t, g.InfixRules(), 1)
}

func TestLoadGrammarDirRejectsDuplicateRuleAcrossFiles(t *testing.T) {
	dir := writeGrammarDir(t,
		"a.fb", `
			grammar Multi
			interface Greeting { Name string }
			entry Greeting: "hello" Name=ID
		`+multiTokens,
		"b.fb", `
			grammar Multi
			Greeting: "hi" Name=ID
		`,
	)

	_, err := LoadGrammarDir(dir)
	require.ErrorContains(t, err, "aborting code generation due to 2 errors")
}

func TestLoadGrammarDirRejectsMismatchedGrammarNames(t *testing.T) {
	dir := writeGrammarDir(t,
		"a.fb", `
			grammar Multi
			interface Greeting { Name string }
			entry Greeting: "hello" Name=ID
		`+multiTokens,
		"b.fb", `
			grammar Other
			interface Farewell { Name string }
			Farewell: "goodbye" Name=ID
		`,
	)

	_, err := LoadGrammarDir(dir)
	require.ErrorContains(t, err, "aborting code generation due to 2 errors")
}

func TestLoadGrammarDirRejectsTwoDefaultTokenModes(t *testing.T) {
	dir := writeGrammarDir(t,
		"a.fb", `
			grammar Multi
			interface Greeting { Name string }
			entry Greeting: "hello" Name=ID
			token mode default {
				"hello"
				ID
				hidden WS
			}
		`+multiTokens,
		"b.fb", `
			grammar Multi
			token mode default {
				ID
			}
		`,
	)

	_, err := LoadGrammarDir(dir)
	require.ErrorContains(t, err, "aborting code generation due to")
}

const modeGrammar = `
	grammar Multi
	interface Greeting { Name string }
	interface Farewell { Name string }
	entry Greeting: "hello" Name=ID
	entry Farewell: "goodbye" Name=ID
	token ID: /[_a-zA-Z][\w_]*/
	hidden token WS: /\s+/
	token mode default {
		"hello" -> push(Inner)
		ID
		hidden WS
	}
	token mode Inner {
		"goodbye"
		ID -> pop
		hidden WS
	}
`

func TestBuildRejectsUnknownTokenMode(t *testing.T) {
	dir := writeGrammarDir(t, "a.fb", modeGrammar)
	ctx := &BuildContext{Input: dir, Output: t.TempDir(), Package: "multi", Languages: []Language{
		{Entry: "Farewell", TokenMode: "Nope", LanguageID: "farewell"},
	}}
	require.ErrorContains(t, ctx.Build(), `token mode "Nope" not found`)
}

func TestBuildRejectsInvalidLanguageConfig(t *testing.T) {
	dir := writeGrammarDir(t, "a.fb", modeGrammar)
	for _, c := range []struct {
		name      string
		languages []Language
		err       string
	}{
		{"empty id", []Language{{Entry: "Greeting"}}, "has no language id"},
		{"duplicate id", []Language{
			{Entry: "Greeting", LanguageID: "same", Patterns: []string{"**/*.hello"}},
			{Entry: "Farewell", TokenMode: "Inner", LanguageID: "same", Patterns: []string{"**/*.bye"}},
		}, `language id "same" is used by more than one language`},
		{"duplicate entry", []Language{
			{Entry: "Greeting", LanguageID: "foo", Patterns: []string{"*.foo"}},
			{Entry: "Greeting", LanguageID: "foo-legacy", Patterns: []string{"*.fool"}},
		}, "entry rule Greeting is used by more than one language"},
		{"bad pattern", []Language{{Entry: "Greeting", LanguageID: "greeting", Patterns: []string{"**/*.[a"}}}, "invalid pattern"},
	} {
		t.Run(c.name, func(t *testing.T) {
			ctx := &BuildContext{Input: dir, Output: t.TempDir(), Package: "multi", Languages: c.languages}
			require.ErrorContains(t, ctx.Build(), c.err)
		})
	}
}

func TestBuildUsesConfiguredStartTokenMode(t *testing.T) {
	dir := writeGrammarDir(t, "a.fb", modeGrammar)
	out := t.TempDir()
	ctx := &BuildContext{Input: dir, Output: out, Package: "multi", Languages: []Language{
		{Entry: "Greeting", LanguageID: "greeting", Patterns: []string{"**/*.hello"}},
		{Entry: "Farewell", TokenMode: "Inner", LanguageID: "farewell", Patterns: []string{"**/*.bye"}},
	}}
	require.NoError(t, ctx.Build())
	code, err := os.ReadFile(filepath.Join(out, "lexer_gen.go"))
	require.NoError(t, err)
	require.Contains(t, string(code), "lexer.NewMultiLanguageLexer(sc, []int{TokenMode_default, TokenMode_Inner}, modes...)")
}

func TestBuildWritesRailroadDiagrams(t *testing.T) {
	dir := writeGrammarDir(t, "a.fb", modeGrammar)
	out := t.TempDir()
	ctx := &BuildContext{Input: dir, Output: out, Package: "multi", Railroad: true, Languages: []Language{
		{Entry: "Greeting", LanguageID: "greeting", Patterns: []string{"**/*.hello"}},
	}}
	require.NoError(t, ctx.Build())
	require.FileExists(t, filepath.Join(out, "railroad", "index.md"))
	require.FileExists(t, filepath.Join(out, "railroad", "Greeting.svg"))
}

func TestBuildRequiresEntryModeWithoutDefaultTokenMode(t *testing.T) {
	dir := writeGrammarDir(t, "a.fb", `
		grammar Multi
		interface Greeting { Name string }
		interface Farewell { Name string }
		entry Greeting: "hello" Name=ID
		entry Farewell: "goodbye" Name=ID
		token ID: /[_a-zA-Z][\w_]*/
		hidden token WS: /\s+/
		token mode GreetingMode {
			"hello"
			ID
			hidden WS
		}
		token mode FarewellMode {
			"goodbye"
			ID
			hidden WS
		}
	`)
	ctx := &BuildContext{Input: dir, Output: t.TempDir(), Package: "multi", Languages: []Language{
		{Entry: "Greeting", TokenMode: "GreetingMode", LanguageID: "greeting"},
		{Entry: "Farewell", LanguageID: "farewell"},
	}}
	require.ErrorContains(t, ctx.Build(), "declares no default token mode")
}

// Greeting uses "goodbye", which only FarewellMode lexes. Folder-wide
// validation is satisfied, but Greeting's entry mode can never produce it.
func TestBuildRejectsKeywordMissingFromLanguageModes(t *testing.T) {
	dir := writeGrammarDir(t, "a.fb", `
		grammar Multi
		interface Greeting { Name string }
		interface Farewell { Name string }
		entry Greeting: "hello" "goodbye" Name=ID
		entry Farewell: "goodbye" Name=ID
		token ID: /[_a-zA-Z][\w_]*/
		hidden token WS: /\s+/
		token mode GreetingMode { "hello" ID hidden WS }
		token mode FarewellMode { "goodbye" ID hidden WS }
	`)
	ctx := &BuildContext{Input: dir, Output: t.TempDir(), Package: "multi", Languages: []Language{
		{Entry: "Greeting", TokenMode: "GreetingMode", LanguageID: "greeting", Patterns: []string{"**/*.hello"}},
		{Entry: "Farewell", TokenMode: "FarewellMode", LanguageID: "farewell", Patterns: []string{"**/*.bye"}},
	}}
	require.ErrorContains(t, ctx.Build(), `language "greeting" (entry Greeting) uses keywords that no token mode reachable from its entry mode registers: ["goodbye"]`)
}

// Operators of an infix rule are only reachable through that rule, so a
// multi-language lexer must keep them in the modes of the language that calls
// the rule.
func TestBuildMultiLanguageLexerKeepsInfixOperators(t *testing.T) {
	dir := writeGrammarDir(t, "a.fb", `
		grammar Multi
		interface Greeting { Name string }
		interface Calc { Value Expression }
		interface Expression {}
		interface BinaryExpression extends Expression { Left Expression Operator string Right Expression }
		interface NumberLiteral extends Expression { Value string }
		entry Greeting: "hello" Name=ID
		entry Calc: "calc" Value=Expression
		Expression returns Expression: BinaryExpression
		infix BinaryExpression on PrimaryExpression:
			"+" | "-"
		PrimaryExpression returns Expression: NumberLiteral
		NumberLiteral: Value=NUMBER
		token NUMBER: /[0-9]+/
	`+multiTokens)
	out := t.TempDir()
	ctx := &BuildContext{Input: dir, Output: out, Package: "multi", Languages: []Language{
		{Entry: "Greeting", LanguageID: "greeting", Patterns: []string{"**/*.hello"}},
		{Entry: "Calc", LanguageID: "calc", Patterns: []string{"**/*.calc"}},
	}}
	require.NoError(t, ctx.Build())
	code, err := os.ReadFile(filepath.Join(out, "lexer_gen.go"))
	require.NoError(t, err)
	// The grammar declares no token modes, so each language gets a synthetic
	// one. Everything from the Calc mode on belongs to the second language.
	greetingMode, calcMode, found := strings.Cut(string(code), "modes[TokenMode_Calc] =")
	require.True(t, found)
	_, greetingMode, found = strings.Cut(greetingMode, "modes[TokenMode_Greeting] =")
	require.True(t, found)
	require.Contains(t, calcMode, "Keyword_Plus")
	require.Contains(t, calcMode, "Keyword_Dash")
	require.Contains(t, calcMode, "Token_NUMBER")
	require.NotContains(t, greetingMode, "Keyword_Plus")
	require.NotContains(t, greetingMode, "Token_NUMBER")
}

// recordingPlugin records its calls into a shared log and fails with err, if set.
type recordingPlugin struct {
	name      string
	err       error
	log       *[]string
	artifacts []pluginapi.BuildArtifacts
	// genFiles lists the *_gen.go files present in the output dir during Build.
	genFiles []string
}

func (p *recordingPlugin) Name() string { return p.name }

func (p *recordingPlugin) Build(artifacts pluginapi.BuildArtifacts) error {
	if p.log != nil {
		*p.log = append(*p.log, p.name)
	}
	p.artifacts = append(p.artifacts, artifacts)
	files, err := filepath.Glob(filepath.Join(artifacts.OutputDir, "*_gen.go"))
	if err != nil {
		return err
	}
	p.genFiles = files
	return p.err
}

func greetingBuildContext(t *testing.T, plugins ...pluginapi.Plugin) *BuildContext {
	t.Helper()
	return &BuildContext{
		Input:   writeGrammarDir(t, "a.fb", modeGrammar),
		Output:  t.TempDir(),
		Package: "multi",
		Languages: []Language{
			{Entry: "Greeting", LanguageID: "greeting", Patterns: []string{"**/*.hello"}},
		},
		Plugins: plugins,
	}
}

func TestBuildPassesArtifactsToPlugin(t *testing.T) {
	plugin := &recordingPlugin{name: "rec"}
	ctx := greetingBuildContext(t, plugin)
	require.NoError(t, ctx.Build())

	require.Len(t, plugin.artifacts, 1)
	artifacts := plugin.artifacts[0]
	require.Equal(t, ctx.Input, artifacts.InputDir)
	require.Equal(t, ctx.Output, artifacts.OutputDir)
	require.True(t, filepath.IsAbs(artifacts.InputDir))
	require.True(t, filepath.IsAbs(artifacts.OutputDir))
	require.Equal(t, "multi", artifacts.Package)
}

func TestBuildResolvesRelativeDirsAndDefaultsForPlugins(t *testing.T) {
	dir := writeGrammarDir(t, "a.fb", modeGrammar)
	t.Chdir(filepath.Dir(dir))
	plugin := &recordingPlugin{name: "rec"}
	ctx := &BuildContext{Input: filepath.Base(dir), Languages: []Language{
		{Entry: "Greeting", LanguageID: "greeting", Patterns: []string{"**/*.hello"}},
	}, Plugins: []pluginapi.Plugin{plugin}}
	require.NoError(t, ctx.Build())

	require.Len(t, plugin.artifacts, 1)
	artifacts := plugin.artifacts[0]
	// Compare resolved paths, the temp dir may be behind a symlink (macOS).
	resolved := func(p string) string {
		r, err := filepath.EvalSymlinks(p)
		require.NoError(t, err)
		return r
	}
	require.True(t, filepath.IsAbs(artifacts.InputDir))
	require.Equal(t, resolved(dir), resolved(artifacts.InputDir))
	require.Equal(t, resolved(dir), resolved(artifacts.OutputDir))
	require.Equal(t, filepath.Base(dir), artifacts.Package)
}

func TestBuildRunsPluginsAfterGeneratedFilesAreWritten(t *testing.T) {
	plugin := &recordingPlugin{name: "rec"}
	ctx := greetingBuildContext(t, plugin)
	require.NoError(t, ctx.Build())

	for _, name := range []string{"lexer_gen.go", "parser_gen.go", "types_gen.go", "services_gen.go", "atn_gen.go"} {
		require.Contains(t, plugin.genFiles, filepath.Join(ctx.Output, name))
	}
}

func TestBuildRunsPluginsInRegistrationOrder(t *testing.T) {
	var log []string
	ctx := greetingBuildContext(t,
		&recordingPlugin{name: "first", log: &log},
		&recordingPlugin{name: "second", log: &log},
		&recordingPlugin{name: "third", log: &log},
	)
	require.NoError(t, ctx.Build())
	require.Equal(t, []string{"first", "second", "third"}, log)
}

func TestBuildAbortsOnFailingPlugin(t *testing.T) {
	var log []string
	errBoom := errors.New("boom")
	later := &recordingPlugin{name: "later", log: &log}
	ctx := greetingBuildContext(t,
		&recordingPlugin{name: "ok", log: &log},
		&recordingPlugin{name: "failing", log: &log, err: errBoom},
		later,
	)
	err := ctx.Build()
	require.Error(t, err)
	require.ErrorIs(t, err, errBoom)
	require.ErrorContains(t, err, `plugin "failing"`)
	require.Equal(t, []string{"ok", "failing"}, log)
	require.Empty(t, later.artifacts)
}

func TestBuildRejectsNilPlugins(t *testing.T) {
	var nilPointer *recordingPlugin
	for name, plugin := range map[string]pluginapi.Plugin{"nil interface": nil, "nil pointer": nilPointer} {
		t.Run(name, func(t *testing.T) {
			ctx := greetingBuildContext(t, &recordingPlugin{name: "ok"}, plugin)
			require.EqualError(t, ctx.Build(), "plugin 1 is nil")
			require.NoFileExists(t, filepath.Join(ctx.Output, "lexer_gen.go"))
		})
	}
}

func TestBuildWithoutPlugins(t *testing.T) {
	ctx := greetingBuildContext(t)
	require.NoError(t, ctx.Build())
	require.FileExists(t, filepath.Join(ctx.Output, "lexer_gen.go"))
	require.FileExists(t, filepath.Join(ctx.Output, "parser_gen.go"))
}

func TestBuildPassesLanguagesToPlugin(t *testing.T) {
	plugin := &recordingPlugin{name: "rec"}
	// The configuration of internal/languages/multilang/gen/main.go.
	ctx := &BuildContext{
		Input:   filepath.Join("..", "internal", "languages", "multilang"),
		Output:  t.TempDir(),
		Package: "multilang",
		Languages: []Language{
			{Entry: "Greeting", TokenMode: "GreetingMode", LanguageID: "greeting", Patterns: []string{"*.hello"}},
			{Entry: "Farewell", TokenMode: "FarewellMode", LanguageID: "farewell", Patterns: []string{"*.bye"}},
		},
		Plugins: []pluginapi.Plugin{plugin},
	}
	require.NoError(t, ctx.Build())

	require.Len(t, plugin.artifacts, 1)
	langs := plugin.artifacts[0].Languages
	require.Len(t, langs, 2)
	require.Equal(t, "greeting", langs[0].LanguageID)
	require.Equal(t, "Greeting", langs[0].Entry)
	require.Equal(t, []string{"*.hello"}, langs[0].Patterns)
	require.Equal(t, "farewell", langs[1].LanguageID)
	require.Equal(t, "Farewell", langs[1].Entry)
	require.Equal(t, []string{"*.bye"}, langs[1].Patterns)

	langs[0].Patterns[0] = "*.changed"
	require.Equal(t, []string{"*.hello"}, ctx.Languages[0].Patterns)
}

func TestBuildPassesTokenTypesToPlugin(t *testing.T) {
	plugin := &recordingPlugin{name: "rec"}
	ctx := &BuildContext{
		Input:     filepath.Join("..", "examples", "arithmetics"),
		Output:    t.TempDir(),
		Package:   "arithmetics",
		Languages: []Language{{Entry: "Module", LanguageID: "arithmetics"}},
		Plugins:   []pluginapi.Plugin{plugin},
	}
	require.NoError(t, ctx.Build())

	require.Len(t, plugin.artifacts, 1)
	tts := plugin.artifacts[0].TokenTypes
	require.NotEmpty(t, tts)
	for i, tt := range tts {
		require.Equal(t, i, tt.Index)
	}
	require.Equal(t, tokens.KindEOF, tts[0].Kind)
	// Infix rules are expanded before the token types are converted.
	i := slices.IndexFunc(tts, func(tt *tokens.TokenType) bool { return tt.Name == "BinaryExpressionOperator" })
	require.NotEqual(t, -1, i)
	require.True(t, tts[i].Synthetic)
}
