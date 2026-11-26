// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package artifacts

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/internal/generator"
	"typefox.dev/fastbelt/internal/grammar"
	"typefox.dev/fastbelt/pluginapi/ast"
	"typefox.dev/fastbelt/test"
)

const elementsGrammar = `grammar Elems;

interface Model {
    Name composite
    Flag bool
    Ref *Model
    Kind string
    Items []Item
}
interface Item {}
interface Leaf extends Item { Value string }
interface Pair extends Item { Left Item Right Item }

entry Model: "model" Name=QName Flag?="flag"? ("ref" Ref=[Model:QName])? Kind=("a" | Sign) Items+=Item+;
Item returns Item: Leaf ({Pair.Left=current} "," Right=Leaf)?;
Leaf returns Leaf: {Leaf} Value=ID | "(" Value=INT ")";
composite QName: ID ("." ID)*;

token ID: /[a-z]+/;
token INT: /[0-9]+/;
token group Sign { "+" "-" }
hidden token WS: /\s+/;
`

func composite(t *testing.T, g *ast.Grammar, name string) *ast.CompositeRule {
	return named(t, g.Composites, name, func(r *ast.CompositeRule) string { return r.Name })
}

func infixRule(t *testing.T, g *ast.Grammar, name string) *ast.InfixRule {
	return named(t, g.InfixRules, name, func(r *ast.InfixRule) string { return r.Name })
}

// requireKeyword checks that e is a keyword with the given value and its
// token type.
func requireKeyword(t *testing.T, c converted, value string, e ast.Element) *ast.Keyword {
	t.Helper()
	kw, ok := e.(*ast.Keyword)
	require.True(t, ok, "want *ast.Keyword, got %T", e)
	require.Equal(t, value, kw.Value)
	require.Same(t, byName(t, c.tts, value), kw.TokenType)
	require.False(t, kw.Synthetic)
	return kw
}

func TestElementsOfParserRule(t *testing.T) {
	c := grammarOf(t, elementsGrammar)
	g := c.root
	modelRule := rule(t, g, "Model")
	model := iface(t, g, "Model")

	body := modelRule.Body.(*ast.Group)
	requireLocated(t, c.source, `"model" Name=QName Flag?="flag"? ("ref" Ref=[Model:QName])? Kind=("a" | Sign) Items+=Item+`, modelRule, body)
	require.Equal(t, ast.CardinalityOne, body.Cardinality)
	require.False(t, body.Synthetic)
	require.Len(t, body.Elements, 6)

	kw := requireKeyword(t, c, "model", body.Elements[0])
	requireLocated(t, c.source, `"model"`, body, kw)
	require.Equal(t, ast.CardinalityOne, kw.Cardinality)

	// Name=QName calls a composite rule: no token type.
	name := body.Elements[1].(*ast.Assignment)
	requireLocated(t, c.source, "Name=QName", body, name)
	require.Same(t, field(t, model, "Name"), name.Property)
	require.Equal(t, ast.AssignmentSet, name.Operator)
	qname := name.Value.(*ast.RuleCall)
	requireLocated(t, c.source, "QName", name, qname)
	require.Same(t, composite(t, g, "QName"), qname.Rule)
	require.Nil(t, qname.TokenType)

	flag := body.Elements[2].(*ast.Assignment)
	requireLocated(t, c.source, `Flag?="flag"?`, body, flag)
	require.Equal(t, ast.AssignmentFlag, flag.Operator)
	require.Equal(t, ast.CardinalityOptional, flag.Cardinality)
	require.Same(t, field(t, model, "Flag"), flag.Property)
	requireLocated(t, c.source, `"flag"`, flag, requireKeyword(t, c, "flag", flag.Value))

	ref := body.Elements[3].(*ast.Group)
	require.Equal(t, ast.CardinalityOptional, ref.Cardinality)
	require.Same(t, body, ref.Container())
	requireKeyword(t, c, "ref", ref.Elements[0])
	refAssignment := ref.Elements[1].(*ast.Assignment)
	require.Same(t, field(t, model, "Ref"), refAssignment.Property)
	crossRef := refAssignment.Value.(*ast.CrossRef)
	requireLocated(t, c.source, "[Model:QName]", refAssignment, crossRef)
	require.Same(t, model, crossRef.Type)
	require.Equal(t, ast.CardinalityOne, crossRef.Cardinality)
	requireLocated(t, c.source, "QName", crossRef, crossRef.Rule)
	require.Same(t, composite(t, g, "QName"), crossRef.Rule.Rule)

	kind := body.Elements[4].(*ast.Assignment)
	require.Same(t, field(t, model, "Kind"), kind.Property)
	alts := kind.Value.(*ast.Alternatives)
	require.Same(t, kind, alts.Container())
	require.Len(t, alts.Alts, 2)
	requireLocated(t, c.source, `"a"`, alts, requireKeyword(t, c, "a", alts.Alts[0]))
	sign := alts.Alts[1].(*ast.RuleCall)
	requireLocated(t, c.source, "Sign", alts, sign)
	require.Same(t, tokenGroup(t, g, "Sign"), sign.Rule)
	require.Same(t, byName(t, c.tts, "Sign"), sign.TokenType)
	require.Same(t, tokenGroup(t, g, "Sign").TokenType, sign.TokenType)

	items := body.Elements[5].(*ast.Assignment)
	requireLocated(t, c.source, "Items+=Item+", body, items)
	require.Equal(t, ast.AssignmentAppend, items.Operator)
	require.Equal(t, ast.CardinalityOneOrMore, items.Cardinality)
	itemCall := items.Value.(*ast.RuleCall)
	require.Same(t, rule(t, g, "Item"), itemCall.Rule)
	require.Nil(t, itemCall.TokenType, "parser rule call")
	require.Equal(t, ast.CardinalityOne, itemCall.Cardinality)
	require.False(t, itemCall.Synthetic)
}

