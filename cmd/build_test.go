// Copyright 2025 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package cmd

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/internal/grammar"
	"typefox.dev/fastbelt/pluginapi"
	"typefox.dev/fastbelt/pluginapi/ast"
	"typefox.dev/fastbelt/pluginapi/tokens"
	"typefox.dev/fastbelt/pluginapi/types"
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

// warningPlugin logs msg as a warning.
type warningPlugin struct{ msg string }

func (p *warningPlugin) Name() string { return "warn" }

func (p *warningPlugin) Build(artifacts pluginapi.BuildArtifacts) error {
	artifacts.Logger.Warn(p.msg)
	return nil
}

// collectingHandler collects the messages of all records.
type collectingHandler struct {
	slog.Handler
	messages []string
}

func (h *collectingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *collectingHandler) Handle(_ context.Context, r slog.Record) error {
	h.messages = append(h.messages, r.Message)
	return nil
}

func TestBuildPassesCustomLoggerToPlugin(t *testing.T) {
	plugin := &recordingPlugin{name: "rec"}
	handler := &collectingHandler{Handler: slog.DiscardHandler}
	ctx := greetingBuildContext(t, plugin, &warningPlugin{msg: "careful"})
	ctx.Logger = slog.New(handler)
	require.NoError(t, ctx.Build())

	require.Len(t, plugin.artifacts, 1)
	require.Same(t, ctx.Logger, plugin.artifacts[0].Logger)
	require.Equal(t, []string{"careful"}, handler.messages)
}

func TestBuildPassesDefaultLoggerToPlugin(t *testing.T) {
	stderr, err := os.Create(filepath.Join(t.TempDir(), "stderr"))
	require.NoError(t, err)
	defer stderr.Close()
	orig := os.Stderr
	os.Stderr = stderr
	t.Cleanup(func() { os.Stderr = orig })

	plugin := &recordingPlugin{name: "rec"}
	ctx := greetingBuildContext(t, plugin, &warningPlugin{msg: "careful"})
	require.NoError(t, ctx.Build())

	require.Len(t, plugin.artifacts, 1)
	require.NotNil(t, plugin.artifacts[0].Logger)
	written, err := os.ReadFile(stderr.Name())
	require.NoError(t, err)
	require.Equal(t, "Warning: careful\n", string(written))
}

func TestWarningHandlerWritesWarnings(t *testing.T) {
	var buf strings.Builder
	logger := slog.New(newWarningHandler(&buf))
	logger.Info("not shown")
	logger.Warn("first problem")
	logger.Error("second problem")
	require.Equal(t, "Warning: first problem\nWarning: second problem\n", buf.String())
}

