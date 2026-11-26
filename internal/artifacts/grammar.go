// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package artifacts

import (
	"context"
	"fmt"

	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/internal/artifacts/astinit"
	"typefox.dev/fastbelt/internal/generator"
	"typefox.dev/fastbelt/internal/grammar"
	"typefox.dev/fastbelt/pluginapi/ast"
	"typefox.dev/fastbelt/pluginapi/tokens"
)

// Grammar converts the grammar AST into its plugin mirror. It also returns
// the token declaration of each token type that has one, see
// [typefox.dev/fastbelt/pluginapi.BuildArtifacts.TokenDeclaration].
//
// g must be the grammar after [grammar.ExpandInfixRules]. nodeSources maps
// every parsed node to its source file, recorded before merging; g itself has
// no entry if it was merged from several files. Nodes without an entry were
// created by infix expansion: they are marked synthetic and located in the
// file of their infix rule. result and tts are the token types of g, see
// [TokenTypes].
//
// The conversion runs in two passes: the first creates all nodes, the second
// resolves the references between them. It fails on an unresolved reference
// or a node without a token index, which the build rules out beforehand.
func Grammar(g grammar.Grammar, nodeSources map[core.AstNode]*ast.SourceFile, result generator.GenerateTokenTypesResult, tts []*tokens.TokenType) (*ast.Grammar, map[*tokens.TokenType]ast.TokenDeclaration, error) {
	c := &grammarConverter{
		ctx:         context.Background(),
		root:        g,
		nodeSources: nodeSources,
		origins:     map[core.AstNode]core.AstNode{},
		result:      result,
		tts:         tts,
		nodes:       map[core.AstNode]ast.Node{},
	}
	// Infix expansion attaches the operator groups to the root, so the file
	// of a synthetic group cannot be found through its container. The group
	// is matched to its rule by the name ExpandInfixRules gives it; a parsed
	// group of that name would have made the expansion fail.
	for _, rule := range g.InfixRules() {
		name := grammar.InfixOperatorGroupName(rule)
		for _, group := range g.TokenGroups() {
			if _, parsed := nodeSources[group]; !parsed && group.Name() == name {
				c.origins[group] = rule
			}
		}
	}

	root := c.grammar(g)
	if c.err != nil {
		return nil, nil, c.err
	}
	for _, link := range c.links {
		if err := link(); err != nil {
			return nil, nil, err
		}
	}
	decls, err := c.tokenDeclarations()
	if err != nil {
		return nil, nil, err
	}
	return root, decls, nil
}

// grammarConverter holds the state of a [Grammar] conversion.
type grammarConverter struct {
	ctx         context.Context
	root        grammar.Grammar
	nodeSources map[core.AstNode]*ast.SourceFile
	// origins maps the synthetic operator groups to their infix rules.
	origins map[core.AstNode]core.AstNode
	result  generator.GenerateTokenTypesResult
	tts     []*tokens.TokenType
	// nodes maps every converted node to its mirror.
	nodes map[core.AstNode]ast.Node
	// links resolve the references once all nodes exist.
	links []func() error
	// err is the first error of the first pass.
	err error
}

// init registers n as the mirror of src and sets its container and location.
func (c *grammarConverter) init(n ast.Node, src core.AstNode, container ast.Node) {
	c.nodes[src] = n
	astinit.SetContainer(n, container)
	astinit.SetLocation(n, c.location(src))
}

func (c *grammarConverter) fail(err error) {
	if c.err == nil {
		c.err = err
	}
}

// sourceOf returns the file src was parsed from, or for a synthetic node the
// file of its infix rule. It is nil for the merged root.
func (c *grammarConverter) sourceOf(src core.AstNode) *ast.SourceFile {
	for n := src; n != nil; n = n.Container() {
		if source, ok := c.nodeSources[n]; ok {
			return source
		}
		if rule, ok := c.origins[n]; ok {
			return c.nodeSources[rule]
		}
	}
	return nil
}

func (c *grammarConverter) location(src core.AstNode) ast.Location {
	source := c.sourceOf(src)
	if source == nil {
		return ast.Location{}
	}
	if src == core.AstNode(c.root) {
		// The root of a single-file grammar covers the whole file, including
		// leading and trailing comments and whitespace.
		return ast.Location{Source: source, Start: 0, End: len(source.Text)}
	}
	rng := src.TextRange()
	return ast.Location{Source: source, Start: int(rng.Start), End: int(rng.End)}
}

// synthetic reports whether src was created by infix expansion.
func (c *grammarConverter) synthetic(src core.AstNode) bool {
	_, parsed := c.nodeSources[src]
	return !parsed && src != core.AstNode(c.root)
}

func (c *grammarConverter) tokenType(index int, ok bool, what string) *tokens.TokenType {
	if !ok || index < 0 || index >= len(c.tts) {
		c.fail(fmt.Errorf("%s has no token type", what))
		return nil
	}
	return c.tts[index]
}