func TestElementsActions(t *testing.T) {
	c := grammarOf(t, elementsGrammar)
	g := c.root

	// Item returns Item: Leaf ({Pair.Left=current} "," Right=Leaf)?
	itemRule := rule(t, g, "Item")
	body := itemRule.Body.(*ast.Group)
	require.Same(t, rule(t, g, "Leaf"), body.Elements[0].(*ast.RuleCall).Rule)
	tail := body.Elements[1].(*ast.Group)
	require.Equal(t, ast.CardinalityOptional, tail.Cardinality)
	action := tail.Elements[0].(*ast.Action)
	requireLocated(t, c.source, "{Pair.Left=current}", tail, action)
	pair := iface(t, g, "Pair")
	require.Same(t, pair, action.Type)
	require.Same(t, field(t, pair, "Left"), action.Property)
	require.Equal(t, ast.ActionSet, action.Operator)
	requireKeyword(t, c, ",", tail.Elements[1])
	right := tail.Elements[2].(*ast.Assignment)
	require.Same(t, field(t, pair, "Right"), right.Property)

	// Leaf returns Leaf: {Leaf} Value=ID | "(" Value=INT ")"
	leafRule := rule(t, g, "Leaf")
	alts := leafRule.Body.(*ast.Alternatives)
	requireLocated(t, c.source, `{Leaf} Value=ID | "(" Value=INT ")"`, leafRule, alts)
	require.Len(t, alts.Alts, 2)
	first := alts.Alts[0].(*ast.Group)
	require.Same(t, alts, first.Container())
	leafAction := first.Elements[0].(*ast.Action)
	require.Same(t, iface(t, g, "Leaf"), leafAction.Type)
	require.Nil(t, leafAction.Property)
	require.Equal(t, ast.ActionNone, leafAction.Operator)
	value := first.Elements[1].(*ast.Assignment)
	require.Same(t, field(t, iface(t, g, "Leaf"), "Value"), value.Property)
	id := value.Value.(*ast.RuleCall)
	require.Same(t, terminal(t, g, "ID"), id.Rule)
	require.Same(t, byName(t, c.tts, "ID"), id.TokenType)
	require.Same(t, terminal(t, g, "ID").TokenType, id.TokenType)

	second := alts.Alts[1].(*ast.Group)
	requireKeyword(t, c, "(", second.Elements[0])
	require.Same(t, byName(t, c.tts, "INT"), second.Elements[1].(*ast.Assignment).Value.(*ast.RuleCall).TokenType)
	requireKeyword(t, c, ")", second.Elements[2])
}