func TestWarningHandlerWritesAttributes(t *testing.T) {
	var buf strings.Builder
	logger := slog.New(newWarningHandler(&buf)).With("plugin", "textmate").WithGroup("token")
	logger.Warn("no scope", "name", "ID", "pattern", "a b", slog.Group("pos", "line", 3))
	require.Equal(t, `Warning: no scope plugin=textmate token.name=ID token.pattern="a b" token.pos.line=3`+"\n", buf.String())
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

// TestBuildLinksElementsToGeneratedTokenIndices checks the token types of
// keywords and token rule calls in rule bodies against the _Idx constants of
// the generated lexer.
func TestBuildLinksElementsToGeneratedTokenIndices(t *testing.T) {
	plugin := &recordingPlugin{name: "rec"}
	out := t.TempDir()
	ctx := &BuildContext{
		Input:     filepath.Join("..", "examples", "arithmetics"),
		Output:    out,
		Package:   "arithmetics",
		Languages: []Language{{Entry: "Module", LanguageID: "arithmetics"}},
		Plugins:   []pluginapi.Plugin{plugin},
	}
	require.NoError(t, ctx.Build())
	code, err := os.ReadFile(filepath.Join(out, "lexer_gen.go"))
	require.NoError(t, err)
	constants := map[string]int{}
	for _, m := range regexp.MustCompile(`(?m)^const (\w+_Idx) = (\d+)$`).FindAllStringSubmatch(string(code), -1) {
		index, err := strconv.Atoi(m[2])
		require.NoError(t, err)
		constants[m[1]] = index
	}
	g := plugin.artifacts[0].Grammar

	// Keywords, by the generated names of their values.
	keywordConstants := map[string]string{"module": "Keyword_module_Idx", "(": "Keyword_LeftParen_Idx", "%": "Keyword_Percent_Idx", "^": "Keyword_Caret_Idx"}
	seen := map[string]bool{}
	var calls int
	var visit func(e ast.Element)
	visit = func(e ast.Element) {
		switch e := e.(type) {
		case *ast.Group:
			for _, child := range e.Elements {
				visit(child)
			}
		case *ast.Alternatives:
			for _, child := range e.Alts {
				visit(child)
			}
		case *ast.Assignment:
			visit(e.Value)
		case *ast.CrossRef:
			visit(e.Rule)
		case *ast.Keyword:
			if name, ok := keywordConstants[e.Value]; ok {
				require.Equal(t, constants[name], e.TokenType.Index, e.Value)
				seen[e.Value] = true
			}
		case *ast.RuleCall:
			var name string
			switch r := e.Rule.(type) {
			case *ast.TokenDecl:
				name = "Token_" + r.Name + "_Idx"
			case *ast.TokenGroup:
				name = "TokenGroup_" + r.Name + "_Idx"
			default:
				require.Nil(t, e.TokenType)
				return
			}
			require.Contains(t, constants, name)
			require.Equal(t, constants[name], e.TokenType.Index, name)
			calls++
		}
	}
	for _, r := range g.Rules {
		visit(r.Body)
	}
	infix := g.InfixRules[0]
	visit(infix.Body)
	for _, group := range infix.Groups {
		for _, op := range group.Operators {
			visit(op)
		}
	}
	require.Len(t, seen, len(keywordConstants))
	require.Greater(t, calls, 4, "ID, NUMBER, the operator group and the groups of the infix rule")
}

func TestBuildPassesTokenModesToPlugin(t *testing.T) {
	plugin := &recordingPlugin{name: "rec"}
	out := t.TempDir()
	ctx := &BuildContext{
		Input:   writeGrammarDir(t, "a.fb", modeGrammar),
		Output:  out,
		Package: "multi",
		Languages: []Language{
			{Entry: "Greeting", LanguageID: "greeting"},
			{Entry: "Farewell", LanguageID: "farewell", TokenMode: "Inner"},
		},
		Plugins: []pluginapi.Plugin{plugin},
	}
	require.NoError(t, ctx.Build())

	require.Len(t, plugin.artifacts, 1)
	artifacts := plugin.artifacts[0]
	modes := artifacts.TokenModes
	require.Len(t, modes, 2)
	requireModeConstants(t, out, modes)
	def, inner := modes[0], modes[1]
	require.Equal(t, "default", def.Name)
	require.Equal(t, "Inner", inner.Name)
	require.Same(t, def, artifacts.Languages[0].EntryMode)
	require.Same(t, inner, artifacts.Languages[1].EntryMode)
	// Members link to the shared token types.
	hello := def.Members[0]
	require.Same(t, artifacts.TokenTypes[hello.TokenType.Index], hello.TokenType)
	require.Equal(t, &tokens.TokenCommand{Kind: tokens.CommandPush, Target: inner}, hello.Command)
}

func TestBuildPassesSyntheticTokenModesToPlugin(t *testing.T) {
	plugin := &recordingPlugin{name: "rec"}
	out := t.TempDir()
	ctx := &BuildContext{
		Input: writeGrammarDir(t, "a.fb", `
			grammar Multi
			interface Greeting { Name string }
			interface Farewell { Name string }
			entry Greeting: "hello" Name=ID
			entry Farewell: "goodbye" Name=ID
		`+multiTokens),
		Output:  out,
		Package: "multi",
		Languages: []Language{
			{Entry: "Greeting", LanguageID: "greeting"},
			{Entry: "Farewell", LanguageID: "farewell"},
		},
		Plugins: []pluginapi.Plugin{plugin},
	}
	require.NoError(t, ctx.Build())

	require.Len(t, plugin.artifacts, 1)
	artifacts := plugin.artifacts[0]
	modes := artifacts.TokenModes
	require.Len(t, modes, 2)
	requireModeConstants(t, out, modes)
	for i, lang := range artifacts.Languages {
		require.Same(t, modes[i], lang.EntryMode)
		require.Equal(t, lang.Entry, lang.EntryMode.Name)
		require.True(t, lang.EntryMode.Synthetic)
	}
}

func TestBuildPassesEntryRulesToPlugin(t *testing.T) {
	plugin := &recordingPlugin{name: "rec"}
	ctx := &BuildContext{
		Input:   writeGrammarDir(t, "a.fb", splitGreetingGrammarA, "b.fb", "grammar Multi\nentry Farewell: \"goodbye\" Name=ID\n"),
		Output:  t.TempDir(),
		Package: "multi",
		Languages: []Language{
			{Entry: "Farewell", LanguageID: "farewell"},
			{Entry: "Greeting", LanguageID: "greeting"},
		},
		Plugins: []pluginapi.Plugin{plugin},
	}
	require.NoError(t, ctx.Build())

	artifacts := plugin.artifacts[0]
	rules := map[string]*ast.ParserRule{}
	for _, r := range artifacts.Grammar.Rules {
		rules[r.Name] = r
	}
	require.Len(t, rules, 2)
	for _, lang := range artifacts.Languages {
		require.NotNil(t, lang.EntryRule, lang.LanguageID)
		require.Same(t, rules[lang.Entry], lang.EntryRule, lang.LanguageID)
	}
	// The entry rules come from different files.
	require.Same(t, artifacts.Sources[0], artifacts.Languages[1].EntryRule.Location().Source)
	require.Same(t, artifacts.Sources[1], artifacts.Languages[0].EntryRule.Location().Source)
}

// requireModeConstants asserts that the generated lexer in dir declares a
// TokenMode_<Name> constant with the index of each mode.
func requireModeConstants(t *testing.T, dir string, modes []*tokens.TokenMode) {
	t.Helper()
	code, err := os.ReadFile(filepath.Join(dir, "lexer_gen.go"))
	require.NoError(t, err)
	for i, mode := range modes {
		require.Equal(t, i, mode.Index)
		pattern := regexp.MustCompile(`(?m)^\s*TokenMode_` + regexp.QuoteMeta(mode.Name) + `\s*=\s*(\d+)$`)
		match := pattern.FindStringSubmatch(string(code))
		require.NotNil(t, match, "constant of mode %q", mode.Name)
		require.Equal(t, strconv.Itoa(mode.Index), match[1], "mode %q", mode.Name)
	}
	require.Len(t, regexp.MustCompile(`(?m)^\s*TokenMode_\w+\s*=`).FindAllString(string(code), -1), len(modes))
}

// splitGreetingGrammarA and splitGreetingGrammarB form a grammar whose
// Farewell rule lives in a second file.
const (
	splitGreetingGrammarA = `
		grammar Multi
		interface Greeting { Name string }
		interface Farewell { Name string }
		entry Greeting: "hello" Name=ID
	` + multiTokens
	splitGreetingGrammarB = "grammar Multi\nFarewell: \"goodbye\" Name=ID\n"
)

func TestBuildPassesSourcesToPlugin(t *testing.T) {
	plugin := &recordingPlugin{name: "rec"}
	aText, bText := splitGreetingGrammarA, splitGreetingGrammarB
	// Written in reverse order to check the sorting.
	ctx := &BuildContext{
		Input:     writeGrammarDir(t, "b.fb", bText, "a.fb", aText),
		Output:    t.TempDir(),
		Package:   "multi",
		Languages: []Language{{Entry: "Greeting", LanguageID: "greeting"}},
		Plugins:   []pluginapi.Plugin{plugin},
	}
	require.NoError(t, ctx.Build())

	require.Len(t, plugin.artifacts, 1)
	sources := plugin.artifacts[0].Sources
	require.Len(t, sources, 2)
	require.Equal(t, filepath.Join(ctx.Input, "a.fb"), sources[0].Path)
	require.Equal(t, filepath.Join(ctx.Input, "b.fb"), sources[1].Path)
	require.True(t, filepath.IsAbs(sources[0].Path))
	require.Equal(t, aText, sources[0].Text)
	require.Equal(t, bText, sources[1].Text)
	line, col := sources[1].LineColumn(strings.Index(bText, "goodbye"))
	require.Equal(t, []int{2, 12}, []int{line, col})
}

func TestBuildPassesSingleSourceToPlugin(t *testing.T) {
	plugin := &recordingPlugin{name: "rec"}
	ctx := greetingBuildContext(t, plugin)
	require.NoError(t, ctx.Build())

	sources := plugin.artifacts[0].Sources
	require.Len(t, sources, 1)
	require.Equal(t, filepath.Join(ctx.Input, "a.fb"), sources[0].Path)
	require.Equal(t, modeGrammar, sources[0].Text)
}

func TestLoadGrammarDirRecordsNodeSourcesBeforeMerging(t *testing.T) {
	dir := writeGrammarDir(t,
		"a.fb", splitGreetingGrammarA,
		"b.fb", splitGreetingGrammarB,
	)
	// A relative input dir still yields absolute source paths.
	wd, err := os.Getwd()
	require.NoError(t, err)
	rel, err := filepath.Rel(wd, dir)
	require.NoError(t, err)

	loaded, err := loadGrammarDir(rel)
	require.NoError(t, err)
	require.Len(t, loaded.sources, 2)
	a, b := loaded.sources[0], loaded.sources[1]
	require.Equal(t, filepath.Join(dir, "a.fb"), a.Path)
	require.Equal(t, filepath.Join(dir, "b.fb"), b.Path)

	// The merged root has no single source.
	require.NotContains(t, loaded.nodeSources, core.AstNode(loaded.grammar))
	wantSources := map[string]*ast.SourceFile{"Greeting": a, "Farewell": b}
	wantTexts := map[string]string{"Greeting": `entry Greeting: "hello" Name=ID`, "Farewell": `Farewell: "goodbye" Name=ID`}
	for _, rule := range loaded.grammar.Rules() {
		want := wantSources[rule.Name()]
		for node := range core.AllNodes(rule) {
			require.Same(t, want, loaded.nodeSources[node], "rule %s", rule.Name())
		}
		// The merged document has no text, so the range must be read from the
		// recorded source.
		rng := rule.TextRange()
		text := ast.Location{Source: want, Start: int(rng.Start), End: int(rng.End)}.Text()
		require.Equal(t, wantTexts[rule.Name()], text)
	}
	for _, iface := range loaded.grammar.Interfaces() {
		require.Same(t, a, loaded.nodeSources[iface])
	}
}

func TestLoadGrammarDirRecordsNodeSourcesForSingleFile(t *testing.T) {
	dir := writeGrammarDir(t, "a.fb", modeGrammar)
	loaded, err := loadGrammarDir(dir)
	require.NoError(t, err)
	require.Len(t, loaded.sources, 1)
	for node := range core.AllNodes(loaded.grammar) {
		require.Same(t, loaded.sources[0], loaded.nodeSources[node])
	}
}

// splitASTGrammarA and splitASTGrammarB form a grammar whose nodes reference
// each other across files, with an infix rule in the second file.
const (
	splitASTGrammarA = `grammar Multi
interface Model { Exprs []Expr }
interface Expr {}
entry Model: Exprs+=Expr*
Num returns Expr: {Num} Value=NUM
token NUM: /[0-9]+/
hidden token WS: /\s+/
`
	splitASTGrammarB = `grammar Multi
interface Num extends Expr { Value string }
interface Bin extends Expr { Left Expr Operator string Right Expr }
Expr returns Expr: Bin
infix Bin on Num: "*" > Plus
token group Plus { "+" NUM }
`
)

func TestBuildPassesMultiFileGrammarToPlugin(t *testing.T) {
	plugin := &recordingPlugin{name: "rec"}
	ctx := &BuildContext{
		Input:     writeGrammarDir(t, "a.fb", splitASTGrammarA, "b.fb", splitASTGrammarB),
		Output:    t.TempDir(),
		Package:   "multi",
		Languages: []Language{{Entry: "Model", LanguageID: "model"}},
		Plugins:   []pluginapi.Plugin{plugin},
	}
	require.NoError(t, ctx.Build())
	artifacts := plugin.artifacts[0]
	g := artifacts.Grammar
	require.NotNil(t, g)
	a, b := artifacts.Sources[0], artifacts.Sources[1]

	// The merged root has no source.
	require.Equal(t, "Multi", g.Name)
	require.Equal(t, ast.Location{}, g.Location())
	require.Nil(t, g.Container())

	ifaces := map[string]*ast.Interface{}
	for _, i := range g.Interfaces {
		ifaces[i.Name] = i
		require.Same(t, g, i.Container())
	}
	// Interfaces in b.fb extend an interface in a.fb.
	require.Equal(t, []*ast.Interface{ifaces["Expr"]}, ifaces["Num"].Extends)
	require.Same(t, b, ifaces["Num"].Location().Source)
	require.Equal(t, "interface Num extends Expr { Value string }", ifaces["Num"].Location().Text())
	value := ifaces["Num"].Fields[0]
	require.Same(t, b, value.Location().Source)
	require.Equal(t, "Value string", value.Location().Text())
	// A field type in a.fb refers to an interface in a.fb.
	exprs := ifaces["Model"].Fields[0]
	require.Same(t, a, exprs.Location().Source)
	require.Same(t, ifaces["Expr"], exprs.Type.(*ast.ArrayType).InternalType.(*ast.SimpleType).Type)

	rules := map[string]*ast.ParserRule{}
	for _, r := range g.Rules {
		rules[r.Name] = r
	}
	// A rule in a.fb returns an interface declared in b.fb and vice versa.
	require.Same(t, ifaces["Expr"], rules["Num"].ReturnType)
	require.Same(t, a, rules["Num"].Location().Source)
	require.Same(t, ifaces["Expr"], rules["Expr"].ReturnType)
	require.Same(t, b, rules["Expr"].Location().Source)
	require.True(t, rules["Model"].Entry)

	groups := map[string]*ast.TokenGroup{}
	for _, group := range g.TokenGroups {
		groups[group.Name] = group
	}
	num := g.Terminals[0]
	require.Equal(t, "NUM", num.Name)
	// A group in b.fb references a token in a.fb.
	require.Equal(t, []ast.AbstractTokenRule{num}, groups["Plus"].TokenRefs)
	require.Same(t, b, groups["Plus"].Location().Source)

	// The synthetic operator group is located at its infix rule in b.fb,
	// although it is attached to the merged root.
	operators := groups["BinOperator"]
	require.True(t, operators.Synthetic)
	require.Same(t, g, operators.Container())
	require.Same(t, b, operators.Location().Source)
	require.Equal(t, `infix Bin on Num: "*" > Plus`, operators.Location().Text())
	require.Equal(t, []ast.AbstractTokenRule{groups["Plus"]}, operators.TokenRefs)
	// So is the synthetic body of the infix rule.
	require.Len(t, g.InfixRules, 1)
	infix := g.InfixRules[0]
	body := infix.Body.(*ast.Group)
	require.True(t, body.Synthetic)
	require.Same(t, b, body.Location().Source)
	require.Equal(t, infix.Location(), body.Location())
	operator := body.Elements[1].(*ast.Group).Elements[0].(*ast.RuleCall)
	require.True(t, operator.Synthetic)
	require.Same(t, operators, operator.Rule)
	require.Same(t, operators.TokenType, operator.TokenType)
	require.Same(t, b, operator.Location().Source)
	require.Same(t, rules["Model"], artifacts.Languages[0].EntryRule)
	require.Len(t, operators.Keywords, 1)
	require.True(t, operators.Keywords[0].Synthetic)
	require.Same(t, b, operators.Keywords[0].Location().Source)
	require.Equal(t, `"*"`, operators.Keywords[0].Location().Text())

	// Token declarations and token types agree.
	for _, decl := range []ast.TokenDeclaration{num, g.Terminals[1], groups["Plus"], operators} {
		var tt *tokens.TokenType
		switch d := decl.(type) {
		case *ast.TokenDecl:
			tt = d.TokenType
		case *ast.TokenGroup:
			tt = d.TokenType
		}
		require.Same(t, tt, artifacts.TokenTypes[tt.Index])
		require.Same(t, decl, artifacts.TokenDeclaration(tt))
	}
	require.Nil(t, artifacts.TokenDeclaration(artifacts.TokenTypes[0]), "EOF")
	for _, tt := range artifacts.TokenTypes {
		if tt.Kind == tokens.KindKeyword {
			require.Nil(t, artifacts.TokenDeclaration(tt), "keyword %s", tt.Name)
		}
	}
}

func TestBuildPassesSingleFileGrammarToPlugin(t *testing.T) {
	plugin := &recordingPlugin{name: "rec"}
	ctx := greetingBuildContext(t, plugin)
	require.NoError(t, ctx.Build())
	artifacts := plugin.artifacts[0]
	g := artifacts.Grammar
	require.Same(t, artifacts.Sources[0], g.Location().Source)
	require.Equal(t, modeGrammar, g.Location().Text())

	id := g.Terminals[0]
	require.Equal(t, "ID", id.Name)
	require.Same(t, id, artifacts.TokenDeclaration(id.TokenType))
	inner := g.TokenModes[1]
	hello := g.TokenModes[0].Members[0].(*ast.KeywordUsage)
	require.Same(t, inner, hello.Command.Mode)
	require.Nil(t, artifacts.TokenDeclaration(hello.Keyword.TokenType))
}

// The types span all files: an interface in one file is returned by rules
// in others, implicitly by name and explicitly via returns, and extends an
// interface of a third file.
func TestBuildPassesTypesToPlugin(t *testing.T) {
	plugin := &recordingPlugin{name: "rec"}
	ctx := &BuildContext{
		Input: writeGrammarDir(t,
			"a.fb", "grammar Multi\ninterface Named { Name string }\n"+multiTokens,
			"b.fb", "grammar Multi\ninterface Greeting extends Named { To *Named }\n",
			"c.fb", "grammar Multi\nentry Greeting: \"hello\" Name=ID (\"to\" To=[Named:ID])?\nOther returns Greeting: \"hi\" Name=ID\n",
		),
		Output:    t.TempDir(),
		Package:   "multi",
		Languages: []Language{{Entry: "Greeting", LanguageID: "greeting"}},
		Plugins:   []pluginapi.Plugin{plugin},
	}
	require.NoError(t, ctx.Build())

	artifacts := plugin.artifacts[0]
	require.Len(t, artifacts.Types, len(artifacts.Grammar.Interfaces))
	for i, typ := range artifacts.Types {
		require.Same(t, artifacts.Grammar.Interfaces[i], typ.Declaration)
	}
	// Same order as the generated interfaces.
	code, err := os.ReadFile(filepath.Join(ctx.Output, "types_gen.go"))
	require.NoError(t, err)
	declared := regexp.MustCompile(`(?m)^type (\w+) interface \{`).FindAllStringSubmatch(string(code), -1)
	require.Len(t, declared, len(artifacts.Types))
	for i, typ := range artifacts.Types {
		require.Equal(t, declared[i][1], typ.Name)
	}

	var greeting, named *types.Type
	for _, typ := range artifacts.Types {
		switch typ.Name {
		case "Greeting":
			greeting = typ
		case "Named":
			named = typ
		}
	}
	require.NotNil(t, greeting)
	require.NotNil(t, named)
	require.Equal(t, []*types.Type{named}, greeting.Extends)
	require.Equal(t, []*types.Type{greeting}, named.Subtypes)
	require.Equal(t, []*types.Field{named.OwnFields[0], greeting.OwnFields[0]}, greeting.AllFields)
	require.Same(t, named, greeting.OwnFields[0].Type.Target)
	require.Equal(t, artifacts.Sources[1], greeting.Declaration.Location().Source)

	require.Len(t, greeting.ReturnedBy, 2)
	returnedBy := map[string]*ast.ParserRule{}
	for _, r := range greeting.ReturnedBy {
		returnedBy[r.Name] = r
		require.Same(t, artifacts.Sources[2], r.Location().Source)
	}
	require.Same(t, artifacts.Languages[0].EntryRule, returnedBy["Greeting"])
	require.Same(t, greeting.Declaration, returnedBy["Other"].ReturnType)
	require.Empty(t, named.ReturnedBy)
}