// link resolves ref in the second pass and passes the mirror of its target to
// set. A nil ref (an absent optional reference) is skipped.
func link[S core.AstNode, T ast.Node](c *grammarConverter, ref *core.Reference[S], set func(T)) {
	if ref == nil {
		return
	}
	c.links = append(c.links, func() error {
		target := ref.Ref(c.ctx)
		mirror, ok := c.nodes[core.AstNode(target)]
		if !ok {
			return fmt.Errorf("unresolved reference %q", ref.Text())
		}
		typed, ok := mirror.(T)
		if !ok {
			return fmt.Errorf("reference %q resolves to %T, want %T", ref.Text(), mirror, *new(T))
		}
		set(typed)
		return nil
	})
}

// linkAll resolves the references of a list into a slice of the same length.
func linkAll[S core.AstNode, T ast.Node](c *grammarConverter, refs []*core.Reference[S]) []T {
	if len(refs) == 0 {
		return nil
	}
	targets := make([]T, len(refs))
	for i, ref := range refs {
		link(c, ref, func(t T) { targets[i] = t })
	}
	return targets
}

// convertAll converts each item of a list, nil for an empty list.
func convertAll[S any, T any](items []S, convert func(S) T) []T {
	if len(items) == 0 {
		return nil
	}
	converted := make([]T, len(items))
	for i, item := range items {
		converted[i] = convert(item)
	}
	return converted
}

func (c *grammarConverter) grammar(g grammar.Grammar) *ast.Grammar {
	n := &ast.Grammar{Name: g.Name()}
	c.init(n, g, nil)
	n.Rules = convertAll(g.Rules(), func(r grammar.ParserRule) *ast.ParserRule { return c.parserRule(r, n) })
	n.Terminals = convertAll(g.Terminals(), func(d grammar.TokenDecl) *ast.TokenDecl { return c.tokenDecl(d, n) })
	n.TokenGroups = convertAll(g.TokenGroups(), func(tg grammar.TokenGroup) *ast.TokenGroup { return c.tokenGroup(tg, n) })
	n.TokenModes = convertAll(g.TokenModes(), func(m grammar.TokenMode) *ast.TokenMode { return c.tokenMode(m, n) })
	n.Interfaces = convertAll(g.Interfaces(), func(i grammar.Interface) *ast.Interface { return c.iface(i, n) })
	return n
}

func (c *grammarConverter) iface(i grammar.Interface, container ast.Node) *ast.Interface {
	n := &ast.Interface{Name: i.Name()}
	c.init(n, i, container)
	n.Extends = linkAll[grammar.Interface, *ast.Interface](c, i.Extends())
	n.Fields = convertAll(i.Fields(), func(f grammar.Field) *ast.Field { return c.field(f, n) })
	return n
}

func (c *grammarConverter) field(f grammar.Field, container ast.Node) *ast.Field {
	n := &ast.Field{Name: f.Name()}
	c.init(n, f, container)
	n.Type = c.fieldType(f.Type(), n)
	return n
}

func (c *grammarConverter) fieldType(ft grammar.FieldType, container ast.Node) ast.FieldType {
	switch ft := ft.(type) {
	case grammar.ArrayType:
		n := &ast.ArrayType{}
		c.init(n, ft, container)
		n.InternalType = c.fieldType(ft.InternalType(), n)
		return n
	case grammar.ReferenceType:
		n := &ast.ReferenceType{}
		c.init(n, ft, container)
		link(c, ft.Type(), func(t *ast.Interface) { n.Type = t })
		return n
	case grammar.SimpleType:
		n := &ast.SimpleType{}
		c.init(n, ft, container)
		link(c, ft.Type(), func(t *ast.Interface) { n.Type = t })
		return n
	case grammar.PrimitiveType:
		n := &ast.PrimitiveType{Type: ast.Primitive(ft.Type())}
		c.init(n, ft, container)
		return n
	case nil:
		return nil
	default:
		c.fail(fmt.Errorf("unknown field type %T", ft))
		return nil
	}
}

func (c *grammarConverter) parserRule(r grammar.ParserRule, container ast.Node) *ast.ParserRule {
	n := &ast.ParserRule{Name: r.Name(), Entry: r.IsEntry()}
	c.init(n, r, container)
	link(c, r.ReturnType(), func(t *ast.Interface) { n.ReturnType = t })
	return n
}

func (c *grammarConverter) tokenDecl(d grammar.TokenDecl, container ast.Node) *ast.TokenDecl {
	n := &ast.TokenDecl{Name: d.Name(), Modifier: tokens.Modifier(d.Modifier())}
	c.init(n, d, container)
	switch content := d.Content().(type) {
	case grammar.RegexpTokenContent:
		rc := &ast.RegexpTokenContent{Regexp: grammar.RegexpValue(content.Regexp())}
		c.init(rc, content, n)
		n.Content = rc
	case grammar.KeywordTokenContent:
		kc := &ast.KeywordTokenContent{}
		c.init(kc, content, n)
		kc.Keyword = c.keyword(content.Keyword(), kc)
		n.Content = kc
	default:
		c.fail(fmt.Errorf("token %s: unknown content %T", d.Name(), content))
	}
	n.Command = c.command(d.Command(), n)
	index, ok := c.result.TokenDeclTokenIndex(d)
	n.TokenType = c.tokenType(index, ok, "token "+d.Name())
	return n
}

