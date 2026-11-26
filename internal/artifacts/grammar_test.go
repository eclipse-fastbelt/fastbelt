// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package artifacts

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/internal/generator"
	"typefox.dev/fastbelt/internal/grammar"
	"typefox.dev/fastbelt/pluginapi/ast"
	"typefox.dev/fastbelt/pluginapi/tokens"
	"typefox.dev/fastbelt/test"
)

// converted holds the converted AST of a grammar and the data it links to.
type converted struct {
	root   *ast.Grammar
	decls  map[*tokens.TokenType]ast.TokenDeclaration
	tts    []*tokens.TokenType
	source *ast.SourceFile
}

// grammarOf converts a single-file grammar the way the build does.
func grammarOf(t *testing.T, text string) converted {
	t.Helper()
	doc := test.New(t, grammar.CreateServices()).Parse(text).AssertNoErrors()
	g := doc.Document.Root.(grammar.Grammar)
	source := ast.NewSourceFile("/test.fb", text)
	nodeSources := map[core.AstNode]*ast.SourceFile{}
	for node := range core.AllNodes(g) {
		nodeSources[node] = source
	}
	require.NoError(t, grammar.ExpandInfixRules(g))
	result := generator.GenerateTokenTypes(g)
	tts, err := TokenTypes(g, result)
	require.NoError(t, err)
	root, decls, err := Grammar(g, nodeSources, result, tts)
	require.NoError(t, err)
	return converted{root: root, decls: decls, tts: tts, source: source}
}

func grammarOfFile(t *testing.T, path string) converted {
	t.Helper()
	text, err := os.ReadFile(path)
	require.NoError(t, err)
	return grammarOf(t, string(text))
}

// named returns the element of items with the given name.
func named[T any](t *testing.T, items []T, name string, nameOf func(T) string) T {
	t.Helper()
	for _, item := range items {
		if nameOf(item) == name {
			return item
		}
	}
	require.Failf(t, "not found", "no element named %s", name)
	var zero T
	return zero
}

func iface(t *testing.T, g *ast.Grammar, name string) *ast.Interface {
	return named(t, g.Interfaces, name, func(i *ast.Interface) string { return i.Name })
}

func rule(t *testing.T, g *ast.Grammar, name string) *ast.ParserRule {
	return named(t, g.Rules, name, func(r *ast.ParserRule) string { return r.Name })
}

func terminal(t *testing.T, g *ast.Grammar, name string) *ast.TokenDecl {
	return named(t, g.Terminals, name, func(d *ast.TokenDecl) string { return d.Name })
}

func tokenGroup(t *testing.T, g *ast.Grammar, name string) *ast.TokenGroup {
	return named(t, g.TokenGroups, name, func(d *ast.TokenGroup) string { return d.Name })
}

func tokenMode(t *testing.T, g *ast.Grammar, name string) *ast.TokenMode {
	return named(t, g.TokenModes, name, func(m *ast.TokenMode) string { return m.Name })
}

func field(t *testing.T, i *ast.Interface, name string) *ast.Field {
	return named(t, i.Fields, name, func(f *ast.Field) string { return f.Name })
}

// requireLocated checks that node is located at text in source and contained
// in container.
func requireLocated(t *testing.T, source *ast.SourceFile, text string, container, node ast.Node) {
	t.Helper()
	require.Same(t, source, node.Location().Source)
	require.Equal(t, text, node.Location().Text())
	require.Equal(t, container, node.Container())
}

const declsGrammar = `grammar Decls;

interface Base { Name string }
interface Model extends Base {
    Items []Item
    Target *Item
    Flag bool
    Kind composite
}
interface Item extends Base {}

entry Model returns Model: "model" Name=ID Items+=Item* Target=[Item:ID];
Item: "item" Name=ID;

token A: "a";
token B: "a";
token ID: /[a-z]+/;
hidden token WS: /\s+/;
token group Words { ID B "model" keywords /it.*/ }

token mode default {
    "model" -> push(INNER)
    hidden WS
    comment token SL: /\/\/[^\n]*/
    token group Nested { ID Words }
    keywords /it.*/
}

token mode INNER {
    ID -> pop
    WS
    comment "item" -> mode(default)
}
`