func TestElementsOfCompositeRule(t *testing.T) {
	c := grammarOf(t, elementsGrammar)
	g := c.root
	require.Len(t, g.Composites, 1)
	qname := composite(t, g, "QName")
	requireLocated(t, c.source, `composite QName: ID ("." ID)*;`, g, qname)

	body := qname.Body.(*ast.Group)
	requireLocated(t, c.source, `ID ("." ID)*`, qname, body)
	require.Same(t, terminal(t, g, "ID"), body.Elements[0].(*ast.RuleCall).Rule)
	tail := body.Elements[1].(*ast.Group)
	require.Equal(t, ast.CardinalityZeroOrMore, tail.Cardinality)
	requireKeyword(t, c, ".", tail.Elements[0])
	id := tail.Elements[1].(*ast.RuleCall)
	require.Same(t, byName(t, c.tts, "ID"), id.TokenType)
	require.Same(t, tail, id.Container())

	require.Empty(t, g.InfixRules)
}

// TestElementsOfInfixRule checks the infix rule and the synthetic body that
// infix expansion gives it.
func TestElementsOfInfixRule(t *testing.T) {
	c := grammarOfFile(t, "../../examples/arithmetics/arithmetics.fb")
	g := c.root
	require.Len(t, g.InfixRules, 1)
	infix := infixRule(t, g, "BinaryExpression")
	require.Same(t, g, infix.Container())
	ruleText := infix.Location().Text()
	require.True(t, strings.HasPrefix(ruleText, "infix BinaryExpression on PrimaryExpression:"), ruleText)
	require.True(t, strings.HasSuffix(ruleText, "> AdditionOp"), ruleText)
	require.Nil(t, infix.ReturnType)

	primary := rule(t, g, "PrimaryExpression")
	requireLocated(t, c.source, "PrimaryExpression", infix, infix.Call)
	require.Same(t, primary, infix.Call.Rule)
	require.Nil(t, infix.Call.TokenType)
	require.False(t, infix.Call.Synthetic)

	require.Len(t, infix.Groups, 4)
	mod := infix.Groups[0]
	requireLocated(t, c.source, `"%"`, infix, mod)
	require.Equal(t, ast.AssociativityDefault, mod.Associativity)
	requireLocated(t, c.source, `"%"`, mod, requireKeyword(t, c, "%", mod.Operators[0]))
	pow := infix.Groups[1]
	requireLocated(t, c.source, `right "^"`, infix, pow)
	require.Equal(t, ast.AssociativityRight, pow.Associativity)
	requireKeyword(t, c, "^", pow.Operators[0])
	mul := infix.Groups[2].Operators[0].(*ast.RuleCall)
	require.Same(t, tokenGroup(t, g, "MultiplicationOp"), mul.Rule)
	require.Same(t, byName(t, c.tts, "MultiplicationOp"), mul.TokenType)
	require.Same(t, tokenGroup(t, g, "AdditionOp"), infix.Groups[3].Operators[0].(*ast.RuleCall).Rule)

	// Synthetic body: Primary (Operator Primary)*.
	body := infix.Body.(*ast.Group)
	require.True(t, body.Synthetic)
	requireLocated(t, c.source, ruleText, infix, body)
	require.Equal(t, ast.CardinalityOne, body.Cardinality)
	require.Len(t, body.Elements, 2)

	first := body.Elements[0].(*ast.RuleCall)
	require.True(t, first.Synthetic)
	require.Same(t, primary, first.Rule)
	require.Nil(t, first.TokenType)
	requireLocated(t, c.source, "PrimaryExpression", body, first)

	tail := body.Elements[1].(*ast.Group)
	require.True(t, tail.Synthetic)
	require.Equal(t, ast.CardinalityZeroOrMore, tail.Cardinality)
	requireLocated(t, c.source, ruleText, body, tail)
	require.Len(t, tail.Elements, 2)

	operators := tokenGroup(t, g, "BinaryExpressionOperator")
	operator := tail.Elements[0].(*ast.RuleCall)
	require.True(t, operator.Synthetic)
	require.Same(t, operators, operator.Rule)
	require.Same(t, operators.TokenType, operator.TokenType)
	require.Same(t, byName(t, c.tts, "BinaryExpressionOperator"), operator.TokenType)
	requireLocated(t, c.source, ruleText, tail, operator)

	second := tail.Elements[1].(*ast.RuleCall)
	require.True(t, second.Synthetic)
	require.Same(t, primary, second.Rule)
	requireLocated(t, c.source, "PrimaryExpression", tail, second)
	require.NotSame(t, first, second)

	// Parsed elements are not synthetic.
	for _, r := range g.Rules {
		requireNotSynthetic(t, r.Body)
	}
	requireNotSynthetic(t, infix.Call)
}