func (c *grammarConverter) keyword(k grammar.Keyword, container ast.Node) *ast.Keyword {
	n := &ast.Keyword{Value: grammar.KeywordValue(k), Synthetic: c.synthetic(k)}
	c.init(n, k, container)
	index, ok := c.result.KeywordTokenIndex(k)
	n.TokenType = c.tokenType(index, ok, "keyword "+k.Value())
	return n
}

// command converts an optional mode command; nil stays nil.
func (c *grammarConverter) command(cmd grammar.TokenCommand, container ast.Node) *ast.TokenCommand {
	if cmd == nil {
		return nil
	}
	n := &ast.TokenCommand{Type: tokens.CommandKind(cmd.Type())}
	c.init(n, cmd, container)
	link(c, cmd.Mode(), func(t *ast.TokenMode) { n.Mode = t })
	return n
}

func (c *grammarConverter) tokenGroup(tg grammar.TokenGroup, container ast.Node) *ast.TokenGroup {
	n := &ast.TokenGroup{Name: tg.Name(), Synthetic: c.synthetic(tg)}
	c.init(n, tg, container)
	n.TokenRefs = linkAll[grammar.AbstractTokenRule, ast.AbstractTokenRule](c, tg.TokenRefs())
	n.Keywords = convertAll(tg.Keywords(), func(k grammar.Keyword) *ast.Keyword { return c.keyword(k, n) })
	n.KeywordSelectors = convertAll(tg.KeywordSelectors(), func(t *core.Token) string { return grammar.RegexpValue(t.Image) })
	index, ok := c.result.TokenGroupTokenIndex(tg)
	n.TokenType = c.tokenType(index, ok, "token group "+tg.Name())
	return n
}

func (c *grammarConverter) tokenMode(m grammar.TokenMode, container ast.Node) *ast.TokenMode {
	n := &ast.TokenMode{Name: m.Name()}
	c.init(n, m, container)
	n.Members = convertAll(m.Members(), func(member grammar.TokenModeMember) ast.TokenModeMember {
		return c.tokenModeMember(member, n)
	})
	return n
}

func (c *grammarConverter) tokenModeMember(member grammar.TokenModeMember, container ast.Node) ast.TokenModeMember {
	switch member := member.(type) {
	case grammar.TokenDeclUsage:
		n := &ast.TokenDeclUsage{}
		c.init(n, member, container)
		n.Declaration = c.tokenDecl(member.Declaration(), n)
		return n
	case grammar.TokenGroupUsage:
		n := &ast.TokenGroupUsage{}
		c.init(n, member, container)
		n.Group = c.tokenGroup(member.Group(), n)
		return n
	case grammar.TokenUsage:
		n := &ast.TokenUsage{Modifier: tokens.Modifier(member.Modifier())}
		c.init(n, member, container)
		link(c, member.TokenRef(), func(t ast.AbstractTokenRule) { n.TokenRef = t })
		n.Command = c.command(member.Command(), n)
		return n
	case grammar.KeywordUsage:
		n := &ast.KeywordUsage{Modifier: tokens.Modifier(member.Modifier())}
		c.init(n, member, container)
		n.Keyword = c.keyword(member.Keyword(), n)
		n.Command = c.command(member.Command(), n)
		return n
	case grammar.KeywordSelector:
		n := &ast.KeywordSelector{Selector: grammar.RegexpValue(member.Selector())}
		c.init(n, member, container)
		return n
	default:
		c.fail(fmt.Errorf("token mode member: unknown type %T", member))
		return nil
	}
}

// tokenDeclarations maps each token type to its declaration. Declarations
// are visited in the order [TokenTypes] uses, so a keyword shared by several
// declarations maps to the one that gave the token type its name.
func (c *grammarConverter) tokenDeclarations() (map[*tokens.TokenType]ast.TokenDeclaration, error) {
	decls := map[*tokens.TokenType]ast.TokenDeclaration{}
	add := func(src core.AstNode, name string) error {
		decl, ok := c.nodes[src].(ast.TokenDeclaration)
		if !ok {
			return fmt.Errorf("%s was not converted", name)
		}
		var tt *tokens.TokenType
		switch decl := decl.(type) {
		case *ast.TokenDecl:
			tt = decl.TokenType
		case *ast.TokenGroup:
			tt = decl.TokenType
		}
		if _, taken := decls[tt]; !taken {
			decls[tt] = decl
		}
		return nil
	}
	for _, d := range c.result.TokenDecls.All {
		if err := add(d, "token "+d.Name()); err != nil {
			return nil, err
		}
	}
	for _, g := range c.result.TokenGroups.All {
		if err := add(g, "token group "+g.Name()); err != nil {
			return nil, err
		}
	}
	return decls, nil
}