func TestGrammarInterfacesAndFields(t *testing.T) {
	c := grammarOf(t, declsGrammar)
	g := c.root
	require.Equal(t, "Decls", g.Name)
	require.Equal(t, []string{"Base", "Model", "Item"}, []string{g.Interfaces[0].Name, g.Interfaces[1].Name, g.Interfaces[2].Name})

	base, model, item := iface(t, g, "Base"), iface(t, g, "Model"), iface(t, g, "Item")
	require.Equal(t, []*ast.Interface{base}, model.Extends)
	require.Equal(t, []*ast.Interface{base}, item.Extends)
	require.Empty(t, base.Extends)

	name := field(t, base, "Name")
	require.Equal(t, ast.PrimitiveString, name.Type.(*ast.PrimitiveType).Type)

	items := field(t, model, "Items").Type.(*ast.ArrayType)
	require.Same(t, item, items.InternalType.(*ast.SimpleType).Type)
	require.Same(t, item, field(t, model, "Target").Type.(*ast.ReferenceType).Type)
	require.Equal(t, ast.PrimitiveBool, field(t, model, "Flag").Type.(*ast.PrimitiveType).Type)
	require.Equal(t, ast.PrimitiveComposite, field(t, model, "Kind").Type.(*ast.PrimitiveType).Type)

	// Containers and locations of nested nodes.
	require.Nil(t, g.Container())
	requireLocated(t, c.source, "interface Base { Name string }", g, base)
	requireLocated(t, c.source, "Name string", base, name)
	requireLocated(t, c.source, "string", name, name.Type)
	itemsField := field(t, model, "Items")
	requireLocated(t, c.source, "[]Item", itemsField, itemsField.Type)
	requireLocated(t, c.source, "Item", itemsField.Type, items.InternalType)
}

func TestGrammarParserRules(t *testing.T) {
	c := grammarOf(t, declsGrammar)
	g := c.root
	model, item := rule(t, g, "Model"), rule(t, g, "Item")
	require.True(t, model.Entry)
	require.False(t, item.Entry)
	require.Same(t, iface(t, g, "Model"), model.ReturnType)
	require.Nil(t, item.ReturnType)
	require.Nil(t, model.Body, "rule bodies follow in step 7")
	requireLocated(t, c.source, `Item: "item" Name=ID;`, g, item)
}

func TestGrammarTokenDecls(t *testing.T) {
	c := grammarOf(t, declsGrammar)
	g := c.root

	id := terminal(t, g, "ID")
	require.Equal(t, tokens.ModifierNone, id.Modifier)
	require.Nil(t, id.Command)
	content := id.Content.(*ast.RegexpTokenContent)
	require.Equal(t, "[a-z]+", content.Regexp)
	requireLocated(t, c.source, "token ID: /[a-z]+/;", g, id)
	requireLocated(t, c.source, "/[a-z]+/", id, content)
	require.Same(t, byName(t, c.tts, "ID"), id.TokenType)

	ws := terminal(t, g, "WS")
	require.Equal(t, tokens.ModifierHidden, ws.Modifier)

	a, b := terminal(t, g, "A"), terminal(t, g, "B")
	kw := a.Content.(*ast.KeywordTokenContent)
	require.Equal(t, "a", kw.Keyword.Value)
	requireLocated(t, c.source, `"a"`, a, kw)
	requireLocated(t, c.source, `"a"`, kw, kw.Keyword)
	// Both declarations share the keyword's token type.
	shared := byName(t, c.tts, "A")
	require.Equal(t, tokens.KindKeyword, shared.Kind)
	require.Same(t, shared, a.TokenType)
	require.Same(t, shared, b.TokenType)
	require.Same(t, shared, kw.Keyword.TokenType)
	require.False(t, kw.Keyword.Synthetic)
}