// requireNotSynthetic checks that no element below e is synthetic.
func requireNotSynthetic(t *testing.T, e ast.Element) {
	t.Helper()
	switch e := e.(type) {
	case *ast.Group:
		require.False(t, e.Synthetic)
		for _, child := range e.Elements {
			requireNotSynthetic(t, child)
		}
	case *ast.Alternatives:
		for _, child := range e.Alts {
			requireNotSynthetic(t, child)
		}
	case *ast.Assignment:
		requireNotSynthetic(t, e.Value)
	case *ast.CrossRef:
		requireNotSynthetic(t, e.Rule)
	case *ast.RuleCall:
		require.False(t, e.Synthetic)
	case *ast.Keyword:
		require.False(t, e.Synthetic)
	}
}

// TestElementsOfFastbeltGrammar converts the bodies of the grammar language
// itself, which use every element kind.
func TestElementsOfFastbeltGrammar(t *testing.T) {
	c := grammarOfFile(t, "../grammar/grammar.fb")
	g := c.root

	// PrecedenceGroup: Associativity=("left" | "right")? Operators+=... ("|" Operators+=...)*
	pg := rule(t, g, "PrecedenceGroup")
	body := pg.Body.(*ast.Group)
	assoc := body.Elements[0].(*ast.Assignment)
	require.Equal(t, ast.CardinalityOptional, assoc.Cardinality)
	require.Equal(t, ast.AssignmentSet, assoc.Operator)
	alts := assoc.Value.(*ast.Alternatives)
	requireKeyword(t, c, "left", alts.Alts[0])
	requireKeyword(t, c, "right", alts.Alts[1])

	// CrossRef: "[" Type=[Interface:ID] (":" Rule=RuleCall)? "]"
	crossRef := rule(t, g, "CrossRef").Body.(*ast.Group).Elements[1].(*ast.Assignment).Value.(*ast.CrossRef)
	require.Same(t, iface(t, g, "Interface"), crossRef.Type)
	require.Same(t, terminal(t, g, "ID"), crossRef.Rule.Rule)
	require.Same(t, byName(t, c.tts, "ID"), crossRef.Rule.TokenType)

	// Alternatives returns Element: Group ({Alternatives.Alts+=current} ...)?
	action := rule(t, g, "Alternatives").Body.(*ast.Group).Elements[1].(*ast.Group).Elements[0].(*ast.Action)
	require.Equal(t, ast.ActionAppend, action.Operator)
	require.Same(t, field(t, iface(t, g, "Alternatives"), "Alts"), action.Property)

	// Element: (...) Cardinality=Cardinality? calls a token group.
	cardinality := rule(t, g, "Element").Body.(*ast.Group).Elements[1].(*ast.Assignment)
	require.Same(t, tokenGroup(t, g, "Cardinality").TokenType, cardinality.Value.(*ast.RuleCall).TokenType)

	require.Len(t, g.Composites, 0)
	require.Len(t, g.InfixRules, 0)
}

// TestElementsSyntheticNodeWithoutFlagIsAnError checks that a node kind
// without a Synthetic flag fails the conversion if it has no source, that is
// if infix expansion would have created it.
func TestElementsSyntheticNodeWithoutFlagIsAnError(t *testing.T) {
	doc := test.New(t, grammar.CreateServices()).Parse(elementsGrammar).AssertNoErrors()
	g := doc.Document.Root.(grammar.Grammar)
	source := ast.NewSourceFile("/test.fb", elementsGrammar)
	nodeSources := map[core.AstNode]*ast.SourceFile{}
	for node := range core.AllNodes(g) {
		if _, alts := node.(grammar.Alternatives); !alts {
			nodeSources[node] = source
		}
	}
	result := generator.GenerateTokenTypes(g)
	tts, err := TokenTypes(g, result)
	require.NoError(t, err)
	_, _, err = Grammar(g, nodeSources, result, tts)
	require.ErrorContains(t, err, "*ast.Alternatives created by infix expansion cannot be flagged as synthetic")
}