func TestGrammarTokenGroups(t *testing.T) {
	c := grammarOf(t, declsGrammar)
	g := c.root
	words := tokenGroup(t, g, "Words")
	require.Equal(t, []ast.AbstractTokenRule{terminal(t, g, "ID"), terminal(t, g, "B")}, words.TokenRefs)
	require.Len(t, words.Keywords, 1)
	require.Equal(t, "model", words.Keywords[0].Value)
	require.Same(t, byName(t, c.tts, "model"), words.Keywords[0].TokenType)
	requireLocated(t, c.source, `"model"`, words, words.Keywords[0])
	require.Equal(t, []string{"it.*"}, words.KeywordSelectors)
	require.Same(t, byName(t, c.tts, "Words"), words.TokenType)
	require.False(t, words.Synthetic)
	requireLocated(t, c.source, `token group Words { ID B "model" keywords /it.*/ }`, g, words)
}

func TestGrammarTokenModes(t *testing.T) {
	c := grammarOf(t, declsGrammar)
	g := c.root
	outer, inner := tokenMode(t, g, "default"), tokenMode(t, g, "INNER")
	require.Equal(t, []*ast.TokenMode{outer, inner}, g.TokenModes)
	require.Len(t, outer.Members, 5)
	require.Len(t, inner.Members, 3)

	model := outer.Members[0].(*ast.KeywordUsage)
	requireLocated(t, c.source, `"model" -> push(INNER)`, outer, model)
	require.Equal(t, tokens.ModifierNone, model.Modifier)
	require.Equal(t, "model", model.Keyword.Value)
	require.Same(t, byName(t, c.tts, "model"), model.Keyword.TokenType)
	requireLocated(t, c.source, `"model"`, model, model.Keyword)
	require.Equal(t, tokens.CommandPush, model.Command.Type)
	require.Same(t, inner, model.Command.Mode)
	requireLocated(t, c.source, `-> push(INNER)`, model, model.Command)

	ws := outer.Members[1].(*ast.TokenUsage)
	require.Equal(t, tokens.ModifierHidden, ws.Modifier)
	require.Same(t, terminal(t, g, "WS"), ws.TokenRef)
	require.Nil(t, ws.Command)

	slUsage := outer.Members[2].(*ast.TokenDeclUsage)
	sl := slUsage.Declaration
	requireLocated(t, c.source, `comment token SL: /\/\/[^\n]*/`, slUsage, sl)
	requireLocated(t, c.source, `comment token SL: /\/\/[^\n]*/`, outer, slUsage)
	require.Equal(t, "SL", sl.Name)
	require.Equal(t, tokens.ModifierComment, sl.Modifier)
	require.Equal(t, `\/\/[^\n]*`, sl.Content.(*ast.RegexpTokenContent).Regexp)
	require.Same(t, byName(t, c.tts, "SL"), sl.TokenType)

	nestedUsage := outer.Members[3].(*ast.TokenGroupUsage)
	nested := nestedUsage.Group
	require.Equal(t, "Nested", nested.Name)
	require.Same(t, nestedUsage, nested.Container())
	require.Equal(t, []ast.AbstractTokenRule{terminal(t, g, "ID"), tokenGroup(t, g, "Words")}, nested.TokenRefs)
	require.Same(t, byName(t, c.tts, "Nested"), nested.TokenType)

	selector := outer.Members[4].(*ast.KeywordSelector)
	require.Equal(t, "it.*", selector.Selector)
	requireLocated(t, c.source, `keywords /it.*/`, outer, selector)

	id := inner.Members[0].(*ast.TokenUsage)
	require.Equal(t, tokens.CommandPop, id.Command.Type)
	require.Nil(t, id.Command.Mode)
	require.Nil(t, inner.Members[1].(*ast.TokenUsage).Command)
	item := inner.Members[2].(*ast.KeywordUsage)
	require.Equal(t, tokens.ModifierComment, item.Modifier)
	require.Equal(t, tokens.CommandMode, item.Command.Type)
	require.Same(t, outer, item.Command.Mode)
}

func TestGrammarSingleFileRootCoversFile(t *testing.T) {
	c := grammarOf(t, declsGrammar)
	loc := c.root.Location()
	require.Same(t, c.source, loc.Source)
	require.Equal(t, declsGrammar, loc.Text())
}

func TestGrammarTokenDeclarations(t *testing.T) {
	c := grammarOf(t, declsGrammar)
	g := c.root
	// Every declaration links to its token type and, unless it is a later
	// declaration of a shared keyword, back.
	require.Same(t, terminal(t, g, "ID"), c.decls[byName(t, c.tts, "ID")])
	// A and B share the keyword "a"; the first declaration wins.
	require.Same(t, terminal(t, g, "A"), c.decls[byName(t, c.tts, "A")])
	require.Same(t, terminal(t, g, "B").TokenType, terminal(t, g, "A").TokenType)
	require.Same(t, tokenGroup(t, g, "Words"), c.decls[byName(t, c.tts, "Words")])
	sl := tokenMode(t, g, "default").Members[2].(*ast.TokenDeclUsage).Declaration
	require.Same(t, sl, c.decls[sl.TokenType])
	nested := tokenMode(t, g, "default").Members[3].(*ast.TokenGroupUsage).Group
	require.Same(t, nested, c.decls[nested.TokenType])

	// No declaration for EOF and plain keywords.
	require.NotContains(t, c.decls, c.tts[0])
	require.NotContains(t, c.decls, byName(t, c.tts, "model"))
	require.NotContains(t, c.decls, byName(t, c.tts, "item"))

	for tt, decl := range c.decls {
		switch d := decl.(type) {
		case *ast.TokenDecl:
			require.Same(t, tt, d.TokenType)
		case *ast.TokenGroup:
			require.Same(t, tt, d.TokenType)
		default:
			t.Fatalf("unexpected declaration %T", decl)
		}
	}
	require.Len(t, c.decls, len(c.tts)-3, "all but EOF, model and item")
}

func TestGrammarInfixOperatorGroupIsSynthetic(t *testing.T) {
	path := "../../examples/arithmetics/arithmetics.fb"
	c := grammarOfFile(t, path)
	g := c.root

	operators := tokenGroup(t, g, "BinaryExpressionOperator")
	require.True(t, operators.Synthetic)
	require.Same(t, g, operators.Container())
	require.Same(t, byName(t, c.tts, "BinaryExpressionOperator"), operators.TokenType)
	require.Same(t, operators, c.decls[operators.TokenType])
	// The group is located at its infix rule.
	text := operators.Location().Text()
	require.True(t, strings.HasPrefix(text, "infix BinaryExpression on PrimaryExpression:"), text)
	require.Same(t, c.source, operators.Location().Source)

	require.Equal(t, []ast.AbstractTokenRule{tokenGroup(t, g, "MultiplicationOp"), tokenGroup(t, g, "AdditionOp")}, operators.TokenRefs)
	require.NotEmpty(t, operators.Keywords)
	for _, kw := range operators.Keywords {
		require.True(t, kw.Synthetic)
		require.Same(t, operators, kw.Container())
		// Each keyword is located at the operator it was copied from.
		require.Equal(t, `"`+kw.Value+`"`, kw.Location().Text())
		require.Same(t, byName(t, c.tts, kw.Value), kw.TokenType)
	}

	for _, group := range g.TokenGroups {
		if group != operators {
			require.False(t, group.Synthetic, group.Name)
		}
	}
}

// TestGrammarOfFastbeltGrammar converts the grammar language itself.
func TestGrammarOfFastbeltGrammar(t *testing.T) {
	c := grammarOfFile(t, "../grammar/grammar.fb")
	g := c.root
	require.Equal(t, "Fastbelt", g.Name)
	parserRule := iface(t, g, "ParserRule")
	require.Equal(t, []*ast.Interface{iface(t, g, "AbstractRuleWithReturnType")}, parserRule.Extends)
	require.Same(t, iface(t, g, "Element"), rule(t, g, "Alternatives").ReturnType)
	require.Same(t, tokenGroup(t, g, "Cardinality"), c.decls[tokenGroup(t, g, "Cardinality").TokenType])
	require.Equal(t, tokens.ModifierComment, terminal(t, g, "SL_COMMENT").Modifier)
}

func TestGrammarUnresolvedReferenceIsAnError(t *testing.T) {
	doc := test.New(t, grammar.CreateServices()).Parse(`grammar G;
		interface A extends Missing {}
	`)
	g := doc.Document.Root.(grammar.Grammar)
	result := generator.GenerateTokenTypes(g)
	tts, err := TokenTypes(g, result)
	require.NoError(t, err)
	_, _, err = Grammar(g, map[core.AstNode]*ast.SourceFile{}, result, tts)
	require.ErrorContains(t, err, "Missing")
}
